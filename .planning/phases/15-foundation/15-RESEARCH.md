# Phase 15: Foundation - Grafana API Client & Graph Schema - Research

**Researched:** 2026-01-22
**Domain:** Grafana API integration, FalkorDB graph database, Kubernetes secret management
**Confidence:** HIGH

## Summary

Research investigated how to build a Grafana API client that authenticates to both Cloud and self-hosted instances, retrieves dashboard metadata, validates connectivity, and stores dashboard structure in separate FalkorDB graph databases. The codebase already has strong patterns from VictoriaLogs and Logz.io integrations that can be followed.

Key findings:
- Grafana API uses service account tokens (Bearer auth) for both Cloud and self-hosted
- Dashboard listing via `/api/search` endpoint, retrieval via `/api/dashboards/uid/{uid}`
- Health check should test both dashboard read access AND datasource access (warn if datasource fails)
- FalkorDB supports multiple graph databases on same Redis instance
- Existing integration patterns provide complete blueprint for factory registration, SecretWatcher, health checks, UI forms

**Primary recommendation:** Follow VictoriaLogs/Logz.io integration pattern exactly. Use SecretWatcher for token hot-reload, create one FalkorDB graph per Grafana integration instance, implement health check that validates both dashboard and datasource access.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/FalkorDB/falkordb-go/v2 | v2 | FalkorDB graph database client | Already in use, supports multiple named graphs |
| k8s.io/client-go | - | Kubernetes Secret watching | Used by VictoriaLogs/Logz.io, proven pattern |
| net/http | stdlib | HTTP client for Grafana API | Standard library, no need for third-party HTTP lib |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| gopkg.in/yaml.v3 | v3 | Integration config marshaling | Already used for integration configs |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Manual HTTP client | grafana-api-golang-client | Third-party client adds dependency, may lag Grafana API changes. Manual HTTP gives full control and is already working pattern in Logz.io |

**Installation:**
Already in go.mod - no new dependencies needed

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── grafana.go              # Integration lifecycle (Start/Stop/Health/RegisterTools)
├── types.go                # Config, Dashboard metadata structures
├── client.go               # HTTP client for Grafana API
├── graph.go                # FalkorDB graph operations for dashboards
└── secret_watcher.go       # Reuse victorialogs.SecretWatcher
```

### Pattern 1: Integration Factory Registration
**What:** Compile-time registration using init() function
**When to use:** Every integration type needs global factory registration
**Example:**
```go
// Source: internal/integration/victorialogs/victorialogs.go:20-27
func init() {
	// Register the Grafana factory with the global registry
	if err := integration.RegisterFactory("grafana", NewGrafanaIntegration); err != nil {
		// Log but don't fail - factory might already be registered in tests
		logger := logging.GetLogger("integration.grafana")
		logger.Warn("Failed to register grafana factory: %v", err)
	}
}
```

### Pattern 2: SecretWatcher Integration
**What:** Hot-reload API tokens from Kubernetes Secrets without restart
**When to use:** When integration uses K8s Secret for credentials
**Example:**
```go
// Source: internal/integration/victorialogs/victorialogs.go:92-131
if v.config.UsesSecretRef() {
	// Create in-cluster Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}
	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	// Get current namespace from ServiceAccount mount
	namespace, err := getCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to determine namespace: %w", err)
	}

	// Create SecretWatcher
	secretWatcher, err := victorialogs.NewSecretWatcher(
		clientset,
		namespace,
		v.config.APITokenRef.SecretName,
		v.config.APITokenRef.Key,
		v.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create secret watcher: %w", err)
	}

	// Start SecretWatcher
	if err := secretWatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start secret watcher: %w", err)
	}

	v.secretWatcher = secretWatcher
}
```

### Pattern 3: Health Check Implementation
**What:** Test connectivity during Start() but warn on failure (degraded state)
**When to use:** Integration needs to validate connection without blocking startup
**Example:**
```go
// Source: internal/integration/victorialogs/victorialogs.go:151-154
// Test connectivity (warn on failure but continue - degraded state with auto-recovery)
if err := v.testConnection(ctx); err != nil {
	v.logger.Warn("Failed initial connectivity test (will retry on health checks): %v", err)
}
```

### Pattern 4: Multiple FalkorDB Graph Databases
**What:** Each integration instance gets its own isolated graph database
**When to use:** When multiple integration instances should not share data
**Example:**
```go
// Create graph client with specific graph name
graphConfig := graph.DefaultClientConfig()
graphConfig.GraphName = fmt.Sprintf("spectre_grafana_%s", integrationName)
graphConfig.Host = "falkordb"  // Service name in K8s
graphConfig.Port = 6379

