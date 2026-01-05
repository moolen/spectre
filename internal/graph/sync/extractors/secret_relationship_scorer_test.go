package extractors

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// mockLookup is a minimal ResourceLookup mock for testing
type mockLookup struct{}

func (m *mockLookup) FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (m *mockLookup) FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (m *mockLookup) FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error) {
	return nil, nil
}

func (m *mockLookup) QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	return nil, nil
}

func TestSecretRelationshipScorer_NameMatch(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		NameMatchWeight:  0.9,
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "cert-123",
			Name:      "my-certificate",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "my-certificate", // Exact match
		Namespace: "default",
		LastSeen:  sourceEvent.Timestamp,
	}

	sourceData := map[string]interface{}{}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		sourceData,
		secret,
		"my-certificate",
	)

	if confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %.2f", confidence)
	}

	if len(evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(evidence))
	}

	if evidence[0].Type != graph.EvidenceTypeLabel {
		t.Errorf("Expected EvidenceTypeLabel, got %v", evidence[0].Type)
	}
}

func TestSecretRelationshipScorer_NamePatternMatch(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		NameMatchWeight:  0.5,
		NamePatterns:     []string{"%s-tls"},
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "cert-123",
			Name:      "my-certificate",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "my-certificate-tls", // Pattern match: %s-tls
		Namespace: "default",
		LastSeen:  sourceEvent.Timestamp,
	}

	sourceData := map[string]interface{}{}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		sourceData,
		secret,
		"my-certificate", // targetSecretName doesn't match, but pattern does
	)

	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5, got %.2f", confidence)
	}

	if len(evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(evidence))
	}

	if evidence[0].Type != graph.EvidenceTypeLabel {
		t.Errorf("Expected EvidenceTypeLabel, got %v", evidence[0].Type)
	}
}

func TestSecretRelationshipScorer_AnnotationMatch(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		AnnotationMatchWeight: 0.9,
		CheckAnnotations:      true,
		AnnotationKey:         "cert-manager.io/certificate-name",
		TemporalWindowMs:      60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "cert-123",
			Name:      "my-certificate",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "tls-secret",
		Namespace: "default",
		LastSeen:  sourceEvent.Timestamp,
		Labels: map[string]string{
			"cert-manager.io/certificate-name": "my-certificate",
		},
	}

	sourceData := map[string]interface{}{}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		sourceData,
		secret,
		"tls-secret",
	)

	if confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %.2f", confidence)
	}

	if len(evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(evidence))
	}

	if evidence[0].Type != graph.EvidenceTypeAnnotation {
		t.Errorf("Expected EvidenceTypeAnnotation, got %v", evidence[0].Type)
	}
}

func TestSecretRelationshipScorer_LabelMatch(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		LabelMatchWeight: 0.5,
		CheckLabels:      true,
		LabelKey:         "external-secrets.io/name",
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "es-123",
			Name:      "my-external-secret",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "my-secret",
		Namespace: "default",
		LastSeen:  sourceEvent.Timestamp,
		Labels: map[string]string{
			"external-secrets.io/name": "my-external-secret",
		},
	}

	sourceData := map[string]interface{}{}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		sourceData,
		secret,
		"my-secret",
	)

	if confidence != 0.5 {
		t.Errorf("Expected confidence 0.5, got %.2f", confidence)
	}

	if len(evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(evidence))
	}

	if evidence[0].Type != graph.EvidenceTypeLabel {
		t.Errorf("Expected EvidenceTypeLabel, got %v", evidence[0].Type)
	}
}

