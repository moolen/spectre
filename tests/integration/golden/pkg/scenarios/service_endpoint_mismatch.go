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

// ServiceEndpointMismatch scenario: Service with no ready endpoints
type ServiceEndpointMismatch struct{}

// NewServiceEndpointMismatch creates the scenario
func NewServiceEndpointMismatch() Scenario {
	return &ServiceEndpointMismatch{}
}

func (s *ServiceEndpointMismatch) Name() string {
	return "service-endpoint-mismatch"
}

func (s *ServiceEndpointMismatch) Description() string {
	return "Service with no ready endpoints due to unhealthy pods"
}

func (s *ServiceEndpointMismatch) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Setting up service-endpoint-mismatch scenario\n")

	// Create a Service
	fmt.Printf("[DEBUG] Creating Service 'web-service' in namespace %s\n", namespace)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-service",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "web",
			},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(8080),
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

	// Create working Deployment
	fmt.Printf("[DEBUG] Creating Deployment 'web' with 2 replicas\n")
	replicas := int32(2)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "web"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:1.20",
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
	if err := WaitForDeploymentReady(ctx, client, namespace, "web"); err != nil {
		fmt.Printf("[ERROR] Deployment failed to become ready: %v\n", err)
		return err
	}

	// Check initial endpoint slice status
	fmt.Printf("[DEBUG] Checking initial EndpointSlice status...\n")
	if err := s.logEndpointSliceStatus(ctx, client, namespace, "web-service"); err != nil {
		fmt.Printf("[WARN] Failed to check EndpointSlice: %v\n", err)
	}

	return nil
}

func (s *ServiceEndpointMismatch) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Executing service-endpoint-mismatch scenario: scaling down deploymentge\n")

	// Get deployment
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "web", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to get deployment: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Current deployment: replicas=%d, image=%s\n",
		*deployment.Spec.Replicas, deployment.Spec.Template.Spec.Containers[0].Image)

	// Scale down to 0 to terminate old pods first
	fmt.Printf("[DEBUG] Scaling down deployment to 0 replicas to terminate old pods...\n")
	zeroReplicas := int32(0)
	deployment.Spec.Replicas = &zeroReplicas
	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to scale down deployment: %v\n", err)
		return err
	}

	// Wait for pods to be terminated
	fmt.Printf("[DEBUG] Waiting for old pods to be terminated...\n")
	maxWait := 30 * time.Second
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=web",
		})
		if err == nil && len(pods.Items) == 0 {
			fmt.Printf("[DEBUG] All old pods terminated\n")
			break
		}
		time.Sleep(1 * time.Second)
	}

	return nil
}

