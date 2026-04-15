package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// GraphFindRootCauseTool implements root cause analysis using the graph
type GraphFindRootCauseTool struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewGraphFindRootCauseTool creates a new root cause analysis tool
func NewGraphFindRootCauseTool(graphClient graph.Client) *GraphFindRootCauseTool {
	return &GraphFindRootCauseTool{
		graphClient: graphClient,
		logger:      logging.GetLogger("mcp.tools.root_cause"),
	}
}

// FindRootCauseInput defines the input parameters
type FindRootCauseInput struct {
	ResourceUID      string  `json:"resourceUID"`
	FailureTimestamp int64   `json:"failureTimestamp"` // Unix seconds or nanoseconds
	MaxDepth         int     `json:"maxDepth,omitempty"`
	MinConfidence    float64 `json:"minConfidence,omitempty"`
}

// RootCauseCandidate represents a potential root cause
type RootCauseCandidate struct {
	Resource          GraphResourceInfo       `json:"resource"`
	ChangeEvent       GraphChangeEventInfo    `json:"changeEvent"`
	Evidence          []GraphEvidenceItem     `json:"evidence"`
	ImpactScore       float64            `json:"impactScore"`
	ConfidenceScore   float64            `json:"confidenceScore"`
	TimeLagMs         int64              `json:"timeLagMs"`
}

// FindRootCauseOutput defines the output format
type FindRootCauseOutput struct {
	Candidates          []RootCauseCandidate `json:"candidates"`
	InvestigationPrompt string               `json:"investigationPrompt"`
	QueryExecutionMs    int64                `json:"queryExecutionMs"`
}

// Execute runs the root cause analysis using V2 causality-first approach
func (t *GraphFindRootCauseTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	// Use V2 implementation by default
	return t.ExecuteV2(ctx, input)
}

// ExecuteV2 runs the new causality-first root cause analysis
func (t *GraphFindRootCauseTool) ExecuteV2(ctx context.Context, input json.RawMessage) (*RootCauseAnalysisV2, error) {
	var params FindRootCauseInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Set defaults
	if params.MaxDepth == 0 {
		params.MaxDepth = 5
	}
	if params.MinConfidence == 0 {
		params.MinConfidence = 0.6
	}

	// Normalize timestamp
	failureTimestamp := normalizeTimestamp(params.FailureTimestamp)

	startTime := time.Now()

	// 1. Extract observed symptom (facts only, no inference)
	t.logger.Debug("Extracting observed symptom for resource %s", params.ResourceUID)
	symptom, err := t.extractObservedSymptom(ctx, params.ResourceUID, failureTimestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to extract symptom: %w", err)
	}

	// 2. Build causal chain
	t.logger.Debug("Building causal chain from symptom")
	causalChain, err := t.buildCausalChain(ctx, symptom, failureTimestamp)
	if err != nil {
		t.logger.Debug("Failed to build causal chain: %v, using symptom-only response", err)
		// Fallback: create minimal chain with just the symptom
		causalChain = []CausalStep{
			{
				StepNumber: 1,
				Resource:   symptom.Resource,
				ChangeEvent: &ChangeEventInfo{
					EventID:       "",
					Timestamp:     symptom.ObservedAt,
					EventType:     "OBSERVED",
					ConfigChanged: false,
					StatusChanged: true,
					Description:   "Observed failure",
				},
				RelationshipType: "SYMPTOM",
				Reasoning:        fmt.Sprintf("Direct observation of %s", symptom.SymptomType),
			},
		}
	}

	// If chain is empty, create symptom-only chain
	if len(causalChain) == 0 {
		t.logger.Debug("Empty causal chain, using symptom-only response")
		causalChain = []CausalStep{
			{
				StepNumber: 1,
				Resource:   symptom.Resource,
				ChangeEvent: &ChangeEventInfo{
					EventID:       "",
					Timestamp:     symptom.ObservedAt,
					EventType:     "OBSERVED",
					ConfigChanged: false,
					StatusChanged: true,
					Description:   "Observed failure",
				},
				RelationshipType: "SYMPTOM",
				Reasoning:        fmt.Sprintf("Direct observation of %s", symptom.SymptomType),
			},
		}
	}

	// 3. Identify root cause
	t.logger.Debug("Identifying root cause from chain of %d steps", len(causalChain))
	rootCause, err := t.identifyRootCause(causalChain, failureTimestamp)
	if err != nil {
		t.logger.Debug("Failed to identify root cause: %v, using symptom as root", err)
		// Fallback: use symptom itself as root cause
		rootCause = &RootCauseHypothesis{
			Resource: symptom.Resource,
			ChangeEvent: ChangeEventInfo{
				EventID:       "",
				Timestamp:     symptom.ObservedAt,
				EventType:     "OBSERVED",
				ConfigChanged: false,
				StatusChanged: true,
				Description:   "Observed failure",
			},
			CausationType: "DirectObservation",
			Explanation:   fmt.Sprintf("%s '%s' failed with %s. No causal chain found in graph data.", 
				symptom.Resource.Kind, symptom.Resource.Name, symptom.SymptomType),
			TimeLagMs:     0,
		}
	}

	// 4. Calculate confidence score
	t.logger.Debug("Calculating confidence score")
	confidence := t.calculateConfidence(symptom, causalChain, rootCause)

	// 5. Collect supporting evidence
	t.logger.Debug("Collecting supporting evidence")
	evidence := t.collectSupportingEvidence(causalChain, rootCause)

	// 6. Detect excluded alternatives
	t.logger.Debug("Detecting excluded alternatives")
	excluded := t.detectExcludedAlternatives(ctx, symptom, rootCause, failureTimestamp)

	executionMs := time.Since(startTime).Milliseconds()

	return &RootCauseAnalysisV2{
		Incident: IncidentAnalysis{
			ObservedSymptom: *symptom,
			CausalChain:     causalChain,
			RootCause:       *rootCause,
			Confidence:      confidence,
		},
		SupportingEvidence:   evidence,
		ExcludedAlternatives: excluded,
		QueryMetadata: QueryMetadata{
			QueryExecutionMs: executionMs,
			AlgorithmVersion: "v2.0",
			ExecutedAt:       time.Now(),
		},
	}, nil
}

