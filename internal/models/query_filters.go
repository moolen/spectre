package models

// QueryFilters specifies which events to return based on resource dimensions
type QueryFilters struct {
	// Group is the API group filter ("" means match all)
	Group string `json:"group"`

	// Version is the API version filter ("" means match all)
	Version string `json:"version"`

	// Kind is the resource kind filter ("" means match all)
	//
	// Deprecated: Use Kinds for multi-value filtering
	Kind string `json:"kind"`

	// Namespace is the Kubernetes namespace filter ("" means match all)
	//
	// Deprecated: Use Namespaces for multi-value filtering
	Namespace string `json:"namespace"`

	// Kinds is the list of resource kinds to filter (empty = match all)
	// Takes precedence over Kind if both are set
	Kinds []string `json:"kinds,omitempty"`

	// Namespaces is the list of namespaces to filter (empty = match all)
	// Takes precedence over Namespace if both are set
	Namespaces []string `json:"namespaces,omitempty"`
}

// IsEmpty checks if no filters are specified
func (f *QueryFilters) IsEmpty() bool {
	return f.Group == "" && f.Version == "" &&
		f.Kind == "" && f.Namespace == "" &&
		len(f.Kinds) == 0 && len(f.Namespaces) == 0
}

// GetKinds returns the effective list of kinds to filter by
// If Kinds is set, it takes precedence; otherwise, Kind is used
func (f *QueryFilters) GetKinds() []string {
	if len(f.Kinds) > 0 {
		return f.Kinds
	}
	if f.Kind != "" {
		return []string{f.Kind}
	}
	return nil
}

// GetNamespaces returns the effective list of namespaces to filter by
// If Namespaces is set, it takes precedence; otherwise, Namespace is used
func (f *QueryFilters) GetNamespaces() []string {
	if len(f.Namespaces) > 0 {
		return f.Namespaces
	}
	if f.Namespace != "" {
		return []string{f.Namespace}
	}
	return nil
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

	// Check kind filter (supports both single and multi-value)
	kinds := f.GetKinds()
	if len(kinds) > 0 && !containsString(kinds, resource.Kind) {
		return false
	}

	// Check namespace filter (supports both single and multi-value)
	namespaces := f.GetNamespaces()
	if len(namespaces) > 0 && !containsString(namespaces, resource.Namespace) {
		return false
	}

	return true
}

// containsString checks if a slice contains a string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
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
	// Show effective kinds (Kinds takes precedence over Kind)
	kinds := f.GetKinds()
	if len(kinds) > 0 {
		result += "kinds=" + joinStrings(kinds, ",") + " "
	}
	// Show effective namespaces (Namespaces takes precedence over Namespace)
	namespaces := f.GetNamespaces()
	if len(namespaces) > 0 {
		result += "namespaces=" + joinStrings(namespaces, ",") + " "
	}
	if result == "" {
		return "(no filters)"
	}
	return result
}

// joinStrings joins a slice of strings with a separator
func joinStrings(slice []string, sep string) string {
	if len(slice) == 0 {
		return ""
	}
	result := slice[0]
	for i := 1; i < len(slice); i++ {
		result += sep + slice[i]
	}
	return result
}
