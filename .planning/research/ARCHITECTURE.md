# Architecture Research: Logz.io Integration + Secret Management

**Project:** Spectre v1.2 - Logz.io Integration
**Researched:** 2026-01-22
**Confidence:** HIGH

## Executive Summary

Logz.io integration follows the existing VictoriaLogs plugin pattern with three architectural additions:
1. **Multi-region client** with region-aware endpoint selection
2. **Secret file watcher** for hot-reload of API tokens from Kubernetes-mounted secrets
3. **Elasticsearch DSL query builder** instead of LogsQL

The architecture leverages existing patterns (factory registry, integration lifecycle, hot-reload via fsnotify) with zero changes to core plugin infrastructure. Secret management follows Kubernetes-native volume mount pattern with application-level file watching.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                      Integration Manager                         │
│  (internal/integration/manager.go)                              │
│                                                                   │
│  - Factory registry for integration types                        │
│  - Config hot-reload via fsnotify (integrations.yaml)          │
│  - Lifecycle orchestration (Start/Stop/Health/RegisterTools)   │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ Creates instances via factory
                         │
        ┌────────────────┴────────────────┐
        │                                  │
        v                                  v
┌──────────────────┐            ┌──────────────────────┐
│  VictoriaLogs    │            │    Logz.io           │
│  Integration     │            │    Integration       │ ◄── NEW
│                  │            │                      │
│  - Client        │            │  - RegionalClient    │
│  - Pipeline      │            │  - SecretWatcher     │
│  - Tools         │            │  - Tools             │
└──────────────────┘            └──────────────────────┘
        │                                  │
        │                                  │
        v                                  v
┌──────────────────┐            ┌──────────────────────┐
│  MCP Server      │            │  MCP Server          │
│  (mcp/server.go) │            │  (mcp/server.go)     │
│                  │            │                      │
│  RegisterTool()  │            │  RegisterTool()      │
└──────────────────┘            └──────────────────────┘
        │                                  │
        └──────────────┬───────────────────┘
                       │
                       v
        ┌──────────────────────────┐
        │  MCP Clients (Claude,   │
        │  Cline, etc.)            │
        └──────────────────────────┘


┌─────────────────────────────────────────────────────────────────┐
│              Secret Management Flow (Kubernetes)                 │
└─────────────────────────────────────────────────────────────────┘

Kubernetes Secret                   Logz.io Integration
(logzio-api-token)                 (internal/integration/logzio/)
        │                                    │
        │ Volume mount                       │
        │ (extraVolumes)                     │
        v                                    │
/var/lib/spectre/secrets/          SecretWatcher (fsnotify)
logzio-token                                │
        │                                    │
        │ File read                          │
        │ (at startup)                       │
        └───────────────────────────────────>│
                                             │
        ┌────────────────────────────────────┤
        │ File change event                  │
        │ (on secret rotation)               │
        └───────────────────────────────────>│
                                             │
                                        Hot-reload
                                        (re-read file,
                                         update client)
```

## Logz.io Client Architecture

### Component: RegionalClient

**Location:** `internal/integration/logzio/client.go`

**Structure:**
```go
type RegionalClient struct {
    region      string              // 2-letter region code (us, eu, au, ca, uk)
    baseURL     string              // Computed from region
    apiToken    string              // Loaded from secret file
    tokenMu     sync.RWMutex        // Protects token during hot-reload
    httpClient  *http.Client        // Standard HTTP client with connection pooling
    logger      *logging.Logger
}

// Region endpoint mapping
var RegionEndpoints = map[string]string{
    "us": "https://api.logz.io",
    "eu": "https://api-eu.logz.io",
    "au": "https://api-au.logz.io",
    "ca": "https://api-ca.logz.io",
    "uk": "https://api-uk.logz.io",
}
```

**Design rationale:**
- **Region-aware URL construction:** Maps 2-letter region code to API endpoint at client creation time
- **Thread-safe token updates:** RWMutex allows concurrent reads (queries) during token rotation
- **Bearer token authentication:** Uses `Authorization: Bearer <token>` header on all requests
- **Connection pooling:** Reuses HTTP client transport (same pattern as VictoriaLogs)

**API methods:**
```go
// Query interface (mirrors VictoriaLogs pattern)
func (c *RegionalClient) SearchLogs(ctx context.Context, params SearchParams) (*SearchResponse, error)
func (c *RegionalClient) Aggregations(ctx context.Context, params AggregationParams) (*AggregationResponse, error)

