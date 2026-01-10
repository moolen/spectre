package anomaly

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDetector creates a detector with a logger for testing
func newTestDetector() *AnomalyDetector {
	return &AnomalyDetector{
		logger: logging.GetLogger("anomaly.detector.test"),
	}
}

func TestDetectSecretMissingAnomalies(t *testing.T) {
	detector := newTestDetector()

	timeWindow := TimeWindow{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	tests := []struct {
		name           string
		podNode        *analysis.GraphNode
		nodeByID       map[string]*analysis.GraphNode
		edgesBySource  map[string][]analysis.GraphEdge
		expectedCount  int
		expectedType   string
		expectedFields map[string]interface{}
	}{
		{
			name: "Pod with MOUNTS edge to non-existent Secret",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
				// Note: secret-456 is NOT in nodeByID - this triggers SecretMissing
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "secret-456", // This target doesn't exist in nodeByID
						RelationshipType: "MOUNTS",
					},
				},
			},
			expectedCount: 1,
			expectedType:  "SecretMissing",
			expectedFields: map[string]interface{}{
				"target_id": "secret-456",
				"reason":    "referenced_resource_not_found",
			},
		},
		{
			name: "Pod with MOUNTS edge to existing Secret - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
				"secret-456": { // Secret exists
					ID: "secret-456",
					Resource: analysis.SymptomResource{
						UID:       "secret-456",
						Kind:      "Secret",
						Namespace: "default",
						Name:      "my-secret",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "secret-456",
						RelationshipType: "MOUNTS",
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Pod with non-MOUNTS edge - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "service-789", // Non-existent but not MOUNTS
						RelationshipType: "USES_SERVICE",
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Pod with no edges - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{},
			expectedCount: 0,
		},
		{
			name: "Pod with multiple missing MOUNTS targets",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "secret-missing-1",
						RelationshipType: "MOUNTS",
					},
					{
						From:             "pod-123",
						To:               "configmap-missing-2",
						RelationshipType: "MOUNTS",
					},
				},
			},
			expectedCount: 2, // Two missing resources
			expectedType:  "SecretMissing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.detectSecretMissingAnomalies(
				tt.podNode,
				tt.nodeByID,
				tt.edgesBySource,
				timeWindow,
			)

			assert.Len(t, anomalies, tt.expectedCount, "Expected %d anomalies", tt.expectedCount)

			if tt.expectedCount > 0 && len(anomalies) > 0 {
				anomaly := anomalies[0]
				assert.Equal(t, tt.expectedType, anomaly.Type)
				assert.Equal(t, CategoryState, anomaly.Category)
				assert.Equal(t, SeverityCritical, anomaly.Severity)
				assert.Equal(t, "Pod", anomaly.Node.Kind)

				// Check expected fields in details
				for key, expectedValue := range tt.expectedFields {
					assert.Equal(t, expectedValue, anomaly.Details[key],
						"Details[%s] should be %v", key, expectedValue)
				}
			}
		})
	}
}

func TestDetectCertificateExpiredAnomalies(t *testing.T) {
	detector := newTestDetector()

	timeWindow := TimeWindow{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	tests := []struct {
		name          string
		certNode      *analysis.GraphNode
		expectedCount int
		expectedType  string
	}{
		{
			name: "Expired certificate",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"status": map[string]interface{}{
								"notAfter": time.Now().Add(-24 * time.Hour).Format(time.RFC3339), // Expired yesterday
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedType:  "CertExpired",
		},
		{
			name: "Valid certificate (not expired)",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"status": map[string]interface{}{
								"notAfter": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339), // Expires in 30 days
							},
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Certificate without notAfter field",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"status":     map[string]interface{}{
								// No notAfter field
							},
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Certificate without status",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							// No status field
						},
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Certificate with no events",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{},
			},
			expectedCount: 0,
		},
		{
			name: "Certificate with Data field instead of FullSnapshot",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						Data: func() json.RawMessage {
							data, _ := json.Marshal(map[string]interface{}{
								"apiVersion": "cert-manager.io/v1",
								"kind":       "Certificate",
								"status": map[string]interface{}{
									"notAfter": time.Now().Add(-1 * time.Hour).Format(time.RFC3339), // Expired 1 hour ago
								},
							})
							return data
						}(),
					},
				},
			},
			expectedCount: 1,
			expectedType:  "CertExpired",
		},
		{
			name: "Certificate just expired (boundary)",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				Resource: analysis.SymptomResource{
					UID:       "cert-123",
					Kind:      "Certificate",
					Namespace: "default",
					Name:      "my-cert",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: time.Now(),
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
							"kind":       "Certificate",
							"status": map[string]interface{}{
								"notAfter": time.Now().Add(-1 * time.Second).Format(time.RFC3339), // Expired 1 second ago
							},
						},
					},
				},
			},
			expectedCount: 1,
			expectedType:  "CertExpired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.detectCertificateExpiredAnomalies(tt.certNode, timeWindow)

			assert.Len(t, anomalies, tt.expectedCount, "Expected %d anomalies", tt.expectedCount)

			if tt.expectedCount > 0 && len(anomalies) > 0 {
				anomaly := anomalies[0]
				assert.Equal(t, tt.expectedType, anomaly.Type)
				assert.Equal(t, CategoryState, anomaly.Category)
				assert.Equal(t, SeverityCritical, anomaly.Severity)
				assert.Equal(t, "Certificate", anomaly.Node.Kind)
				assert.Contains(t, anomaly.Summary, "expired")
				assert.NotEmpty(t, anomaly.Details["not_after"])
				assert.NotEmpty(t, anomaly.Details["expired_for"])
			}
		})
	}
}

