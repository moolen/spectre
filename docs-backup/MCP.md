# Model Context Protocol (MCP) Server

Spectre includes a Model Context Protocol (MCP) server that exposes Spectre's Kubernetes observability capabilities as MCP tools for AI assistants like Claude Code.

## Overview

The MCP server provides:
- **4 Tools** for cluster analysis: cluster health, resource changes, investigation, and resource exploration
- **2 Prompts** for incident handling: post-mortem analysis and live incident triage
- **2 Transport Modes**: HTTP (independent server) and stdio (subprocess-based)

## Transport Modes

### HTTP Transport (Default)

The HTTP transport runs Spectre MCP as an independent server with REST-like endpoints.

**Use cases:**
- Independent deployment alongside Spectre
- Multiple concurrent clients
- Web-based MCP clients
- Service mesh integration

**Starting the server:**
```bash
# Default: HTTP on port 8081
spectre mcp

# Custom port
spectre mcp --http-addr :9000

# With custom Spectre API URL
spectre mcp --spectre-url http://spectre-api:8080 --http-addr :8081
```

**Environment variables:**
```bash
export SPECTRE_URL=http://localhost:8080
export MCP_HTTP_ADDR=:8081
spectre mcp
```

**Testing the server:**
```bash
# Health check
curl http://localhost:8081/health

# Server info
curl http://localhost:8081/

# MCP endpoint
curl -X POST http://localhost:8081/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test-client","version":"1.0.0"}}}'
```

### stdio Transport

The stdio transport runs Spectre MCP as a subprocess that communicates via standard input/output, following the MCP specification for stdio transport.

**Use cases:**
- Claude Code and other subprocess-based MCP clients
- CLI tools that spawn MCP servers
- Isolated, single-session use cases

**Starting the server:**
```bash
# stdio mode
spectre mcp --transport stdio --spectre-url http://localhost:8080

# Note: In stdio mode, --http-addr is ignored
```

**Key differences from HTTP:**
- **Messages**: Newline-delimited JSON on stdin/stdout
- **Logging**: All logs go to stderr (stdout is reserved for MCP messages)
- **Session**: Single client per subprocess instance
- **Lifecycle**: Subprocess exits when stdin closes

**Example client (Python):**
```python
import subprocess
import json

# Start MCP server as subprocess
proc = subprocess.Popen(
    ['spectre', 'mcp', '--transport', 'stdio', '--spectre-url', 'http://localhost:8080'],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    stderr=subprocess.PIPE
)

# Send initialize request
request = {
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
        "protocolVersion": "2024-11-05",
        "clientInfo": {"name": "test-client", "version": "1.0.0"}
    }
}
proc.stdin.write((json.dumps(request) + '\n').encode())
proc.stdin.flush()

# Read response
response = json.loads(proc.stdout.readline().decode())
print(response)

# Clean shutdown
proc.stdin.close()
proc.wait()
```

## Available Tools

### 1. cluster_health
Get cluster health overview with resource status breakdown and top issues.

**Parameters:**
- `start_time` (required): Start timestamp (Unix seconds)
- `end_time` (required): End timestamp (Unix seconds)
- `namespace` (optional): Filter by Kubernetes namespace
- `max_resources` (optional): Max resources to list per status (default 100, max 500)

### 2. resource_changes
Get summarized resource changes with categorization and impact scoring for LLM analysis.

**Parameters:**
- `start_time` (required): Start timestamp (Unix seconds)
- `end_time` (required): End timestamp (Unix seconds)
- `kinds` (optional): Comma-separated resource kinds to filter (e.g., 'Pod,Deployment')
- `impact_threshold` (optional): Minimum impact score 0-1.0 to include in results
- `max_resources` (optional): Max resources to return (default 50, max 500)

### 3. investigate
Get detailed investigation evidence with status timeline, events, and investigation prompts for RCA.

**Parameters:**
- `resource_kind` (required): Resource kind to investigate (e.g., 'Pod', 'Deployment')
- `resource_name` (optional): Specific resource name to investigate, or '*' for all
- `namespace` (optional): Kubernetes namespace to filter by
- `start_time` (required): Start timestamp (Unix seconds)
- `end_time` (required): End timestamp (Unix seconds)
- `investigation_type` (optional): 'incident' for live response, 'post-mortem' for historical analysis, or 'auto' to detect
- `max_investigations` (optional): Max resources to investigate when using '*' (default 20, max 100)

### 4. resource_explorer
Browse and discover resources in the cluster with filtering and status overview.

**Parameters:**
- `kind` (optional): Filter by resource kind (e.g., 'Pod', 'Deployment')
- `namespace` (optional): Filter by Kubernetes namespace
- `status` (optional): Filter by status (Ready, Warning, Error, Terminating)
- `time` (optional): Snapshot at specific time (Unix seconds), 0 or omit for latest
- `max_resources` (optional): Max resources to return (default 200, max 1000)

