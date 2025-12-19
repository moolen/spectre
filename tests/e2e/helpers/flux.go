package helpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureFluxInstalled checks if Flux is installed and installs it if not
func EnsureFluxInstalled(t *testing.T, k8sClient *K8sClient, kubeContext string) error {
	t.Helper()
	startTime := time.Now()

	// Check if Flux namespace exists
	ctx := context.Background()
	_, err := k8sClient.Clientset.CoreV1().Namespaces().Get(ctx, "flux-system", metav1.GetOptions{})

	if err == nil {
		t.Logf("✓ Flux is already installed (check took %v)", time.Since(startTime))
		return nil
	}

	t.Log("Flux not found, installing Flux...")
	installStart := time.Now()

	// Get absolute path to flux-install.yaml relative to project root
	// First, try to find the project root by looking for go.mod
	currentDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	projectRoot := findProjectRoot(currentDir)
	if projectRoot == "" {
		return fmt.Errorf("could not find project root (no go.mod found)")
	}

	fluxManifest := filepath.Join(projectRoot, "hack/demo/flux/flux-install.yaml")
	if _, err := os.Stat(fluxManifest); err != nil {
		return fmt.Errorf("flux manifest not found at %s: %w", fluxManifest, err)
	}

	// Install Flux using the prepared YAML manifest
	cmd := exec.Command("kubectl", "apply", "-f", fluxManifest, "--context", kubeContext)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Flux: %w\nOutput: %s", err, string(output))
	}

	t.Logf("Flux installation initiated (took %v)", time.Since(installStart))

	// Wait for Flux to be ready
	waitStart := time.Now()
	err = waitForFluxReady(ctx, k8sClient, 2*time.Minute)
	if err != nil {
		return fmt.Errorf("Flux installation timeout: %w", err)
	}

	t.Logf("✓ Flux installed successfully (total time: %v, wait time: %v)", time.Since(startTime), time.Since(waitStart))
	return nil
}

// waitForFluxReady waits for Flux controllers to be ready
func waitForFluxReady(ctx context.Context, k8sClient *K8sClient, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Check source-controller deployment
		sourceController, err := k8sClient.Clientset.AppsV1().Deployments("flux-system").Get(
			ctx, "source-controller", metav1.GetOptions{},
		)
		if err == nil && sourceController.Status.ReadyReplicas > 0 {
			// Check helm-controller deployment
			helmController, err := k8sClient.Clientset.AppsV1().Deployments("flux-system").Get(
				ctx, "helm-controller", metav1.GetOptions{},
			)
			if err == nil && helmController.Status.ReadyReplicas > 0 {
				return nil
			}
		}
		
		time.Sleep(5 * time.Second)
	}
	
	return fmt.Errorf("timeout waiting for Flux controllers to be ready")
}

// IsFluxInstalled checks if Flux is installed
func IsFluxInstalled(k8sClient *K8sClient) bool {
	ctx := context.Background()
	_, err := k8sClient.Clientset.CoreV1().Namespaces().Get(ctx, "flux-system", metav1.GetOptions{})
	return err == nil
}

