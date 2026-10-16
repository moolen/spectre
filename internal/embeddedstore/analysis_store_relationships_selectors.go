package embeddedstore

import "sort"

func (s *Store) selectingResourcesForTarget(target *resourceVersion, timestampNs int64) []*resourceVersion {
	if target.meta.Kind != "Pod" {
		return nil
	}

	var result []*resourceVersion
	for _, record := range s.projection.resourcesByUID {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.meta.Namespace != target.meta.Namespace {
			continue
		}
		if !resourceUsesPodSelector(version.meta.Kind) {
			continue
		}
		if selectorMatches(selectorSpec(version), target.labels()) {
			result = append(result, version)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].meta.Kind != result[j].meta.Kind {
			return result[i].meta.Kind < result[j].meta.Kind
		}
		return result[i].meta.Name < result[j].meta.Name
	})

	return result
}

func (s *Store) ingressesReferencingService(service *resourceVersion, failureTimestampNs, windowStartNs int64) []*resourceVersion {
	var result []*resourceVersion
	for _, record := range s.projection.resourcesByUID {
		version := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs)
		if version == nil || version.meta.Kind != "Ingress" || version.meta.Namespace != service.meta.Namespace {
			continue
		}
		for _, ref := range ingressReferences(version) {
			if ref.kind == "Service" && ref.name == service.meta.Name {
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
		if version == nil || (version.meta.Kind != "RoleBinding" && version.meta.Kind != "ClusterRoleBinding") {
			continue
		}
		for _, ref := range roleBindingReferences(version) {
			if ref.relType != "GRANTS_TO" {
				continue
			}
			namespace := ref.namespaceFor(version.meta.Namespace)
			if ref.kind == "ServiceAccount" && ref.name == serviceAccount.meta.Name && namespace == serviceAccount.meta.Namespace {
				result = append(result, version)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].meta.Kind != result[j].meta.Kind {
			return result[i].meta.Kind < result[j].meta.Kind
		}
		return result[i].meta.Name < result[j].meta.Name
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
	spec := getMap(parsedVersionObject(version), "spec")
	switch version.meta.Kind {
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
