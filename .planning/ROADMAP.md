# Project Roadmap: Spectre MCP Plugin System + VictoriaLogs Integration

**Project:** Spectre MCP Plugin System with VictoriaLogs Integration
**Created:** 2026-01-21
**Depth:** Standard (5-8 phases, 3-5 plans each)

## Overview

Enable AI assistants to explore logs progressively via MCP tools. Plugin system allows dynamic loading of observability integrations. VictoriaLogs integration delivers progressive disclosure: global overview → aggregated patterns → detailed logs.

This roadmap delivers 31 v1 requirements across 5 phases, building from plugin foundation through VictoriaLogs client, template mining, and progressive disclosure tools.

## Phases

### Phase 1: Plugin Infrastructure Foundation

**Goal:** MCP server dynamically loads/unloads integrations with clean lifecycle and config hot-reload.

**Dependencies:** None (foundation phase)

**Requirements:** PLUG-01, PLUG-02, PLUG-03, PLUG-04, PLUG-05, PLUG-06, CONF-01, CONF-03

**Success Criteria:**
1. MCP server discovers plugins via naming convention without manual registration
2. Plugin errors isolated (one broken plugin doesn't crash server)
3. MCP server hot-reloads config when integration file changes on disk
4. Plugins declare semantic version and server validates compatibility before loading

**Plans:** 4 plans

Plans:
- [x] 01-01-PLAN.md — Config schema & integration interface
- [x] 01-02-PLAN.md — Integration registry & config loader
- [x] 01-03-PLAN.md — Hot-reload with file watcher
- [x] 01-04-PLAN.md — Instance lifecycle & health management

**Notes:**
- Uses in-tree integrations (compiled into Spectre, not external plugins)
- Multiple instances of same integration type supported
- Atomic pointer swap pattern for race-free config reload
- Koanf v2.3.0 for hot-reload with fsnotify
- Research suggests this phase must be correct from day 1 (changing plugin system later forces complete rewrite)

---

### Phase 2: Config Management & UI

**Goal:** Users enable/configure integrations via UI backed by REST API.

**Dependencies:** Phase 1 (needs plugin system to configure)

**Requirements:** CONF-02, CONF-04, CONF-05

**Success Criteria:**
1. User sees available integrations in UI with enable/disable toggle
2. User configures integration connection details (e.g., VictoriaLogs URL) via UI
3. REST API persists integration config to disk and triggers hot-reload

**Plans:** 3 plans

Plans:
- [x] 02-01-PLAN.md — REST API for integration config CRUD with atomic writes
- [x] 02-02-PLAN.md — React UI components (modal, table, forms)
- [x] 02-03-PLAN.md — Server integration and end-to-end verification

**Notes:**
- REST API endpoints for reading/writing integration configs
- Atomic YAML writes using temp-file-then-rename pattern
- Reuses existing React UI patterns from Spectre
- Modal-based add/edit flow with connection testing
- Table view with health status indicators
- Hot-reload automatic via Phase 1 file watcher

---

### Phase 3: VictoriaLogs Client & Basic Pipeline

**Goal:** MCP server ingests logs into VictoriaLogs instance with backpressure handling.

**Dependencies:** Phase 1 (plugin system must exist), Phase 2 (VictoriaLogs URL configured)

**Requirements:** VLOG-01, VLOG-02, VLOG-03, VLOG-04, VLOG-05, VLOG-06

**Success Criteria:**
1. VictoriaLogs plugin connects to instance and queries logs using LogsQL syntax
2. Plugin supports time range filtering (default: last 60min, min: 15min)
3. Plugin returns log counts aggregated by time window (histograms)
4. Plugin returns log counts grouped by namespace/pod/deployment
5. Pipeline handles backpressure via bounded channels (prevents memory exhaustion)

**Plans:** 3 plans

Plans:
- [x] 03-01-PLAN.md — Core client implementation (types, query builder, HTTP client)
- [x] 03-02-PLAN.md — Pipeline & metrics (Prometheus instrumentation, backpressure handling)
- [x] 03-03-PLAN.md — Integration wiring & verification (wire client/pipeline into integration)
- [x] 03-04-PLAN.md — Gap closure: Time range validation (enforce 15-minute minimum)

**Notes:**
- HTTP client using net/http (stdlib) with tuned connection pooling (MaxIdleConnsPerHost: 10)
- Structured LogsQL query builder (no raw LogsQL exposed to MCP tools)
- Bounded channel pipeline (1000 buffer, 100-item batches) for backpressure
- Prometheus metrics for pipeline observability (queue depth, throughput, errors)
- 30-second query timeout per requirements
- No template mining yet (Phase 4)
- Validates VictoriaLogs integration before adding complexity

---

### Phase 4: Log Template Mining

**Goal:** Logs are automatically clustered into templates for pattern detection without manual config.

**Dependencies:** Phase 3 (needs log pipeline and VictoriaLogs client)

**Requirements:** MINE-01, MINE-02, MINE-03, MINE-04, MINE-05, MINE-06

**Success Criteria:**
1. Log processing package extracts templates using Drain algorithm with O(log n) matching
2. Template extraction normalizes logs (lowercase, remove numbers/UUIDs/IPs) for stable grouping
3. Templates have stable hash IDs for cross-client consistency
4. Canonical templates stored in MCP server and persist across restarts
5. Mining samples high-volume namespaces and uses time-window batching for efficiency

**Plans:** 4 plans

Plans:
- [ ] 04-01-PLAN.md — Core template mining foundation (Drain wrapper, template types, hashing)
- [ ] 04-02-PLAN.md — Processing pipeline (normalization, masking, K8s patterns)
- [ ] 04-03-PLAN.md — Storage & persistence (namespace store, disk snapshots)
- [ ] 04-04-PLAN.md — Lifecycle management (rebalancing, pruning, testing)

**Notes:**
- Log processing package is integration-agnostic (reusable beyond VictoriaLogs)
- Uses github.com/faceair/drain library (official Go port of Drain3)
- Post-tokenization masking to prevent template explosion from variable-starting logs
- Periodic rebalancing mechanism (5 minutes) to prevent template drift
- Count-based pruning (threshold: 10) and auto-merge (similarity: 0.7) for self-healing
- Namespace-scoped template storage for multi-tenant environments
- In-memory with periodic JSON snapshots (every 5 minutes) for persistence
- Comprehensive test suite targeting >80% coverage

---

### Phase 5: Progressive Disclosure MCP Tools

**Goal:** AI assistants explore logs progressively via MCP tools: overview → patterns → details.

**Dependencies:** Phase 3 (VictoriaLogs client), Phase 4 (template mining)

**Requirements:** PROG-01, PROG-02, PROG-03, PROG-04, PROG-05, NOVL-01, NOVL-02, NOVL-03

**Success Criteria:**
1. MCP tool returns global overview (error/panic/timeout counts by namespace over time)
2. MCP tool returns aggregated view (log templates with counts, novelty flags)
3. MCP tool returns full logs for specific scope (namespace + time range)
4. Tools preserve filter state across drill-down levels (no context loss)
5. Overview highlights errors, panics, timeouts first via smart defaults
6. System compares current templates to previous time window and flags novel patterns

**Plans:** 0 plans

Plans:
- [ ] TBD (awaiting `/gsd:plan-phase 5`)

**Notes:**
- Three-level drill-down: global → aggregated → detail
- MCP tool descriptions with JSON Schema inputs
- MCP Resources for VictoriaLogs schema docs
- Novelty detection compares to previous window (not long-term baseline)
- Research suggests limiting to 10-20 MCP tools maximum (context window constraints)

---

## Progress

| Phase | Status | Requirements | Plans | Completion |
|-------|--------|--------------|-------|------------|
| 1 - Plugin Infrastructure Foundation | ✓ Complete | 8/8 | 4/4 | 100% |
| 2 - Config Management & UI | ✓ Complete | 3/3 | 3/3 | 100% |
| 3 - VictoriaLogs Client & Basic Pipeline | ✓ Complete | 6/6 | 4/4 | 100% |
| 4 - Log Template Mining | Pending | 6/6 | 4/4 | 0% |
| 5 - Progressive Disclosure MCP Tools | Pending | 8/8 | 0/0 | 0% |

**Overall:** 17/31 requirements complete (55%)

---

## Coverage Validation

**Total v1 requirements:** 31
**Mapped to phases:** 31
**Unmapped:** 0

All v1 requirements covered. No orphaned requirements.

---

## Milestone Metadata

**Mode:** yolo
**Depth:** standard
**Parallelization:** enabled

---

*Last updated: 2026-01-21 (Phase 4 planned)*
