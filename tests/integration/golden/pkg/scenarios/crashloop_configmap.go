package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CrashLoopConfigMap scenario: Pod crashes due to invalid ConfigMap
type CrashLoopConfigMap struct{}

// NewCrashLoopConfigMap creates the scenario
func NewCrashLoopConfigMap() Scenario {
	return &CrashLoopConfigMap{}
}

func (s *CrashLoopConfigMap) Name() string {
	return "crashloop-configmap"
}

func (s *CrashLoopConfigMap) Description() string {
	return "CrashLoopBackOff due to ConfigMap change introducing invalid env var"
}

func (s *CrashLoopConfigMap) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create ConfigMap with valid config
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-config",
			Namespace: namespace,
		},
		Data: map[string]string{
			"CRASH_ON_START": "false",
		},
	}
	if _, err := client.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Create Deployment that uses the ConfigMap
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-app"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test-app"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "busybox:latest",
							Command: []string{
								"/bin/sh", "-c",
								`if [ "$CRASH_ON_START" = "true" ]; then echo "Crashing due to config!"; exit 1; fi; echo "Running normally"; sleep infinity`,
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "app-config",
										},
									},
								},
							},
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

	// Wait for deployment to be ready
	return WaitForDeploymentReady(ctx, client, namespace, "test-app")
}

func (s *CrashLoopConfigMap) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Update ConfigMap to cause crash
	configMap, err := client.CoreV1().ConfigMaps(namespace).Get(ctx, "app-config", metav1.GetOptions{})
	if err != nil {
		return err
	}

	configMap.Data["CRASH_ON_START"] = "true"
	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// Restart pods to pick up new config
	return client.CoreV1().Pods(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: "app=test-app",
	})
}

func (s *CrashLoopConfigMap) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return WaitForPodCondition(ctx, client, namespace, "app=test-app", func(pod *corev1.Pod) bool {
		if len(pod.Status.ContainerStatuses) == 0 {
			return false
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				return true
			}
		}
		return false
	})
}

func (s *CrashLoopConfigMap) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	return nil // Namespace deletion handles cleanup
}

func (s *CrashLoopConfigMap) ExpectedAnomalies() []ExpectedAnomaly {
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
			Type:        "CrashLoopBackOff",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Pod",
			Category:    "Event",
			Type:        "BackOff",
			MinSeverity: "medium",
		},
	}
}

func (s *CrashLoopConfigMap) ExpectedCausalPath() ExpectedPath {
	// The causal path algorithm finds a direct link from ConfigMap to Pod
	// through the REFERENCES_SPEC edge (Pod references ConfigMap via envFrom/volumeMounts).
	// It doesn't traverse through Deployment/ReplicaSet because that would be
	// following OWNS edges which are materialization edges, not cause-introducing.
	return ExpectedPath{
		RootKind:          "ConfigMap",
		IntermediateKinds: []string{}, // Direct path, no intermediate resources
		SymptomKind:       "Pod",
		MinConfidence:     0.6,
	}
}

func (s *CrashLoopConfigMap) Timeout() time.Duration {
	return 2 * time.Minute
}
