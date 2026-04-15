# Watcher-Disabled Timeline Full-Range Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the timeline page automatically open to the first and last available event timestamps, while skipping only the startup full-page chooser, when Spectre runs with `--watcher-enabled=false` and the URL does not already specify `start` and `end`.

**Architecture:** Expose watcher runtime mode through the existing `/health` response, cache that mode in the frontend API module, and keep URL params as the source of truth for timeline state. Add a small pure helper to normalize metadata time bounds so `TimelinePage` can safely convert watcher-disabled metadata into a valid initial range without changing live-watcher or demo-mode behavior.

**Tech Stack:** Go, Cobra, React 19, React Router, Vitest, Playwright-Go, existing Spectre API/UI/e2e helpers

---

## File Structure

- Create: `internal/api/server_health_test.go`
- Create: `ui/src/services/api.test.ts`
- Create: `ui/src/pages/timelineStartup.ts`
- Create: `ui/src/pages/timelineStartup.test.ts`
- Create: `ui/src/pages/TimelinePage.test.tsx`
- Modify: `internal/api/server.go`
- Modify: `cmd/spectre/commands/server.go`
- Modify: `ui/src/services/api.ts`
- Modify: `ui/src/pages/TimelinePage.tsx`
- Modify: `tests/e2e/import_export_stage_test.go`
- Modify: `tests/e2e/import_export_test.go`

### Task 1: Expose Watcher Mode Through `/health`

**Files:**
- Create: `internal/api/server_health_test.go`
- Modify: `internal/api/server.go`
- Modify: `cmd/spectre/commands/server.go`

- [ ] **Step 1: Write the failing backend health tests**

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServer_HealthIncludesWatcherEnabledFalse(t *testing.T) {
	server := NewWithStorage(8080, &mockQueryExecutor{}, nil, &mockReadinessChecker{ready: true}, false, false, &mockTelemetryProvider{})

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got, ok := resp["watcherEnabled"].(bool); !ok || got {
		t.Fatalf("expected watcherEnabled=false, got %#v", resp["watcherEnabled"])
	}
}

func TestServer_HealthIncludesWatcherEnabledTrue(t *testing.T) {
	server := NewWithStorage(8080, &mockQueryExecutor{}, nil, &mockReadinessChecker{ready: true}, false, true, &mockTelemetryProvider{})

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(rr, req)

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if got, ok := resp["watcherEnabled"].(bool); !ok || !got {
		t.Fatalf("expected watcherEnabled=true, got %#v", resp["watcherEnabled"])
	}
}
```

- [ ] **Step 2: Run the backend health tests to verify they fail**

Run: `go test ./internal/api -run 'TestServer_HealthIncludesWatcherEnabled(False|True)' -count=1`

Expected: FAIL with a compile error because `NewWithStorage` does not yet accept a `watcherEnabled` argument and `/health` does not return `watcherEnabled`.

- [ ] **Step 3: Add watcher mode plumbing to the API server**

```go
// internal/api/server.go
type Server struct {
	port             int
	server           *http.Server
	grpcServer       *grpc.Server
	grpcListener     net.Listener
	logger           *logging.Logger
	queryExecutor    QueryExecutor
	storage          *storage.Storage
	router           *http.ServeMux
	readinessChecker ReadinessChecker
	demoMode         bool
	watcherEnabled   bool
	tracingProvider  interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	}
}

func New(port int, queryExecutor QueryExecutor, readinessChecker ReadinessChecker, tracingProvider interface {
	GetTracer(string) trace.Tracer
	IsEnabled() bool
}) *Server {
	return NewWithStorage(port, queryExecutor, nil, readinessChecker, false, true, tracingProvider)
}

