# Phase 14: UI and Helm Chart - Research

**Researched:** 2026-01-22
**Domain:** React TypeScript UI forms, Helm chart volume mounting patterns
**Confidence:** HIGH

## Summary

Phase 14 delivers a Logz.io configuration form in the React UI and Helm chart support for mounting Kubernetes Secrets. The research confirms that the existing UI architecture is well-suited for this extension, with established patterns for integration forms, connection testing, and real-time updates via SSE.

The codebase already has the complete backend infrastructure for connection testing (via `/api/config/integrations/test` endpoint), health monitoring with SSE, and Secret watching via `SecretWatcher`. The Logz.io integration type exists with proper validation and supports the `SecretRef` pattern.

The Helm chart follows standard Kubernetes patterns with `extraVolumes`/`extraVolumeMounts` already documented in `values.yaml`, providing a proven pattern for Secret mounting documentation.

**Primary recommendation:** Extend the existing `IntegrationConfigForm.tsx` component with a Logz.io-specific form section, following the established VictoriaLogs pattern. Use native HTML `<select>` elements for the region dropdown (no external library needed). Document Helm secret mounting using the existing `extraVolumes`/`extraVolumeMounts` pattern with commented examples.

## Standard Stack

The established libraries/tools for this domain:

### Core (UI)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| React | 18.x | UI framework | Already in use, TypeScript-first |
| TypeScript | 5.x | Type safety | Project standard, full type coverage |
| Native HTML forms | N/A | Form elements | Best accessibility, zero bundle size |

### Supporting (UI)
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| react-hot-toast | 2.x (optional) | Toast notifications | If adding toast library; 5KB, zero deps |
| sonner | 1.x (optional) | Toast notifications | Alternative if using shadcn/ui patterns |

### Core (Backend - Existing)
| Library | Version | Purpose | Already Implemented |
|---------|---------|---------|---------------------|
| Kubernetes client-go | N/A | Secret watching | Yes - `SecretWatcher` exists |
| Go encoding/json | stdlib | Config validation | Yes - `Config.Validate()` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Native `<select>` | react-select | +Features, +20KB bundle, -accessibility effort |
| Inline notifications | Toast library | +UX polish, +5KB bundle, +dependency |
| Custom form validation | react-hook-form | +Features, -simple use case doesn't warrant it |

**Installation:**
```bash
# No new dependencies required for MVP
# Optional toast library if desired:
npm install react-hot-toast
```

## Architecture Patterns

### Recommended Project Structure (UI)
```
ui/src/
├── components/
│   ├── IntegrationModal.tsx           # Existing - modal wrapper
│   ├── IntegrationConfigForm.tsx      # EXTEND - add Logz.io section
│   └── IntegrationTable.tsx           # Existing - no changes
└── pages/
    └── IntegrationsPage.tsx            # Existing - no changes
```

### Pattern 1: Type-Specific Form Sections
**What:** Conditional rendering based on `config.type` within shared form component
**When to use:** Multiple integration types sharing common fields (name, enabled, type)
**Example:**
```typescript
// Source: ui/src/components/IntegrationConfigForm.tsx (lines 169-217)
// Existing pattern for VictoriaLogs:
{config.type === 'victorialogs' && (
  <div style={{ marginBottom: '20px' }}>
    <label htmlFor="integration-url">URL</label>
    <input
      id="integration-url"
      type="text"
      value={config.config.url || ''}
      onChange={handleUrlChange}
      placeholder="http://victorialogs:9428"
    />
  </div>
)}

// New pattern for Logz.io:
{config.type === 'logzio' && (
  <>
    {/* Region selector dropdown */}
    {/* SecretRef fields */}
  </>
)}
```

### Pattern 2: Config Object Nesting
**What:** Type-specific fields stored in `config.config` object, matches backend structure
**When to use:** Always - maintains consistency with API and backend validation
**Example:**
```typescript
// Source: internal/integration/logzio/types.go (lines 18-25)
// Backend expects this structure:
{
  name: "logzio-prod",
  type: "logzio",
  enabled: true,
  config: {
    region: "us",
    apiTokenRef: {
      secretName: "logzio-creds",
      key: "api-token"
    }
  }
}
```

