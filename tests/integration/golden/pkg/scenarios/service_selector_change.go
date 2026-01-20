package scenarios

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// ServiceSelectorChange scenario: Service selector changed to not match any pods
// This tests the case where:
// - Deployment and Pods are healthy
// - Service selector is changed to not match any pods
// - Service should have NoReadyEndpoints anomaly
// - Service itself is the root cause (selector change)
type ServiceSelectorChange struct{}

// NewServiceSelectorChange creates the scenario
func NewServiceSelectorChange() Scenario {
	return &ServiceSelectorChange{}
}

func (s *ServiceSelectorChange) Name() string {
	return "service-selector-change"
}

func (s *ServiceSelectorChange) Description() string {
	return "Service selector changed to not match any pods"
}

func (s *ServiceSelectorChange) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Setting up service-selector-change scenario\n")

	// Create a Deployment with healthy pods
	fmt.Printf("[DEBUG] Creating Deployment 'backend' with 2 replicas\n")
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "backend"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "backend"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:1.20",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80},
							},
						},
					},
				},
			},
		},
	}

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to create deployment: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Deployment created, waiting for it to be ready...\n")

	// Wait for deployment to be ready
	if err := WaitForDeploymentReady(ctx, client, namespace, "backend"); err != nil {
		fmt.Printf("[ERROR] Deployment failed to become ready: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Deployment is ready\n")

	// Create a Service that selects the pods
	fmt.Printf("[DEBUG] Creating Service 'backend-svc' selecting app=backend\n")
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-svc",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "backend",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Name:       "http",
				},
			},
		},
	}

	if _, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to create service: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Service created successfully\n")

	// Wait for service to have ready endpoints
	fmt.Printf("[DEBUG] Waiting for Service to have ready endpoints...\n")
	if err := s.waitForServiceReady(ctx, client, namespace); err != nil {
		fmt.Printf("[ERROR] Service failed to have ready endpoints: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Service has ready endpoints\n")

	// Check endpoint slice status
	if err := s.logEndpointSliceStatus(ctx, client, namespace, "backend-svc"); err != nil {
		fmt.Printf("[WARN] Failed to check EndpointSlice: %v\n", err)
	}

	return nil
}

func (s *ServiceSelectorChange) waitForServiceReady(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitCondition(ctx, 1*time.Minute, func() (bool, error) {
		endpointSlices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set{
				discoveryv1.LabelServiceName: "backend-svc",
			}.String(),
		})
		if err != nil {
			return false, err
		}

		for _, es := range endpointSlices.Items {
			for _, endpoint := range es.Endpoints {
				if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
					return true, nil
				}
			}
		}
		return false, nil
	})
}

func (s *ServiceSelectorChange) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Executing service-selector-change scenario: changing Service selector to not match any pods\n")

	// Get the service
	service, err := client.CoreV1().Services(namespace).Get(ctx, "backend-svc", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to get service: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Current service selector: %v\n", service.Spec.Selector)

	// Change the selector to something that doesn't match any pods
	fmt.Printf("[DEBUG] Changing service selector to app=nonexistent\n")
	service.Spec.Selector = map[string]string{
		"app": "nonexistent",
	}

	_, err = client.CoreV1().Services(namespace).Update(ctx, service, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to update service: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Service selector updated successfully\n")

	// Wait a bit for the update to propagate
	time.Sleep(2 * time.Second)

	return nil
}

func (s *ServiceSelectorChange) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Waiting for service to have no ready endpoints...\n")

	// Wait for service to have no ready endpoints
	return WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		// List EndpointSlices for the service
		endpointSlices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set{
				discoveryv1.LabelServiceName: "backend-svc",
			}.String(),
		})
		if err != nil {
			fmt.Printf("[DEBUG] Error listing EndpointSlices: %v\n", err)
			return false, err
		}

		fmt.Printf("[DEBUG] Found %d EndpointSlice(s) for service 'backend-svc'\n", len(endpointSlices.Items))

		totalReadyEndpoints := 0
		for _, es := range endpointSlices.Items {
			for _, endpoint := range es.Endpoints {
				if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
					totalReadyEndpoints++
				}
			}
		}

		fmt.Printf("[DEBUG] Total ready endpoints: %d\n", totalReadyEndpoints)

		// Also verify deployment is still healthy
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "backend", metav1.GetOptions{})
		if err == nil {
			fmt.Printf("[DEBUG] Deployment status: readyReplicas=%d, availableReplicas=%d\n",
				deployment.Status.ReadyReplicas, deployment.Status.AvailableReplicas)
		}

		// Verify pods are still healthy
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=backend",
		})
		if err == nil {
			readyPods := 0
			for i := range pods.Items {
				pod := &pods.Items[i]
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						readyPods++
						break
					}
				}
			}
			fmt.Printf("[DEBUG] Healthy pods: %d/%d\n", readyPods, len(pods.Items))
		}

		// Service has no ready endpoints when totalReadyEndpoints is 0
		if totalReadyEndpoints == 0 {
			fmt.Printf("[DEBUG] Service has no ready endpoints (condition met)\n")
			return true, nil
		}

		fmt.Printf("[DEBUG] Service still has %d ready endpoint(s), continuing to wait...\n", totalReadyEndpoints)
		return false, nil
	})
}

// logEndpointSliceStatus logs the current status of EndpointSlices for debugging
func (s *ServiceSelectorChange) logEndpointSliceStatus(ctx context.Context, client kubernetes.Interface, namespace, serviceName string) error {
	endpointSlices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{
			discoveryv1.LabelServiceName: serviceName,
		}.String(),
	})
	if err != nil {
		return err
	}

	fmt.Printf("[DEBUG] EndpointSlice status for service '%s': found %d slice(s)\n", serviceName, len(endpointSlices.Items))
	for i, es := range endpointSlices.Items {
		readyCount := 0
		for _, endpoint := range es.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				readyCount++
			}
		}
		fmt.Printf("[DEBUG]   Slice %d: %s - %d/%d endpoints ready\n",
			i+1, es.Name, readyCount, len(es.Endpoints))
	}
	return nil
}

func (s *ServiceSelectorChange) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Cleanup happens via namespace deletion
	return nil
}

func (s *ServiceSelectorChange) ExpectedAnomalies() []ExpectedAnomaly {
	// Only Service should have an anomaly - Deployment is still healthy
	return []ExpectedAnomaly{
		{
			NodeKind:    "Service",
			Category:    "State",
			Type:        "NoReadyEndpoints",
			MinSeverity: "high",
		},
	}
}

func (s *ServiceSelectorChange) ExpectedCausalPath() ExpectedPath {
	// In this scenario, the Service itself is the root cause because
	// the selector changed. There's no upstream chain - the Service
	// change is what caused the problem.
	// We don't expect a full causal path here since Service has no owners.
	return ExpectedPath{
		RootKind:          "Service",
		IntermediateKinds: []string{},
		SymptomKind:       "Service",
		MinConfidence:     0.5,
	}
}

func (s *ServiceSelectorChange) Timeout() time.Duration {
	return 2 * time.Minute
}
