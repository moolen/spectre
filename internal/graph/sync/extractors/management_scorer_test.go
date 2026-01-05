package extractors

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test configs for different GitOps tools
func fluxHelmReleaseConfig() ManagementScorerConfig {
	return ManagementScorerConfig{
		LabelTemplates: map[string]string{
			"name":      "helm.toolkit.fluxcd.io/name",
			"namespace": "helm.toolkit.fluxcd.io/namespace",
		},
		NamePrefixWeight:     0.4,
		NamespaceWeight:      0.1,
		TemporalWeight:       0.3,
		ReconcileWeight:      0.2,
		TemporalWindowMs:     30000,
		CheckReconcileEvents: true,
	}
}

func fluxKustomizationConfig() ManagementScorerConfig {
	return ManagementScorerConfig{
		LabelTemplates: map[string]string{
			"name":      "kustomize.toolkit.fluxcd.io/name",
			"namespace": "kustomize.toolkit.fluxcd.io/namespace",
		},
		NamePrefixWeight:     0.4,
		NamespaceWeight:      0.3,
		TemporalWeight:       0.3,
		TemporalWindowMs:     30000,
		CheckReconcileEvents: false,
	}
}

func argoCDApplicationConfig() ManagementScorerConfig {
	return ManagementScorerConfig{
		LabelTemplates: map[string]string{
			"name": "argocd.argoproj.io/instance",
		},
		NamePrefixWeight:     0.4,
		NamespaceWeight:      0.3,
		TemporalWeight:       0.3,
		TemporalWindowMs:     120000,
		CheckReconcileEvents: false,
	}
}

func TestManagementScorer_PerfectLabelMatch(t *testing.T) {
	tests := []struct {
		name       string
		config     ManagementScorerConfig
		candidate  *graph.ResourceIdentity
		wantScore  float64
		wantReason string
	}{
		{
			name:   "FluxHelmRelease perfect match",
			config: fluxHelmReleaseConfig(),
			candidate: &graph.ResourceIdentity{
				UID:       "deployment-1",
				Name:      "frontend-deployment",
				Namespace: "prod",
				Labels: map[string]string{
					"helm.toolkit.fluxcd.io/name":      "frontend",
					"helm.toolkit.fluxcd.io/namespace": "prod",
				},
			},
			wantScore:  1.0,
			wantReason: "Labels match",
		},
		{
			name:   "FluxKustomization perfect match",
			config: fluxKustomizationConfig(),
			candidate: &graph.ResourceIdentity{
				UID:       "service-1",
				Name:      "backend-service",
				Namespace: "staging",
				Labels: map[string]string{
					"kustomize.toolkit.fluxcd.io/name":      "backend",
					"kustomize.toolkit.fluxcd.io/namespace": "staging",
				},
			},
			wantScore:  1.0,
			wantReason: "Labels match",
		},
		{
			name:   "ArgoCDApplication perfect match (name only)",
			config: argoCDApplicationConfig(),
			candidate: &graph.ResourceIdentity{
				UID:       "ingress-1",
				Name:      "api-ingress",
				Namespace: "default",
				Labels: map[string]string{
					"argocd.argoproj.io/instance": "api",
				},
			},
			wantScore:  1.0,
			wantReason: "Label matches", // Singular because only one label (name, no namespace)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := newMockResourceLookup()
			lookup.addResource(tt.candidate.UID, tt.candidate)

			scorer := NewManagementScorer(tt.config, lookup, func(format string, args ...interface{}) {})

			managerEvent := models.Event{
				Resource:  models.ResourceMetadata{UID: "manager-1"},
				Timestamp: time.Now().UnixNano(),
			}

			managerName := "frontend"
			if tt.name == "FluxKustomization perfect match" {
				managerName = "backend"
			} else if tt.name == "ArgoCDApplication perfect match (name only)" {
				managerName = "api"
			}

			confidence, evidence := scorer.ScoreRelationship(
				context.Background(),
				managerEvent,
				tt.candidate.UID,
				managerName,
				tt.candidate.Namespace,
				tt.candidate.Namespace,
			)

			assert.Equal(t, tt.wantScore, confidence)
			require.Len(t, evidence, 1)
			assert.Contains(t, evidence[0].Value, tt.wantReason)
			assert.Equal(t, graph.EvidenceTypeLabel, evidence[0].Type)
		})
	}
}

