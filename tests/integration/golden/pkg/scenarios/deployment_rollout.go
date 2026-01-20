package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentRollout scenario: Deployment rollout stuck due to failing new replicas
type DeploymentRollout struct{}

// NewDeploymentRollout creates the scenario
func NewDeploymentRollout() Scenario {
	return &DeploymentRollout{}
}

func (s *DeploymentRollout) Name() string {
	return "deployment-rollout-stuck"
}

func (s *DeploymentRollout) Description() string {
	return "Deployment rollout stuck due to failing new replicas"
}

func (s *DeploymentRollout) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create working deployment with 3 replicas
	replicas := int32(3)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rolling-app",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "rolling-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "rolling-app"},
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

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Wait for deployment to be ready
	return WaitForDeploymentReady(ctx, client, namespace, "rolling-app")
}

func (s *DeploymentRollout) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Update to non-existent image
	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "rolling-app", metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Containers[0].Image = "ghcr.io/moolen/does-not-exist:latest"

	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	return err
}

func (s *DeploymentRollout) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	var firstUnavailableTime time.Time
	const unavailableMinDuration = 15 * time.Second // Wait for stuck state to persist

	return WaitCondition(ctx, 3*time.Minute, func() (bool, error) {
		deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, "rolling-app", metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check for progressing condition being false (definitive stuck state)
		for _, condition := range deployment.Status.Conditions {
			if condition.Type == appsv1.DeploymentProgressing {
				if condition.Status == corev1.ConditionFalse || condition.Reason == "ProgressDeadlineExceeded" {
					return true, nil
				}
			}
		}

		// Check if unavailable replicas persist for minimum duration
		if deployment.Status.UnavailableReplicas > 0 {
			if firstUnavailableTime.IsZero() {
				firstUnavailableTime = time.Now()
			}
			// Return true only if unavailable replicas have persisted
			return time.Since(firstUnavailableTime) >= unavailableMinDuration, nil
		} else {
			// Reset timer if replicas become available again
			firstUnavailableTime = time.Time{}
		}

		return false, nil
	})
}

func (s *DeploymentRollout) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return nil
}

func (s *DeploymentRollout) ExpectedAnomalies() []ExpectedAnomaly {
	return []ExpectedAnomaly{
		{
			NodeKind:    "Deployment",
			Category:    "Change",
			Type:        "ImageChange",
			MinSeverity: "medium",
		},
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "ImagePullBackOff",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Deployment",
			Category:    "State",
			Type:        "RolloutStuck",
			MinSeverity: "high",
		},
	}
}

func (s *DeploymentRollout) ExpectedCausalPath() ExpectedPath {
	// Note: Confidence is lower (~0.67) due to temporal scoring with extended windows
	return ExpectedPath{
		RootKind:          "Deployment",
		IntermediateKinds: []string{"ReplicaSet"},
		SymptomKind:       "Pod",
		MinConfidence:     0.65,
	}
}

func (s *DeploymentRollout) Timeout() time.Duration {
	return 3 * time.Minute
}
