package logprocessing

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"time"
)

// Template represents a log template with stable identifier and metadata.
// Templates are scoped per-namespace for multi-tenant environments.
type Template struct {
	// ID is a SHA-256 hash (hex-encoded) of namespace|pattern for stable cross-client identification.
	// Requirement MINE-03: Templates have stable hashes.
	ID string

	// Namespace is the Kubernetes namespace this template belongs to.
	// Same pattern in different namespaces = different template IDs.
	Namespace string

	// Pattern is the template pattern with wildcards (e.g., "connected to <*>").
	Pattern string

	// Tokens is the tokenized pattern for similarity comparison during auto-merge.
	Tokens []string

	// Count is the occurrence count for pruning low-frequency templates.
	Count int

	// FirstSeen is the timestamp of the first log matching this template.
	FirstSeen time.Time

	// LastSeen is the timestamp of the most recent log matching this template.
	LastSeen time.Time
}

// GenerateTemplateID creates a stable SHA-256 hash for a template.
// The hash is deterministic and consistent across restarts and clients.
//
// Requirement MINE-03: Templates have stable hashes for cross-client consistency.
func GenerateTemplateID(namespace, pattern string) string {
	// Canonicalize input for deterministic hashing
	canonical := fmt.Sprintf("%s|%s", namespace, pattern)

	// SHA-256 hash (deterministic, collision-resistant)
	hash := sha256.Sum256([]byte(canonical))

	// Return hex-encoded hash as template ID (64 characters)
	return hex.EncodeToString(hash[:])
}

// TemplateList is a collection of templates with helper methods.
type TemplateList []Template

// FindByID performs a linear search for a template by ID.
// Linear search is acceptable for small lists (<1000 templates per namespace).
func (tl TemplateList) FindByID(id string) *Template {
	for i := range tl {
		if tl[i].ID == id {
			return &tl[i]
		}
	}
	return nil
}

// SortByCount sorts templates in descending order by occurrence count.
// Used for ranking templates by frequency (most common patterns first).
func (tl TemplateList) SortByCount() {
	sort.Slice(tl, func(i, j int) bool {
		return tl[i].Count > tl[j].Count
	})
}

// SortByLastSeen sorts templates in descending order by last seen timestamp.
// Used for identifying recently active templates.
func (tl TemplateList) SortByLastSeen() {
	sort.Slice(tl, func(i, j int) bool {
		return tl[i].LastSeen.After(tl[j].LastSeen)
	})
}

// FilterByMinCount returns templates with count >= minCount.
// Used for pruning low-frequency templates below occurrence threshold.
func (tl TemplateList) FilterByMinCount(minCount int) TemplateList {
	result := make(TemplateList, 0, len(tl))
	for _, template := range tl {
		if template.Count >= minCount {
			result = append(result, template)
		}
	}
	return result
}
