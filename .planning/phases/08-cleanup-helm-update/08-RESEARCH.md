# Phase 8: Cleanup & Helm Chart Update - Research

**Researched:** 2026-01-21
**Domain:** CLI cleanup, Helm chart migration, documentation updates
**Confidence:** HIGH

## Summary

Phase 8 removes dead code from the MCP sidecar architecture and updates the Helm chart for single-container deployment. The research reveals that:

1. **CLI Commands**: Two commands need removal - `mcp` (already disabled in mcp.go:49) and `agent` (already disabled in agent.go:84-86). Both are currently stubbed with error messages. The `mock` command (mock.go) is build-excluded (`//go:build disabled`) but imports agent package.

2. **Agent Package**: The entire `internal/agent/` directory is build-excluded via `//go:build disabled` tags on all files. Package contains 11 subdirectories and is imported only by build-excluded code (mock.go) and within itself. Safe for complete deletion.

3. **Helm Chart**: Extensive MCP sidecar configuration exists across multiple files:
   - deployment.yaml (lines 158-206): Full MCP container definition with probes, resources, environment
   - values.yaml (lines 57-105): 49 lines of MCP sidecar configuration
   - service.yaml (lines 39-44): MCP port exposure
   - ingress.yaml: MCP-specific ingress rules (lines 1, 17, 28, 55-68)
   - Test fixtures: helm-values-test.yaml contains MCP sidecar config

4. **Documentation Impact**: 28 documentation files reference "MCP" with multiple containing sidecar architecture diagrams, deployment instructions, and troubleshooting guides for the old architecture.

**Primary recommendation:** Clean deletion approach - remove all traces of standalone MCP/agent commands and sidecar configuration. No deprecation stubs, no migration guides. Update documentation to reflect consolidated single-container architecture.

## Standard Stack

### Helm Chart Structure
Spectre uses standard Helm 3 chart structure with no custom deprecation mechanisms.

| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| Helm | v3.x | Kubernetes package manager | Industry standard for K8s deployments |
| Go | 1.24.4 | CLI and server implementation | Current stable Go version |
| Cobra | Latest | CLI command framework | Standard Go CLI framework (spf13/cobra) |

### Tools Used
| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| go build tags | Go 1.24.4 | Exclude code from compilation | Already applied to agent package |
| git | Any | Version control | Commit deletions for history preservation |

**Installation:**
```bash
# No new dependencies required - cleanup phase only
```

## Architecture Patterns

### Current State Assessment

**CLI Command Structure:**
```
cmd/spectre/commands/
├── root.go              # Root command, adds mcpCmd, agentCmd, debugCmd
├── server.go            # Main server command (kept)
├── mcp.go               # Standalone MCP command (DELETE)
├── mcp_health_test.go   # MCP health test (DELETE)
├── agent.go             # Agent command (DELETE)
├── mock.go              # Mock command (DELETE - imports agent package)
└── debug.go             # Debug command (kept)
```

**Agent Package Structure:**
```
internal/agent/          # All files have //go:build disabled
├── audit/               # Agent audit logging
├── commands/            # Agent TUI commands
├── incident/            # Incident agent
├── model/               # Model providers (Anthropic, Azure)
├── multiagent/          # Multi-agent pipeline
│   ├── builder/
│   ├── coordinator/
│   ├── gathering/
│   ├── intake/
│   ├── reviewer/
│   ├── rootcause/
│   └── types/
├── provider/            # Provider abstractions
├── runner/              # CLI runner
├── tools/               # Agent tools
└── tui/                 # Terminal UI
```

**Helm Chart MCP Sidecar Configuration:**
```
chart/
├── values.yaml
│   └── mcp:                      # Lines 57-105 (DELETE)
│       ├── enabled: true
│       ├── spectreURL
│       ├── httpAddr
│       ├── port: 8082
│       ├── resources
│       ├── securityContext
│       ├── extraArgs
│       ├── extraVolumeMounts
│       ├── livenessProbe
│       └── readinessProbe
└── templates/
    ├── deployment.yaml
    │   └── mcp container         # Lines 158-206 (DELETE)
    ├── service.yaml
    │   └── mcp port              # Lines 39-44 (DELETE)
    └── ingress.yaml
        └── mcp ingress rules     # Lines referencing .Values.mcp (MODIFY)
```