func TestSecretRelationshipScorer_TemporalProximity(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		TemporalWeight:      0.8,
		TemporalWindowMs:    60000, // 60 seconds
		CheckReadyCondition: false, // Don't gate on Ready
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	baseTime := time.Now().UnixNano()

	tests := []struct {
		name              string
		secretLastSeenLag int64 // milliseconds after sourceEvent
		expectedMin       float64
		expectedMax       float64
	}{
		{
			name:              "immediate (0ms lag)",
			secretLastSeenLag: 0,
			expectedMin:       0.8, // Full weight
			expectedMax:       0.8,
		},
		{
			name:              "half window (30s lag)",
			secretLastSeenLag: 30000,
			expectedMin:       0.39, // 0.8 * 0.5 = 0.4 (with rounding tolerance)
			expectedMax:       0.41,
		},
		{
			name:              "at window boundary (60s lag)",
			secretLastSeenLag: 60000,
			expectedMin:       0.0, // 0.8 * 0.0 = 0.0
			expectedMax:       0.01,
		},
		{
			name:              "outside window (120s lag)",
			secretLastSeenLag: 120000,
			expectedMin:       0.0, // No evidence
			expectedMax:       0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceEvent := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "source-123",
					Name:      "source",
					Namespace: "default",
				},
				Timestamp: baseTime,
			}

			secret := &graph.ResourceIdentity{
				UID:       "secret-456",
				Name:      "secret",
				Namespace: "default",
				LastSeen:  baseTime + (tt.secretLastSeenLag * 1_000_000), // Convert ms to ns
			}

			confidence, evidence := scorer.ScoreRelationship(
				context.Background(),
				sourceEvent,
				map[string]interface{}{},
				secret,
				"secret",
			)

			if confidence < tt.expectedMin || confidence > tt.expectedMax {
				t.Errorf("Expected confidence between %.2f and %.2f, got %.2f", tt.expectedMin, tt.expectedMax, confidence)
			}

			if tt.expectedMin > 0 && len(evidence) != 1 {
				t.Errorf("Expected 1 evidence item, got %d", len(evidence))
			}

			if tt.expectedMin > 0 && evidence[0].Type != graph.EvidenceTypeTemporal {
				t.Errorf("Expected EvidenceTypeTemporal, got %v", evidence[0].Type)
			}
		})
	}
}

func TestSecretRelationshipScorer_TemporalWithReadyCondition(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		TemporalWeight:      0.8,
		TemporalWindowMs:    60000,
		CheckReadyCondition: true, // Gate on Ready condition
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	baseTime := time.Now().UnixNano()

	t.Run("Ready=True with temporal match", func(t *testing.T) {
		sourceEvent := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "cert-123",
				Name:      "my-certificate",
				Namespace: "default",
			},
			Timestamp: baseTime,
		}

		secret := &graph.ResourceIdentity{
			UID:       "secret-456",
			Name:      "my-secret",
			Namespace: "default",
			LastSeen:  baseTime + (10 * 1_000_000_000), // 10 seconds later
		}

		sourceData := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		confidence, evidence := scorer.ScoreRelationship(
			context.Background(),
			sourceEvent,
			sourceData,
			secret,
			"my-secret",
		)

		if confidence < 0.65 || confidence > 0.75 {
			t.Errorf("Expected confidence ~0.7 (temporal evidence), got %.2f", confidence)
		}

		if len(evidence) != 1 {
			t.Errorf("Expected 1 evidence item (temporal), got %d", len(evidence))
		}
	})

	t.Run("Ready=False blocks temporal check", func(t *testing.T) {
		sourceEvent := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "cert-123",
				Name:      "my-certificate",
				Namespace: "default",
			},
			Timestamp: baseTime,
		}

		secret := &graph.ResourceIdentity{
			UID:       "secret-456",
			Name:      "my-secret",
			Namespace: "default",
			LastSeen:  baseTime + (10 * 1_000_000_000), // 10 seconds later
		}

		sourceData := map[string]interface{}{
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "False",
					},
				},
			},
		}

		confidence, evidence := scorer.ScoreRelationship(
			context.Background(),
			sourceEvent,
			sourceData,
			secret,
			"my-secret",
		)

		if confidence != 0.0 {
			t.Errorf("Expected confidence 0.0 (no Ready condition), got %.2f", confidence)
		}

		if len(evidence) != 0 {
			t.Errorf("Expected 0 evidence items, got %d", len(evidence))
		}
	})
}

func TestSecretRelationshipScorer_NamespaceMatch(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		NamespaceWeight:  0.3,
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "source-123",
			Name:      "source",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "secret",
		Namespace: "default", // Same namespace
		LastSeen:  sourceEvent.Timestamp,
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		map[string]interface{}{},
		secret,
		"secret",
	)

	if confidence != 0.3 {
		t.Errorf("Expected confidence 0.3, got %.2f", confidence)
	}

	if len(evidence) != 1 {
		t.Errorf("Expected 1 evidence item, got %d", len(evidence))
	}

	if evidence[0].Type != graph.EvidenceTypeNamespace {
		t.Errorf("Expected EvidenceTypeNamespace, got %v", evidence[0].Type)
	}
}

func TestSecretRelationshipScorer_MultipleEvidence(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		NameMatchWeight:  0.9,
		TemporalWeight:   0.7,
		NamespaceWeight:  0.3,
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	baseTime := time.Now().UnixNano()

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "source-123",
			Name:      "my-resource",
			Namespace: "default",
		},
		Timestamp: baseTime,
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "my-resource", // Name match (0.9)
		Namespace: "default",     // Namespace match (0.3)
		LastSeen:  baseTime,      // Temporal match (0.7 * 1.0 = 0.7)
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		map[string]interface{}{},
		secret,
		"my-resource",
	)

	// Expected: 0.9 + 0.7 + 0.3 = 1.9, capped at 1.0
	if confidence != 1.0 {
		t.Errorf("Expected confidence capped at 1.0, got %.2f", confidence)
	}

	if len(evidence) != 3 {
		t.Errorf("Expected 3 evidence items, got %d", len(evidence))
	}
}