func TestManagementScorer_NamePrefixMatch(t *testing.T) {
	config := fluxKustomizationConfig()
	lookup := newMockResourceLookup()

	candidate := &graph.ResourceIdentity{
		UID:       "deployment-1",
		Name:      "frontend-deployment",
		Namespace: "prod",
		FirstSeen: time.Now().UnixNano(),
	}
	lookup.addResource(candidate.UID, candidate)

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "kustomization-1"},
		Timestamp: candidate.FirstSeen,
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		candidate.UID,
		"frontend",
		"prod",
		"prod",
	)

	// Should have name prefix (0.4) + namespace (0.3) + temporal (0.3) = 1.0
	assert.Equal(t, 1.0, confidence)
	assert.Len(t, evidence, 3)

	// Verify evidence types
	evidenceTypes := []graph.EvidenceType{evidence[0].Type, evidence[1].Type, evidence[2].Type}
	assert.Contains(t, evidenceTypes, graph.EvidenceTypeLabel)      // name prefix
	assert.Contains(t, evidenceTypes, graph.EvidenceTypeNamespace)  // namespace
	assert.Contains(t, evidenceTypes, graph.EvidenceTypeTemporal)   // temporal
}

func TestManagementScorer_TemporalProximity(t *testing.T) {
	config := fluxKustomizationConfig()
	lookup := newMockResourceLookup()

	baseTime := time.Now().UnixNano()

	tests := []struct {
		name          string
		managerTime   int64
		resourceTime  int64
		wantTemporal  bool
		minConfidence float64
	}{
		{
			name:          "Immediate creation (0ms lag)",
			managerTime:   baseTime,
			resourceTime:  baseTime,
			wantTemporal:  true,
			minConfidence: 0.6, // namespace (0.3) + temporal (0.3) = 0.6
		},
		{
			name:          "Created 10s later",
			managerTime:   baseTime,
			resourceTime:  baseTime + (10 * 1000 * 1_000_000), // +10s in nanos
			wantTemporal:  true,
			minConfidence: 0.5,
		},
		{
			name:          "Created 29s later (within window)",
			managerTime:   baseTime,
			resourceTime:  baseTime + (29 * 1000 * 1_000_000),
			wantTemporal:  true,
			minConfidence: 0.3,
		},
		{
			name:          "Created 31s later (outside window)",
			managerTime:   baseTime,
			resourceTime:  baseTime + (31 * 1000 * 1_000_000),
			wantTemporal:  false,
			minConfidence: 0.3, // only namespace match
		},
		{
			name:          "Created before manager (negative lag)",
			managerTime:   baseTime,
			resourceTime:  baseTime - (5 * 1000 * 1_000_000),
			wantTemporal:  false,
			minConfidence: 0.3, // only namespace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := &graph.ResourceIdentity{
				UID:       "resource-1",
				Name:      "other-deployment",
				Namespace: "prod",
				FirstSeen: tt.resourceTime,
			}
			lookup.addResource(candidate.UID, candidate)

			scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

			managerEvent := models.Event{
				Resource:  models.ResourceMetadata{UID: "manager-1"},
				Timestamp: tt.managerTime,
			}

			confidence, evidence := scorer.ScoreRelationship(
				context.Background(),
				managerEvent,
				candidate.UID,
				"frontend",
				"prod",
				"prod",
			)

			assert.GreaterOrEqual(t, confidence, tt.minConfidence)

			// Check for temporal evidence
			hasTemporal := false
			for _, ev := range evidence {
				if ev.Type == graph.EvidenceTypeTemporal {
					hasTemporal = true
					break
				}
			}
			assert.Equal(t, tt.wantTemporal, hasTemporal)
		})
	}
}