### Pattern 3: Connection Test via API
**What:** POST to `/api/config/integrations/test` with full config object
**When to use:** Before saving (optional), triggered by "Test Connection" button
**Example:**
```typescript
// Source: ui/src/components/IntegrationModal.tsx (lines 113-136)
const handleTest = async () => {
  setIsTesting(true);
  const response = await fetch('/api/config/integrations/test', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
  const result = await response.json();
  setTestResult({
    success: result.success,
    message: result.message
  });
};
```

### Pattern 4: SSE for Real-Time Health Updates
**What:** Server-Sent Events stream at `/api/config/integrations/stream`
**When to use:** Table view for monitoring integration health status
**Example:**
```typescript
// Source: ui/src/pages/IntegrationsPage.tsx (lines 150-173)
useEffect(() => {
  const eventSource = new EventSource('/api/config/integrations/stream');
  eventSource.addEventListener('status', (event) => {
    const data = JSON.parse(event.data);
    setIntegrations(data || []);
  });
  return () => eventSource.close();
}, []);
```

### Pattern 5: Helm extraVolumes Secret Mounting
**What:** User-provided `extraVolumes` and `extraVolumeMounts` in values.yaml
**When to use:** Mounting Kubernetes Secrets into pods for sensitive configuration
**Example:**
```yaml
# Source: chart/values.yaml (lines 328-329) + Helm documentation pattern
extraVolumes:
  - name: logzio-secret
    secret:
      secretName: logzio-creds
      defaultMode: 0400

extraVolumeMounts:
  - name: logzio-secret
    mountPath: /var/secrets/logzio
    readOnly: true
```

### Anti-Patterns to Avoid
- **Custom dropdown libraries for simple use case:** React-select adds 20KB+ for functionality not needed (5 options, no search, no multi-select)
- **Environment variables for secrets:** Requires pod restart on rotation, no automatic updates
- **Global toast state management:** Inline notifications or simple toast library sufficient for this use case
- **Complex form libraries:** react-hook-form overkill for 3-4 fields with basic validation

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Accessible dropdowns | Custom styled `<select>` | Native `<select>` with styling | Browser handles ARIA, keyboard nav, screen readers |
| Secret watching in K8s | Custom Secret polling | Existing `SecretWatcher` | Already implemented, handles errors, caching, updates |
| Integration validation | Client-side validation | Backend `Config.Validate()` | Already exists, consistent with backend, type-safe |
| Connection testing | Custom health checks | Existing `/test` endpoint | Already implemented, uses integration's `Health()` method |
| Form state management | Redux/context | React `useState` | Simple form, no complex state, no cross-component sharing needed |

**Key insight:** The backend infrastructure for Phase 14 already exists. The SecretWatcher pattern, validation logic, connection testing, and health monitoring are proven and working for VictoriaLogs. Reuse these patterns rather than inventing new approaches.

## Common Pitfalls

### Pitfall 1: Using Custom Dropdown Libraries
**What goes wrong:** Adding react-select or similar for a 5-option dropdown
**Why it happens:** Developers assume custom styling requires custom library
**How to avoid:** Use native `<select>` with CSS styling - handles accessibility automatically
**Warning signs:** PR includes new dependencies for form components

### Pitfall 2: Forgetting readOnly on Secret Mounts
**What goes wrong:** Secret volumeMounts without `readOnly: true` flag
**Why it happens:** Example code often omits the security best practice
**How to avoid:** Always specify `readOnly: true` in volumeMount definitions
**Warning signs:** Security scanners flag writable secret mounts

### Pitfall 3: Incomplete Region URL Mapping
**What goes wrong:** Missing region in `GetBaseURL()` map returns empty string
**Why it happens:** Adding new region to validation but forgetting URL map
**How to avoid:** Keep validation and URL map in sync, add test coverage
**Warning signs:** Connection test fails with "invalid URL" for valid region

### Pitfall 4: Toast Notifications Without Auto-Dismiss
**What goes wrong:** Success toasts require manual dismiss, cluttering UI
**Why it happens:** Copying error notification pattern (which should persist)
**How to avoid:** Success = 3-5s auto-dismiss, Error = persist until dismissed
**Warning signs:** User feedback about closing success messages manually

