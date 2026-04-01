package analysis

import (
	"context"
	"fmt"
)

// getOwnershipChain retrieves the ownership chain from the symptom resource up to 3 levels
// This follows the OWNS relationship backwards from the symptom to find parent resources.
// For example: Pod -> ReplicaSet -> Deployment
func (a *RootCauseAnalyzer) getOwnershipChain(ctx context.Context, symptomUID string) ([]ResourceWithDistance, error) {
	a.logger.Debug("getOwnershipChain: getting ownership chain for %s", symptomUID)

	chain, err := a.store.GetOwnershipChain(ctx, symptomUID, 0, MaxOwnershipDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to query ownership chain: %w", err)
	}

	converted := convertStoreResourceWithDistanceList(chain)
	a.logger.Debug("getOwnershipChain: found %d resources in chain", len(converted))
	return converted, nil
}
