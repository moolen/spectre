package reconciler

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// PodTerminationReconciler checks if Pods in the graph still exist in Kubernetes.
// It detects Pods whose DELETE events were missed and marks them as deleted.
type PodTerminationReconciler struct {
	dynamicClient dynamic.Interface
	logger        *logging.Logger
}

// NewPodTerminationReconciler creates a new PodTerminationReconciler.
func NewPodTerminationReconciler(dynamicClient dynamic.Interface) *PodTerminationReconciler {
	return &PodTerminationReconciler{
		dynamicClient: dynamicClient,
		logger:        logging.GetLogger("reconciler.pod"),
	}
}

// Name implements ReconcileHandler.
func (p *PodTerminationReconciler) Name() string {
	return "PodTerminationReconciler"
}

// ResourceKind implements ReconcileHandler.
func (p *PodTerminationReconciler) ResourceKind() string {
	return "Pod"
}

// Reconcile implements ReconcileHandler.
// It checks if Pods in the graph still exist in Kubernetes.
func (p *PodTerminationReconciler) Reconcile(ctx context.Context, input ReconcileInput) (*ReconcileOutput, error) {
	output := &ReconcileOutput{
		ResourcesDeleted:    []string{},
		ResourcesStillExist: []string{},
		Errors:              []error{},
	}

	podGVR := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}

	// Group resources by namespace for efficient batch checking
	byNamespace := make(map[string][]GraphResource)
	for _, resource := range input.Resources {
		byNamespace[resource.Namespace] = append(byNamespace[resource.Namespace], resource)
	}

	// Check each namespace
	for namespace, resources := range byNamespace {
		// List all pods in the namespace
		existingPods := make(map[string]bool)

		list, err := p.dynamicClient.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Log error but continue with other namespaces
			p.logger.Warn("Failed to list pods in namespace %s: %v", namespace, err)
			output.Errors = append(output.Errors, fmt.Errorf("list pods in %s: %w", namespace, err))
			continue
		}

		// Build set of existing pod UIDs
		for _, item := range list.Items {
			existingPods[string(item.GetUID())] = true
		}

		// Check each resource from graph
		for _, resource := range resources {
			output.ResourcesChecked++

			if existingPods[resource.UID] {
				output.ResourcesStillExist = append(output.ResourcesStillExist, resource.UID)
			} else {
				p.logger.Info("Pod %s/%s (UID: %s) no longer exists in Kubernetes, marking as deleted",
					resource.Namespace, resource.Name, resource.UID)
				output.ResourcesDeleted = append(output.ResourcesDeleted, resource.UID)
			}
		}
	}

	return output, nil
}
