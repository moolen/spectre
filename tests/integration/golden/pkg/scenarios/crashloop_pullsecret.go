package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CrashLoopPullSecret scenario: Pod fails due to missing image pull secret
type CrashLoopPullSecret struct{}

// NewCrashLoopPullSecret creates the scenario
func NewCrashLoopPullSecret() Scenario {
	return &CrashLoopPullSecret{}
}

func (s *CrashLoopPullSecret) Name() string {
	return "crashloop-pullsecret"
}

func (s *CrashLoopPullSecret) Description() string {
	return "CrashLoopBackOff due to missing image pull secret"
}

func (s *CrashLoopPullSecret) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create Deployment with non-existent private image
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "private-app",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "private-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "private-app"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "ghcr.io/moolen/does-not-exist:latest",
							Command: []string{
								"/bin/sleep", "infinity",
							},
						},
					},
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}

func (s *CrashLoopPullSecret) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// The scenario is already in failed state from Setup
	// No additional action needed - the image pull will fail
	return nil
}

func (s *CrashLoopPullSecret) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForPodCondition(ctx, client, namespace, "app=private-app", func(pod *corev1.Pod) bool {
		if len(pod.Status.ContainerStatuses) == 0 {
			return false
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
					return true
				}
			}
		}
		return false
	})
}

func (s *CrashLoopPullSecret) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return nil
}

func (s *CrashLoopPullSecret) ExpectedAnomalies() []ExpectedAnomaly {
	// With ResourceCreated detection, we can now trace the root cause to the
	// Deployment creation. When a workload is created with a misconfiguration
	// (like a non-existent image), the CREATE event becomes the root cause.
	return []ExpectedAnomaly{
		{
			NodeKind:    "Deployment",
			Category:    "Change",
			Type:        "ResourceCreated",
			MinSeverity: "low",
		},
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "ImagePullBackOff",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Pod",
			Category:    "Event",
			Type:        "Failed",
			MinSeverity: "medium",
		},
	}
}

func (s *CrashLoopPullSecret) ExpectedCausalPath() ExpectedPath {
	// With ResourceCreated detection, we can now establish the causal path:
	// Deployment (created with bad image) → ReplicaSet → Pod (ImagePullBackOff)
	// The Deployment CREATE event is identified as the root cause.
	// Note: Confidence is ~0.68 due to temporal scoring with extended windows
	return ExpectedPath{
		RootKind:          "Deployment",
		IntermediateKinds: []string{"ReplicaSet"},
		SymptomKind:       "Pod",
		MinConfidence:     0.65,
	}
}

func (s *CrashLoopPullSecret) Timeout() time.Duration {
	return 2 * time.Minute
}
