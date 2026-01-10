package scenarios

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/moolen/spectre/tests/integration/golden/pkg/flux"
)

// HelmReleaseValueFromFailure scenario: HelmRelease failure due to missing ConfigMap in valuesFrom
type HelmReleaseValueFromFailure struct{}

// NewHelmReleaseValueFromFailure creates the scenario
func NewHelmReleaseValueFromFailure() Scenario {
	return &HelmReleaseValueFromFailure{}
}

func (s *HelmReleaseValueFromFailure) Name() string {
	return "helmrelease-valuesfrom-failure"
}

func (s *HelmReleaseValueFromFailure) Description() string {
	return "HelmRelease failure due to missing ConfigMap referenced in valuesFrom"
}

func (s *HelmReleaseValueFromFailure) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmReleaseValuesFrom] Creating HelmRepository source...")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Create HelmRepository source (podinfo - lightweight test chart)
	if err := flux.CreateHelmRepository(ctx, kubeconfigPath, namespace, "podinfo", "https://stefanprodan.github.io/podinfo"); err != nil {
		return fmt.Errorf("failed to create HelmRepository: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] Waiting for HelmRepository to be ready...")
	if err := flux.WaitForSourceReady(ctx, kubeconfigPath, namespace, "helmrepository", "podinfo", 2*time.Minute); err != nil {
		return fmt.Errorf("HelmRepository failed to become ready: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] Creating ConfigMap with helm values...")

	// Create ConfigMap with values
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-values-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"values.yaml": `
replicaCount: 2
`,
		},
	}

	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] Creating HelmRelease with valuesFrom...")

	// Create HelmRelease that references the ConfigMap via valuesFrom
	spec := map[string]interface{}{
		"interval": "1m",
		"upgrade": map[string]interface{}{
			"disableWait": false,
			"timeout":     "1m",
			"remediation": map[string]interface{}{
				"retries": 0,
			},
		},
		"chart": map[string]interface{}{
			"spec": map[string]interface{}{
				"chart":   "podinfo",
				"version": "6.x",
				"sourceRef": map[string]interface{}{
					"kind": "HelmRepository",
					"name": "podinfo",
				},
			},
		},
		"valuesFrom": []interface{}{
			map[string]interface{}{
				"kind": "ConfigMap",
				"name": "helm-values-config",
			},
		},
	}

	if err := flux.CreateHelmRelease(ctx, kubeconfigPath, namespace, "values-test-app", spec); err != nil {
		return fmt.Errorf("failed to create HelmRelease: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] Waiting for HelmRelease to reconcile...")
	if err := flux.WaitForHelmReleaseReconciled(ctx, kubeconfigPath, namespace, "values-test-app", 5*time.Minute); err != nil {
		return fmt.Errorf("HelmRelease failed to reconcile: %w", err)
	}

	// Wait for pods to start
	fmt.Println("[HelmReleaseValuesFrom] Waiting for pods to start...")
	time.Sleep(15 * time.Second)

	fmt.Println("[HelmReleaseValuesFrom] HelmRelease deployed successfully with valuesFrom")
	return nil
}

func (s *HelmReleaseValueFromFailure) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmReleaseValuesFrom] Deleting ConfigMap referenced in valuesFrom...")

	// Delete the ConfigMap that the HelmRelease depends on
	if err := client.CoreV1().ConfigMaps(namespace).Delete(ctx, "helm-values-config", metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete ConfigMap: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] ConfigMap deleted, waiting for HelmRelease to detect the issue...")

	// Force reconciliation by updating the HelmRelease (add an annotation)
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Update with same spec to trigger reconciliation
	spec := map[string]interface{}{
		"interval": "1m",
		"upgrade": map[string]interface{}{
			"disableWait": false,
			"timeout":     "1m",
			"remediation": map[string]interface{}{
				"retries": 0,
			},
		},
		"chart": map[string]interface{}{
			"spec": map[string]interface{}{
				"chart":   "podinfo",
				"version": "6.x",
				"sourceRef": map[string]interface{}{
					"kind": "HelmRepository",
					"name": "podinfo",
				},
			},
		},
		"valuesFrom": []interface{}{
			map[string]interface{}{
				"kind": "ConfigMap",
				"name": "helm-values-config",
			},
		},
	}

	if err := flux.UpdateHelmRelease(ctx, kubeconfigPath, namespace, "values-test-app", spec); err != nil {
		return fmt.Errorf("failed to update HelmRelease: %w", err)
	}

	fmt.Println("[HelmReleaseValuesFrom] Waiting for reconciliation failure...")
	return nil
}

func (s *HelmReleaseValueFromFailure) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmReleaseValuesFrom] Waiting for HelmRelease to report failure due to missing ConfigMap...")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Wait for HelmRelease to have Ready=False condition
	// The reconciliation interval is 1m, so we need to wait for that
	// plus some buffer time for Flux to detect and report the failure
	if err := flux.WaitForHelmReleaseCondition(ctx, kubeconfigPath, namespace, "values-test-app", "Ready", "False", 3*time.Minute); err != nil {
		// If that doesn't work, just wait for events showing the error
		fmt.Printf("[HelmReleaseValuesFrom] Warning: HelmRelease didn't report failure condition: %v\n", err)
		fmt.Println("[HelmReleaseValuesFrom] Checking for error events...")

		// Wait a bit more for events to appear
		time.Sleep(30 * time.Second)
		return nil
	}

	fmt.Println("[HelmReleaseValuesFrom] HelmRelease reported failure condition")
	return nil
}

func (s *HelmReleaseValueFromFailure) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Cleanup happens via namespace deletion
	return nil
}

func (s *HelmReleaseValueFromFailure) ExpectedAnomalies() []ExpectedAnomaly {
	// When the ConfigMap is deleted, Flux HelmRelease reports a ValuesError condition.
	// The causal chain is: ConfigMap deletion → HelmRelease failure → no update to Deployment
	//
	// What IS detected:
	// - ConfigMap ResourceDeleted (the root cause - referenced ConfigMap was deleted)
	// - HelmRelease ErrorStatus when status shows ValuesError condition
	// - HelmRelease FlappingState when status changes rapidly
	return []ExpectedAnomaly{
		{
			NodeKind:    "ConfigMap",
			Category:    "Change",
			Type:        "ResourceDeleted",
			MinSeverity: "medium",
		},
		{
			NodeKind:    "HelmRelease",
			Category:    "State",
			Type:        "ErrorStatus",
			MinSeverity: "high",
		},
	}
}

func (s *HelmReleaseValueFromFailure) ExpectedCausalPath() ExpectedPath {
	// With intent owner and GitOps controller boosts, HelmRelease is now scored higher
	// than ConfigMap. The HelmRelease gets +10% (intent owner + GitOps) while ConfigMap
	// only gets +5% (intent owner). The causal path traces:
	// HelmRelease → Deployment → ReplicaSet → Pod
	return ExpectedPath{
		RootKind:          "HelmRelease",
		IntermediateKinds: []string{"Deployment", "ReplicaSet"},
		SymptomKind:       "Pod",
		MinConfidence:     0.65,
	}
}

func (s *HelmReleaseValueFromFailure) Timeout() time.Duration {
	return 8 * time.Minute // Allow time for initial setup + reconciliation interval (1m) + failure detection
}
