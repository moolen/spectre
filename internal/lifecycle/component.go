package lifecycle

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