func TestIsCertManagerCertificate(t *testing.T) {
	detector := newTestDetector()

	tests := []struct {
		name     string
		certNode *analysis.GraphNode
		expected bool
	}{
		{
			name: "cert-manager.io/v1",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "cert-manager.io/v1alpha2",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1alpha2",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "cert-manager.io/v1alpha3",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1alpha3",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "cert-manager.io/v1beta1",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"apiVersion": "cert-manager.io/v1beta1",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Non-cert-manager Certificate CRD",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"apiVersion": "networking.k8s.io/v1",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "No apiVersion",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"kind": "Certificate",
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "No events",
			certNode: &analysis.GraphNode{
				ID:        "cert-123",
				AllEvents: []analysis.ChangeEventInfo{},
			},
			expected: false,
		},
		{
			name: "Data field with cert-manager apiVersion",
			certNode: &analysis.GraphNode{
				ID: "cert-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						Data: func() json.RawMessage {
							data, _ := json.Marshal(map[string]interface{}{
								"apiVersion": "cert-manager.io/v1",
							})
							return data
						}(),
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.isCertManagerCertificate(tt.certNode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResourceExistsInGraph(t *testing.T) {
	detector := newTestDetector()

	nodeByID := map[string]*analysis.GraphNode{
		"secret-123": {
			ID: "secret-123",
			Resource: analysis.SymptomResource{
				UID:       "secret-123",
				Kind:      "Secret",
				Namespace: "default",
				Name:      "my-secret",
			},
		},
		"configmap-456": {
			ID: "configmap-456",
			Resource: analysis.SymptomResource{
				UID:       "configmap-456",
				Kind:      "ConfigMap",
				Namespace: "kube-system",
				Name:      "my-configmap",
			},
		},
	}

	tests := []struct {
		name      string
		kind      string
		namespace string
		resName   string
		expected  bool
	}{
		{
			name:      "Existing Secret",
			kind:      "Secret",
			namespace: "default",
			resName:   "my-secret",
			expected:  true,
		},
		{
			name:      "Existing ConfigMap in different namespace",
			kind:      "ConfigMap",
			namespace: "kube-system",
			resName:   "my-configmap",
			expected:  true,
		},
		{
			name:      "Non-existing Secret",
			kind:      "Secret",
			namespace: "default",
			resName:   "non-existent",
			expected:  false,
		},
		{
			name:      "Wrong namespace",
			kind:      "Secret",
			namespace: "other-namespace",
			resName:   "my-secret",
			expected:  false,
		},
		{
			name:      "Wrong kind",
			kind:      "ConfigMap",
			namespace: "default",
			resName:   "my-secret",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.resourceExistsInGraph(nodeByID, tt.kind, tt.namespace, tt.resName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsPodFailureAnomaly(t *testing.T) {
	tests := []struct {
		anomalyType string
		expected    bool
	}{
		{"CrashLoopBackOff", true},
		{"ImagePullBackOff", true},
		{"ErrImagePull", true},
		{"OOMKilled", true},
		{"ContainerCreateError", true},
		{"CreateContainerConfigError", true},
		{"InvalidImageNameError", true},
		{"PodPending", true},
		{"Evicted", true},
		{"ErrorStatus", true},
		{"InitContainerFailed", true},
		// Non-failure types
		{"ConfigMapModified", false},
		{"ImageChanged", false},
		{"SpecModified", false},
		{"SecretMissing", false},
		{"CertExpired", false},
		{"NoReadyEndpoints", false},
	}

	for _, tt := range tests {
		t.Run(tt.anomalyType, func(t *testing.T) {
			result := IsPodFailureAnomaly(tt.anomalyType)
			assert.Equal(t, tt.expected, result, "IsPodFailureAnomaly(%q)", tt.anomalyType)
		})
	}
}

func TestDeduplicateAnomalies(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		input    []Anomaly
		expected int
	}{
		{
			name:     "Empty list",
			input:    []Anomaly{},
			expected: 0,
		},
		{
			name: "No duplicates",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-2"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
			},
			expected: 2,
		},
		{
			name: "Exact duplicates",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
			},
			expected: 1,
		},
		{
			name: "Different timestamps",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now.Add(1 * time.Second)},
			},
			expected: 2,
		},
		{
			name: "Different types same node",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeB", Timestamp: now},
			},
			expected: 2,
		},
		{
			name: "Different categories same node",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryEvent, Type: "TypeA", Timestamp: now},
			},
			expected: 2,
		},
		{
			name: "Multiple duplicates",
			input: []Anomaly{
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-1"}, Category: CategoryState, Type: "TypeA", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-2"}, Category: CategoryState, Type: "TypeB", Timestamp: now},
				{Node: AnomalyNode{UID: "uid-2"}, Category: CategoryState, Type: "TypeB", Timestamp: now},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateAnomalies(tt.input)
			assert.Len(t, result, tt.expected)
		})
	}
}

