# Requirements: Spectre MCP Plugin System + VictoriaLogs Integration

**Defined:** 2026-01-20
**Core Value:** Enable AI assistants to explore logs progressively—starting from high-level signals, drilling into patterns, and viewing raw logs only when context is narrow.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Plugin System

- [x] **PLUG-01**: MCP server discovers plugins via convention-based naming pattern
- [x] **PLUG-02**: MCP server loads/unloads plugins with clean lifecycle (start/stop)
- [x] **PLUG-03**: Plugin errors are isolated (one broken plugin doesn't crash server)
- [x] **PLUG-04**: Plugin interface defines contract for tool registration
- [x] **PLUG-05**: Plugins declare semantic version for compatibility checking
- [x] **PLUG-06**: MCP server validates plugin version compatibility before loading

### Config Management

- [x] **CONF-01**: Integration configs stored on disk (JSON/YAML)
- [x] **CONF-02**: REST API endpoints for reading/writing integration configs
- [x] **CONF-03**: MCP server hot-reloads config when file changes
- [x] **CONF-04**: UI displays available integrations with enable/disable toggle
- [x] **CONF-05**: UI allows configuring integration connection details (e.g., VictoriaLogs URL)

### VictoriaLogs Integration

- [x] **VLOG-01**: VictoriaLogs plugin connects to VictoriaLogs instance via HTTP
- [x] **VLOG-02**: Plugin queries logs using LogsQL syntax
- [x] **VLOG-03**: Plugin supports time range filtering (default: last 60min, min: 15min)
- [x] **VLOG-04**: Plugin supports field-based filtering (namespace, pod, level)
- [x] **VLOG-05**: Plugin returns log count aggregated by time window (histograms)
- [x] **VLOG-06**: Plugin returns log count grouped by namespace/pod/deployment

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
| PLUG-01 | Phase 1 | Complete |
| PLUG-02 | Phase 1 | Complete |
| PLUG-03 | Phase 1 | Complete |
| PLUG-04 | Phase 1 | Complete |
| PLUG-05 | Phase 1 | Complete |
| PLUG-06 | Phase 1 | Complete |
| CONF-01 | Phase 1 | Complete |
| CONF-02 | Phase 2 | Complete |
| CONF-03 | Phase 1 | Complete |
| CONF-04 | Phase 2 | Complete |
| CONF-05 | Phase 2 | Complete |
| VLOG-01 | Phase 3 | Complete |
| VLOG-02 | Phase 3 | Complete |
| VLOG-03 | Phase 3 | Complete |
| VLOG-04 | Phase 3 | Complete |
| VLOG-05 | Phase 3 | Complete |
| VLOG-06 | Phase 3 | Complete |
| MINE-01 | Phase 4 | Pending |
| MINE-02 | Phase 4 | Pending |
| MINE-03 | Phase 4 | Pending |
| MINE-04 | Phase 4 | Pending |
| MINE-05 | Phase 4 | Pending |
| MINE-06 | Phase 4 | Pending |
| NOVL-01 | Phase 5 | Pending |
| NOVL-02 | Phase 5 | Pending |
| NOVL-03 | Phase 5 | Pending |
| PROG-01 | Phase 5 | Pending |
| PROG-02 | Phase 5 | Pending |
| PROG-03 | Phase 5 | Pending |
| PROG-04 | Phase 5 | Pending |
| PROG-05 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 31 total
- Mapped to phases: 31
- Unmapped: 0

---
*Requirements defined: 2026-01-20*
*Last updated: 2026-01-21 (Phase 3 requirements marked complete)*
