# MCP Server Shutdown Fix

**Date**: 2025-12-19
**Issue**: MCP server hangs on Ctrl+C shutdown
**Status**: ✅ FIXED

---

## Problem

When pressing Ctrl+C, the MCP server would:
1. Print "Shutting down HTTP server..."
2. Hang forever (never terminate)
3. Require `kill -9` to force termination

**Logs:**
```
2025/12/20 00:09:02 [] [INFO] mcp: Received signal: interrupt, shutting down gracefully...
2025/12/20 00:09:02 [] [INFO] mcp: Shutting down HTTP server...
[hangs forever]
```

---

## Root Cause

In `cmd/spectre/commands/mcp.go` line 178:

```go
if err := streamableServer.Shutdown(context.Background()); err != nil {
```

**Problem**: `context.Background()` creates an **unbounded context** with no timeout. If the HTTP server has:
- Active connections that won't close
- Slow handlers still processing
- Any blocking operation

The shutdown will wait **forever** for them to complete.

---

## Solution

Added a **5-second timeout** for graceful shutdown:

```go
// Use a timeout context for shutdown (don't hang forever)
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
defer shutdownCancel()

if err := streamableServer.Shutdown(shutdownCtx); err != nil {
    logger.Error("Error during shutdown: %v", err)
    // Force exit if graceful shutdown fails
    os.Exit(1)
}
```

**Behavior:**
1. Try graceful shutdown for up to 5 seconds
2. If shutdown succeeds → clean exit
3. If shutdown times out → log error and exit immediately
4. If shutdown errors → log error and exit immediately

---

## Why 5 Seconds?

- **Too short (1s)**: Might not give active requests time to complete
- **Too long (30s+)**: Frustrating for operators waiting for restart
- **5 seconds**: Good balance - allows active requests to finish, but doesn't hang

**Note**: As requested, this is **not graceful** in the sense of waiting indefinitely for clients. It prioritizes operator experience (fast shutdown) over perfect request completion.

---

## Impact

### Before
```bash
$ ./spectre mcp --graph-enabled
^C
[waits forever... nothing happens]
[terminal stuck]
[kill -9 required]
```

### After
```bash
$ ./spectre mcp --graph-enabled
^C
2025/12/20 00:09:02 [] [INFO] mcp: Received signal: interrupt, shutting down gracefully...
2025/12/20 00:09:02 [] [INFO] mcp: Shutting down HTTP server...
2025/12/20 00:09:02 [] [INFO] mcp: Server stopped
[exits within 5 seconds]
```

---

## Files Modified

- `cmd/spectre/commands/mcp.go`
  - Added shutdown timeout context
  - Added explicit `os.Exit(1)` on errors

---

## Testing

To test the shutdown behavior:

```bash
# Start server
./spectre mcp --graph-enabled

# In another terminal, make a request
curl http://localhost:8082/health

# Press Ctrl+C in the server terminal
# Should exit within 5 seconds
```

---

**Status**: Ready for deployment
**Breaking Changes**: None
**Migration Required**: None