func TestServiceHasSelector(t *testing.T) {
	detector := newTestDetector()

	tests := []struct {
		name        string
		serviceNode *analysis.GraphNode
		expected    bool
	}{
		{
			name: "Service with selector",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"spec": map[string]interface{}{
								"selector": map[string]interface{}{
									"app": "my-app",
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Service with empty selector",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"spec": map[string]interface{}{
								"selector": map[string]interface{}{},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Service without selector",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"spec": map[string]interface{}{
								"ports": []interface{}{},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Service with no events",
			serviceNode: &analysis.GraphNode{
				ID:        "svc-123",
				AllEvents: []analysis.ChangeEventInfo{},
			},
			expected: false,
		},
		{
			name: "Service with nil FullSnapshot",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: nil,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.serviceHasSelector(tt.serviceNode)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectServiceEndpointAnomalies(t *testing.T) {
	detector := newTestDetector()

	timeWindow := TimeWindow{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	tests := []struct {
		name          string
		serviceNode   *analysis.GraphNode
		nodeByID      map[string]*analysis.GraphNode
		edgesBySource map[string][]analysis.GraphEdge
		nodeAnomalies map[string][]Anomaly
		expectedCount int
		expectedType  string
	}{
		{
			name: "Service with no SELECTS edges and has selector",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				Resource: analysis.SymptomResource{
					UID:       "svc-123",
					Kind:      "Service",
					Namespace: "default",
					Name:      "my-service",
				},
				AllEvents: []analysis.ChangeEventInfo{
					{
						FullSnapshot: map[string]interface{}{
							"spec": map[string]interface{}{
								"selector": map[string]interface{}{
									"app": "my-app",
								},
							},
						},
					},
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"svc-123": {ID: "svc-123"},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"svc-123": {}, // No edges
			},
			nodeAnomalies: map[string][]Anomaly{},
			expectedCount: 1,
			expectedType:  "NoReadyEndpoints",
		},
		{
			name: "Service with all pods failing",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				Resource: analysis.SymptomResource{
					UID:       "svc-123",
					Kind:      "Service",
					Namespace: "default",
					Name:      "my-service",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"svc-123": {ID: "svc-123"},
				"pod-1": {
					ID: "pod-1",
					Resource: analysis.SymptomResource{
						UID:  "pod-1",
						Kind: "Pod",
						Name: "pod-1",
					},
				},
				"pod-2": {
					ID: "pod-2",
					Resource: analysis.SymptomResource{
						UID:  "pod-2",
						Kind: "Pod",
						Name: "pod-2",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"svc-123": {
					{From: "svc-123", To: "pod-1", RelationshipType: "SELECTS"},
					{From: "svc-123", To: "pod-2", RelationshipType: "SELECTS"},
				},
			},
			nodeAnomalies: map[string][]Anomaly{
				"pod-1": {{Type: "CrashLoopBackOff"}},
				"pod-2": {{Type: "OOMKilled"}},
			},
			expectedCount: 1,
			expectedType:  "NoReadyEndpoints",
		},
		{
			name: "Service with at least one healthy pod",
			serviceNode: &analysis.GraphNode{
				ID: "svc-123",
				Resource: analysis.SymptomResource{
					UID:       "svc-123",
					Kind:      "Service",
					Namespace: "default",
					Name:      "my-service",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"svc-123": {ID: "svc-123"},
				"pod-1": {
					ID: "pod-1",
					Resource: analysis.SymptomResource{
						UID:  "pod-1",
						Kind: "Pod",
						Name: "pod-1",
					},
				},
				"pod-2": {
					ID: "pod-2",
					Resource: analysis.SymptomResource{
						UID:  "pod-2",
						Kind: "Pod",
						Name: "pod-2",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"svc-123": {
					{From: "svc-123", To: "pod-1", RelationshipType: "SELECTS"},
					{From: "svc-123", To: "pod-2", RelationshipType: "SELECTS"},
				},
			},
			nodeAnomalies: map[string][]Anomaly{
				"pod-1": {{Type: "CrashLoopBackOff"}},
				"pod-2": {}, // Healthy pod
			},
			expectedCount: 0, // At least one healthy pod
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.detectServiceEndpointAnomalies(
				tt.serviceNode,
				tt.nodeByID,
				tt.edgesBySource,
				tt.nodeAnomalies,
				timeWindow,
			)

			assert.Len(t, anomalies, tt.expectedCount)

			if tt.expectedCount > 0 && len(anomalies) > 0 {
				assert.Equal(t, tt.expectedType, anomalies[0].Type)
				assert.Equal(t, SeverityHigh, anomalies[0].Severity)
			}
		})
	}
}

func TestDetectServiceAccountMissingAnomalies(t *testing.T) {
	detector := newTestDetector()

	timeWindow := TimeWindow{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
	}

	tests := []struct {
		name           string
		podNode        *analysis.GraphNode
		nodeByID       map[string]*analysis.GraphNode
		edgesBySource  map[string][]analysis.GraphEdge
		expectedCount  int
		expectedType   string
		expectedFields map[string]interface{}
	}{
		{
			name: "Pod with USES_SERVICE_ACCOUNT edge to non-existent ServiceAccount",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
				// Note: sa-456 is NOT in nodeByID - this triggers ServiceAccountMissing
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "sa-456", // This target doesn't exist in nodeByID
						RelationshipType: "USES_SERVICE_ACCOUNT",
					},
				},
			},
			expectedCount: 1,
			expectedType:  "ServiceAccountMissing",
			expectedFields: map[string]interface{}{
				"target_id": "sa-456",
				"reason":    "serviceaccount_not_found",
			},
		},
		{
			name: "Pod with USES_SERVICE_ACCOUNT edge to existing ServiceAccount - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
				"sa-456": { // ServiceAccount exists
					ID: "sa-456",
					Resource: analysis.SymptomResource{
						UID:       "sa-456",
						Kind:      "ServiceAccount",
						Namespace: "default",
						Name:      "my-sa",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "sa-456",
						RelationshipType: "USES_SERVICE_ACCOUNT",
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Pod with non-USES_SERVICE_ACCOUNT edge - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{
				"pod-123": {
					{
						From:             "pod-123",
						To:               "svc-789", // Non-existent but not USES_SERVICE_ACCOUNT
						RelationshipType: "SELECTS",
					},
				},
			},
			expectedCount: 0,
		},
		{
			name: "Pod with no edges - no anomaly",
			podNode: &analysis.GraphNode{
				ID: "pod-123",
				Resource: analysis.SymptomResource{
					UID:       "pod-123",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-app",
				},
			},
			nodeByID: map[string]*analysis.GraphNode{
				"pod-123": {
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-app",
					},
				},
			},
			edgesBySource: map[string][]analysis.GraphEdge{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.detectServiceAccountMissingAnomalies(
				tt.podNode,
				tt.nodeByID,
				tt.edgesBySource,
				timeWindow,
			)

			assert.Len(t, anomalies, tt.expectedCount, "Expected %d anomalies", tt.expectedCount)

			if tt.expectedCount > 0 && len(anomalies) > 0 {
				anomaly := anomalies[0]
				assert.Equal(t, tt.expectedType, anomaly.Type)
				assert.Equal(t, CategoryState, anomaly.Category)
				assert.Equal(t, SeverityCritical, anomaly.Severity)
				assert.Equal(t, "Pod", anomaly.Node.Kind)

				// Check expected fields in details
				for key, expectedValue := range tt.expectedFields {
					assert.Equal(t, expectedValue, anomaly.Details[key],
						"Details[%s] should be %v", key, expectedValue)
				}
			}
		})
	}
}

func TestRBACDeniedEventMapping(t *testing.T) {
	detector := NewEventAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
	}{
		{
			name: "Forbidden event with forbidden in message maps to RBACDenied",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Forbidden",
						Type:      "Warning",
						Message:   "User system:serviceaccount:default:my-sa is forbidden from getting secrets in namespace default",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RBACDenied",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedCreate with forbidden message maps to RBACDenied",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "rs-123",
					Resource: analysis.SymptomResource{
						UID:       "rs-123",
						Kind:      "ReplicaSet",
						Namespace: "default",
						Name:      "my-rs",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: pods \"my-pod\" is forbidden: unable to validate against any security context constraint",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RBACDenied",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedCreate with cannot and permission message maps to RBACDenied",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "deploy-123",
					Resource: analysis.SymptomResource{
						UID:       "deploy-123",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "my-deploy",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: pods \"my-pod\" cannot be created due to permission denied",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RBACDenied",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedCreate without forbidden/cannot+permission message stays as ReplicaCreationFailure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "deploy-123",
					Resource: analysis.SymptomResource{
						UID:       "deploy-123",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "my-deploy",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: quota exceeded",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReplicaCreationFailure",
			expectedSev:  SeverityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find the anomaly matching our expected type
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s, got types: %v",
				tt.expectedType, func() []string {
					types := make([]string, len(anomalies))
					for i, a := range anomalies {
						types[i] = a.Type
					}
					return types
				}())

			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryEvent, found.Category)
		})
	}
}

func TestClusterRoleModifiedClassification(t *testing.T) {
	detector := NewChangeAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
	}{
		{
			name: "ClusterRole spec change maps to ClusterRoleModified",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "clusterrole-123",
					Resource: analysis.SymptomResource{
						UID:  "clusterrole-123",
						Kind: "ClusterRole",
						Name: "my-clusterrole",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "rules",
								OldValue: []interface{}{},
								NewValue: []interface{}{map[string]interface{}{"verbs": []string{"get"}}},
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedType: "ClusterRoleModified",
			expectedSev:  SeverityHigh,
		},
		{
			name: "ClusterRoleBinding spec change maps to ClusterRoleBindingModified",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "clusterrolebinding-123",
					Resource: analysis.SymptomResource{
						UID:  "clusterrolebinding-123",
						Kind: "ClusterRoleBinding",
						Name: "my-clusterrolebinding",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "subjects",
								OldValue: []interface{}{},
								NewValue: []interface{}{map[string]interface{}{"name": "my-sa"}},
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedType: "ClusterRoleBindingModified",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Role spec change maps to RoleModified (not ClusterRoleModified)",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "role-123",
					Resource: analysis.SymptomResource{
						UID:       "role-123",
						Kind:      "Role",
						Namespace: "default",
						Name:      "my-role",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "rules",
								OldValue: []interface{}{},
								NewValue: []interface{}{map[string]interface{}{"verbs": []string{"get"}}},
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedType: "RoleModified",
			expectedSev:  SeverityHigh,
		},
		{
			name: "RoleBinding spec change maps to RoleBindingModified (not ClusterRoleBindingModified)",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "rolebinding-123",
					Resource: analysis.SymptomResource{
						UID:       "rolebinding-123",
						Kind:      "RoleBinding",
						Namespace: "default",
						Name:      "my-rolebinding",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "subjects",
								OldValue: []interface{}{},
								NewValue: []interface{}{map[string]interface{}{"name": "my-sa"}},
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedType: "RoleBindingModified",
			expectedSev:  SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find the anomaly matching our expected type
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s, got types: %v",
				tt.expectedType, func() []string {
					types := make([]string, len(anomalies))
					for i, a := range anomalies {
						types[i] = a.Type
					}
					return types
				}())

			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryChange, found.Category)
		})
	}
}

func TestInvalidConfigReferenceEventMapping(t *testing.T) {
	detector := NewEventAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute) // Event occurred 30 minutes ago (clearly within window)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute), // Add buffer to ensure events at "now" are included
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
	}{
		{
			name: "FailedMount with secret not found",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "MountVolume.SetUp failed for volume \"config\" : secret \"my-secret\" not found",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "InvalidConfigReference",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedMount with configmap not found",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "MountVolume.SetUp failed for volume \"config\" : configmap \"my-config\" not found",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "InvalidConfigReference",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedMount with secret doesn't exist",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "Unable to attach or mount volumes: secret \"db-credentials\" doesn't exist",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "InvalidConfigReference",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedMount for other reasons (not config reference) maps to VolumeMountFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "MountVolume.SetUp failed for volume: permission denied",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeMountFailed", // Now mapped to VolumeMountFailed for non-config mount failures
			expectedSev:  SeverityHigh,        // VolumeMountFailed is high per taxonomy
		},
		{
			name: "FailedCreate on Deployment with non-RBAC error maps to ReplicaCreationFailure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "deploy-123",
					Resource: analysis.SymptomResource{
						UID:       "deploy-123",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "my-deployment",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: pods \"my-deployment-xyz\" exceeded quota",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReplicaCreationFailure",
			expectedSev:  SeverityMedium,
		},
		{
			name: "FailedCreate on Deployment with forbidden message maps to RBACDenied",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "deploy-123",
					Resource: analysis.SymptomResource{
						UID:       "deploy-123",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "my-deployment",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: pods \"my-deployment-xyz\" is forbidden: policy violation",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RBACDenied",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedCreate on ReplicaSet maps to ReplicaCreationFailure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "rs-123",
					Resource: analysis.SymptomResource{
						UID:       "rs-123",
						Kind:      "ReplicaSet",
						Namespace: "default",
						Name:      "my-rs",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Error creating: quota exceeded",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReplicaCreationFailure",
			expectedSev:  SeverityMedium,
		},
		{
			name: "FailedCreate on Pod does NOT map to ReplicaCreationFailure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedCreate",
						Type:      "Warning",
						Message:   "Some error",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "FailedCreate", // Not mapped - only for Deployment/ReplicaSet
			expectedSev:  SeverityHigh,   // FailedCreate is high per severity.go K8sEventSeverityMap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find the anomaly matching our expected type (filter out HighFrequencyEvent etc)
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s, got types: %v",
				tt.expectedType, func() []string {
					types := make([]string, len(anomalies))
					for i, a := range anomalies {
						types[i] = a.Type
					}
					return types
				}())

			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryEvent, found.Category)
		})
	}
}

func TestImageAnomalyEventMapping(t *testing.T) {
	detector := NewEventAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
	}{
		// ImageNotFound cases
		{
			name: "ErrImagePull with not found maps to ImageNotFound",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image \"myregistry/myimage:v1\": rpc error: code = NotFound desc = not found",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImageNotFound",
			expectedSev:  SeverityCritical,
		},
		{
			name: "ErrImagePull with manifest unknown maps to ImageNotFound",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: manifest unknown: manifest unknown",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImageNotFound",
			expectedSev:  SeverityCritical,
		},
		{
			name: "ImagePullBackOff with does not exist maps to ImageNotFound",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ImagePullBackOff",
						Type:      "Warning",
						Message:   "Back-off pulling image: image does not exist",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImageNotFound",
			expectedSev:  SeverityCritical,
		},

		// RegistryAuthFailed cases
		{
			name: "ErrImagePull with unauthorized maps to RegistryAuthFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: unauthorized: authentication required",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RegistryAuthFailed",
			expectedSev:  SeverityCritical,
		},
		{
			name: "Failed with x509 certificate error maps to RegistryAuthFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "Failed to pull image: x509: certificate signed by unknown authority",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RegistryAuthFailed",
			expectedSev:  SeverityCritical,
		},
		{
			name: "ErrImagePull with forbidden maps to RegistryAuthFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: forbidden: access denied to repository",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "RegistryAuthFailed",
			expectedSev:  SeverityCritical,
		},

		// ImagePullTimeout cases
		{
			name: "ErrImagePull with i/o timeout maps to ImagePullTimeout",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: i/o timeout",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImagePullTimeout",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Failed with context deadline exceeded maps to ImagePullTimeout",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "Failed to pull image: context deadline exceeded",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImagePullTimeout",
			expectedSev:  SeverityHigh,
		},
		{
			name: "ErrImagePull with connection refused maps to ImagePullTimeout",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: dial tcp: connection refused",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ImagePullTimeout",
			expectedSev:  SeverityHigh,
		},

		// Fallback case - stays as original reason
		{
			name: "ErrImagePull without specific pattern stays as ErrImagePull",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "ErrImagePull",
						Type:      "Warning",
						Message:   "Failed to pull image: some generic error",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ErrImagePull",
			expectedSev:  SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find the anomaly matching our expected type
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s, got types: %v",
				tt.expectedType, func() []string {
					types := make([]string, len(anomalies))
					for i, a := range anomalies {
						types[i] = a.Type
					}
					return types
				}())

			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryEvent, found.Category)
		})
	}
}

func TestHelmReleaseFailedStateDetection(t *testing.T) {
	detector := NewStateAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
		expectCount  int
	}{
		{
			name: "HelmRelease with Ready=False detects HelmReleaseFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: eventTime,
						FullSnapshot: map[string]interface{}{
							"apiVersion": "helm.toolkit.fluxcd.io/v2beta1",
							"kind":       "HelmRelease",
							"status": map[string]interface{}{
								"conditions": []interface{}{
									map[string]interface{}{
										"type":    "Ready",
										"status":  "False",
										"reason":  "UpgradeFailed",
										"message": "Helm upgrade failed: timeout waiting for resources",
									},
								},
							},
						},
					},
				},
			},
			expectedType: "HelmReleaseFailed",
			expectedSev:  SeverityCritical,
			expectCount:  1,
		},
		{
			name: "HelmRelease with Released=False detects HelmReleaseFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: eventTime,
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"conditions": []interface{}{
									map[string]interface{}{
										"type":    "Released",
										"status":  "False",
										"reason":  "InstallFailed",
										"message": "Helm install failed: chart not found",
									},
								},
							},
						},
					},
				},
			},
			expectedType: "HelmReleaseFailed",
			expectedSev:  SeverityCritical,
			expectCount:  1,
		},
		{
			name: "HelmRelease with Ready=True does not detect failure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: eventTime,
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"conditions": []interface{}{
									map[string]interface{}{
										"type":   "Ready",
										"status": "True",
										"reason": "ReconciliationSucceeded",
									},
								},
							},
						},
					},
				},
			},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			if tt.expectCount == 0 {
				for _, a := range anomalies {
					assert.NotEqual(t, "HelmReleaseFailed", a.Type, "Should not have HelmReleaseFailed anomaly")
				}
				return
			}

			// Find the HelmReleaseFailed anomaly
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s", tt.expectedType)
			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryState, found.Category)
		})
	}
}

