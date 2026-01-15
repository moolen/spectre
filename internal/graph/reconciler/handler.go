package reconciler

import (
	"context"
)

// ReconcileHandler defines the interface for resource-specific reconciliation.
// Handlers are responsible for checking if resources in the graph still exist
// in Kubernetes and reporting which ones should be marked as deleted.
type ReconcileHandler interface {
	// Name returns the handler name for logging and identification.
	Name() string

	// ResourceKind returns the Kubernetes kind this handler reconciles (e.g., "Pod").
	ResourceKind() string

	// Reconcile performs reconciliation for a batch of resources.
	// It checks whether the given resources still exist in Kubernetes
	// and returns the UIDs of resources that should be marked as deleted.
	Reconcile(ctx context.Context, input ReconcileInput) (*ReconcileOutput, error)
}

// ReconcileInput contains resources to reconcile.
type ReconcileInput struct {
	// Resources from the graph that need reconciliation.
	Resources []GraphResource

	// BatchSize limits how many resources to check per cycle.
	BatchSize int
}

// ReconcileOutput contains the results of reconciliation.
type ReconcileOutput struct {
	// ResourcesChecked is the total number of resources checked.
	ResourcesChecked int

	// ResourcesDeleted contains UIDs of resources that no longer exist in Kubernetes.
	ResourcesDeleted []string

	// ResourcesStillExist contains UIDs of resources confirmed to still exist.
	ResourcesStillExist []string

	// Errors contains any non-fatal errors encountered during reconciliation.
	Errors []error
}

// GraphResource represents a resource from the graph database.
type GraphResource struct {
	UID       string
	Kind      string
	APIGroup  string
	Namespace string
	Name      string
}