### Pattern 1: Clean Deletion with Git History

**What:** Remove all traces of deprecated functionality without leaving stubs or migration shims.

**When to use:** Breaking changes in minor version where clean break is acceptable (v1.1).

**Rationale:**
- User decisions specify "clean deletion with no traces"
- Git history preserves deleted code if needed
- No TODO comments, no deprecation warnings
- Cobra automatically shows "unknown command" error

**Example - Cobra's Unknown Command Behavior:**
```bash
# After deletion, Cobra automatically handles unknown commands:
$ spectre mcp
Error: unknown command "mcp" for "spectre"

Did you mean this?
        server
        debug

Run 'spectre --help' for usage.
```
Source: [Cobra Issue #706](https://github.com/spf13/cobra/issues/706)

### Pattern 2: Helm Values Silent Ignore

**What:** Remove values from values.yaml without validation or warnings. Old configs with deleted keys are silently ignored by Helm templates.

**When to use:** Breaking changes where old values don't cause errors, just have no effect.

**Rationale:**
- Helm templates use `{{ if .Values.mcp.enabled }}` - evaluates to false when missing
- No runtime errors from undefined values
- Users updating chart get new defaults automatically
- Clean values.yaml without deprecated sections

**Example:**
```yaml
# Old user values.yaml (still works, just ignored)
mcp:
  enabled: true
  port: 8082

# New chart ignores mcp section completely
# No validation error, no warning
# MCP served on main port 8080 at /v1/mcp path
```

### Pattern 3: Documentation Update for Consolidated Architecture

**What:** Update documentation to remove sidecar references and describe single-container architecture.

**Sections needing updates:**
- Architecture diagrams showing sidecar
- Deployment instructions mentioning MCP container
- Troubleshooting guides for sidecar issues
- Port allocation documentation (remove 8082 references)
- Health check endpoints (remove separate MCP health endpoint)

**Example:**
```markdown
# Old architecture diagram
┌─────────────────┐
│  Spectre Pod    │
│  ┌───────────┐  │
│  │ Spectre   │  │  Port 8080
│  │ Server    │  │
│  └───────────┘  │
│  ┌───────────┐  │
│  │ MCP       │  │  Port 8082
│  │ Sidecar   │  │
│  └───────────┘  │
└─────────────────┘

# New architecture diagram
┌─────────────────┐
│  Spectre Pod    │
│  ┌───────────┐  │
│  │ Spectre   │  │  Port 8080
│  │ Server    │  │  /v1/mcp endpoint
│  └───────────┘  │
└─────────────────┘
```

### Anti-Patterns to Avoid

- **Deprecation warnings**: Don't add warnings for deleted commands - Cobra handles this
- **Migration shims**: Don't proxy old MCP port to new endpoint - clean break
- **TODO comments**: Don't leave "TODO: remove this" comments - delete completely
- **Partial cleanup**: Don't leave unused imports or dead code paths

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Unknown command handling | Custom error messages | Cobra's built-in behavior | Cobra shows "Did you mean?" suggestions automatically |
| Helm value deprecation | Custom validation | Template conditionals | Helm ignores missing values in conditionals, no errors |
| Git history preservation | Archive old code in docs | Git history | Git log/blame provides complete history, searchable |

**Key insight:** Both Cobra and Helm have built-in mechanisms for handling removed functionality. Custom deprecation logic adds complexity without benefit.

## Common Pitfalls

### Pitfall 1: Forgetting Import Cleanup

**What goes wrong:** Removing command file but leaving it imported in root.go causes build failure.

**Why it happens:** Go requires all imports to resolve successfully.

**How to avoid:**
1. Remove command registration from root.go `init()` first
2. Remove command file
3. Test build: `go build ./cmd/spectre`

**Warning signs:**
```bash
# Build error indicating missing import
cmd/spectre/commands/root.go:40:15: undefined: mcpCmd
```

### Pitfall 2: Incomplete Helm Template Cleanup

**What goes wrong:** Removing values but leaving template conditionals that reference them causes rendering errors in edge cases.

**Why it happens:** Helm templates can have deeply nested references to removed values.

**How to avoid:**
1. Search for all references: `grep -r "\.Values\.mcp\." chart/templates/`
2. Remove or update all template blocks referencing deleted values
3. Test rendering: `helm template spectre chart/ --values chart/values.yaml`
4. Check ingress.yaml carefully - contains MCP-specific ingress rules

**Warning signs:**
```bash
# Helm template error
Error: template: spectre/templates/ingress.yaml:56:
  executing "spectre/templates/ingress.yaml" at <.Values.mcp.port>:
  nil pointer evaluating interface {}.port
```

### Pitfall 3: Documentation References Missed

**What goes wrong:** Updating main docs but missing references in examples, troubleshooting guides, or configuration reference.

**Why it happens:** Documentation spread across 28+ files with various contexts (getting started, troubleshooting, examples, configuration).

**How to avoid:**
1. Search all docs: `grep -r "sidecar\|localhost:3000\|8082\|mcp.enabled" docs/`
2. Review architecture diagrams for visual sidecar representations
3. Check configuration examples for old port references
4. Update troubleshooting sections removing sidecar-specific issues

**Warning signs:**
- Architecture diagrams showing two containers
- Port forwarding examples using 8082
- Troubleshooting "MCP container not starting"
- Configuration examples with `mcp.enabled: true`

### Pitfall 4: Test Fixture Staleness

**What goes wrong:** E2E tests continue passing with old helm-values-test.yaml but real deployments fail.

**Why it happens:** Test fixtures contain MCP sidecar configuration that's ignored if chart doesn't render it.

**How to avoid:**
1. Update tests/e2e/fixtures/helm-values-test.yaml to remove MCP section
2. Verify E2E tests still pass: `make test-e2e`
3. Check that tests validate single-container deployment

**Warning signs:**
```yaml
# In helm-values-test.yaml line 146
# Reduced MCP sidecar resources for CI
mcp:
  enabled: true
  resources:
    requests:
      memory: "32Mi"
```

### Pitfall 5: Build Tag Misunderstanding

**What goes wrong:** Assuming `//go:build disabled` means code isn't in repository, attempting to "re-exclude" it.

**Why it happens:** Build tags prevent compilation but code still exists in tree.

**How to avoid:**
- Understand: `//go:build disabled` = code exists but never compiles
- For cleanup: Delete the entire directory, don't modify build tags
- Build tags were temporary exclusion, deletion is permanent removal

**Warning signs:**
- Trying to add more restrictive build tags
- Checking if code "might be included" somehow

## Code Examples

### Example 1: Root Command Cleanup

**File:** `cmd/spectre/commands/root.go`

```go
// Before (lines 39-42)
func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)      // DELETE THIS
	rootCmd.AddCommand(debugCmd)
}

// After
func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(debugCmd)
}
```

### Example 2: Helm Deployment Template Cleanup

**File:** `chart/templates/deployment.yaml`

```yaml
# DELETE lines 158-206 (entire MCP container block)
# Before:
      {{- if .Values.mcp.enabled }}
      - name: mcp
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        # ... 48 lines of MCP container configuration ...
      {{- end }}

# After: Block completely removed
```

### Example 3: Helm Service Template Cleanup

**File:** `chart/templates/service.yaml`

```yaml
# DELETE lines 39-44 (MCP port exposure)
# Before:
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
    {{- if .Values.mcp.enabled }}
    - port: {{ .Values.mcp.port }}
      targetPort: mcp
      protocol: TCP
      name: mcp
    {{- end }}

# After:
  ports:
    - port: {{ .Values.service.port }}
      targetPort: http
      protocol: TCP
      name: http
```

### Example 4: Helm Values Port Documentation

**File:** `chart/values.yaml`

```yaml
# Before (lines 30-34):
# Service configuration
# Port allocation:
#   - 8080: HTTP REST API with gRPC-Web support (main service)
#   - 8082: MCP HTTP server (sidecar)
#   - 9999: pprof profiling endpoint

# After:
# Service configuration
# Port allocation:
#   - 8080: HTTP REST API with gRPC-Web support, MCP at /v1/mcp (main service)
#   - 9999: pprof profiling endpoint

# DELETE lines 57-105 (entire mcp: section)
```

### Example 5: Test Fixture Update

**File:** `tests/e2e/fixtures/helm-values-test.yaml`

```yaml
# DELETE lines 146-154 (MCP sidecar configuration)
# Before:
# Reduced MCP sidecar resources for CI
mcp:
  enabled: true
  resources:
    requests:
      memory: "32Mi"
      cpu: "25m"
    limits:
      memory: "128Mi"

# After: Section removed completely
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| MCP as separate container | MCP in-process on /v1/mcp | Phase 6 (Jan 2026) | Single container deployment |
| HTTP client for MCP tools | Direct service layer calls | Phase 7 (Jan 2026) | No network overhead |
| Standalone `spectre mcp` command | `spectre server` with MCP integrated | Phase 6 (Jan 2026) | Simplified CLI |
| Separate MCP port (8082) | Single port (8080) with path routing | Phase 6 (Jan 2026) | Simpler networking |

**Deprecated/outdated:**
- `spectre mcp` command: Removed in Phase 8, use `spectre server` (MCP on port 8080)
- `spectre agent` command: Removed in Phase 8, was disabled in Phase 7
- `mcp.enabled` Helm value: Removed in Phase 8, MCP always available at /v1/mcp
- `mcp.port` Helm value: Removed in Phase 8, use single service port 8080
- MCP sidecar container: Removed in Phase 8, consolidated into main container
- Helm ingress `mcp:` section: Removed in Phase 8, route /v1/mcp through main ingress

## Open Questions

1. **Default MCP path value**
   - What we know: Context decisions say "Add `mcp.path` option to allow customizing the MCP endpoint path (default: /v1/mcp)"
   - What's unclear: Should this be in values.yaml now or deferred to when users request customization?
   - Recommendation: Document `/v1/mcp` as the endpoint in README and values.yaml comments. Don't add `mcp.path` configuration option until user request. Simplicity over premature flexibility.

2. **Ingress template MCP section handling**
   - What we know: ingress.yaml has MCP-specific ingress rules (lines 1, 17, 28, 55-68)
   - What's unclear: Should we completely remove MCP ingress capability or update to route main ingress `/v1/mcp` path?
   - Recommendation: Remove separate `ingress.mcp` section from values.yaml. If users need ingress to MCP, they configure paths in main ingress section pointing to port 8080 with path `/v1/mcp`. Keep it simple, no special MCP ingress logic.

3. **Documentation update scope**
   - What we know: 28 documentation files reference "MCP", many contain sidecar architecture details
   - What's unclear: Update all 28 files vs. focus on user-facing docs (getting started, installation)?
   - Recommendation: Prioritize user-facing documentation (getting-started.md, installation/helm.md, configuration/mcp-configuration.md, architecture/overview.md). Internal/reference docs can remain unless they contradict new architecture. Project README.md must be updated as it's the first thing users see.

## Sources

### Primary (HIGH confidence)
- `/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/` - Direct inspection of CLI command structure
- `/home/moritz/dev/spectre-via-ssh/internal/agent/` - Verified build tag exclusion on all files
- `/home/moritz/dev/spectre-via-ssh/chart/` - Complete Helm chart structure and values
- `.planning/phases/08-cleanup-helm-update/08-CONTEXT.md` - User decisions from phase discussion

### Secondary (MEDIUM confidence)
- [Helm Charts Documentation](https://helm.sh/docs/topics/charts/) - Helm chart structure and best practices
- [Helm Chart Tips and Tricks](https://helm.sh/docs/howto/charts_tips_and_tricks/) - Template best practices
- [Cobra Unknown Command Handling](https://github.com/spf13/cobra/issues/706) - Default error behavior

### Tertiary (LOW confidence)
- [Helm Values Deprecation Issue](https://github.com/helm/helm/issues/8766) - No built-in deprecation mechanism confirmed
- [Grafana Mimir Helm Chart Breaking Changes](https://github.com/elastic/helm-charts/blob/main/BREAKING_CHANGES.md) - Example of breaking change documentation

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Direct inspection of go.mod, Chart.yaml, existing tooling
- Architecture: HIGH - Complete codebase analysis of files to delete and modify
- Pitfalls: HIGH - Identified specific line numbers and file locations for all changes

**Research date:** 2026-01-21
**Valid until:** 2026-02-21 (30 days - stable cleanup phase, no fast-moving dependencies)