### Pitfall 5: Not Handling Secret Rotation in Documentation
**What goes wrong:** Docs show Secret creation but not rotation workflow
**Why it happens:** Focusing on initial setup, assuming rotation is obvious
**How to avoid:** Document complete lifecycle: create → mount → verify → rotate
**Warning signs:** User questions about "how do I change the token?"

### Pitfall 6: Testing Connection Without SecretRef
**What goes wrong:** Connection test fails if Secret doesn't exist yet
**Why it happens:** Test logic assumes Secret is available at test time
**How to avoid:** Backend's `testConnection()` method already handles this - it creates temporary instance
**Warning signs:** Users report "can't test before saving" confusion

## Code Examples

Verified patterns from official sources:

### Region Selector Dropdown
```typescript
// Native select with TypeScript typing
interface Config {
  region: string;
  apiTokenRef?: {
    secretName: string;
    key: string;
  };
}

const REGIONS = [
  { code: 'us', label: 'US (United States)' },
  { code: 'eu', label: 'EU (Europe)' },
  { code: 'uk', label: 'UK (United Kingdom)' },
  { code: 'au', label: 'AU (Australia)' },
  { code: 'ca', label: 'CA (Canada)' },
];

const handleRegionChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
  onChange({
    ...config,
    config: { ...config.config, region: e.target.value }
  });
};

<select value={config.config.region || ''} onChange={handleRegionChange}>
  <option value="">Select a region...</option>
  {REGIONS.map(r => (
    <option key={r.code} value={r.code}>{r.label}</option>
  ))}
</select>
```

### SecretRef Fields
```typescript
// Two text inputs for Secret reference
const handleSecretNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
  onChange({
    ...config,
    config: {
      ...config.config,
      apiTokenRef: {
        ...config.config.apiTokenRef,
        secretName: e.target.value
      }
    }
  });
};

const handleSecretKeyChange = (e: React.ChangeEvent<HTMLInputElement>) => {
  onChange({
    ...config,
    config: {
      ...config.config,
      apiTokenRef: {
        ...config.config.apiTokenRef,
        key: e.target.value
      }
    }
  });
};

<div>
  <label>Secret Name</label>
  <input
    type="text"
    value={config.config.apiTokenRef?.secretName || ''}
    onChange={handleSecretNameChange}
    placeholder="logzio-creds"
  />
</div>
<div>
  <label>Key</label>
  <input
    type="text"
    value={config.config.apiTokenRef?.key || ''}
    onChange={handleSecretKeyChange}
    placeholder="api-token"
  />
</div>
```

### Connection Test with Specific Errors
```typescript
// Source: internal/api/handlers/integration_config_handler.go (lines 494-542)
// Backend returns structured errors:
// - "Failed to create instance: invalid config: region is required"
// - "Failed to start: failed to create secret watcher: Secret 'my-secret' not found"
// - "Health check failed: degraded"

// UI displays these directly:
{testResult && (
  <div style={{
    padding: '12px 16px',
    backgroundColor: testResult.success
      ? 'rgba(16, 185, 129, 0.1)'
      : 'rgba(239, 68, 68, 0.1)',
    border: `1px solid ${testResult.success ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)'}`
  }}>
    <span>{testResult.success ? '✓' : '✗'}</span>
    <span>{testResult.message}</span>
  </div>
)}
```

### Helm Secret Example (Commented)
```yaml
# Example Kubernetes Secret for Logz.io API token
# Create with: kubectl create secret generic logzio-creds \
#   --from-literal=api-token=YOUR_TOKEN_HERE \
#   --namespace monitoring

