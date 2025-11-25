# Data Model: Graceful Component Initialization and Shutdown

**Created**: 2025-11-25
**Feature**: [spec.md](spec.md)

## Overview

This document defines the key entities and relationships for managing application component lifecycle (initialization and graceful shutdown).

## Core Entities

### 1. Component

**Purpose**: Interface that all managed components must implement to participate in lifecycle management.

**Methods**:

- `Start(ctx context.Context) error`
  - Initializes and starts the component
  - Must be idempotent (safe to call multiple times)
  - Returns error if initialization fails
  - Should log startup activity with component name

- `Stop(ctx context.Context) error`
  - Gracefully stops the component
  - Must handle in-flight operations completion
  - Should respect context deadline for timeout
  - Returns error if shutdown fails
  - Should log shutdown activity with component name

- `Name() string`
  - Returns human-readable name of the component
  - Used for logging and error reporting

**Implementations**:
- `Storage` component (internal/storage)
- `Watcher` component (internal/watcher)
- `APIServer` component (internal/api)

---

### 2. Manager

**Purpose**: Orchestrates the lifecycle of multiple components with dependency awareness.

**Responsibilities**:
- Register components with their dependencies
- Start components in correct dependency order
- Stop components in reverse dependency order
- Enforce startup/shutdown timeouts
- Log all lifecycle events

**Fields**:

- `components: []Component` - List of registered components in order
- `dependencies: map[Component][]Component` - Dependency graph
- `shutdownTimeout: time.Duration` - Max time for graceful shutdown (default: 30s)
- `componentTimeouts: map[Component]time.Duration` - Per-component override timeouts
- `logger: Logger` - For structured logging

**Methods**:

- `Register(component Component, dependsOn ...Component) error`
  - Registers a component with optional dependencies
  - Validates no circular dependencies
  - Returns error if validation fails

- `Start(ctx context.Context) error`
  - Starts all registered components
  - Respects dependency order (dependencies first)
  - Stops all started components and returns error if any fail
  - Logs each component startup with timestamp

- `Stop(ctx context.Context) error`
  - Gracefully stops all started components
  - Stops in reverse dependency order (dependents first)
  - Enforces timeout per component
  - Forcefully terminates components exceeding timeout
  - Logs each component shutdown with timestamp

- `IsRunning(component Component) bool`
  - Returns true if component has successfully started and not stopped

---

### 3. ShutdownConfig

**Purpose**: Configuration for graceful shutdown behavior.

**Fields**:

- `GracePeriod: time.Duration`
  - Total time allowed for graceful shutdown (default: 30s)
  - Applied per component

- `CheckInterval: time.Duration`
  - How often to check if component shutdown completed (default: 100ms)
  - Used for polling shutdown status

- `LogLevel: string`
  - Logging verbosity ("debug", "info", "warn", "error")
  - Allows detailed troubleshooting of shutdown issues

**Default Values**:
```
GracePeriod: 30 seconds
CheckInterval: 100 milliseconds
LogLevel: "info"
```

---

### 4. InitializationError

**Purpose**: Error type for startup failures with detailed context.

**Fields**:

- `Component: string` - Name of component that failed
- `Message: string` - Error description
- `WrappedError: error` - Underlying error if any

**Methods**:

- `Error() string`
  - Returns formatted error message
  - Format: "initialization failed for [component]: [message]"

---

## Relationships

### Startup Sequence

```
Root Context (with cancellation)
    ↓
Start Storage Component
    ↓
Start Watchers Component (parallel)
Start API Server Component (parallel)
    ↓
Application Ready
```

**Dependency Rules**:
- Storage MUST start before Watchers
- Storage MUST start before API Server
- Watchers and API Server can start in parallel

### Shutdown Sequence

```
SIGINT/SIGTERM Signal
    ↓
Cancel Root Context
    ↓
Stop API Server Component (async, max 30s)
    ↓
Stop Watchers Component (async, max 30s)
    ↓
Stop Storage Component (async, max 30s)
    ↓
Application Exits (status 0 if successful, 1 if failures)
```

**Dependency Rules**:
- API Server MUST stop before Watchers
- API Server MUST stop before Storage
- Watchers MUST stop before Storage
- Each component gets full grace period independently

---

## State Transitions

### Component States

```
┌─────────────┐
│ REGISTERED  │ (Component registered, not started)
└──────┬──────┘
       │ Start() called
       ↓
┌─────────────┐
│  STARTING   │ (Initialization in progress)
└──────┬──────┘
       │
       ├─ Success ──→ ┌─────────────┐
       │              │   RUNNING   │ (Running, ready to accept operations)
       │              └──────┬──────┘
       │                     │ Stop() called
       │                     ↓
       │              ┌─────────────┐
       │              │  STOPPING   │ (Graceful shutdown in progress)
       │              └──────┬──────┘
       │                     │
       │                     ├─ Completed ──→ ┌──────────────┐
       │                     │                 │    STOPPED   │
       │                     │                 └──────────────┘
       │                     │
       │                     ├─ Timeout ──→ ┌──────────────┐
       │                                      │  TERMINATED  │
       └─ Failure ──→ ┌──────────────┐       └──────────────┘
                      │  FAILED      │
                      └──────────────┘
```

---

## Validation Rules

### At Registration

- Component name must be non-empty
- No circular dependency chains allowed
- Duplicate component registration not allowed

### At Startup

- All dependency components must be registered
- Context must not be already cancelled
- No components currently starting or running

### At Shutdown

- Shutdown timeout must be positive (default 30s)
- Must handle components that fail gracefully

---

## Error Handling

### Startup Errors

- `InitializationError`: Raised if any component fails to start
- All successfully started components are stopped
- Application exits with status code 1

### Shutdown Errors

- Logged as warnings but don't prevent shutdown completion
- Forceful termination occurs if timeout exceeded
- Application exits with status 0 (shutdown completed, even if errors)

---

## Logging Events

All lifecycle events must be logged with timestamps:

**Startup Events**:
- `component_start_begin: component=[name]`
- `component_start_complete: component=[name], duration_ms=[duration]`
- `component_start_failed: component=[name], error=[error]`

**Shutdown Events**:
- `component_stop_begin: component=[name]`
- `component_stop_complete: component=[name], duration_ms=[duration]`
- `component_stop_timeout: component=[name], grace_period_ms=[period]`
- `component_stop_forced: component=[name], reason=[reason]`

---

## Example Usage

```go
// Register components with dependencies
manager := lifecycle.NewManager()

storage := &StorageComponent{}
watchers := &WatcherComponent{}
apiServer := &APIServerComponent{}

manager.Register(storage)                          // No dependencies
manager.Register(watchers, storage)               // Depends on storage
manager.Register(apiServer, storage)              // Depends on storage

// Start all components
ctx := context.Background()
if err := manager.Start(ctx); err != nil {
    // Handle startup failure
}

// Handle graceful shutdown on signal
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

<-sigChan // Wait for signal
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
manager.Stop(ctx)
```