func TestSecretRelationshipScorer_ZeroConfidence(t *testing.T) {
	config := SecretRelationshipScorerConfig{
		NameMatchWeight:  0.9,
		TemporalWeight:   0.7,
		NamespaceWeight:  0.3,
		TemporalWindowMs: 60000,
	}

	scorer := NewSecretRelationshipScorer(config, &mockLookup{}, func(string, ...interface{}) {})

	sourceEvent := models.Event{
		Resource: models.ResourceMetadata{
			UID:       "source-123",
			Name:      "my-resource",
			Namespace: "default",
		},
		Timestamp: time.Now().UnixNano(),
	}

	secret := &graph.ResourceIdentity{
		UID:       "secret-456",
		Name:      "different-name", // No name match
		Namespace: "other-namespace", // No namespace match
		LastSeen:  sourceEvent.Timestamp - (120 * 1_000_000_000), // 120s before (outside window)
	}

	confidence, evidence := scorer.ScoreRelationship(
		context.Background(),
		sourceEvent,
		map[string]interface{}{},
		secret,
		"my-resource",
	)

	if confidence != 0.0 {
		t.Errorf("Expected confidence 0.0, got %.2f", confidence)
	}

	if len(evidence) != 0 {
		t.Errorf("Expected 0 evidence items, got %d", len(evidence))
	}
}

func TestCreateCertificateSecretScorerConfig(t *testing.T) {
	config := CreateCertificateSecretScorerConfig()

	if config.OwnerReferenceWeight != 1.0 {
		t.Errorf("Expected OwnerReferenceWeight 1.0, got %.2f", config.OwnerReferenceWeight)
	}

	if config.NameMatchWeight != 0.5 {
		t.Errorf("Expected NameMatchWeight 0.5, got %.2f", config.NameMatchWeight)
	}

	if config.AnnotationMatchWeight != 0.9 {
		t.Errorf("Expected AnnotationMatchWeight 0.9, got %.2f", config.AnnotationMatchWeight)
	}

	if config.TemporalWeight != 0.8 {
		t.Errorf("Expected TemporalWeight 0.8, got %.2f", config.TemporalWeight)
	}

	if config.NamespaceWeight != 0.3 {
		t.Errorf("Expected NamespaceWeight 0.3, got %.2f", config.NamespaceWeight)
	}

	if config.TemporalWindowMs != 60000 {
		t.Errorf("Expected TemporalWindowMs 60000, got %d", config.TemporalWindowMs)
	}

	if config.AnnotationKey != "cert-manager.io/certificate-name" {
		t.Errorf("Expected AnnotationKey 'cert-manager.io/certificate-name', got '%s'", config.AnnotationKey)
	}

	if len(config.NamePatterns) != 1 || config.NamePatterns[0] != "%s-tls" {
		t.Errorf("Expected NamePatterns ['%%s-tls'], got %v", config.NamePatterns)
	}
}

func TestCreateExternalSecretScorerConfig(t *testing.T) {
	config := CreateExternalSecretScorerConfig()

	if config.OwnerReferenceWeight != 1.0 {
		t.Errorf("Expected OwnerReferenceWeight 1.0, got %.2f", config.OwnerReferenceWeight)
	}

	if config.NameMatchWeight != 0.9 {
		t.Errorf("Expected NameMatchWeight 0.9, got %.2f", config.NameMatchWeight)
	}

	if config.LabelMatchWeight != 0.5 {
		t.Errorf("Expected LabelMatchWeight 0.5, got %.2f", config.LabelMatchWeight)
	}

	if config.TemporalWeight != 0.7 {
		t.Errorf("Expected TemporalWeight 0.7, got %.2f", config.TemporalWeight)
	}

	if config.NamespaceWeight != 0.3 {
		t.Errorf("Expected NamespaceWeight 0.3, got %.2f", config.NamespaceWeight)
	}

	if config.TemporalWindowMs != 120000 {
		t.Errorf("Expected TemporalWindowMs 120000, got %d", config.TemporalWindowMs)
	}

	if config.LabelKey != "external-secrets.io/name" {
		t.Errorf("Expected LabelKey 'external-secrets.io/name', got '%s'", config.LabelKey)
	}
}
