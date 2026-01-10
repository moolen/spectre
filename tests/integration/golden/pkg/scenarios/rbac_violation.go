package scenarios

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RBACViolation scenario: Pod fails after Role permissions are removed
// This tests that RBAC resource changes (Role/RoleBinding) are properly tracked
// in the causal path when they cause downstream Pod failures.
type RBACViolation struct{}

// NewRBACViolation creates the scenario
func NewRBACViolation() Scenario {
	return &RBACViolation{}
}

func (s *RBACViolation) Name() string {
	return "rbac-violation"
}

func (s *RBACViolation) Description() string {
	return "Pod fails after Role permissions are removed - tests RBAC causal path tracking"
}

func (s *RBACViolation) Setup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Setting up rbac-violation scenario\n")

	// Step 1: Create a service account
	fmt.Printf("[DEBUG] Creating ServiceAccount 'rbac-test-sa' in namespace %s\n", namespace)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rbac-test-sa",
			Namespace: namespace,
		},
	}

	if _, err := client.CoreV1().ServiceAccounts(namespace).Create(ctx, sa, metav1.CreateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to create service account: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] ServiceAccount created successfully\n")

	// Step 2: Create a Role WITH ConfigMap read permissions (initially working)
	fmt.Printf("[DEBUG] Creating Role 'rbac-test-role' with ConfigMap read permissions\n")
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rbac-test-role",
			Namespace: namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				// Allow reading pods and configmaps - the pod will be able to list configmaps
				APIGroups: []string{""}, // Core API group
				Resources: []string{"pods", "configmaps"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	if _, err := client.RbacV1().Roles(namespace).Create(ctx, role, metav1.CreateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to create role: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] Role created successfully (allows listing pods AND configmaps)\n")

	// Step 3: Create a RoleBinding to bind the ServiceAccount to the Role
	fmt.Printf("[DEBUG] Creating RoleBinding 'rbac-test-binding'\n")
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rbac-test-binding",
			Namespace: namespace,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "rbac-test-role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "rbac-test-sa",
				Namespace: namespace,
			},
		},
	}

	if _, err := client.RbacV1().RoleBindings(namespace).Create(ctx, roleBinding, metav1.CreateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to create role binding: %v\n", err)
		return err
	}
	fmt.Printf("[DEBUG] RoleBinding created successfully\n")

	// Step 4: Create a Deployment that continuously checks ConfigMap access
	// The pod runs an infinite loop checking if it can list configmaps
	fmt.Printf("[DEBUG] Creating Deployment 'rbac-checker' with ServiceAccount 'rbac-test-sa'\n")
	replicas := int32(1)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rbac-checker",
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "rbac-checker"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "rbac-checker"},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:            "rbac-test-sa",
					TerminationGracePeriodSeconds: int64Ptr(0),
					Containers: []corev1.Container{
						{
							Name:  "checker",
							Image: "bitnami/kubectl:latest",
							Command: []string{
								"/bin/sh",
								"-c",
								// Infinite loop: check ConfigMap access every 5 seconds
								// Exit with error (causing crash) if access is denied
								`while true; do
									echo "Checking ConfigMap access..."
									if ! kubectl get configmap -n ` + namespace + ` 2>&1; then
										echo "RBAC ERROR: Cannot access ConfigMaps!"
										exit 1
									fi
									echo "ConfigMap access OK"
									sleep 5
								done`,
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
	fmt.Printf("[DEBUG] Deployment created, waiting for pod to be healthy...\n")

	// Step 5: Wait for the pod to be running and healthy (successful ConfigMap access)
	err := WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		opts := metav1.ListOptions{LabelSelector: "app=rbac-checker"}
		pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			return false, err
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.Status.Phase == corev1.PodRunning {
				// Check that container is actually running (not just started)
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.State.Running != nil && cs.Ready {
						fmt.Printf("[DEBUG] Pod %s is running and ready\n", pod.Name)
						return true, nil
					}
				}
			}
		}
		return false, nil
	})

	if err != nil {
		fmt.Printf("[ERROR] Pod did not become healthy: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Setup complete - pod is healthy with ConfigMap access\n")
	return nil
}

func (s *RBACViolation) Execute(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Executing rbac-violation scenario: removing ConfigMap permissions from Role\n")

	// Modify the Role to REMOVE ConfigMap permissions
	// This will cause the running pod to fail on its next check
	role, err := client.RbacV1().Roles(namespace).Get(ctx, "rbac-test-role", metav1.GetOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to get role: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Current role rules: %+v\n", role.Rules)

	// Update rules to only allow pod access (remove configmaps)
	role.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},     // Core API group
			Resources: []string{"pods"}, // REMOVED: configmaps
			Verbs:     []string{"get", "list"},
		},
	}

	if _, err := client.RbacV1().Roles(namespace).Update(ctx, role, metav1.UpdateOptions{}); err != nil {
		fmt.Printf("[ERROR] Failed to update role: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] Role updated - ConfigMap permissions removed\n")
	fmt.Printf("[DEBUG] Pod should fail on next ConfigMap access check...\n")

	return nil
}

