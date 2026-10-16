package embeddedstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
)

func (s *Store) GetNamespaceGraph(_ context.Context, input analysisstore.NamespaceGraphQuery) (*analysisstore.NamespaceGraphData, error) {
	s.projection.mu.RLock()
	defer s.projection.mu.RUnlock()

	startTime := time.Now()
	query := normalizeNamespaceQuery(input, s.projection.RetentionWindowNs())

	namespacedResources := s.projection.activeResourcesInNamespace(query.Namespace, query.TimestampNs)
	startIndex, err := decodeCursor(query.Cursor)
	if err != nil {
		startIndex = namespaceCursor{}
	}
	pagedResources := applyCursor(namespacedResources, startIndex)

	hasMore := len(pagedResources) > query.Limit
	if hasMore {
		pagedResources = pagedResources[:query.Limit]
	}

	selectedUIDs := make(map[string]*resourceVersion, len(pagedResources))
	for _, version := range pagedResources {
		selectedUIDs[version.meta.UID] = version
	}
	for _, version := range s.clusterScopedReachableFrom(pagedResources, query.TimestampNs, query.LookbackNs, query.MaxDepth) {
		selectedUIDs[version.meta.UID] = version
	}

	allVersions := make([]*resourceVersion, 0, len(selectedUIDs))
	for _, version := range selectedUIDs {
		allVersions = append(allVersions, version)
	}
	sort.Slice(allVersions, func(i, j int) bool {
		if allVersions[i].meta.Kind != allVersions[j].meta.Kind {
			return allVersions[i].meta.Kind < allVersions[j].meta.Kind
		}
		if allVersions[i].meta.Name != allVersions[j].meta.Name {
			return allVersions[i].meta.Name < allVersions[j].meta.Name
		}
		return allVersions[i].meta.UID < allVersions[j].meta.UID
	})

	nodes := make([]analysisstore.NamespaceGraphNode, 0, len(allVersions))
	for _, version := range allVersions {
		node := analysisstore.NamespaceGraphNode{
			UID:       version.meta.UID,
			Kind:      version.meta.Kind,
			APIGroup:  version.meta.APIGroup,
			Namespace: version.meta.Namespace,
			Name:      version.meta.Name,
			Status:    version.status(),
			Labels:    copyStringMap(version.labels()),
		}
		latestEvent := analysisstore.NamespaceGraphChangeEvent{
			TimestampNs:  version.timestamp,
			EventType:    string(version.eventType()),
			Status:       version.status(),
			SpecChanges:  s.specDiffWithinLookback(version.meta.UID, query.TimestampNs, query.LookbackNs),
			SpecReplicas: specReplicasForVersion(version),
		}
		node.LatestEvent = &latestEvent
		nodes = append(nodes, node)
	}

	edges := s.namespaceEdgesForSet(selectedUIDs, query.TimestampNs, query.LookbackNs)

	var nextCursor string
	if hasMore && len(pagedResources) > 0 {
		last := pagedResources[len(pagedResources)-1]
		nextCursor = encodeCursor(namespaceCursor{
			LastKind: last.meta.Kind,
			LastName: last.meta.Name,
		})
	}

	return &analysisstore.NamespaceGraphData{
		Graph: analysisstore.NamespaceGraph{
			Nodes: nodes,
			Edges: edges,
		},
		Metadata: analysisstore.NamespaceGraphMetadata{
			Namespace:        query.Namespace,
			TimestampNs:      query.TimestampNs,
			NodeCount:        len(nodes),
			EdgeCount:        len(edges),
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			HasMore:          hasMore,
			NextCursor:       nextCursor,
		},
	}, nil
}

type namespaceCursor struct {
	LastKind string `json:"lastKind"`
	LastName string `json:"lastName"`
}

func normalizeNamespaceQuery(input analysisstore.NamespaceGraphQuery, maxLookbackNs int64) analysisstore.NamespaceGraphQuery {
	normalized := input
	if normalized.Limit <= 0 {
		normalized.Limit = defaultLimit
	}
	if normalized.Limit > maxLimit {
		normalized.Limit = maxLimit
	}
	if normalized.MaxDepth <= 0 {
		normalized.MaxDepth = defaultMaxDepth
	}
	if normalized.MaxDepth > maxMaxDepth {
		normalized.MaxDepth = maxMaxDepth
	}
	if normalized.LookbackNs <= 0 {
		normalized.LookbackNs = defaultLookbackNs
	}
	if maxLookbackNs > 0 && normalized.LookbackNs > maxLookbackNs {
		normalized.LookbackNs = maxLookbackNs
	}
	return normalized
}

func encodeCursor(cursor namespaceCursor) string {
	data, _ := json.Marshal(cursor)
	return base64.StdEncoding.EncodeToString(data)
}

func decodeCursor(value string) (namespaceCursor, error) {
	if value == "" {
		return namespaceCursor{}, nil
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return namespaceCursor{}, err
	}
	var cursor namespaceCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return namespaceCursor{}, err
	}
	return cursor, nil
}

func applyCursor(resources []*resourceVersion, cursor namespaceCursor) []*resourceVersion {
	if cursor.LastKind == "" && cursor.LastName == "" {
		return resources
	}
	result := make([]*resourceVersion, 0, len(resources))
	for _, version := range resources {
		if version.meta.Kind > cursor.LastKind || (version.meta.Kind == cursor.LastKind && version.meta.Name > cursor.LastName) {
			result = append(result, version)
		}
	}
	return result
}

