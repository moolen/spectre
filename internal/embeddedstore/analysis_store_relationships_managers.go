package embeddedstore

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
