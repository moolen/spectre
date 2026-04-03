package analysis

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

// buildCausalGraph constructs the causal graph from symptom to root cause.
func (a *RootCauseAnalyzer) buildCausalGraph(
	ctx context.Context,
	symptom *ObservedSymptom,
	failureTimestamp int64,
	lookbackNs int64,
	maxDepth int,
) (CausalGraph, error) {
	queryStart := time.Now()

	a.logger.Debug("buildCausalGraph: getting ownership chain for symptom %s", symptom.Resource.UID)
	chainStart := time.Now()
	chain, err := a.getOwnershipChain(ctx, symptom.Resource.UID, failureTimestamp, maxDepth)
	chainDuration := time.Since(chainStart)
	if err != nil {
		a.logger.Error("buildCausalGraph: ownership chain query failed after %v: %v", chainDuration, err)
		return CausalGraph{}, fmt.Errorf("failed to get ownership chain: %w", err)
	}

	a.logger.Debug("buildCausalGraph: ownership chain query completed in %v", chainDuration)
	if chainDuration.Milliseconds() > SlowQueryThresholdMs {
		a.logger.Warn("buildCausalGraph: slow ownership chain query: %v (threshold: %dms)",
			chainDuration, SlowQueryThresholdMs)
	}

	resourceUIDs := extractUIDs(chain)
	a.logger.Debug("buildCausalGraph: found %d resources in chain", len(resourceUIDs))

	managers, err := a.getManagers(ctx, resourceUIDs)
	if err != nil {
		return CausalGraph{}, fmt.Errorf("failed to get managers: %w", err)
	}
	a.logger.Debug("buildCausalGraph: found %d managers", len(managers))

	allUIDs := appendManagerUIDs(resourceUIDs, managers, a)
	a.logger.Debug("buildCausalGraph: querying for related resources of %d resources (chain + managers): %v", len(allUIDs), allUIDs)

	related, changeEvents, k8sEvents, err := a.loadCausalGraphDetails(ctx, allUIDs, resourceUIDs, failureTimestamp, lookbackNs)
	if err != nil {
		return CausalGraph{}, fmt.Errorf("failed to build causal graph: %w", err)
	}

	if err := a.mergeManagerAndRelatedEvents(ctx, managers, related, changeEvents, failureTimestamp, lookbackNs); err != nil {
		return CausalGraph{}, err
	}

	a.logger.Debug("buildCausalGraph: all queries completed in %v", time.Since(queryStart))
	return a.mergeIntoCausalGraph(symptom, chain, managers, related, changeEvents, k8sEvents, failureTimestamp)
}

func appendManagerUIDs(resourceUIDs []string, managers map[string]*ManagerData, analyzer *RootCauseAnalyzer) []string {
	allUIDs := append([]string{}, resourceUIDs...)

	for managerUID, mgr := range managers {
		analyzer.logger.Debug("buildCausalGraph: found manager %s/%s (UID: %s) for resource %s",
			mgr.Manager.Kind, mgr.Manager.Name, mgr.Manager.UID, managerUID)

		found := false
		for _, uid := range resourceUIDs {
			if uid == mgr.Manager.UID {
				found = true
				break
			}
		}
		if !found {
			analyzer.logger.Debug("buildCausalGraph: adding manager UID %s to query list", mgr.Manager.UID)
			allUIDs = append(allUIDs, mgr.Manager.UID)
		} else {
			analyzer.logger.Debug("buildCausalGraph: manager UID %s already in chain", mgr.Manager.UID)
		}
	}

	return allUIDs
}

func (a *RootCauseAnalyzer) loadCausalGraphDetails(
	ctx context.Context,
	allUIDs []string,
	resourceUIDs []string,
	failureTimestamp int64,
	lookbackNs int64,
) (
	map[string][]RelatedResourceData,
	map[string][]ChangeEventInfo,
	map[string][]K8sEventInfo,
	error,
) {
	a.logger.Debug("buildCausalGraph: executing parallel queries (related resources, change events, k8s events)")
	parallelStart := time.Now()

	g, gctx := errgroup.WithContext(ctx)
	var related map[string][]RelatedResourceData
	var changeEvents map[string][]ChangeEventInfo
	var k8sEvents map[string][]K8sEventInfo

	g.Go(func() error {
		start := time.Now()
		var err error
		related, err = a.getRelatedResources(gctx, allUIDs, failureTimestamp, lookbackNs)
		a.logCausalQueryResult("related resources", start, err, len(related))
		return err
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		changeEvents, err = a.getChangeEvents(gctx, resourceUIDs, failureTimestamp, lookbackNs)
		a.logCausalQueryResult("change events", start, err, len(changeEvents))
		return err
	})

	g.Go(func() error {
		start := time.Now()
		var err error
		k8sEvents, err = a.getK8sEvents(gctx, resourceUIDs, failureTimestamp, lookbackNs)
		a.logCausalQueryResult("k8s events", start, err, len(k8sEvents))
		return err
	})

	if err := g.Wait(); err != nil {
		a.logger.Error("buildCausalGraph: parallel queries failed after %v: %v", time.Since(parallelStart), err)
		return nil, nil, nil, err
	}

	a.logger.Debug("buildCausalGraph: all parallel queries completed in %v", time.Since(parallelStart))
	return related, changeEvents, k8sEvents, nil
}

func (a *RootCauseAnalyzer) logCausalQueryResult(queryName string, start time.Time, err error, resultCount int) {
	duration := time.Since(start)
	if err != nil {
		a.logger.Error("buildCausalGraph: %s query failed after %v: %v", queryName, duration, err)
		return
	}
	a.logger.Debug("buildCausalGraph: %s query completed in %v (%d resources)", queryName, duration, resultCount)
}

func (a *RootCauseAnalyzer) mergeManagerAndRelatedEvents(
	ctx context.Context,
	managers map[string]*ManagerData,
	related map[string][]RelatedResourceData,
	changeEvents map[string][]ChangeEventInfo,
	failureTimestamp int64,
	lookbackNs int64,
) error {
	managerUIDs := make([]string, 0, len(managers))
	for _, mgr := range managers {
		managerUIDs = append(managerUIDs, mgr.Manager.UID)
	}

	if len(managerUIDs) > 0 {
		managerEvents, err := a.getChangeEvents(ctx, managerUIDs, failureTimestamp, lookbackNs)
		if err != nil {
			return fmt.Errorf("failed to get manager events: %w", err)
		}
		for uid, events := range managerEvents {
			changeEvents[uid] = events
			a.logger.Debug("buildCausalGraph: merged %d events for manager %s", len(events), uid)
		}
	}

	if err := a.getChangeEventsForRelated(ctx, related, failureTimestamp, lookbackNs); err != nil {
		return fmt.Errorf("failed to get related resource events: %w", err)
	}

	return nil
}
