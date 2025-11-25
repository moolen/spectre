# Quickstart: Component Lifecycle Management

This guide explains how to use the graceful initialization and shutdown system in your application.

## Overview

The component lifecycle management system orchestrates the startup and shutdown of application components (Storage, Watchers, API Server) in the correct dependency order, with graceful shutdown and timeout protection.

## Basic Usage

### 1. Implementing the Component Interface

Each component must implement the `Component` interface with three methods:

```go
package storage

import "context"

type Storage struct {
    // component fields
}

// Start initializes the storage component
func (s *Storage) Start(ctx context.Context) error {
    logger := logging.GetLogger("storage")
    logger.Info("Starting storage component")

    // Initialize storage (database, file system, etc.)
    // Return early if context is already cancelled
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // ... initialization code ...

    logger.Info("Storage component started successfully")
    return nil
}

// Stop gracefully shuts down the storage component
func (s *Storage) Stop(ctx context.Context) error {
    logger := logging.GetLogger("storage")
    logger.Info("Stopping storage component")

    // Create a channel to signal completion
    done := make(chan error, 1)

    go func() {
        // Gracefully close connections, flush buffers, etc.
        done <- nil
    }()

    // Wait for shutdown to complete or context deadline
    select {
    case err := <-done:
        if err != nil {
            logger.Error("Storage component shutdown error: %v", err)
            return err
        }
        logger.Info("Storage component stopped")
        return nil
    case <-ctx.Done():
        logger.Warn("Storage component shutdown timeout, forcing termination")
        return ctx.Err()
    }
}

// Name returns the component name for logging
func (s *Storage) Name() string {
    return "Storage"
}
```

### 2. Registering Components with Dependencies

In your `main.go`, register components in any order, specifying their dependencies:

```go
package main

import (
    "context"
    "github.com/moritz/rpk/internal/lifecycle"
    "github.com/moritz/rpk/internal/storage"
    "github.com/moritz/rpk/internal/watcher"
    "github.com/moritz/rpk/internal/api"
)

func main() {
    // Create lifecycle manager
    manager := lifecycle.NewManager()

    // Create component instances
    storageComp := storage.New()
    watcherComp := watcher.New()
    apiComp := api.NewServer()

    // Register components with dependencies
    // Storage has no dependencies
    manager.Register(storageComp)

    // Watchers depend on storage
    manager.Register(watcherComp, storageComp)

    // API Server depends on storage
    manager.Register(apiComp, storageComp)

    // Start all components
    if err := manager.Start(context.Background()); err != nil {
        // Handle startup error
    }
}
```

### 3. Graceful Shutdown on Signal

Install a signal handler to trigger graceful shutdown:

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    "time"
)

func main() {
    // ... component registration and startup ...

    // Set up signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

    // Wait for shutdown signal
    <-sigChan
    logger.Info("Shutdown signal received")

    // Create context with 30-second timeout for graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Stop all components gracefully
    manager.Stop(ctx)

    logger.Info("Application shutdown complete")
}
```

## Dependency Management

### Dependency Order During Startup

```
Start Storage
    ↓
Start Watchers (after Storage ready)
Start API Server (after Storage ready, can be parallel to Watchers)
    ↓
Application Ready
```

**Rule**: A component only starts after ALL its dependencies have successfully started.

### Dependency Order During Shutdown

```
Stop API Server
Stop Watchers
    ↓
Stop Storage
    ↓
Application Stopped
```

**Rule**: A component stops before any of its dependents stop. Shutdown happens in reverse dependency order.

### Defining Multiple Dependencies

If a component depends on multiple other components, list them all:

```go
// Hypothetical: analytics service depends on both storage and API
manager.Register(analyticsComp, storageComp, apiComp)
```

This ensures both `storageComp` and `apiComp` start before `analyticsComp`.

## Error Handling

### Startup Errors

If any component fails to start, the Manager will:
1. Stop all successfully started components (in reverse order)
2. Return an `InitializationError` with details

```go
if err := manager.Start(ctx); err != nil {
    fmt.Fprintf(os.Stderr, "Startup failed: %v\n", err)
    os.Exit(1)
}
```

### Shutdown Errors

Errors during shutdown are logged as warnings but don't prevent the application from exiting:

```go
// Even if components report errors during shutdown,
// the application will exit with status 0
manager.Stop(ctx)
os.Exit(0)
```

## Timeout Behavior

Each component gets a grace period (default 30 seconds) to shutdown gracefully:

```
Component Stop() called
    ↓
