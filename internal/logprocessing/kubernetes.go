package logprocessing

import "regexp"

// Kubernetes resource naming pattern regexes
var (
	// k8sPodPattern matches Kubernetes pod names with format:
	// <deployment>-<replicaset-hash>-<pod-hash>
	// Example: nginx-deployment-66b6c48dd5-8w7xz
	k8sPodPattern = regexp.MustCompile(`\b[a-z0-9-]+-[a-z0-9]{8,10}-[a-z0-9]{5}\b`)

	// k8sReplicaSetPattern matches Kubernetes replicaset names with format:
	// <deployment>-<hash>
	// Example: nginx-deployment-66b6c48dd5
	k8sReplicaSetPattern = regexp.MustCompile(`\b[a-z0-9-]+-[a-z0-9]{8,10}\b`)
)

// MaskKubernetesNames replaces dynamic Kubernetes resource names with <K8S_NAME> placeholder.
// Order matters: pod pattern is a superset of replicaset pattern, so it must be applied first.
//
// User decision from CONTEXT.md: "pod names (app-xyz-abc123) become <K8S_NAME>"
func MaskKubernetesNames(template string) string {
	// Replace pod names first (more specific pattern)
	template = k8sPodPattern.ReplaceAllString(template, "<K8S_NAME>")

	// Then replace replicaset names
	template = k8sReplicaSetPattern.ReplaceAllString(template, "<K8S_NAME>")

	return template
}
