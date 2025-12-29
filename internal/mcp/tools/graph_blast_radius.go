package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// GraphBlastRadiusTool calculates the blast radius of a resource change
type GraphBlastRadiusTool struct {
	graphClient graph.Client
}

// NewGraphBlastRadiusTool creates a new blast radius tool
func NewGraphBlastRadiusTool(graphClient graph.Client) *GraphBlastRadiusTool {
	return &GraphBlastRadiusTool{
		graphClient: graphClient,
	}
}

// BlastRadiusInput defines the input parameters
type BlastRadiusInput struct {
	ResourceUID       string   `json:"resourceUID"`
	ChangeTimestamp   int64    `json:"changeTimestamp"` // Unix seconds or nanoseconds
	TimeWindowMs      int64    `json:"timeWindowMs,omitempty"`
	RelationshipTypes []string `json:"relationshipTypes,omitempty"`
}

// ImpactedResource represents a resource affected by the change
type ImpactedResource struct {
	Resource     GraphResourceInfo     `json:"resource"`
	Relationship GraphRelationshipInfo `json:"relationship"`
	GraphImpactEvents []GraphImpactEvent    `json:"impactEvents"`
	Severity     string           `json:"severity"` // critical, high, medium, low
}

// BlastRadiusOutput defines the output format
type BlastRadiusOutput struct {
	TriggerResource     GraphResourceInfo       `json:"triggerResource"`
	TriggerTimestamp    int64              `json:"triggerTimestamp"`
	ImpactedResources   []ImpactedResource `json:"impactedResources"`
	TotalImpacted       int                `json:"totalImpacted"`
	ByKind              map[string]int     `json:"byKind"`
	BySeverity          map[string]int     `json:"bySeverity"`
	InvestigationPrompt string             `json:"investigationPrompt"`
	QueryExecutionMs    int64              `json:"queryExecutionMs"`
}

// Execute runs the blast radius calculation
func (t *GraphBlastRadiusTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params BlastRadiusInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Set defaults
	if params.TimeWindowMs == 0 {
		params.TimeWindowMs = 300000 // 5 minutes
	}
	if len(params.RelationshipTypes) == 0 {
		params.RelationshipTypes = []string{"OWNS", "SELECTS", "SCHEDULED_ON"}
	}

	// Normalize timestamp
	changeTimestamp := normalizeTimestamp(params.ChangeTimestamp)

	// Build and execute query
	startTime := time.Now()
	query := graph.CalculateBlastRadiusQuery(
		params.ResourceUID,
		changeTimestamp,
		params.TimeWindowMs,
		params.RelationshipTypes,
	)

	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	executionMs := time.Since(startTime).Milliseconds()

	// Parse results
	impacted, err := t.parseImpactedResources(result, changeTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %w", err)
	}

	// Calculate statistics
	byKind := make(map[string]int)
	bySeverity := make(map[string]int)
	for _, resource := range impacted {
		byKind[resource.Resource.Kind]++
		bySeverity[resource.Severity]++
	}

	// Generate investigation prompt
	prompt := t.generateBlastRadiusPrompt(params, impacted, byKind)

	return BlastRadiusOutput{
		TriggerResource: GraphResourceInfo{
			UID: params.ResourceUID,
		},
		TriggerTimestamp:    changeTimestamp,
		ImpactedResources:   impacted,
		TotalImpacted:       len(impacted),
		ByKind:              byKind,
		BySeverity:          bySeverity,
		InvestigationPrompt: prompt,
		QueryExecutionMs:    executionMs,
	}, nil
}

// parseImpactedResources parses query results
func (t *GraphBlastRadiusTool) parseImpactedResources(result *graph.QueryResult, triggerTime int64) ([]ImpactedResource, error) {
	impacted := []ImpactedResource{}

	// Expected columns: impacted, impactEvent, relType, distance
	// Rows contain: [impacted_resource_node, impact_event_node, relationship_type_string, distance_int]
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		// Parse impacted resource node
		impactedProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			continue
		}
		impactedResource := graph.ParseResourceIdentityFromNode(impactedProps)

		// Parse impact event node
		impactEventProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil {
			continue
		}
		impactEvent := graph.ParseChangeEventFromNode(impactEventProps)

		// Parse relationship type
		relType := "UNKNOWN"
		if len(row) > 2 && row[2] != nil {
			if rt, ok := row[2].(string); ok {
				relType = rt
			}
		}

		// Parse distance
		distance := 1
		if len(row) > 3 && row[3] != nil {
			if d, ok := row[3].(int64); ok {
				distance = int(d)
			} else if d, ok := row[3].(float64); ok {
				distance = int(d)
			}
		}

		// Calculate lag from trigger
		lagMs := (impactEvent.Timestamp - triggerTime) / 1_000_000

		// Determine severity based on status and error details
		severity := "low"
		if impactEvent.Status == "Error" {
			if len(impactEvent.ContainerIssues) > 0 {
				// Critical if container issues present
				severity = "critical"
			} else {
				severity = "high"
			}
		} else if impactEvent.Status == "Warning" {
			severity = "medium"
		}

		resource := ImpactedResource{
			Resource: GraphResourceInfo{
				UID:       impactedResource.UID,
				Kind:      impactedResource.Kind,
				Namespace: impactedResource.Namespace,
				Name:      impactedResource.Name,
			},
			Relationship: GraphRelationshipInfo{
				Type:     relType,
				Distance: distance,
			},
			GraphImpactEvents: []GraphImpactEvent{
				{
					Timestamp:      impactEvent.Timestamp,
					Status:         impactEvent.Status,
					ErrorMessage:   impactEvent.ErrorMessage,
					LagFromTrigger: lagMs,
				},
			},
			Severity: severity,
		}

		impacted = append(impacted, resource)
	}

	return impacted, nil
}

// generateBlastRadiusPrompt creates investigation guidance
func (t *GraphBlastRadiusTool) generateBlastRadiusPrompt(input BlastRadiusInput, impacted []ImpactedResource, byKind map[string]int) string {
	if len(impacted) == 0 {
		return fmt.Sprintf("No downstream impact detected for resource %s within %d ms. "+
			"This change appears to be isolated or the time window may be too short.",
			input.ResourceUID, input.TimeWindowMs)
	}

	// Build kind summary
	kindSummary := ""
	for kind, count := range byKind {
		kindSummary += fmt.Sprintf("  - %d %s(s)\n", count, kind)
	}

	criticalCount := 0
	for _, r := range impacted {
		if r.Severity == "critical" || r.Severity == "high" {
			criticalCount++
		}
	}

	return fmt.Sprintf("Blast radius analysis for resource %s:\n\n"+
		"Total impacted resources: %d\n"+
		"Critical/High severity: %d\n\n"+
		"Resources affected by kind:\n%s\n"+
		"Investigation steps:\n"+
		"1. Review the triggering change at timestamp %d\n"+
		"2. Identify if the change was intentional (rollout, scaling) or unexpected\n"+
		"3. For critical impacts, check error messages and logs\n"+
		"4. Verify if impacted resources have recovered\n"+
		"5. Consider rollback if impact was unintentional\n"+
		"6. Update deployment procedures to prevent similar cascading failures",
		input.ResourceUID,
		len(impacted),
		criticalCount,
		kindSummary,
		input.ChangeTimestamp,
	)
}
