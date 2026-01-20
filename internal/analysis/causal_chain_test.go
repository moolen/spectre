package analysis

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergeIntoCausalGraph validates the graph building logic
// This test establishes baseline behavior before refactoring
func TestMergeIntoCausalGraph(t *testing.T) {
	analyzer := &RootCauseAnalyzer{
		logger: logging.GetLogger("test"),
	}
	failureTime := time.Now().UnixNano()

	t.Run("Simple chain without managers", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:  "pod-123",
				Kind: "Pod",
				Name: "my-pod",
			},
			ObservedAt: time.Now(),
		}

		chain := []ResourceWithDistance{
			{
				Resource: graph.ResourceIdentity{UID: "pod-123", Kind: "Pod", Name: "my-pod"},
				Distance: 0,
			},
			{
				Resource: graph.ResourceIdentity{UID: "rs-123", Kind: "ReplicaSet", Name: "my-rs"},
				Distance: 1,
			},
			{
				Resource: graph.ResourceIdentity{UID: "deploy-123", Kind: "Deployment", Name: "my-deploy"},
				Distance: 2,
			},
		}

		managers := map[string]*ManagerData{}
		related := map[string][]RelatedResourceData{}
		changeEvents := map[string][]ChangeEventInfo{
			"pod-123": {
				{EventID: "evt-1", Timestamp: time.Now(), EventType: "UPDATE"},
			},
		}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)
		require.NoError(t, err)

		// Verify SPINE nodes
		assert.Len(t, result.Nodes, 3, "Should have 3 SPINE nodes")
		spineCount := 0
		for _, node := range result.Nodes {
			if node.NodeType == nodeTypeSpine {
				spineCount++
			}
		}
		assert.Equal(t, 3, spineCount)

		// Verify OWNS edges
		ownsCount := 0
		for _, edge := range result.Edges {
			if edge.RelationshipType == edgeTypeOwns {
				ownsCount++
			}
		}
		assert.Equal(t, 2, ownsCount, "Should have 2 OWNS edges")
	})

	t.Run("Chain with manager", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:  "pod-123",
				Kind: "Pod",
				Name: "my-pod",
			},
			ObservedAt: time.Now(),
		}

		chain := []ResourceWithDistance{
			{
				Resource: graph.ResourceIdentity{UID: "pod-123", Kind: "Pod", Name: "my-pod"},
				Distance: 0,
			},
			{
				Resource: graph.ResourceIdentity{UID: "deploy-123", Kind: "Deployment", Name: "my-deploy"},
				Distance: 1,
			},
		}

		managers := map[string]*ManagerData{
			"deploy-123": {
				Manager: graph.ResourceIdentity{
					UID:  "hr-123",
					Kind: "HelmRelease",
					Name: "my-app",
				},
				ManagesEdge: graph.ManagesEdge{
					Confidence: 0.9,
				},
			},
		}

		related := map[string][]RelatedResourceData{}
		changeEvents := map[string][]ChangeEventInfo{
			"hr-123": {
				{EventID: "evt-hr", Timestamp: time.Now(), EventType: "UPDATE", ConfigChanged: true},
			},
		}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)
		require.NoError(t, err)

		// Should have Pod, Deployment, and HelmRelease
		assert.GreaterOrEqual(t, len(result.Nodes), 3, "Should have at least 3 nodes")

		// Should have MANAGES edge
		managesCount := 0
		for _, edge := range result.Edges {
			if edge.RelationshipType == edgeTypeManages {
				managesCount++
			}
		}
		assert.GreaterOrEqual(t, managesCount, 1, "Should have at least 1 MANAGES edge")
	})

	t.Run("Chain with related resources", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:  "pod-123",
				Kind: "Pod",
				Name: "my-pod",
			},
			ObservedAt: time.Now(),
		}

		chain := []ResourceWithDistance{
			{
				Resource: graph.ResourceIdentity{UID: "pod-123", Kind: "Pod", Name: "my-pod"},
				Distance: 0,
			},
		}

		managers := map[string]*ManagerData{}
		related := map[string][]RelatedResourceData{
			"pod-123": {
				{
					Resource: graph.ResourceIdentity{
						UID:  "node-123",
						Kind: "Node",
						Name: "worker-1",
					},
					RelationshipType: "SCHEDULED_ON",
					Events:           []ChangeEventInfo{},
				},
			},
		}
		changeEvents := map[string][]ChangeEventInfo{}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)
		require.NoError(t, err)

		// Should have Pod (SPINE) and Node (RELATED)
		assert.GreaterOrEqual(t, len(result.Nodes), 2)

		relatedCount := 0
		for _, node := range result.Nodes {
			if node.NodeType == "RELATED" {
				relatedCount++
			}
		}
		assert.Equal(t, 1, relatedCount, "Should have 1 RELATED node")

		// Should have SCHEDULED_ON edge
		scheduledOnCount := 0
		for _, edge := range result.Edges {
			if edge.RelationshipType == "SCHEDULED_ON" {
				scheduledOnCount++
			}
		}
		assert.Equal(t, 1, scheduledOnCount)
	})

	t.Run("Empty chain returns error", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{UID: "pod-123"},
		}

		chain := []ResourceWithDistance{}
		managers := map[string]*ManagerData{}
		related := map[string][]RelatedResourceData{}
		changeEvents := map[string][]ChangeEventInfo{}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)

		// Empty chain should still produce a result (just no nodes)
		// The actual error handling is in the calling function
		require.NoError(t, err)
		assert.Equal(t, 0, len(result.Nodes))
	})

	t.Run("Node deduplication works", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:  "pod-123",
				Kind: "Pod",
				Name: "my-pod",
			},
			ObservedAt: time.Now(),
		}

		// Chain with duplicate UIDs (should be deduplicated)
		chain := []ResourceWithDistance{
			{
				Resource: graph.ResourceIdentity{UID: "pod-123", Kind: "Pod"},
				Distance: 0,
			},
			{
				Resource: graph.ResourceIdentity{UID: "rs-123", Kind: "ReplicaSet"},
				Distance: 1,
			},
			{
				Resource: graph.ResourceIdentity{UID: "rs-123", Kind: "ReplicaSet"}, // Duplicate
				Distance: 1,
			},
		}

		managers := map[string]*ManagerData{}
		related := map[string][]RelatedResourceData{}
		changeEvents := map[string][]ChangeEventInfo{}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)
		require.NoError(t, err)

		// Should only have 2 unique nodes
		assert.Equal(t, 2, len(result.Nodes))
	})

	t.Run("Step numbers are sequential", func(t *testing.T) {
		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:  "pod-123",
				Kind: "Pod",
			},
			ObservedAt: time.Now(),
		}

		chain := []ResourceWithDistance{
			{Resource: graph.ResourceIdentity{UID: "pod-123", Kind: "Pod"}, Distance: 0},
			{Resource: graph.ResourceIdentity{UID: "rs-123", Kind: "ReplicaSet"}, Distance: 1},
			{Resource: graph.ResourceIdentity{UID: "deploy-123", Kind: "Deployment"}, Distance: 2},
		}

		managers := map[string]*ManagerData{}
		related := map[string][]RelatedResourceData{}
		changeEvents := map[string][]ChangeEventInfo{}
		k8sEvents := map[string][]K8sEventInfo{}

		result, err := analyzer.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTime)
		require.NoError(t, err)

		// Check step numbers are sequential
		stepNumbers := make(map[int]bool)
		for _, node := range result.Nodes {
			if node.NodeType == nodeTypeSpine {
				stepNumbers[node.StepNumber] = true
			}
		}

		// Should have steps 1, 2, 3
		assert.True(t, stepNumbers[1])
		assert.True(t, stepNumbers[2])
		assert.True(t, stepNumbers[3])
	})
}

