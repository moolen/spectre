package scenarios

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/moolen/spectre/tests/integration/golden/pkg/flux"
)

// HelmReleaseUpgrade scenario: HelmRelease upgrade failure due to invalid image
type HelmReleaseUpgrade struct{}

// NewHelmReleaseUpgrade creates the scenario
func NewHelmReleaseUpgrade() Scenario {
	return &HelmReleaseUpgrade{}
}

func (s *HelmReleaseUpgrade) Name() string {
	return "helmrelease-upgrade-failure"
}

func (s *HelmReleaseUpgrade) Description() string {
	return "HelmRelease upgrade failure due to invalid container image tag"
}

func (s *HelmReleaseUpgrade) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmRelease] Creating HelmRepository source...")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Create HelmRepository source (podinfo - lightweight test chart)
	if err := flux.CreateHelmRepository(ctx, kubeconfigPath, namespace, "podinfo", "https://stefanprodan.github.io/podinfo"); err != nil {
		return fmt.Errorf("failed to create HelmRepository: %w", err)
	}

	fmt.Println("[HelmRelease] Waiting for HelmRepository to be ready...")
	if err := flux.WaitForSourceReady(ctx, kubeconfigPath, namespace, "helmrepository", "podinfo", 2*time.Minute); err != nil {
		return fmt.Errorf("HelmRepository failed to become ready: %w", err)
	}

	fmt.Println("[HelmRelease] Creating initial HelmRelease with valid podinfo chart...")
	spec := map[string]interface{}{
		"interval": "5m",
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
		"values": map[string]interface{}{
			// TODO(mj): we should add another helmrelease upgrade case where the replicaCount is 1
			//       this causes the HelmRelease to become ready _immediately_ after the upgrade
			//       this a special case that we should handle in the anomaly detection logic
			//       because then we don't see a failure on this node, but only a change event.
			"replicaCount": 2,
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"cpu":    "10m",
					"memory": "16Mi",
				},
			},
		},
	}

	if err := flux.CreateHelmRelease(ctx, kubeconfigPath, namespace, "test-app", spec); err != nil {
		return fmt.Errorf("failed to create HelmRelease: %w", err)
	}

	fmt.Println("[HelmRelease] Waiting for initial HelmRelease to reconcile...")
	if err := flux.WaitForHelmReleaseReconciled(ctx, kubeconfigPath, namespace, "test-app", 5*time.Minute); err != nil {
		return fmt.Errorf("HelmRelease failed to reconcile: %w", err)
	}

	// Wait for pods to start
	fmt.Println("[HelmRelease] Waiting for pods to start...")
	time.Sleep(15 * time.Second)

	fmt.Println("[HelmRelease] Initial HelmRelease deployed successfully")
	return nil
}

func (s *HelmReleaseUpgrade) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmRelease] Updating HelmRelease with invalid image tag...")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Update HelmRelease to use an invalid image tag
	spec := map[string]interface{}{
		"interval": "5m",
		"timeout":  "1m",
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
		"upgrade": map[string]interface{}{
			"disableWait": false,
			"timeout":     "1m",
			"remediation": map[string]interface{}{
				"retries": 0,
			},
		},
		"values": map[string]interface{}{
			"replicaCount": 2,
			"resources": map[string]interface{}{
				"requests": map[string]interface{}{
					"cpu":    "10m",
					"memory": "16Mi",
				},
			},
			"image": map[string]interface{}{
				"repository": "ghcr.io/nonexistent/invalid-image", // Invalid repository to cause ImagePullBackOff
				"tag":        "nonexistent",
			},
		},
	}

	if err := flux.UpdateHelmRelease(ctx, kubeconfigPath, namespace, "test-app", spec); err != nil {
		return fmt.Errorf("failed to update HelmRelease: %w", err)
	}

	fmt.Println("[HelmRelease] HelmRelease updated")
	fmt.Println("[HelmRelease] Waiting for new pods to be created with bad image...")
	time.Sleep(60 * time.Second)

	fmt.Println("[HelmRelease] Upgrade executed, waiting for failure symptoms...")
	return nil
}

func (s *HelmReleaseUpgrade) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Println("[HelmRelease] Waiting for HelmRelease upgrade to fail...")

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		return fmt.Errorf("KUBECONFIG environment variable not set")
	}

	// Wait for HelmRelease to have Ready=False condition (upgrade failed)
	// The upgrade timeout is 1m (set in Execute), so we need to wait for that
	// plus some buffer time for Flux to detect and report the failure
	if err := flux.WaitForHelmReleaseCondition(ctx, kubeconfigPath, namespace, "test-app", "Ready", "False", 3*time.Minute); err != nil {
		// If that doesn't work, try waiting for pods with ImagePullBackOff as fallback
		fmt.Printf("[HelmRelease] Warning: HelmRelease didn't report failure condition: %v\n", err)
		fmt.Println("[HelmRelease] Falling back to waiting for pod with ImagePullBackOff...")

		return WaitForPodCondition(ctx, client, namespace, "app.kubernetes.io/name=podinfo", func(pod *corev1.Pod) bool {
			if len(pod.Status.ContainerStatuses) > 0 {
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Waiting != nil {
						reason := cs.State.Waiting.Reason
						if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
							fmt.Printf("[HelmRelease] Pod %s has %s - failure detected\n", pod.Name, reason)
							return true
						}
					}
				}
			}
			return false
		})
	}

	fmt.Println("[HelmRelease] HelmRelease upgrade failed (Ready=False) - failure detected")
	return nil
}

func (s *HelmReleaseUpgrade) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Cleanup happens via namespace deletion
	return nil
}

func (s *HelmReleaseUpgrade) ExpectedAnomalies() []ExpectedAnomaly {
	// The HelmRelease upgrade causes a Deployment update with an invalid image.
	// This creates a chain: HelmRelease (ErrorStatus) → Deployment (ImageChanged) → Pod (ImagePullBackOff)
	return []ExpectedAnomaly{
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "ImagePullBackOff",
			MinSeverity: "high",
		},
		{
			NodeKind:    "HelmRelease",
			Category:    "State",
			Type:        "ErrorStatus",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Deployment",
			Category:    "Change",
			Type:        "ImageChanged",
			MinSeverity: "medium",
		},
	}
}

func (s *HelmReleaseUpgrade) ExpectedCausalPath() ExpectedPath {
	// The causal path follows: HelmRelease → Deployment → ReplicaSet → Pod
	// The causal path algorithm follows both OWNS and MANAGES edges to discover
	// the full chain from the HelmRelease (which manages the Deployment) down to the Pod.
	return ExpectedPath{
		RootKind:          "HelmRelease",
		IntermediateKinds: []string{"Deployment", "ReplicaSet"},
		SymptomKind:       "Pod",
		MinConfidence:     0.85,
	}
}

func (s *HelmReleaseUpgrade) Timeout() time.Duration {
	return 8 * time.Minute // Allow time for initial setup + upgrade timeout (1m) + failure detection
}
