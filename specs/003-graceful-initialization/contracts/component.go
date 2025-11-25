package contracts

import "context"

// Component defines the lifecycle interface that all managed components must implement.
// This allows the lifecycle manager to orchestrate startup and shutdown of components
// in the correct dependency order.
type Component interface {
	// Start initializes and starts the component.
	// The provided context can be used to signal shutdown or set deadlines.
	// Must be idempotent - safe to call multiple times.
	// Should log startup activity with component name.
	// Returns error if initialization fails.
	Start(ctx context.Context) error

	// Stop gracefully stops the component.
	// Must handle in-flight operations completion within the context deadline.
	// Should respect context deadline for graceful shutdown timeout.
	// Should log shutdown activity with component name.
	// Returns error if shutdown fails (but shouldn't prevent other components from stopping).
	Stop(ctx context.Context) error

	// Name returns the human-readable name of the component.
	// Used for logging, error reporting, and dependency declarations.
	// Must return a non-empty string.
	Name() string
}

// Manager orchestrates the lifecycle of multiple components with dependency awareness.
// It ensures components are started in the correct dependency order and stopped
// in reverse dependency order, with timeout protection to prevent indefinite hangs.
type Manager interface {
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
	Register(component Component, dependsOn ...Component) error

	// Start initializes and starts all registered components in dependency order.
	// If any component fails to start, all successfully started components are stopped
	// (in reverse order) and an InitializationError is returned.
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
	// Returns InitializationError if any component fails.
	Start(ctx context.Context) error

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
	Stop(ctx context.Context) error

	// IsRunning returns true if the component has successfully started and has not stopped.
	IsRunning(component Component) bool
}
