package tools

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// Phase 2: Core Logic Implementation
// This file implements the causality-first analysis logic for root cause determination.

// extractObservedSymptom extracts facts from the failure event (no inference)
func (t *GraphFindRootCauseTool) extractObservedSymptom(
	ctx context.Context,
	resourceUID string,
	failureTimestamp int64,
) (*ObservedSymptom, error) {
	// Query for the ChangeEvent at the failure timestamp
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp <= $failureTimestamp + $tolerance
			  AND e.timestamp >= $failureTimestamp - $tolerance
			RETURN e
			ORDER BY abs(e.timestamp - $failureTimestamp) ASC
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"resourceUID":      resourceUID,
			"failureTimestamp": failureTimestamp,
			"tolerance":        int64(300_000_000_000), // 5 minutes
		},
	}

	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query failure event: %w", err)
	}

	if len(result.Rows) == 0 {
		// No event found in the tolerance window - provide helpful diagnostics
		// Query for any events for this resource
		diagnosticQuery := graph.GraphQuery{
			Query: `
				MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
				RETURN e.timestamp
				ORDER BY e.timestamp ASC
				LIMIT 5
			`,
			Parameters: map[string]interface{}{
				"resourceUID": resourceUID,
			},
		}

		diagnosticResult, diagErr := t.graphClient.ExecuteQuery(ctx, diagnosticQuery)
		if diagErr == nil && len(diagnosticResult.Rows) > 0 {
			// Get first event timestamp
			if firstEventTS, ok := diagnosticResult.Rows[0][0].(int64); ok {
				firstEventTime := time.Unix(0, firstEventTS)
				providedTime := time.Unix(0, failureTimestamp)
				diffSeconds := (firstEventTS - failureTimestamp) / 1_000_000_000

				if diffSeconds > 300 {
					// Timestamp is too early
					return nil, fmt.Errorf(
						"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
							"First event for this resource occurred at %s (%d), which is %d seconds later. "+
							"Try using timestamp: %d",
						failureTimestamp, providedTime.Format(time.RFC3339),
						firstEventTime.Format(time.RFC3339), firstEventTS,
						diffSeconds, firstEventTS,
					)
				} else if diffSeconds < -300 {
					// Timestamp is too late
					return nil, fmt.Errorf(
						"no ChangeEvent found within ±5 minutes of timestamp %d (%s). "+
							"First event for this resource occurred at %s (%d), which is %d seconds earlier",
						failureTimestamp, providedTime.Format(time.RFC3339),
						firstEventTime.Format(time.RFC3339), firstEventTS,
						-diffSeconds,
					)
				}
			}
		}

		return nil, fmt.Errorf("no ChangeEvent found for resource %s at timestamp %d", resourceUID, failureTimestamp)
	}

	// Parse the event node
	eventProps, err := graph.ParseNodeFromResult(result.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse event node: %w", err)
	}
	event := graph.ParseChangeEventFromNode(eventProps)

	// Get resource identity
	resourceQuery := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			RETURN r
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
		},
	}

	resourceResult, err := t.graphClient.ExecuteQuery(ctx, resourceQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource: %w", err)
	}

	if len(resourceResult.Rows) == 0 {
		return nil, fmt.Errorf("resource %s not found", resourceUID)
	}

	resourceProps, err := graph.ParseNodeFromResult(resourceResult.Rows[0][0])
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource node: %w", err)
	}
	resource := graph.ParseResourceIdentityFromNode(resourceProps)

	// Classify symptom type based on error message and container issues
	symptomType := classifySymptomType(event.Status, event.ErrorMessage, event.ContainerIssues)

	return &ObservedSymptom{
		Resource: SymptomResource{
			UID:       resource.UID,
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
		},
		Status:       event.Status,
		ErrorMessage: event.ErrorMessage,
		ObservedAt:   time.Unix(0, event.Timestamp),
		SymptomType:  symptomType,
	}, nil
}

