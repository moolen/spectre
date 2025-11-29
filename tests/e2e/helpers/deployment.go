// Package helpers provides deployment utilities for e2e testing.
package helpers

import (
	"context"
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentBuilder helps construct Kubernetes Deployment objects.
type DeploymentBuilder struct {
	name      string
	namespace string
	image     string
	replicas  int32
	labels    map[string]string
	t         *testing.T
}

// NewDeploymentBuilder creates a new deployment builder.
func NewDeploymentBuilder(t *testing.T, name, namespace string) *DeploymentBuilder {
	return &DeploymentBuilder{
		name:      name,
		namespace: namespace,
		image:     "nginx:latest",
		replicas:  2,
		labels: map[string]string{
			"app": name,
		},
		t: t,
	}
}

// WithImage sets the container image.
func (b *DeploymentBuilder) WithImage(image string) *DeploymentBuilder {
	b.image = image
	return b
}

// WithReplicas sets the number of replicas.
func (b *DeploymentBuilder) WithReplicas(replicas int32) *DeploymentBuilder {
	b.replicas = replicas
	return b
}

// WithLabels sets custom labels.
func (b *DeploymentBuilder) WithLabels(labels map[string]string) *DeploymentBuilder {
	b.labels = labels
	return b
}

// Build constructs the Deployment object.
func (b *DeploymentBuilder) Build() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
			Labels:    b.labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &b.replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: b.labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: b.labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  b.name,
							Image: b.image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("64Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("200m"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// StatefulSetBuilder helps construct Kubernetes StatefulSet objects.
type StatefulSetBuilder struct {
	name      string
	namespace string
	image     string
	replicas  int32
	labels    map[string]string
	t         *testing.T
}

// NewStatefulSetBuilder creates a new statefulset builder.
func NewStatefulSetBuilder(t *testing.T, name, namespace string) *StatefulSetBuilder {
	return &StatefulSetBuilder{
		name:      name,
		namespace: namespace,
		image:     "nginx:latest",
		replicas:  1,
		labels: map[string]string{
			"app": name,
		},
		t: t,
	}
}

// WithImage sets the container image.
func (b *StatefulSetBuilder) WithImage(image string) *StatefulSetBuilder {
	b.image = image
	return b
}

// WithReplicas sets the number of replicas.
func (b *StatefulSetBuilder) WithReplicas(replicas int32) *StatefulSetBuilder {
	b.replicas = replicas
	return b
}

// WithLabels sets custom labels.
func (b *StatefulSetBuilder) WithLabels(labels map[string]string) *StatefulSetBuilder {
	b.labels = labels
	return b
}

// Build constructs the StatefulSet object.
func (b *StatefulSetBuilder) Build() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
			Labels:    b.labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &b.replicas,
			ServiceName: fmt.Sprintf("%s-service", b.name),
			Selector: &metav1.LabelSelector{
				MatchLabels: b.labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: b.labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  b.name,
							Image: b.image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("64Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("200m"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// CreateTestDeployment creates a standard test deployment.
func CreateTestDeployment(ctx context.Context, t *testing.T, k8s *K8sClient, namespace string) (*appsv1.Deployment, error) {
	t.Logf("Creating test deployment in namespace %s", namespace)

	deployment := NewDeploymentBuilder(t, "test-deployment", namespace).
		WithImage("nginx:latest").
		WithReplicas(2).
		Build()

	return k8s.CreateDeployment(ctx, namespace, deployment)
}
