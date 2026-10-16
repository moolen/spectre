package embeddedstore

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

// The analysis store derives edges from graph facts and reverse indexes. These tests
// check it against the previous implementation (kept below as legacy*), which parsed
// every payload and scanned every record, on randomized clusters.

func TestGraphFactsParityWithPayloadScan(t *testing.T) {
	for seed := int64(1); seed <= 6; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			events, checkpoints := randomGraphCluster(seed)
			projection := NewProjection()
			for _, event := range events {
				require.NoError(t, projection.Apply(event))
			}
			store := NewAnalysisStore(projection)
			assertGraphParity(t, store, checkpoints)

			// Retention trims history and drops records; indexes must follow.
			projection.ApplyRetentionCutoff(checkpoints[1])
			assertGraphParity(t, store, checkpoints[1:])
		})
	}
}

func TestGraphFactsSurviveCheckpointRoundTrip(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			events, checkpoints := randomGraphCluster(11)
			projection := NewProjection()
			for _, event := range events {
				require.NoError(t, projection.Apply(event))
			}
			if legacy {
				// Checkpoints written before facts existed carry none.
				for _, record := range projection.resourcesByUID {
					for i := range record.versions {
						if record.versions[i].meta != nil {
							record.versions[i].meta.graph = nil
						}
					}
				}
			}
			root := t.TempDir()
			meta, err := writeCheckpointV3(root, projection, projection.captureCheckpointSnapshot(1))
			require.NoError(t, err)
			restored, _, err := loadCheckpoint(root, meta)
			require.NoError(t, err)
			store := NewAnalysisStore(restored)
			for _, record := range restored.resourcesByUID {
				for i := range record.versions {
					if versionHasPayload(&record.versions[i]) {
						require.NotNil(t, record.versions[i].meta.graph, "facts missing after restore")
					}
				}
			}
			assertGraphParity(t, store, checkpoints)
		})
	}
}

func assertGraphParity(t *testing.T, store *Store, timestamps []int64) {
	t.Helper()
	ctx := context.Background()
	relTypes := make(map[string]bool)
	clusterScopedNodes := 0
	store.projection.mu.RLock()
	var pods []string
	for uid, record := range store.projection.resourcesByUID {
		if latest := record.latestVersion(); latest != nil && latest.meta.Kind == "Pod" {
			pods = append(pods, uid)
		}
	}
	store.projection.mu.RUnlock()
	sort.Strings(pods)

	for _, ts := range timestamps {
		for _, namespace := range []string{"ns-a", "ns-b", "ns-c", ""} {
			for _, depth := range []int{1, 2, 4} {
				for _, limit := range []int{3, 50} {
					query := analysisstore.NamespaceGraphQuery{Namespace: namespace, TimestampNs: ts, LookbackNs: int64(20 * time.Minute), MaxDepth: depth, Limit: limit}
					for page := 0; page < 20; page++ {
						got, err := store.GetNamespaceGraph(ctx, query)
						require.NoError(t, err)
						want := legacyGetNamespaceGraph(store, query)
						got.Metadata.QueryExecutionMs, want.Metadata.QueryExecutionMs = 0, 0
						require.Equal(t, want, got, "namespace graph %+v", query)
						for _, edge := range got.Graph.Edges {
							relTypes[edge.RelationshipType] = true
						}
						for _, node := range got.Graph.Nodes {
							if node.Namespace == "" && namespace != "" {
								clusterScopedNodes++
							}
						}
						if !got.Metadata.HasMore {
							break
						}
						query.Cursor = got.Metadata.NextCursor
					}
				}
			}
		}
		window := analysisstore.ResourceWindow{FailureTimestampNs: ts, LookbackNs: int64(20 * time.Minute)}
		got, err := store.GetRelatedResources(ctx, pods, window)
		require.NoError(t, err)
		require.Equal(t, legacyGetRelatedResources(store, pods, window), got, "related resources at %d", ts)
		for _, uid := range pods {
			chain, err := store.GetOwnershipChain(ctx, uid, ts, 5)
			require.NoError(t, err)
			require.Equal(t, legacyOwnershipChain(store, uid, ts, 5), chain)
		}
	}
	// The generated clusters must exercise every edge type.
	for _, relType := range []string{"OWNS", "SELECTS", "REFERENCES_SPEC", "SCHEDULED_ON", "USES_SERVICE_ACCOUNT", "MANAGES", "GRANTS_TO", "BINDS_ROLE"} {
		require.True(t, relTypes[relType], "no %s edge compared", relType)
	}
	require.Positive(t, clusterScopedNodes, "no cluster-scoped node reached")
}

