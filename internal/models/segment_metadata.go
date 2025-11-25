package models

// SegmentMetadata enables efficient filtering by allowing queries to skip entire segments
type SegmentMetadata struct {
	// ResourceSummary is a set of (group, version, kind, namespace) tuples present in segment
	ResourceSummary []ResourceMetadata `json:"resourceSummary"`

	// MinTimestamp is the earliest event timestamp in the segment
	MinTimestamp int64 `json:"minTimestamp"`

	// MaxTimestamp is the latest event timestamp in the segment
	MaxTimestamp int64 `json:"maxTimestamp"`

	// NamespaceSet is a set of unique namespaces in the segment
	NamespaceSet map[string]bool `json:"namespaceSet"`

	// KindSet is a set of unique kinds in the segment
	KindSet map[string]bool `json:"kindSet"`

	// CompressionAlgorithm is the algorithm used (e.g., "gzip")
	CompressionAlgorithm string `json:"compressionAlgorithm"`
}

// Validate checks that the metadata is well-formed
func (m *SegmentMetadata) Validate() error {
	// Resource summary must not be empty
	if len(m.ResourceSummary) == 0 {
		return NewValidationError("resourceSummary must not be empty")
	}

	// Namespace set must not be empty
	if len(m.NamespaceSet) == 0 {
		return NewValidationError("namespaceSet must not be empty")
	}

	// Kind set must not be empty
	if len(m.KindSet) == 0 {
		return NewValidationError("kindSet must not be empty")
	}

	// Compression algorithm must be specified
	if m.CompressionAlgorithm == "" {
		return NewValidationError("compressionAlgorithm must not be empty")
	}

	return nil
}

// ContainsNamespace checks if the segment contains events from a given namespace
func (m *SegmentMetadata) ContainsNamespace(namespace string) bool {
	return m.NamespaceSet[namespace]
}

// ContainsKind checks if the segment contains events of a given kind
func (m *SegmentMetadata) ContainsKind(kind string) bool {
	return m.KindSet[kind]
}

// ContainsResource checks if the segment contains events matching the resource metadata
func (m *SegmentMetadata) ContainsResource(resource ResourceMetadata) bool {
	for _, r := range m.ResourceSummary {
		if r.Group == resource.Group &&
			r.Version == resource.Version &&
			r.Kind == resource.Kind &&
			r.Namespace == resource.Namespace {
			return true
		}
	}
	return false
}

// MatchesFilters checks if the segment might contain events matching the filters
// Returns false only if we can definitively rule out all events in this segment
func (m *SegmentMetadata) MatchesFilters(filters QueryFilters) bool {
	// If no filters specified, segment matches
	if filters.IsEmpty() {
		return true
	}

	// If namespace is filtered and segment doesn't contain it, skip
	if filters.Namespace != "" && !m.ContainsNamespace(filters.Namespace) {
		return false
	}

	// If kind is filtered and segment doesn't contain it, skip
	if filters.Kind != "" && !m.ContainsKind(filters.Kind) {
		return false
	}

	// For group and version, we need to check the resource summary
	// This is a more expensive check but necessary for accuracy
	if filters.Group != "" || filters.Version != "" {
		found := false
		for _, r := range m.ResourceSummary {
			if (filters.Group == "" || r.Group == filters.Group) &&
				(filters.Version == "" || r.Version == filters.Version) &&
				(filters.Kind == "" || r.Kind == filters.Kind) &&
				(filters.Namespace == "" || r.Namespace == filters.Namespace) {
				found = true
				break
			}
		}
		return found
	}

	return true
}

// GetNamespaces returns a list of all namespaces in the segment
func (m *SegmentMetadata) GetNamespaces() []string {
	namespaces := make([]string, 0, len(m.NamespaceSet))
	for ns := range m.NamespaceSet {
		namespaces = append(namespaces, ns)
	}
	return namespaces
}

// GetKinds returns a list of all kinds in the segment
func (m *SegmentMetadata) GetKinds() []string {
	kinds := make([]string, 0, len(m.KindSet))
	for k := range m.KindSet {
		kinds = append(kinds, k)
	}
	return kinds
}

// IsValid checks if the metadata is valid
func (m *SegmentMetadata) IsValid() bool {
	return m.Validate() == nil
}