func TestManagementScorer_ReconcileEvents(t *testing.T) {
	config := fluxHelmReleaseConfig() // Has CheckReconcileEvents=true
	lookup := newMockResourceLookup()

	baseTime := time.Now().UnixNano()

	candidate := &graph.ResourceIdentity{
		UID:       "deployment-1",
		Name:      "other-deployment",
		Namespace: "prod",
		FirstSeen: baseTime,
	}
	lookup.addResource(candidate.UID, candidate)

	// Add reconcile event
	lookup.addEvents("manager-1", []graph.ChangeEvent{
		{
			ID:        "event-1",
			Status:    "Ready",
			EventType: "UPDATE",
			Timestamp: baseTime,
		},
	})

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "manager-1"},
		Timestamp: baseTime,
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		candidate.UID,
		"frontend",
		"prod",
		"prod",
	)

	// Should have namespace (0.1) + temporal (0.3) + reconcile (0.2) = 0.6
	assert.InDelta(t, 0.6, confidence, 0.01)

	// Check for reconcile evidence
	hasReconcile := false
	for _, ev := range evidence {
		if ev.Type == graph.EvidenceTypeReconcile {
			hasReconcile = true
			break
		}
	}
	assert.True(t, hasReconcile, "Expected reconcile evidence")
}

func TestManagementScorer_NoReconcileEventsWhenDisabled(t *testing.T) {
	config := fluxKustomizationConfig() // Has CheckReconcileEvents=false
	lookup := newMockResourceLookup()

	baseTime := time.Now().UnixNano()

	candidate := &graph.ResourceIdentity{
		UID:       "deployment-1",
		Name:      "other-deployment",
		Namespace: "prod",
		FirstSeen: baseTime,
	}
	lookup.addResource(candidate.UID, candidate)

	// Add reconcile event (should be ignored)
	lookup.addEvents("manager-1", []graph.ChangeEvent{
		{
			ID:        "event-1",
			Status:    "Ready",
			EventType: "UPDATE",
			Timestamp: baseTime,
		},
	})

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "manager-1"},
		Timestamp: baseTime,
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		candidate.UID,
		"frontend",
		"prod",
		"prod",
	)

	// Should NOT have reconcile evidence
	for _, ev := range evidence {
		assert.NotEqual(t, graph.EvidenceTypeReconcile, ev.Type, "Reconcile evidence should not be present")
	}

	// Confidence should be namespace (0.3) + temporal (0.3) = 0.6
	assert.InDelta(t, 0.6, confidence, 0.01)
}

func TestManagementScorer_DifferentNamespaceWeights(t *testing.T) {
	baseTime := time.Now().UnixNano()

	tests := []struct {
		name       string
		config     ManagementScorerConfig
		wantWeight float64
	}{
		{
			name:       "FluxHelmRelease (0.1 namespace weight)",
			config:     fluxHelmReleaseConfig(),
			wantWeight: 0.1,
		},
		{
			name:       "FluxKustomization (0.3 namespace weight)",
			config:     fluxKustomizationConfig(),
			wantWeight: 0.3,
		},
		{
			name:       "ArgoCDApplication (0.3 namespace weight)",
			config:     argoCDApplicationConfig(),
			wantWeight: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := newMockResourceLookup()

			candidate := &graph.ResourceIdentity{
				UID:       "resource-1",
				Name:      "other-name",
				Namespace: "prod",
				FirstSeen: baseTime + (130 * 1000 * 1_000_000), // 130s - outside all temporal windows (max is 120s for ArgoCD)
			}
			lookup.addResource(candidate.UID, candidate)

			scorer := NewManagementScorer(tt.config, lookup, func(format string, args ...interface{}) {})

			managerEvent := models.Event{
				Resource:  models.ResourceMetadata{UID: "manager-1"},
				Timestamp: baseTime,
			}

			confidence, evidence := scorer.ScoreRelationship(
				context.Background(),
				managerEvent,
				candidate.UID,
				"frontend",
				"prod",
				"prod",
			)

			// Only namespace should match
			assert.Equal(t, tt.wantWeight, confidence)
			require.Len(t, evidence, 1)
			assert.Equal(t, graph.EvidenceTypeNamespace, evidence[0].Type)
		})
	}
}