// randomGraphCluster generates events touching every edge type: owners, selectors
// (matchLabels, matchExpressions, Services, NetworkPolicies, malformed ones), volume and
// env references, scheduling, service accounts and the bindings granting to them across
// namespaces, Ingress backends, Flux and Argo CD manager labels and Flux sources; with
// label and selector changes, rescheduling, deletes with and without payloads, name
// reuse and out-of-order delivery. It returns the events and timestamps to query at.
func randomGraphCluster(seed int64) ([]models.Event, []int64) {
	rng := rand.New(rand.NewSource(seed))
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	clock := base
	seq := 0
	var events []models.Event
	type object struct {
		meta   models.ResourceMetadata
		body   map[string]any
		alive  bool
		labels map[string]any
	}
	var objects []*object
	namespaces := []string{"ns-a", "ns-b", "ns-c"}
	apps := []string{"web", "api", "db"}
	emit := func(eventType models.EventType, obj *object, withData bool) {
		seq++
		clock += int64(time.Duration(rng.Intn(90)+1) * time.Second)
		ts := clock
		if rng.Intn(15) == 0 {
			ts -= int64(time.Duration(rng.Intn(120)) * time.Second) // late delivery
		}
		var data []byte
		if withData {
			metadata := map[string]any{"name": obj.meta.Name, "uid": obj.meta.UID, "labels": obj.labels}
			if obj.meta.Namespace != "" {
				metadata["namespace"] = obj.meta.Namespace
			}
			if owner, ok := obj.body["_owner"].(string); ok {
				metadata["ownerReferences"] = []any{map[string]any{"uid": owner, "kind": "ReplicaSet", "name": "x"}}
			}
			body := map[string]any{"apiVersion": "v1", "kind": obj.meta.Kind, "metadata": metadata}
			for key, value := range obj.body {
				if key != "_owner" {
					body[key] = value
				}
			}
			data, _ = json.Marshal(body)
		}
		if eventType == models.EventTypeDelete {
			obj.alive = false
		}
		events = append(events, models.Event{
			ID: fmt.Sprintf("ev-%06d", seq), Timestamp: ts, Type: eventType, Resource: obj.meta, Data: data,
		})
	}
	newObject := func(kind, namespace, name string, labels map[string]any, body map[string]any) *object {
		seq++
		obj := &object{
			meta:   models.ResourceMetadata{Version: "v1", Kind: kind, Namespace: namespace, Name: name, UID: fmt.Sprintf("uid-%05d", seq)},
			body:   body,
			alive:  true,
			labels: labels,
		}
		objects = append(objects, obj)
		emit(models.EventTypeCreate, obj, true)
		return obj
	}
	pick := func(values []string) string { return values[rng.Intn(len(values))] }
	selector := func(app string) map[string]any {
		switch rng.Intn(6) {
		case 0:
			return map[string]any{"matchExpressions": []any{map[string]any{"key": "app", "operator": "In", "values": []any{app, "other"}}}}
		case 1:
			return map[string]any{"matchLabels": map[string]any{"app": app}, "matchExpressions": []any{map[string]any{"key": "tier", "operator": "NotIn", "values": []any{"cache"}}}}
		case 2:
			return map[string]any{"matchLabels": map[string]any{"app": app}, "matchExpressions": []any{map[string]any{"key": "tier", "operator": pick([]string{"Exists", "DoesNotExist"})}}}
		case 3:
			return map[string]any{"matchLabels": map[string]any{"app": 7}}
		default:
			return map[string]any{"matchLabels": map[string]any{"app": app}}
		}
	}
	podLabels := func(app string) map[string]any {
		labels := map[string]any{"app": app}
		if rng.Intn(3) == 0 {
			labels["tier"] = pick([]string{"cache", "front"})
		}
		switch rng.Intn(5) {
		case 0:
			labels["helm.toolkit.fluxcd.io/name"] = "release"
			if rng.Intn(2) == 0 {
				labels["helm.toolkit.fluxcd.io/namespace"] = pick(namespaces)
			}
		case 1:
			labels["argocd.argoproj.io/instance"] = "argo-app"
		}
		return labels
	}

	for _, node := range []string{"n1", "n2", "n3"} {
		newObject("Node", "", node, map[string]any{}, map[string]any{"spec": map[string]any{}})
	}
	newObject("ClusterRole", "", "reader", map[string]any{}, map[string]any{"rules": []any{}})
	newObject("GitRepository", "ns-a", "platform", map[string]any{}, map[string]any{"spec": map[string]any{}})
	newObject("ClusterRoleBinding", "", "readers", map[string]any{}, map[string]any{
		"roleRef":  map[string]any{"kind": "ClusterRole", "name": "reader"},
		"subjects": []any{map[string]any{"kind": "ServiceAccount", "name": "web", "namespace": "ns-b"}, map[string]any{"kind": "ServiceAccount", "name": "api"}},
	})
	var pods, mutable []*object
	for _, ns := range namespaces {
		newObject("HelmRelease", ns, "release", map[string]any{}, map[string]any{"spec": map[string]any{
			"chart":      map[string]any{"spec": map[string]any{"sourceRef": map[string]any{"kind": "GitRepository", "name": "platform", "namespace": "ns-a"}}},
			"valuesFrom": []any{map[string]any{"name": "web-config"}, map[string]any{"kind": "Secret", "name": "api-secret", "namespace": "ns-c"}},
		}})
		newObject("Application", ns, "argo-app", map[string]any{}, map[string]any{})
		newObject("Kustomization", ns, "infra", map[string]any{}, map[string]any{"spec": map[string]any{"sourceRef": map[string]any{"kind": "GitRepository", "name": "platform", "namespace": "ns-a"}}})
		for _, app := range apps {
			newObject("ConfigMap", ns, app+"-config", podLabels(app), map[string]any{"data": map[string]any{}})
			newObject("Secret", ns, app+"-secret", map[string]any{}, map[string]any{})
			newObject("ServiceAccount", ns, app, map[string]any{}, map[string]any{})
			newObject("Role", ns, app, map[string]any{}, map[string]any{})
			mutable = append(mutable, newObject("RoleBinding", ns, app, map[string]any{}, map[string]any{
				"roleRef": map[string]any{"kind": pick([]string{"Role", "ClusterRole"}), "name": pick([]string{app, "reader"})},
				"subjects": []any{
					map[string]any{"kind": "ServiceAccount", "name": app},
					map[string]any{"kind": "ServiceAccount", "name": pick(apps), "namespace": pick(namespaces)},
				},
			}))
			deploy := newObject("Deployment", ns, app, podLabels(app), map[string]any{"spec": map[string]any{"replicas": rng.Intn(4), "selector": selector(app)}})
			mutable = append(mutable, deploy)
			mutable = append(mutable, newObject("Service", ns, app, map[string]any{}, map[string]any{"spec": map[string]any{"selector": map[string]any{"app": app}}}))
			if rng.Intn(2) == 0 {
				mutable = append(mutable, newObject("NetworkPolicy", ns, app, map[string]any{}, map[string]any{"spec": map[string]any{"podSelector": selector(app)}}))
			}
			if rng.Intn(2) == 0 {
				mutable = append(mutable, newObject("Ingress", ns, app, map[string]any{}, map[string]any{"spec": map[string]any{
					"defaultBackend": map[string]any{"service": map[string]any{"name": pick(apps)}},
					"rules":          []any{map[string]any{"http": map[string]any{"paths": []any{map[string]any{"backend": map[string]any{"service": map[string]any{"name": app}}}}}}},
				}}))
			}
			rs := newObject("ReplicaSet", ns, app+"-1", podLabels(app), map[string]any{"_owner": deploy.meta.UID, "spec": map[string]any{"selector": selector(app)}})
			mutable = append(mutable, rs)
			for i := 0; i < 1+rng.Intn(3); i++ {
				spec := map[string]any{
					"serviceAccountName": app,
					"volumes":            []any{map[string]any{"configMap": map[string]any{"name": app + "-config"}}, map[string]any{"secret": map[string]any{"secretName": pick(apps) + "-secret"}}},
					"containers":         []any{map[string]any{"envFrom": []any{map[string]any{"configMapRef": map[string]any{"name": pick(apps) + "-config"}}}}},
				}
				if rng.Intn(3) > 0 {
					spec["nodeName"] = pick([]string{"n1", "n2", "n3"})
				}
				pods = append(pods, newObject("Pod", ns, fmt.Sprintf("%s-%d", app, i), podLabels(app), map[string]any{"_owner": rs.meta.UID, "spec": spec}))
			}
		}
	}
	var checkpoints []int64
	for step := 0; step < 400; step++ {
		if step%100 == 50 {
			checkpoints = append(checkpoints, clock)
		}
		switch roll := rng.Intn(10); {
		case roll < 4:
			pod := pods[rng.Intn(len(pods))]
			if !pod.alive {
				continue
			}
			spec := pod.body["spec"].(map[string]any)
			if rng.Intn(2) == 0 {
				spec["nodeName"] = pick([]string{"n1", "n2", "n3"})
			} else {
				pod.labels = podLabels(pick(apps))
			}
			emit(models.EventTypeUpdate, pod, true)
		case roll < 6:
			obj := mutable[rng.Intn(len(mutable))]
			if !obj.alive {
				continue
			}
			if spec, ok := obj.body["spec"].(map[string]any); ok {
				if _, has := spec["selector"]; has && obj.meta.Kind != "Service" {
					spec["selector"] = selector(pick(apps))
				}
				if _, has := spec["replicas"]; has {
					spec["replicas"] = rng.Intn(5)
				}
			}
			if obj.meta.Kind == "RoleBinding" {
				obj.body["subjects"] = []any{map[string]any{"kind": "ServiceAccount", "name": pick(apps), "namespace": pick(namespaces)}}
			}
			emit(models.EventTypeUpdate, obj, true)
		case roll < 8:
			pod := pods[rng.Intn(len(pods))]
			if !pod.alive {
				continue
			}
			emit(models.EventTypeDelete, pod, rng.Intn(2) == 0)
			// Recreate under the same name with a new UID.
			spec := map[string]any{"serviceAccountName": pick(apps), "nodeName": pick([]string{"n1", "n2", "n3"})}
			pods = append(pods, newObject("Pod", pod.meta.Namespace, pod.meta.Name, podLabels(pick(apps)), map[string]any{"_owner": pod.body["_owner"], "spec": spec}))
		default:
			obj := mutable[rng.Intn(len(mutable))]
			if obj.alive && rng.Intn(4) == 0 {
				emit(models.EventTypeDelete, obj, rng.Intn(2) == 0)
			}
		}
	}
	checkpoints = append(checkpoints, clock, clock+int64(10*time.Minute))
	return events, checkpoints
}