// WaitForHelmReleaseReady waits for a HelmRelease to be ready
func WaitForHelmReleaseReady(ctx context.Context, t *testing.T, kubeContext, namespace, name string, timeout time.Duration) error {
	t.Helper()
	
	// Get kubeconfig from environment (set by shared cluster test setup)
	// kubectl commands use default kubeconfig
	
	deadline := time.Now().Add(timeout)
	
	t.Logf("Waiting for HelmRelease %s/%s to be ready...", namespace, name)

	// First, try to get the source repository name and kind from the HelmRelease
	// This will be empty for OCI charts specified with inline URLs (oci://...)
	cmdName := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
		"-n", namespace,
		"-o", "jsonpath={.spec.chart.spec.sourceRef.name}",
		"--context", kubeContext,
	)
	repoNameBytes, _ := cmdName.Output()

	cmdKind := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
		"-n", namespace,
		"-o", "jsonpath={.spec.chart.spec.sourceRef.kind}",
		"--context", kubeContext,
	)
	repoKindBytes, _ := cmdKind.Output()

	// Wait for source repository to be ready if we found it (only for sourced charts, not inline OCI)
	if len(repoNameBytes) > 0 && len(repoKindBytes) > 0 {
		repoName := strings.TrimSpace(string(repoNameBytes))
		repoKind := strings.TrimSpace(string(repoKindBytes))
		resourceType := strings.ToLower(repoKind) // helmrepository, ocirepository, gitrepository, or bucket

		t.Logf("Waiting for %s %s/%s to be ready first...", repoKind, namespace, repoName)

		repoDeadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(repoDeadline) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			cmd := exec.CommandContext(ctx, "kubectl", "get", resourceType, repoName,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
				"--context", kubeContext,
			)
			output, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "True" {
				t.Logf("✓ %s %s/%s is ready", repoKind, namespace, repoName)
				break
			}
			time.Sleep(5 * time.Second)
		}
	} else {
		t.Logf("No sourceRef found (using inline OCI chart URL), skipping repository wait")
	}
	
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastLogTime := time.Time{}
	debugLogInterval := 15 * time.Second
	firstNoStatusLog := true

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}

		cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
			"-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
			"--context", kubeContext,
		)

		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "True" {
			t.Logf("✓ HelmRelease %s/%s is ready", namespace, name)
			return nil
		}

		// Log current status every 15 seconds for debugging
		if time.Since(lastLogTime) >= debugLogInterval {
			// Get Ready condition message
			statusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].message}",
				"--context", kubeContext,
			)
			statusMsg, _ := statusCmd.Output()

			// Get Ready condition status
			statusStatusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
				"--context", kubeContext,
			)
			statusStatus, _ := statusStatusCmd.Output()

			if len(statusMsg) > 0 || len(statusStatus) > 0 {
				t.Logf("HelmRelease %s/%s status=%s, message=%s",
					namespace, name,
					strings.TrimSpace(string(statusStatus)),
					strings.TrimSpace(string(statusMsg)))
			} else {
				t.Logf("HelmRelease %s/%s has no status conditions yet", namespace, name)

				// First time we see no status, do comprehensive debugging
				if firstNoStatusLog {
					firstNoStatusLog = false
					t.Logf("DEBUG: Investigating why HelmRelease has no status...")

					// 1. Check Flux controller pods
					fluxPodsCmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", "flux-system", "--context", kubeContext)
					fluxPodsOutput, _ := fluxPodsCmd.Output()
					t.Logf("DEBUG: Flux controller pods:\n%s", string(fluxPodsOutput))

					// 2. Check helm-controller logs
					helmCtrlLogsCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", "flux-system",
						"deployment/helm-controller", "--tail=30", "--context", kubeContext)
					helmCtrlLogs, _ := helmCtrlLogsCmd.Output()
					t.Logf("DEBUG: helm-controller recent logs:\n%s", string(helmCtrlLogs))

					// 3. Check source-controller logs
					sourceCtrlLogsCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", "flux-system",
						"deployment/source-controller", "--tail=30", "--context", kubeContext)
					sourceCtrlLogs, _ := sourceCtrlLogsCmd.Output()
					t.Logf("DEBUG: source-controller recent logs:\n%s", string(sourceCtrlLogs))

					// 4. Check events in the namespace
					eventsCmd := exec.CommandContext(ctx, "kubectl", "get", "events", "-n", namespace,
						"--sort-by=.lastTimestamp", "--field-selector", fmt.Sprintf("involvedObject.name=%s", name), "--context", kubeContext)
					eventsOutput, _ := eventsCmd.Output()
					t.Logf("DEBUG: Events for HelmRelease %s:\n%s", name, string(eventsOutput))

					// 5. Show the full HelmRelease YAML
					hrYamlCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name, "-n", namespace, "-o", "yaml", "--context", kubeContext)
					hrYaml, _ := hrYamlCmd.Output()
					t.Logf("DEBUG: HelmRelease YAML:\n%s", string(hrYaml))
				}
			}
			lastLogTime = time.Now()
		}
	}

	// Get full debug info before failing
	t.Logf("DEBUG: HelmRelease timeout - gathering final debug info...")

	// Flux controller status
	fluxPodsCmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "-n", "flux-system", "-o", "wide", "--context", kubeContext)
	fluxPodsOutput, _ := fluxPodsCmd.Output()
	t.Logf("DEBUG: Final Flux controller pods status:\n%s", string(fluxPodsOutput))

	// HelmRelease full YAML
	statusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name, "-n", namespace, "-o", "yaml", "--context", kubeContext)
	statusOutput, _ := statusCmd.Output()

	// Recent helm-controller logs
	helmCtrlLogsCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", "flux-system",
		"deployment/helm-controller", "--tail=50", "--context", kubeContext)
	helmCtrlLogs, _ := helmCtrlLogsCmd.Output()
	t.Logf("DEBUG: Final helm-controller logs:\n%s", string(helmCtrlLogs))

	return fmt.Errorf("timeout waiting for HelmRelease %s/%s to be ready.\nStatus:\n%s",
		namespace, name, string(statusOutput))
}