func NewWithStorage(port int, queryExecutor QueryExecutor, storage *storage.Storage, readinessChecker ReadinessChecker, demoMode bool, watcherEnabled bool, tracingProvider interface {
	GetTracer(string) trace.Tracer
	IsEnabled() bool
}) *Server {
	s := &Server{
		port:             port,
		logger:           logging.GetLogger("api"),
		queryExecutor:    queryExecutor,
		storage:          storage,
		router:           http.NewServeMux(),
		readinessChecker: readinessChecker,
		demoMode:         demoMode,
		watcherEnabled:   watcherEnabled,
		tracingProvider:  tracingProvider,
	}

	s.grpcServer = grpc.NewServer()

	var tracer trace.Tracer
	if tracingProvider != nil && tracingProvider.IsEnabled() {
		tracer = tracingProvider.GetTracer("spectre.api.grpc")
	} else {
		tracer = otel.GetTracerProvider().Tracer("spectre.api.grpc")
	}

	timelineGRPCService := NewTimelineGRPCService(queryExecutor, s.logger, tracer)
	pb.RegisterTimelineServiceServer(s.grpcServer, timelineGRPCService)

	grpcWebWrapper := grpcweb.WrapServer(s.grpcServer,
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(origin string) bool { return true }),
	)

	s.registerHandlers()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if grpcWebWrapper.IsGrpcWebRequest(r) || grpcWebWrapper.IsAcceptableGrpcCorsRequest(r) {
			grpcWebWrapper.ServeHTTP(w, r)
			return
		}
		s.corsMiddleware(s.router).ServeHTTP(w, r)
	})

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":         "healthy",
		"demo":           s.demoMode,
		"watcherEnabled": s.watcherEnabled,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = writeJSON(w, response)
}
```

```go
// cmd/spectre/commands/server.go
	apiComponent := api.NewWithStorage(
		cfg.APIPort,
		queryExecutor,
		storageComponent,
		readinessChecker,
		demo,
		watcherEnabled,
		tracingProvider,
	)
```

- [ ] **Step 4: Run the backend health tests and the existing API route smoke test**

Run: `go test ./internal/api -run 'TestServer_HealthIncludesWatcherEnabled(False|True)|TestServer_Routes' -count=1`

Expected:

```text
ok  	github.com/moolen/spectre/internal/api
```

- [ ] **Step 5: Commit the backend health plumbing**

```bash
git add internal/api/server.go internal/api/server_health_test.go cmd/spectre/commands/server.go
git commit -m "feat: expose watcher mode in health endpoint"
```

### Task 2: Cache Runtime Mode In The Frontend API Module

**Files:**
- Create: `ui/src/services/api.test.ts`
- Modify: `ui/src/services/api.ts`

- [ ] **Step 1: Write the failing frontend runtime-mode tests**

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  detectDemoMode,
  getDemoMode,
  getWatcherEnabled,
  resetRuntimeModeForTests,
} from './api';

describe('api runtime mode detection', () => {
  afterEach(() => {
    resetRuntimeModeForTests();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('caches demo and watcherEnabled from /health', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ demo: false, watcherEnabled: false }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const demo = await detectDemoMode();

    expect(demo).toBe(false);
    expect(getDemoMode()).toBe(false);
    expect(getWatcherEnabled()).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('defaults watcherEnabled to true when the health response omits it', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ demo: true }),
    }));

    const demo = await detectDemoMode();

    expect(demo).toBe(true);
    expect(getWatcherEnabled()).toBe(true);
  });
});
```

- [ ] **Step 2: Run the frontend runtime-mode tests to verify they fail**

Run: `cd ui && npm test -- src/services/api.test.ts`

Expected: FAIL because `getWatcherEnabled` and `resetRuntimeModeForTests` do not exist yet.

- [ ] **Step 3: Replace the demo-only cache with a runtime-mode cache**