// --- previous implementation, parsing payloads ---

func legacyOwnerUIDs(version *resourceVersion) []string {
	return ownerUIDs(parsedVersionObject(version))
}

func legacyDirectReferences(version *resourceVersion) []directRef {
	return directReferencesFromObject(version.meta.Kind, version.meta.Namespace, parsedVersionObject(version))
}

func legacySelectorMatches(selector map[string]any, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	matchLabels := getMap(selector, "matchLabels")
	if len(matchLabels) == 0 && len(selector) > 0 {
		matchLabels = selector
	}
	for key, value := range matchLabels {
		stringValue, ok := value.(string)
		if !ok || labels[key] != stringValue {
			return false
		}
	}
	for _, exprItem := range getSlice(selector, "matchExpressions") {
		expr, _ := exprItem.(map[string]any)
		key := getString(expr, "key")
		operator := getString(expr, "operator")
		values := getSlice(expr, "values")
		labelValue, hasLabel := labels[key]
		switch operator {
		case "In":
			matched := false
			for _, value := range values {
				if stringValue, ok := value.(string); ok && labelValue == stringValue {
					matched = true
					break
				}
			}
			if !matched {
				return false
			}
		case "NotIn":
			for _, value := range values {
				if stringValue, ok := value.(string); ok && labelValue == stringValue {
					return false
				}
			}
		case "Exists":
			if !hasLabel {
				return false
			}
		case "DoesNotExist":
			if hasLabel {
				return false
			}
		}
	}
	return true
}

