package grafana

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInferWorkloadFromLabels_LabelPriority(t *testing.T) {
	testCases := []struct {
		name                  string
		labels                map[string]string
		expectedWorkloadName  string
		expectedInferredFrom  string
		expectedConfidence    float64
	}{
		{
			name: "Deployment has highest priority",
			labels: map[string]string{
				"namespace":  "prod",
				"deployment": "api-server",
				"app":        "api",
				"service":    "api-svc",
			},
			expectedWorkloadName: "api-server",
			expectedInferredFrom: "deployment",
			expectedConfidence:   0.9,
		},
		{
			name: "App.kubernetes.io/name when deployment absent",
			labels: map[string]string{
				"namespace":                "prod",
				"app.kubernetes.io/name":   "frontend",
				"app":                      "frontend-app",
				"service":                  "frontend-svc",
			},
			expectedWorkloadName: "frontend",
			expectedInferredFrom: "app.kubernetes.io/name",
			expectedConfidence:   0.9,
		},
		{
			name: "App label when higher priority absent",
			labels: map[string]string{
				"namespace": "staging",
				"app":       "backend",
				"service":   "backend-svc",
			},
			expectedWorkloadName: "backend",
			expectedInferredFrom: "app",
			expectedConfidence:   0.9,
		},
		{
			name: "Service label when app absent",
			labels: map[string]string{
				"namespace": "test",
				"service":   "database",
			},
			expectedWorkloadName: "database",
			expectedInferredFrom: "service",
			expectedConfidence:   0.9,
		},
		{
			name: "Job label priority",
			labels: map[string]string{
				"namespace": "prod",
				"job":       "batch-processor",
				"pod":       "batch-processor-abc123",
			},
			expectedWorkloadName: "batch-processor",
			expectedInferredFrom: "job",
			expectedConfidence:   0.9,
		},
		{
			name: "Pod label as lowest priority",
			labels: map[string]string{
				"namespace": "prod",
				"pod":       "standalone-pod-xyz789",
			},
			expectedWorkloadName: "standalone-pod-xyz789",
			expectedInferredFrom: "pod",
			expectedConfidence:   0.9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inference := InferWorkloadFromLabels(tc.labels)

			assert.NotNil(t, inference)
			assert.Equal(t, tc.expectedWorkloadName, inference.WorkloadName)
			assert.Equal(t, tc.expectedInferredFrom, inference.InferredFrom)
			assert.Equal(t, tc.expectedConfidence, inference.Confidence)
		})
	}
}

func TestInferWorkloadFromLabels_NamespaceInference(t *testing.T) {
	testCases := []struct {
		name              string
		labels            map[string]string
		expectedNamespace string
		expectedConfidence float64
	}{
		{
			name: "Namespace present with deployment",
			labels: map[string]string{
				"namespace":  "production",
				"deployment": "api",
			},
			expectedNamespace:  "production",
			expectedConfidence: 0.9,
		},
		{
			name: "Namespace present with app",
			labels: map[string]string{
				"namespace": "staging",
				"app":       "frontend",
			},
			expectedNamespace:  "staging",
			expectedConfidence: 0.9,
		},
		{
			name: "Namespace absent",
			labels: map[string]string{
				"deployment": "api",
			},
			expectedNamespace:  "",
			expectedConfidence: 0.9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inference := InferWorkloadFromLabels(tc.labels)

			assert.NotNil(t, inference)
			assert.Equal(t, tc.expectedNamespace, inference.Namespace)
			assert.Equal(t, tc.expectedConfidence, inference.Confidence)
		})
	}
}

func TestInferWorkloadFromLabels_EmptyLabels(t *testing.T) {
	inference := InferWorkloadFromLabels(map[string]string{})

	assert.Nil(t, inference)
}

