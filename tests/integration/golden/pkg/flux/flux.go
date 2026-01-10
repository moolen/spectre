package flux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// InstallFlux installs Flux CD using the flux-install.yaml manifest
func InstallFlux(ctx context.Context, client kubernetes.Interface, kubeconfigPath string) error {
	// Check if Flux is already installed
	_, err := client.CoreV1().Namespaces().Get(ctx, "flux-system", metav1.GetOptions{})
	if err == nil {
		fmt.Println("Flux is already installed")
		return nil
	}

	fmt.Println("Installing Flux CD...")

	// Find project root and flux manifest
	projectRoot := findProjectRoot()
	fluxManifest := filepath.Join(projectRoot, "hack/demo/flux/flux-install.yaml")
	if _, err := os.Stat(fluxManifest); err != nil {
		return fmt.Errorf("flux manifest not found at %s: %w", fluxManifest, err)
	}

	// Install Flux using kubectl
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", fluxManifest, "--kubeconfig", kubeconfigPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Flux: %w\nOutput: %s", err, string(output))
	}

	fmt.Println("Flux installation initiated, waiting for controllers to be ready...")

	// Wait for Flux controllers to be ready
	return WaitForFluxReady(ctx, client, 3*time.Minute)
}

// WaitForFluxReady waits for Flux controllers to be ready
func WaitForFluxReady(ctx context.Context, client kubernetes.Interface, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check source-controller deployment
		sourceController, err := client.AppsV1().Deployments("flux-system").Get(
			ctx, "source-controller", metav1.GetOptions{},
		)
		if err == nil && sourceController.Status.ReadyReplicas > 0 {
			// Check helm-controller deployment
			helmController, err := client.AppsV1().Deployments("flux-system").Get(
				ctx, "helm-controller", metav1.GetOptions{},
			)
			if err == nil && helmController.Status.ReadyReplicas > 0 {
				fmt.Println("Flux controllers are ready")
				return nil
			}
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("timeout waiting for Flux controllers to be ready")
}

// CreateHelmRepository creates a HelmRepository resource
func CreateHelmRepository(ctx context.Context, kubeconfigPath string, namespace, name, url string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	helmRepoGVR := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "helmrepositories",
	}

	helmRepo := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "source.toolkit.fluxcd.io/v1",
			"kind":       "HelmRepository",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"interval": "5m",
				"url":      url,
			},
		},
	}

	_, err = dynamicClient.Resource(helmRepoGVR).Namespace(namespace).Create(ctx, helmRepo, metav1.CreateOptions{})
	return err
}

// CreateOCIRepository creates an OCIRepository resource
func CreateOCIRepository(ctx context.Context, kubeconfigPath string, namespace, name, url string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	ociRepoGVR := schema.GroupVersionResource{
		Group:    "source.toolkit.fluxcd.io",
		Version:  "v1beta2",
		Resource: "ocirepositories",
	}

	ociRepo := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "source.toolkit.fluxcd.io/v1beta2",
			"kind":       "OCIRepository",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"interval": "5m",
				"url":      url,
			},
		},
	}

	_, err = dynamicClient.Resource(ociRepoGVR).Namespace(namespace).Create(ctx, ociRepo, metav1.CreateOptions{})
	return err
}

// WaitForSourceReady waits for a Flux source (OCIRepository, HelmRepository, etc) to be ready
func WaitForSourceReady(ctx context.Context, kubeconfigPath string, namespace, resourceType, name string, timeout time.Duration) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	// Map resource type to GVR
	var gvr schema.GroupVersionResource
	switch resourceType {
	case "ocirepository":
		gvr = schema.GroupVersionResource{
			Group:    "source.toolkit.fluxcd.io",
			Version:  "v1beta2",
			Resource: "ocirepositories",
		}
	case "helmrepository":
		gvr = schema.GroupVersionResource{
			Group:    "source.toolkit.fluxcd.io",
			Version:  "v1",
			Resource: "helmrepositories",
		}
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	deadline := time.Now().Add(timeout)
	fmt.Printf("Waiting for %s %s/%s to be ready...\n", resourceType, namespace, name)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resource, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		conditions, found, err := unstructured.NestedSlice(resource.Object, "status", "conditions")
		if err == nil && found {
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]interface{})
				if !ok {
					continue
				}

				condTypeStr, _, _ := unstructured.NestedString(condMap, "type")
				condStatusStr, _, _ := unstructured.NestedString(condMap, "status")

				if condTypeStr == "Ready" && condStatusStr == "True" {
					fmt.Printf("%s %s/%s is ready\n", resourceType, namespace, name)
					return nil
				}
			}
		}

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("timeout waiting for %s %s/%s to be ready", resourceType, namespace, name)
}