Wait for Stop() to complete or deadline
    ↓
Deadline exceeded? Force terminate and log warning
    ↓
Move to next component
```

### Custom Timeouts

To set custom timeout for a specific component:

```go
manager := lifecycle.NewManager()
manager.SetComponentTimeout(storageComp, 10*time.Second)
manager.Register(storageComp)
```

## Logging and Debugging

All lifecycle events are logged automatically. Enable debug logging for detailed diagnostics:

```go
import "github.com/moritz/rpk/internal/logging"

// Set log level to debug
logging.Initialize("debug")
```

**Log Events to Expect**:
- `component_start_begin: component=Storage` - Starting initialization
- `component_start_complete: component=Storage, duration_ms=150` - Initialization complete
- `component_start_failed: component=Watcher, error=...` - Initialization failed
- `component_stop_begin: component=API Server` - Starting graceful shutdown
- `component_stop_complete: component=API Server, duration_ms=200` - Shutdown complete
- `component_stop_timeout: component=Storage, grace_period_ms=30000` - Timeout exceeded

## Testing Component Lifecycle

### Unit Testing a Component

```go
func TestStorageStartStop(t *testing.T) {
    comp := &storage.Storage{}

    // Test start
    ctx := context.Background()
    if err := comp.Start(ctx); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    // Test stop
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := comp.Stop(ctx); err != nil {
        t.Fatalf("Stop failed: %v", err)
    }
}
```

### Integration Testing Manager

```go
func TestManagerStartStop(t *testing.T) {
    manager := lifecycle.NewManager()

    storage := &storage.Storage{}
    watcher := &watcher.Watcher{}

    manager.Register(storage)
    manager.Register(watcher, storage)

    // Test start sequence
    if err := manager.Start(context.Background()); err != nil {
        t.Fatalf("Start failed: %v", err)
    }

    if !manager.IsRunning(storage) || !manager.IsRunning(watcher) {
        t.Fatal("Components not running")
    }

    // Test stop sequence
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    manager.Stop(ctx)

    if manager.IsRunning(storage) || manager.IsRunning(watcher) {
        t.Fatal("Components still running after stop")
    }
}
```

## Best Practices

### 1. Always Use Context Deadlines

```go
// Good: respects timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
manager.Stop(ctx)

// Bad: no timeout protection
manager.Stop(context.Background())
```

### 2. Check Context Before Long Operations

```go
func (s *Storage) Start(ctx context.Context) error {
    // Check for cancellation before doing work
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Now do initialization...
}
```

### 3. Log Component Lifecycle Events

```go
func (s *Storage) Start(ctx context.Context) error {
    logger := logging.GetLogger("storage")
    logger.Info("Starting storage")
    // ... init code ...
    logger.Info("Storage ready")
    return nil
}

func (s *Storage) Stop(ctx context.Context) error {
    logger := logging.GetLogger("storage")
    logger.Info("Stopping storage")
    // ... cleanup code ...
    logger.Info("Storage stopped")
    return nil
}
```

### 4. Handle In-Flight Operations

```go
func (a *APIServer) Stop(ctx context.Context) error {
    logger := logging.GetLogger("api")
    logger.Info("API server: waiting for in-flight requests")

    done := make(chan error, 1)
    go func() {
        a.server.Shutdown(ctx)
        done <- nil
    }()

    select {
    case <-done:
        logger.Info("API server shutdown complete")
        return nil
    case <-ctx.Done():
        logger.Warn("API server shutdown timeout, forcing close")
        a.server.Close()
        return ctx.Err()
    }
}
```

## Troubleshooting

### Application Hangs on Shutdown

**Cause**: Component's Stop() method is blocking without checking context deadline

**Fix**: Ensure Stop() respects context deadline:

```go
select {
case <-doneChannel:
    return nil
case <-ctx.Done():
    return ctx.Err()
}
```

### Component Fails to Start

**Cause**: Dependency not started or initialization error

**Check**:
1. Verify all dependencies are registered before the dependent component
2. Check logs for detailed error message from dependency
3. Ensure Start() implementation returns early on context cancellation

### Missing Startup Logs

**Cause**: Logging not initialized

**Fix**: Initialize logging at application start:

```go
logging.Initialize(cfg.LogLevel)
```

## References

- [Feature Specification](spec.md)
- [Data Model](data-model.md)
- [API Contracts](contracts/)
- [Implementation Plan](plan.md)
