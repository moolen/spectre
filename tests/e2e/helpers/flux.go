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
func EnsureFluxInstalled(t *testing.T, k8sClient *K8sClient, kubeconfig string) error {
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
	cmd := exec.Command("kubectl", "apply", "-f", fluxManifest)
	
	// Set kubeconfig environment variable
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}
	
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
func WaitForHelmReleaseReady(ctx context.Context, t *testing.T, namespace, name string, timeout time.Duration) error {
	t.Helper()
	
	// Get kubeconfig from environment (set by shared cluster test setup)
	kubeconfig := os.Getenv("KUBECONFIG")
	
	deadline := time.Now().Add(timeout)
	
	t.Logf("Waiting for HelmRelease %s/%s to be ready...", namespace, name)
	
	// First, try to get the HelmRepository name from the HelmRelease
	cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
		"-n", namespace,
		"-o", "jsonpath={.spec.chart.spec.sourceRef.name}",
	)
	if kubeconfig != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}
	helmRepoName, _ := cmd.Output()
	
	// Wait for HelmRepository to be ready if we found it
	if len(helmRepoName) > 0 {
		repoName := strings.TrimSpace(string(helmRepoName))
		t.Logf("Waiting for HelmRepository %s/%s to be ready first...", namespace, repoName)
		
		repoDeadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(repoDeadline) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrepository", repoName,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
			)
			if kubeconfig != "" {
				cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
			}
			output, err := cmd.Output()
			if err == nil && strings.TrimSpace(string(output)) == "True" {
				t.Logf("✓ HelmRepository %s/%s is ready", namespace, repoName)
				break
			}
			time.Sleep(5 * time.Second)
		}
	}
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
			"-n", namespace,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}",
		)
		if kubeconfig != "" {
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
		}
		
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "True" {
			t.Logf("✓ HelmRelease %s/%s is ready", namespace, name)
			return nil
		}
		
		// Log current status for debugging
		if time.Now().Unix()%15 == 0 {
			statusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
				"-n", namespace,
				"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].message}",
			)
			if kubeconfig != "" {
				statusCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
			}
			statusMsg, _ := statusCmd.Output()
			if len(statusMsg) > 0 {
				t.Logf("HelmRelease status: %s", strings.TrimSpace(string(statusMsg)))
			}
		}
		
		time.Sleep(5 * time.Second)
	}
	
	// Get full status for debugging
	statusCmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name, "-n", namespace, "-o", "yaml")
	if kubeconfig != "" {
		statusCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}
	statusOutput, _ := statusCmd.Output()
	
	return fmt.Errorf("timeout waiting for HelmRelease %s/%s to be ready.\nStatus:\n%s", 
		namespace, name, string(statusOutput))
}

// WaitForHelmReleaseReconciled waits for a HelmRelease to reconcile (even if failed)
func WaitForHelmReleaseReconciled(ctx context.Context, t *testing.T, namespace, name string, timeout time.Duration) error {
	t.Helper()
	
	// Get kubeconfig from environment (set by shared cluster test setup)
	kubeconfig := os.Getenv("KUBECONFIG")
	
	deadline := time.Now().Add(timeout)
	
	t.Logf("Waiting for HelmRelease %s/%s to reconcile...", namespace, name)
	
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		cmd := exec.CommandContext(ctx, "kubectl", "get", "helmrelease", name,
			"-n", namespace,
			"-o", "jsonpath={.status.observedGeneration}",
		)
		if kubeconfig != "" {
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
		}
		
		output, err := cmd.Output()
		if err == nil && len(output) > 0 && string(output) != "0" {
			t.Logf("✓ HelmRelease %s/%s has reconciled", namespace, name)
			return nil
		}
		
		time.Sleep(3 * time.Second)
	}
	
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
