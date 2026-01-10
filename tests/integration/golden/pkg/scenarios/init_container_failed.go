package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// InitContainerFailed scenario: Pod fails due to init container failure
type InitContainerFailed struct{}

// NewInitContainerFailed creates the scenario
func NewInitContainerFailed() Scenario {
	return &InitContainerFailed{}
}

func (s *InitContainerFailed) Name() string {
	return "init-container-failed"
}

func (s *InitContainerFailed) Description() string {
	return "Pod blocked due to init container failure (non-zero exit code)"
}

func (s *InitContainerFailed) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create ConfigMap that the init container will check
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"INIT_SHOULD_FAIL": "false",
		},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Create Deployment with init container that reads from ConfigMap
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-with-init",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "app-with-init"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "app-with-init"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					InitContainers: []corev1.Container{
						{
							Name:  "init-check",
							Image: "busybox:latest",
							Command: []string{
								"/bin/sh", "-c",
								`if [ "$INIT_SHOULD_FAIL" = "true" ]; then echo "Init container failing as configured"; exit 1; fi; echo "Init container passed"`,
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "init-config",
										},
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:    "app",
							Image:   "busybox:latest",
							Command: []string{"/bin/sleep", "infinity"},
						},
					},
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Wait for deployment to be ready (init container passes initially)
	return WaitForDeploymentReady(ctx, client, namespace, "app-with-init")
}

func (s *InitContainerFailed) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Update ConfigMap to cause init container failure
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, "init-config", metav1.GetOptions{})
	if err != nil {
		return err
	}

	configMap.Data["INIT_SHOULD_FAIL"] = "true"
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// Delete pods to force recreation with new config
	return client.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app=app-with-init",
	})
}

func (s *InitContainerFailed) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForPodCondition(ctx, client, namespace, "app=app-with-init", func(pod *corev1.Pod) bool {
		// Check init container statuses for failure
		for _, ics := range pod.Status.InitContainerStatuses {
			// Check for waiting state (init container restarting)
			if ics.State.Waiting != nil {
				reason := ics.State.Waiting.Reason
				if reason == "CrashLoopBackOff" || reason == "Error" {
					return true
				}
			}
			// Check for terminated with non-zero exit code
			if ics.State.Terminated != nil && ics.State.Terminated.ExitCode != 0 {
				return true
			}
			// Check restart count (indicates init container has failed at least once)
			if ics.RestartCount > 0 {
				return true
			}
		}
		return false
	})
}

func (s *InitContainerFailed) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return nil // Namespace deletion handles cleanup
}

func (s *InitContainerFailed) ExpectedAnomalies() []ExpectedAnomaly {
	return []ExpectedAnomaly{
		{
			NodeKind:    "ConfigMap",
			Category:    "Change",
			Type:        "ConfigChange",
			MinSeverity: "medium",
		},
		{
			NodeKind:    "Pod",
			Category:    "State",
			Type:        "InitContainerFailed",
			MinSeverity: "high",
		},
	}
}

func (s *InitContainerFailed) ExpectedCausalPath() ExpectedPath {
	// ConfigMap change causes init container to fail
	// Direct path via REFERENCES_SPEC edge
	return ExpectedPath{
		RootKind:          "ConfigMap",
		IntermediateKinds: []string{},
		SymptomKind:       "Pod",
		MinConfidence:     0.6,
	}
}

func (s *InitContainerFailed) Timeout() time.Duration {
	return 2 * time.Minute
}
