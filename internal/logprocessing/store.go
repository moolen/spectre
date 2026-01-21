package logprocessing

import (
	"errors"
	"strings"
	"sync"
	"time"
)

// Errors returned by TemplateStore operations
var (
	ErrNamespaceNotFound = errors.New("namespace not found")
	ErrTemplateNotFound  = errors.New("template not found")
)

// NamespaceTemplates holds per-namespace template state.
// Each namespace has its own Drain instance and template collection.
type NamespaceTemplates struct {
	// drain is the per-namespace Drain instance for clustering
	drain *DrainProcessor

	// templates maps templateID -> Template for fast lookup
	templates map[string]*Template

	// counts tracks occurrence counts per template (templateID -> count)
	counts map[string]int

	// mu protects templates and counts maps from concurrent access
	mu sync.RWMutex
}

// TemplateStore manages namespace-scoped template storage.
// It provides thread-safe operations for processing logs and retrieving templates.
//
// Design decision from CONTEXT.md: "Templates scoped per-namespace - same log pattern
// in different namespaces = different template IDs"
type TemplateStore struct {
	// namespaces maps namespace name -> NamespaceTemplates
	namespaces map[string]*NamespaceTemplates

	// config is the shared Drain configuration for all namespaces
	config DrainConfig

	// mu protects the namespaces map from concurrent access
	mu sync.RWMutex
}

// NewTemplateStore creates a new template store with the given Drain configuration.
// The config is used to create per-namespace Drain instances on-demand.
func NewTemplateStore(config DrainConfig) *TemplateStore {
	return &TemplateStore{
		namespaces: make(map[string]*NamespaceTemplates),
		config:     config,
	}
}

// Process processes a log message through the full pipeline:
// 1. PreProcess (normalize: lowercase, trim)
// 2. Drain.Train (cluster into template)
// 3. AggressiveMask (mask variables)
// 4. GenerateTemplateID (create stable hash)
// 5. Store/update template with count
//
// Returns the template ID for the processed log.
//
// Design decision from CONTEXT.md: "Masking happens AFTER Drain clustering"
func (ts *TemplateStore) Process(namespace, logMessage string) (string, error) {
	// Get or create namespace
	ns := ts.getOrCreateNamespace(namespace)

	// Step 1: Normalize log (lowercase, trim, extract message from JSON)
	normalized := PreProcess(logMessage)

	// Step 2: Train Drain to get cluster
	cluster := ns.drain.Train(normalized)

	// Step 3: Extract pattern from cluster (format: "id={X} : size={Y} : [pattern]")
	clusterStr := cluster.String()
	pattern := extractPattern(clusterStr)

	// Step 4: Mask variables in cluster template
	// Apply aggressive masking to actual values
	maskedPattern := AggressiveMask(pattern)

	// Step 5: Normalize all variable placeholders for stable template IDs
	// This ensures consistency regardless of when Drain learned the pattern
	normalizedPattern := normalizeDrainWildcards(maskedPattern)

	// Step 6: Generate stable template ID from normalized pattern
	templateID := GenerateTemplateID(namespace, normalizedPattern)

	// Tokenize pattern for similarity comparison during auto-merge
	// Use the semantic masked pattern (not fully normalized) for tokens
	tokens := strings.Fields(maskedPattern)

	// Step 7: Store/update template
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Check if template exists
	if template, exists := ns.templates[templateID]; exists {
		// Update existing template
		template.Count++
		template.LastSeen = time.Now()
		ns.counts[templateID]++
	} else {
		// Create new template
		now := time.Now()
		newTemplate := &Template{
			ID:        templateID,
			Namespace: namespace,
			Pattern:   maskedPattern,
			Tokens:    tokens,
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		ns.templates[templateID] = newTemplate
		ns.counts[templateID] = 1
	}

	return templateID, nil
}

// GetTemplate retrieves a template by namespace and template ID.
// Returns a deep copy to avoid external mutation.
func (ts *TemplateStore) GetTemplate(namespace, templateID string) (*Template, error) {
	// Lock store for reading namespace
	ts.mu.RLock()
	ns, exists := ts.namespaces[namespace]
	ts.mu.RUnlock()

	if !exists {
		return nil, ErrNamespaceNotFound
	}

	// Lock namespace for reading template
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	template, exists := ns.templates[templateID]
	if !exists {
		return nil, ErrTemplateNotFound
	}

	// Return deep copy to prevent external mutation
	copyTemplate := *template
	return &copyTemplate, nil
}

// ListTemplates returns all templates for a namespace, sorted by count descending.
// Returns a deep copy to avoid external mutation.
func (ts *TemplateStore) ListTemplates(namespace string) ([]Template, error) {
	// Lock store for reading namespace
	ts.mu.RLock()
	ns, exists := ts.namespaces[namespace]
	ts.mu.RUnlock()

	if !exists {
		return nil, ErrNamespaceNotFound
	}

	// Lock namespace for reading templates
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	// Build template list
	list := make(TemplateList, 0, len(ns.templates))
	for _, template := range ns.templates {
		// Deep copy to prevent external mutation
		copyTemplate := *template
		list = append(list, copyTemplate)
	}

	// Sort by count descending (most common first)
	list.SortByCount()

	return list, nil
}

// GetNamespaces returns a list of all namespace names currently in the store.
func (ts *TemplateStore) GetNamespaces() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	namespaces := make([]string, 0, len(ts.namespaces))
	for namespace := range ts.namespaces {
		namespaces = append(namespaces, namespace)
	}

	return namespaces
}