// Token management (for hot-reload)
func (c *RegionalClient) UpdateToken(newToken string)
```

**HTTP request pattern:**
```go
// POST /v1/search
// Authorization: Bearer <api_token>
// Content-Type: application/json
// Body: Elasticsearch DSL query object
{
  "query": {
    "bool": {
      "must": [...],
      "filter": [...]
    }
  },
  "size": 100,
  "from": 0,
  "sort": [...]
}
```

**Sources:**
- [Logz.io API Authentication](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)
- [Logz.io Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

### Component: Query Builder

**Location:** `internal/integration/logzio/query.go`

**Structure:**
```go
type SearchParams struct {
    TimeRange   TimeRange           // Start/end timestamps
    Namespace   string              // Kubernetes namespace filter
    Severity    string              // Log level filter (error, warn, info, debug)
    Pod         string              // Pod name filter
    Container   string              // Container name filter
    Limit       int                 // Result limit (default 100, max 10,000)
}

func BuildElasticsearchDSL(params SearchParams) map[string]interface{} {
    // Returns Elasticsearch DSL query object
}
```

**Design rationale:**
- **Structured parameters → DSL:** Avoids exposing raw Elasticsearch DSL to MCP tools
- **Kubernetes-aware filters:** Maps to Logz.io's Kubernetes log fields (namespace, pod, container)
- **Time range handling:** Converts Unix timestamps to Elasticsearch range queries
- **Bool query structure:** Uses `must` + `filter` clauses for optimal performance

**Example DSL output:**
```json
{
  "query": {
    "bool": {
      "filter": [
        {
          "range": {
            "@timestamp": {
              "gte": "2026-01-22T00:00:00Z",
              "lte": "2026-01-22T23:59:59Z"
            }
          }
        },
        {
          "term": {
            "kubernetes.namespace.keyword": "production"
          }
        },
        {
          "term": {
            "severity.keyword": "error"
          }
        }
      ]
    }
  },
  "size": 100,
  "sort": [
    {"@timestamp": "desc"}
  ]
}
```

**Sources:**
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)

## Secret Management Architecture

### Component: SecretWatcher

**Location:** `internal/integration/logzio/secret_watcher.go`

**Structure:**
```go
type SecretWatcher struct {
    filePath    string              // Path to secret file (e.g., /var/lib/spectre/secrets/logzio-token)
    onUpdate    func(string) error  // Callback to update client with new token
    watcher     *fsnotify.Watcher   // fsnotify file watcher
    logger      *logging.Logger
    cancel      context.CancelFunc
}

func NewSecretWatcher(filePath string, onUpdate func(string) error) (*SecretWatcher, error)
func (sw *SecretWatcher) Start(ctx context.Context) error
func (sw *SecretWatcher) Stop() error
```

**Design rationale:**
- **fsnotify for file watching:** Reuses pattern from `internal/config/integration_watcher.go`
- **Callback pattern:** Integration provides `UpdateToken()` as callback
- **Atomic write handling:** Kubernetes secrets use symlink rotation (no inotify issues)
- **Error resilience:** Failed token updates log error but don't crash watcher

**File watching strategy:**

Kubernetes secret volume mounts use **atomic symlink rotation**:
```
/var/lib/spectre/secrets/
├── logzio-token -> ..data/token   # Symlink (watched path)
└── ..data -> ..2026_01_22_10_30_00_12345/
    └── token                       # Actual file content

# On rotation:
1. New directory created: ..2026_01_22_11_00_00_67890/
2. ..data symlink updated atomically
3. Old directory removed after grace period
```

**fsnotify event handling:**
```go
// From research: Kubernetes secrets emit IN_DELETE_SELF on atomic updates
// Must re-establish watch after each update
for {
    select {
    case event := <-watcher.Events:
        if event.Op&fsnotify.Write == fsnotify.Write ||
           event.Op&fsnotify.Remove == fsnotify.Remove {
            // Re-add watch (atomic writes break inotify)
            watcher.Add(filePath)
            // Reload secret
            newToken := readSecretFile(filePath)
            onUpdate(newToken)
        }
    }
}
```

**Sources:**
- [Kubernetes Secret Volume Mount Behavior](https://kubernetes.io/docs/concepts/configuration/secret/)
- [fsnotify with Kubernetes Secrets](https://ahmet.im/blog/kubernetes-inotify/)
- [Secrets Store CSI Driver Auto Rotation](https://secrets-store-csi-driver.sigs.k8s.io/topics/secret-auto-rotation)

### Kubernetes Deployment Pattern

**Helm values.yaml:**
```yaml
# extraVolumes in chart/values.yaml
extraVolumes:
  - name: logzio-secrets
    secret:
      secretName: logzio-api-token
      optional: false

