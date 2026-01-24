package integration

import (
	"fmt"
	"sort"
	"sync"
)

// IntegrationFactory is a function that creates a new integration instance.
// name: unique instance name (e.g., "victorialogs-prod")
// config: instance-specific configuration as key-value map
// Returns: initialized Integration instance or error
type IntegrationFactory func(name string, config map[string]interface{}) (Integration, error)

// FactoryRegistry stores integration factory functions for compile-time type discovery.
// This implements PLUG-01 (convention-based discovery) using Go's init() or explicit
// registration in main(), NOT runtime filesystem scanning.
//
// Usage pattern:
//
//	// In integration package (e.g., internal/integration/victorialogs/victorialogs.go):
//	func init() {
//	  integration.RegisterFactory("victorialogs", NewVictoriaLogsIntegration)
//	}
//
//	// Or explicit registration in main():
//	func main() {
//	  integration.RegisterFactory("victorialogs", victorialogs.NewVictoriaLogsIntegration)
//	}
type FactoryRegistry struct {
	factories map[string]IntegrationFactory
	mu        sync.RWMutex
}

// defaultRegistry is the global factory registry used by package-level functions
var defaultRegistry = NewFactoryRegistry()

// NewFactoryRegistry creates a new empty factory registry
func NewFactoryRegistry() *FactoryRegistry {
	return &FactoryRegistry{
		factories: make(map[string]IntegrationFactory),
	}
}

// Register adds a factory function for the given integration type.
// Returns error if:
//   - integrationType is empty string
//   - integrationType is already registered
//
// Thread-safe for concurrent registration (though typically done at init time)
func (r *FactoryRegistry) Register(integrationType string, factory IntegrationFactory) error {
	if integrationType == "" {
		return fmt.Errorf("integration type cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[integrationType]; exists {
		return fmt.Errorf("integration type %q is already registered", integrationType)
	}

	r.factories[integrationType] = factory
	return nil
}

// Get retrieves the factory function for the given integration type.
// Returns (factory, true) if found, (nil, false) if not registered.
// Thread-safe for concurrent reads.
func (r *FactoryRegistry) Get(integrationType string) (IntegrationFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[integrationType]
	return factory, exists
}

// List returns a sorted list of all registered integration types.
// Thread-safe for concurrent reads.
func (r *FactoryRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}

	sort.Strings(types)
	return types
}

// RegisterFactory registers a factory function with the default global registry.
// This is the primary API for integration packages to register themselves.
func RegisterFactory(integrationType string, factory IntegrationFactory) error {
	return defaultRegistry.Register(integrationType, factory)
}

// GetFactory retrieves a factory function from the default global registry.
func GetFactory(integrationType string) (IntegrationFactory, bool) {
	return defaultRegistry.Get(integrationType)
}

// ListFactories returns all registered integration types from the default global registry.
func ListFactories() []string {
	return defaultRegistry.List()
}