func (s *ServiceEndpointMismatch) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Waiting for service to have no ready endpoints...\n")
	fmt.Printf("[DEBUG] Expected condition: EndpointSlice should have no ready endpoints\n")

	// Wait for service to have no ready endpoints using EndpointSlice
	return WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		// List EndpointSlices for the service
		endpointSlices, err := client.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.Set{
				discoveryv1.LabelServiceName: "web-service",
			}.String(),
		})
		if err != nil {
			fmt.Printf("[DEBUG] Error listing EndpointSlices: %v\n", err)
			return false, err
		}

		fmt.Printf("[DEBUG] Found %d EndpointSlice(s) for service 'web-service'\n", len(endpointSlices.Items))

		totalReadyEndpoints := 0
		totalNotReadyEndpoints := 0

		for i, es := range endpointSlices.Items {
			fmt.Printf("[DEBUG] EndpointSlice %d: name=%s\n", i+1, es.Name)

			readyCount := 0
			notReadyCount := 0

			for _, endpoint := range es.Endpoints {
				nodeName := "<nil>"
				if endpoint.NodeName != nil {
					nodeName = *endpoint.NodeName
				}

				if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
					readyCount++
					totalReadyEndpoints++
					if len(endpoint.Addresses) > 0 {
						fmt.Printf("[DEBUG]   Ready endpoint: %s (node: %s)\n",
							endpoint.Addresses[0], nodeName)
					} else {
						fmt.Printf("[DEBUG]   Ready endpoint: <no address> (node: %s)\n", nodeName)
					}
				} else {
					notReadyCount++
					totalNotReadyEndpoints++
					readyStatus := "not ready"
					if endpoint.Conditions.Ready != nil {
						readyStatus = fmt.Sprintf("ready=%v", *endpoint.Conditions.Ready)
					}
					if len(endpoint.Addresses) > 0 {
						fmt.Printf("[DEBUG]   Not ready endpoint: %s (node: %s, %s)\n",
							endpoint.Addresses[0], nodeName, readyStatus)
					} else {
						fmt.Printf("[DEBUG]   Not ready endpoint: <no address> (node: %s, %s)\n",
							nodeName, readyStatus)
					}
				}
			}

			fmt.Printf("[DEBUG]   Summary: %d ready, %d not ready\n", readyCount, notReadyCount)
		}

		fmt.Printf("[DEBUG] Total: %d ready endpoints, %d not ready endpoints\n",
			totalReadyEndpoints, totalNotReadyEndpoints)

		// Service has no ready addresses when totalReadyEndpoints is 0
		if totalReadyEndpoints == 0 {
			fmt.Printf("[DEBUG] ✓ Service has no ready endpoints (condition met)\n")
			return true, nil
		}

		fmt.Printf("[DEBUG] Service still has %d ready endpoint(s), continuing to wait...\n", totalReadyEndpoints)

		// Detailed pod analysis
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=web",
		})
		if err == nil {
			fmt.Printf("[DEBUG] Checking %d pod(s) for deployment:\n", len(pods.Items))
			oldPods := 0
			newPods := 0
			readyPods := 0
			failingPods := 0

			// Get deployment to see current generation
			deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "web", metav1.GetOptions{})
			if err == nil {
				currentGen := deployment.Generation
				observedGen := deployment.Status.ObservedGeneration
				fmt.Printf("[DEBUG] Deployment generation: current=%d, observed=%d, readyReplicas=%d, unavailableReplicas=%d\n",
					currentGen, observedGen, deployment.Status.ReadyReplicas, deployment.Status.UnavailableReplicas)
			}

			for _, pod := range pods.Items {
				if gen, ok := pod.Labels["pod-template-generation"]; ok {
					fmt.Printf("[DEBUG]   Pod %s: phase=%s, reason=%s, generation=%s\n",
						pod.Name, pod.Status.Phase, pod.Status.Reason, gen)
				} else {
					fmt.Printf("[DEBUG]   Pod %s: phase=%s, reason=%s\n",
						pod.Name, pod.Status.Phase, pod.Status.Reason)
				}

				isReady := false
				if len(pod.Status.ContainerStatuses) > 0 {
					cs := pod.Status.ContainerStatuses[0]
					fmt.Printf("[DEBUG]     Container: name=%s, ready=%v\n", cs.Name, cs.Ready)

					if cs.Ready {
						readyPods++
						isReady = true
						oldPods++
					} else {
						if cs.State.Waiting != nil {
							fmt.Printf("[DEBUG]     Container waiting: reason=%s, message=%s\n",
								cs.State.Waiting.Reason, cs.State.Waiting.Message)
							if cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull" {
								failingPods++
								newPods++
							}
						} else if cs.State.Running != nil {
							fmt.Printf("[DEBUG]     Container running but not ready\n")
						} else if cs.State.Terminated != nil {
							fmt.Printf("[DEBUG]     Container terminated: reason=%s, exitCode=%d\n",
								cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
						}
					}

					// Check image being used
					fmt.Printf("[DEBUG]     Image: %s\n", pod.Spec.Containers[0].Image)
				}

				// Check if pod has endpoints
				podAddress := ""
				if pod.Status.PodIP != "" {
					podAddress = pod.Status.PodIP
				}
				fmt.Printf("[DEBUG]     PodIP: %s, Ready: %v\n", podAddress, isReady)
			}

			fmt.Printf("[DEBUG] Pod summary: %d old (ready), %d new (failing), %d total ready, %d total failing\n",
				oldPods, newPods, readyPods, failingPods)

			// Check ReplicaSet status
			replicaSets, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=web",
			})
			if err == nil {
				fmt.Printf("[DEBUG] Found %d ReplicaSet(s):\n", len(replicaSets.Items))
				for _, rs := range replicaSets.Items {
					fmt.Printf("[DEBUG]   RS %s: replicas=%d, readyReplicas=%d, availableReplicas=%d, generation=%d\n",
						rs.Name, rs.Status.Replicas, rs.Status.ReadyReplicas, rs.Status.AvailableReplicas, rs.Generation)
				}
			}

			// Check events for deployment
			events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: "involvedObject.name=web,involvedObject.kind=Deployment",
			})
			if err == nil {
				fmt.Printf("[DEBUG] Recent deployment events:\n")
				for _, event := range events.Items {
					fmt.Printf("[DEBUG]   %s: %s - %s\n", event.Type, event.Reason, event.Message)
				}
			}
		}

		return false, nil
	})
}

// logEndpointSliceStatus logs the current status of EndpointSlices for debugging
func (s *ServiceEndpointMismatch) logEndpointSliceStatus(ctx context.Context, client kubernetes.Interface, namespace, serviceName string) error {
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

func (s *ServiceEndpointMismatch) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Cleanup happens via namespace deletion
	return nil
}

func (s *ServiceEndpointMismatch) ExpectedAnomalies() []ExpectedAnomaly {
	// Service has NoReadyEndpoints because all selected Pods have ImagePullBackOff
	return []ExpectedAnomaly{
		{
			NodeKind:    "Service",
			Category:    "State",
			Type:        "NoReadyEndpoints",
			MinSeverity: "high",
		},
	}
}

func (s *ServiceEndpointMismatch) ExpectedCausalPath() ExpectedPath {
	// Note: Confidence is ~0.66 due to temporal scoring with extended windows
	return ExpectedPath{
		RootKind:          "Service",
		IntermediateKinds: []string{},
		SymptomKind:       "Service",
		MinConfidence:     0.65,
	}
}

func (s *ServiceEndpointMismatch) Timeout() time.Duration {
	return 2 * time.Minute
}
