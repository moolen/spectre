package scenarios

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NodeEphemeralStoragePressure scenario: Pod eviction due to ephemeral storage pressure
// caused by a Deployment update that changes the container command to write excessive data.
type NodeEphemeralStoragePressure struct{}

// NewNodeEphemeralStoragePressure creates the scenario
func NewNodeEphemeralStoragePressure() Scenario {
	return &NodeEphemeralStoragePressure{}
}

func (s *NodeEphemeralStoragePressure) Name() string {
	return "node-ephemeral-storage-pressure"
}

func (s *NodeEphemeralStoragePressure) Description() string {
	return "Pod eviction due to ephemeral storage pressure caused by Deployment update"
}

func (s *NodeEphemeralStoragePressure) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Setting up node-ephemeral-storage-pressure scenario\n")

	// Create initial Deployment with a pod that does NOT write to ephemeral storage
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "storage-app",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "storage-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "storage-app"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "busybox:latest",
							Command: []string{
								"/bin/sh",
								"-c",
								// Initial command: just sleep, no storage usage
								"echo 'Starting without storage pressure'; sleep 3600",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceEphemeralStorage: resource.MustParse("1Gi"),
								},
								Limits: corev1.ResourceList{
									// Set a low limit so the pod gets evicted quickly
									corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
								},
							},
						},
					},
				},
			},
		},
	}

	fmt.Printf("[DEBUG] Creating initial Deployment 'storage-app' (no storage pressure)\n")
	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to create deployment: %v\n", err)
		return err
	}

	// Wait for the initial pod to be running
	fmt.Printf("[DEBUG] Waiting for initial pod to be running...\n")
	err = WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=storage-app",
		})
		if err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				fmt.Printf("[DEBUG] Initial pod %s is running\n", pod.Name)
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		fmt.Printf("[WARN] Initial pod did not reach running state: %v\n", err)
	}

	return nil
}

func (s *NodeEphemeralStoragePressure) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Executing node-ephemeral-storage-pressure scenario: updating deployment to trigger storage pressure\n")

	// Update the Deployment to write excessive data to /tmp (ephemeral storage)
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "storage-app", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to get deployment: %v\n", err)
		return err
	}

	// Change the command to write excessive data to /tmp
	// This will exceed the ephemeral storage limit and trigger eviction
	deployment.Spec.Template.Spec.Containers[0].Command = []string{
		"/bin/sh",
		"-c",
		// Write ~10GB of data to /tmp, exceeding the 2Gi limit
		"echo 'Starting storage pressure'; dd if=/dev/zero of=/tmp/largefile bs=1M count=10000 || true; sleep 300",
	}

	fmt.Printf("[DEBUG] Updating Deployment to write ~10GB to /tmp (limit: 2Gi)\n")
	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to update deployment: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Deployment updated - new pod should be evicted due to storage pressure\n")

	// Wait a bit for the new pod to be created
	time.Sleep(5 * time.Second)

	// Check deployment status
	deployStatus, err := client.AppsV1().Deployments(namespace).Get(ctx, "storage-app", metav1.GetOptions{})
	if err == nil {
		fmt.Printf("[DEBUG] Deployment status: Available=%d, Ready=%d, Updated=%d\n",
			deployStatus.Status.AvailableReplicas,
			deployStatus.Status.ReadyReplicas,
			deployStatus.Status.UpdatedReplicas)
	}

	return nil
}

func (s *NodeEphemeralStoragePressure) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Waiting for pod eviction due to ephemeral storage pressure...\n")
	return WaitCondition(ctx, 3*time.Minute, func() (bool, error) {
		opts := metav1.ListOptions{LabelSelector: "app=storage-app"}
		pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			fmt.Printf("[DEBUG] Error listing pods: %v\n", err)
			return false, err
		}

		fmt.Printf("[DEBUG] Found %d pod(s) matching selector 'app=storage-app'\n", len(pods.Items))

		if len(pods.Items) == 0 {
			fmt.Printf("[DEBUG] No pods found yet, waiting for pod creation...\n")
			return false, nil
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			fmt.Printf("[DEBUG] Checking pod %s:\n", pod.Name)
			fmt.Printf("[DEBUG]   phase=%s, reason=%s, node=%s\n",
				pod.Status.Phase, pod.Status.Reason, pod.Spec.NodeName)

			// Check conditions
			for _, cond := range pod.Status.Conditions {
				fmt.Printf("[DEBUG]   Condition: type=%s, status=%s, reason=%s\n",
					cond.Type, cond.Status, cond.Reason)
			}

			// Pod is evicted when reason is "Evicted"
			if pod.Status.Reason == "Evicted" {
				fmt.Printf("[DEBUG] ✓ Pod %s is evicted (reason: Evicted)\n", pod.Name)
				return true, nil
			}

			// Also check for pod phase indicating eviction
			if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
				fmt.Printf("[DEBUG] ✓ Pod %s is evicted (phase: Failed, reason: Evicted)\n", pod.Name)
				return true, nil
			}

			// Check container statuses for OOMKilled or similar
			if len(pod.Status.ContainerStatuses) > 0 {
				for j, cs := range pod.Status.ContainerStatuses {
					fmt.Printf("[DEBUG]   Container %d (%s):\n", j+1, cs.Name)
					if cs.State.Terminated != nil {
						fmt.Printf("[DEBUG]     State: Terminated, Reason=%s, ExitCode=%d\n",
							cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
					} else if cs.State.Running != nil {
						fmt.Printf("[DEBUG]     State: Running\n")
					} else if cs.State.Waiting != nil {
						fmt.Printf("[DEBUG]     State: Waiting, Reason=%s\n", cs.State.Waiting.Reason)
					}
				}
			}

			// Check events for eviction
			events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
			})
			if err == nil && len(events.Items) > 0 {
				fmt.Printf("[DEBUG]   Recent events for pod:\n")
				for _, event := range events.Items {
					fmt.Printf("[DEBUG]     %s: %s - %s\n", event.Type, event.Reason, event.Message)
					if event.Reason == "Evicted" || event.Reason == "Eviction" {
						fmt.Printf("[DEBUG]   ✓ Eviction event found!\n")
						return true, nil
					}
				}
			}
		}

		fmt.Printf("[DEBUG] No pods evicted yet, continuing to wait...\n")
		return false, nil
	})
}

func (s *NodeEphemeralStoragePressure) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Deployment cleanup happens via namespace deletion
	return nil
}

func (s *NodeEphemeralStoragePressure) ExpectedAnomalies() []ExpectedAnomaly {
	return []ExpectedAnomaly{
		{
			// The Deployment was updated (command changed)
			NodeKind:    "Deployment",
			Category:    "Change",
			Type:        "WorkloadSpecModified",
			MinSeverity: "medium",
		},
		{
			// Pod was evicted due to ephemeral storage pressure
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "Evicted",
			MinSeverity: "high",
		},
	}
}

func (s *NodeEphemeralStoragePressure) ExpectedCausalPath() ExpectedPath {
	// The causal path traces from Deployment (spec change) through ReplicaSet to Pod (evicted)
	// Deployment update → ReplicaSet → Pod (evicted)
	return ExpectedPath{
		RootKind:          "Deployment",
		IntermediateKinds: []string{"ReplicaSet"},
		SymptomKind:       "Pod",
		MinConfidence:     0.8,
	}
}

func (s *NodeEphemeralStoragePressure) Timeout() time.Duration {
	return 5 * time.Minute
}