```ts
// ui/src/services/api.ts
const BUILD_TIME_DEMO_MODE = import.meta.env.VITE_DEMO_MODE === 'true';

interface RuntimeMode {
  demo: boolean;
  watcherEnabled: boolean;
}

let runtimeMode: RuntimeMode | null = null;

export async function detectDemoMode(): Promise<boolean> {
  if (runtimeMode !== null) {
    return runtimeMode.demo;
  }

  if (BUILD_TIME_DEMO_MODE) {
    runtimeMode = { demo: true, watcherEnabled: true };
    return true;
  }

  try {
    const response = await fetch('/health', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' },
    });

    if (!response.ok) {
      runtimeMode = { demo: BUILD_TIME_DEMO_MODE, watcherEnabled: true };
      return runtimeMode.demo;
    }

    const data = await response.json() as { demo?: boolean; watcherEnabled?: boolean };
    runtimeMode = {
      demo: data.demo ?? false,
      watcherEnabled: data.watcherEnabled ?? true,
    };
    return runtimeMode.demo;
  } catch {
    runtimeMode = { demo: BUILD_TIME_DEMO_MODE, watcherEnabled: true };
    return runtimeMode.demo;
  }
}

export function getDemoMode(): boolean {
  if (runtimeMode !== null) {
    return runtimeMode.demo;
  }
  return BUILD_TIME_DEMO_MODE;
}

export function getWatcherEnabled(): boolean {
  if (runtimeMode !== null) {
    return runtimeMode.watcherEnabled;
  }
  return true;
}

export function resetRuntimeModeForTests(): void {
  runtimeMode = null;
}
```

- [ ] **Step 4: Run the frontend runtime-mode tests and one existing UI component test**

Run: `cd ui && npm test -- src/services/api.test.ts src/components/FilterBar.test.tsx`

Expected:

```text
✓ src/services/api.test.ts
✓ src/components/FilterBar.test.tsx
```

- [ ] **Step 5: Commit the frontend runtime-mode cache**

```bash
git add ui/src/services/api.ts ui/src/services/api.test.ts
git commit -m "feat: cache watcher mode in frontend runtime state"
```

### Task 3: Add Safe Metadata-Bounds Normalization For Watcher-Disabled Startup

**Files:**
- Create: `ui/src/pages/timelineStartup.ts`
- Create: `ui/src/pages/timelineStartup.test.ts`

- [ ] **Step 1: Write the failing pure helper tests**

```ts
import { describe, expect, it } from 'vitest';
import { buildWatcherDisabledInitialRange } from './timelineStartup';

describe('buildWatcherDisabledInitialRange', () => {
  it('returns the metadata bounds when earliest is before latest', () => {
    const range = buildWatcherDisabledInitialRange({ earliest: 1735689600, latest: 1735693200 });

    expect(range).not.toBeNull();
    expect(range?.start.toISOString()).toBe('2025-01-01T00:00:00.000Z');
    expect(range?.end.toISOString()).toBe('2025-01-01T01:00:00.000Z');
  });

  it('pads single-event bounds by one second on each side', () => {
    const range = buildWatcherDisabledInitialRange({ earliest: 1735689600, latest: 1735689600 });

    expect(range?.start.toISOString()).toBe('2024-12-31T23:59:59.000Z');
    expect(range?.end.toISOString()).toBe('2025-01-01T00:00:01.000Z');
  });

  it('returns null when the bounds are missing or invalid', () => {
    expect(buildWatcherDisabledInitialRange({ earliest: 0, latest: 0 })).toBeNull();
    expect(buildWatcherDisabledInitialRange({ earliest: -1, latest: 10 })).toBeNull();
    expect(buildWatcherDisabledInitialRange({ earliest: 20, latest: 10 })).toBeNull();
  });
});
```

- [ ] **Step 2: Run the helper tests to verify they fail**

Run: `cd ui && npm test -- src/pages/timelineStartup.test.ts`

Expected: FAIL because `timelineStartup.ts` does not exist yet.

- [ ] **Step 3: Implement the metadata-bounds helper**