func (s *Store) specDiffWithinLookback(uid string, timestampNs, lookbackNs int64) string {
	record := s.projection.resourcesByUID[uid]
	if record == nil {
		return ""
	}

	var earliest, latest *resourceVersion
	startNs := timestampNs - lookbackNs
	for i := range record.versions {
		version := &record.versions[i]
		if version.timestamp > timestampNs || !versionHasPayload(version) {
			continue
		}
		if latest == nil || version.timestamp > latest.timestamp {
			latest = version
		}
		if version.timestamp >= startNs && (earliest == nil || version.timestamp < earliest.timestamp) {
			earliest = version
		}
	}
	if earliest == nil || latest == nil || earliest.timestamp == latest.timestamp {
		return ""
	}

	diffs, err := analysis.ComputeJSONDiff(versionPayload(earliest), versionPayload(latest))
	if err != nil {
		return ""
	}
	diffs = analysis.FilterSpecOnly(diffs)
	if len(diffs) == 0 {
		return ""
	}
	return analysis.FormatUnifiedDiff(diffs)
}

type currentEdge struct {
	id        string
	sourceUID string
	targetUID string
	relType   string
}

func (s *Store) namespaceEdgesForSet(versionsByUID map[string]*resourceVersion, timestampNs, lookbackNs int64) []analysisstore.NamespaceGraphEdge {
	edges := s.currentEdgesForVersions(versionsByUID, timestampNs, timestampNs-lookbackNs)
	result := make([]analysisstore.NamespaceGraphEdge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, analysisstore.NamespaceGraphEdge{
			ID:               edge.id,
			Source:           edge.sourceUID,
			Target:           edge.targetUID,
			RelationshipType: edge.relType,
		})
	}
	return result
}

func (s *Store) clusterScopedReachableFrom(start []*resourceVersion, timestampNs, lookbackNs int64, maxDepth int) []*resourceVersion {
	startNs := timestampNs - lookbackNs
	visible := s.visibleVersions(timestampNs, startNs)
	edges := s.currentEdgesForVersions(visible, timestampNs, startNs)
	adjacency := make(map[string][]string)
	for _, edge := range edges {
		adjacency[edge.sourceUID] = append(adjacency[edge.sourceUID], edge.targetUID)
		adjacency[edge.targetUID] = append(adjacency[edge.targetUID], edge.sourceUID)
	}

	seen := make(map[string]bool)
	queue := make([]struct {
		uid   string
		depth int
	}, 0, len(start))
	for _, version := range start {
		seen[version.meta.UID] = true
		queue = append(queue, struct {
			uid   string
			depth int
		}{uid: version.meta.UID})
	}

	var result []*resourceVersion
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.depth >= maxDepth {
			continue
		}
		for _, nextUID := range adjacency[item.uid] {
			if seen[nextUID] {
				continue
			}
			seen[nextUID] = true
			next := visible[nextUID]
			if next == nil {
				continue
			}
			if next.meta.Namespace == "" {
				result = append(result, next)
			}
			queue = append(queue, struct {
				uid   string
				depth int
			}{uid: nextUID, depth: item.depth + 1})
		}
	}
	return result
}

func (s *Store) visibleVersions(timestampNs, windowStartNs int64) map[string]*resourceVersion {
	result := make(map[string]*resourceVersion)
	for uid, record := range s.projection.resourcesByUID {
		version := record.visibleVersionWithinWindow(timestampNs, windowStartNs)
		if version == nil {
			continue
		}
		result[uid] = version
	}
	return result
}

func (s *Store) currentEdgesForVersions(versionsByUID map[string]*resourceVersion, timestampNs, windowStartNs int64) []currentEdge {
	seen := make(map[string]currentEdge)
	add := func(sourceUID, targetUID, relType string) {
		if sourceUID == "" || targetUID == "" || sourceUID == targetUID {
			return
		}
		if versionsByUID[targetUID] == nil || versionsByUID[sourceUID] == nil {
			return
		}
		key := sourceUID + "|" + relType + "|" + targetUID
		seen[key] = currentEdge{
			id:        key,
			sourceUID: sourceUID,
			targetUID: targetUID,
			relType:   relType,
		}
	}

	for _, version := range versionsByUID {
		for _, ownerUID := range ownerUIDs(parsedVersionObject(version)) {
			add(ownerUID, version.meta.UID, "OWNS")
		}
		for _, ref := range directReferences(version) {
			target := s.projection.resolveByName(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name, timestampNs, windowStartNs)
			if target == nil {
				continue
			}
			add(version.meta.UID, target.meta.UID, ref.relType)
		}
		if manager := managerReference(version); manager != nil {
			managerVersion := s.projection.resolveByName(manager.namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			if managerVersion == nil && manager.namespace == "" {
				managerVersion = s.projection.resolveByName(version.meta.Namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			}
			if managerVersion != nil {
				add(managerVersion.meta.UID, version.meta.UID, "MANAGES")
			}
		}
	}

	pods := make([]*resourceVersion, 0)
	for _, version := range versionsByUID {
		if version.meta.Kind == "Pod" && version.eventType() != models.EventTypeDelete {
			pods = append(pods, version)
		}
	}
	for _, version := range versionsByUID {
		if !resourceUsesPodSelector(version.meta.Kind) || version.eventType() == models.EventTypeDelete {
			continue
		}
		for _, pod := range pods {
			if pod.meta.Namespace != version.meta.Namespace {
				continue
			}
			if selectorMatches(selectorSpec(version), pod.labels()) {
				add(version.meta.UID, pod.meta.UID, "SELECTS")
			}
		}
	}

	edges := make([]currentEdge, 0, len(seen))
	for _, edge := range seen {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].relType != edges[j].relType {
			return edges[i].relType < edges[j].relType
		}
		if edges[i].sourceUID != edges[j].sourceUID {
			return edges[i].sourceUID < edges[j].sourceUID
		}
		return edges[i].targetUID < edges[j].targetUID
	})
	return edges
}