func TestManagementScorer_DifferentTemporalWindows(t *testing.T) {
	baseTime := time.Now().UnixNano()
	lagMs := int64(60000) // 60 seconds

	tests := []struct {
		name              string
		config            ManagementScorerConfig
		wantTemporalMatch bool
	}{
		{
			name:              "FluxHelmRelease (30s window) - no match",
			config:            fluxHelmReleaseConfig(),
			wantTemporalMatch: false, // 60s > 30s
		},
		{
			name:              "FluxKustomization (30s window) - no match",
			config:            fluxKustomizationConfig(),
			wantTemporalMatch: false,
		},
		{
			name:              "ArgoCDApplication (120s window) - match",
			config:            argoCDApplicationConfig(),
			wantTemporalMatch: true, // 60s < 120s
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := newMockResourceLookup()

			candidate := &graph.ResourceIdentity{
				UID:       "resource-1",
				Name:      "other-name",
				Namespace: "other-ns",
				FirstSeen: baseTime + (lagMs * 1_000_000),
			}
			lookup.addResource(candidate.UID, candidate)

			scorer := NewManagementScorer(tt.config, lookup, func(format string, args ...interface{}) {})

			managerEvent := models.Event{
				Resource:  models.ResourceMetadata{UID: "manager-1"},
				Timestamp: baseTime,
			}

			_, evidence := scorer.ScoreRelationship(
				context.Background(),
				managerEvent,
				candidate.UID,
				"frontend",
				"prod",
				"other-ns",
			)

			hasTemporal := false
			for _, ev := range evidence {
				if ev.Type == graph.EvidenceTypeTemporal {
					hasTemporal = true
					break
				}
			}

			assert.Equal(t, tt.wantTemporalMatch, hasTemporal)
		})
	}
}

func TestManagementScorer_NoLabels(t *testing.T) {
	config := fluxHelmReleaseConfig()
	lookup := newMockResourceLookup()

	candidate := &graph.ResourceIdentity{
		UID:       "resource-1",
		Name:      "frontend-deployment",
		Namespace: "prod",
		FirstSeen: time.Now().UnixNano(),
		Labels:    nil, // No labels
	}
	lookup.addResource(candidate.UID, candidate)

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "manager-1"},
		Timestamp: candidate.FirstSeen,
	}

	confidence, _ := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		candidate.UID,
		"frontend",
		"prod",
		"prod",
	)

	// Should fall back to heuristic (not 1.0)
	assert.Less(t, confidence, 1.0)
	assert.Greater(t, confidence, 0.0)
}

func TestManagementScorer_PartialLabelMatch(t *testing.T) {
	config := fluxHelmReleaseConfig()
	lookup := newMockResourceLookup()

	candidate := &graph.ResourceIdentity{
		UID:       "resource-1",
		Name:      "frontend-deployment",
		Namespace: "prod",
		FirstSeen: time.Now().UnixNano(),
		Labels: map[string]string{
			"helm.toolkit.fluxcd.io/name": "frontend", // Name matches, but no namespace label
		},
	}
	lookup.addResource(candidate.UID, candidate)

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "manager-1"},
		Timestamp: candidate.FirstSeen,
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		candidate.UID,
		"frontend",
		"prod",
		"prod",
	)

	// Should fall back to heuristic (not 1.0)
	assert.Less(t, confidence, 1.0)

	// Should not have perfect label match evidence
	for _, ev := range evidence {
		assert.NotContains(t, ev.Value, "Labels match")
	}
}

func TestManagementScorer_LookupFailure(t *testing.T) {
	config := fluxHelmReleaseConfig()
	lookup := newMockResourceLookup()
	// Don't add resource to lookup

	scorer := NewManagementScorer(config, lookup, func(format string, args ...interface{}) {})

	managerEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "manager-1"},
		Timestamp: time.Now().UnixNano(),
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		managerEvent,
		"nonexistent-uid",
		"frontend",
		"prod",
		"prod",
	)

	assert.Equal(t, 0.0, confidence)
	assert.Empty(t, evidence)
}