func legacySelector(version *resourceVersion) map[string]any {
	return selectorSpecFromObject(version.meta.Kind, parsedVersionObject(version))
}

func legacyGetNamespaceGraph(s *Store, input analysisstore.NamespaceGraphQuery) *analysisstore.NamespaceGraphData {
	p := s.projection
	p.mu.RLock()
	defer p.mu.RUnlock()
	query := normalizeNamespaceQuery(input, p.retentionWindowNs)

	var namespaced []*resourceVersion
	for _, record := range p.resourcesByUID {
		version := record.activeVersionAt(query.TimestampNs)
		if version == nil || version.meta.Kind == "Event" || version.meta.Namespace != query.Namespace {
			continue
		}
		namespaced = append(namespaced, version)
	}
	sort.Slice(namespaced, func(i, j int) bool {
		if namespaced[i].meta.Kind != namespaced[j].meta.Kind {
			return namespaced[i].meta.Kind < namespaced[j].meta.Kind
		}
		if namespaced[i].meta.Name != namespaced[j].meta.Name {
			return namespaced[i].meta.Name < namespaced[j].meta.Name
		}
		return namespaced[i].meta.UID < namespaced[j].meta.UID
	})
	cursor, err := decodeCursor(query.Cursor)
	if err != nil {
		cursor = namespaceCursor{}
	}
	paged := legacyApplyCursor(namespaced, cursor)
	hasMore := len(paged) > query.Limit
	if hasMore {
		paged = paged[:query.Limit]
	}
	selected := make(map[string]*resourceVersion)
	for _, version := range paged {
		selected[version.meta.UID] = version
	}
	for _, version := range legacyClusterScopedReachableFrom(s, paged, query.TimestampNs, query.LookbackNs, query.MaxDepth) {
		selected[version.meta.UID] = version
	}
	all := make([]*resourceVersion, 0, len(selected))
	for _, version := range selected {
		all = append(all, version)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].meta.Kind != all[j].meta.Kind {
			return all[i].meta.Kind < all[j].meta.Kind
		}
		if all[i].meta.Name != all[j].meta.Name {
			return all[i].meta.Name < all[j].meta.Name
		}
		return all[i].meta.UID < all[j].meta.UID
	})
	nodes := make([]analysisstore.NamespaceGraphNode, 0, len(all))
	for _, version := range all {
		nodes = append(nodes, analysisstore.NamespaceGraphNode{
			UID: version.meta.UID, Kind: version.meta.Kind, APIGroup: version.meta.APIGroup,
			Namespace: version.meta.Namespace, Name: version.meta.Name, Status: version.status(),
			Labels: copyStringMap(version.labels()),
			LatestEvent: &analysisstore.NamespaceGraphChangeEvent{
				TimestampNs: version.timestamp, EventType: string(version.eventType()), Status: version.status(),
				SpecChanges:  legacySpecDiff(s, version.meta.UID, query.TimestampNs, query.LookbackNs),
				SpecReplicas: specReplicas(parsedVersionObject(version)),
			},
		})
	}
	var edges []analysisstore.NamespaceGraphEdge
	for _, edge := range legacyEdges(s, selected, query.TimestampNs, query.TimestampNs-query.LookbackNs) {
		edges = append(edges, analysisstore.NamespaceGraphEdge{ID: edge.id, Source: edge.sourceUID, Target: edge.targetUID, RelationshipType: edge.relType})
	}
	if edges == nil {
		edges = []analysisstore.NamespaceGraphEdge{}
	}
	var next string
	if hasMore && len(paged) > 0 {
		last := paged[len(paged)-1]
		next = encodeCursor(namespaceCursor{LastKind: last.meta.Kind, LastName: last.meta.Name})
	}
	return &analysisstore.NamespaceGraphData{
		Graph: analysisstore.NamespaceGraph{Nodes: nodes, Edges: edges},
		Metadata: analysisstore.NamespaceGraphMetadata{
			Namespace: query.Namespace, TimestampNs: query.TimestampNs, NodeCount: len(nodes), EdgeCount: len(edges),
			HasMore: hasMore, NextCursor: next,
		},
	}
}