client := graph.NewClient(graphConfig)
if err := client.Connect(ctx); err != nil {
	return fmt.Errorf("failed to connect to graph: %w", err)
}

// Initialize schema with indexes
if err := client.InitializeSchema(ctx); err != nil {
	return fmt.Errorf("failed to initialize schema: %w", err)
}
```

### Pattern 5: UI Form with Secret Reference
**What:** Integration form captures K8s Secret reference (name + key), not raw token
**When to use:** All integrations that require authentication
**Example:**
```typescript
// Source: ui/src/components/IntegrationConfigForm.tsx:312-425
// Authentication Section with Secret Name and Key fields
<div style={{
  marginBottom: '20px',
  padding: '16px',
  borderRadius: '8px',
  border: '1px solid var(--color-border-soft)',
  backgroundColor: 'var(--color-surface-muted)',
}}>
  <h4>Authentication</h4>

  {/* Secret Name */}
  <input
    value={config.config.apiTokenRef?.secretName || ''}
    onChange={handleSecretNameChange}
    placeholder="grafana-token"
  />

  {/* Secret Key */}
  <input
    value={config.config.apiTokenRef?.key || ''}
    onChange={handleSecretKeyChange}
    placeholder="api-token"
  />
</div>
```

### Anti-Patterns to Avoid
- **Direct token storage in config:** Never store raw API tokens in YAML config files. Always use K8s Secret references with SecretWatcher pattern for hot-reload.
- **Blocking startup on failed health check:** Integration should start in degraded state if connection fails, allowing auto-recovery when connectivity is restored.
- **Shared graph databases:** Each integration instance must have its own graph database to avoid data collision and enable clean deletion.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Kubernetes Secret watching | Custom Secret polling loop | victorialogs.SecretWatcher | Already implemented with proper watch API, handles reconnection, provides IsHealthy() check |
| HTTP retry logic | Custom retry wrapper | Standard http.Client with MaxRetries in Transport | VictoriaLogs client.go shows tuned transport settings (MaxIdleConnsPerHost: 10 to avoid connection churn) |
| Graph database connection | Custom Redis client | graph.NewClient() with FalkorDB wrapper | Handles Cypher query execution, parameter substitution, schema initialization |
| Integration config validation | Manual field checking | config.IntegrationsFile.Validate() | Centralized validation with helpful error messages |
| Health status tracking | Custom status enum | integration.HealthStatus type | Defined in integration/types.go (Healthy/Degraded/Stopped), integrated with SSE push |

**Key insight:** The VictoriaLogs and Logz.io integrations provide complete working examples of every pattern needed for Grafana. Don't reinvent - copy and adapt.

## Common Pitfalls

### Pitfall 1: Authentication Header Format
**What goes wrong:** Grafana API authentication fails with 401
**Why it happens:** Different header format than expected
**How to avoid:**
- Grafana uses standard `Authorization: Bearer <token>` header (not custom like Logz.io's `X-API-TOKEN`)
- Token is from Grafana Service Account (not API key - those are deprecated)
- Both Cloud and self-hosted use same Bearer token format
**Warning signs:** 401 Unauthorized response when token exists in Secret

### Pitfall 2: Dashboard UID vs ID
**What goes wrong:** Using deprecated numeric dashboard ID instead of UID
**Why it happens:** Older Grafana documentation mentioned ID, but it's deprecated
**How to avoid:**
- Always use UID (string, max 40 chars) for dashboard identification
- Search API returns both, but only store/use UID
- Dashboard retrieval endpoint: `/api/dashboards/uid/{uid}` not `/api/dashboards/{id}`
**Warning signs:** Inconsistent dashboard URLs across Grafana installs

### Pitfall 3: Health Check Scope
**What goes wrong:** Health check only validates dashboard access, not datasource access
**Why it happens:** Datasource access is a separate permission in Grafana RBAC
**How to avoid:**
- Test both dashboard read (`/api/search?limit=1`) AND datasource access (`/api/datasources`)
- If datasource access fails but dashboard succeeds: return Degraded status with warning message
- Don't block integration creation - allow saving with warning
**Warning signs:** Integration appears healthy but MCP tools fail when querying metrics

### Pitfall 4: Graph Database Naming Collision
**What goes wrong:** Multiple Grafana integrations share same graph database, causing data collision
**Why it happens:** Using static graph name like "spectre_grafana"
**How to avoid:**
- Graph name MUST include integration instance name: `spectre_grafana_{name}`
- Example: user creates "grafana-prod" and "grafana-staging" → graphs "spectre_grafana_prod" and "spectre_grafana_staging"
- When integration is deleted, delete its specific graph: `client.DeleteGraph(ctx)`
**Warning signs:** Dashboard data from one integration appears in another

### Pitfall 5: Pagination Handling
**What goes wrong:** Only first 1000 dashboards retrieved from large Grafana instances
**Why it happens:** `/api/search` defaults to limit=1000
**How to avoid:**
- Use `limit` (max 5000) and `page` parameters for pagination
- For initial implementation, fetch up to 5000 dashboards (single request with `?type=dash-db&limit=5000`)
- If more than 5000 dashboards exist, implement pagination loop in Phase 16
**Warning signs:** Integration with 2000+ dashboards only shows subset

## Code Examples

Verified patterns from codebase:

### Grafana Client HTTP Request with Bearer Token
```go
// Pattern from internal/integration/victorialogs/client.go:86-99
req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
if err != nil {
	return fmt.Errorf("create request: %w", err)
}
req.Header.Set("Content-Type", "application/json")

