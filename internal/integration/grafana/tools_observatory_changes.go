package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// maxLookback is the maximum allowed lookback duration for changes queries.
const maxLookback = 24 * time.Hour

// defaultLookback is the default lookback when not specified.
const defaultLookback = 1 * time.Hour

// maxChanges is the maximum number of changes to return.
const maxChanges = 20

// ObservatoryChangesTool provides recent deployment and config changes for the Orient stage.
// Returns changes from the K8s graph that could explain current anomalies.
type ObservatoryChangesTool struct {
	graphClient     graph.Client
	integrationName string
	logger          *logging.Logger
}

// NewObservatoryChangesTool creates a new observatory changes tool.
func NewObservatoryChangesTool(
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *ObservatoryChangesTool {
	return &ObservatoryChangesTool{
		graphClient:     graphClient,
		integrationName: integrationName,
		logger:          logger,
	}
}

// ObservatoryChangesParams defines input parameters for the observatory_changes tool.
type ObservatoryChangesParams struct {
	Namespace string `json:"namespace,omitempty"` // Optional: filter to namespace
	Lookback  string `json:"lookback,omitempty"`  // Default "1h", max "24h"
}

// ObservatoryChangesResponse contains recent deployment and config changes.
// Per CONTEXT.md: minimal JSON responses, empty results when no changes.
type ObservatoryChangesResponse struct {
	Changes   []Change `json:"changes"`
	Lookback  string   `json:"lookback"`
	Timestamp string   `json:"timestamp"` // RFC3339
}

// Change represents a recent K8s change (deployment, config update, etc).
type Change struct {
	Kind      string `json:"kind"`                // Deployment, HelmRelease, etc.
	Namespace string `json:"namespace"`           // K8s namespace
	Name      string `json:"name"`                // Resource name
	Reason    string `json:"reason"`              // Progressing, Scaled, etc.
	Message   string `json:"message,omitempty"`   // Event message
	Timestamp string `json:"timestamp"`           // RFC3339
}

// Execute runs the observatory_changes tool.
func (t *ObservatoryChangesTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryChangesParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Parse lookback with default
	lookback := defaultLookback
	lookbackStr := "1h"
	if params.Lookback != "" {
		parsed, err := time.ParseDuration(params.Lookback)
		if err != nil {
			return nil, fmt.Errorf("invalid lookback duration %q: %w", params.Lookback, err)
		}
		lookback = parsed
		lookbackStr = params.Lookback
	}

	// Cap at max lookback
	if lookback > maxLookback {
		lookback = maxLookback
		lookbackStr = "24h"
	}

	// Query for recent changes from K8s graph
	changes, err := t.getRecentChanges(ctx, params.Namespace, lookback)
	if err != nil {
		return nil, fmt.Errorf("get recent changes: %w", err)
	}

	return &ObservatoryChangesResponse{
		Changes:   changes,
		Lookback:  lookbackStr,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// getRecentChanges queries the K8s graph for recent deployment and config changes.
// It looks for ChangeEvent nodes where the resource kind indicates deployment activity.
func (t *ObservatoryChangesTool) getRecentChanges(
	ctx context.Context,
	namespace string,
	lookback time.Duration,
) ([]Change, error) {
	lookbackStart := time.Now().Add(-lookback).UnixNano()

	// Query for recent ChangeEvents where the resource kind indicates deployment/config change
	// The ChangeEvent nodes are linked to ResourceIdentity via CHANGED relationship
	// We look for kinds that indicate deployment activity: Deployment, HelmRelease,
	// Kustomization, ConfigMap, Secret, StatefulSet, DaemonSet
	query := `
		MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp > $lookbackStart
		  AND r.kind IN ['Deployment', 'HelmRelease', 'Kustomization', 'ConfigMap', 'Secret', 'StatefulSet', 'DaemonSet', 'ReplicaSet']
		  AND ($namespace = '' OR r.namespace = $namespace)
		  AND (e.configChanged = true OR e.eventType = 'CREATE')
		RETURN r.kind AS kind,
		       r.namespace AS namespace,
		       r.name AS name,
		       e.eventType AS reason,
		       CASE
		           WHEN e.configChanged = true THEN 'Configuration changed'
		           WHEN e.eventType = 'CREATE' THEN 'Resource created'
		           WHEN e.statusChanged = true THEN 'Status updated'
		           ELSE ''
		       END AS message,
		       e.timestamp AS timestamp
		ORDER BY e.timestamp DESC
		LIMIT $maxChanges
	`

	result, err := t.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"lookbackStart": lookbackStart,
			"namespace":     namespace,
			"maxChanges":    maxChanges,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query recent changes: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var changes []Change
	for _, row := range result.Rows {
		change := Change{}

		if idx, ok := colIdx["kind"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Kind = v
			}
		}
		if idx, ok := colIdx["namespace"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Namespace = v
			}
		}
		if idx, ok := colIdx["name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Name = v
			}
		}
		if idx, ok := colIdx["reason"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Reason = v
			}
		}
		if idx, ok := colIdx["message"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Message = v
			}
		}
		if idx, ok := colIdx["timestamp"]; ok && idx < len(row) {
			// Timestamp from graph is in nanoseconds
			var ts int64
			switch v := row[idx].(type) {
			case int64:
				ts = v
			case float64:
				ts = int64(v)
			}
			if ts > 0 {
				change.Timestamp = time.Unix(0, ts).Format(time.RFC3339)
			}
		}

		// Only add if we have a name (basic validation)
		if change.Name != "" {
			changes = append(changes, change)
		}
	}

	return changes, nil
}