func TestInferWorkloadFromLabels_NoWorkloadLabels(t *testing.T) {
	// Only has namespace but no workload identifiers
	labels := map[string]string{
		"namespace": "prod",
		"cluster":   "us-west-1",
		"region":    "us-west",
	}

	inference := InferWorkloadFromLabels(labels)

	// Should return namespace-only inference (empty workload name)
	assert.NotNil(t, inference)
	assert.Equal(t, "prod", inference.Namespace)
	assert.Equal(t, "", inference.WorkloadName)
	assert.Equal(t, "namespace", inference.InferredFrom)
	assert.Equal(t, 0.7, inference.Confidence)
}

func TestInferWorkloadFromLabels_MultipleLabelsHighestWins(t *testing.T) {
	// Multiple workload labels present - should pick deployment (highest priority)
	labels := map[string]string{
		"namespace":  "prod",
		"deployment": "api-deployment",
		"app":        "api-app",
		"service":    "api-service",
		"job":        "api-job",
		"pod":        "api-pod-123",
	}

	inference := InferWorkloadFromLabels(labels)

	assert.NotNil(t, inference)
	assert.Equal(t, "api-deployment", inference.WorkloadName)
	assert.Equal(t, "deployment", inference.InferredFrom)
	assert.Equal(t, 0.9, inference.Confidence)
}

func TestInferWorkloadFromLabels_StandardK8sRecommendedLabels(t *testing.T) {
	// Test standard K8s recommended labels pattern
	labels := map[string]string{
		"namespace":                "production",
		"app.kubernetes.io/name":   "nginx",
		"app.kubernetes.io/version": "1.21",
		"app.kubernetes.io/component": "frontend",
	}

	inference := InferWorkloadFromLabels(labels)

	assert.NotNil(t, inference)
	assert.Equal(t, "nginx", inference.WorkloadName)
	assert.Equal(t, "app.kubernetes.io/name", inference.InferredFrom)
	assert.Equal(t, "production", inference.Namespace)
	assert.Equal(t, 0.9, inference.Confidence)
}

func TestInferWorkloadFromLabels_InferredFromTracking(t *testing.T) {
	testCases := []struct {
		name                 string
		labels               map[string]string
		expectedInferredFrom string
	}{
		{
			name: "Deployment label tracked",
			labels: map[string]string{
				"namespace":  "prod",
				"deployment": "api",
			},
			expectedInferredFrom: "deployment",
		},
		{
			name: "App.kubernetes.io/name label tracked",
			labels: map[string]string{
				"namespace":              "prod",
				"app.kubernetes.io/name": "frontend",
			},
			expectedInferredFrom: "app.kubernetes.io/name",
		},
		{
			name: "App label tracked",
			labels: map[string]string{
				"namespace": "prod",
				"app":       "backend",
			},
			expectedInferredFrom: "app",
		},
		{
			name: "Service label tracked",
			labels: map[string]string{
				"namespace": "prod",
				"service":   "database",
			},
			expectedInferredFrom: "service",
		},
		{
			name: "Job label tracked",
			labels: map[string]string{
				"namespace": "prod",
				"job":       "batch",
			},
			expectedInferredFrom: "job",
		},
		{
			name: "Pod label tracked",
			labels: map[string]string{
				"namespace": "prod",
				"pod":       "standalone",
			},
			expectedInferredFrom: "pod",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inference := InferWorkloadFromLabels(tc.labels)

			assert.NotNil(t, inference)
			assert.Equal(t, tc.expectedInferredFrom, inference.InferredFrom)
		})
	}
}

func TestInferWorkloadFromLabels_EmptyWorkloadName(t *testing.T) {
	// Labels with empty values should be skipped
	labels := map[string]string{
		"namespace":  "prod",
		"deployment": "", // Empty deployment name
		"app":        "backend",
	}

	inference := InferWorkloadFromLabels(labels)

	assert.NotNil(t, inference)
	assert.Equal(t, "backend", inference.WorkloadName) // Falls through to app
	assert.Equal(t, "app", inference.InferredFrom)
}

func TestInferWorkloadFromLabels_NilInput(t *testing.T) {
	inference := InferWorkloadFromLabels(nil)

	assert.Nil(t, inference)
}