// classifySymptomType determines the symptom category from observed facts
func classifySymptomType(status string, errorMessage string, containerIssues []string) string {
	// Check container issues first (most specific)
	for _, issue := range containerIssues {
		switch issue {
		case "ImagePullBackOff", "ErrImagePull":
			return "ImagePullError"
		case "CrashLoopBackOff":
			return "CrashLoop"
		case "OOMKilled":
			return "OOMKilled"
		case "ContainerCreating":
			return "ContainerStartup"
		}
	}

	// Check error message patterns (case-insensitive)
	errorLower := strings.ToLower(errorMessage)
	if strings.Contains(errorLower, "image") && (strings.Contains(errorLower, "pull") || strings.Contains(errorLower, "failed")) {
		return "ImagePullError"
	}
	if strings.Contains(errorLower, "crash") || strings.Contains(errorLower, "backoff") {
		return "CrashLoop"
	}
	if strings.Contains(errorLower, "oom") || strings.Contains(errorLower, "out of memory") {
		return "OOMKilled"
	}
	if strings.Contains(errorLower, "evicted") {
		return "Evicted"
	}
	if strings.Contains(errorLower, "unschedulable") || strings.Contains(errorLower, "insufficient") {
		return "SchedulingFailure"
	}

	// Fallback to status
	switch status {
	case "Error":
		return "Error"
	case "Warning":
		return "Warning"
	case "Terminating":
		return "Terminating"
	case "Pending":
		// Check if it's a scheduling issue
		if strings.Contains(errorLower, "node") || strings.Contains(errorLower, "pending") {
			return "SchedulingFailure"
		}
		return "Pending"
	default:
		return "Unknown"
	}
}