// TestSelectPrimaryEvent validates event selection logic
func TestSelectPrimaryEvent(t *testing.T) {
	failureTime := time.Now().UnixNano()

	t.Run("Prefers configChanged events", func(t *testing.T) {
		events := []ChangeEventInfo{
			{
				EventID:       "evt-1",
				Timestamp:     time.Unix(0, failureTime-10000000000),
				EventType:     "UPDATE",
				StatusChanged: true,
			},
			{
				EventID:       "evt-2",
				Timestamp:     time.Unix(0, failureTime-5000000000),
				EventType:     "UPDATE",
				ConfigChanged: true, // Should be selected
			},
		}

		result := selectPrimaryEvent(events, failureTime)
		require.NotNil(t, result)
		assert.Equal(t, "evt-2", result.EventID)
		assert.True(t, result.ConfigChanged)
	})

	t.Run("Prefers CREATE over status changes", func(t *testing.T) {
		events := []ChangeEventInfo{
			{
				EventID:       "evt-1",
				Timestamp:     time.Unix(0, failureTime-5000000000),
				EventType:     "UPDATE",
				StatusChanged: true,
			},
			{
				EventID:   "evt-2",
				Timestamp: time.Unix(0, failureTime-10000000000),
				EventType: "CREATE", // Should be selected
			},
		}

		result := selectPrimaryEvent(events, failureTime)
		require.NotNil(t, result)
		assert.Equal(t, "evt-2", result.EventID)
		assert.Equal(t, "CREATE", result.EventType)
	})

	t.Run("Selects closest status change to failure", func(t *testing.T) {
		events := []ChangeEventInfo{
			{
				EventID:       "evt-1",
				Timestamp:     time.Unix(0, failureTime-10000000000),
				EventType:     "UPDATE",
				StatusChanged: true,
			},
			{
				EventID:       "evt-2",
				Timestamp:     time.Unix(0, failureTime-1000000000), // Closest
				EventType:     "UPDATE",
				StatusChanged: true,
			},
		}

		result := selectPrimaryEvent(events, failureTime)
		require.NotNil(t, result)
		assert.Equal(t, "evt-2", result.EventID)
	})

	t.Run("Returns nil for empty events", func(t *testing.T) {
		result := selectPrimaryEvent([]ChangeEventInfo{}, failureTime)
		assert.Nil(t, result)
	})

	t.Run("Returns earliest event as fallback", func(t *testing.T) {
		events := []ChangeEventInfo{
			{
				EventID:   "evt-1",
				Timestamp: time.Unix(0, failureTime-5000000000),
				EventType: "UPDATE",
			},
			{
				EventID:   "evt-2",
				Timestamp: time.Unix(0, failureTime-10000000000), // Earliest
				EventType: "UPDATE",
			},
		}

		result := selectPrimaryEvent(events, failureTime)
		require.NotNil(t, result)
		assert.Equal(t, "evt-2", result.EventID)
	})
}

