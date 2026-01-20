package anomaly

import (
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/stretchr/testify/assert"
)

func TestIsReplicaSetRoutineChange(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		changedFields  []string
		expectRoutine  bool
	}{
		{
			name: "ReplicaSet with only routine changes",
			kind: "ReplicaSet",
			changedFields: []string{
				"metadata.annotations.deployment.kubernetes.io/revision",
				"metadata.annotations.deployment.kubernetes.io/revision-history",
				"spec.replicas",
				"status.fullyLabeledReplicas",
				"status.replicas",
			},
			expectRoutine: true,
		},
		{
			name: "ReplicaSet with only metadata and status",
			kind: "ReplicaSet",
			changedFields: []string{
				"metadata.annotations.deployment.kubernetes.io/revision",
				"status.observedGeneration",
			},
			expectRoutine: true,
		},
		{
			name: "ReplicaSet with only spec.replicas",
			kind: "ReplicaSet",
			changedFields: []string{
				"spec.replicas",
			},
			expectRoutine: true,
		},
		{
			name: "ReplicaSet with spec.template change (meaningful)",
			kind: "ReplicaSet",
			changedFields: []string{
				"metadata.annotations.deployment.kubernetes.io/revision",
				"spec.replicas",
				"spec.template.spec.containers[0].image",
				"status.replicas",
			},
			expectRoutine: false,
		},
		{
			name: "ReplicaSet with non-deployment annotation (meaningful)",
			kind: "ReplicaSet",
			changedFields: []string{
				"metadata.annotations.custom-annotation",
				"spec.replicas",
			},
			expectRoutine: false,
		},
		{
			name: "Deployment with same changes (not routine - different kind)",
			kind: "Deployment",
			changedFields: []string{
				"metadata.annotations.deployment.kubernetes.io/revision",
				"spec.replicas",
				"status.replicas",
			},
			expectRoutine: false,
		},
		{
			name: "Pod with routine fields (not routine - different kind)",
			kind: "Pod",
			changedFields: []string{
				"status.phase",
			},
			expectRoutine: false,
		},
		{
			name: "Empty changed fields",
			kind: "ReplicaSet",
			changedFields: []string{},
			expectRoutine: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isReplicaSetRoutineChange(tt.kind, tt.changedFields)
			assert.Equal(t, tt.expectRoutine, result,
				"isReplicaSetRoutineChange(%q, %v) = %v, want %v",
				tt.kind, tt.changedFields, result, tt.expectRoutine)
		})
	}
}

func TestIsOnlyReplicaChange(t *testing.T) {
	tests := []struct {
		name          string
		changedFields []string
		expected      bool
	}{
		{
			name: "Only spec.replicas",
			changedFields: []string{
				"spec.replicas",
			},
			expected: true,
		},
		{
			name: "Only status.replicas",
			changedFields: []string{
				"status.replicas",
				"status.fullyLabeledReplicas",
			},
			expected: true,
		},
		{
			name: "Mix of replicas and other fields",
			changedFields: []string{
				"spec.replicas",
				"spec.template.spec.containers[0].image",
			},
			expected: false,
		},
		{
			name:          "Empty fields",
			changedFields: []string{},
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOnlyReplicaChange(tt.changedFields)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAreAllStatusChanges(t *testing.T) {
	tests := []struct {
		name          string
		changedFields []string
		expected      bool
	}{
		{
			name: "Only status fields",
			changedFields: []string{
				"status.replicas",
				"status.observedGeneration",
			},
			expected: true,
		},
		{
			name: "Mix of status and spec",
			changedFields: []string{
				"status.replicas",
				"spec.replicas",
			},
			expected: false,
		},
		{
			name:          "Empty fields",
			changedFields: []string{},
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := areAllStatusChanges(tt.changedFields)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestChangeAnomalyDetector_EmptyDiff(t *testing.T) {
	detector := NewChangeAnomalyDetector()

	input := DetectorInput{
		Node: &analysis.GraphNode{
			ID: "node-helm-release-uid",
			Resource: analysis.SymptomResource{
				UID:       "helm-release-uid",
				Kind:      "HelmRelease",
				Namespace: "external-secrets",
				Name:      "external-secrets",
			},
		},
		AllEvents: []analysis.ChangeEventInfo{
			{
				EventID:       "event-1",
				Timestamp:     time.Now(),
				EventType:     "UPDATE",
				ConfigChanged: true,
				Diff:          []analysis.EventDiff{}, // Empty diff (from database query)
			},
		},
		TimeWindow: TimeWindow{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now().Add(1 * time.Hour),
		},
	}

	anomalies := detector.Detect(input)

	// Should still report the anomaly even without diff information
	assert.Len(t, anomalies, 1, "Should detect config change even without diff")
	assert.Equal(t, "HelmReleaseUpdated", anomalies[0].Type)
	assert.Equal(t, SeverityHigh, anomalies[0].Severity)

	// Details should not have changed_fields key
	details := anomalies[0].Details
	_, hasChangedFields := details["changed_fields"]
	assert.False(t, hasChangedFields, "Should not include changed_fields when diff is empty")
	assert.Equal(t, "UPDATE", details["event_type"])
}

func TestChangeAnomalyDetector_WithDiff(t *testing.T) {
	detector := NewChangeAnomalyDetector()

	input := DetectorInput{
		Node: &analysis.GraphNode{
			ID: "node-deployment-uid",
			Resource: analysis.SymptomResource{
				UID:       "deployment-uid",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "my-app",
			},
		},
		AllEvents: []analysis.ChangeEventInfo{
			{
				EventID:       "event-2",
				Timestamp:     time.Now(),
				EventType:     "UPDATE",
				ConfigChanged: true,
				Diff: []analysis.EventDiff{
					{Path: "spec.template.spec.containers[0].image", Op: "replace"},
					{Path: "spec.replicas", Op: "replace"},
				},
			},
		},
		TimeWindow: TimeWindow{
			Start: time.Now().Add(-1 * time.Hour),
			End:   time.Now().Add(1 * time.Hour),
		},
	}

	anomalies := detector.Detect(input)

	// Should report anomaly with changed_fields
	assert.Len(t, anomalies, 2, "Should detect config change and specific image change")

	// First anomaly is the general config change
	assert.Equal(t, "WorkloadSpecModified", anomalies[0].Type)
	details := anomalies[0].Details
	changedFields, ok := details["changed_fields"].([]string)
	assert.True(t, ok, "Should have changed_fields")
	assert.Contains(t, changedFields, "spec.template.spec.containers[0].image")
	assert.Contains(t, changedFields, "spec.replicas")
}