// buildCausalChain constructs the ordered causal chain from symptom to root cause
func (t *GraphFindRootCauseTool) buildCausalChain(
	ctx context.Context,
	symptom *ObservedSymptom,
	failureTimestamp int64,
) ([]CausalStep, error) {
	// Query to traverse ownership backward and find MANAGES relationships
	query := graph.GraphQuery{
		Query: `
			MATCH (symptomResource:ResourceIdentity {uid: $symptomUID})
			
			// Collect all owners in the ownership chain (up to 5 levels)
			OPTIONAL MATCH (symptomResource)<-[:OWNS*1..5]-(owner:ResourceIdentity)
			
			// Combine symptom resource with its owners
			WITH symptomResource, collect(DISTINCT owner) as owners
			WITH symptomResource, [symptomResource] + owners as chainResources
			
			// For each resource in the chain, find its manager and changes
			UNWIND chainResources as resource
			
			OPTIONAL MATCH (manager:ResourceIdentity)-[manages:MANAGES]->(resource)
			WHERE manages.confidence >= 0.5
			
			// Get change events for the resource
			OPTIONAL MATCH (resource)-[:CHANGED]->(changeEvent:ChangeEvent)
			WHERE changeEvent.timestamp <= $failureTimestamp
			  AND changeEvent.timestamp >= $failureTimestamp - $lookback
			
			// Get change events for the manager (HelmRelease, etc.)
			OPTIONAL MATCH (manager)-[:CHANGED]->(managerEvent:ChangeEvent)
			WHERE managerEvent.timestamp <= $failureTimestamp
			  AND managerEvent.timestamp >= $failureTimestamp - $lookback
			
			// Calculate distance from symptom (0 = symptom itself)
			WITH resource, manager, manages, changeEvent, managerEvent,
			     CASE WHEN resource.uid = $symptomUID THEN 0
			          ELSE size([(symptomResource)<-[:OWNS*]-(resource)])
			     END as distance
			
			// Return each resource with its manager and relevant changes
			RETURN DISTINCT 
			  resource,
			  manager,
			  manages,
			  changeEvent,
			  managerEvent,
			  distance
			ORDER BY distance DESC, changeEvent.timestamp ASC, managerEvent.timestamp ASC
		`,
		Parameters: map[string]interface{}{
			"symptomUID":       symptom.Resource.UID,
			"failureTimestamp": failureTimestamp,
			"lookback":         int64(600_000_000_000), // 10 minutes
		},
	}

	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query causal chain: %w", err)
	}

	t.logger.Debug("buildCausalChain: query returned %d rows", len(result.Rows))

	// Build chain steps from query results
	steps := []CausalStep{}
	seenResources := make(map[string]bool)
	stepNumber := 1

	for _, row := range result.Rows {
		if len(row) < 6 {
			continue
		}

		// Parse resource
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			t.logger.Debug("Failed to parse resource: %v", err)
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)

		// Skip if we've already seen this resource
		if seenResources[resource.UID] {
			continue
		}
		seenResources[resource.UID] = true

		// Parse manager (may be null)
		var manager *graph.ResourceIdentity
		var managesEdge *graph.ManagesEdge
		if row[1] != nil {
			managerProps, err := graph.ParseNodeFromResult(row[1])
			if err == nil {
				mgr := graph.ParseResourceIdentityFromNode(managerProps)
				manager = &mgr
			}
		}
		if row[2] != nil {
			_, edgeProps, err := graph.ParseEdgeFromResult(row[2])
			if err == nil {
				edge := graph.ParseManagesEdge(edgeProps)
				managesEdge = &edge
			}
		}

		// Parse change event for the resource (may be null)
		var changeEvent *ChangeEventInfo
		if row[3] != nil {
			eventProps, err := graph.ParseNodeFromResult(row[3])
			if err == nil {
				event := graph.ParseChangeEventFromNode(eventProps)
				changeEvent = &ChangeEventInfo{
					EventID:       event.ID,
					Timestamp:     time.Unix(0, event.Timestamp),
					EventType:     event.EventType,
					ConfigChanged: event.ConfigChanged,
					StatusChanged: event.StatusChanged,
					Description:   fmt.Sprintf("%s event", event.EventType),
				}
			}
		}

		// Parse change event for the manager (may be null)
		var managerChangeEvent *ChangeEventInfo
		if row[4] != nil {
			eventProps, err := graph.ParseNodeFromResult(row[4])
			if err == nil {
				event := graph.ParseChangeEventFromNode(eventProps)
				managerChangeEvent = &ChangeEventInfo{
					EventID:       event.ID,
					Timestamp:     time.Unix(0, event.Timestamp),
					EventType:     event.EventType,
					ConfigChanged: event.ConfigChanged,
					StatusChanged: event.StatusChanged,
					Description:   fmt.Sprintf("%s event", event.EventType),
				}
			}
		}

		// Determine relationship type and next resource in chain
		var relationshipType string
		var relationshipTo *SymptomResource

		if manager != nil {
			// This resource is managed by something
			relationshipType = "MANAGES"
			relationshipTo = &SymptomResource{
				UID:       manager.UID,
				Kind:      manager.Kind,
				Namespace: manager.Namespace,
				Name:      manager.Name,
			}
		} else if resource.UID != symptom.Resource.UID {
			// This resource owns the previous one in the chain
			relationshipType = "OWNS"
			// relationshipTo will be set based on the next step
		} else {
			// This is the symptom itself
			relationshipType = "SYMPTOM"
		}

		// Generate reasoning for this step
		reasoning := generateStepReasoning(resource, manager, managesEdge, changeEvent, relationshipType)

		step := CausalStep{
			StepNumber: stepNumber,
			Resource: SymptomResource{
				UID:       resource.UID,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
				Name:      resource.Name,
			},
			ChangeEvent:      changeEvent,
			RelationshipType: relationshipType,
			RelationshipTo:   relationshipTo,
			Reasoning:        reasoning,
		}

		steps = append(steps, step)
		stepNumber++

		// If this resource has a manager with a change event, add the manager as a separate step
		if manager != nil && managerChangeEvent != nil && !seenResources[manager.UID] {
			seenResources[manager.UID] = true

			managerStep := CausalStep{
				StepNumber: stepNumber,
				Resource: SymptomResource{
					UID:       manager.UID,
					Kind:      manager.Kind,
					Namespace: manager.Namespace,
					Name:      manager.Name,
				},
				ChangeEvent:      managerChangeEvent,
				RelationshipType: "MANAGES",
				RelationshipTo: &SymptomResource{
					UID:       resource.UID,
					Kind:      resource.Kind,
					Namespace: resource.Namespace,
					Name:      resource.Name,
				},
				Reasoning: fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
					manager.Kind, resource.Kind,
					managesEdge.Confidence*100),
			}

			steps = append(steps, managerStep)
			stepNumber++
		}
	}

	t.logger.Debug("Built causal chain with %d steps", len(steps))
	return steps, nil
}

// generateStepReasoning creates a human-readable explanation for a causal step
func generateStepReasoning(
	resource graph.ResourceIdentity,
	manager *graph.ResourceIdentity,
	managesEdge *graph.ManagesEdge,
	changeEvent *ChangeEventInfo,
	relationshipType string,
) string {
	switch relationshipType {
	case "MANAGES":
		if manager != nil {
			confidence := 0.0
			if managesEdge != nil {
				confidence = managesEdge.Confidence
			}
			return fmt.Sprintf("%s manages %s lifecycle (confidence: %.0f%%)",
				manager.Kind, resource.Kind, confidence*100)
		}
		return "Lifecycle management relationship"

	case "OWNS":
		return fmt.Sprintf("%s owns resources in the next layer", resource.Kind)

	case "SYMPTOM":
		if changeEvent != nil && changeEvent.ConfigChanged {
			return "Configuration change triggered the failure"
		}
		return "Observed failure symptom"

	default:
		if changeEvent != nil {
			return fmt.Sprintf("%s occurred in this resource", changeEvent.EventType)
		}
		return "Part of the causal chain"
	}
}