```ts
// ui/src/pages/timelineStartup.ts
import { TimeRangeInfo } from '../services/apiTypes';
import { TimeRange } from '../types';

export function buildWatcherDisabledInitialRange(bounds: TimeRangeInfo | null | undefined): TimeRange | null {
  if (!bounds) {
    return null;
  }

  const { earliest, latest } = bounds;
  if (earliest <= 0 || latest <= 0 || latest < earliest) {
    return null;
  }

  if (earliest === latest) {
    return {
      start: new Date((earliest - 1) * 1000),
      end: new Date((latest + 1) * 1000),
    };
  }

  return {
    start: new Date(earliest * 1000),
    end: new Date(latest * 1000),
  };
}
```

- [ ] **Step 4: Run the helper tests**

Run: `cd ui && npm test -- src/pages/timelineStartup.test.ts`

Expected:

```text
✓ src/pages/timelineStartup.test.ts
```

- [ ] **Step 5: Commit the pure metadata-bounds helper**

```bash
git add ui/src/pages/timelineStartup.ts ui/src/pages/timelineStartup.test.ts
git commit -m "test: add watcher-disabled startup range helper"
```

### Task 4: Teach `TimelinePage` To Auto-Initialize Only In Watcher-Disabled Mode

**Files:**
- Create: `ui/src/pages/TimelinePage.test.tsx`
- Modify: `ui/src/pages/TimelinePage.tsx`

- [ ] **Step 1: Write the failing `TimelinePage` startup tests**

```tsx
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import TimelinePage from './TimelinePage';

const mockGetMetadata = vi.fn();
const mockGetWatcherEnabled = vi.fn(() => true);
const mockGetDemoMode = vi.fn(() => false);
const mockUseTimeline = vi.fn(() => ({
  resources: [],
  loading: false,
  error: null,
  totalCount: 0,
  loadedCount: 0,
}));

vi.mock('../services/api', () => ({
  apiClient: { getMetadata: (...args: unknown[]) => mockGetMetadata(...args) },
  getDemoMode: () => mockGetDemoMode(),
  getWatcherEnabled: () => mockGetWatcherEnabled(),
}));

vi.mock('../hooks/useTimeline', () => ({
  useTimeline: (...args: unknown[]) => mockUseTimeline(...args),
}));

vi.mock('../hooks/useMetadata', () => ({
  useMetadata: () => ({ namespaces: [], kinds: [], loading: false, error: null }),
}));

vi.mock('../hooks/usePersistedFilters', () => ({
  usePersistedFilters: () => ({ kinds: [], namespaces: [], setKinds: vi.fn(), setNamespaces: vi.fn() }),
}));

vi.mock('../hooks/usePersistedQuickPreset', () => ({
  usePersistedQuickPreset: () => ({ clearPreset: vi.fn() }),
}));

vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({ autoRefresh: 'off' }),
}));

vi.mock('../components/FilterBar', () => ({
  FilterBar: () => <div data-testid="filter-bar">filter-bar</div>,
}));

vi.mock('../components/Timeline', () => ({
  Timeline: () => <div data-testid="timeline">timeline</div>,
}));

vi.mock('../components/DetailPanel', () => ({
  DetailPanel: () => <div data-testid="detail-panel">detail-panel</div>,
}));

vi.mock('../components/TimeRangePicker', () => ({
  TimeRangePicker: () => <div data-testid="time-range-picker">picker</div>,
}));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.search}</div>;
}

describe('TimelinePage watcher-disabled startup', () => {
  beforeEach(() => {
    mockGetMetadata.mockReset();
    mockGetWatcherEnabled.mockReset();
    mockGetDemoMode.mockReset();
    mockUseTimeline.mockClear();
    mockGetWatcherEnabled.mockReturnValue(true);
    mockGetDemoMode.mockReturnValue(false);
  });

  it('uses metadata bounds when watcher is disabled, skips the startup chooser, and keeps navbar controls', async () => {
    mockGetWatcherEnabled.mockReturnValue(false);
    mockGetMetadata.mockResolvedValue({
      namespaces: [],
      kinds: [],
      groups: [],
      resourceCounts: {},
      totalEvents: 10,
      timeRange: { earliest: 1735689600, latest: 1735693200 },
    });

    render(
      <MemoryRouter initialEntries={['/']}>
        <Routes>
          <Route path="/" element={<><TimelinePage /><LocationProbe /></>} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => expect(mockGetMetadata).toHaveBeenCalledTimes(1));

    const params = new URLSearchParams(screen.getByTestId('location').textContent ?? '');
    expect(params.get('start')).toBe('2025-01-01T00:00:00.000Z');
    expect(params.get('end')).toBe('2025-01-01T01:00:00.000Z');
    expect(screen.queryByTestId('time-range-picker')).not.toBeInTheDocument();
    expect(screen.getByTestId('filter-bar')).toBeInTheDocument();
  });

  it('preserves explicit URL params when watcher is disabled', async () => {
    mockGetWatcherEnabled.mockReturnValue(false);

    render(
      <MemoryRouter initialEntries={['/?start=2025-01-01T02:00:00.000Z&end=2025-01-01T03:00:00.000Z']}>
        <Routes>
          <Route path="/" element={<><TimelinePage /><LocationProbe /></>} />
        </Routes>
      </MemoryRouter>
    );

    await waitFor(() => expect(mockUseTimeline).toHaveBeenCalled());

    expect(mockGetMetadata).not.toHaveBeenCalled();
    const params = new URLSearchParams(screen.getByTestId('location').textContent ?? '');
    expect(params.get('start')).toBe('2025-01-01T02:00:00.000Z');
    expect(params.get('end')).toBe('2025-01-01T03:00:00.000Z');
  });
});
```

