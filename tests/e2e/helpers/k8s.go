// Package helpers provides Kubernetes client utilities for e2e testing.
package helpers

import (
	"context"
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient provides methods to interact with a Kubernetes cluster.
type K8sClient struct {
	Clientset *kubernetes.Clientset
	t         *testing.T
}

// NewK8sClient creates a new Kubernetes client from kubeconfig.
func NewK8sClient(t *testing.T, kubeConfigPath string) (*K8sClient, error) {
	t.Logf("Creating Kubernetes client from kubeconfig: %s", kubeConfigPath)

	// Load kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build kube config: %w", err)
	}

	config.QPS = -1
	config.Burst = 200

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	t.Logf("✓ Kubernetes client created")

	return &K8sClient{
		Clientset: clientset,
		t:         t,
	}, nil
}

// CreateNamespace creates a new namespace.
func (k *K8sClient) CreateNamespace(ctx context.Context, name string) error {
	k.t.Logf("Creating namespace: %s", name)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err := k.Clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", name, err)
	}

	k.t.Logf("✓ Namespace created: %s", name)
	return nil
}

// DeleteNamespace deletes a namespace.
func (k *K8sClient) DeleteNamespace(ctx context.Context, name string) error {
	k.t.Logf("Deleting namespace: %s", name)

	err := k.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	k.t.Logf("✓ Namespace deleted: %s", name)
	return nil
}

// CreateDeployment creates a deployment from a YAML file.
func (k *K8sClient) CreateDeployment(ctx context.Context, namespace string, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	k.t.Logf("Creating deployment %s in namespace %s", deployment.Name, namespace)

	created, err := k.Clientset.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment: %w", err)
	}

	k.t.Logf("✓ Deployment created: %s/%s", namespace, deployment.Name)
	return created, nil
}

// GetDeployment retrieves a deployment.
func (k *K8sClient) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	return k.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeleteDeployment deletes a deployment.
func (k *K8sClient) DeleteDeployment(ctx context.Context, namespace, name string) error {
	k.t.Logf("Deleting deployment %s from namespace %s", name, namespace)

	propagationPolicy := metav1.DeletePropagationForeground
	err := k.Clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	k.t.Logf("✓ Deployment deleted: %s/%s", namespace, name)
	return nil
}

// ListPods lists all pods in a namespace.
func (k *K8sClient) ListPods(ctx context.Context, namespace string, selector string) (*corev1.PodList, error) {
	opts := metav1.ListOptions{}
	if selector != "" {
		opts.LabelSelector = selector
	}
	return k.Clientset.CoreV1().Pods(namespace).List(ctx, opts)
}

// GetPod retrieves a single pod.
func (k *K8sClient) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	return k.Clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DeletePod deletes a pod.
func (k *K8sClient) DeletePod(ctx context.Context, namespace, name string) error {
	k.t.Logf("Deleting pod %s from namespace %s", name, namespace)

	propagationPolicy := metav1.DeletePropagationForeground
	err := k.Clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	k.t.Logf("✓ Pod deleted: %s/%s", namespace, name)
	return nil
}

// WaitForPodReady waits for a pod to be ready.
func (k *K8sClient) WaitForPodReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	k.t.Logf("Waiting for pod to be ready: %s/%s", namespace, name)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod %s/%s to be ready", namespace, name)
		case <-ticker.C:
			pod, err := k.GetPod(ctx, namespace, name)
			if err != nil {
				continue
			}

			// Check if pod is ready
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					k.t.Logf("✓ Pod is ready: %s/%s", namespace, name)
					return nil
				}
			}
		}
	}
}

// WaitForDeploymentReady waits for a deployment to be ready with all replicas available.
func (k *K8sClient) WaitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	k.t.Logf("Waiting for deployment to be ready: %s/%s", namespace, name)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", namespace, name)
		case <-ticker.C:
			deployment, err := k.GetDeployment(ctx, namespace, name)
			if err != nil {
				continue
			}

			// Check if deployment is ready:
			// 1. ObservedGeneration must match Generation (deployment controller has processed the latest spec)
			// 2. ReadyReplicas must equal desired replicas
			// 3. AvailableReplicas must be at least the desired replicas
			desiredReplicas := int32(1) // default if replicas is nil
			if deployment.Spec.Replicas != nil {
				desiredReplicas = *deployment.Spec.Replicas
			}

			if deployment.Status.ObservedGeneration >= deployment.Generation &&
				deployment.Status.ReadyReplicas == desiredReplicas &&
				deployment.Status.AvailableReplicas >= desiredReplicas {
				k.t.Logf("✓ Deployment is ready: %s/%s (replicas: %d/%d available, %d/%d ready)",
					namespace, name,
					deployment.Status.AvailableReplicas, desiredReplicas,
					deployment.Status.ReadyReplicas, desiredReplicas)
				return nil
			}
		}
	}
}

// AnnotatePod annotates a pod with a given set of annotations.
func (k *K8sClient) AnnotatePod(ctx context.Context, namespace, name string, annotations map[string]string) error {
	k.t.Logf("Annotating pod %s/%s with annotations: %v", namespace, name, annotations)

	pod, err := k.GetPod(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	for k, v := range annotations {
		pod.Annotations[k] = v
	}
	_, err = k.Clientset.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod: %w", err)
	}

	k.t.Logf("✓ Pod annotated: %s/%s", namespace, name)
	return nil
}

// GetClusterVersion returns the Kubernetes version.
func (k *K8sClient) GetClusterVersion(ctx context.Context) (string, error) {
	version, err := k.Clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return version.GitVersion, nil
}

// UpdateConfigMap updates the data in an existing ConfigMap.
func (k *K8sClient) UpdateConfigMap(ctx context.Context, namespace, name string, data map[string]string) error {
	k.t.Logf("Updating ConfigMap %s/%s", namespace, name)

	cm, err := k.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	cm.Data = data
	_, err = k.Clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	k.t.Logf("✓ ConfigMap updated: %s/%s", namespace, name)
	return nil
}