## Available Prompts

### 1. post_mortem_incident_analysis
Conduct a comprehensive post-mortem analysis of a past incident.

**Arguments:**
- `start_time` (required): Start of the incident time window (Unix timestamp)
- `end_time` (required): End of the incident time window (Unix timestamp)
- `namespace` (optional): Kubernetes namespace
- `incident_description` (optional): Brief description

### 2. live_incident_handling
Triage and investigate an ongoing incident.

**Arguments:**
- `incident_start_time` (required): When symptoms first appeared (Unix timestamp)
- `current_time` (optional): Current time
- `namespace` (optional): Kubernetes namespace
- `symptoms` (optional): Brief description of symptoms

## Deployment

### Standalone Deployment

```bash
# Run MCP server independently
spectre mcp --spectre-url http://spectre-api:8080 --http-addr :8081
```

### Kubernetes Deployment (Sidecar)

The Helm chart includes an optional MCP sidecar container:

```yaml
# values.yaml
mcp:
  enabled: true
  spectreURL: "http://localhost:8080"
  httpAddr: ":8081"
  port: 8081
```

The sidecar:
- Runs alongside the main Spectre container
- Connects to Spectre via localhost
- Exposes MCP on port 8081
- Includes health checks and resource limits

### Docker Compose

```yaml
version: '3.8'
services:
  spectre:
    image: spectre:latest
    command: ["--api-port=8080", "--data-dir=/data"]
    volumes:
      - spectre-data:/data
    ports:
      - "8080:8080"

  spectre-mcp:
    image: spectre:latest
    command: ["mcp", "--spectre-url=http://spectre:8080", "--http-addr=:8081"]
    depends_on:
      - spectre
    ports:
      - "8081:8081"

volumes:
  spectre-data:
```

## Testing

### HTTP Transport Test
```bash
# Run HTTP transport integration test
go test -v ./tests/e2e -run TestMCPHTTPTransport -timeout 30m
```

### stdio Transport Test
```bash
# Run stdio transport integration test
go test -v ./tests/e2e -run TestMCPStdioTransport -timeout 30m
```

### Both Transports
```bash
# Run all MCP tests
go test -v ./tests/e2e -run "TestMCP.*Transport" -timeout 30m
```

## Protocol Specification

The MCP server implements the [Model Context Protocol specification](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports).

**Supported features:**
- ✅ JSON-RPC 2.0
- ✅ Tools (list, call)
- ✅ Prompts (list, get)
- ✅ Logging (setLevel)
- ✅ HTTP transport
- ✅ stdio transport
- ✅ Session initialization

## Architecture

```
cmd/spectre/commands/mcp.go           # Command entry point
internal/mcp/
  ├── protocol.go                     # MCP protocol types
  ├── handler.go                      # Transport-agnostic handler
  ├── server.go                       # Core MCP server
  └── transport/
      ├── http/transport.go           # HTTP transport
      └── stdio/transport.go          # stdio transport
```

The architecture uses a **transport abstraction** pattern:
1. **Handler** processes MCP requests independently of transport
2. **Transports** handle I/O and message delivery
3. **Server** manages tools and prompts

This design allows easy addition of new transports (e.g., WebSocket) without changing core logic.

## Troubleshooting

### HTTP Transport

**Problem**: Connection refused
```bash
# Check if server is running
curl http://localhost:8081/health

# Check logs for startup errors
spectre mcp --log-level debug
```

**Problem**: Can't connect to Spectre API
```bash
# Verify Spectre API is accessible
curl http://localhost:8080/health

# Update spectre-url flag
spectre mcp --spectre-url http://correct-host:8080
```

### stdio Transport

**Problem**: No output on stdout
- Ensure you're sending valid JSON-RPC 2.0 messages
- Check stderr for error logs
- Verify newline-delimited JSON format

**Problem**: Subprocess hangs
- Check that stdin is not blocked
- Ensure messages don't contain embedded newlines
- Verify proper UTF-8 encoding

**Problem**: Logs mixed with output
- In stdio mode, logs automatically go to stderr
- Only MCP messages appear on stdout

## Security Considerations

1. **Authentication**: MCP server does not implement authentication. Use network policies or reverse proxies for access control.
2. **Authorization**: All clients have full access to all tools. Deploy MCP server with same permissions as Spectre.
3. **Resource Limits**: Tool parameters have built-in limits to prevent excessive resource usage.
4. **Network Isolation**: In Kubernetes, use network policies to restrict MCP server access.

## Performance

- **HTTP Transport**: Supports multiple concurrent clients with connection pooling
- **stdio Transport**: Single client per subprocess, minimal overhead
- **Tool Execution**: Tools query Spectre API, performance depends on cluster size and time ranges
- **Memory**: ~64Mi typical, ~256Mi limit recommended
- **CPU**: Minimal (50m request, 200m limit recommended)
