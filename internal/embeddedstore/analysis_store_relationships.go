package embeddedstore

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

type managerRef struct {
	kind       string
	name       string
	namespace  string
	confidence float64
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
