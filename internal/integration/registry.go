package integration

import (
	"fmt"
	"sort"
	"sync"
)

// Registry manages integration instances at runtime.
// Stores instances by unique name and provides thread-safe operations
// for adding, retrieving, removing, and listing instances.
//
// Multiple instances of the same integration type can be registered
// with different names (e.g., "victorialogs-prod", "victorialogs-staging").
type Registry struct {
	instances map[string]Integration
	mu        sync.RWMutex
}

// NewRegistry creates a new empty integration instance registry
func NewRegistry() *Registry {
	return &Registry{
		instances: make(map[string]Integration),
	}
}

// Register adds an integration instance to the registry.
// Returns error if:
//   - name is empty string
//   - name already exists in registry
//
// Thread-safe for concurrent registration.
func (r *Registry) Register(name string, integration Integration) error {
	if name == "" {
		return fmt.Errorf("instance name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[name]; exists {
		return fmt.Errorf("instance %q is already registered", name)
	}

	r.instances[name] = integration
	return nil
}

// Get retrieves an integration instance by name.
// Returns (instance, true) if found, (nil, false) if not registered.
// Thread-safe for concurrent reads.
func (r *Registry) Get(name string) (Integration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, exists := r.instances[name]
	return instance, exists
}

// List returns a sorted list of all registered instance names.
// Thread-safe for concurrent reads.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.instances))
	for name := range r.instances {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// Remove removes an integration instance from the registry.
// Returns true if the instance existed and was removed, false if it didn't exist.
// Thread-safe for concurrent removal.
func (r *Registry) Remove(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.instances[name]
	if exists {
		delete(r.instances, name)
	}

	return exists
}
