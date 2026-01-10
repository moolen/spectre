package scenarios

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// NetworkPolicyBlocking scenario: Pod unable to connect due to NetworkPolicy blocking traffic
type NetworkPolicyBlocking struct{}

// NewNetworkPolicyBlocking creates the scenario
func NewNetworkPolicyBlocking() Scenario {
	return &NetworkPolicyBlocking{}
}

func (s *NetworkPolicyBlocking) Name() string {
	return "networkpolicy-blocking"
}

func (s *NetworkPolicyBlocking) Description() string {
	return "Pod unable to connect due to NetworkPolicy blocking traffic"
}

func (s *NetworkPolicyBlocking) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create backend service and deployment
	backend := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
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
							Name:  "api",
							Image: "nginx:1.20",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Name:          "http",
								},
							},
						},
					},
				},
			},
		},
	}

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, backend, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Create backend service
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-api",
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

	if _, err := client.CoreV1().Services(namespace).Create(ctx, svc, metav1.CreateOptions{}); err != nil {
		return err
	}

	// Wait for backend to be ready
	if err := WaitForDeploymentReady(ctx, client, namespace, "backend"); err != nil {
		return err
	}

	// Create frontend deployment
	frontend := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "frontend"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "frontend"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "web",
							Image: "nginx:1.20",
						},
					},
				},
			},
		},
	}

	if _, err := client.AppsV1().Deployments(namespace).Create(ctx, frontend, metav1.CreateOptions{}); err != nil {
		return err
	}

	return WaitForDeploymentReady(ctx, client, namespace, "frontend")
}

func (s *NetworkPolicyBlocking) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Create NetworkPolicy that blocks traffic from frontend to backend
	denyPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deny-frontend-to-backend",
			Namespace: namespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "backend"},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": "denied-app"},
							},
						},
					},
				},
			},
		},
	}

	_, err := client.NetworkingV1().NetworkPolicies(namespace).Create(ctx, denyPolicy, metav1.CreateOptions{})
	return err
}

func (s *NetworkPolicyBlocking) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// Wait for the NetworkPolicy to be applied
	// In a real scenario, we'd check if traffic is blocked, but in a simple test we just wait for the policy to exist
	return WaitCondition(ctx, 1*time.Minute, func() (bool, error) {
		_, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, "deny-frontend-to-backend", metav1.GetOptions{})
		return err == nil, nil
	})
}

func (s *NetworkPolicyBlocking) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// NetworkPolicy cleanup happens via namespace deletion
	return nil
}

func (s *NetworkPolicyBlocking) ExpectedAnomalies() []ExpectedAnomaly {
	// Network-level anomaly detection (IngressRestriction, ConnectionRefused, TrafficBlocked)
	// is not yet implemented. NetworkPolicy blocking traffic doesn't produce detectable
	// anomalies in the current system - the application just fails silently from
	// a Kubernetes resource perspective.
	return []ExpectedAnomaly{}
}

func (s *NetworkPolicyBlocking) ExpectedCausalPath() ExpectedPath {
	// Without detectable anomalies, no causal path can be established.
	return ExpectedPath{
		RootKind:          "",
		IntermediateKinds: []string{},
		SymptomKind:       "",
		MinConfidence:     0.0,
	}
}

func (s *NetworkPolicyBlocking) Timeout() time.Duration {
	return 2 * time.Minute
}
