# Requirements: Spectre v1.2 Logz.io Integration

**Defined:** 2026-01-22
**Core Value:** Enable AI assistants to explore logs from multiple backends (VictoriaLogs + Logz.io) through unified MCP interface

## v1.2 Requirements

Requirements for Logz.io integration with secret management. Each maps to roadmap phases.

### Logz.io Client

- [ ] **LZIO-01**: HTTP client connects to Logz.io Search API with bearer token authentication
- [ ] **LZIO-02**: Client supports all 5 regional endpoints (US, EU, UK, AU, CA)
- [ ] **LZIO-03**: Query builder generates valid Elasticsearch DSL from structured parameters
- [ ] **LZIO-04**: Health check validates API token with minimal test query
- [ ] **LZIO-05**: Client handles rate limits with exponential backoff (100 concurrent limit)

### Secret Management

- [ ] **SECR-01**: Integration reads API token from file at startup (K8s Secret volume mount)
- [ ] **SECR-02**: fsnotify watches secret file for changes (hot-reload without pod restart)
- [ ] **SECR-03**: Token updates are thread-safe (RWMutex, concurrent queries not blocked)
- [ ] **SECR-04**: Secret values never logged or included in error messages
- [ ] **SECR-05**: Watch re-established after atomic write events (Kubernetes symlink rotation)

### MCP Tools

- [ ] **TOOL-01**: `logzio_{name}_overview` returns namespace severity summary (errors, warnings, total)
- [ ] **TOOL-02**: `logzio_{name}_logs` returns raw logs with filters (namespace, pod, container, level)
- [ ] **TOOL-03**: `logzio_{name}_patterns` returns log templates with occurrence counts
- [ ] **TOOL-04**: Tools enforce result limits (max 500 logs, max 50 templates)
- [ ] **TOOL-05**: Tools reject leading wildcard queries with helpful error message

### Configuration

- [ ] **CONF-01**: Integration config includes region and api_token_path fields
- [ ] **CONF-02**: UI displays Logz.io configuration form with region selector
- [ ] **CONF-03**: Connection test validates token before saving config

### Helm Chart

- [ ] **HELM-01**: Helm values include extraVolumes example for secret mounting
- [ ] **HELM-02**: Documentation covers secret rotation workflow
- [ ] **HELM-03**: Example Kubernetes Secret manifest provided

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Enhanced Features

- **LZIO-06**: Scroll API pagination for >1,000 results
- **LZIO-07**: Native pattern metadata if Logz.io API exposes it
- **SECR-06**: Dual-phase rotation support (multiple active tokens)
- **TOOL-06**: Time histogram aggregation for trend visualization

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Logz.io alerting integration | Logz.io has native alerting, Spectre is query-driven |
| Sub-account management | Out of scope for read-only observability tool |
| Environment variable secrets | No hot-reload support, file-based preferred |
| Multi-account parallel querying | Scroll API limited to single account |
| Grafana Cloud integration | Defer to v1.3 milestone |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| LZIO-01 | Phase 10 | Pending |
| LZIO-02 | Phase 10 | Pending |
| LZIO-03 | Phase 10 | Pending |
| LZIO-04 | Phase 10 | Pending |
| LZIO-05 | Phase 10 | Pending |
| SECR-01 | Phase 11 | Pending |
| SECR-02 | Phase 11 | Pending |
| SECR-03 | Phase 11 | Pending |
| SECR-04 | Phase 11 | Pending |
| SECR-05 | Phase 11 | Pending |
| TOOL-01 | Phase 12 | Pending |
| TOOL-02 | Phase 12 | Pending |
| TOOL-03 | Phase 13 | Pending |
| TOOL-04 | Phase 12 | Pending |
| TOOL-05 | Phase 12 | Pending |
| CONF-01 | Phase 10 | Pending |
| CONF-02 | Phase 14 | Pending |
| CONF-03 | Phase 14 | Pending |
| HELM-01 | Phase 14 | Pending |
| HELM-02 | Phase 14 | Pending |
| HELM-03 | Phase 14 | Pending |

**Coverage:**
- v1.2 requirements: 21 total
- Mapped to phases: 21
- Unmapped: 0 ✓

---
*Requirements defined: 2026-01-22*
*Last updated: 2026-01-22 after roadmap creation*