func (s *RBACViolation) WaitCondition(ctx context.Context, client kubernetes.Interface, namespace string) error {
	fmt.Printf("[DEBUG] Waiting for pod to crash due to RBAC permission removal...\n")
	fmt.Printf("[DEBUG] Expected condition: Pod should enter CrashLoopBackOff or Failed state\n")

	return WaitCondition(ctx, 2*time.Minute, func() (bool, error) {
		opts := metav1.ListOptions{LabelSelector: "app=rbac-checker"}
		pods, err := client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			fmt.Printf("[DEBUG] Error listing pods: %v\n", err)
			return false, err
		}

		fmt.Printf("[DEBUG] Found %d pod(s) matching selector 'app=rbac-checker'\n", len(pods.Items))

		if len(pods.Items) == 0 {
			fmt.Printf("[DEBUG] No pods found yet, waiting...\n")
			return false, nil
		}

		for i := range pods.Items {
			pod := &pods.Items[i]
			fmt.Printf("[DEBUG] Checking pod %s:\n", pod.Name)
			fmt.Printf("[DEBUG]   phase=%s, reason=%s\n", pod.Status.Phase, pod.Status.Reason)

			// Check container statuses
			if len(pod.Status.ContainerStatuses) > 0 {
				cs := pod.Status.ContainerStatuses[0]
				fmt.Printf("[DEBUG]   Container status:\n")

				if cs.State.Waiting != nil {
					fmt.Printf("[DEBUG]     State: Waiting, Reason=%s, Message=%s\n",
						cs.State.Waiting.Reason, cs.State.Waiting.Message)
					// Pod is in error state due to permission denied
					if cs.State.Waiting.Reason == "CrashLoopBackOff" {
						fmt.Printf("[DEBUG] Pod %s is in CrashLoopBackOff state (RBAC violation detected)\n", pod.Name)
						return true, nil
					}
				} else if cs.State.Running != nil {
					fmt.Printf("[DEBUG]     State: Running (waiting for next check cycle to fail)\n")
				} else if cs.State.Terminated != nil {
					fmt.Printf("[DEBUG]     State: Terminated, Reason=%s, ExitCode=%d, Message=%s\n",
						cs.State.Terminated.Reason, cs.State.Terminated.ExitCode, cs.State.Terminated.Message)
					if cs.State.Terminated.ExitCode != 0 {
						fmt.Printf("[DEBUG] Pod %s terminated with non-zero exit code (RBAC violation)\n", pod.Name)
						return true, nil
					}
				}

				if cs.LastTerminationState.Terminated != nil {
					fmt.Printf("[DEBUG]     Last Termination: Reason=%s, ExitCode=%d, Message=%s\n",
						cs.LastTerminationState.Terminated.Reason,
						cs.LastTerminationState.Terminated.ExitCode,
						cs.LastTerminationState.Terminated.Message)
					if cs.LastTerminationState.Terminated.ExitCode != 0 {
						fmt.Printf("[DEBUG] Pod %s has failed termination (RBAC violation)\n", pod.Name)
						return true, nil
					}
				}
			} else {
				fmt.Printf("[DEBUG]   No container statuses yet\n")
			}

			// Check pod phase
			if pod.Status.Phase == corev1.PodFailed {
				fmt.Printf("[DEBUG] Pod %s has phase=Failed\n", pod.Name)
				return true, nil
			}

			// Check conditions
			for _, cond := range pod.Status.Conditions {
				fmt.Printf("[DEBUG]   Condition: type=%s, status=%s, reason=%s, message=%s\n",
					cond.Type, cond.Status, cond.Reason, cond.Message)
			}

			// Check events for RBAC errors
			events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
			})
			if err == nil && len(events.Items) > 0 {
				fmt.Printf("[DEBUG]   Recent events for pod:\n")
				for _, event := range events.Items {
					fmt.Printf("[DEBUG]     %s: %s - %s\n", event.Type, event.Reason, event.Message)
				}
			}
		}

		fmt.Printf("[DEBUG] No pods in expected failed state yet, continuing to wait...\n")
		return false, nil
	})
}

func (s *RBACViolation) Cleanup(ctx context.Context, client kubernetes.Interface, namespace string) error {
	// RBAC resources cleanup happens via namespace deletion
	return nil
}

func (s *RBACViolation) ExpectedAnomalies() []ExpectedAnomaly {
	// With RBAC causal path support, we expect:
	// 1. Role was modified (root cause - permission removal)
	// 2. Pod BackOff event (symptom - container restarting)
	//
	// Note: CrashLoopBackOff state may not always be present depending on timing
	// of fixture capture. The BackOff event is the reliable indicator.
	//
	// The causal chain should be:
	// Role (RoleModified) -> RoleBinding -> ServiceAccount -> Pod (BackOff)
	return []ExpectedAnomaly{
		{
			NodeKind:    "Role",
			Category:    "Change",
			Type:        "RoleModified",
			MinSeverity: "high",
		},
		{
			NodeKind:    "Pod",
			Category:    "Event",
			Type:        "BackOff",
			MinSeverity: "high",
		},
	}
}

func (s *RBACViolation) ExpectedCausalPath() ExpectedPath {
	// The causal path should trace from the Role modification through RBAC relationships:
	// Role (permission removed) -> RoleBinding -> ServiceAccount -> Pod (CrashLoopBackOff)
	//
	// Edge types:
	// - RoleBinding --BINDS_ROLE--> Role
	// - RoleBinding --GRANTS_TO--> ServiceAccount
	// - Pod --USES_SERVICE_ACCOUNT--> ServiceAccount
	//
	// Note: Confidence is ~0.73 due to temporal scoring with extended windows
	return ExpectedPath{
		RootKind:          "Role",
		IntermediateKinds: []string{"RoleBinding", "ServiceAccount"},
		SymptomKind:       "Pod",
		MinConfidence:     0.70,
	}
}

func (s *RBACViolation) Timeout() time.Duration {
	return 3 * time.Minute
}