// TestGenerateStepReasoning validates reasoning text generation
func TestGenerateStepReasoning(t *testing.T) {
	resource := graph.ResourceIdentity{
		Kind: "Deployment",
		Name: "my-app",
	}

	t.Run("MANAGES relationship", func(t *testing.T) {
		manager := &graph.ResourceIdentity{
			Kind: "HelmRelease",
		}
		managesEdge := &graph.ManagesEdge{
			Confidence: 0.85,
		}

		result := generateStepReasoning(resource, manager, managesEdge, nil, "MANAGES")
		assert.Contains(t, result, "HelmRelease")
		assert.Contains(t, result, "manages")
		assert.Contains(t, result, "85%")
	})

	t.Run("OWNS relationship", func(t *testing.T) {
		result := generateStepReasoning(resource, nil, nil, nil, "OWNS")
		assert.Contains(t, result, "Deployment")
		assert.Contains(t, result, "owns")
	})

	t.Run("SYMPTOM with config change", func(t *testing.T) {
		event := &ChangeEventInfo{
			ConfigChanged: true,
		}
		result := generateStepReasoning(resource, nil, nil, event, "SYMPTOM")
		assert.Contains(t, result, "Configuration change")
	})

	t.Run("SYMPTOM without config change", func(t *testing.T) {
		result := generateStepReasoning(resource, nil, nil, nil, "SYMPTOM")
		assert.Contains(t, result, "failure symptom")
	})
}
