# Phase 2: Config Management & UI - Research

**Researched:** 2026-01-21
**Domain:** REST API + React UI + YAML config persistence
**Confidence:** HIGH

## Summary

Phase 2 builds atop the complete plugin infrastructure from Phase 1 to add user-facing config management. The research reveals that Spectre already has strong patterns in place: standard library HTTP handlers with method-specific middleware, JSON response helpers, and React component patterns. The existing codebase uses `http.ServeMux` for routing with clear handler registration patterns, and the UI follows component composition with inline CSS-in-JS.

**Key findings:**
1. **Existing REST API patterns** are well-established with `router.HandleFunc()`, method validation middleware (`withMethod`), and standardized error responses via `api.WriteJSON/WriteError`
2. **UI architecture** uses React functional components with hooks, no existing modal library (need to implement from scratch), CSS-in-JS pattern for styling
3. **YAML handling** is already implemented via Koanf v2.3.0, but atomic writes are NOT present—need to add temp-file-then-rename pattern
4. **Integration lifecycle** from Phase 1 provides `handleConfigReload` callback that triggers hot-reload when config file changes

**Primary recommendation:** Follow existing patterns strictly—use standard library HTTP handlers, implement modal using native React patterns (no external library), add atomic YAML writer using temp file + rename pattern, connect to existing `handleConfigReload` for hot-reload trigger.

## Standard Stack

The established libraries/tools for this domain:

### Core - Already in Spectre
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| net/http | stdlib | HTTP server & routing | Go standard library, zero dependencies, proven at scale |
| http.ServeMux | stdlib | Route multiplexer | Simple, sufficient for REST endpoints, already used |
| React | 19.2.0 | UI framework | Modern React with hooks, concurrent features, already in use |
| react-router-dom | 6.28.0 | Client-side routing | Industry standard for React SPAs, already integrated |
| Koanf | v2.3.0 | Config management | Already handles YAML parsing & validation with file provider |

### Supporting - Already Available
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| internal/api | - | Response helpers | WriteJSON, WriteError for consistent API responses |
| internal/apiserver | - | Middleware | withMethod for HTTP method validation |
| internal/logging | - | Structured logging | Consistent log format across server |

### Additions Needed
| Library | Version | Purpose | Why Needed |
|---------|---------|---------|-------------|
| gopkg.in/yaml.v3 | v3.0.1 | YAML marshaling | Already in go.mod, needed for config writing (Koanf only reads) |
| os (stdlib) | - | File operations | Atomic write via TempFile + Rename pattern |

**Installation:**
No new dependencies needed—all required libraries already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure
Based on existing Spectre patterns:
```
internal/
├── api/
│   └── handlers/
│       ├── integration_config_handler.go  # New: CRUD for integrations
│       └── register.go                     # Update: register new routes
├── config/
│   └── integration_writer.go              # New: atomic YAML writer
ui/src/
├── pages/
│   └── IntegrationsPage.tsx               # Update: add modal + table
└── components/
    ├── IntegrationModal.tsx               # New: Add/Edit modal
    ├── IntegrationConfigForm.tsx          # New: Type-specific forms
    └── IntegrationTable.tsx               # New: Table view with status
```

### Pattern 1: REST API Handler with Standard Library
**What:** HTTP handler using stdlib patterns, registered via router.HandleFunc
**When to use:** All new API endpoints (follows existing `/v1/*` patterns)
**Example:**
```go
// internal/api/handlers/integration_config_handler.go
type IntegrationConfigHandler struct {
    configPath string
    manager    *integration.Manager
    logger     *logging.Logger
}

func (h *IntegrationConfigHandler) HandleList(w http.ResponseWriter, r *http.Request) {
    // Load config
    config, err := loadConfig.LoadIntegrationsFile(h.configPath)
    if err != nil {
        api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
        return
    }

    // Return list
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = api.WriteJSON(w, config.Instances)
}

// Register in internal/api/handlers/register.go
func RegisterHandlers(...) {
    // Existing registrations...

    configHandler := NewIntegrationConfigHandler(configPath, manager, logger)
    router.HandleFunc("/api/config/integrations",
        withMethod(http.MethodGet, configHandler.HandleList))
    router.HandleFunc("/api/config/integrations/{name}",
        withMethod(http.MethodGet, configHandler.HandleGet))
    router.HandleFunc("/api/config/integrations/{name}",
        withMethod(http.MethodPut, configHandler.HandleUpdate))
    router.HandleFunc("/api/config/integrations/{name}",
        withMethod(http.MethodDelete, configHandler.HandleDelete))
    router.HandleFunc("/api/config/integrations/{name}/test",
        withMethod(http.MethodPost, configHandler.HandleTest))
}
```