- [ ] **Step 2: Run the `TimelinePage` tests to verify they fail**

Run: `cd ui && npm test -- src/pages/TimelinePage.test.tsx`

Expected: FAIL because `getWatcherEnabled` is not yet used by `TimelinePage`, the empty URL path still renders the startup time-range picker, and the page does not auto-initialize to metadata bounds.

- [ ] **Step 3: Add watcher-disabled startup initialization to `TimelinePage`**

```tsx
// ui/src/pages/TimelinePage.tsx
import React, { useState, useMemo, useEffect, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { FilterBar } from '../components/FilterBar';
import { Timeline } from '../components/Timeline';
import { DetailPanel } from '../components/DetailPanel';
import { TimeRangePicker } from '../components/TimeRangePicker';
import { useTimeline } from '../hooks/useTimeline';
import { useMetadata } from '../hooks/useMetadata';
import { usePersistedFilters } from '../hooks/usePersistedFilters';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { K8sResource, FilterState, SelectedPoint, TimeRange, ResourceStatus } from '../types';
import { useSettings } from '../hooks/useSettings';
import { parseTimeExpression } from '../utils/timeParsing';
import { apiClient, getDemoMode, getWatcherEnabled } from '../services/api';
import { buildWatcherDisabledInitialRange } from './timelineStartup';

function TimelinePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [startupResolved, setStartupResolved] = useState(false);
  const initialTimeRangeRef = useRef<TimeRange | null>(null);
  const [timeRange, setTimeRange] = useState<TimeRange | null>(null);
  const [rawTimeExpressions, setRawTimeExpressions] = useState<{ start?: string; end?: string }>({});
  const originalUrlParamsRef = useRef<{ start?: string; end?: string } | null>(null);
  const isUpdatingFromZoom = useRef(false);

  useEffect(() => {
    let cancelled = false;

    const initializeStartupRange = async () => {
      const startParam = searchParams.get('start');
      const endParam = searchParams.get('end');

      if (startParam && endParam) {
        if (!cancelled) setStartupResolved(true);
        return;
      }

      if (getDemoMode()) {
        setSearchParams({ start: 'now-2h', end: 'now' }, { replace: true });
        if (!cancelled) setStartupResolved(true);
        return;
      }

      if (getWatcherEnabled()) {
        if (!cancelled) setStartupResolved(true);
        return;
      }

      try {
        const metadata = await apiClient.getMetadata();
        const initialRange = buildWatcherDisabledInitialRange(metadata.timeRange);

        if (initialRange && !cancelled) {
          setSearchParams({
            start: initialRange.start.toISOString(),
            end: initialRange.end.toISOString(),
          }, { replace: true });
        }
      } catch (error) {
        console.error('Failed to initialize watcher-disabled startup range:', error);
      } finally {
        if (!cancelled) {
          setStartupResolved(true);
        }
      }
    };

    initializeStartupRange();

    return () => {
      cancelled = true;
    };
  }, [searchParams, setSearchParams]);

  if (!startupResolved) {
    return null;
  }
}
```

