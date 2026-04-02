package embedded

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
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

type Store struct {
	snapshot *snapshot
}

var _ analysisstore.AnalysisStore = (*Store)(nil)

func New(events []models.Event) (*Store, error) {
	snapshot, err := buildSnapshot(events)
	if err != nil {
		return nil, err
	}
	return &Store{snapshot: snapshot}, nil
}

func (s *Store) GetResource(_ context.Context, uid string) (*graph.ResourceIdentity, error) {
	record := s.snapshot.resourcesByUID[uid]
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
	if maxDepth <= 0 {
		maxDepth = 3
	}
	record := s.snapshot.resourcesByUID[uid]
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
			for _, ownerUID := range ownerUIDs(version.object) {
				if ownerUID == "" || seen[ownerUID] {
					continue
				}
				ownerRecord := s.snapshot.resourcesByUID[ownerUID]
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
	result := make(map[string]*analysisstore.ManagerData)
	for _, uid := range resourceUIDs {
		record := s.snapshot.resourcesByUID[uid]
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
			targetNamespace = version.identity.Namespace
		}
		managerVersion := s.snapshot.resolveByName(targetNamespace, managerRef.kind, managerRef.name, version.timestamp, 0)
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
	result := make(map[string][]analysisstore.RelatedResourceData)
	startNs := window.Start()
	for _, uid := range resourceUIDs {
		record := s.snapshot.resourcesByUID[uid]
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
			key := relType + ":" + target.identity.UID + ":" + referenceTargetUID
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

		for _, ref := range directReferences(version) {
			target := s.snapshot.resolveByName(ref.namespaceFor(version.identity.Namespace), ref.kind, ref.name, window.FailureTimestampNs, startNs)
			add(target, ref.relType, "")
			if ref.relType == "USES_SERVICE_ACCOUNT" && target != nil {
				for _, binding := range s.roleBindingsGrantingToServiceAccount(target, window.FailureTimestampNs, startNs) {
					add(binding, "GRANTS_TO", "")
					for _, roleRef := range roleBindingReferences(binding) {
						if roleRef.relType != "BINDS_ROLE" {
							continue
						}
						role := s.snapshot.resolveByName(
							roleRef.namespaceFor(binding.identity.Namespace),
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
			if selector.identity.Kind == "Service" {
				for _, ingress := range s.ingressesReferencingService(selector, window.FailureTimestampNs, startNs) {
					add(ingress, edgeTypeIngress, selector.identity.UID)
				}
			}
		}

		result[uid] = items
	}
	return result, nil
}

func (s *Store) GetChangeEvents(_ context.Context, resourceUIDs []string, window analysisstore.ResourceWindow) (map[string][]analysisstore.ChangeEventInfo, error) {
	result := make(map[string][]analysisstore.ChangeEventInfo)
	startNs := window.Start()
	for _, uid := range resourceUIDs {
		record := s.snapshot.resourcesByUID[uid]
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
			if version.changeEvent.ConfigChanged {
				configEvents = append(configEvents, version.changeEvent)
			}
			if len(recentEvents) < maxRecentEvents {
				recentEvents = append(recentEvents, version.changeEvent)
			}
		}

		events := mergeChangeEvents(configEvents, recentEvents)
		if len(events) > 0 {
			result[uid] = events
		}
	}
	return result, nil
}

func (s *Store) GetK8sEvents(_ context.Context, resourceUIDs []string, window analysisstore.ResourceWindow) (map[string][]analysisstore.K8sEventInfo, error) {
	result := make(map[string][]analysisstore.K8sEventInfo)
	startNs := window.Start()
	for _, uid := range resourceUIDs {
		events := s.snapshot.k8sEventsByInvolvedUID[uid]
		if len(events) == 0 {
			continue
		}
		filtered := make([]analysisstore.K8sEventInfo, 0, maxK8sEvents)
		for _, event := range events {
			ts := event.Timestamp.UnixNano()
			if ts < startNs || ts > window.FailureTimestampNs {
				continue
			}
			filtered = append(filtered, event)
			if len(filtered) == maxK8sEvents {
				break
			}
		}
		if len(filtered) > 0 {
			result[uid] = filtered
		}
	}
	return result, nil
}

func (s *Store) GetNamespaceGraph(_ context.Context, input analysisstore.NamespaceGraphQuery) (*analysisstore.NamespaceGraphData, error) {
	startTime := time.Now()
	query := normalizeNamespaceQuery(input)

	namespacedResources := s.snapshot.activeResourcesInNamespace(query.Namespace, query.TimestampNs)
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
		selectedUIDs[version.identity.UID] = version
	}

	for _, version := range s.clusterScopedReachableFrom(pagedResources, query.TimestampNs, query.LookbackNs, query.MaxDepth) {
		selectedUIDs[version.identity.UID] = version
	}

	allVersions := make([]*resourceVersion, 0, len(selectedUIDs))
	for _, version := range selectedUIDs {
		allVersions = append(allVersions, version)
	}
	sort.Slice(allVersions, func(i, j int) bool {
		if allVersions[i].identity.Kind != allVersions[j].identity.Kind {
			return allVersions[i].identity.Kind < allVersions[j].identity.Kind
		}
		if allVersions[i].identity.Name != allVersions[j].identity.Name {
			return allVersions[i].identity.Name < allVersions[j].identity.Name
		}
		return allVersions[i].identity.UID < allVersions[j].identity.UID
	})

	nodes := make([]analysisstore.NamespaceGraphNode, 0, len(allVersions))
	for _, version := range allVersions {
		node := analysisstore.NamespaceGraphNode{
			UID:       version.identity.UID,
			Kind:      version.identity.Kind,
			APIGroup:  version.identity.APIGroup,
			Namespace: version.identity.Namespace,
			Name:      version.identity.Name,
			Status:    version.changeEvent.Status,
			Labels:    copyStringMap(version.identity.Labels),
		}
		latestEvent := analysisstore.NamespaceGraphChangeEvent{
			TimestampNs:     version.timestamp,
			EventType:       string(version.eventType),
			Status:          version.changeEvent.Status,
			SpecChanges:     s.specDiffWithinLookback(version.identity.UID, query.TimestampNs, query.LookbackNs),
			SpecReplicas:    specReplicas(version.object),
		}
		node.LatestEvent = &latestEvent
		nodes = append(nodes, node)
	}

	edges := s.namespaceEdgesForSet(selectedUIDs, query.TimestampNs, query.LookbackNs)

	var nextCursor string
	if hasMore && len(pagedResources) > 0 {
		last := pagedResources[len(pagedResources)-1]
		nextCursor = encodeCursor(namespaceCursor{
			LastKind: last.identity.Kind,
			LastName: last.identity.Name,
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

func normalizeNamespaceQuery(input analysisstore.NamespaceGraphQuery) analysisstore.NamespaceGraphQuery {
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
	if normalized.LookbackNs > maxLookbackNs {
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
		if version.identity.Kind > cursor.LastKind || (version.identity.Kind == cursor.LastKind && version.identity.Name > cursor.LastName) {
			result = append(result, version)
		}
	}
	return result
}

type directRef struct {
	kind      string
	name      string
	namespace string
	relType   string
}

func (r directRef) namespaceFor(defaultNamespace string) string {
	if r.namespace != "" {
		return r.namespace
	}
	if isClusterScopedKind(r.kind) {
		return ""
	}
	return defaultNamespace
}

func isClusterScopedKind(kind string) bool {
	switch kind {
	case "Node", "ClusterRole", "ClusterRoleBinding", "ClusterIssuer", "ClusterSecretStore", "GatewayClass":
		return true
	default:
		return false
	}
}

func ownerUIDs(object map[string]any) []string {
	metadata := getMap(object, "metadata")
	items := getSlice(metadata, "ownerReferences")
	result := make([]string, 0, len(items))
	for _, item := range items {
		owner, _ := item.(map[string]any)
		if uid := getString(owner, "uid"); uid != "" {
			result = append(result, uid)
		}
	}
	return result
}

func directReferences(version *resourceVersion) []directRef {
	result := make([]directRef, 0)
	switch version.identity.Kind {
	case "Pod":
		result = append(result, podReferences(version)...)
	case "Ingress":
		result = append(result, ingressReferences(version)...)
	case "RoleBinding", "ClusterRoleBinding":
		result = append(result, roleBindingReferences(version)...)
	case "HelmRelease":
		result = append(result, helmReleaseReferences(version)...)
	case "Kustomization":
		result = append(result, sourceRefReferences(version, "GitRepository", "Bucket", "OCIRepository")...)
	case "HTTPRoute":
		result = append(result, sourceRefReferences(version, "Gateway", "Service")...)
	}
	if version.identity.Kind == "Pod" {
		if nodeName := getString(getMap(version.object, "spec"), "nodeName"); nodeName != "" {
			result = append(result, directRef{kind: "Node", name: nodeName, relType: "SCHEDULED_ON"})
		}
		serviceAccountName := getString(getMap(version.object, "spec"), "serviceAccountName")
		if serviceAccountName == "" {
			serviceAccountName = getString(getMap(version.object, "spec"), "serviceAccount")
		}
		if serviceAccountName != "" {
			result = append(result, directRef{kind: "ServiceAccount", name: serviceAccountName, namespace: version.identity.Namespace, relType: "USES_SERVICE_ACCOUNT"})
		}
	}
	return result
}

func podReferences(version *resourceVersion) []directRef {
	spec := getMap(version.object, "spec")
	result := make([]directRef, 0)
	for _, volumeItem := range getSlice(spec, "volumes") {
		volume, _ := volumeItem.(map[string]any)
		if configMap := getMap(volume, "configMap"); configMap != nil {
			if name := getString(configMap, "name"); name != "" {
				result = append(result, directRef{kind: "ConfigMap", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
			}
		}
		if secret := getMap(volume, "secret"); secret != nil {
			if name := getString(secret, "secretName"); name != "" {
				result = append(result, directRef{kind: "Secret", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
			}
		}
		if projected := getMap(volume, "projected"); projected != nil {
			for _, sourceItem := range getSlice(projected, "sources") {
				source, _ := sourceItem.(map[string]any)
				if configMap := getMap(source, "configMap"); configMap != nil {
					if name := getString(configMap, "name"); name != "" {
						result = append(result, directRef{kind: "ConfigMap", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
				if secret := getMap(source, "secret"); secret != nil {
					if name := getString(secret, "name"); name != "" {
						result = append(result, directRef{kind: "Secret", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
			}
		}
	}
	for _, containerGroup := range [][]any{getSlice(spec, "containers"), getSlice(spec, "initContainers")} {
		for _, containerItem := range containerGroup {
			container, _ := containerItem.(map[string]any)
			for _, envFromItem := range getSlice(container, "envFrom") {
				envFrom, _ := envFromItem.(map[string]any)
				if configMapRef := getMap(envFrom, "configMapRef"); configMapRef != nil {
					if name := getString(configMapRef, "name"); name != "" {
						result = append(result, directRef{kind: "ConfigMap", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
				if secretRef := getMap(envFrom, "secretRef"); secretRef != nil {
					if name := getString(secretRef, "name"); name != "" {
						result = append(result, directRef{kind: "Secret", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
			}
			for _, envItem := range getSlice(container, "env") {
				env, _ := envItem.(map[string]any)
				valueFrom := getMap(env, "valueFrom")
				if configMapRef := getMap(valueFrom, "configMapKeyRef"); configMapRef != nil {
					if name := getString(configMapRef, "name"); name != "" {
						result = append(result, directRef{kind: "ConfigMap", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
				if secretRef := getMap(valueFrom, "secretKeyRef"); secretRef != nil {
					if name := getString(secretRef, "name"); name != "" {
						result = append(result, directRef{kind: "Secret", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
					}
				}
			}
		}
	}
	return dedupeRefs(result)
}

func ingressReferences(version *resourceVersion) []directRef {
	spec := getMap(version.object, "spec")
	result := make([]directRef, 0)
	if backend := getMap(spec, "defaultBackend"); backend != nil {
		if service := getMap(backend, "service"); service != nil {
			if name := getString(service, "name"); name != "" {
				result = append(result, directRef{kind: "Service", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
			}
		}
	}
	for _, ruleItem := range getSlice(spec, "rules") {
		rule, _ := ruleItem.(map[string]any)
		httpRule := getMap(rule, "http")
		for _, pathItem := range getSlice(httpRule, "paths") {
			pathMap, _ := pathItem.(map[string]any)
			backend := getMap(pathMap, "backend")
			service := getMap(backend, "service")
			if name := getString(service, "name"); name != "" {
				result = append(result, directRef{kind: "Service", name: name, namespace: version.identity.Namespace, relType: "REFERENCES_SPEC"})
			}
		}
	}
	return dedupeRefs(result)
}

func roleBindingReferences(version *resourceVersion) []directRef {
	result := make([]directRef, 0)
	roleRef := getMap(version.object, "roleRef")
	roleKind := getString(roleRef, "kind")
	roleName := getString(roleRef, "name")
	if roleKind != "" && roleName != "" {
		result = append(result, directRef{kind: roleKind, name: roleName, relType: "BINDS_ROLE"})
	}
	for _, subjectItem := range getSlice(version.object, "subjects") {
		subject, _ := subjectItem.(map[string]any)
		if getString(subject, "kind") != "ServiceAccount" {
			continue
		}
		result = append(result, directRef{
			kind:      "ServiceAccount",
			name:      getString(subject, "name"),
			namespace: getString(subject, "namespace"),
			relType:   "GRANTS_TO",
		})
	}
	return dedupeRefs(result)
}

func helmReleaseReferences(version *resourceVersion) []directRef {
	spec := getMap(version.object, "spec")
	result := sourceRefReferences(version, "GitRepository", "Bucket", "OCIRepository")
	for _, valueItem := range getSlice(spec, "valuesFrom") {
		valueRef, _ := valueItem.(map[string]any)
		kind := getString(valueRef, "kind")
		if kind == "" {
			kind = "ConfigMap"
		}
		name := getString(valueRef, "name")
		if name == "" {
			continue
		}
		namespace := getString(valueRef, "namespace")
		result = append(result, directRef{kind: kind, name: name, namespace: namespace, relType: "REFERENCES_SPEC"})
	}
	return dedupeRefs(result)
}

func sourceRefReferences(version *resourceVersion, allowedKinds ...string) []directRef {
	spec := getMap(version.object, "spec")
	sourceRef := getMap(spec, "sourceRef")
	if len(sourceRef) == 0 {
		chart := getMap(spec, "chart")
		if chart != nil {
			sourceRef = getMap(chart, "sourceRef")
			if sourceRef == nil {
				sourceSpec := getMap(chart, "spec")
				sourceRef = getMap(sourceSpec, "sourceRef")
			}
		}
	}
	if len(sourceRef) == 0 {
		return nil
	}
	kind := getString(sourceRef, "kind")
	name := getString(sourceRef, "name")
	namespace := getString(sourceRef, "namespace")
	if kind == "" || name == "" {
		return nil
	}
	if len(allowedKinds) > 0 {
		matched := false
		for _, allowed := range allowedKinds {
			if allowed == kind {
				matched = true
				break
			}
		}
		if !matched {
			return nil
		}
	}
	return []directRef{{kind: kind, name: name, namespace: namespace, relType: "REFERENCES_SPEC"}}
}

func dedupeRefs(input []directRef) []directRef {
	result := make([]directRef, 0, len(input))
	seen := make(map[string]bool)
	for _, ref := range input {
		key := strings.Join([]string{ref.kind, ref.namespace, ref.name, ref.relType}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, ref)
	}
	return result
}

type managerRef struct {
	kind       string
	name       string
	namespace  string
	confidence float64
}

func managerReference(version *resourceVersion) *managerRef {
	labels := version.identity.Labels
	switch {
	case labels["helm.toolkit.fluxcd.io/name"] != "":
		return &managerRef{
			kind:       "HelmRelease",
			name:       labels["helm.toolkit.fluxcd.io/name"],
			namespace:  labels["helm.toolkit.fluxcd.io/namespace"],
			confidence: 1.0,
		}
	case labels["kustomize.toolkit.fluxcd.io/name"] != "":
		return &managerRef{
			kind:       "Kustomization",
			name:       labels["kustomize.toolkit.fluxcd.io/name"],
			namespace:  labels["kustomize.toolkit.fluxcd.io/namespace"],
			confidence: 1.0,
		}
	case labels["argocd.argoproj.io/instance"] != "":
		return &managerRef{
			kind:       "Application",
			name:       labels["argocd.argoproj.io/instance"],
			namespace:  version.identity.Namespace,
			confidence: 1.0,
		}
	default:
		return nil
	}
}

func (s *Store) selectingResourcesForTarget(target *resourceVersion, timestampNs int64) []*resourceVersion {
	if target.identity.Kind != "Pod" {
		return nil
	}
	var result []*resourceVersion
	for _, record := range s.snapshot.resourcesByUID {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.identity.Namespace != target.identity.Namespace {
			continue
		}
		if !resourceUsesPodSelector(version.identity.Kind) {
			continue
		}
		if selectorMatches(selectorSpec(version), target.identity.Labels) {
			result = append(result, version)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].identity.Kind != result[j].identity.Kind {
			return result[i].identity.Kind < result[j].identity.Kind
		}
		return result[i].identity.Name < result[j].identity.Name
	})
	return result
}

func (s *Store) ingressesReferencingService(service *resourceVersion, failureTimestampNs, windowStartNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range s.snapshot.resourcesByUID {
		version := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs)
		if version == nil || version.identity.Kind != "Ingress" || version.identity.Namespace != service.identity.Namespace {
			continue
		}
		for _, ref := range ingressReferences(version) {
			if ref.kind == "Service" && ref.name == service.identity.Name {
				result = append(result, version)
				break
			}
		}
	}
	return result
}

func (s *Store) roleBindingsGrantingToServiceAccount(serviceAccount *resourceVersion, failureTimestampNs, windowStartNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range s.snapshot.resourcesByUID {
		version := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs)
		if version == nil || (version.identity.Kind != "RoleBinding" && version.identity.Kind != "ClusterRoleBinding") {
			continue
		}
		for _, ref := range roleBindingReferences(version) {
			if ref.relType != "GRANTS_TO" {
				continue
			}
			namespace := ref.namespaceFor(version.identity.Namespace)
			if ref.kind == "ServiceAccount" && ref.name == serviceAccount.identity.Name && namespace == serviceAccount.identity.Namespace {
				result = append(result, version)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].identity.Kind != result[j].identity.Kind {
			return result[i].identity.Kind < result[j].identity.Kind
		}
		return result[i].identity.Name < result[j].identity.Name
	})
	return result
}

func resourceUsesPodSelector(kind string) bool {
	switch kind {
	case "Service", "NetworkPolicy", "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet":
		return true
	default:
		return false
	}
}

func selectorSpec(version *resourceVersion) map[string]any {
	spec := getMap(version.object, "spec")
	switch version.identity.Kind {
	case "Service":
		selector := getMap(spec, "selector")
		if selector == nil {
			return nil
		}
		return map[string]any{"matchLabels": selector}
	case "NetworkPolicy":
		return getMap(spec, "podSelector")
	default:
		return getMap(spec, "selector")
	}
}

func selectorMatches(selector map[string]any, labels map[string]string) bool {
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

func specReplicas(object map[string]any) *int {
	spec := getMap(object, "spec")
	value, ok := getInt(spec, "replicas")
	if !ok {
		return nil
	}
	replicas := int(value)
	return &replicas
}

func mergeChangeEvents(configEvents, recentEvents []analysisstore.ChangeEventInfo) []analysisstore.ChangeEventInfo {
	if len(configEvents) == 0 && len(recentEvents) == 0 {
		return nil
	}

	combined := make([]analysisstore.ChangeEventInfo, 0, len(configEvents)+len(recentEvents))
	seen := make(map[string]bool, len(configEvents)+len(recentEvents))
	appendUnique := func(events []analysisstore.ChangeEventInfo) {
		for _, event := range events {
			if seen[event.EventID] {
				continue
			}
			seen[event.EventID] = true
			combined = append(combined, event)
		}
	}

	appendUnique(configEvents)
	appendUnique(recentEvents)

	sort.Slice(combined, func(i, j int) bool {
		if combined[i].Timestamp.Equal(combined[j].Timestamp) {
			return combined[i].EventID > combined[j].EventID
		}
		return combined[i].Timestamp.After(combined[j].Timestamp)
	})
	return combined
}

func (s *Store) specDiffWithinLookback(uid string, timestampNs, lookbackNs int64) string {
	record := s.snapshot.resourcesByUID[uid]
	if record == nil {
		return ""
	}
	var earliest, latest *resourceVersion
	startNs := timestampNs - lookbackNs
	for i := range record.versions {
		version := &record.versions[i]
		if version.timestamp > timestampNs || len(version.data) == 0 {
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
	diffs, err := analysis.ComputeJSONDiff(earliest.data, latest.data)
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
	id         string
	sourceUID  string
	targetUID  string
	relType    string
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
		seen[version.identity.UID] = true
		queue = append(queue, struct {
			uid   string
			depth int
		}{uid: version.identity.UID})
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
			if next.identity.Namespace == "" {
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
	for uid, record := range s.snapshot.resourcesByUID {
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
		for _, ownerUID := range ownerUIDs(version.object) {
			add(ownerUID, version.identity.UID, "OWNS")
		}
		for _, ref := range directReferences(version) {
			target := s.snapshot.resolveByName(ref.namespaceFor(version.identity.Namespace), ref.kind, ref.name, timestampNs, windowStartNs)
			if target == nil {
				continue
			}
			add(version.identity.UID, target.identity.UID, ref.relType)
		}
		if manager := managerReference(version); manager != nil {
			managerVersion := s.snapshot.resolveByName(manager.namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			if managerVersion == nil && manager.namespace == "" {
				managerVersion = s.snapshot.resolveByName(version.identity.Namespace, manager.kind, manager.name, timestampNs, windowStartNs)
			}
			if managerVersion != nil {
				add(managerVersion.identity.UID, version.identity.UID, "MANAGES")
			}
		}
	}

	pods := make([]*resourceVersion, 0)
	for _, version := range versionsByUID {
		if version.identity.Kind == "Pod" && version.eventType != models.EventTypeDelete {
			pods = append(pods, version)
		}
	}
	for _, version := range versionsByUID {
		if !resourceUsesPodSelector(version.identity.Kind) || version.eventType == models.EventTypeDelete {
			continue
		}
		for _, pod := range pods {
			if pod.identity.Namespace != version.identity.Namespace {
				continue
			}
			if selectorMatches(selectorSpec(version), pod.identity.Labels) {
				add(version.identity.UID, pod.identity.UID, "SELECTS")
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