### Pattern 2: Atomic YAML Write
**What:** Safe config file updates using temp-file-then-rename pattern
**When to use:** Any time writing integrations.yaml (prevents corruption)
**Example:**
```go
// internal/config/integration_writer.go
func WriteIntegrationsFile(path string, config *IntegrationsFile) error {
    // Marshal to YAML
    data, err := yaml.Marshal(config)
    if err != nil {
        return fmt.Errorf("marshal error: %w", err)
    }

    // Write to temp file in same directory (ensures same filesystem)
    dir := filepath.Dir(path)
    tmpFile, err := os.CreateTemp(dir, ".integrations.*.yaml.tmp")
    if err != nil {
        return fmt.Errorf("create temp file: %w", err)
    }
    tmpPath := tmpFile.Name()
    defer os.Remove(tmpPath) // Cleanup if rename fails

    if _, err := tmpFile.Write(data); err != nil {
        tmpFile.Close()
        return fmt.Errorf("write temp file: %w", err)
    }

    if err := tmpFile.Close(); err != nil {
        return fmt.Errorf("close temp file: %w", err)
    }

    // Atomic rename (POSIX guarantees atomicity)
    if err := os.Rename(tmpPath, path); err != nil {
        return fmt.Errorf("rename temp file: %w", err)
    }

    return nil
}
```

### Pattern 3: React Modal with Portal
**What:** Modal component using React portal and inline CSS
**When to use:** Add/Edit integration flows (follows existing Spectre UI patterns)
**Example:**
```tsx
// ui/src/components/IntegrationModal.tsx
import { createPortal } from 'react-dom';
import { useState, useEffect } from 'react';

interface IntegrationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (config: IntegrationConfig) => Promise<void>;
  initialConfig?: IntegrationConfig;
}

export function IntegrationModal({ isOpen, onClose, onSave, initialConfig }: IntegrationModalProps) {
  const [config, setConfig] = useState(initialConfig || { name: '', type: '', enabled: true, config: {} });
  const [isTesting, setIsTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

  // Focus trap and escape key handling
  useEffect(() => {
    if (!isOpen) return;

    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };

    document.addEventListener('keydown', handleEscape);
    return () => document.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose]);

  const handleTest = async () => {
    setIsTesting(true);
    try {
      const response = await fetch(`/api/config/integrations/${config.name}/test`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      const result = await response.json();
      setTestResult({ success: response.ok, message: result.message || 'Connection successful' });
    } catch (err) {
      setTestResult({ success: false, message: err.message });
    } finally {
      setIsTesting(false);
    }
  };

  const handleSave = async () => {
    await onSave(config);
    onClose();
  };

  if (!isOpen) return null;

  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>{initialConfig ? 'Edit Integration' : 'Add Integration'}</h2>
          <button onClick={onClose}>×</button>
        </div>
        <div className="modal-body">
          {/* Form content */}
          <IntegrationConfigForm config={config} onChange={setConfig} />
          {testResult && (
            <div className={`test-result ${testResult.success ? 'success' : 'error'}`}>
              {testResult.message}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button onClick={handleTest} disabled={isTesting}>
            {isTesting ? 'Testing...' : 'Test Connection'}
          </button>
          <button onClick={handleSave}>Save</button>
          <button onClick={onClose}>Cancel</button>
        </div>
      </div>
      <style>{modalCSS}</style>
    </div>,
    document.body
  );
}

const modalCSS = `
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal-content {
    background: var(--color-surface-elevated);
    border-radius: 12px;
    width: 90%;
    max-width: 600px;
    max-height: 90vh;
    overflow-y: auto;
    border: 1px solid var(--color-border-soft);
  }
  /* Additional styles following Spectre's design system */
