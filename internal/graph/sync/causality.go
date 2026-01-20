package sync

import (
	"context"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const kindNode = "Node"

// causalityEngine implements the CausalityEngine interface
type causalityEngine struct {
	logger     *logging.Logger
	heuristics []CausalityHeuristic
	maxLag     time.Duration
	minConfidence float64
}

// NewCausalityEngine creates a new causality inference engine
func NewCausalityEngine(maxLag time.Duration, minConfidence float64) CausalityEngine {
	engine := &causalityEngine{
		logger:        logging.GetLogger("graph.sync.causality"),
		maxLag:        maxLag,
		minConfidence: minConfidence,
		heuristics:    []CausalityHeuristic{},
	}

	// Register default heuristics
	engine.registerDefaultHeuristics()

	return engine
}

// InferCausality analyzes events and creates TRIGGERED_BY edges
func (e *causalityEngine) InferCausality(ctx context.Context, events []models.Event) ([]CausalityLink, error) {
	if len(events) < 2 {
		return []CausalityLink{}, nil
	}

	// Sort events by timestamp
	sortedEvents := make([]models.Event, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	links := []CausalityLink{}

	// Analyze pairs of events within the time window
	for i := 0; i < len(sortedEvents); i++ {
		for j := i + 1; j < len(sortedEvents); j++ {
			cause := sortedEvents[i]
			effect := sortedEvents[j]

			// Check if events are within max lag
			lagNs := effect.Timestamp - cause.Timestamp
			if lagNs > e.maxLag.Nanoseconds() {
				break // No point checking further events for this cause
			}

			// Try to find causality
			link, err := e.AnalyzePair(ctx, cause, effect)
			if err != nil {
				e.logger.Warn("Failed to analyze pair (%s, %s): %v", cause.ID, effect.ID, err)
				continue
			}

			if link != nil && link.Confidence >= e.minConfidence {
				links = append(links, *link)
			}
		}
	}

	e.logger.Info("Inferred %d causality links from %d events", len(links), len(events))
	return links, nil
}

// AnalyzePair checks if two events have a causal relationship
func (e *causalityEngine) AnalyzePair(ctx context.Context, cause, effect models.Event) (*CausalityLink, error) {
	// Calculate time lag
	lagNs := effect.Timestamp - cause.Timestamp
	if lagNs <= 0 {
		return nil, nil // Effect must come after cause
	}

	lagMs := lagNs / 1_000_000

	// Try each heuristic
	for _, heuristic := range e.heuristics {
		// Check lag bounds
		if lagMs < heuristic.MinLagMs || lagMs > heuristic.MaxLagMs {
			continue
		}

		// Apply heuristic
		if heuristic.Apply(cause, effect) {
			return &CausalityLink{
				CauseEventID:  cause.ID,
				EffectEventID: effect.ID,
				Confidence:    heuristic.Confidence,
				LagMs:         lagMs,
				Reason:        heuristic.Description,
				HeuristicUsed: heuristic.Name,
			}, nil
		}
	}

	return nil, nil
}

// GetHeuristics returns the configured causality heuristics
func (e *causalityEngine) GetHeuristics() []CausalityHeuristic {
	return e.heuristics
}

// registerDefaultHeuristics registers the default causality heuristics
func (e *causalityEngine) registerDefaultHeuristics() {
	e.heuristics = append(e.heuristics,
		// Heuristic 1: Deployment update → Pod changes
		CausalityHeuristic{
		Name:        "deployment-rollout",
		Description: "Deployment update triggered Pod changes",
		MinLagMs:    0,
		MaxLagMs:    300_000, // 5 minutes
		Confidence:  0.9,
		Apply: func(cause, effect models.Event) bool {
			// Cause: Deployment UPDATE
			// Effect: Pod CREATE/DELETE
			if cause.Resource.Kind == "Deployment" && cause.Type == models.EventTypeUpdate {
				if effect.Resource.Kind == kindPod &&
					(effect.Type == models.EventTypeCreate || effect.Type == models.EventTypeDelete) {
					// Check if they're in the same namespace
					return cause.Resource.Namespace == effect.Resource.Namespace
				}
			}
			return false
		},
	},
	// Heuristic 2: Deployment update → ReplicaSet changes
	CausalityHeuristic{
		Name:        "deployment-replicaset",
		Description: "Deployment update triggered ReplicaSet changes",
		MinLagMs:    0,
		MaxLagMs:    60_000, // 1 minute
		Confidence:  0.9,
		Apply: func(cause, effect models.Event) bool {
			// Cause: Deployment UPDATE
			// Effect: ReplicaSet CREATE/UPDATE
			if cause.Resource.Kind == "Deployment" && cause.Type == models.EventTypeUpdate {
				if effect.Resource.Kind == "ReplicaSet" &&
					(effect.Type == models.EventTypeCreate || effect.Type == models.EventTypeUpdate) {
					// Check if they're in the same namespace
					return cause.Resource.Namespace == effect.Resource.Namespace
				}
			}
			return false
		},
	},
	// Heuristic 3: ReplicaSet update → Pod changes
	CausalityHeuristic{
		Name:        "replicaset-scaling",
		Description: "ReplicaSet update triggered Pod changes",
		MinLagMs:    0,
		MaxLagMs:    60_000, // 1 minute
		Confidence:  0.85,
		Apply: func(cause, effect models.Event) bool {
			if cause.Resource.Kind == "ReplicaSet" && cause.Type == models.EventTypeUpdate {
				if effect.Resource.Kind == kindPod &&
					(effect.Type == models.EventTypeCreate || effect.Type == models.EventTypeDelete || effect.Type == models.EventTypeUpdate) {
					return cause.Resource.Namespace == effect.Resource.Namespace
				}
			}
			return false
		},
	},
	// Heuristic 4: Node issues → Pod evictions
	CausalityHeuristic{
		Name:        "node-pressure-eviction",
		Description: "Node pressure triggered Pod eviction",
		MinLagMs:    0,
		MaxLagMs:    180_000, // 3 minutes
		Confidence:  0.7,
		Apply: func(cause, effect models.Event) bool {
			if cause.Resource.Kind == kindNode && cause.Type == models.EventTypeUpdate {
				if effect.Resource.Kind == kindPod && effect.Type == models.EventTypeDelete {
					// Would need to check Node status for pressure conditions
					// For now, just check timing and resource types
					return true
				}
			}
			return false
		},
	},
	// Heuristic 5: ConfigMap/Secret update → Pod restart
	CausalityHeuristic{
		Name:        "config-change-restart",
		Description: "ConfigMap/Secret update triggered Pod restart",
		MinLagMs:    0,
		MaxLagMs:    120_000, // 2 minutes
		Confidence:  0.75,
		Apply: func(cause, effect models.Event) bool {
			if (cause.Resource.Kind == "ConfigMap" || cause.Resource.Kind == "Secret") &&
				cause.Type == models.EventTypeUpdate {
				if effect.Resource.Kind == kindPod &&
					(effect.Type == models.EventTypeUpdate || effect.Type == models.EventTypeDelete) {
					return cause.Resource.Namespace == effect.Resource.Namespace
				}
			}
			return false
		},
	},
	// Heuristic 6: PVC pending → Pod stuck in Pending
	CausalityHeuristic{
		Name:        "pvc-pending",
		Description: "PVC pending state caused Pod to remain Pending",
		MinLagMs:    0,
		MaxLagMs:    300_000, // 5 minutes
		Confidence:  0.8,
		Apply: func(cause, effect models.Event) bool {
			if cause.Resource.Kind == "PersistentVolumeClaim" {
				if effect.Resource.Kind == kindPod && effect.Type == models.EventTypeUpdate {
					// Would need to check PVC status and Pod status
					return cause.Resource.Namespace == effect.Resource.Namespace
				}
			}
			return false
		},
	},
	// Heuristic 7: Same resource status transitions
	CausalityHeuristic{
		Name:        "same-resource-transition",
		Description: "Status transition within same resource",
		MinLagMs:    100,     // At least 100ms apart
		MaxLagMs:    600_000, // 10 minutes
		Confidence:  0.95,
		Apply: func(cause, effect models.Event) bool {
			// Same resource UID and both are UPDATEs
			if cause.Resource.UID == effect.Resource.UID {
				if cause.Type == models.EventTypeUpdate && effect.Type == models.EventTypeUpdate {
					return true
				}
			}
			return false
		},
	},
	// Heuristic 8: Error propagation (same error message in related resources)
	CausalityHeuristic{
		Name:        "error-propagation",
		Description: "Error propagated between related resources",
		MinLagMs:    0,
		MaxLagMs:    60_000, // 1 minute
		Confidence:  0.65,
		Apply: func(cause, effect models.Event) bool {
			// Both events are errors in same namespace
			// Would need to parse event data for error messages
			if cause.Type == models.EventTypeUpdate && effect.Type == models.EventTypeUpdate {
				return cause.Resource.Namespace == effect.Resource.Namespace
			}
			return false
		},
	},
	// Heuristic 9: Namespace deletion → Resource deletion
	CausalityHeuristic{
		Name:        "namespace-cascade-delete",
		Description: "Namespace deletion triggered resource deletion",
		MinLagMs:    0,
		MaxLagMs:    120_000, // 2 minutes
		Confidence:  0.95,
		Apply: func(cause, effect models.Event) bool {
			if cause.Resource.Kind == "Namespace" && cause.Type == models.EventTypeDelete {
				if effect.Type == models.EventTypeDelete {
					// Check if effect's namespace matches deleted namespace name
					return effect.Resource.Namespace == cause.Resource.Name
				}
			}
			return false
		},
	})
}
