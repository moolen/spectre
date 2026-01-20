# Requirements: Spectre MCP Plugin System + VictoriaLogs Integration

**Defined:** 2026-01-20
**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Plugin System

- [ ] **PLUG-01**: MCP server discovers plugins via convention-based naming pattern
- [ ] **PLUG-02**: MCP server loads/unloads plugins with clean lifecycle (start/stop)
- [ ] **PLUG-03**: Plugin errors are isolated (one broken plugin doesn't crash server)
- [ ] **PLUG-04**: Plugin interface defines contract for tool registration
- [ ] **PLUG-05**: Plugins declare semantic version for compatibility checking
- [ ] **PLUG-06**: MCP server validates plugin version compatibility before loading

### Config Management

- [ ] **CONF-01**: Integration configs stored on disk (JSON/YAML)
- [ ] **CONF-02**: REST API endpoints for reading/writing integration configs
- [ ] **CONF-03**: MCP server hot-reloads config when file changes
- [ ] **CONF-04**: UI displays available integrations with enable/disable toggle
- [ ] **CONF-05**: UI allows configuring integration connection details (e.g., VictoriaLogs URL)

### VictoriaLogs Integration

- [ ] **VLOG-01**: VictoriaLogs plugin connects to VictoriaLogs instance via HTTP
- [ ] **VLOG-02**: Plugin queries logs using LogsQL syntax
- [ ] **VLOG-03**: Plugin supports time range filtering (default: last 60min, min: 15min)
- [ ] **VLOG-04**: Plugin supports field-based filtering (namespace, pod, level)
- [ ] **VLOG-05**: Plugin returns log count aggregated by time window (histograms)
- [ ] **VLOG-06**: Plugin returns log count grouped by namespace/pod/deployment

### Log Template Mining

- [ ] **MINE-01**: Log processing package extracts templates using Drain algorithm
- [ ] **MINE-02**: Template extraction normalizes logs (lowercase, remove numbers/UUIDs/IPs)
- [ ] **MINE-03**: Templates have stable hashes for cross-client consistency
- [ ] **MINE-04**: Canonical templates stored in MCP server for persistence
- [ ] **MINE-05**: Mining samples logs for high-volume namespaces (performance)
- [ ] **MINE-06**: Mining uses time-window batching for efficiency

### Novelty Detection

- [ ] **NOVL-01**: System compares current templates to previous time window
- [ ] **NOVL-02**: New patterns (not in previous window) are flagged as novel
- [ ] **NOVL-03**: High-volume patterns are ranked by count

### Progressive Disclosure Tools

- [ ] **PROG-01**: MCP tool returns global overview (error/panic/timeout counts by namespace over time)
- [ ] **PROG-02**: MCP tool returns aggregated view (log templates with counts, novelty flags)
- [ ] **PROG-03**: MCP tool returns full logs for specific scope (namespace + time range)
- [ ] **PROG-04**: Tools preserve filter state across drill-down levels
- [ ] **PROG-05**: Overview highlights errors, panics, timeouts first (smart defaults)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Additional Integrations

- **INT-01**: Logz.io integration with progressive disclosure
- **INT-02**: Grafana Cloud Loki integration with progressive disclosure
- **INT-03**: VictoriaMetrics (metrics) integration

### Advanced Features

- **ADV-01**: Long-term pattern baseline tracking (beyond single time window)
- **ADV-02**: Plugin scaffolding CLI for developers
- **ADV-03**: MCP Prompts for common log exploration workflows
- **ADV-04**: Health check hooks for plugin monitoring
- **ADV-05**: Anomaly scoring for log patterns

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| VictoriaLogs authentication | No auth needed (just base URL per user requirement) |
| Real-time log streaming (live tail) | Adds complexity, not needed for progressive disclosure workflow |
| Network-based plugin discovery | Unnecessary for local plugins, adds deployment complexity |
| Mobile UI | Web-first approach |
| Go native .so plugins | Platform limitations, build coupling — use go-plugin RPC instead |
| Unbounded log queries | Anti-pattern — always require time range |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| PLUG-01 | TBD | Pending |
| PLUG-02 | TBD | Pending |
| PLUG-03 | TBD | Pending |
| PLUG-04 | TBD | Pending |
| PLUG-05 | TBD | Pending |
| PLUG-06 | TBD | Pending |
| CONF-01 | TBD | Pending |
| CONF-02 | TBD | Pending |
| CONF-03 | TBD | Pending |
| CONF-04 | TBD | Pending |
| CONF-05 | TBD | Pending |
| VLOG-01 | TBD | Pending |
| VLOG-02 | TBD | Pending |
| VLOG-03 | TBD | Pending |
| VLOG-04 | TBD | Pending |
| VLOG-05 | TBD | Pending |
| VLOG-06 | TBD | Pending |
| MINE-01 | TBD | Pending |
| MINE-02 | TBD | Pending |
| MINE-03 | TBD | Pending |
| MINE-04 | TBD | Pending |
| MINE-05 | TBD | Pending |
| MINE-06 | TBD | Pending |
| NOVL-01 | TBD | Pending |
| NOVL-02 | TBD | Pending |
| NOVL-03 | TBD | Pending |
| PROG-01 | TBD | Pending |
| PROG-02 | TBD | Pending |
| PROG-03 | TBD | Pending |
| PROG-04 | TBD | Pending |
| PROG-05 | TBD | Pending |

**Coverage:**
- v1 requirements: 30 total
- Mapped to phases: 0
- Unmapped: 30

---
*Requirements defined: 2026-01-20*
*Last updated: 2026-01-20 after initial definition*
