package helpers

import (
	"fmt"
	"sync"
)

// SharedDeployment represents a Spectre deployment that is shared across multiple tests.
// Each test creates its own namespace for test resources, but connects to this shared
// deployment via port-forward for API access.
type SharedDeployment struct {
	// Name is the unique identifier for this deployment (e.g., "standard", "flux")
	Name string

	// Namespace is the Kubernetes namespace where Spectre is deployed
	Namespace string

	// ReleaseName is the Helm release name for this deployment
	ReleaseName string

	// Cluster is the Kind cluster where this deployment runs
	Cluster *TestCluster
}

var (
	// deploymentRegistry stores shared deployments by name
	deploymentRegistry = make(map[string]*SharedDeployment)
	// registryMu protects concurrent access to the registry
	registryMu sync.RWMutex
)

// RegisterSharedDeployment adds a shared deployment to the global registry.
// This is typically called in TestMain after deploying shared Spectre instances.
func RegisterSharedDeployment(key string, deployment *SharedDeployment) {
	registryMu.Lock()
	defer registryMu.Unlock()
	deploymentRegistry[key] = deployment
}

// GetSharedDeployment retrieves a shared deployment from the registry by key.
// Returns an error if the deployment is not found.
func GetSharedDeployment(key string) (*SharedDeployment, error) {
	registryMu.RLock()
	defer registryMu.RUnlock()

	dep, exists := deploymentRegistry[key]
	if !exists {
		return nil, fmt.Errorf("shared deployment %q not found in registry", key)
	}
	return dep, nil
}

// ListSharedDeployments returns a copy of all registered shared deployments.
// Useful for debugging and cleanup operations.
func ListSharedDeployments() map[string]*SharedDeployment {
	registryMu.RLock()
	defer registryMu.RUnlock()

	// Create a copy to avoid external mutations
	result := make(map[string]*SharedDeployment, len(deploymentRegistry))
	for k, v := range deploymentRegistry {
		result[k] = v
	}
	return result
}

// ClearRegistry removes all entries from the deployment registry.
// This is primarily used for testing the registry itself.
func ClearRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	deploymentRegistry = make(map[string]*SharedDeployment)
}
