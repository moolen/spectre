# API Contracts for Graceful Component Initialization and Shutdown

This directory contains the interface contracts that define how components interact with the lifecycle management system.

## Files

### component.go

Defines two core interfaces:

1. **Component**
   - `Start(ctx context.Context) error` - Initialize and start the component
   - `Stop(ctx context.Context) error` - Gracefully stop the component
   - `Name() string` - Get component name for logging

   All managed components (Storage, Watchers, API Server) must implement this interface.

2. **Manager**
   - `Register(component Component, dependsOn ...Component) error` - Register components with dependencies
   - `Start(ctx context.Context) error` - Start all components in dependency order
   - `Stop(ctx context.Context) error` - Stop all components in reverse dependency order
   - `IsRunning(component Component) bool` - Check if component is currently running

## Design Principles

### 1. Context-Based Lifecycle
- Uses Go's `context.Context` for graceful shutdown signaling
- Context deadline defines the shutdown timeout
- Cancellation of context should gracefully stop in-flight operations

### 2. Dependency Awareness
- Manager understands component dependencies
- Enforces correct startup order (dependencies before dependents)
- Enforces reverse order for shutdown (dependents before dependencies)
- Prevents circular dependencies

### 3. Timeout Protection
- Each component gets a grace period for shutdown (default 30 seconds)
- Components exceeding timeout are forcefully terminated
- Prevents application hangs during shutdown

### 4. Error Handling
- Start errors are fatal - stops all started components
- Stop errors are non-fatal - logged as warnings but don't prevent other components from stopping
- Always attempts graceful shutdown, even if components fail

### 5. Logging
- All lifecycle events logged with timestamps
- Component names in all log messages for traceability
- Separate logs for success, timeout, and forced termination

## Implementation Notes

### For Component Implementers

When implementing the `Component` interface in storage, watchers, and API server:

1. **Start() method**
   - Should be idempotent (safe to call multiple times)
   - Must check context for cancellation before returning
   - Log startup with: `logger.Info("Starting [component]")`
   - Log completion with: `logger.Info("[Component] started")`

2. **Stop() method**
   - Should gracefully stop in-flight operations
   - Must check context deadline and respect it
   - Log shutdown with: `logger.Info("Stopping [component]")`
   - Log completion with: `logger.Info("[Component] stopped")`

3. **Name() method**
   - Return consistent name: "Storage", "Watcher", "API Server"
   - Used in logging and error messages

### For Manager Implementers

The Manager implementation must:

1. **Validate at registration time**
   - Check for circular dependencies
   - Ensure all dependencies are registered first
   - Reject duplicate registrations

2. **Start in dependency order**
   - Start Storage first (no dependencies)
   - Start Watchers and API Server in parallel after Storage
   - Stop and rollback if any component fails

3. **Stop in reverse order**
   - Stop API Server first
   - Stop Watchers second
   - Stop Storage last
   - Allow 30 seconds per component
   - Forcefully terminate if timeout exceeded

4. **Log all events**
   - Include component name and timestamp
   - Log start, completion, timeout, and forced termination

## References

- Feature Spec: [spec.md](../spec.md)
- Data Model: [data-model.md](../data-model.md)
- Implementation Plan: [plan.md](../plan.md)
