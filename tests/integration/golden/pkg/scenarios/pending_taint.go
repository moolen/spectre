package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// PendingTaint scenario: Pod pending due to node taint without toleration
type PendingTaint struct{}

// NewPendingTaint creates the scenario
func NewPendingTaint() Scenario {
	return &PendingTaint{}
}

func (s *PendingTaint) Name() string {
	return "pending-taint"
}

func (s *PendingTaint) Description() string {
	return "Pod Pending due to node taint without toleration"
}

func (s *PendingTaint) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Taint all nodes
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		nodeCopy := node.DeepCopy()
		nodeCopy.Spec.Taints = append(nodeCopy.Spec.Taints, corev1.Taint{
			Key:    "foo.example.io/bar",
			Value:  "true",
			Effect: corev1.TaintEffectNoSchedule,
		})
		if _, err := client.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *PendingTaint) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create Deployment without toleration
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unschedulable-app",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "unschedulable-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "unschedulable-app"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "busybox:latest",
							Command: []string{
								"/bin/sleep", "infinity",
							},
						},
					},
					// No tolerations - pod should remain Pending
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

func (s *PendingTaint) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForPodCondition(ctx, client, namespace, "app=unschedulable-app", func(pod *corev1.Pod) bool {
		return pod.Status.Phase == corev1.PodPending
	})
}

func (s *PendingTaint) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Remove taints from all nodes
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		nodeCopy := node.DeepCopy()
		var newTaints []corev1.Taint
		for _, taint := range nodeCopy.Spec.Taints {
			if taint.Key != "foo.example.io/bar" {
				newTaints = append(newTaints, taint)
			}
		}
		nodeCopy.Spec.Taints = newTaints
		if _, err := client.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *PendingTaint) ExpectedAnomalies() []ExpectedAnomaly {
	// Node events are now captured with cluster-scoped resource support.
	// However, the causal graph is built from the symptom (Pod) upward through
	// ownership chains. Nodes are NOT in the ownership chain of Pods, so the
	// TaintAdded anomaly on the Node won't be detected as part of root cause analysis.
	//
	// Future enhancement: Parse FailedScheduling event messages to extract
	// node names and include those nodes in the causal graph analysis.
	//
	// What IS detected:
	// - Pod Unschedulable state when the pod can't be scheduled
	// - FailedScheduling events from the scheduler (includes taint info in message)
	return []ExpectedAnomaly{
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "Unschedulable",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Pod",
			Category:    "Event",
			Type:        "FailedScheduling",
			MinSeverity: "medium",
		},
	}
}

func (s *PendingTaint) ExpectedCausalPath() ExpectedPath {
	// Causal path from Node → Pod cannot be established because:
	// - Nodes are cluster-scoped and don't have owner references to Pods
	// - The Pod is never scheduled (so no .spec.nodeName link exists)
	// - The relationship exists only implicitly via scheduler decisions
	// The taint addition is visible as a Node anomaly, but can't be traced
	// as the root cause of the Pod's scheduling failure through ownership edges.
	return ExpectedPath{
		RootKind:          "",
		IntermediateKinds: []string{},
		SymptomKind:       "",
		MinConfidence:     0.0,
	}
}

func (s *PendingTaint) Timeout() time.Duration {
	return 1 * time.Minute
}