# Mount Secret into Spectre pod:
# extraVolumes:
#   - name: logzio-secret
#     secret:
#       secretName: logzio-creds
#       defaultMode: 0400
#
# extraVolumeMounts:
#   - name: logzio-secret
#     mountPath: /var/secrets/logzio
#     readOnly: true
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Embedding tokens in URLs | SecretRef pattern | Phase 13 | Enables rotation without restart |
| Manual Secret watching | SecretWatcher with cache | Phase 13 | Automatic updates, error recovery |
| Form libraries for simple forms | Native elements + TypeScript | 2023+ | Better accessibility, smaller bundles |
| Custom toast implementations | Specialized libraries (sonner, react-hot-toast) | 2024+ | Better UX, maintained, accessible |
| Generic `extraVolumes` docs | Type-specific Secret examples | Current | Copy-paste ready for users |

**Deprecated/outdated:**
- **react-select for simple dropdowns:** Native `<select>` with proper styling is now recommended for basic use cases (5-10 options, no search)
- **react-toastify for new projects:** Newer alternatives (sonner, react-hot-toast) have better DX and smaller bundles
- **Environment variables for rotating secrets:** Kubernetes Secret mounts with SecretWatcher pattern is current best practice

## Open Questions

Things that couldn't be fully resolved:

1. **Toast notification library choice**
   - What we know: Project doesn't currently use toast library; inline notifications exist in modal
   - What's unclear: Whether to add toast library or use inline notifications for connection test feedback
   - Recommendation: Start with inline notifications (existing pattern), add toast library if users request persistent notifications

2. **Documentation location for Helm examples**
   - What we know: No docs/ directory with Helm-specific guides; README.md has basic Helm install
   - What's unclear: Whether to add docs/helm/secrets.md or inline in values.yaml comments
   - Recommendation: Inline comments in values.yaml (copy-paste friendly) + README section

3. **Region selector UX preference**
   - What we know: User specified dropdown with "code + name" display (e.g., "US (United States)")
   - What's unclear: Whether to show region code or full name in saved configs display (table view)
   - Recommendation: Show full name in table (more user-friendly), store code in config (API requirement)

## Sources

### Primary (HIGH confidence)
- Existing codebase:
  - `/home/moritz/dev/spectre-via-ssh/ui/src/components/IntegrationConfigForm.tsx` - Form patterns
  - `/home/moritz/dev/spectre-via-ssh/ui/src/components/IntegrationModal.tsx` - Modal and test logic
  - `/home/moritz/dev/spectre-via-ssh/internal/integration/logzio/types.go` - Config structure
  - `/home/moritz/dev/spectre-via-ssh/internal/integration/logzio/logzio.go` - SecretWatcher usage
  - `/home/moritz/dev/spectre-via-ssh/internal/api/handlers/integration_config_handler.go` - Test endpoint
  - `/home/moritz/dev/spectre-via-ssh/chart/values.yaml` - Helm patterns
  - `/home/moritz/dev/spectre-via-ssh/chart/templates/deployment.yaml` - Volume mount templating

### Secondary (MEDIUM confidence)
- [Helm extraVolumes pattern documentation](https://docs.posit.co/helm/examples/connect/storage/additional-volumes.html)
- [Kubernetes Secret mounting examples](https://github.com/criblio/helm-charts/blob/master/common_docs/EXTRA_EXAMPLES.md)
- [Secret rotation best practices](https://medium.com/@quicksilversel/kubernetes-secrets-management-how-we-rotate-secrets-without-breaking-production-6c6ed6fcb115)

### Tertiary (LOW confidence)
- [React toast library comparison 2026](https://knock.app/blog/the-top-notification-libraries-for-react) - Market overview
- [React TypeScript form best practices](https://blog.logrocket.com/react-select-comprehensive-guide/) - General patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - Existing codebase demonstrates all patterns, no new libraries required
- Architecture: HIGH - Backend infrastructure complete, UI patterns established with VictoriaLogs
- Pitfalls: HIGH - Based on actual code inspection and K8s best practices documentation
- Helm patterns: HIGH - Chart already uses extraVolumes/extraVolumeMounts pattern

**Research date:** 2026-01-22
**Valid until:** 30 days (stable stack, no fast-moving dependencies)

**Key findings:**
1. No new dependencies required - use existing patterns
2. Backend infrastructure complete - focus is UI form extension
3. SecretWatcher pattern proven and battle-tested
4. Helm chart already supports secret mounting via extraVolumes
5. Native HTML `<select>` sufficient for 5-option region selector