- [ ] **Step 4: Run the `TimelinePage` tests and one existing dropdown regression test**

Run: `cd ui && npm test -- src/pages/TimelinePage.test.tsx src/components/TimeRangeDropdown.test.tsx`

Expected:

```text
✓ src/pages/TimelinePage.test.tsx
✓ src/components/TimeRangeDropdown.test.tsx
```

- [ ] **Step 5: Commit the watcher-disabled startup wiring**

```bash
git add ui/src/pages/TimelinePage.tsx ui/src/pages/TimelinePage.test.tsx
git commit -m "feat: auto-initialize timeline for watcher-disabled mode"
```

### Task 5: Add An End-To-End Regression For Imported CI Data

**Files:**
- Modify: `tests/e2e/import_export_stage_test.go`
- Modify: `tests/e2e/import_export_test.go`

- [ ] **Step 1: Write the failing watcher-disabled CLI import UI test**

```go
func TestCLIImportOnStartupWatcherDisabledUsesFullTimelineRange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	given, when, then := NewImportExportStage(t)

	given.a_test_cluster().and().
		generated_test_events_stored_in_configmap()

	when.spectre_is_deployed_with_import_on_startup_and_watcher_disabled().and().
		wait_for_spectre_to_become_ready().and().
		port_forward_to_spectre().and().
		browser_is_initialized()

	then.root_page_initializes_to_imported_metadata_bounds().and().
		startup_time_range_picker_is_not_shown().and().
		navbar_time_controls_are_still_visible()
}
```

- [ ] **Step 2: Run the new e2e test to verify it fails**

Run: `go test ./tests/e2e -run TestCLIImportOnStartupWatcherDisabledUsesFullTimelineRange -count=1 -timeout 20m`

Expected: FAIL because the stage methods do not exist yet and the root page does not rewrite the URL to the imported metadata bounds.

- [ ] **Step 3: Add watcher-disabled deployment and browser verification helpers**

