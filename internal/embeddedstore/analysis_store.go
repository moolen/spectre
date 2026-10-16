package embeddedstore

import (
	"context"
	"fmt"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

const (
	defaultLimit      = 50
	maxLimit          = 500
	defaultMaxDepth   = 1
	maxMaxDepth       = 10
	defaultLookbackNs = int64(30 * time.Minute)
	maxLookbackNs     = int64(24 * time.Hour)
	maxRecentEvents   = 10
	maxK8sEvents      = 20
	edgeTypeIngress   = "INGRESS_REF"
)

// Store implements analysis queries over checkpoint-restored projection state.
// It does not scan raw hot/cold event tiers during normal query execution.
type Store struct {
	projection *Projection
	// olderK8sEvents reads event summaries the projection no longer holds in memory
	// (see Projection.k8sEventInfosComplete); nil when only the projection is available.
	olderK8sEvents func(ctx context.Context, involvedUIDs []string, startNs, endNs int64) (map[string][]analysisstore.K8sEventInfo, error)
}

var _ analysisstore.AnalysisStore = (*Store)(nil)

func NewAnalysisStore(projection *Projection) *Store {
	if projection == nil {
		projection = NewProjection()
	}
	projection.resolvePendingGraphFacts()
	return &Store{projection: projection}
}

func (s *Store) GetResource(_ context.Context, uid string) (*graph.ResourceIdentity, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	record := s.projection.resourcesByUID[uid]
	if record == nil {
		return nil, nil
	}
	version := record.latestVersion()
	if version == nil {
		return nil, nil
	}
	identity := copyIdentity(version)
	return &identity, nil
}

func (s *Store) GetOwnershipChain(_ context.Context, uid string, atTimestampNs int64, maxDepth int) ([]analysisstore.ResourceWithDistance, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	if maxDepth <= 0 {
		maxDepth = 3
	}
	record := s.projection.resourcesByUID[uid]
	if record == nil {
		return nil, fmt.Errorf("symptom resource not found: %s", uid)
	}
	start := record.versionAt(atTimestampNs)
	if start == nil {
		start = record.latestVersion()
	}
	if start == nil {
		return nil, fmt.Errorf("symptom resource not found: %s", uid)
	}

	result := []analysisstore.ResourceWithDistance{{
		Resource: copyIdentity(start),
		Distance: 0,
	}}
	seen := map[string]bool{uid: true}
	current := []*resourceVersion{start}

	for depth := 1; depth <= maxDepth; depth++ {
		var next []*resourceVersion
		for _, version := range current {
			for _, ownerUID := range factsFor(version).owners() {
				if ownerUID == "" || seen[ownerUID] {
					continue
				}
				ownerRecord := s.projection.resourcesByUID[ownerUID]
				if ownerRecord == nil {
					continue
				}
				ownerVersion := ownerRecord.versionAt(atTimestampNs)
				if ownerVersion == nil {
					continue
				}
				seen[ownerUID] = true
				result = append(result, analysisstore.ResourceWithDistance{
					Resource: copyIdentity(ownerVersion),
					Distance: depth,
				})
				next = append(next, ownerVersion)
			}
		}
		current = next
	}

	return result, nil
}

func (s *Store) GetManagers(_ context.Context, resourceUIDs []string, minConfidence float64) (map[string]*analysisstore.ManagerData, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	result := make(map[string]*analysisstore.ManagerData)
	for _, uid := range resourceUIDs {
		record := s.projection.resourcesByUID[uid]
		if record == nil {
			continue
		}
		version := record.latestVersion()
		if version == nil {
			continue
		}
		managerRef := managerReference(version)
		if managerRef == nil || managerRef.confidence < minConfidence {
			continue
		}
		targetNamespace := managerRef.namespace
		if targetNamespace == "" {
			targetNamespace = version.meta.Namespace
		}
		managerVersion := s.projection.resolveByName(targetNamespace, managerRef.kind, managerRef.name, version.timestamp, 0)
		if managerVersion == nil {
			continue
		}
		result[uid] = &analysisstore.ManagerData{
			Manager: copyIdentity(managerVersion),
			ManagesEdge: graph.ManagesEdge{
				Confidence:      managerRef.confidence,
				FirstObserved:   managerVersion.timestamp,
				LastValidated:   version.timestamp,
				ValidationState: graph.ValidationStateValid,
			},
		}
	}
	return result, nil
}

func (s *Store) GetRelatedResources(_ context.Context, resourceUIDs []string, window analysisstore.ResourceWindow) (map[string][]analysisstore.RelatedResourceData, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	result := make(map[string][]analysisstore.RelatedResourceData)
	startNs := window.Start()
	for _, uid := range resourceUIDs {
		record := s.projection.resourcesByUID[uid]
		if record == nil {
			continue
		}
		version := record.visibleVersionWithinWindow(window.FailureTimestampNs, startNs)
		if version == nil {
			continue
		}
		items := make([]analysisstore.RelatedResourceData, 0)
		added := make(map[string]bool)
		add := func(target *resourceVersion, relType, referenceTargetUID string) {
			if target == nil {
				return
			}
			key := relType + ":" + target.meta.UID + ":" + referenceTargetUID
			if added[key] {
				return
			}
			added[key] = true
			items = append(items, analysisstore.RelatedResourceData{
				Resource:           copyIdentity(target),
				RelationshipType:   relType,
				ReferenceTargetUID: referenceTargetUID,
			})
		}

		for ref := range factsFor(version).refs() {
			target := s.projection.resolveByName(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name, window.FailureTimestampNs, startNs)
			add(target, ref.relType, "")
			if ref.relType == "USES_SERVICE_ACCOUNT" && target != nil {
				for _, binding := range s.roleBindingsGrantingToServiceAccount(target, window.FailureTimestampNs, startNs) {
					add(binding, "GRANTS_TO", "")
					for roleRef := range factsFor(binding).refs() {
						if roleRef.relType != "BINDS_ROLE" {
							continue
						}
						role := s.projection.resolveByName(
							roleRef.namespaceFor(binding.meta.Namespace),
							roleRef.kind,
							roleRef.name,
							window.FailureTimestampNs,
							startNs,
						)
						add(role, "BINDS_ROLE", "")
					}
				}
			}
		}

		for _, selector := range s.selectingResourcesForTarget(version, window.FailureTimestampNs) {
			add(selector, "SELECTS", "")
			if selector.meta.Kind == "Service" {
				for _, ingress := range s.ingressesReferencingService(selector, window.FailureTimestampNs, startNs) {
					add(ingress, edgeTypeIngress, selector.meta.UID)
				}
			}
		}

		result[uid] = items
	}
	return result, nil
}

func (s *Store) GetChangeEvents(_ context.Context, resourceUIDs []string, window analysisstore.ResourceWindow) (map[string][]analysisstore.ChangeEventInfo, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	result := make(map[string][]analysisstore.ChangeEventInfo)
	startNs := window.Start()
	for _, uid := range resourceUIDs {
		record := s.projection.resourcesByUID[uid]
		if record == nil {
			continue
		}
		configEvents := make([]analysisstore.ChangeEventInfo, 0)
		recentEvents := make([]analysisstore.ChangeEventInfo, 0, maxRecentEvents)
		for i := len(record.versions) - 1; i >= 0; i-- {
			version := record.versions[i]
			if version.timestamp < startNs || version.timestamp > window.FailureTimestampNs {
				continue
			}
			changeEvent := version.changeEventInfo()
			if len(changeEvent.Data) == 0 && versionHasPayload(&version) {
				changeEvent.Data = versionPayloadCopy(&version)
			}
			if version.configChanged() {
				configEvents = append(configEvents, changeEvent)
			}
			if len(recentEvents) < maxRecentEvents {
				recentEvents = append(recentEvents, changeEvent)
			}
		}

		events := mergeChangeEvents(configEvents, recentEvents)
		if len(events) > 0 {
			result[uid] = events
		}
	}
	return result, nil
}

func (s *Store) GetK8sEvents(ctx context.Context, resourceUIDs []string, window analysisstore.ResourceWindow) (map[string][]analysisstore.K8sEventInfo, error) {
	startNs := window.Start()
	result := make(map[string][]analysisstore.K8sEventInfo)
	var incomplete []string

	s.projection.mu.RLock()
	for _, uid := range resourceUIDs {
		if !s.projection.k8sEventInfosComplete(uid, startNs) {
			incomplete = append(incomplete, uid)
			continue
		}
		if filtered := newestK8sEventsInWindow(s.projection.k8sEventsByInvolvedUID[uid], startNs, window.FailureTimestampNs); len(filtered) > 0 {
			result[uid] = filtered
		}
	}
	s.projection.mu.RUnlock()

	if len(incomplete) == 0 || s.olderK8sEvents == nil {
		return result, nil
	}
	older, err := s.olderK8sEvents(ctx, incomplete, startNs, window.FailureTimestampNs)
	if err != nil {
		return nil, err
	}
	for uid, events := range older {
		if filtered := newestK8sEventsInWindow(events, startNs, window.FailureTimestampNs); len(filtered) > 0 {
			result[uid] = filtered
		}
	}
	return result, nil
}

// newestK8sEventsInWindow returns up to maxK8sEvents of a newest-first list that fall in
// [startNs, endNs].
func newestK8sEventsInWindow(events []analysisstore.K8sEventInfo, startNs, endNs int64) []analysisstore.K8sEventInfo {
	filtered := make([]analysisstore.K8sEventInfo, 0, min(len(events), maxK8sEvents))
	for _, event := range events {
		ts := event.Timestamp.UnixNano()
		if ts < startNs || ts > endNs {
			continue
		}
		filtered = append(filtered, event)
		if len(filtered) == maxK8sEvents {
			break
		}
	}
	return filtered
}