extraVolumeMounts:
  - name: logzio-secrets
    mountPath: /var/lib/spectre/secrets
    readOnly: true
```

**integrations.yaml config:**
```yaml
schema_version: v1
instances:
  - name: logzio-prod
    type: logzio
    enabled: true
    config:
      region: eu
      api_token_path: /var/lib/spectre/secrets/logzio-token
```

**Design rationale:**
- **No plaintext secrets in config:** Config only references file path
- **Kubernetes-native secret rotation:** Use `kubectl apply` or external-secrets-operator
- **Optional CSI driver:** Can use Secrets Store CSI Driver for advanced rotation (HashiCorp Vault, AWS Secrets Manager)
- **Backward compatible:** Existing integrations without secret files continue working

**Token rotation workflow:**
```
1. User rotates token in Logz.io UI
2. User updates Kubernetes Secret:
   kubectl create secret generic logzio-api-token \
     --from-literal=logzio-token=<new-token> \
     --dry-run=client -o yaml | kubectl apply -f -
3. Kubernetes updates secret file in pod (atomic symlink rotation)
4. SecretWatcher detects file change (fsnotify event)
5. SecretWatcher reads new token from file
6. SecretWatcher calls integration.UpdateToken(newToken)
7. RegionalClient updates token under RWMutex
8. Subsequent queries use new token (no pod restart required)
```

**Fallback for failed rotation:**
- Old token continues working until Logz.io revokes it
- Health check will detect authentication failures
- Integration enters Degraded state (auto-recovery on next health check)

## Integration Points

### 1. Factory Registration

**Location:** `internal/integration/logzio/logzio.go`

```go
func init() {
    integration.RegisterFactory("logzio", NewLogzioIntegration)
}

func NewLogzioIntegration(name string, config map[string]interface{}) (integration.Integration, error) {
    // Parse config
    region := config["region"].(string)
    apiTokenPath := config["api_token_path"].(string)

    // Read initial token from file
    initialToken, err := os.ReadFile(apiTokenPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read API token: %w", err)
    }

    // Create client
    client := NewRegionalClient(region, string(initialToken))

    // Create secret watcher
    secretWatcher := NewSecretWatcher(apiTokenPath, client.UpdateToken)

    return &LogzioIntegration{
        name:          name,
        client:        client,
        secretWatcher: secretWatcher,
    }, nil
}
```

**Integration points:**
- Uses existing `integration.RegisterFactory()` (no changes to factory system)
- Follows VictoriaLogs pattern (same function signature)
- Config validation happens in factory constructor

### 2. Integration Lifecycle

**Location:** `internal/integration/logzio/logzio.go`

```go
type LogzioIntegration struct {
    name          string
    client        *RegionalClient
    secretWatcher *SecretWatcher
    registry      integration.ToolRegistry
    logger        *logging.Logger
}

func (l *LogzioIntegration) Start(ctx context.Context) error {
    // Test connectivity (health check with current token)
    if err := l.client.testConnection(ctx); err != nil {
        l.logger.Warn("Initial connectivity test failed (degraded state): %v", err)
    }

    // Start secret watcher
    if err := l.secretWatcher.Start(ctx); err != nil {
        return fmt.Errorf("failed to start secret watcher: %w", err)
    }

    l.logger.Info("Logz.io integration started (region: %s)", l.client.region)
    return nil
}

func (l *LogzioIntegration) Stop(ctx context.Context) error {
    // Stop secret watcher
    if err := l.secretWatcher.Stop(); err != nil {
        l.logger.Error("Error stopping secret watcher: %v", err)
    }

    // Clear references
    l.client = nil
    l.secretWatcher = nil

    return nil
}

func (l *LogzioIntegration) Health(ctx context.Context) integration.HealthStatus {
    if l.client == nil {
        return integration.Stopped
    }

    // Test connectivity (will use current token, even if rotated)
    if err := l.client.testConnection(ctx); err != nil {
        return integration.Degraded
    }

    return integration.Healthy
}