func TestKustomizationFailedStateDetection(t *testing.T) {
	detector := NewStateAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
		expectCount  int
	}{
		{
			name: "Kustomization with Ready=False detects KustomizationFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "kustomization-123",
					Resource: analysis.SymptomResource{
						UID:       "kustomization-123",
						Kind:      "Kustomization",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: eventTime,
						FullSnapshot: map[string]interface{}{
							"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
							"kind":       "Kustomization",
							"status": map[string]interface{}{
								"conditions": []interface{}{
									map[string]interface{}{
										"type":    "Ready",
										"status":  "False",
										"reason":  "BuildFailed",
										"message": "kustomize build failed: error loading manifest",
									},
								},
							},
						},
					},
				},
			},
			expectedType: "KustomizationFailed",
			expectedSev:  SeverityCritical,
			expectCount:  1,
		},
		{
			name: "Kustomization with Ready=True does not detect failure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "kustomization-123",
					Resource: analysis.SymptomResource{
						UID:       "kustomization-123",
						Kind:      "Kustomization",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: eventTime,
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"conditions": []interface{}{
									map[string]interface{}{
										"type":   "Ready",
										"status": "True",
										"reason": "ReconciliationSucceeded",
									},
								},
							},
						},
					},
				},
			},
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			if tt.expectCount == 0 {
				for _, a := range anomalies {
					assert.NotEqual(t, "KustomizationFailed", a.Type, "Should not have KustomizationFailed anomaly")
				}
				return
			}

			// Find the KustomizationFailed anomaly
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s", tt.expectedType)
			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryState, found.Category)
		})
	}
}