```go
// tests/e2e/import_export_stage_test.go
type ImportExportStage struct {
	t         *testing.T
	require   *require.Assertions
	assert    *assert.Assertions
	testCtx   *helpers.TestContext
	k8sClient *helpers.K8sClient
	apiClient *helpers.APIClient
	browserTest *helpers.BrowserTest

	testNamespaces  []string
	deploymentNames map[string][]string
	events          []*models.Event
	baseTime        time.Time

	exportPath      string
	exportTimestamp int64
	helmDeployer    *helpers.HelmDeployer

	resourceID       string
	timelineResource *helpers.Resource
	kubernetesEvents []*models.Event
	involvedPodUIDs  []string

	spectreNamespace string
	testCluster      *helpers.TestCluster
	configMapName    string
}

func (s *ImportExportStage) browser_is_initialized() *ImportExportStage {
	bt, err := helpers.NewBrowserTest(s.t)
	s.require.NoError(err, "failed to create browser test")
	s.browserTest = bt
	s.t.Cleanup(func() {
		if err := bt.Close(); err != nil {
			s.t.Logf("Warning: failed to close browser: %v", err)
		}
	})
	return s
}

func (s *ImportExportStage) spectre_is_deployed_with_import_on_startup_and_watcher_disabled() *ImportExportStage {
	values, imageRef, err := helpers.LoadHelmValues()
	s.require.NoError(err, "failed to load Helm values")

	err = helpers.BuildAndLoadTestImage(s.t, s.testCluster.Name, imageRef)
	s.require.NoError(err, "failed to build/load image")

	importMountPath := "/import-data"
	values["extraVolumes"] = []map[string]interface{}{
		{
			"name": "import-data",
			"configMap": map[string]string{
				"name": s.configMapName,
			},
		},
	}
	values["extraVolumeMounts"] = []map[string]interface{}{
		{
			"name":      "import-data",
			"mountPath": importMountPath,
			"readOnly":  true,
		},
	}
	values["extraArgs"] = []string{
		fmt.Sprintf("--import=%s", importMountPath),
		"--watcher-enabled=false",
	}

	helmDeployer, err := helpers.NewHelmDeployer(s.t, s.testCluster.GetKubeConfig(), s.spectreNamespace)
	s.require.NoError(err, "failed to create Helm deployer")

	chartPath, err := helpers.RepoPath("chart")
	s.require.NoError(err, "failed to get chart path")

	err = helmDeployer.InstallOrUpgrade(s.testCluster.Name, chartPath, values)
	s.require.NoError(err, "failed to install Helm release")
	return s
}

func (s *ImportExportStage) root_page_initializes_to_imported_metadata_bounds() *ImportExportStage {
	ctx, cancel := context.WithTimeout(s.t.Context(), 30*time.Second)
	defer cancel()

	metadata, err := s.apiClient.GetMetadata(ctx, nil, nil)
	s.require.NoError(err, "failed to query metadata")

	_, err = s.browserTest.Page.Goto(s.apiClient.BaseURL+"/", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(60000),
	})
	s.require.NoError(err, "failed to navigate to Spectre root page")

	deadline := time.Now().Add(30 * time.Second)
	var currentURL string
	for time.Now().Before(deadline) {
		currentURL = s.browserTest.Page.URL()
		parsed, parseErr := url.Parse(currentURL)
		s.require.NoError(parseErr, "failed to parse current browser URL")
		q := parsed.Query()
		if q.Get("start") != "" && q.Get("end") != "" {
			expectedStart := time.Unix(metadata.TimeRange.Earliest, 0).UTC().Format(time.RFC3339)
			expectedEnd := time.Unix(metadata.TimeRange.Latest, 0).UTC().Format(time.RFC3339)
			s.assert.Equal(expectedStart, q.Get("start"))
			s.assert.Equal(expectedEnd, q.Get("end"))
			return s
		}
		time.Sleep(250 * time.Millisecond)
	}

	s.t.Fatalf("expected root page to populate start/end query params, last URL: %s", currentURL)
	return s
}
```

- [ ] **Step 4: Run the e2e regression test**

Run: `go test ./tests/e2e -run TestCLIImportOnStartupWatcherDisabledUsesFullTimelineRange -count=1 -timeout 20m`

Expected:

```text
ok  	github.com/moolen/spectre/tests/e2e
```

- [ ] **Step 5: Commit the e2e regression coverage**

```bash
git add tests/e2e/import_export_stage_test.go tests/e2e/import_export_test.go
git commit -m "test: cover watcher-disabled imported timeline startup"
```

## Self-Review

- Spec coverage:
  - `/health` exposes `watcherEnabled`: Task 1
  - frontend caches runtime watcher mode: Task 2
  - watcher-disabled auto-range only when URL lacks params: Task 4
  - metadata time bounds reused for full-range initialization: Tasks 3 and 4
  - single-event padding and invalid/no-data handling: Task 3
  - startup chooser is skipped while navbar time controls remain available: Tasks 4 and 5
  - regression coverage for imported CI-style data: Task 5
- Placeholder scan:
  - No `TBD`, `TODO`, or “implement later” markers remain.
  - Each task includes explicit files, code, commands, and commit messages.
- Type consistency:
  - Backend uses `watcherEnabled` consistently in `/health`.
  - Frontend API exposes `getWatcherEnabled()` and `resetRuntimeModeForTests()`.
  - Timeline startup helper uses `buildWatcherDisabledInitialRange()` consistently across pure tests and `TimelinePage`.