// WaitForHelmReleaseReconciled waits for a HelmRelease to reconcile (even if failed)
func WaitForHelmReleaseReconciled(ctx context.Context, t *testing.T, kubeContext, namespace, name string, timeout time.Duration) error {
	t.Helper()

	// Get kubeconfig from environment (set by shared cluster test setup)
	// kubectl commands use default kubeconfig

	deadline := time.Now().Add(timeout)

	t.Logf("Waiting for HelmRelease %s/%s to reconcile...", namespace, name)

	lastLogTime := time.Time{}
	debugLogInterval := 15 * time.Second
	firstNoReconcileLog := true

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
			"-n", namespace,
			"-o", "jsonpath={.status.observedGeneration}",
			"--context", kubeContext,
		)

		output, err := cmd.Output()
		if err == nil && len(output) > 0 && string(output) != "0" {
			t.Logf("✓ HelmRelease %s/%s has reconciled", namespace, name)
			return nil
		}

		// Log debug info every 15 seconds if not reconciling
		if time.Since(lastLogTime) >= debugLogInterval {
			t.Logf("HelmRelease %s/%s observedGeneration=%s (waiting for reconciliation...)",
				namespace, name, string(output))

			// First time we see no reconciliation, do comprehensive debugging
			if firstNoReconcileLog {
				firstNoReconcileLog = false
				t.Logf("DEBUG: Investigating why HelmRelease is not reconciling...")

				// Check Flux controller logs
				helmCtrlLogsCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", "flux-system",
					"deployment/helm-controller", "--tail=30", "--context", kubeContext)
				helmCtrlLogs, _ := helmCtrlLogsCmd.Output()
				t.Logf("DEBUG: helm-controller recent logs:\n%s", string(helmCtrlLogs))

				// Check events
				eventsCmd := exec.CommandContext(ctx, "kubectl", "get", "events", "-n", namespace,
					"--sort-by=.lastTimestamp", "--field-selector", fmt.Sprintf("involvedObject.name=%s", name), "--context", kubeContext)
				eventsOutput, _ := eventsCmd.Output()
				t.Logf("DEBUG: Events for HelmRelease %s:\n%s", name, string(eventsOutput))

				// Show HelmRelease status
				hrStatusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
					"-n", namespace, "-o", "jsonpath={.status}", "--context", kubeContext)
				hrStatus, _ := hrStatusCmd.Output()
				t.Logf("DEBUG: HelmRelease status: %s", string(hrStatus))
			}
			lastLogTime = time.Now()
		}

		time.Sleep(3 * time.Second)
	}

	// Final debug output
	t.Logf("DEBUG: HelmRelease reconciliation timeout - gathering final debug info...")

	helmCtrlLogsCmd := exec.CommandContext(ctx, "kubectl", "logs", "-n", "flux-system",
		"deployment/helm-controller", "--tail=50", "--context", kubeContext)
	helmCtrlLogs, _ := helmCtrlLogsCmd.Output()
	t.Logf("DEBUG: Final helm-controller logs:\n%s", string(helmCtrlLogs))

	hrYamlCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name, "-n", namespace, "-o", "yaml", "--context", kubeContext)
	hrYaml, _ := hrYamlCmd.Output()
	t.Logf("DEBUG: Final HelmRelease YAML:\n%s", string(hrYaml))

	return fmt.Errorf("timeout waiting for HelmRelease %s/%s to reconcile", namespace, name)
}

// findProjectRoot searches for the project root by looking for go.mod
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return ""
		}
		dir = parent
	}
}
