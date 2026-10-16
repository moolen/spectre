package embeddedstore

func managerReference(version *resourceVersion) *managerRef {
	meta := version.metaOrEmpty()
	return managerReferenceFor(meta.Labels, meta.Namespace)
}

// managerReferenceFor derives the GitOps manager of a resource in namespace from its
// labels.
func managerReferenceFor(labels map[string]string, namespace string) *managerRef {
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
			namespace:  namespace,
			confidence: 1.0,
		}
	default:
		return nil
	}
}