// Add authentication header if using secret watcher
if g.secretWatcher != nil {
	token, err := g.secretWatcher.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get API token: %w", err)
	}
	// Grafana uses standard Bearer token format
	req.Header.Set("Authorization", "Bearer "+token)
}
```

### FalkorDB Dashboard Node Upsert
```go
// Pattern adapted from internal/graph/schema.go:30-89
func UpsertDashboardNode(dashboard Dashboard) graph.GraphQuery {
	tagsJSON, _ := json.Marshal(dashboard.Tags)

	query := `
		MERGE (d:Dashboard {uid: $uid})
		ON CREATE SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.folder = $folder,
			d.url = $url,
			d.firstSeen = $firstSeen,
			d.lastSeen = $lastSeen
		ON MATCH SET
			d.title = $title,
			d.version = $version,
			d.tags = $tags,
			d.folder = $folder,
			d.url = $url,
			d.lastSeen = $lastSeen
	`

	return graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":       dashboard.UID,
			"title":     dashboard.Title,
			"version":   dashboard.Version,
			"tags":      string(tagsJSON),
			"folder":    dashboard.Folder,
			"url":       dashboard.URL,
			"firstSeen": time.Now().UnixNano(),
			"lastSeen":  time.Now().UnixNano(),
		},
	}
}
```

### Health Check with Dashboard and Datasource Validation
```go
func (g *GrafanaIntegration) testConnection(ctx context.Context) error {
	// Test 1: Dashboard read access
	dashboardURL := fmt.Sprintf("%s/api/search?type=dash-db&limit=1", g.config.URL)
	dashReq, _ := http.NewRequestWithContext(ctx, "GET", dashboardURL, nil)
	dashReq.Header.Set("Authorization", "Bearer "+g.getToken())

	dashResp, err := g.client.Do(dashReq)
	if err != nil {
		return fmt.Errorf("dashboard access failed: %w", err)
	}
	dashResp.Body.Close()

	if dashResp.StatusCode != 200 {
		return fmt.Errorf("dashboard access denied: status %d", dashResp.StatusCode)
	}

	// Test 2: Datasource access (warn if fails, don't block)
	datasourceURL := fmt.Sprintf("%s/api/datasources", g.config.URL)
	dsReq, _ := http.NewRequestWithContext(ctx, "GET", datasourceURL, nil)
	dsReq.Header.Set("Authorization", "Bearer "+g.getToken())

	dsResp, err := g.client.Do(dsReq)
	if err == nil {
		dsResp.Body.Close()
		if dsResp.StatusCode != 200 {
			g.logger.Warn("Datasource access limited: status %d (MCP metrics tools may fail)", dsResp.StatusCode)
		}
	} else {
		g.logger.Warn("Datasource access test failed: %v (MCP metrics tools may fail)", err)
	}

	return nil
}
```

### Integration Test Handler Pattern
```go
// Source: internal/api/handlers/integration_config_handler.go:494-542
func (h *IntegrationConfigHandler) testConnection(factory integration.IntegrationFactory, testReq TestConnectionRequest) (success bool, message string) {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			success = false
			message = fmt.Sprintf("Test panicked: %v", r)
		}
	}()

	// Create instance
	instance, err := factory(testReq.Name, testReq.Config)
	if err != nil {
		return false, fmt.Sprintf("Failed to create instance: %v", err)
	}

	// Start with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := instance.Start(ctx); err != nil {
		return false, fmt.Sprintf("Failed to start: %v", err)
	}

	// Check health
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer healthCancel()

	healthStatus := instance.Health(healthCtx)
	if healthStatus != integration.Healthy {
		// Stop cleanly even on failure
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = instance.Stop(stopCtx)

		return false, fmt.Sprintf("Health check failed: %s", healthStatus.String())
	}

	// Stop instance after successful test
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	_ = instance.Stop(stopCtx)

	return true, "Connection successful"
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Grafana API Keys | Service Account Tokens | Grafana v9+ | API keys deprecated, service account tokens more secure with fine-grained permissions |
| Dashboard numeric ID | Dashboard UID (string) | Grafana v5+ | UID allows consistent URLs across Grafana instances, ID is instance-specific |
| `/v1/search` endpoint | `/api/search` endpoint | Current | Older API versions deprecated, use current API |
| Manual health checks | Degraded state pattern | Current (this codebase) | Integrations start in degraded state on connection failure, auto-recover via periodic health checks |

