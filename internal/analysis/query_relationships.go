package analysis

import (
	"context"
	"fmt"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

// getManagers retrieves manager relationships for the given resources.
// Managers are resources connected via MANAGES edges (e.g., HelmRelease -> Deployment).
// Only managers with confidence >= MinManagerConfidence are included.
func (a *RootCauseAnalyzer) getManagers(ctx context.Context, resourceUIDs []string) (map[string]*ManagerData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*ManagerData), nil
	}

	a.logger.Debug("getManagers: querying managers for %d resources", len(resourceUIDs))
	managers, err := a.store.GetManagers(ctx, resourceUIDs, MinManagerConfidence)
	if err != nil {
		return nil, fmt.Errorf("failed to get managers: %w", err)
	}

	converted := convertStoreManagers(managers)
	a.logger.Debug("getManagers: found managers for %d resources", len(converted))
	return converted, nil
}

// getRelatedResources retrieves resources related through various relationship types.
// This includes:
// - REFERENCES_SPEC: Resources referenced in spec (e.g., HelmRelease -> ConfigMap)
// - SCHEDULED_ON: Pods scheduled on Nodes
// - USES_SERVICE_ACCOUNT: Pods using ServiceAccounts
// - SELECTS: Services/NetworkPolicies selecting resources
// - GRANTS_TO: RoleBindings granting permissions to ServiceAccounts
// - BINDS_ROLE: RoleBindings binding to Roles/ClusterRoles
// - edgeTypeIngressRef: Ingresses referencing Services
//
// The failureTimestamp and lookbackNs parameters are used to include deleted resources
// that were deleted within the time window (important for root cause analysis).
func (a *RootCauseAnalyzer) getRelatedResources(ctx context.Context, resourceUIDs []string, failureTimestamp, lookbackNs int64) (map[string][]RelatedResourceData, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string][]RelatedResourceData), nil
	}

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: failureTimestamp,
		LookbackNs:         lookbackNs,
	}

	a.logger.Debug("getRelatedResources: querying related resources for %d resources: %v", len(resourceUIDs), resourceUIDs)
	related, err := a.store.GetRelatedResources(ctx, resourceUIDs, window)
	if err != nil {
		return nil, fmt.Errorf("failed to get related resources: %w", err)
	}

	converted := convertStoreRelatedResources(related)
	a.logger.Debug("getRelatedResources: found related resources for %d resources", len(converted))
	for uid, relList := range converted {
		a.logger.Debug("getRelatedResources: resource %s has %d related resources", uid, len(relList))
		for _, rel := range relList {
			a.logger.Debug("getRelatedResources: - %s/%s (type=%s)", rel.Resource.Kind, rel.Resource.Name, rel.RelationshipType)
			if rel.RelationshipType == edgeTypeReferencesSpec {
				a.logger.Debug("getRelatedResources: *** resource %s REFERENCES_SPEC %s/%s", uid, rel.Resource.Kind, rel.Resource.Name)
			}
		}
	}

	return converted, nil
}