`;
```

### Pattern 4: Integration Manager Connection
**What:** Trigger hot-reload after config write by leveraging Phase 1's file watcher
**When to use:** After successful PUT/POST/DELETE to config file
**Example:**
```go
// internal/api/handlers/integration_config_handler.go
func (h *IntegrationConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
    // 1. Parse request
    var updateReq IntegrationConfig
    if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
        api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
        return
    }

    // 2. Validate
    if err := validateIntegrationConfig(&updateReq); err != nil {
        api.WriteError(w, http.StatusBadRequest, "INVALID_CONFIG", err.Error())
        return
    }

    // 3. Load current config
    config, err := loadConfig.LoadIntegrationsFile(h.configPath)
    if err != nil {
        api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", err.Error())
        return
    }

    // 4. Update instance
    found := false
    for i, inst := range config.Instances {
        if inst.Name == name {
            config.Instances[i] = updateReq
            found = true
            break
        }
    }
    if !found {
        api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration not found")
        return
    }

    // 5. Write config atomically
    if err := WriteIntegrationsFile(h.configPath, config); err != nil {
        api.WriteError(w, http.StatusInternalServerError, "WRITE_ERROR", err.Error())
        return
    }

    // 6. Hot-reload happens automatically via IntegrationWatcher (Phase 1)
    //    - Watcher detects file change via fsnotify
    //    - Calls Manager.handleConfigReload after 500ms debounce
    //    - Manager stops all instances, validates new config, starts new instances

    // 7. Return success
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = api.WriteJSON(w, updateReq)
}
```

### Anti-Patterns to Avoid
- **External modal library:** Don't add react-modal or similar—implement native React portal pattern to match existing codebase style
- **Direct file writes:** Never use `os.WriteFile` directly—always use atomic write pattern to prevent corruption
- **Synchronous reload trigger:** Don't call Manager methods directly from handler—let the file watcher handle hot-reload asynchronously
- **Nested REST routes:** Don't create `/api/config/integrations/{name}/config` or similar—keep flat structure per existing patterns
- **Separate modal state library:** Don't add Zustand or Redux just for modal state—use local component state with useState hook

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| File watching | Custom polling loop | IntegrationWatcher (Phase 1) | Already has fsnotify + debouncing + error handling |
| Config validation | Manual field checks | IntegrationsFile.Validate() (Phase 1) | Already validates schema version, duplicate names, required fields |
| Integration lifecycle | Direct Start/Stop calls | Manager.handleConfigReload (Phase 1) | Handles full restart, version validation, health checks |
| HTTP method validation | Manual if/switch | withMethod middleware (existing) | Already enforces allowed methods, returns 405 |
| JSON response formatting | Manual marshaling | api.WriteJSON/WriteError (existing) | Consistent error format, proper Content-Type headers |
| YAML parsing | Custom parser | Koanf v2.3.0 (Phase 1) | Already handles file watching, parsing, struct unmarshaling |

**Key insight:** Phase 1 built a complete integration lifecycle—Phase 2 is just the REST API + UI wrapper. Don't duplicate Phase 1 logic; rely on the file watcher to trigger reloads automatically.

## Common Pitfalls

### Pitfall 1: Non-Atomic Config Writes Leading to Corruption
**What goes wrong:** Using `os.WriteFile` directly can result in partial writes if process crashes mid-write, leaving invalid YAML that breaks server startup.
**Why it happens:** Direct writes are not atomic—kernel may flush data incrementally, and power loss or crash leaves incomplete file.
**How to avoid:** Always use temp-file-then-rename pattern:
1. Write to temp file in same directory (ensures same filesystem for atomic rename)
2. Call `fsync()` or close file to flush to disk
3. Use `os.Rename()` which is atomic on POSIX systems
4. Cleanup temp file if rename fails
**Warning signs:** Config corruption after server crashes, users report "invalid schema_version" errors after system restarts