func TestHelmReleaseChangeDetection(t *testing.T) {
	detector := NewChangeAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name          string
		input         DetectorInput
		expectedTypes []string
		expectedSevs  map[string]Severity
	}{
		{
			name: "HelmRelease version upgrade detects HelmUpgrade",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "spec.chart.spec.version",
								OldValue: "1.2.3",
								NewValue: "1.3.0",
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedTypes: []string{"HelmReleaseUpdated", "HelmUpgrade"},
			expectedSevs: map[string]Severity{
				"HelmReleaseUpdated": SeverityHigh,
				"HelmUpgrade":        SeverityHigh,
			},
		},
		{
			name: "HelmRelease version rollback detects HelmRollback",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "spec.chart.spec.version",
								OldValue: "2.0.0",
								NewValue: "1.5.0",
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedTypes: []string{"HelmReleaseUpdated", "HelmRollback"},
			expectedSevs: map[string]Severity{
				"HelmReleaseUpdated": SeverityHigh,
				"HelmRollback":       SeverityMedium,
			},
		},
		{
			name: "HelmRelease values change detects ValuesChanged",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "spec.values.image.tag",
								OldValue: "v1.0.0",
								NewValue: "v1.1.0",
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedTypes: []string{"HelmReleaseUpdated", "ValuesChanged"},
			expectedSevs: map[string]Severity{
				"HelmReleaseUpdated": SeverityHigh,
				"ValuesChanged":      SeverityHigh,
			},
		},
		{
			name: "HelmRelease valuesFrom change detects ValuesChanged",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "spec.valuesFrom",
								OldValue: []interface{}{},
								NewValue: []interface{}{map[string]interface{}{"kind": "ConfigMap", "name": "my-values"}},
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedTypes: []string{"HelmReleaseUpdated", "ValuesChanged"},
			expectedSevs: map[string]Severity{
				"HelmReleaseUpdated": SeverityHigh,
				"ValuesChanged":      SeverityHigh,
			},
		},
		{
			name: "HelmRelease revision change from status.lastAppliedRevision detects HelmUpgrade",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "helmrelease-123",
					Resource: analysis.SymptomResource{
						UID:       "helmrelease-123",
						Kind:      "HelmRelease",
						Namespace: "flux-system",
						Name:      "my-app",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:       "event-1",
						Timestamp:     eventTime,
						EventType:     "UPDATE",
						ConfigChanged: true,
						Diff: []analysis.EventDiff{
							{
								Path:     "status.lastAppliedRevision",
								OldValue: "1.0.0",
								NewValue: "1.1.0",
								Op:       "replace",
							},
							{
								Path:     "spec.chart.spec.version",
								OldValue: "1.0.0",
								NewValue: "1.1.0",
								Op:       "replace",
							},
						},
					},
				},
			},
			expectedTypes: []string{"HelmReleaseUpdated", "HelmUpgrade"},
			expectedSevs: map[string]Severity{
				"HelmReleaseUpdated": SeverityHigh,
				"HelmUpgrade":        SeverityHigh,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find all expected anomaly types
			foundTypes := make(map[string]bool)
			for _, a := range anomalies {
				foundTypes[a.Type] = true

				// Check severity for expected types
				if expectedSev, ok := tt.expectedSevs[a.Type]; ok {
					assert.Equal(t, expectedSev, a.Severity, "Severity for %s", a.Type)
				}
			}

			for _, expectedType := range tt.expectedTypes {
				assert.True(t, foundTypes[expectedType], "Expected to find anomaly type %s, got types: %v",
					expectedType, func() []string {
						types := make([]string, 0, len(foundTypes))
						for t := range foundTypes {
							types = append(types, t)
						}
						return types
					}())
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int // -1 if v1 < v2, 0 if equal, 1 if v1 > v2
	}{
		{"semver equal", "1.2.3", "1.2.3", 0},
		{"semver less than major", "1.2.3", "2.0.0", -1},
		{"semver greater than major", "2.0.0", "1.2.3", 1},
		{"semver less than minor", "1.2.3", "1.3.0", -1},
		{"semver greater than minor", "1.3.0", "1.2.3", 1},
		{"semver less than patch", "1.2.3", "1.2.4", -1},
		{"semver greater than patch", "1.2.4", "1.2.3", 1},
		{"revision equal", "v1", "v1", 0},
		{"revision less than", "v1", "v2", -1},
		{"revision greater than", "v2", "v1", 1},
		{"numeric string", "10", "9", 1},
		{"semver with prerelease", "1.0.0-alpha", "1.0.0-beta", -1},
		{"different length versions", "1.0", "1.0.0", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareVersions(tt.v1, tt.v2)
			assert.Equal(t, tt.expected, result, "compareVersions(%q, %q)", tt.v1, tt.v2)
		})
	}
}

