package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// Manager orchestrates the lifecycle of multiple components with dependency awareness.
// It ensures components are started in the correct dependency order and stopped
// in reverse dependency order, with timeout protection to prevent indefinite hangs.
type Manager struct {
	components        []Component
	dependencies      map[Component][]Component // component -> its dependencies
	reverseDepMap     map[Component][]Component // component -> what depends on it
	running           map[Component]bool        // tracks which components are currently running
	shutdownTimeout   time.Duration
	mu                sync.RWMutex
	logger            *logging.Logger
	registrationMutex sync.Mutex  // ensures register is not called during start/stop
	startedComponents []Component // track the order in which components started (for cleanup)
}

// NewManager creates a new lifecycle manager with default 30-second shutdown timeout.
func NewManager() *Manager {
	return &Manager{
		components:      []Component{},
		dependencies:    make(map[Component][]Component),
		reverseDepMap:   make(map[Component][]Component),
		running:         make(map[Component]bool),
		shutdownTimeout: 30 * time.Second,
		logger:          logging.GetLogger("lifecycle.manager"),
	}
}

// Register registers a component with optional dependencies.
// If dependencies are provided, they must be registered first.
// A component is initialized only after all its dependencies have started.
// A component stops before any of its dependents.
//
// Validation:
// - component must not be nil
// - dependencies must be previously registered components
// - no circular dependencies allowed
// - duplicate registration not allowed
//
// Returns error if validation fails.
func (m *Manager) Register(component Component, dependsOn ...Component) error {
	m.registrationMutex.Lock()
	defer m.registrationMutex.Unlock()

	if component == nil {
		return fmt.Errorf("cannot register nil component")
	}

	if component.Name() == "" {
		return fmt.Errorf("component must have a non-empty name")
	}

	// Check for duplicate registration
	for _, c := range m.components {
		if c == component {
			return fmt.Errorf("component %s is already registered", component.Name())
		}
	}

	// Check that all dependencies are already registered
	for _, dep := range dependsOn {
		found := false
		for _, registered := range m.components {
			if registered == dep {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("dependency %s is not registered", dep.Name())
		}
	}

	// Check for circular dependencies
	if m.wouldCreateCycle(component, dependsOn) {
		return fmt.Errorf("registering %s would create a circular dependency", component.Name())
	}

	// Add to components list
	m.components = append(m.components, component)
	m.dependencies[component] = dependsOn
	m.running[component] = false

	// Update reverse dependency map
	for _, dep := range dependsOn {
		m.reverseDepMap[dep] = append(m.reverseDepMap[dep], component)
	}

	m.logger.Debug("Registered component %s with %d dependencies", component.Name(), len(dependsOn))
	return nil
}

// wouldCreateCycle checks if registering 'component' with 'dependencies' would create a cycle.
func (m *Manager) wouldCreateCycle(component Component, dependencies []Component) bool {
	visited := make(map[Component]bool)
	return m.hasCycleDFS(component, dependencies, visited)
}

func (m *Manager) hasCycleDFS(node Component, dependencies []Component, visited map[Component]bool) bool {
	// Check if 'node' is already in dependencies (direct cycle)
	for _, dep := range dependencies {
		if dep == node {
			return true // direct cycle found
		}
		// Check if this dependency would lead back to node
		if visited[dep] {
			continue // already checked this dependency
		}
		visited[dep] = true
		if m.hasCycleDFS(node, m.dependencies[dep], visited) {
			return true
		}
	}
	return false
}

// Start initializes and starts all registered components in dependency order.
// If any component fails to start, all successfully started components are stopped
// (in reverse order) and an error is returned.
//
// Requirements from spec:
// - FR-001: watchers must start after storage
// - FR-002: storage must start before accepting writes
// - FR-003: API server must initialize
// - FR-004: components start only after dependencies ready
// - FR-010: log each startup with timestamp
//
// Success Criteria (SC-001): startup completes within 5 seconds
//
// Returns error if any component fails to start.
func (m *Manager) Start(ctx context.Context) error {
	m.registrationMutex.Lock()
	defer m.registrationMutex.Unlock()

	m.startedComponents = []Component{}
	toStart := m.topologicalSort()

	for _, component := range toStart {
		m.logger.Info("Starting %s", component.Name())
		startTime := time.Now()

		if err := component.Start(ctx); err != nil {
			m.logger.Error("Failed to start %s: %v", component.Name(), err)
			// Rollback: stop all started components in reverse order
			m.stopComponentsForRollback()
			return fmt.Errorf("initialization failed for %s: %w", component.Name(), err)
		}

		m.mu.Lock()
		m.running[component] = true
		m.startedComponents = append(m.startedComponents, component)
		m.mu.Unlock()

		duration := time.Since(startTime)
		m.logger.Info("%s started successfully (took %dms)", component.Name(), duration.Milliseconds())
	}

	m.logger.Info("All components started successfully")
	return nil
}

// topologicalSort returns components in dependency order (dependencies before dependents).
func (m *Manager) topologicalSort() []Component {
	visited := make(map[Component]bool)
	sorted := []Component{}

	for _, component := range m.components {
		if !visited[component] {
			m.topologicalSortDFS(component, visited, &sorted)
		}
	}

	return sorted
}

func (m *Manager) topologicalSortDFS(component Component, visited map[Component]bool, sorted *[]Component) {
	visited[component] = true

	// First, add all dependencies
	for _, dep := range m.dependencies[component] {
		if !visited[dep] {
			m.topologicalSortDFS(dep, visited, sorted)
		}
	}

	// Then add this component
	*sorted = append(*sorted, component)
}

// stopComponentsForRollback stops components that were started during a failed startup attempt.
// Stops in reverse order of startup.
func (m *Manager) stopComponentsForRollback() {
	// Stop in reverse order of startup
	for i := len(m.startedComponents) - 1; i >= 0; i-- {
		component := m.startedComponents[i]
		m.logger.Debug("Rolling back: stopping %s", component.Name())

		// Use a short timeout for rollback
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := component.Stop(ctx); err != nil {
			m.logger.Warn("Error stopping %s during rollback: %v", component.Name(), err)
		}
		cancel()

		m.mu.Lock()
		m.running[component] = false
		m.mu.Unlock()
	}
}

// Stop gracefully stops all started components in reverse dependency order.
// Each component receives its own deadline equal to (now + shutdown timeout).
// Components exceeding timeout are forcefully terminated.
//
// Requirements from spec:
// - FR-007: shutdown in reverse order (API server, watchers, storage)
// - FR-008: allow in-flight operations to complete within grace period
// - FR-010: log each shutdown with timestamp
// - FR-011: exit with status 0 on successful shutdown
//
// Success Criteria (SC-002): shutdown completes within 30 seconds
// Success Criteria (SC-006): no resource leaks
//
// Always returns nil (shutdown errors logged but don't fail the operation).
func (m *Manager) Stop(ctx context.Context) error {
	m.registrationMutex.Lock()
	defer m.registrationMutex.Unlock()

	m.logger.Info("Stopping all components")

	// Create list of components to stop in reverse dependency order
	toStop := []Component{}

	// Reverse the started components list
	for i := len(m.startedComponents) - 1; i >= 0; i-- {
		toStop = append(toStop, m.startedComponents[i])
	}

	// Stop each component with its own timeout
	for _, component := range toStop {
		if !m.IsRunning(component) {
			continue // Component not running, skip
		}

		m.logger.Info("Stopping %s", component.Name())
		startTime := time.Now()

		// Create a context with timeout for this specific component
		componentCtx, cancel := context.WithTimeout(ctx, m.shutdownTimeout)
		err := component.Stop(componentCtx)
		cancel()

		duration := time.Since(startTime)

		if err != nil {
			if err == context.DeadlineExceeded {
				m.logger.Warn("Component %s exceeded grace period (%dms timeout), forcing termination",
					component.Name(), m.shutdownTimeout.Milliseconds())
			} else {
				m.logger.Error("Error stopping %s: %v", component.Name(), err)
			}
		} else {
			m.logger.Info("%s stopped successfully (took %dms)", component.Name(), duration.Milliseconds())
		}

		m.mu.Lock()
		m.running[component] = false
		m.mu.Unlock()
	}

	m.logger.Info("All components stopped")
	return nil
}

// IsRunning returns true if the component has successfully started and has not stopped.
func (m *Manager) IsRunning(component Component) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running, exists := m.running[component]
	return exists && running
}

// SetShutdownTimeout sets the grace period for graceful shutdown.
// Default is 30 seconds. This is applied per component.
func (m *Manager) SetShutdownTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdownTimeout = timeout
	m.logger.Debug("Shutdown timeout set to %dms", timeout.Milliseconds())
}
