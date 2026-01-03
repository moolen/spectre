package analysis

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// getOwnershipChain retrieves the ownership chain from the symptom resource up to 3 levels
// This follows the OWNS relationship backwards from the symptom to find parent resources.
// For example: Pod -> ReplicaSet -> Deployment
func (a *RootCauseAnalyzer) getOwnershipChain(ctx context.Context, symptomUID string) ([]ResourceWithDistance, error) {
	// First, get the symptom resource
	symptomQuery := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			RETURN symptom as resource, 0 as distance
		`,
		Parameters: map[string]interface{}{
			"symptomUID": symptomUID,
		},
	}

	a.logger.Debug("getOwnershipChain: getting symptom resource %s", symptomUID)
	symptomResult, err := a.graphClient.ExecuteQuery(ctx, symptomQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to query symptom resource: %w", err)
	}

	chain := []ResourceWithDistance{}

	// Parse symptom
	for _, row := range symptomResult.Rows {
		if len(row) < 2 {
			continue
		}
		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil || resourceProps == nil || len(resourceProps) == 0 {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)
		chain = append(chain, ResourceWithDistance{
			Resource: resource,
			Distance: 0,
		})
	}

	if len(chain) == 0 {
		return nil, fmt.Errorf("symptom resource not found: %s", symptomUID)
	}

	// Now get owners (traverse up to 3 levels)
	ownersQuery := graph.GraphQuery{
		Timeout: 5000,
		Query: `
			MATCH (symptom:ResourceIdentity {uid: $symptomUID})
			MATCH path = (symptom)<-[:OWNS*1..3]-(owner:ResourceIdentity)
			RETURN DISTINCT owner as resource, length(path) as distance
			ORDER BY distance ASC
		`,
		Parameters: map[string]interface{}{
			"symptomUID": symptomUID,
		},
	}

	a.logger.Debug("getOwnershipChain: getting owners for %s", symptomUID)
	ownersResult, err := a.graphClient.ExecuteQuery(ctx, ownersQuery)
	if err != nil {
		// If there are no owners, this query might fail or return empty - that's OK
		a.logger.Debug("getOwnershipChain: no owners found (or query error): %v", err)
		return chain, nil
	}

	seenUIDs := make(map[string]bool)
	seenUIDs[chain[0].Resource.UID] = true

	for _, row := range ownersResult.Rows {
		if len(row) < 2 {
			continue
		}

		resourceProps, err := graph.ParseNodeFromResult(row[0])
		if err != nil || resourceProps == nil || len(resourceProps) == 0 {
			continue
		}
		resource := graph.ParseResourceIdentityFromNode(resourceProps)

		if seenUIDs[resource.UID] {
			continue
		}
		seenUIDs[resource.UID] = true

		distance := 0
		if d, ok := row[1].(int64); ok {
			distance = int(d)
		} else if d, ok := row[1].(float64); ok {
			distance = int(d)
		}

		chain = append(chain, ResourceWithDistance{
			Resource: resource,
			Distance: distance,
		})
	}

	a.logger.Debug("getOwnershipChain: found %d resources in chain", len(chain))
	return chain, nil
}