func (l *LogzioIntegration) RegisterTools(registry integration.ToolRegistry) error {
    l.registry = registry

    // Register MCP tools (logzio_{name}_search, logzio_{name}_aggregations, etc.)
    // Same pattern as VictoriaLogs tools

    return nil
}
```

**Integration points:**
- Implements `integration.Integration` interface (no interface changes)
- Start() initializes client and secret watcher
- Stop() cleans up watchers
- Health() tests connectivity (auth failures detected here)
- RegisterTools() follows VictoriaLogs pattern

### 3. MCP Tool Registration

**Location:** `internal/integration/logzio/tools_search.go`

```go
type SearchTool struct {
    ctx ToolContext
}

type ToolContext struct {
    Client   *RegionalClient
    Logger   *logging.Logger
    Instance string
}

func (t *SearchTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    var params SearchParams
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid parameters: %w", err)
    }

    // Query Logz.io (uses current token, even if rotated)
    response, err := t.ctx.Client.SearchLogs(ctx, params)
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }

    return response, nil
}
```

**Tool naming convention:**
```
logzio_{instance}_search       # Raw log search
logzio_{instance}_aggregations # Aggregated stats
logzio_{instance}_patterns     # Log pattern mining (if Phase 2 includes)
```

**Integration points:**
- Uses `integration.ToolRegistry.RegisterTool()` (existing interface)
- Tools reference client from ToolContext (same as VictoriaLogs)
- MCP server adapts to mcp-go server via `MCPToolRegistry` (existing adapter)

### 4. Config Hot-Reload

**Existing behavior (no changes needed):**

`internal/integration/manager.go` already handles config hot-reload:
```go
func (m *Manager) handleConfigReload(newConfig *config.IntegrationsFile) error {
    // Stop all existing instances (including secret watchers)
    m.stopAllInstancesLocked(ctx)

    // Clear registry
    // ...

    // Start instances from new config (factories re-create clients with new paths)
    m.startInstances(context.Background(), newConfig)
}
```

**Secret hot-reload vs config hot-reload:**
- **Config hot-reload:** integrations.yaml changes → full restart (existing)
- **Secret hot-reload:** Secret file changes → token update only (new, per-integration)

Both use fsnotify but at different layers:
- `IntegrationWatcher` watches integrations.yaml (Manager level)
- `SecretWatcher` watches secret files (Integration instance level)

## Data Flow Diagrams

### Query Flow (Normal Operation)

```
MCP Client (Claude)
        │
        │ CallTool("logzio_prod_search", {"namespace": "default", ...})
        │
        v
MCP Server (internal/mcp/server.go)
        │
        │ Lookup tool handler
        │
        v
SearchTool.Execute() (internal/integration/logzio/tools_search.go)
        │
        │ BuildElasticsearchDSL(params)
        │
        v
RegionalClient.SearchLogs() (internal/integration/logzio/client.go)
        │
        │ tokenMu.RLock()
        │ Authorization: Bearer <current_token>
        │ tokenMu.RUnlock()
        │
        v