// identifyRootCause extracts the root cause from the causal chain
func (t *GraphFindRootCauseTool) identifyRootCause(
	causalChain []CausalStep,
	failureTimestamp int64,
) (*RootCauseHypothesis, error) {
	if len(causalChain) == 0 {
		return nil, fmt.Errorf("empty causal chain")
	}

	// Root cause is the last step in the chain (furthest from symptom)
	rootStep := causalChain[len(causalChain)-1]

	// If no change event at root, use first step with change event
	var rootEvent *ChangeEventInfo
	for i := len(causalChain) - 1; i >= 0; i-- {
		if causalChain[i].ChangeEvent != nil {
			rootEvent = causalChain[i].ChangeEvent
			rootStep = causalChain[i]
			break
		}
	}

	if rootEvent == nil {
		return nil, fmt.Errorf("no change event found in causal chain")
	}

	// Classify the causation type
	causationType := classifyCausationType(rootEvent, rootStep.RelationshipType)

	// Generate explanation
	explanation := generateRootCauseExplanation(rootStep, rootEvent, causationType, causalChain)

	// Calculate time lag
	timeLagMs := (failureTimestamp - rootEvent.Timestamp.UnixNano()) / 1_000_000

	return &RootCauseHypothesis{
		Resource: rootStep.Resource,
		ChangeEvent: ChangeEventInfo{
			EventID:       rootEvent.EventID,
			Timestamp:     rootEvent.Timestamp,
			EventType:     rootEvent.EventType,
			ConfigChanged: rootEvent.ConfigChanged,
			StatusChanged: rootEvent.StatusChanged,
			Description:   rootEvent.Description,
		},
		CausationType: causationType,
		Explanation:   explanation,
		TimeLagMs:     timeLagMs,
	}, nil
}

// classifyCausationType determines the type of root cause
func classifyCausationType(event *ChangeEventInfo, relationshipType string) string {
	if event.ConfigChanged {
		return "ConfigChange"
	}
	switch event.EventType {
	case "CREATE":
		return "ResourceCreation"
	case "UPDATE":
		if relationshipType == "MANAGES" {
			return "DeploymentUpdate"
		}
		return "ResourceUpdate"
	case "DELETE":
		return "ResourceDeletion"
	default:
		return "Unknown"
	}
}

// generateRootCauseExplanation creates a human-readable explanation
func generateRootCauseExplanation(
	rootStep CausalStep,
	event *ChangeEventInfo,
	causationType string,
	chain []CausalStep,
) string {
	explanation := fmt.Sprintf("%s '%s' ", rootStep.Resource.Kind, rootStep.Resource.Name)

	switch causationType {
	case "ConfigChange":
		explanation += "configuration was changed"
	case "DeploymentUpdate":
		explanation += "was updated (deployment)"
	case "ResourceCreation":
		explanation += "was created"
	case "ResourceUpdate":
		explanation += "was updated"
	case "ResourceDeletion":
		explanation += "was deleted"
	default:
		explanation += "changed"
	}

	// Add propagation path if chain is longer than just root
	if len(chain) > 1 {
		explanation += ", which cascaded through "
		for i := len(chain) - 2; i >= 0; i-- {
			explanation += chain[i].Resource.Kind
			if i > 0 {
				explanation += " → "
			}
		}
	}

	return explanation
}

