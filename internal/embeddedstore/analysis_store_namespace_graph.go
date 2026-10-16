package embeddedstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

// GetNamespaceGraph returns one page of a namespace's resources at a point in time, the
// cluster-scoped resources within MaxDepth hops of them, and the edges between all of
// those. Edges come from in-memory graph facts and reverse indexes; payloads are read
// only for the spec diffs of the returned nodes, after the projection lock is released.
func (s *Store) GetNamespaceGraph(_ context.Context, input analysisstore.NamespaceGraphQuery) (*analysisstore.NamespaceGraphData, error) {
	startTime := time.Now()

	s.projection.mu.RLock()
	// Read the field directly: RetentionWindowNs takes the read lock again, which blocks
	// forever if a writer queues up between the two acquisitions.
	query := normalizeNamespaceQuery(input, s.projection.retentionWindowNs)

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

	type specDiffJob struct {
		node             int
		earliest, latest resourceVersion
	}
	var diffJobs []specDiffJob
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
		node.LatestEvent = &analysisstore.NamespaceGraphChangeEvent{
			TimestampNs:  version.timestamp,
			EventType:    string(version.eventType()),
			Status:       version.status(),
			SpecReplicas: factsFor(version).specReplicas(),
		}
		if earliest, latest, ok := s.specDiffVersions(version.meta.UID, query.TimestampNs, query.LookbackNs); ok {
			// Copies: versions are immutable apart from payload references, and a copy
			// keeps the reference it was taken with.
			diffJobs = append(diffJobs, specDiffJob{node: len(nodes), earliest: *earliest, latest: *latest})
		}
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
	s.projection.mu.RUnlock()

	for i := range diffJobs {
		nodes[diffJobs[i].node].LatestEvent.SpecChanges = specDiff(&diffJobs[i].earliest, &diffJobs[i].latest)
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

// specDiffVersions returns the versions a node's spec diff compares: the earliest with
// a payload inside the lookback window and the latest with a payload at or before
// timestampNs (the first of equal timestamps in both cases). ok is false when there is
// nothing to compare.
func (s *Store) specDiffVersions(uid string, timestampNs, lookbackNs int64) (*resourceVersion, *resourceVersion, bool) {
	record := s.projection.resourcesByUID[uid]
	if record == nil {
		return nil, nil, false
	}
	versions := record.versions
	end := sort.Search(len(versions), func(i int) bool { return versions[i].timestamp > timestampNs })

	var latest *resourceVersion
	for i := end - 1; i >= 0; i-- {
		version := &versions[i]
		if latest != nil && version.timestamp != latest.timestamp {
			break
		}
		if versionHasPayload(version) {
			latest = version
		}
	}

	startNs := timestampNs - lookbackNs
	var earliest *resourceVersion
	for i := sort.Search(end, func(i int) bool { return versions[i].timestamp >= startNs }); i < end; i++ {
		if versionHasPayload(&versions[i]) {
			earliest = &versions[i]
			break
		}
	}
	if earliest == nil || latest == nil || earliest.timestamp == latest.timestamp {
		return nil, nil, false
	}
	return earliest, latest, true
}

// specDiff formats the spec-only differences between two versions' payloads.
func specDiff(earliest, latest *resourceVersion) string {
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

// graphWalk resolves the neighbours of resources visible at one point in time. A
// resource is visible when it exists at timestampNs or was deleted inside the window;
// edges only connect visible resources.
type graphWalk struct {
	store         *Store
	timestampNs   int64
	windowStartNs int64
	// namespaces caches the visible pods and pod-selecting resources per namespace.
	namespaces map[string]*namespaceSelectorSet
}

type namespaceSelectorSet struct {
	pods      []*resourceVersion
	selectors []*resourceVersion
}

func (w *graphWalk) visible(record *resourceRecord) *resourceVersion {
	if record == nil {
		return nil
	}
	return record.visibleVersionWithinWindow(w.timestampNs, w.windowStartNs)
}

func (w *graphWalk) resolve(namespace, kind, name string) *resourceVersion {
	return w.store.projection.resolveByName(namespace, kind, name, w.timestampNs, w.windowStartNs)
}

// resolveManager resolves a GitOps manager reference, falling back to the resource's own
// namespace when the reference has none.
func (w *graphWalk) resolveManager(version *resourceVersion) *resourceVersion {
	manager := managerReference(version)
	if manager == nil {
		return nil
	}
	resolved := w.resolve(manager.namespace, manager.kind, manager.name)
	if resolved == nil && manager.namespace == "" {
		resolved = w.resolve(version.meta.Namespace, manager.kind, manager.name)
	}
	return resolved
}

func (w *graphWalk) selectorSet(namespace string) *namespaceSelectorSet {
	if set, ok := w.namespaces[namespace]; ok {
		return set
	}
	set := &namespaceSelectorSet{}
	for _, record := range uniqueRecords(w.store.projection.graph.byNamespace[namespace]) {
		version := w.visible(record)
		if version == nil || version.isDelete() || version.meta.Namespace != namespace {
			continue
		}
		switch {
		case version.meta.Kind == "Pod":
			set.pods = append(set.pods, version)
		case resourceUsesPodSelector(version.meta.Kind):
			set.selectors = append(set.selectors, version)
		}
	}
	if w.namespaces == nil {
		w.namespaces = make(map[string]*namespaceSelectorSet)
	}
	w.namespaces[namespace] = set
	return set
}

// neighbours returns the visible resources sharing an edge with version, in either
// direction. With clusterScopedOnly it skips namespaced neighbours, which lets the last
// hop of a walk avoid selector matching (selectors only connect resources of one
// namespace).
func (w *graphWalk) neighbours(version *resourceVersion, clusterScopedOnly bool) []*resourceVersion {
	uid := version.meta.UID
	var result []*resourceVersion
	add := func(other *resourceVersion) {
		if other == nil || other.meta.UID == "" || other.meta.UID == uid {
			return
		}
		if clusterScopedOnly && other.meta.Namespace != "" {
			return
		}
		result = append(result, other)
	}
	projection := w.store.projection
	facts := factsFor(version)

	// Outgoing: owners, name references and the GitOps manager.
	for _, owner := range facts.owners() {
		add(w.visible(projection.resourcesByUID[owner]))
	}
	for ref := range facts.refs() {
		add(w.resolve(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name))
	}
	add(w.resolveManager(version))

	// Incoming: children, and resources whose references or manager labels resolve here.
	for _, record := range uniqueRecords(projection.graph.byOwner[uid]) {
		if child := w.visible(record); child != nil && containsString(factsFor(child).owners(), uid) {
			add(child)
		}
	}
	key := resourceKey{namespace: version.meta.Namespace, kind: version.meta.Kind, name: version.meta.Name}
	for _, record := range uniqueRecords(projection.graph.byTarget[key]) {
		source := w.visible(record)
		if source == nil {
			continue
		}
		if w.referencesResolveTo(source, key, uid) {
			add(source)
		}
	}

	// Selector edges: pods and the resources selecting them share a namespace.
	if clusterScopedOnly && version.meta.Namespace != "" || version.isDelete() {
		return result
	}
	set := w.selectorSet(version.meta.Namespace)
	switch {
	case version.meta.Kind == "Pod":
		for _, selector := range set.selectors {
			if factsFor(selector).selector().matches(version.labels()) {
				add(selector)
			}
		}
	case resourceUsesPodSelector(version.meta.Kind):
		for _, pod := range set.pods {
			if facts.selector().matches(pod.labels()) {
				add(pod)
			}
		}
	}
	return result
}

// referencesResolveTo reports whether a name reference or the manager of source names
// key and resolves to the resource uid.
func (w *graphWalk) referencesResolveTo(source *resourceVersion, key resourceKey, uid string) bool {
	for ref := range factsFor(source).refs() {
		refKey := resourceKey{namespace: ref.namespaceFor(source.meta.Namespace), kind: ref.kind, name: ref.name}
		if refKey != key {
			continue
		}
		if target := w.resolve(refKey.namespace, refKey.kind, refKey.name); target != nil && target.meta.UID == uid {
			return true
		}
	}
	if manager := w.resolveManager(source); manager != nil && manager.meta.UID == uid {
		return true
	}
	return false
}

// clusterScopedReachableFrom returns the cluster-scoped resources within maxDepth hops of
// start. The walk expands only the resources it reaches.
func (s *Store) clusterScopedReachableFrom(start []*resourceVersion, timestampNs, lookbackNs int64, maxDepth int) []*resourceVersion {
	walk := &graphWalk{store: s, timestampNs: timestampNs, windowStartNs: timestampNs - lookbackNs}

	type queued struct {
		version *resourceVersion
		depth   int
	}
	seen := make(map[string]bool, len(start))
	queue := make([]queued, 0, len(start))
	for _, version := range start {
		seen[version.meta.UID] = true
		queue = append(queue, queued{version: version})
	}

	var result []*resourceVersion
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if item.depth >= maxDepth {
			continue
		}
		// Resources found at the last hop are not expanded, so only cluster-scoped ones
		// matter there.
		lastHop := item.depth+1 >= maxDepth
		for _, next := range walk.neighbours(item.version, lastHop) {
			if seen[next.meta.UID] {
				continue
			}
			seen[next.meta.UID] = true
			if next.meta.Namespace == "" {
				result = append(result, next)
			}
			queue = append(queue, queued{version: next, depth: item.depth + 1})
		}
	}
	return result
}

// currentEdgesForVersions returns the edges among versionsByUID.
func (s *Store) currentEdgesForVersions(versionsByUID map[string]*resourceVersion, timestampNs, windowStartNs int64) []currentEdge {
	walk := &graphWalk{store: s, timestampNs: timestampNs, windowStartNs: windowStartNs}
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

	podsByNamespace := make(map[string][]*resourceVersion)
	for _, version := range versionsByUID {
		facts := factsFor(version)
		for _, ownerUID := range facts.owners() {
			add(ownerUID, version.meta.UID, "OWNS")
		}
		for ref := range facts.refs() {
			if target := walk.resolve(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name); target != nil {
				add(version.meta.UID, target.meta.UID, ref.relType)
			}
		}
		if managerVersion := walk.resolveManager(version); managerVersion != nil {
			add(managerVersion.meta.UID, version.meta.UID, "MANAGES")
		}
		if version.meta.Kind == "Pod" && !version.isDelete() {
			podsByNamespace[version.meta.Namespace] = append(podsByNamespace[version.meta.Namespace], version)
		}
	}
	for _, version := range versionsByUID {
		if !resourceUsesPodSelector(version.meta.Kind) || version.isDelete() {
			continue
		}
		selector := factsFor(version).selector()
		for _, pod := range podsByNamespace[version.meta.Namespace] {
			if selector.matches(pod.labels()) {
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