Logz.io API (https://api-eu.logz.io/v1/search)
        │
        │ Elasticsearch DSL query execution
        │
        v
Response (JSON)
        │
        v
SearchTool formats response
        │
        v
MCP Client receives results
```

### Secret Rotation Flow

```
User updates Kubernetes Secret
        │
        v
Kubernetes updates volume mount
/var/lib/spectre/secrets/logzio-token
        │
        │ Atomic symlink rotation
        │
        v
fsnotify emits IN_DELETE_SELF event
        │
        v
SecretWatcher.watchLoop() (internal/integration/logzio/secret_watcher.go)
        │
        │ Re-add watch (handle broken inotify)
        │ Read new token from file
        │
        v
SecretWatcher.onUpdate(newToken)
        │
        │ Callback to integration
        │
        v
RegionalClient.UpdateToken(newToken)
        │
        │ tokenMu.Lock()
        │ apiToken = newToken
        │ tokenMu.Unlock()
        │
        v
Token updated (no pod restart)
        │
        │ Next query uses new token
        │
        v
Health check validates new token
```

### Error Recovery Flow

```
Token expires or is revoked
        │
        v
RegionalClient.SearchLogs() returns 401 Unauthorized
        │
        v
SearchTool.Execute() returns error
        │
        v
Manager health check detects Degraded state
        │
        │ Periodic health checks (30s interval)
        │
        v
LogzioIntegration.Health() returns integration.Degraded
        │
        v
Manager attempts auto-recovery
        │
        │ Calls integration.Start() again
        │
        v
Start() tests connectivity with current token
        │
        ├─ Success → Healthy (token was rotated by SecretWatcher)
        │
        └─ Failure → Degraded (token still invalid, user action needed)
```

## Suggested Build Order

### Phase 1: Core Client (No Secrets)

**Deliverables:**
- `internal/integration/logzio/client.go` (RegionalClient)
- `internal/integration/logzio/query.go` (Elasticsearch DSL builder)
- `internal/integration/logzio/types.go` (Request/response types)
- Unit tests with mocked HTTP responses

**Config (plain token):**
```yaml
instances:
  - name: logzio-dev
    type: logzio
    enabled: true
    config:
      region: us
      api_token: "plaintext-token-for-testing"  # NOT RECOMMENDED FOR PRODUCTION
```

**Rationale:**
- Test Logz.io API integration without secret complexity
- Validate region endpoint mapping
- Verify Elasticsearch DSL query generation
- Establish baseline health checks

**Dependencies:** None (uses existing plugin interfaces)

### Phase 2: Secret File Reading (No Hot-Reload)

**Deliverables:**
- `internal/integration/logzio/logzio.go` (Integration lifecycle)
- Config parsing for `api_token_path`
- Initial token read from file at startup
- Integration tests with file-mounted secrets

**Config (file path):**
```yaml
instances:
  - name: logzio-prod
    type: logzio
    enabled: true
    config:
      region: eu
      api_token_path: /var/lib/spectre/secrets/logzio-token
```

**Rationale:**
- De-risk secret file reading before hot-reload complexity
- Test Kubernetes secret volume mount pattern
- Validate file permissions and error handling
- Pod restart rotation works (baseline before hot-reload)

**Dependencies:** Phase 1 complete

### Phase 3: Secret Hot-Reload

**Deliverables:**
- `internal/integration/logzio/secret_watcher.go` (SecretWatcher)
- fsnotify integration with Kubernetes symlink behavior
- Thread-safe token updates in RegionalClient
- Integration tests simulating secret rotation

**Rationale:**
- Most complex component (fsnotify with atomic writes)
- Requires careful testing of inotify edge cases
- RWMutex must not block queries during rotation

**Dependencies:** Phase 2 complete

### Phase 4: MCP Tools

**Deliverables:**
- `internal/integration/logzio/tools_search.go` (Search tool)
- `internal/integration/logzio/tools_aggregations.go` (Aggregation tool)
- Tool registration in `RegisterTools()`
- E2E tests with MCP server

**Rationale:**
- Tools depend on stable client (Phase 1-3 complete)
- Can reuse VictoriaLogs tool patterns
- Easier to debug with working client

**Dependencies:** Phase 3 complete

### Phase 5: Helm Chart + Documentation

**Deliverables:**
- Update `chart/values.yaml` with secret mount examples
- Update `chart/templates/deployment.yaml` with extraVolumes/extraVolumeMounts
- README with secret rotation workflow
- Example Kubernetes Secret manifests

**Rationale:**
- Depends on all code being complete and tested
- Documentation should reflect actual implementation

**Dependencies:** Phase 4 complete

## Dependency Graph

```
Phase 1: Core Client
    │
    ├─ Elasticsearch DSL query builder
    ├─ Regional endpoint mapping
    ├─ HTTP client with bearer auth
    └─ Basic health checks
    │
    v
Phase 2: Secret File Reading
    │
    ├─ Config parsing (api_token_path)
    ├─ Initial token read from file
    ├─ Integration lifecycle (Start/Stop/Health)
    └─ Error handling for missing files
    │
    v
Phase 3: Secret Hot-Reload
    │
    ├─ SecretWatcher with fsnotify
    ├─ Atomic write handling (symlink rotation)
    ├─ Thread-safe token updates (RWMutex)
    └─ Watch re-establishment on IN_DELETE_SELF
    │
    v
Phase 4: MCP Tools
    │
    ├─ Tool registration (RegisterTools)
    ├─ Search tool (logs query)
    ├─ Aggregation tool (stats)
    └─ Tool naming convention (logzio_{instance}_*)
    │
    v
Phase 5: Helm Chart + Documentation
    │
    ├─ extraVolumes/extraVolumeMounts examples
    ├─ Secret rotation workflow docs
    └─ Integration guide
```

## Alternative Architectures Considered

### Alternative 1: Environment Variable for Token

**Approach:**
```yaml
env:
  - name: LOGZIO_API_TOKEN
    valueFrom:
      secretKeyRef:
        name: logzio-api-token
        key: token
```

**Why rejected:**
- Environment variables are immutable after pod start
- Token rotation requires pod restart (defeats hot-reload goal)
- No benefit over file-mounted secrets for this use case

### Alternative 2: External Secrets Operator

**Approach:** Use External Secrets Operator to sync secrets from Vault/AWS Secrets Manager

**Why NOT rejected (complementary):**
- External Secrets Operator writes to Kubernetes Secrets
- Kubernetes Secrets still mounted as files
- SecretWatcher still detects file changes
- **This is complementary, not alternative** (supports advanced secret backends)

### Alternative 3: Sidecar for Token Management

**Approach:** Deploy Vault Agent or secrets-sync sidecar

**Why rejected:**
- Adds deployment complexity (another container)
- Same file-mount pattern (sidecar writes, app reads)
- fsnotify in-process is simpler and sufficient

### Alternative 4: Direct Secret Store API Calls

**Approach:** Integration calls Vault/AWS Secrets Manager API directly

**Why rejected:**
- Tight coupling to specific secret store (not Kubernetes-native)
- Requires credentials to access secret store (chicken-egg problem)
- File-mount pattern works with any secret backend via Kubernetes

## Known Limitations and Trade-offs

### Limitation 1: fsnotify Event Delivery

**Issue:** fsnotify on Kubernetes secret volumes emits `IN_DELETE_SELF` on atomic writes, breaking the watch.

**Mitigation:**
- Re-establish watch after every event
- Add 50ms delay before re-adding watch (let rename complete)
- Test with rapid secret rotations (stress test)

**Source:** [Kubernetes inotify pitfalls](https://ahmet.im/blog/kubernetes-inotify/)

### Limitation 2: Token Rotation Window

**Issue:** Brief window where old token is invalid but new token not yet loaded.

**Mitigation:**
- RWMutex ensures queries block during token update (milliseconds)
- Health checks detect auth failures and mark Degraded
- Auto-recovery retries on next health check (30s interval)

**Trade-off:** Prefer availability over strict consistency (degraded state is acceptable)

### Limitation 3: Logz.io API Rate Limits

**Issue:** 100 concurrent API requests per account.

**Mitigation:**
- Document rate limits in README
- Consider connection pooling limits in HTTP client
- MCP tools are user-driven (low concurrency expected)

**Source:** [Logz.io API Rate Limits](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)

### Limitation 4: Query Result Limits

**Issue:** Logz.io returns max 10,000 results for non-aggregated queries, 1,000 for aggregated.

**Mitigation:**
- Document limits in tool descriptions
- Implement pagination if needed (Phase 4 decision)
- Encourage time range filtering for large datasets

**Source:** [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)

## Testing Strategy

### Unit Tests

**Component: RegionalClient**
- Region endpoint mapping correctness
- Bearer token header formatting
- Thread-safe token updates (concurrent reads/writes)
- HTTP error handling (401, 429, 500)

**Component: Query Builder**
- Elasticsearch DSL generation for various filter combinations
- Time range conversion (Unix timestamp → ISO 8601)
- Kubernetes field mapping (namespace, pod, container)

**Component: SecretWatcher**
- File read at startup
- fsnotify event handling
- Watch re-establishment after IN_DELETE_SELF
- Callback invocation on token change

### Integration Tests

**Test: Secret Rotation**
```go
// 1. Start integration with initial token
integration.Start(ctx)

// 2. Write new token to file
os.WriteFile(tokenPath, []byte("new-token"), 0600)

// 3. Wait for fsnotify event processing
time.Sleep(100 * time.Millisecond)

// 4. Verify client uses new token
response, err := client.SearchLogs(ctx, params)
assert.NoError(err)
```

**Test: Config Hot-Reload with Secret Path Change**
```go
// 1. Start with old secret path
manager.Start(ctx)

// 2. Update integrations.yaml with new secret path
updateConfig(newSecretPath)

// 3. Wait for config reload
time.Sleep(500 * time.Millisecond)

// 4. Verify integration reads from new path
verifySecretPath(integration, newSecretPath)
```

### E2E Tests

**Test: Full Rotation Workflow**
1. Deploy Spectre with Logz.io integration
2. Create Kubernetes Secret with initial token
3. Verify MCP tools work with initial token
4. Rotate token in Logz.io UI
5. Update Kubernetes Secret
6. Verify MCP tools work with new token (no pod restart)
7. Check health status remains Healthy

## Confidence Assessment

| Component | Confidence | Rationale |
|-----------|------------|-----------|
| Regional Client | **HIGH** | Logz.io API well-documented, standard REST + bearer auth, region mapping verified |
| Elasticsearch DSL | **HIGH** | Official docs with examples, Logz.io blog posts cover common queries |
| Secret Watcher | **MEDIUM** | fsnotify + Kubernetes symlinks have known pitfalls, needs careful testing |
| Integration Lifecycle | **HIGH** | Reuses VictoriaLogs pattern (proven architecture) |
| MCP Tools | **HIGH** | Same pattern as existing tools (cluster_health, resource_timeline) |
| Config Hot-Reload | **HIGH** | Already works for VictoriaLogs, no changes needed |
| Helm Chart | **HIGH** | extraVolumes/extraVolumeMounts are standard Kubernetes patterns |

**Overall confidence: HIGH** with Medium-confidence area flagged for extra testing (SecretWatcher).

## Research Gaps and Validation Needs

### Gap 1: Logz.io Field Names for Kubernetes Logs

**Issue:** Research found generic Kubernetes field examples but not Logz.io-specific field names.

**Validation needed:**
- Query actual Logz.io account for field names
- Check if fields are `kubernetes.namespace` or `k8s_namespace` or `namespace`
- Verify severity field name (`level`, `severity`, `log.level`?)

**Impact:** Low (field names discovered during Phase 1 testing)

### Gap 2: Logz.io Search API Pagination

**Issue:** Documentation mentions result limits but not pagination mechanism.

**Validation needed:**
- Test if `from` + `size` parameters work for pagination
- Check if cursor-based pagination is available
- Determine if multiple pages are needed for MCP tools

**Impact:** Medium (affects Phase 4 tool design if large result sets are common)

### Gap 3: fsnotify Behavior on Different Kubernetes Versions

**Issue:** Kubernetes secret mount behavior may vary across versions (1.25+ vs older).

**Validation needed:**
- Test on multiple Kubernetes versions (1.25, 1.27, 1.29)
- Verify atomic symlink rotation is consistent
- Check if ConfigMap projection behaves differently

**Impact:** Low (document minimum Kubernetes version if issues found)

## Sources

**Logz.io Documentation:**
- [Logz.io API Authentication](https://docs.logz.io/docs/user-guide/admin/authentication-tokens/api-tokens/)
- [Logz.io Regions](https://docs.logz.io/docs/user-guide/admin/hosting-regions/account-region/)
- [Logz.io Search API](https://api-docs.logz.io/docs/logz/search/)
- [Elasticsearch Query DSL Guide](https://logz.io/blog/elasticsearch-queries/)

**Kubernetes Secret Management:**
- [Kubernetes Secrets Documentation](https://kubernetes.io/docs/concepts/configuration/secret/)
- [Kubernetes inotify Pitfalls](https://ahmet.im/blog/kubernetes-inotify/)
- [Secrets Store CSI Driver Auto Rotation](https://secrets-store-csi-driver.sigs.k8s.io/topics/secret-auto-rotation)
- [Stakater Reloader](https://github.com/stakater/Reloader)

**Go Patterns:**
- [fsnotify Package Documentation](https://pkg.go.dev/github.com/fsnotify/fsnotify)
- [fsnotify Issue #372: Watching Single Files](https://github.com/fsnotify/fsnotify/issues/372)
- [Go Secrets Management for Kubernetes](https://oneuptime.com/blog/post/2026-01-07-go-secrets-management-kubernetes/view)

**Existing Spectre Code:**
- `internal/integration/victorialogs/victorialogs.go` (Integration pattern)
- `internal/integration/victorialogs/client.go` (HTTP client pattern)
- `internal/config/integration_watcher.go` (fsnotify pattern)
- `internal/mcp/server.go` (Tool registration pattern)