// extractPattern extracts the template pattern from Drain cluster string output.
// Drain cluster.String() format: "id={X} : size={Y} : [pattern]"
// Returns just the pattern part.
func extractPattern(clusterStr string) string {
	// Find the last occurrence of " : " which separates metadata from pattern
	lastSep := strings.LastIndex(clusterStr, " : ")
	if lastSep == -1 {
		// No separator found, return as-is (shouldn't happen with normal Drain output)
		return clusterStr
	}

	// Extract pattern (everything after last " : ")
	pattern := clusterStr[lastSep+3:]
	return strings.TrimSpace(pattern)
}

// normalizeDrainWildcards normalizes all variable placeholders to canonical <VAR>.
// This ensures consistent template IDs regardless of when clustering learned the pattern.
//
// Issue: First log gets masked to "connected to <IP>", but once Drain learns the pattern,
// subsequent logs return "connected to <*>". We need consistency across all variable types.
//
// Solution: Normalize ALL placeholders (<*>, <IP>, <UUID>, <NUM>, etc.) to <VAR> for
// template ID generation. The original masked pattern is still stored for display.
func normalizeDrainWildcards(pattern string) string {
	// Replace all common placeholders with canonical <VAR>
	placeholders := []string{
		"<*>", "<IP>", "<UUID>", "<TIMESTAMP>", "<HEX>", "<PATH>",
		"<URL>", "<EMAIL>", "<NUM>", "<K8S_NAME>",
	}

	normalized := pattern
	for _, placeholder := range placeholders {
		normalized = strings.ReplaceAll(normalized, placeholder, "<VAR>")
	}

	return normalized
}

// getOrCreateNamespace retrieves an existing namespace or creates a new one.
// This method handles the double-checked locking pattern for thread-safe lazy initialization.
func (ts *TemplateStore) getOrCreateNamespace(namespace string) *NamespaceTemplates {
	// Fast path: read lock to check if namespace exists
	ts.mu.RLock()
	ns, exists := ts.namespaces[namespace]
	ts.mu.RUnlock()

	if exists {
		return ns
	}

	// Slow path: write lock to create namespace
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Double-check: another goroutine might have created it while we waited
	if ns, exists := ts.namespaces[namespace]; exists {
		return ns
	}

	// Create new namespace with fresh Drain instance
	ns = &NamespaceTemplates{
		drain:     NewDrainProcessor(ts.config),
		templates: make(map[string]*Template),
		counts:    make(map[string]int),
	}
	ts.namespaces[namespace] = ns

	return ns
}