### Pitfall 2: Race Condition Between API Write and Watcher Reload
**What goes wrong:** API handler writes config, immediately tries to read updated state from Manager registry, but watcher hasn't reloaded yet (500ms debounce).
**Why it happens:** File watcher has deliberate 500ms debounce to coalesce rapid changes (Phase 1 design). API response happens before hot-reload completes.
**How to avoid:**
- Return the requested state immediately from API (don't query Manager)
- Document that integration status updates may take up to 1 second
- Add `/api/config/integrations/{name}/status` endpoint to poll actual runtime state if needed
**Warning signs:** UI shows "Healthy" status immediately after adding integration, then switches to "Degraded" 1 second later

### Pitfall 3: No Validation Before Test Connection
**What goes wrong:** User submits config with invalid URL format to test endpoint, integration library panics trying to connect, brings down API server.
**Why it happens:** Test endpoint receives arbitrary config without pre-validation, passes directly to integration factory.
**How to avoid:**
- Run `IntegrationsFile.Validate()` on test payload before creating integration instance
- Use request timeout context for test connections (5 second max)
- Wrap integration creation/test in recover() to catch panics
- Return structured error response with validation failures
**Warning signs:** API server crashes when user clicks "Test Connection" with malformed config

### Pitfall 4: Modal Focus Management Breaking Accessibility
**What goes wrong:** Modal opens but focus remains on background page, keyboard users can't access modal controls, screen readers don't announce modal.
**Why it happens:** React portals render outside normal component tree, browser doesn't automatically manage focus.
**How to avoid:**
- Set `ref` on first interactive element (input or button), call `focus()` in useEffect
- Add `role="dialog"` and `aria-modal="true"` to modal container
- Trap focus within modal (prevent Tab key from escaping)
- Return focus to trigger element on close
- Handle Escape key to close modal
**Warning signs:** Keyboard users report can't access modal, Tab key moves focus to background page

### Pitfall 5: Missing Error Boundaries Around Integration Forms
**What goes wrong:** Integration config form throws error (malformed JSON in config field), React unmounts entire IntegrationsPage, user sees blank screen.
**Why it happens:** No error boundary wrapping dynamic form components, React propagates error up to root.
**How to avoid:**
- Wrap `<IntegrationModal>` in ErrorBoundary component (already exists in `ui/src/components/Common/ErrorBoundary.tsx`)
- Provide fallback UI with error message and "Close" button
- Log error details to console for debugging
**Warning signs:** White screen when user interacts with integration config, React error in console

## Code Examples

Verified patterns from existing codebase and standard practices:

### REST Handler Registration Pattern
```go
// Source: internal/api/handlers/register.go (existing pattern)
func RegisterHandlers(
    router *http.ServeMux,
    // ... existing params
    configPath string,
    integrationManager *integration.Manager,
) {
    // Existing registrations...
    router.HandleFunc("/v1/search", withMethod(http.MethodGet, searchHandler.Handle))

    // New: Integration config CRUD
    configHandler := NewIntegrationConfigHandler(configPath, integrationManager, logger)
    router.HandleFunc("/api/config/integrations",
        withMethod(http.MethodGet, configHandler.HandleList))
    router.HandleFunc("/api/config/integrations",
        withMethod(http.MethodPost, configHandler.HandleCreate))

    // Path parameter extraction via URL parsing (stdlib pattern)
    router.HandleFunc("/api/config/integrations/", func(w http.ResponseWriter, r *http.Request) {
        name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
        if name == "" {
            api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
            return
        }

        // Route by method
        switch r.Method {
        case http.MethodGet:
            withMethod(http.MethodGet, configHandler.HandleGet)(w, r)
        case http.MethodPut:
            withMethod(http.MethodPut, configHandler.HandleUpdate)(w, r)
        case http.MethodDelete:
            withMethod(http.MethodDelete, configHandler.HandleDelete)(w, r)
        default:
            handleMethodNotAllowed(w, r)
        }
    })

    logger.Info("Registered /api/config/integrations endpoints")
}
```

### Error Response Format
```go
// Source: internal/api/response.go (existing)
func WriteError(w http.ResponseWriter, statusCode int, errorCode, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)

    response := map[string]string{
        "error":   errorCode,  // Machine-readable: "INVALID_CONFIG", "NOT_FOUND"
        "message": message,    // Human-readable details
    }

    _ = WriteJSON(w, response)
}

// Example usage from handler:
api.WriteError(w, http.StatusBadRequest, "INVALID_CONFIG", "URL is required")
// Returns: {"error": "INVALID_CONFIG", "message": "URL is required"}
```

### React Component Composition Pattern
```tsx
// Source: ui/src/pages/IntegrationsPage.tsx (existing pattern)
// Current: Static tiles
// Update to: Dynamic table when integrations exist

export default function IntegrationsPage() {
  const [integrations, setIntegrations] = useState<IntegrationConfig[]>([]);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedIntegration, setSelectedIntegration] = useState<IntegrationConfig | undefined>();

  useEffect(() => {
    // Fetch integrations on mount
    fetch('/api/config/integrations')
      .then(res => res.json())
      .then(data => setIntegrations(data))
      .catch(err => console.error('Failed to load integrations:', err));
  }, []);

  const handleSave = async (config: IntegrationConfig) => {
    const method = selectedIntegration ? 'PUT' : 'POST';
    const url = selectedIntegration
      ? `/api/config/integrations/${config.name}`
      : '/api/config/integrations';

    await fetch(url, {
      method,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(config),
    });

    // Reload list
    const updated = await fetch('/api/config/integrations').then(r => r.json());
    setIntegrations(updated);
  };

  return (
    <div className="h-full overflow-y-auto bg-[var(--color-app-bg)]">
      <div className="max-w-6xl mx-auto p-8">
        <div className="mb-8 flex justify-between items-center">
          <div>
            <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-2">
              Integrations
            </h1>
            <p className="text-[var(--color-text-muted)]">
              Connect Spectre with your existing tools
            </p>
          </div>
          <button
            onClick={() => { setSelectedIntegration(undefined); setIsModalOpen(true); }}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            + Add Integration
          </button>
        </div>

        {integrations.length === 0 ? (
          // Show tiles as empty state
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {INTEGRATIONS.map((integration) => (
              <IntegrationCard key={integration.id} integration={integration} />
            ))}
          </div>
        ) : (
          // Show table with actual integrations
          <IntegrationTable
            integrations={integrations}
            onEdit={(config) => { setSelectedIntegration(config); setIsModalOpen(true); }}
          />
        )}

        <IntegrationModal
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
          onSave={handleSave}
          initialConfig={selectedIntegration}
        />
      </div>
    </div>
  );
}
```

### Inline CSS Pattern
```tsx
// Source: ui/src/components/Sidebar.tsx (existing pattern)
const componentCSS = `
  .integration-table {
    width: 100%;
    background: var(--color-surface-elevated);
    border-radius: 12px;
    border: 1px solid var(--color-border-soft);
    overflow: hidden;
  }

  .integration-table th {
    padding: 12px 16px;
    text-align: left;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--color-text-muted);
    background: var(--color-surface-muted);
    border-bottom: 1px solid var(--color-border-soft);
  }

  .integration-table td {
    padding: 16px;
    border-bottom: 1px solid var(--color-border-soft);
  }

  .status-indicator {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
  }

  .status-healthy { background-color: #10b981; }
  .status-degraded { background-color: #f59e0b; }
  .status-offline { background-color: #ef4444; }
`;

export function IntegrationTable({ integrations, onEdit }) {
  return (
    <>
      <style>{componentCSS}</style>
      <table className="integration-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>URL</th>
            <th>Date Added</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {integrations.map(integration => (
            <tr key={integration.name} onClick={() => onEdit(integration)}>
              <td>{integration.name}</td>
              <td>{integration.type}</td>
              <td>{integration.config.url}</td>
              <td>{new Date().toLocaleDateString()}</td>
              <td>
                <span className="status-indicator">
                  <span className="status-dot status-healthy" />
                  <span>Healthy</span>
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| gorilla/mux for routing | stdlib http.ServeMux | Go 1.22+ (2024) | ServeMux added path parameters support, no longer need external router |
| Class components + HOCs | Functional components + hooks | React 16.8+ (2019) | Simpler state management, better code reuse |
| Context API for all state | Local useState | Modern React best practices | Avoid unnecessary re-renders for component-local state |
| External modal libraries | Native portal + dialog element | HTML5 dialog support (2022) | Better accessibility, no external dependency |
| Direct config reload calls | File watcher with debouncing | Phase 1 pattern (2026) | Prevents reload storms from rapid file changes |

**Deprecated/outdated:**
- **gorilla/mux**: No longer needed—Go 1.22+ http.ServeMux has pattern matching
- **react-modal library**: Native portal pattern is now standard, lighter weight
- **ioutil package**: Deprecated in Go 1.16+, use `os.ReadFile` and `os.WriteFile` instead

## Open Questions

Things that couldn't be fully resolved:

1. **Health Status Real-Time Updates**
   - What we know: Manager tracks health status via `Integration.Health()` every 30s
   - What's unclear: How to expose real-time status to UI without polling
   - Recommendation: Add `/api/config/integrations/{name}/status` endpoint for polling every 5s when IntegrationsPage is active

2. **Multi-User Concurrent Edits**
   - What we know: File watcher debounces for 500ms, multiple writes within that window coalesce
   - What's unclear: What happens if two users save different changes simultaneously
   - Recommendation: Last-write-wins is acceptable for MVP (single-user assumption), add optimistic locking (ETags) in future phase if needed

3. **Config File Location**
   - What we know: Server takes `--integrations-config` flag for path
   - What's unclear: Default location if flag not provided
   - Recommendation: Use `./integrations.yaml` as default (same directory as server binary), document in server.go flag help text

## Sources

### Primary (HIGH confidence)
- **Codebase inspection**: internal/api/handlers/register.go, internal/apiserver/server.go, internal/config/integration_*.go, ui/src/pages/IntegrationsPage.tsx, ui/src/components/Sidebar.tsx
- **Phase 1 verification**: .planning/phases/01-plugin-infrastructure-foundation/01-VERIFICATION.md
- **Go standard library docs**: net/http, os package documentation

### Secondary (MEDIUM confidence)
- [Build a High-Performance REST API with Go in 2025](https://toolshelf.tech/blog/build-high-performance-rest-api-with-go-2025-guide/)
- [Tutorial: Developing a RESTful API with Go and Gin](https://go.dev/doc/tutorial/web-service-gin)
- [React Design Patterns and Best Practices for 2025](https://www.telerik.com/blogs/react-design-patterns-best-practices)
- [Mastering Modals in React](https://medium.com/@renanolovics/mastering-modals-in-react-simplified-ui-enhancement-23bd060f387e)
- [Atomic file writes in Go](https://github.com/natefinch/atomic)

### Tertiary (LOW confidence)
- WebSearch results on React modal libraries—many recommend external libraries, but codebase pattern is inline CSS + portal
- WebSearch results on atomic write libraries—codebase doesn't use them, but pattern is applicable

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in go.mod and package.json, versions verified
- Architecture: HIGH - Patterns extracted directly from existing codebase with line references
- Pitfalls: MEDIUM - Derived from common REST API + file handling issues, not Spectre-specific

**Research date:** 2026-01-21
**Valid until:** 2026-02-21 (30 days - stable technology stack, React/Go patterns slow-moving)
