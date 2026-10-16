package embeddedstore

import "sort"

// selectingResourcesForTarget returns the resources active at timestampNs whose pod
// selector matches the target pod.
func (s *Store) selectingResourcesForTarget(target *resourceVersion, timestampNs int64) []*resourceVersion {
	if target.meta.Kind != "Pod" {
		return nil
	}

	var result []*resourceVersion
	for _, record := range uniqueRecords(s.projection.graph.byNamespace[target.meta.Namespace]) {
		version := record.activeVersionAt(timestampNs)
		if version == nil || version.meta.Namespace != target.meta.Namespace {
			continue
		}
		if !resourceUsesPodSelector(version.meta.Kind) {
			continue
		}
		if factsFor(version).selector().matches(target.labels()) {
			result = append(result, version)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return versionIdentityLess(result[i], result[j])
	})

	return result
}

func (s *Store) ingressesReferencingService(service *resourceVersion, failureTimestampNs, windowStartNs int64) []*resourceVersion {
	var result []*resourceVersion
	key := resourceKey{namespace: service.meta.Namespace, kind: "Service", name: service.meta.Name}
	for _, record := range uniqueRecords(s.projection.graph.byTarget[key]) {
		version := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs)
		if version == nil || version.meta.Kind != "Ingress" || version.meta.Namespace != service.meta.Namespace {
			continue
		}
		for ref := range factsFor(version).refs() {
			if ref.kind == "Service" && ref.name == service.meta.Name {
				result = append(result, version)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return versionIdentityLess(result[i], result[j])
	})
	return result
}

func (s *Store) roleBindingsGrantingToServiceAccount(serviceAccount *resourceVersion, failureTimestampNs, windowStartNs int64) []*resourceVersion {
	var result []*resourceVersion
	key := resourceKey{namespace: serviceAccount.meta.Namespace, kind: "ServiceAccount", name: serviceAccount.meta.Name}
	for _, record := range uniqueRecords(s.projection.graph.byTarget[key]) {
		version := record.visibleVersionWithinWindow(failureTimestampNs, windowStartNs)
		if version == nil || (version.meta.Kind != "RoleBinding" && version.meta.Kind != "ClusterRoleBinding") {
			continue
		}
		for ref := range factsFor(version).refs() {
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
		return versionIdentityLess(result[i], result[j])
	})

	return result
}

// versionIdentityLess orders versions by kind, name, namespace and UID.
func versionIdentityLess(a, b *resourceVersion) bool {
	if a.meta.Kind != b.meta.Kind {
		return a.meta.Kind < b.meta.Kind
	}
	if a.meta.Name != b.meta.Name {
		return a.meta.Name < b.meta.Name
	}
	if a.meta.Namespace != b.meta.Namespace {
		return a.meta.Namespace < b.meta.Namespace
	}
	return a.meta.UID < b.meta.UID
}

// uniqueRecords drops repeated index entries (a record indexed under one key by several
// of its metas).
func uniqueRecords(records []*resourceRecord) []*resourceRecord {
	if len(records) < 2 {
		return records
	}
	seen := make(map[*resourceRecord]struct{}, len(records))
	result := make([]*resourceRecord, 0, len(records))
	for _, record := range records {
		if _, ok := seen[record]; ok {
			continue
		}
		seen[record] = struct{}{}
		result = append(result, record)
	}
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

// selectorSpecFromObject returns the pod selector of an object of kind, in
// metav1.LabelSelector form (Service selectors become matchLabels).
func selectorSpecFromObject(kind string, object map[string]any) map[string]any {
	spec := getMap(object, "spec")
	switch kind {
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