// ExecuteV1 runs the legacy root cause analysis (deprecated, kept for backward compatibility)
func (t *GraphFindRootCauseTool) ExecuteV1(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params FindRootCauseInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Set defaults
	if params.MaxDepth == 0 {
		params.MaxDepth = 5
	}
	if params.MinConfidence == 0 {
		params.MinConfidence = 0.6
	}

	// Normalize timestamp (convert seconds to nanoseconds if needed)
	failureTimestamp := normalizeTimestamp(params.FailureTimestamp)

	// First, check if there are any events near the requested timestamp
	// If not, find the nearest event and use that as the failure timestamp
	t.logger.Debug("Looking for events near timestamp %d for resource %s", failureTimestamp, params.ResourceUID)
	adjustedTimestamp, err := t.findNearestEventTimestamp(ctx, params.ResourceUID, failureTimestamp)
	if err != nil {
		t.logger.Debug("Failed to find events for resource: %v", err)
		return nil, fmt.Errorf("failed to find events for resource: %w", err)
	}
	
	if adjustedTimestamp != failureTimestamp {
		t.logger.Debug("No events found at requested timestamp %d, using nearest event at %d (diff: %d seconds)", 
			failureTimestamp, adjustedTimestamp, (failureTimestamp-adjustedTimestamp)/1_000_000_000)
		failureTimestamp = adjustedTimestamp
	} else {
		t.logger.Debug("Found events within tolerance window at timestamp %d", failureTimestamp)
	}

	// Build and execute Cypher query
	startTime := time.Now()
	query := graph.FindRootCauseQuery(
		params.ResourceUID,
		failureTimestamp,
		params.MaxDepth,
		params.MinConfidence,
	)

	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	executionMs := time.Since(startTime).Milliseconds()
	t.logger.Debug("Root cause query returned %d rows in %dms", len(result.Rows), executionMs)

	// Parse results into candidates
	candidates, err := t.parseRootCauseCandidates(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse results: %w", err)
	}

	t.logger.Debug("Parsed %d candidates from causality-based query", len(candidates))

	// If no causality-based candidates found, try relationship-based analysis
	if len(candidates) == 0 {
		t.logger.Debug("No causality-based candidates found, trying relationship-based analysis")
		relationshipCandidates, relErr := t.findRelatedChanges(ctx, params.ResourceUID, failureTimestamp)
		if relErr == nil && len(relationshipCandidates) > 0 {
			t.logger.Debug("Found %d candidates from relationship-based analysis", len(relationshipCandidates))
			candidates = relationshipCandidates
		} else if relErr != nil {
			t.logger.Debug("Relationship-based analysis failed: %v", relErr)
		}
	}

	// Generate investigation prompt
	prompt := t.generateInvestigationPrompt(params, candidates)

	return FindRootCauseOutput{
		Candidates:          candidates,
		InvestigationPrompt: prompt,
		QueryExecutionMs:    executionMs,
	}, nil
}