func TestPVCBindingFailedStateDetection(t *testing.T) {
	detector := NewStateAnomalyDetector()

	now := time.Now()
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
		expectCount  int
	}{
		{
			name: "PVC in Pending phase for extended time detects PVCBindingFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pvc-123",
					Resource: analysis.SymptomResource{
						UID:       "pvc-123",
						Kind:      "PersistentVolumeClaim",
						Namespace: "default",
						Name:      "my-pvc",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: now.Add(-5 * time.Minute), // First pending event
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Pending",
							},
						},
					},
					{
						EventID:   "event-2",
						Timestamp: now.Add(-2 * time.Minute), // Still pending after >1 minute
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Pending",
							},
						},
					},
				},
			},
			expectedType: "PVCBindingFailed",
			expectedSev:  SeverityCritical,
			expectCount:  1,
		},
		{
			name: "PVC in Lost phase detects PVCBindingFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pvc-123",
					Resource: analysis.SymptomResource{
						UID:       "pvc-123",
						Kind:      "PersistentVolumeClaim",
						Namespace: "default",
						Name:      "my-pvc",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: now.Add(-30 * time.Minute),
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Lost",
							},
						},
					},
				},
			},
			expectedType: "PVCBindingFailed",
			expectedSev:  SeverityCritical,
			expectCount:  1,
		},
		{
			name: "PVC in Bound phase does not detect failure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pvc-123",
					Resource: analysis.SymptomResource{
						UID:       "pvc-123",
						Kind:      "PersistentVolumeClaim",
						Namespace: "default",
						Name:      "my-pvc",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: now.Add(-30 * time.Minute),
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Bound",
							},
						},
					},
				},
			},
			expectCount: 0,
		},
		{
			name: "PVC briefly in Pending phase (under threshold) does not detect failure",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pvc-123",
					Resource: analysis.SymptomResource{
						UID:       "pvc-123",
						Kind:      "PersistentVolumeClaim",
						Namespace: "default",
						Name:      "my-pvc",
					},
				},
				TimeWindow: timeWindow,
				AllEvents: []analysis.ChangeEventInfo{
					{
						EventID:   "event-1",
						Timestamp: now.Add(-30 * time.Second), // Only 30 seconds in Pending
						FullSnapshot: map[string]interface{}{
							"status": map[string]interface{}{
								"phase": "Pending",
							},
						},
					},
				},
			},
			expectCount: 0, // Under 1 minute threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			if tt.expectCount == 0 {
				for _, a := range anomalies {
					assert.NotEqual(t, "PVCBindingFailed", a.Type, "Should not have PVCBindingFailed anomaly")
				}
				return
			}

			// Find the PVCBindingFailed anomaly
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s", tt.expectedType)
			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryState, found.Category)
		})
	}
}

