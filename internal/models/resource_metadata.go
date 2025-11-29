package models

import "fmt"

// ResourceMetadata identifies a Kubernetes resource and provides filter dimensions
type ResourceMetadata struct {
	// Group is the API group (e.g., "apps", "" for core API)
	Group string `json:"group"`

	// Version is the API version (e.g., "v1", "v1beta1")
	Version string `json:"version"`

	// Kind is the resource kind (e.g., "Deployment", "Pod")
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace ("" for cluster-scoped resources)
	Namespace string `json:"namespace"`

	// Name is the resource name
	Name string `json:"name"`

	// UID is the unique identifier within the cluster
	UID string `json:"uid"`

	// InvolvedObjectUID links Kubernetes Event objects to the resource they describe
	InvolvedObjectUID string `json:"involvedObjectUid,omitempty"`
}

// Validate checks that the resource metadata has all required fields
func (r *ResourceMetadata) Validate() error {
	if r.Version == "" {
		return NewValidationError("version must not be empty")
	}
	if r.Kind == "" {
		return NewValidationError("kind must not be empty")
	}
	if r.Name == "" {
		return NewValidationError("name must not be empty")
	}
	if r.UID == "" {
		return NewValidationError("uid must not be empty")
	}

	// Namespace can be empty for cluster-scoped resources
	// Group can be empty for core API resources

	return nil
}

// GetFilterKey returns a composite key for indexing this resource
func (r *ResourceMetadata) GetFilterKey() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.Group, r.Version, r.Kind, r.Namespace)
}

// Matches checks if this resource matches the given filters
func (r *ResourceMetadata) Matches(filters QueryFilters) bool {
	// Check group filter
	if filters.Group != "" && r.Group != filters.Group {
		return false
	}

	// Check version filter
	if filters.Version != "" && r.Version != filters.Version {
		return false
	}

	// Check kind filter
	if filters.Kind != "" && r.Kind != filters.Kind {
		return false
	}

	// Check namespace filter
	if filters.Namespace != "" && r.Namespace != filters.Namespace {
		return false
	}

	return true
}

// IsClusterScoped returns true if this is a cluster-scoped resource (no namespace)
func (r *ResourceMetadata) IsClusterScoped() bool {
	return r.Namespace == ""
}