// calculateConfidence computes a deterministic confidence score
func (t *GraphFindRootCauseTool) calculateConfidence(
	symptom *ObservedSymptom,
	causalChain []CausalStep,
	rootCause *RootCauseHypothesis,
) ConfidenceScore {
	// Factor weights (must sum to 1.0)
	const (
		weightSpecChange     = 0.30
		weightTemporal       = 0.25
		weightRelationship   = 0.25
		weightErrorMatch     = 0.10
		weightCompleteness   = 0.10
	)

	// Calculate each factor
	factors := ConfidenceFactors{
		DirectSpecChange:     calculateSpecChangeFactor(rootCause),
		TemporalProximity:    calculateTemporalFactor(rootCause.TimeLagMs),
		RelationshipStrength: calculateRelationshipFactor(causalChain),
		ErrorMessageMatch:    calculateErrorMatchFactor(symptom, rootCause),
		ChainCompleteness:    calculateCompletenessFactor(causalChain),
	}

	// Weighted average
	score := factors.DirectSpecChange*weightSpecChange +
		factors.TemporalProximity*weightTemporal +
		factors.RelationshipStrength*weightRelationship +
		factors.ErrorMessageMatch*weightErrorMatch +
		factors.ChainCompleteness*weightCompleteness

	// Generate rationale
	rationale := generateConfidenceRationale(factors, score)

	return ConfidenceScore{
		Score:     score,
		Rationale: rationale,
		Factors:   factors,
	}
}

// calculateSpecChangeFactor: 1.0 if configChanged=true, 0.5 if UPDATE, 0.0 otherwise
func calculateSpecChangeFactor(rootCause *RootCauseHypothesis) float64 {
	if rootCause.ChangeEvent.ConfigChanged {
		return 1.0
	}
	if rootCause.ChangeEvent.EventType == "UPDATE" {
		return 0.5
	}
	return 0.0
}

// calculateTemporalFactor: 1.0 - (timeLagMs / 600000) capped at [0, 1]
func calculateTemporalFactor(timeLagMs int64) float64 {
	// 10 minutes = 600,000ms
	maxLagMs := 600000.0
	if timeLagMs < 0 {
		timeLagMs = 0
	}
	factor := 1.0 - (float64(timeLagMs) / maxLagMs)
	if factor < 0 {
		return 0
	}
	if factor > 1 {
		return 1
	}
	return factor
}

// calculateRelationshipFactor: MANAGES=1.0, OWNS=0.8, TRIGGERED_BY=confidence
func calculateRelationshipFactor(chain []CausalStep) float64 {
	if len(chain) == 0 {
		return 0.0
	}

	// Find the strongest relationship in the chain
	maxStrength := 0.0
	for _, step := range chain {
		var strength float64
		switch step.RelationshipType {
		case "MANAGES":
			strength = 1.0
		case "OWNS":
			strength = 0.8
		case "TRIGGERED_BY":
			// Would need to extract confidence from edge, default to 0.7
			strength = 0.7
		default:
			strength = 0.5
		}
		if strength > maxStrength {
			maxStrength = strength
		}
	}
	return maxStrength
}

// calculateErrorMatchFactor: 1.0 if error mentions config/image, 0.5 if generic, 0.0 if none
func calculateErrorMatchFactor(symptom *ObservedSymptom, rootCause *RootCauseHypothesis) float64 {
	errorLower := strings.ToLower(symptom.ErrorMessage)
	
	// Check if error mentions configuration or image issues
	if strings.Contains(errorLower, "image") ||
		strings.Contains(errorLower, "config") ||
		strings.Contains(errorLower, "invalid") ||
		strings.Contains(errorLower, "pull") {
		return 1.0
	}

	// Generic error messages
	if symptom.ErrorMessage != "" {
		return 0.5
	}

	return 0.0
}

// calculateCompletenessFactor: stepsFound / expectedSteps
func calculateCompletenessFactor(chain []CausalStep) float64 {
	// Expected: Pod <- ReplicaSet <- Deployment <- [Manager] = 3-4 steps
	// Actual: len(chain)
	expectedSteps := 3.0
	actualSteps := float64(len(chain))
	
	factor := actualSteps / expectedSteps
	if factor > 1.0 {
		return 1.0
	}
	return factor
}

// generateConfidenceRationale creates a human-readable explanation of the score
func generateConfidenceRationale(factors ConfidenceFactors, score float64) string {
	rationale := fmt.Sprintf("Confidence: %.0f%%. ", score*100)

	// List contributing factors
	contributions := []string{}
	if factors.DirectSpecChange > 0.5 {
		contributions = append(contributions, "direct spec change detected")
	}
	if factors.TemporalProximity > 0.7 {
		contributions = append(contributions, "change occurred shortly before failure")
	}
	if factors.RelationshipStrength > 0.8 {
		contributions = append(contributions, "strong management relationship")
	}
	if factors.ErrorMessageMatch > 0.5 {
		contributions = append(contributions, "error message correlates")
	}
	if factors.ChainCompleteness > 0.8 {
		contributions = append(contributions, "complete causal chain")
	}

	if len(contributions) > 0 {
		rationale += "Based on: " + strings.Join(contributions, ", ") + "."
	}

	return rationale
}

