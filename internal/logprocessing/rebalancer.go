package logprocessing

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/texttheater/golang-levenshtein/levenshtein"
)

// RebalanceConfig configures template lifecycle management parameters.
type RebalanceConfig struct {
	// PruneThreshold is the minimum occurrence count to keep templates.
	// Templates below this threshold are removed during rebalancing.
	// Default: 10 (per user decision from CONTEXT.md)
	PruneThreshold int

	// MergeInterval is how often to run rebalancing.
	// Default: 5 minutes (per user decision from CONTEXT.md)
	MergeInterval time.Duration

	// SimilarityThreshold is the normalized edit distance threshold for merging.
	// Templates with similarity above this threshold are candidates for merging.
	// Default: 0.7 for "loose clustering" (per user decision from CONTEXT.md)
	SimilarityThreshold float64
}

// DefaultRebalanceConfig returns default rebalancing configuration.
func DefaultRebalanceConfig() RebalanceConfig {
	return RebalanceConfig{
		PruneThreshold:      10,
		MergeInterval:       5 * time.Minute,
		SimilarityThreshold: 0.7,
	}
}

// TemplateRebalancer performs periodic template lifecycle management:
// - Prunes low-count templates below occurrence threshold
// - Auto-merges similar templates to handle log format drift
type TemplateRebalancer struct {
	store  *TemplateStore
	config RebalanceConfig
	stopCh chan struct{}
}

// NewTemplateRebalancer creates a new template rebalancer.
func NewTemplateRebalancer(store *TemplateStore, config RebalanceConfig) *TemplateRebalancer {
	return &TemplateRebalancer{
		store:  store,
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic rebalancing.
// Blocks until context is cancelled or Stop is called.
func (tr *TemplateRebalancer) Start(ctx context.Context) error {
	ticker := time.NewTicker(tr.config.MergeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := tr.RebalanceAll(); err != nil {
				log.Printf("Rebalancing error: %v", err)
				// Continue despite error - temporary issues shouldn't halt rebalancing
			}
		case <-ctx.Done():
			return nil
		case <-tr.stopCh:
			return nil
		}
	}
}

// Stop signals the rebalancer to stop gracefully.
func (tr *TemplateRebalancer) Stop() {
	close(tr.stopCh)
}

// RebalanceAll rebalances templates across all namespaces.
// Returns the first error encountered but continues processing other namespaces.
func (tr *TemplateRebalancer) RebalanceAll() error {
	namespaces := tr.store.GetNamespaces()

	var firstErr error
	for _, namespace := range namespaces {
		if err := tr.RebalanceNamespace(namespace); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("Error rebalancing namespace %s: %v", namespace, err)
			// Continue processing other namespaces
		}
	}

	return firstErr
}

// RebalanceNamespace rebalances templates for a single namespace:
// 1. Prunes low-count templates below PruneThreshold
// 2. Auto-merges similar templates above SimilarityThreshold
func (tr *TemplateRebalancer) RebalanceNamespace(namespace string) error {
	// Get namespace templates
	tr.store.mu.RLock()
	ns, exists := tr.store.namespaces[namespace]
	tr.store.mu.RUnlock()

	if !exists {
		return fmt.Errorf("namespace %s not found", namespace)
	}

	// Lock namespace for entire rebalancing operation
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// Step 1: Prune low-count templates
	pruneCount := 0
	for templateID, count := range ns.counts {
		if count < tr.config.PruneThreshold {
			delete(ns.templates, templateID)
			delete(ns.counts, templateID)
			pruneCount++
		}
	}

	if pruneCount > 0 {
		log.Printf("Pruned %d low-count templates from namespace %s (threshold: %d)",
			pruneCount, namespace, tr.config.PruneThreshold)
	}

	// Step 2: Find and merge similar templates
	// Convert templates map to slice for pairwise comparison
	templates := make([]*Template, 0, len(ns.templates))
	for _, template := range ns.templates {
		templates = append(templates, template)
	}

	mergeCount := 0
	// Compare all template pairs
	for i := 0; i < len(templates); i++ {
		for j := i + 1; j < len(templates); j++ {
			// Check if templates[j] still exists (might have been merged in previous iteration)
			if _, exists := ns.templates[templates[j].ID]; !exists {
				continue
			}

			if tr.shouldMerge(templates[i], templates[j]) {
				tr.mergeTemplates(ns, templates[i], templates[j])
				mergeCount++
			}
		}
	}

	if mergeCount > 0 {
		log.Printf("Merged %d similar templates in namespace %s (threshold: %.2f)",
			mergeCount, namespace, tr.config.SimilarityThreshold)
	}

	return nil
}

// shouldMerge determines if two templates should be merged based on similarity.
// Uses normalized edit distance: similarity = 1.0 - (distance / shorter_length)
// Returns true if similarity > threshold.
func (tr *TemplateRebalancer) shouldMerge(t1, t2 *Template) bool {
	// Calculate edit distance between patterns
	distance := editDistance(t1.Pattern, t2.Pattern)

	// Normalize by shorter pattern length
	len1 := len(t1.Pattern)
	len2 := len(t2.Pattern)
	shorter := len1
	if len2 < len1 {
		shorter = len2
	}

	// Avoid division by zero for empty patterns
	if shorter == 0 {
		return false
	}

	// Compute similarity: 1.0 = identical, 0.0 = completely different
	similarity := 1.0 - float64(distance)/float64(shorter)

	return similarity > tr.config.SimilarityThreshold
}

// mergeTemplates merges source template into target template.
// Updates target's count and timestamps, then deletes source.
// Caller must hold ns.mu write lock.
func (tr *TemplateRebalancer) mergeTemplates(ns *NamespaceTemplates, target, source *Template) {
	// Accumulate counts
	target.Count += source.Count

	// Update timestamps: keep earliest FirstSeen, latest LastSeen
	if source.FirstSeen.Before(target.FirstSeen) {
		target.FirstSeen = source.FirstSeen
	}
	if source.LastSeen.After(target.LastSeen) {
		target.LastSeen = source.LastSeen
	}

	// Update counts map
	ns.counts[target.ID] = target.Count

	// Delete source template
	delete(ns.templates, source.ID)
	delete(ns.counts, source.ID)

	log.Printf("Merged template %s into %s (similarity above threshold)", source.ID, target.ID)
}

// editDistance calculates the Levenshtein edit distance between two strings.
func editDistance(s1, s2 string) int {
	return levenshtein.DistanceForStrings([]rune(s1), []rune(s2), levenshtein.DefaultOptions)
}
