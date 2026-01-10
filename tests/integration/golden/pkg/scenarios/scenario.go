package scenarios

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Scenario defines a golden test case that can be executed against a cluster
type Scenario interface {
	// Name returns the unique identifier for this scenario
	Name() string

	// Description returns a human-readable description
	Description() string

	// Setup prepares the initial state (deploy resources)
	Setup(ctx context.Context, client kubernetes.Interface, namespace string) error

	// Execute performs the action that causes the anomaly
	Execute(ctx context.Context, client kubernetes.Interface, namespace string) error

	// WaitCondition returns when the scenario has reached its target state
	WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error

	// Cleanup removes resources (optional, namespace deletion handles most)
	Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error

	// ExpectedAnomalies returns the anomaly types we expect to detect
	ExpectedAnomalies() []ExpectedAnomaly

	// ExpectedCausalPath returns the expected causal path structure
	ExpectedCausalPath() ExpectedPath

	// Timeout returns how long to wait for the scenario
	Timeout() time.Duration
}

// ExpectedAnomaly defines what anomaly we expect to find
type ExpectedAnomaly struct {
	NodeKind     string `json:"node_kind"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	MinSeverity  string `json:"min_severity"`
	SummaryMatch string `json:"summary_match"`
}

// ExpectedPath defines the expected causal path structure
type ExpectedPath struct {
	RootKind          string   `json:"root_kind"`
	IntermediateKinds []string `json:"intermediate_kinds"`
	SymptomKind       string   `json:"symptom_kind"`
	MinConfidence     float64  `json:"min_confidence"`
}

// Helper functions

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func createDeployment(namespace, name, image string, command []string) *corev1.ObjectFieldSelector {
	return nil // placeholder to avoid import issues; will be implemented in helpers
}

// CreateSimpleDeployment creates a basic deployment
func CreateSimpleDeployment(namespace, name, image string, command []string) map[string]interface{} {
	labels := map[string]interface{}{
		"app": name,
	}

	container := map[string]interface{}{
		"name":  name,
		"image": image,
	}

	if command != nil {
		container["command"] = command
	}

	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"replicas": 1,
			"selector": map[string]interface{}{
				"matchLabels": labels,
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": labels,
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{container},
				},
			},
		},
	}
}

// WaitCondition helper for polling conditions
func WaitCondition(ctx context.Context, timeout time.Duration, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timeout):
			return fmt.Errorf("timeout waiting for condition")
		case <-ticker.C:
			done, err := check()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for condition")
			}
		}
	}
}

// WaitForPodCondition waits for a pod matching selector to satisfy condition
func WaitForPodCondition(ctx context.Context, client kubernetes.Interface, namespace, labelSelector string, condition func(*corev1.Pod) bool) error {
	return WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		opts := metav1.ListOptions{}
		if labelSelector != "" {
			opts.LabelSelector = labelSelector
		}

		pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}

		for i := range pods.Items {
			if condition(&pods.Items[i]) {
				return true, nil
			}
		}
		return false, nil
	})
}

// WaitForDeploymentReady waits for a deployment to be ready
func WaitForDeploymentReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas, nil
	})
}