// parseRootCauseCandidates parses query results into structured candidates
func (t *GraphFindRootCauseTool) parseRootCauseCandidates(result *graph.QueryResult) ([]RootCauseCandidate, error) {
	// Use map to deduplicate by resource UID only (not event ID)
	// For root cause analysis, we care about which resource caused the issue,
	// not every individual event. We keep the earliest/most impactful event per resource.
	candidateMap := make(map[string]*RootCauseCandidate)

	t.logger.Debug("Parsing %d result rows", len(result.Rows))

	// Expected columns: causeResource, causeEvent, parentResource, triggers, managesRel
	// Rows contain: [causeResource_node, causeEvent_node, parentResource_node, triggers_edges_array, manages_edge]
	for i, row := range result.Rows {
		t.logger.Debug("Row %d: length=%d, types=[%T, %T, %T, %T, %T]", i, len(row),
			safeIndex(row, 0), safeIndex(row, 1), safeIndex(row, 2), safeIndex(row, 3), safeIndex(row, 4))

		// Debug: print the actual row contents to understand structure
		if len(row) >= 1 {
			if arr, ok := row[0].([]interface{}); ok {
				t.logger.Debug("Row %d col 0 (causeResource): array of length %d", i, len(arr))
			} else {
				t.logger.Debug("Row %d col 0 (causeResource): %T", i, row[0])
			}
		}

		if len(row) < 2 {
			t.logger.Debug("Row %d: skipping, too short", i)
			continue
		}

		// Parse cause resource node
		causeResourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			t.logger.Debug("Row %d: failed to parse causeResource: %v", i, err)
			// Skip rows with parsing errors
			continue
		}
		causeResource := graph.ParseResourceIdentityFromNode(causeResourceProps)

		// Parse cause event node
		causeEventProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil {
			t.logger.Debug("Row %d: failed to parse causeEvent: %v", i, err)
			continue
		}
		causeEvent := graph.ParseChangeEventFromNode(causeEventProps)
		t.logger.Debug("Row %d: successfully parsed causeResource=%s/%s, causeEvent=%s", 
			i, causeResource.Kind, causeResource.Name, causeEvent.ID)

		// Parse parent resource if present (may be null)
		var parentResource *graph.ResourceIdentity
		if len(row) > 2 && row[2] != nil {
			parentProps, err := graph.ParseNodeFromResult(row[2])
			if err == nil {
				parent := graph.ParseResourceIdentityFromNode(parentProps)
				parentResource = &parent
			}
		}

		// Parse TRIGGERED_BY edges if present
		evidence := []GraphEvidenceItem{}
		var maxConfidence float64 = 0.0
		var totalLagMs int64 = 0

		if len(row) > 3 && row[3] != nil {
			// Triggers is an array of edges
			if triggersArray, ok := row[3].([]interface{}); ok {
				for _, triggerVal := range triggersArray {
					edgeType, edgeProps, err := graph.ParseEdgeFromResult(triggerVal)
					if err != nil {
						continue
					}

					if edgeType == "TRIGGERED_BY" {
						triggerEdge := graph.ParseTriggeredByEdge(edgeProps)

						evidence = append(evidence, GraphEvidenceItem{
							Type:        "TRIGGERED_BY",
							Description: triggerEdge.Reason,
							Confidence:  triggerEdge.Confidence,
							Details: map[string]interface{}{
								"lagMs": triggerEdge.LagMs,
							},
						})

						if triggerEdge.Confidence > maxConfidence {
							maxConfidence = triggerEdge.Confidence
						}
						totalLagMs += triggerEdge.LagMs
					}
				}
			}
		}

		// Parse MANAGES edge if present (5th column)
		if len(row) > 4 && row[4] != nil {
			edgeType, edgeProps, err := graph.ParseEdgeFromResult(row[4])
			if err == nil && edgeType == "MANAGES" {
				confidence := graph.GetFloat64Property(edgeProps, "confidence")

				evidence = append(evidence, GraphEvidenceItem{
					Type:        "MANAGES",
					Description: fmt.Sprintf("%s manages this resource (lifecycle control)", causeResource.Kind),
					Confidence:  confidence,
					Details: map[string]interface{}{
						"relationship": "lifecycle_management",
					},
				})

				// MANAGES relationship is very significant for root cause
				if confidence > maxConfidence {
					maxConfidence = confidence
				}

				t.logger.Debug("Row %d: Found MANAGES relationship with confidence %.2f", i, confidence)
			}
		}

		// Create a unique key for deduplication (resource UID only)
		candidateKey := causeResource.UID
		
		// Check if we already have this candidate
		if existing, found := candidateMap[candidateKey]; found {
			// Merge evidence from this event into existing candidate
			t.logger.Debug("Merging duplicate candidate %s (event %s into event %s)",
				candidateKey, causeEvent.ID, existing.ChangeEvent.ID)

			// Add new evidence items (avoiding duplicates)
			for _, newEvidence := range evidence {
				isDuplicate := false

				// Check for duplicates based on type and key attributes
				for _, existingEvidence := range existing.Evidence {
					if existingEvidence.Type == newEvidence.Type {
						switch newEvidence.Type {
						case "TRIGGERED_BY":
							// Duplicate if same lag value
							if existingLag, ok := existingEvidence.Details["lagMs"].(int64); ok {
								if newLag, ok := newEvidence.Details["lagMs"].(int64); ok && existingLag == newLag {
									isDuplicate = true
								}
							}
						case "OWNS", "MANAGES":
							// For OWNS and MANAGES, only keep one of each type
							isDuplicate = true
						}
					}
					if isDuplicate {
						break
					}
				}

				if !isDuplicate {
					existing.Evidence = append(existing.Evidence, newEvidence)
				}
			}

			// Update confidence score if higher
			if maxConfidence > existing.ConfidenceScore {
				existing.ConfidenceScore = maxConfidence
			}

			// Keep the earliest event (most likely the root cause)
			if causeEvent.Timestamp < existing.ChangeEvent.Timestamp {
				existing.ChangeEvent = GraphChangeEventInfo{
					ID:           causeEvent.ID,
					Timestamp:    causeEvent.Timestamp,
					EventType:    causeEvent.EventType,
					Status:       causeEvent.Status,
					ErrorMessage: causeEvent.ErrorMessage,
				}
			}

			// Keep the shortest lag as it's most relevant
			if totalLagMs < existing.TimeLagMs {
				existing.TimeLagMs = totalLagMs
			}

			continue
		}
		
		// Add ownership evidence if parent resource exists
		if parentResource != nil {
			ownershipPath := fmt.Sprintf("%s/%s → %s/%s",
				parentResource.Kind, parentResource.Name,
				causeResource.Kind, causeResource.Name)

			evidence = append(evidence, GraphEvidenceItem{
				Type:        "OWNS",
				Description: fmt.Sprintf("Ownership chain: %s", ownershipPath),
				Details: map[string]interface{}{
					"path": ownershipPath,
				},
			})
		}

		candidate := RootCauseCandidate{
			Resource: GraphResourceInfo{
				UID:       causeResource.UID,
				Kind:      causeResource.Kind,
				Namespace: causeResource.Namespace,
				Name:      causeResource.Name,
			},
			ChangeEvent: GraphChangeEventInfo{
				ID:           causeEvent.ID,
				Timestamp:    causeEvent.Timestamp,
				EventType:    causeEvent.EventType,
				Status:       causeEvent.Status,
				ErrorMessage: causeEvent.ErrorMessage,
			},
			Evidence:        evidence,
			ImpactScore:     causeEvent.ImpactScore,
			ConfidenceScore: maxConfidence,
			TimeLagMs:       totalLagMs,
		}

		candidateMap[candidateKey] = &candidate
	}

	// Convert map to slice and consolidate evidence
	candidates := make([]RootCauseCandidate, 0, len(candidateMap))
	for _, candidate := range candidateMap {
		// Consolidate evidence to reduce duplication
		candidate.Evidence = consolidateEvidence(candidate.Evidence)
		candidates = append(candidates, *candidate)
	}

	// Sort candidates: higher confidence first, then shorter time lag
	for i := 0; i < len(candidates)-1; i++ {
		for j := i + 1; j < len(candidates); j++ {
			// Sort by confidence (descending), then by time lag (ascending)
			if candidates[j].ConfidenceScore > candidates[i].ConfidenceScore ||
				(candidates[j].ConfidenceScore == candidates[i].ConfidenceScore &&
				 candidates[j].TimeLagMs < candidates[i].TimeLagMs) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	return candidates, nil
}

// consolidateEvidence reduces duplicate and similar evidence items
func consolidateEvidence(evidence []GraphEvidenceItem) []GraphEvidenceItem {
	if len(evidence) == 0 {
		return evidence
	}

	// Group evidence by type
	triggeredByItems := []GraphEvidenceItem{}
	ownsItems := []GraphEvidenceItem{}
	managesItems := []GraphEvidenceItem{}
	otherItems := []GraphEvidenceItem{}

	for _, item := range evidence {
		switch item.Type {
		case "TRIGGERED_BY":
			triggeredByItems = append(triggeredByItems, item)
		case "OWNS":
			ownsItems = append(ownsItems, item)
		case "MANAGES":
			managesItems = append(managesItems, item)
		default:
			otherItems = append(otherItems, item)
		}
	}

	consolidated := []GraphEvidenceItem{}

	// For TRIGGERED_BY: keep only the most significant (highest confidence, unique descriptions)
	if len(triggeredByItems) > 0 {
		// Deduplicate by description and confidence
		seen := make(map[string]bool)
		for _, item := range triggeredByItems {
			key := fmt.Sprintf("%s_%.2f", item.Description, item.Confidence)
			if !seen[key] {
				seen[key] = true
				consolidated = append(consolidated, item)
			}
		}

		// Limit to top 3 most significant TRIGGERED_BY items
		if len(consolidated) > 3 {
			// Sort by confidence descending
			for i := 0; i < len(consolidated)-1; i++ {
				for j := i + 1; j < len(consolidated); j++ {
					if consolidated[j].Confidence > consolidated[i].Confidence {
						consolidated[i], consolidated[j] = consolidated[j], consolidated[i]
					}
				}
			}
			consolidated = consolidated[:3]
		}
	}

	// For OWNS: keep only one (the first one, which should be the most relevant)
	if len(ownsItems) > 0 {
		consolidated = append(consolidated, ownsItems[0])
	}

	// For MANAGES: keep only one (should only be one anyway)
	if len(managesItems) > 0 {
		consolidated = append(consolidated, managesItems[0])
	}

	// Add any other types
	consolidated = append(consolidated, otherItems...)

	return consolidated
}

// findNearestEventTimestamp finds the closest ChangeEvent to the requested timestamp
// Returns the nearest event timestamp, or an error if no events exist for the resource
func (t *GraphFindRootCauseTool) findNearestEventTimestamp(ctx context.Context, resourceUID string, requestedTimestamp int64) (int64, error) {
	// First check if there are events within the standard tolerance window (5 minutes)
	toleranceNs := int64(300_000_000_000)
	
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp >= $minTimestamp
			  AND e.timestamp <= $maxTimestamp
			RETURN e.timestamp
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"resourceUID":  resourceUID,
			"minTimestamp": requestedTimestamp - toleranceNs,
			"maxTimestamp": requestedTimestamp + toleranceNs,
		},
	}
	
	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		t.logger.Debug("Error checking events in tolerance window: %v", err)
		return 0, err
	}
	
	t.logger.Debug("Events in tolerance window: %d rows, %d columns", len(result.Rows), len(result.Columns))
	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		t.logger.Debug("First row first column: %v (type: %T)", result.Rows[0][0], result.Rows[0][0])
	}
	
	// If we found an event in the window, use the requested timestamp
	if len(result.Rows) > 0 {
		t.logger.Debug("Found event in tolerance window, using requested timestamp")
		return requestedTimestamp, nil
	}
	
	// Otherwise, find the nearest event (prefer events before the timestamp)
	t.logger.Debug("No events in tolerance window, searching for nearest event")
	nearestQuery := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			WITH e, abs(e.timestamp - $requestedTimestamp) as timeDiff
			ORDER BY timeDiff ASC
			LIMIT 1
			RETURN e.timestamp
		`,
		Parameters: map[string]interface{}{
			"resourceUID":        resourceUID,
			"requestedTimestamp": requestedTimestamp,
		},
	}
	
	nearestResult, err := t.graphClient.ExecuteQuery(ctx, nearestQuery)
	if err != nil {
		t.logger.Debug("Error finding nearest event: %v", err)
		return 0, err
	}
	
	t.logger.Debug("Nearest event query returned %d rows", len(nearestResult.Rows))
	
	if len(nearestResult.Rows) == 0 || len(nearestResult.Rows[0]) == 0 {
		return 0, fmt.Errorf("no ChangeEvents found for resource %s", resourceUID)
	}
	
	// Extract the timestamp from the result
	// FalkorDB sometimes wraps values in an array
	value := nearestResult.Rows[0][0]
	
	// Check if it's wrapped in an array
	if arr, ok := value.([]interface{}); ok && len(arr) > 0 {
		t.logger.Debug("Timestamp wrapped in array, unwrapping")
		value = arr[0]
	}
	
	if ts, ok := value.(int64); ok {
		t.logger.Debug("Found nearest event at timestamp %d (int64)", ts)
		return ts, nil
	} else if ts, ok := value.(float64); ok {
		t.logger.Debug("Found nearest event at timestamp %d (float64)", int64(ts))
		return int64(ts), nil
	}
	
	t.logger.Debug("Unexpected timestamp type: %T, value: %v", value, value)
	return 0, fmt.Errorf("unexpected timestamp type in result: %T", value)
}

// findRelatedChanges looks for changes to related resources when causality links aren't available
func (t *GraphFindRootCauseTool) findRelatedChanges(ctx context.Context, resourceUID string, failureTimestamp int64) ([]RootCauseCandidate, error) {
	// Look 10 minutes before the failure for related changes
	lookbackNs := int64(600_000_000_000)
	minTimestamp := failureTimestamp - lookbackNs
	
	// Query for changes to owned/owner resources around the failure time
	query := graph.GraphQuery{
		Query: `
			MATCH (failedResource:ResourceIdentity {uid: $resourceUID})
			OPTIONAL MATCH (failedResource)-[:OWNS*1..3]->(owned:ResourceIdentity)
			OPTIONAL MATCH (failedResource)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			
			WITH failedResource, collect(DISTINCT owned) + collect(DISTINCT owner) as relatedResources
			UNWIND relatedResources as related
			
			MATCH (related)-[:CHANGED]->(changeEvent:ChangeEvent)
			WHERE changeEvent.timestamp >= $minTimestamp
			  AND changeEvent.timestamp <= $failureTimestamp
			
			OPTIONAL MATCH (related)<-[:OWNS*1..2]-(parent:ResourceIdentity)
			
			RETURN related, changeEvent, parent, [] as triggers
			ORDER BY changeEvent.timestamp DESC
			LIMIT 10
		`,
		Parameters: map[string]interface{}{
			"resourceUID":      resourceUID,
			"failureTimestamp": failureTimestamp,
			"minTimestamp":     minTimestamp,
		},
	}
	
	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	
	return t.parseRootCauseCandidates(result)
}

// safeIndex safely gets an element from a slice, returns nil if out of bounds
func safeIndex(arr []interface{}, i int) interface{} {
	if i < len(arr) {
		return arr[i]
	}
	return nil
}

// generateInvestigationPrompt creates a human-readable investigation prompt
func (t *GraphFindRootCauseTool) generateInvestigationPrompt(input FindRootCauseInput, candidates []RootCauseCandidate) string {
	if len(candidates) == 0 {
		return fmt.Sprintf("No clear root cause found for resource %s. Investigate:\n"+
			"1. Check if resource existed during the failure window\n"+
			"2. Review manual configuration changes not tracked by Spectre\n"+
			"3. Check external dependencies (databases, external APIs)\n"+
			"4. Review cluster-wide events (node failures, network issues)",
			input.ResourceUID)
	}

	topCandidate := candidates[0]
	return fmt.Sprintf("Root cause analysis for resource %s:\n\n"+
		"Most likely cause: %s '%s' (confidence: %.0f%%)\n"+
		"Time lag: %d seconds\n\n"+
		"Recommended investigation steps:\n"+
		"1. Review the %s change at timestamp %d\n"+
		"2. Check if the change introduced a configuration error\n"+
		"3. Verify ownership chain: %s → failed resource\n"+
		"4. Review logs for the %s during this time window\n"+
		"5. Check for related changes in ConfigMaps, Secrets, or PVCs",
		input.ResourceUID,
		topCandidate.Resource.Kind,
		topCandidate.Resource.Name,
		topCandidate.ConfidenceScore*100,
		topCandidate.TimeLagMs/1000,
		topCandidate.Resource.Kind,
		topCandidate.ChangeEvent.Timestamp/1_000_000_000,
		topCandidate.Resource.Kind,
		topCandidate.Resource.Kind,
	)
}