// CreateHelmRelease creates a HelmRelease resource
func CreateHelmRelease(ctx context.Context, kubeconfigPath string, namespace, name string, spec map[string]interface{}) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	helmReleaseGVR := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}

	_, err = dynamicClient.Resource(helmReleaseGVR).Namespace(namespace).Create(ctx, hr, metav1.CreateOptions{})
	return err
}

// UpdateHelmRelease updates a HelmRelease resource
func UpdateHelmRelease(ctx context.Context, kubeconfigPath string, namespace, name string, spec map[string]interface{}) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	helmReleaseGVR := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}

	// Get existing HelmRelease
	existing, err := dynamicClient.Resource(helmReleaseGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Update spec
	existing.Object["spec"] = spec

	_, err = dynamicClient.Resource(helmReleaseGVR).Namespace(namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// WaitForHelmReleaseReconciled waits for a HelmRelease to be reconciled (observedGeneration > 0)
func WaitForHelmReleaseReconciled(ctx context.Context, kubeconfigPath string, namespace, name string, timeout time.Duration) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	helmReleaseGVR := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}

	deadline := time.Now().Add(timeout)
	fmt.Printf("Waiting for HelmRelease %s/%s to reconcile...\n", namespace, name)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hr, err := dynamicClient.Resource(helmReleaseGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			fmt.Printf("Failed to get HelmRelease: %v\n", err)
			time.Sleep(3 * time.Second)
			continue
		}

		status, found, err := unstructured.NestedMap(hr.Object, "status")
		if err == nil && found {
			// Check for Ready=True condition instead of observedGeneration
			conditions, _, _ := unstructured.NestedSlice(status, "conditions")
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]interface{})
				if !ok {
					continue
				}
				condType, _, _ := unstructured.NestedString(condMap, "type")
				condStatus, _, _ := unstructured.NestedString(condMap, "status")

				if condType == "Ready" && condStatus == "True" {
					fmt.Printf("HelmRelease %s/%s reconciled successfully (Ready=True)\n", namespace, name)
					return nil
				}
			}

			// Debug: print current status
			if len(conditions) > 0 {
				for _, cond := range conditions {
					condMap, ok := cond.(map[string]interface{})
					if ok {
						condType, _, _ := unstructured.NestedString(condMap, "type")
						condStatus, _, _ := unstructured.NestedString(condMap, "status")
						condReason, _, _ := unstructured.NestedString(condMap, "reason")
						if condType == "Ready" || condType == "Reconciling" {
							fmt.Printf("HelmRelease %s/%s: %s=%s (%s)\n", namespace, name, condType, condStatus, condReason)
						}
					}
				}
			} else {
				fmt.Printf("HelmRelease %s/%s: no conditions yet\n", namespace, name)
			}
		} else {
			fmt.Printf("HelmRelease %s/%s: no status yet\n", namespace, name)
		}

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("timeout waiting for HelmRelease %s/%s to reconcile", namespace, name)
}

// WaitForHelmReleaseCondition waits for a specific condition on the HelmRelease
func WaitForHelmReleaseCondition(ctx context.Context, kubeconfigPath string, namespace, name string, conditionType string, conditionStatus string, timeout time.Duration) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	helmReleaseGVR := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}

	deadline := time.Now().Add(timeout)
	fmt.Printf("Waiting for HelmRelease %s/%s condition %s=%s...\n", namespace, name, conditionType, conditionStatus)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		hr, err := dynamicClient.Resource(helmReleaseGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}

		conditions, found, err := unstructured.NestedSlice(hr.Object, "status", "conditions")
		if err == nil && found {
			for _, cond := range conditions {
				condMap, ok := cond.(map[string]interface{})
				if !ok {
					continue
				}

				condTypeStr, _, _ := unstructured.NestedString(condMap, "type")
				condStatusStr, _, _ := unstructured.NestedString(condMap, "status")

				if condTypeStr == conditionType && condStatusStr == conditionStatus {
					fmt.Printf("HelmRelease %s/%s has condition %s=%s\n", namespace, name, conditionType, conditionStatus)
					return nil
				}
			}
		}

		time.Sleep(3 * time.Second)
	}

	return fmt.Errorf("timeout waiting for HelmRelease %s/%s condition %s=%s", namespace, name, conditionType, conditionStatus)
}

// findProjectRoot searches for the project root by looking for go.mod
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "."
		}
		dir = parent
	}
}
