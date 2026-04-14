package helpers

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFluxControllersReadyReturnsFalseWhenNamespaceExistsWithoutControllers(t *testing.T) {
	clientset := fake.NewSimpleClientset(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "flux-system"},
	})

	if fluxControllersReady(clientset) {
		t.Fatal("expected Flux to be not installed when controllers are missing")
	}
}

func TestFluxControllersReadyReturnsTrueWhenRequiredControllersAreReady(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "flux-system"}},
		readyFluxDeployment("source-controller"),
		readyFluxDeployment("helm-controller"),
	)

	if !fluxControllersReady(clientset) {
		t.Fatal("expected Flux to be installed when required controllers are ready")
	}
}

func readyFluxDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "flux-system",
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
}