func legacySpecDiff(s *Store, uid string, timestampNs, lookbackNs int64) string {
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
	return specDiff(earliest, latest)
}

func legacyClusterScopedReachableFrom(s *Store, start []*resourceVersion, timestampNs, lookbackNs int64, maxDepth int) []*resourceVersion {
	startNs := timestampNs - lookbackNs
	visible := make(map[string]*resourceVersion)
	for uid, record := range s.projection.resourcesByUID {
		if version := record.visibleVersionWithinWindow(timestampNs, startNs); version != nil {
			visible[uid] = version
		}
	}
	adjacency := make(map[string][]string)
	for _, edge := range legacyEdges(s, visible, timestampNs, startNs) {
		adjacency[edge.sourceUID] = append(adjacency[edge.sourceUID], edge.targetUID)
		adjacency[edge.targetUID] = append(adjacency[edge.targetUID], edge.sourceUID)
	}
	type item struct {
		uid   string
		depth int
	}
	seen := make(map[string]bool)
	var queue []item
	for _, version := range start {
		seen[version.meta.UID] = true
		queue = append(queue, item{uid: version.meta.UID})
	}
	var result []*resourceVersion
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.depth >= maxDepth {
			continue
		}
		for _, next := range adjacency[current.uid] {
			if seen[next] {
				continue
			}
			seen[next] = true
			version := visible[next]
			if version == nil {
				continue
			}
			if version.meta.Namespace == "" {
				result = append(result, version)
			}
			queue = append(queue, item{uid: next, depth: current.depth + 1})
		}
	}
	return result
}

