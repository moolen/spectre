package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// StatefulSetRollout scenario: StatefulSet rollout stuck due to failing new replicas
type StatefulSetRollout struct{}

// NewStatefulSetRollout creates the scenario
func NewStatefulSetRollout() Scenario {
	return &StatefulSetRollout{}
}

func (s *StatefulSetRollout) Name() string {
	return "statefulset-rollout-failure"
}

func (s *StatefulSetRollout) Description() string {
	return "StatefulSet rollout failure due to invalid image"
}

func (s *StatefulSetRollout) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create a headless service (required for StatefulSet)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stateful-service",
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector: map[string]string{
				"app": "stateful-app",
			},
			Ports: []corev1.ServicePort{
				{
					Port: 8080,
					Name: "web",
				},
			},
		},
	}

	if _, err := client.CoreV1().Services(namespace).Create(ctx, service, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Create working StatefulSet with 3 replicas
	replicas := int32(3)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stateful-app",
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &replicas,
			ServiceName: "stateful-service",
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "stateful-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "stateful-app"},
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
				},
			},
		},
	}

	if _, err := client.AppsV1().StatefulSets(namespace).Create(ctx, statefulSet, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Wait for StatefulSet to be ready
	return WaitForStatefulSetReady(ctx, client, namespace, "stateful-app")
}

func (s *StatefulSetRollout) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Update to non-existent image
	statefulSet, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "stateful-app", metav1.GetOptions{})
	if err != nil {
		return err
	}

	statefulSet.Spec.Template.Spec.Containers[0].Image = "ghcr.io/moolen/does-not-exist:latest"

	_, err = client.AppsV1().StatefulSets(namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
	return err
}

func (s *StatefulSetRollout) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Wait for StatefulSet to have unavailable replicas
	return WaitCondition(ctx, 3*time.Minute, func() (bool, error) {
		statefulSet, err := client.AppsV1().StatefulSets(namespace).Get(ctx, "stateful-app", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if any replicas are unavailable or pods are in ImagePullBackOff
		return statefulSet.Status.UpdatedReplicas < *statefulSet.Spec.Replicas || statefulSet.Status.AvailableReplicas < *statefulSet.Spec.Replicas, nil
	})
}

func (s *StatefulSetRollout) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// StatefulSet and Service cleanup happens via namespace deletion
	return nil
}

func (s *StatefulSetRollout) ExpectedAnomalies() []ExpectedAnomaly {
	// UpdateRollback detection is not implemented - the StatefulSet controller
	// doesn't explicitly signal a rollback state through standard resource status.
	// Note: ErrImagePull is detected before ImagePullBackOff in the container status
	return []ExpectedAnomaly{
		{
			NodeKind:    "StatefulSet",
			Category:    "Change",
			Type:        "ImageChanged",
			MinSeverity: "medium",
		},
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "ErrImagePull",
			MinSeverity: "high",
		},
	}
}

func (s *StatefulSetRollout) ExpectedCausalPath() ExpectedPath {
	// StatefulSet → Pod ownership path is now traced correctly.
	// The StatefulSet is the root cause (ImageChanged anomaly) and the Pod is the symptom.
	// Unlike Deployment → ReplicaSet → Pod, StatefulSets directly own Pods.
	// Note: Confidence is lower (~0.68) due to temporal scoring with extended windows
	return ExpectedPath{
		RootKind:          "StatefulSet",
		IntermediateKinds: []string{}, // StatefulSets directly own Pods, no intermediate
		SymptomKind:       "Pod",
		MinConfidence:     0.65,
	}
}

func (s *StatefulSetRollout) Timeout() time.Duration {
	return 3 * time.Minute
}

// Helper to wait for StatefulSet to be ready
func WaitForStatefulSetReady(ctx context.Context, client kubernetes.Interface, namespace, name string) error {
	return WaitCondition(ctx, 5*time.Minute, func() (bool, error) {
		ss, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return *ss.Spec.Replicas == ss.Status.ReadyReplicas, nil
	})
}