**Deprecated/outdated:**
- **API Keys:** Replaced by Service Account tokens. API key endpoint still exists but marked deprecated.
- **Dashboard ID:** Use UID for all dashboard references. ID field still returned but should be ignored.
- **Health endpoint `/api/health`:** This checks Grafana's own health. For integration validation, test actual functionality (`/api/search`, `/api/datasources`).

## Open Questions

Things that couldn't be fully resolved:

1. **Datasource health check endpoint**
   - What we know: `/api/datasources/uid/{uid}/health` endpoint exists but is deprecated since Grafana v9.0.0
   - What's unclear: Best way to validate datasource access without deprecated endpoint
   - Recommendation: Use `/api/datasources` (list datasources) as proxy for datasource access permission. If 200 OK, user has datasource read access.

2. **Graph schema indexes for Dashboard nodes**
   - What we know: Dashboard nodes need uid, title, tags, folder fields. Existing ResourceIdentity has indexes on uid, kind, namespace.
   - What's unclear: Optimal index strategy for dashboard queries (by tag? by folder?)
   - Recommendation: Start with index on uid (primary lookup), add indexes on folder and tags in Phase 16 if query performance requires.

3. **Dashboard version tracking**
   - What we know: Dashboards have version field that increments on each save
   - What's unclear: Whether to track version history or just latest version
   - Recommendation: Phase 15 stores only latest version. Version history tracking deferred to Phase 17 (sync mechanism).

## Sources

### Primary (HIGH confidence)
- [Grafana Authentication API Documentation](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/authentication/)
- [Grafana Dashboard HTTP API Documentation](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/)
- [Grafana Folder/Dashboard Search API Documentation](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/folder_dashboard_search/)
- [Grafana Data Source HTTP API Documentation](https://grafana.com/docs/grafana/latest/developers/http_api/data_source/)
- Codebase: internal/integration/victorialogs/* (working implementation)
- Codebase: internal/integration/logzio/* (working implementation)
- Codebase: internal/graph/client.go (FalkorDB multi-graph support)

### Secondary (MEDIUM confidence)
- [Grafana Cloud vs Self-Hosted Comparison](https://grafana.com/oss-vs-cloud/)
- [Getting Started with Grafana API - Last9](https://last9.io/blog/getting-started-with-the-grafana-api/)

### Tertiary (LOW confidence)
- Community forum discussions on datasource health checks (deprecated endpoint, no clear replacement documented)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, proven patterns exist
- Architecture: HIGH - Direct copy of VictoriaLogs/Logz.io patterns
- Pitfalls: HIGH - Grafana API well-documented, auth patterns verified in existing code
- Graph schema: MEDIUM - Dashboard node structure straightforward, index strategy needs validation in Phase 16

**Research date:** 2026-01-22
**Valid until:** ~2026-04-22 (90 days - Grafana API is stable, existing integration patterns won't change)