func legacyEdges(s *Store, versionsByUID map[string]*resourceVersion, timestampNs, windowStartNs int64) []currentEdge {
	p := s.projection
	seen := make(map[string]currentEdge)
	add := func(sourceUID, targetUID, relType string) {
		if sourceUID == "" || targetUID == "" || sourceUID == targetUID || versionsByUID[targetUID] == nil || versionsByUID[sourceUID] == nil {
			return
		}
		key := sourceUID + "|" + relType + "|" + targetUID
		seen[key] = currentEdge{id: key, sourceUID: sourceUID, targetUID: targetUID, relType: relType}
	}
	for _, version := range versionsByUID {
		for _, ownerUID := range legacyOwnerUIDs(version) {
			add(ownerUID, version.meta.UID, "OWNS")
		}
		for _, ref := range legacyDirectReferences(version) {
			if target := p.resolveByName(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name, timestampNs, windowStartNs); target != nil {
				add(version.meta.UID, target.meta.UID, ref.relType)
			}
		}
		if manager := managerReference(version); manager != nil {
			managerVersion := p.resolveByName(manager.namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			if managerVersion == nil && manager.namespace == "" {
				managerVersion = p.resolveByName(version.meta.Namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			}
			if managerVersion != nil {
				add(managerVersion.meta.UID, version.meta.UID, "MANAGES")
			}
		}
	}
	for _, version := range versionsByUID {
		if !resourceUsesPodSelector(version.meta.Kind) || version.eventType() == models.EventTypeDelete {
			continue
		}
		selector := legacySelector(version)
		for _, pod := range versionsByUID {
			if pod.meta.Kind == "Pod" && pod.eventType() != models.EventTypeDelete && pod.meta.Namespace == version.meta.Namespace && legacySelectorMatches(selector, pod.labels()) {
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

func legacyOwnershipChain(s *Store, uid string, atTimestampNs int64, maxDepth int) []analysisstore.ResourceWithDistance {
	p := s.projection
	p.mu.RLock()
	defer p.mu.RUnlock()
	record := p.resourcesByUID[uid]
	start := record.versionAt(atTimestampNs)
	if start == nil {
		start = record.latestVersion()
	}
	result := []analysisstore.ResourceWithDistance{{Resource: copyIdentity(start)}}
	seen := map[string]bool{uid: true}
	current := []*resourceVersion{start}
	for depth := 1; depth <= maxDepth; depth++ {
		var next []*resourceVersion
		for _, version := range current {
			for _, ownerUID := range legacyOwnerUIDs(version) {
				if ownerUID == "" || seen[ownerUID] {
					continue
				}
				ownerRecord := p.resourcesByUID[ownerUID]
				if ownerRecord == nil {
					continue
				}
				ownerVersion := ownerRecord.versionAt(atTimestampNs)
				if ownerVersion == nil {
					continue
				}
				seen[ownerUID] = true
				result = append(result, analysisstore.ResourceWithDistance{Resource: copyIdentity(ownerVersion), Distance: depth})
				next = append(next, ownerVersion)
			}
		}
		current = next
	}
	return result
}

func legacyGetRelatedResources(s *Store, resourceUIDs []string, window analysisstore.ResourceWindow) map[string][]analysisstore.RelatedResourceData {
	p := s.projection
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make(map[string][]analysisstore.RelatedResourceData)
	startNs := window.Start()
	visibleOfKind := func(match func(*resourceVersion) bool) []*resourceVersion {
		var out []*resourceVersion
		for _, record := range p.resourcesByUID {
			if version := record.visibleVersionWithinWindow(window.FailureTimestampNs, startNs); version != nil && match(version) {
				out = append(out, version)
			}
		}
		sort.Slice(out, func(i, j int) bool {
			return versionIdentityLess(out[i], out[j])
		})
		return out
	}
	for _, uid := range resourceUIDs {
		record := p.resourcesByUID[uid]
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
			items = append(items, analysisstore.RelatedResourceData{Resource: copyIdentity(target), RelationshipType: relType, ReferenceTargetUID: referenceTargetUID})
		}
		for _, ref := range legacyDirectReferences(version) {
			target := p.resolveByName(ref.namespaceFor(version.meta.Namespace), ref.kind, ref.name, window.FailureTimestampNs, startNs)
			add(target, ref.relType, "")
			if ref.relType != "USES_SERVICE_ACCOUNT" || target == nil {
				continue
			}
			bindings := visibleOfKind(func(candidate *resourceVersion) bool {
				if candidate.meta.Kind != "RoleBinding" && candidate.meta.Kind != "ClusterRoleBinding" {
					return false
				}
				for _, subject := range legacyDirectReferences(candidate) {
					if subject.relType == "GRANTS_TO" && subject.kind == "ServiceAccount" && subject.name == target.meta.Name && subject.namespaceFor(candidate.meta.Namespace) == target.meta.Namespace {
						return true
					}
				}
				return false
			})
			for _, binding := range bindings {
				add(binding, "GRANTS_TO", "")
				for _, roleRef := range legacyDirectReferences(binding) {
					if roleRef.relType == "BINDS_ROLE" {
						add(p.resolveByName(roleRef.namespaceFor(binding.meta.Namespace), roleRef.kind, roleRef.name, window.FailureTimestampNs, startNs), "BINDS_ROLE", "")
					}
				}
			}
		}
		if version.meta.Kind == "Pod" {
			var selectors []*resourceVersion
			for _, candidateRecord := range p.resourcesByUID {
				candidate := candidateRecord.activeVersionAt(window.FailureTimestampNs)
				if candidate != nil && candidate.meta.Namespace == version.meta.Namespace && resourceUsesPodSelector(candidate.meta.Kind) && legacySelectorMatches(legacySelector(candidate), version.labels()) {
					selectors = append(selectors, candidate)
				}
			}
			sort.Slice(selectors, func(i, j int) bool {
				return versionIdentityLess(selectors[i], selectors[j])
			})
			for _, selector := range selectors {
				add(selector, "SELECTS", "")
				if selector.meta.Kind != "Service" {
					continue
				}
				ingresses := visibleOfKind(func(candidate *resourceVersion) bool {
					if candidate.meta.Kind != "Ingress" || candidate.meta.Namespace != selector.meta.Namespace {
						return false
					}
					for _, ref := range legacyDirectReferences(candidate) {
						if ref.kind == "Service" && ref.name == selector.meta.Name {
							return true
						}
					}
					return false
				})
				for _, ingress := range ingresses {
					add(ingress, edgeTypeIngress, selector.meta.UID)
				}
			}
		}
		result[uid] = items
	}
	return result
}

// Graph queries run under the projection read lock while ingest, flushes and
// checkpoints mutate indexes, facts and payload references; run them together under the
// race detector.
func TestGraphQueriesConcurrentWithIngest(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 64, HotMaxResourceVersions: 16})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, engine.Close()) })
	events, timestamps := randomGraphCluster(21)
	store := engine.AnalysisStore()
	ctx := context.Background()

	done := make(chan struct{})
	errs := make(chan error, 4)
	for w := 0; w < 3; w++ {
		go func(w int) {
			for i := 0; ; i++ {
				select {
				case <-done:
					errs <- nil
					return
				default:
				}
				ts := timestamps[(i+w)%len(timestamps)]
				if _, err := store.GetNamespaceGraph(ctx, analysisstore.NamespaceGraphQuery{
					Namespace: []string{"ns-a", "ns-b", "ns-c"}[(i+w)%3], TimestampNs: ts, LookbackNs: int64(time.Hour), MaxDepth: 1 + i%3, Limit: 50,
				}); err != nil {
					errs <- err
					return
				}
				if _, err := store.GetRelatedResources(ctx, []string{"uid-00050", "uid-00100"}, analysisstore.ResourceWindow{FailureTimestampNs: ts, LookbackNs: int64(time.Hour)}); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	for i := 0; i < len(events); i += 25 {
		require.NoError(t, engine.ProcessBatch(ctx, events[i:min(i+25, len(events))]))
		if i%200 == 0 {
			require.NoError(t, engine.Flush(ctx))
		}
		if i%500 == 0 {
			require.NoError(t, engine.Checkpoint(ctx))
		}
	}
	close(done)
	for w := 0; w < 3; w++ {
		require.NoError(t, <-errs)
	}
	assertGraphParity(t, store, timestamps[len(timestamps)-2:])
}

func legacyApplyCursor(resources []*resourceVersion, cursor namespaceCursor) []*resourceVersion {
	if cursor.LastKind == "" && cursor.LastName == "" {
		return resources
	}
	result := make([]*resourceVersion, 0, len(resources))
	for _, version := range resources {
		if cursor.before(version) {
			result = append(result, version)
		}
	}
	return result
}
