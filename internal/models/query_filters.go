package models

// QueryFilters specifies which events to return based on resource dimensions
type QueryFilters struct {
	// Group is the API group filter ("" means match all)
	Group string `json:"group"`

	// Version is the API version filter ("" means match all)
	Version string `json:"version"`

	// Kind is the resource kind filter ("" means match all)
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace filter ("" means match all)
	Namespace string `json:"namespace"`
}

// IsEmpty checks if no filters are specified
func (f *QueryFilters) IsEmpty() bool {
	return f.Group == "" && f.Version == "" && f.Kind == "" && f.Namespace == ""
}

// Matches checks if a resource matches all specified filters
// Empty filters match all resources (AND logic: all specified filters must match)
func (f *QueryFilters) Matches(resource ResourceMetadata) bool {
	// Check group filter
	if f.Group != "" && resource.Group != f.Group {
		return false
	}

	// Check version filter
	if f.Version != "" && resource.Version != f.Version {
		return false
	}

	// Check kind filter
	if f.Kind != "" && resource.Kind != f.Kind {
		return false
	}

	// Check namespace filter
	if f.Namespace != "" && resource.Namespace != f.Namespace {
		return false
	}

	return true
}

// String returns a string representation of the filters
func (f *QueryFilters) String() string {
	result := ""
	if f.Group != "" {
		result += "group=" + f.Group + " "
	}
	if f.Version != "" {
		result += "version=" + f.Version + " "
	}
	if f.Kind != "" {
		result += "kind=" + f.Kind + " "
	}
	if f.Namespace != "" {
		result += "namespace=" + f.Namespace + " "
	}
	if result == "" {
		return "(no filters)"
	}
	return result
}