// collectSupportingEvidence consolidates evidence from the causal chain
func (t *GraphFindRootCauseTool) collectSupportingEvidence(
	causalChain []CausalStep,
	rootCause *RootCauseHypothesis,
) []EvidenceItem {
	evidence := []EvidenceItem{}
	seenTypes := make(map[string]bool)

	// RELATIONSHIP evidence (MANAGES edges)
	for _, step := range causalChain {
		if step.RelationshipType == "MANAGES" && !seenTypes["MANAGES"] {
			evidence = append(evidence, EvidenceItem{
				Type:        "RELATIONSHIP",
				Description: step.Reasoning,
				Confidence:  1.0,
				Details: map[string]interface{}{
					"relationshipType": "MANAGES",
					"from":            step.RelationshipTo,
					"to":              step.Resource,
				},
			})
			seenTypes["MANAGES"] = true
		}
	}

	// TEMPORAL evidence
	if rootCause.TimeLagMs > 0 && !seenTypes["TEMPORAL"] {
		evidence = append(evidence, EvidenceItem{
			Type:        "TEMPORAL",
			Description: fmt.Sprintf("Change occurred %d seconds before failure", rootCause.TimeLagMs/1000),
			Confidence:  math.Max(0, 1.0-(float64(rootCause.TimeLagMs)/600000.0)),
			Details: map[string]interface{}{
				"lagMs": rootCause.TimeLagMs,
			},
		})
		seenTypes["TEMPORAL"] = true
	}

	// STRUCTURAL evidence (ownership chain)
	if len(causalChain) > 1 && !seenTypes["STRUCTURAL"] {
		chainDesc := ""
		for i := len(causalChain) - 1; i >= 0; i-- {
			chainDesc += causalChain[i].Resource.Kind
			if i > 0 {
				chainDesc += " → "
			}
		}
		evidence = append(evidence, EvidenceItem{
			Type:        "STRUCTURAL",
			Description: fmt.Sprintf("Ownership chain: %s", chainDesc),
			Confidence:  0.8,
			Details: map[string]interface{}{
				"chainLength": len(causalChain),
			},
		})
		seenTypes["STRUCTURAL"] = true
	}

	// Limit to 5 most relevant items
	if len(evidence) > 5 {
		evidence = evidence[:5]
	}

	return evidence
}

// detectExcludedAlternatives identifies other hypotheses that were considered but rejected
func (t *GraphFindRootCauseTool) detectExcludedAlternatives(
	ctx context.Context,
	symptom *ObservedSymptom,
	rootCause *RootCauseHypothesis,
	failureTimestamp int64,
) []ExcludedHypothesis {
	// Query for other changes in the time window
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp <= $failureTimestamp
			  AND e.timestamp >= $failureTimestamp - $lookback
			  AND r.uid <> $rootCauseUID
			  AND r.namespace = $namespace
			RETURN r, e
			ORDER BY e.timestamp DESC
			LIMIT 5
		`,
		Parameters: map[string]interface{}{
			"failureTimestamp": failureTimestamp,
			"lookback":         int64(600_000_000_000),
			"rootCauseUID":     rootCause.Resource.UID,
			"namespace":        symptom.Resource.Namespace,
		},
	}

	result, err := t.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		t.logger.Debug("Failed to query excluded alternatives: %v", err)
		return nil
	}

	excluded := []ExcludedHypothesis{}
	for _, row := range result.Rows {
		if len(row) < 2 {
			continue
		}

		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)

		eventProps, err := graph.ParseNodeFromResult(row[1])
		if err != nil {
			continue
		}
		_ = graph.ParseChangeEventFromNode(eventProps) // Parse but not used directly

		// Generate hypothesis and reason for exclusion
		hypothesis := fmt.Sprintf("%s '%s' changed at similar time", resource.Kind, resource.Name)
		reason := "No ownership or management relationship to failed resource"

		excluded = append(excluded, ExcludedHypothesis{
			Resource: SymptomResource{
				UID:       resource.UID,
				Kind:      resource.Kind,
				Namespace: resource.Namespace,
				Name:      resource.Name,
			},
			Hypothesis:     hypothesis,
			ReasonExcluded: reason,
		})

		// Limit to 3 alternatives
		if len(excluded) >= 3 {
			break
		}
	}

	return excluded
}