func TestStorageEventDetection(t *testing.T) {
	detector := NewEventAnomalyDetector()

	now := time.Now()
	eventTime := now.Add(-30 * time.Minute)
	timeWindow := TimeWindow{
		Start: now.Add(-1 * time.Hour),
		End:   now.Add(1 * time.Minute),
	}

	tests := []struct {
		name         string
		input        DetectorInput
		expectedType string
		expectedSev  Severity
	}{
		// VolumeMountFailed cases
		{
			name: "FailedMount with PVC issue maps to VolumeMountFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "MountVolume.SetUp failed for volume \"data\" : rpc error: code = Internal desc = failed to mount device",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeMountFailed",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedAttachVolume maps to VolumeMountFailed",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedAttachVolume",
						Type:      "Warning",
						Message:   "Multi-Attach error for volume \"pvc-xyz\" Volume is already attached to a different node",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeMountFailed",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FailedMount with missing secret stays as InvalidConfigReference",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FailedMount",
						Type:      "Warning",
						Message:   "MountVolume.SetUp failed for volume \"config\" : secret \"my-secret\" not found",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "InvalidConfigReference", // Not VolumeMountFailed
			expectedSev:  SeverityHigh,
		},

		// VolumeOutOfSpace cases
		{
			name: "Evicted with disk space message maps to VolumeOutOfSpace",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Evicted",
						Type:      "Warning",
						Message:   "The node was low on resource: ephemeral-storage. Container was using disk space exceeding its request",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeOutOfSpace",
			expectedSev:  SeverityHigh,
		},
		{
			name: "FreeDiskSpaceFailed maps to VolumeOutOfSpace",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "FreeDiskSpaceFailed",
						Type:      "Warning",
						Message:   "Failed to free disk space, usage is above threshold",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeOutOfSpace",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Failed with no space left message maps to VolumeOutOfSpace",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "Error: write /data/file.log: no space left on device",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeOutOfSpace",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Failed with exceeded ephemeral storage maps to VolumeOutOfSpace",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "Container exceeded ephemeral storage limit",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "VolumeOutOfSpace",
			expectedSev:  SeverityHigh,
		},

		// ReadOnlyFilesystem cases
		{
			name: "Event with read-only file system message maps to ReadOnlyFilesystem",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "Error: write /data/file.log: read-only file system",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReadOnlyFilesystem",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Event with erofs error maps to ReadOnlyFilesystem",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Failed",
						Type:      "Warning",
						Message:   "mkdir /app/cache: erofs (errno 30)",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReadOnlyFilesystem",
			expectedSev:  SeverityHigh,
		},
		{
			name: "Event with remount read-only maps to ReadOnlyFilesystem",
			input: DetectorInput{
				Node: &analysis.GraphNode{
					ID: "pod-123",
					Resource: analysis.SymptomResource{
						UID:       "pod-123",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "my-pod",
					},
				},
				TimeWindow: timeWindow,
				K8sEvents: []analysis.K8sEventInfo{
					{
						Reason:    "Warning",
						Type:      "Warning",
						Message:   "Filesystem /dev/sda1 remount read-only due to errors",
						Timestamp: eventTime,
						Count:     1,
					},
				},
			},
			expectedType: "ReadOnlyFilesystem",
			expectedSev:  SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomalies := detector.Detect(tt.input)

			require.NotEmpty(t, anomalies, "Expected at least one anomaly")

			// Find the anomaly matching our expected type
			var found *Anomaly
			for i := range anomalies {
				if anomalies[i].Type == tt.expectedType {
					found = &anomalies[i]
					break
				}
			}

			require.NotNil(t, found, "Expected to find anomaly of type %s, got types: %v",
				tt.expectedType, func() []string {
					types := make([]string, len(anomalies))
					for i, a := range anomalies {
						types[i] = a.Type
					}
					return types
				}())

			assert.Equal(t, tt.expectedType, found.Type)
			assert.Equal(t, tt.expectedSev, found.Severity)
			assert.Equal(t, CategoryEvent, found.Category)
		})
	}
}
