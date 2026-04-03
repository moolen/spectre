package embeddedstore

import (
	"sort"
	"strings"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
)

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
	for _, record := range s.projection.resourcesByUID {
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
	for _, record := range s.projection.resourcesByUID {
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
	for _, record := range s.projection.resourcesByUID {
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
