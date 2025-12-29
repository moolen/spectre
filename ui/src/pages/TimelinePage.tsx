import React, { useState, useMemo, useEffect, useCallback, useRef, startTransition } from 'react';
import { useSearchParams } from 'react-router-dom';
import { FilterBar } from '../components/FilterBar';
import { Timeline } from '../components/Timeline';
import { DetailPanel } from '../components/DetailPanel';
import { RootCauseView } from '../components/RootCauseView';
import { TimeRangePicker } from '../components/TimeRangePicker';
import { useTimeline } from '../hooks/useTimeline';
import { useMetadata } from '../hooks/useMetadata';
import { usePersistedFilters } from '../hooks/usePersistedFilters';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { K8sResource, FilterState, SelectedPoint, TimeRange, ResourceStatus } from '../types';
import { RootCauseAnalysisV2 } from '../types/rootCause';
import { useSettings } from '../hooks/useSettings';
import { parseTimeExpression } from '../utils/timeParsing';
import { getDemoMode } from '../services/api';
import { fetchRootCauseAnalysis } from '../services/rootCauseService';
import { toast } from '../utils/toast';

const AUTO_REFRESH_INTERVALS: Record<string, number> = {
  off: 0,
  '30s': 30_000,
  '60s': 60_000,
  '300s': 300_000
};

function TimelinePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialTimeRangeRef = useRef<TimeRange | null>(null);
  const [timeRange, setTimeRange] = useState<TimeRange | null>(null);
  const [rawTimeExpressions, setRawTimeExpressions] = useState<{ start?: string; end?: string }>({});
  const originalUrlParamsRef = useRef<{ start?: string; end?: string } | null>(null);
  const isUpdatingFromZoom = useRef(false);

  // Auto-set time range in demo mode if no URL params
  useEffect(() => {
    const startParam = searchParams.get('start');
    const endParam = searchParams.get('end');

    console.log('[Demo Mode Check]', {
      hasStart: !!startParam,
      hasEnd: !!endParam,
      isDemoMode: getDemoMode(),
      shouldAutoSet: !startParam && !endParam && getDemoMode()
    });

    // Only auto-set if we're in demo mode and no time range is set
    if (!startParam && !endParam && getDemoMode()) {
      console.log('[Demo Mode] Auto-setting time range to now-2h -> now');
      // Set default time range to "now-2h" to "now" (last 2 hours)
      const endExpr = 'now';
      const startExpr = 'now-2h';

      setSearchParams({
        start: startExpr,
        end: endExpr
      }, { replace: true }); // Use replace to avoid adding to history
    }
  }, [searchParams, setSearchParams]);

  // Create a display time range that reads from URL for FilterBar
  // This allows FilterBar to update without triggering Timeline re-renders
  const displayTimeRange = useMemo(() => {
    const startParam = searchParams.get('start');
    const endParam = searchParams.get('end');
    if (!startParam || !endParam) return timeRange;

    const start = new Date(startParam);
    const end = new Date(endParam);
    if (isNaN(start.getTime()) || isNaN(end.getTime())) return timeRange;

    return { start, end };
  }, [searchParams, timeRange]);

  // Parse time range from URL
  useEffect(() => {
    // Skip processing if this update came from a zoom action (to prevent render loops)
    if (isUpdatingFromZoom.current) {
      isUpdatingFromZoom.current = false;
      return;
    }

    const startParam = searchParams.get('start');
    const endParam = searchParams.get('end');

    if (!startParam || !endParam) {
      setTimeRange(null);
      return;
    }

    // Store original URL parameters to preserve format
    originalUrlParamsRef.current = { start: startParam, end: endParam };

    // Try parsing as human-friendly expressions first, then fall back to ISO dates
    let start = parseTimeExpression(startParam);
    let end = parseTimeExpression(endParam);

    // If human-friendly parsing failed, try ISO date parsing
    if (!start) {
      const isoStart = new Date(startParam);
      if (!isNaN(isoStart.getTime())) {
        start = isoStart;
      }
    }

    if (!end) {
      const isoEnd = new Date(endParam);
      if (!isNaN(isoEnd.getTime())) {
        end = isoEnd;
      }
    }

    // Validate that we have valid dates and start < end
    if (!start || !end || isNaN(start.getTime()) || isNaN(end.getTime()) || start >= end) {
      setTimeRange(null);
      setRawTimeExpressions({});
      originalUrlParamsRef.current = null;
      return;
    }

    const range = { start, end };
    initialTimeRangeRef.current = range;
    setTimeRange(range);

    // Store raw expressions if they were successfully parsed as human-friendly
    // Check by seeing if parseTimeExpression returns a non-null value
    const parsedStart = parseTimeExpression(startParam);
    const parsedEnd = parseTimeExpression(endParam);
    setRawTimeExpressions({
      start: parsedStart !== null ? startParam : undefined,
      end: parsedEnd !== null ? endParam : undefined,
    });
  }, [searchParams]); // Re-run when URL params change

  // Update URL when time range changes (from time picker)
  const handleTimeRangeChange = (range: TimeRange, rawStart?: string, rawEnd?: string) => {
    // If raw expressions are provided (human-friendly), use them in URL
    // Otherwise, use ISO date strings
    const startParam = rawStart || range.start.toISOString();
    const endParam = rawEnd || range.end.toISOString();

    setSearchParams({
      start: startParam,
      end: endParam
    });
    // Also update the local state and initial ref
    initialTimeRangeRef.current = range;
    setTimeRange(range);
    // Store raw expressions for API calls
    setRawTimeExpressions({ start: rawStart, end: rawEnd });
  };

  // Handle visible range changes from zoom/pan
  // Update URL with the new visible range (as ISO timestamps)
  const handleVisibleTimeRangeChange = (range: TimeRange) => {
    // Check if this is a real zoom change or just a precision noise update
    // If current timeRange exists, compare the received range with what we'd expect
    // (current range + 2% padding on each side)
    if (timeRange) {
      const currentDuration = timeRange.end.getTime() - timeRange.start.getTime();
      const expectedPadding = currentDuration * 0.02;
      const expectedDomain = {
        start: new Date(timeRange.start.getTime() - expectedPadding),
        end: new Date(timeRange.end.getTime() + expectedPadding)
      };

      // Allow 2 second tolerance for floating-point precision
      const startDiff = Math.abs(range.start.getTime() - expectedDomain.start.getTime());
      const endDiff = Math.abs(range.end.getTime() - expectedDomain.end.getTime());
      const tolerance = 2000; // 2 seconds

      if (startDiff < tolerance && endDiff < tolerance) {
        return; // Don't update - this is just precision noise
      }
    }

    // The range received from Timeline INCLUDES the 2% padding that Timeline adds to timeDomain.
    // We need to remove this padding before updating the state/URL, otherwise when Timeline
    // recalculates timeDomain it will add another 2% padding on top, causing the zoom-out loop.
    const duration = range.end.getTime() - range.start.getTime();
    const paddingMs = duration * 0.02; // Timeline adds 2% padding
    const unpaddedRange = {
      start: new Date(range.start.getTime() + paddingMs),
      end: new Date(range.end.getTime() - paddingMs)
    };

    // Set flag to prevent the URL effect from processing this change
    isUpdatingFromZoom.current = true;

    // Update URL immediately (for browser history and sharing)
    setSearchParams({
      start: unpaddedRange.start.toISOString(),
      end: unpaddedRange.end.toISOString()
    });

    // DON'T update timeRange state - this would trigger Timeline re-render and cause flash
    // The URL is the source of truth, and FilterBar will display it via displayTimeRange

    // Clear raw expressions so FilterBar displays the ISO timestamps from URL
    setRawTimeExpressions({});
  };

  // Handle zoom detection - clear the persisted preset when user zooms/pans
  const { clearPreset } = usePersistedQuickPreset();
  const handleZoomDetected = useCallback(() => {
    clearPreset();
  }, [clearPreset]);

  // Fetch metadata (namespaces and kinds) for the time range
  const { namespaces: availableNamespaces, kinds: availableKinds } = useMetadata(timeRange);

  // Load persisted filters from localStorage
  const { kinds: persistedKinds, namespaces: persistedNamespaces, setKinds, setNamespaces } =
    usePersistedFilters(availableKinds, availableNamespaces);

  // Selection State
  const [selectedPoint, setSelectedPoint] = useState<SelectedPoint | null>(null);

  // Root Cause State
  const [showRootCause, setShowRootCause] = useState(false);
  const [rootCauseAnalysis, setRootCauseAnalysis] = useState<RootCauseAnalysisV2 | null>(null);
  const [rootCauseLoading, setRootCauseLoading] = useState(false);
  const [rootCauseError, setRootCauseError] = useState<string | null>(null);
  const [rootCauseParams, setRootCauseParams] = useState<{
    uid: string;
    timestamp: Date;
    lookbackMs: number;
  } | null>(null);

  // Debug: Log state changes
  useEffect(() => {
    console.log('[RootCause] State changed:', {
      showRootCause,
      hasAnalysis: !!rootCauseAnalysis,
      analysisChainLength: rootCauseAnalysis?.incident?.causalChain?.length,
    });
  }, [showRootCause, rootCauseAnalysis]);

  // Deep link support - check for rootCause URL params on mount
  const deepLinkLoadedRef = useRef(false);
  useEffect(() => {
    // Only run once on mount, or if URL params change and we haven't loaded yet
    const rcResourceUID = searchParams.get('rcResourceUID');
    const rcTimestamp = searchParams.get('rcTimestamp');

    // Skip if we've already loaded from deep link or if we're already showing root cause
    if (deepLinkLoadedRef.current || showRootCause || !rcResourceUID || !rcTimestamp) {
      return;
    }

    console.log('[RootCause] Deep link detected:', { rcResourceUID, rcTimestamp });
    deepLinkLoadedRef.current = true;

    const fetchFromDeepLink = async () => {
      setRootCauseLoading(true);
      try {
        const timestamp = new Date(parseInt(rcTimestamp));
        const defaultLookbackMs = 600000; // 10 minutes
        const analysis = await fetchRootCauseAnalysis(rcResourceUID, timestamp, { lookbackMs: defaultLookbackMs });
        setRootCauseAnalysis(analysis);
        setRootCauseParams({
          uid: rcResourceUID,
          timestamp,
          lookbackMs: defaultLookbackMs,
        });
        setShowRootCause(true);
      } catch (err) {
        console.error('[RootCause] Failed to fetch from deep link:', err);
        deepLinkLoadedRef.current = false; // Allow retry on error
      } finally {
        setRootCauseLoading(false);
      }
    };

    fetchFromDeepLink();
  }, [searchParams, showRootCause]); // Re-check if searchParams change but only if not already showing

  // Filters - search and hasProblematicStatus are component state only (reset on reload)
  // kinds and namespaces come from persisted filters
  const [search, setSearch] = useState<string>('');
  const [hasProblematicStatus, setHasProblematicStatus] = useState<boolean>(false);

  const filters: FilterState = useMemo(() => ({
    kinds: persistedKinds,
    namespaces: persistedNamespaces,
    search,
    hasProblematicStatus
  }), [persistedKinds, persistedNamespaces, search, hasProblematicStatus]);

  const setFilters = useCallback((updater: React.SetStateAction<FilterState>) => {
    if (typeof updater === 'function') {
      // Use current values directly to avoid stale closure
      const currentFilters: FilterState = {
        kinds: persistedKinds,
        namespaces: persistedNamespaces,
        search,
        hasProblematicStatus
      };
      const newFilters = updater(currentFilters);
      setSearch(newFilters.search);
      setKinds(newFilters.kinds);
      setNamespaces(newFilters.namespaces);
      if (newFilters.hasProblematicStatus !== undefined) {
        setHasProblematicStatus(newFilters.hasProblematicStatus);
      }
    } else {
      setSearch(updater.search);
      setKinds(updater.kinds);
      setNamespaces(updater.namespaces);
      if (updater.hasProblematicStatus !== undefined) {
        setHasProblematicStatus(updater.hasProblematicStatus);
      }
    }
  }, [persistedKinds, persistedNamespaces, search, hasProblematicStatus, setKinds, setNamespaces]);

  // Don't filter at the API level - filter client-side instead
  // This allows selecting multiple kinds/namespaces
  const apiFilters = undefined;

  const [refreshToken, setRefreshToken] = useState(0);
  const { autoRefresh } = useSettings();

  useEffect(() => {
    if (!timeRange) return;
    const intervalMs = AUTO_REFRESH_INTERVALS[autoRefresh] || 0;
    if (intervalMs === 0) return;

    const id = window.setInterval(() => {
      setRefreshToken((prev) => prev + 1);
    }, intervalMs);

    return () => window.clearInterval(id);
  }, [autoRefresh, timeRange]);

  // Fetch timeline data from backend API
  const { resources, loading, error, totalCount, loadedCount } = useTimeline({
    startTime: timeRange?.start,
    endTime: timeRange?.end,
    rawStart: rawTimeExpressions.start,
    rawEnd: rawTimeExpressions.end,
    filters: apiFilters,
    refreshToken
  });

  // Filter Logic
  const filteredResources = useMemo(() => {
    return resources.filter(r => {
      const matchesSearch = r.name.toLowerCase().includes(filters.search.toLowerCase());
      const matchesNs = filters.namespaces.length === 0 || filters.namespaces.includes(r.namespace);
      const matchesKind = filters.kinds.length === 0 || filters.kinds.includes(r.kind);
      const matchesStatus = !filters.hasProblematicStatus ||
        r.statusSegments.some(s => s.status !== ResourceStatus.Ready);
      
      // Exclude resources that were deleted before the current time window
      // A resource is excluded if it has a deletedAt timestamp before the window start
      if (timeRange && r.deletedAt) {
        const deletedBeforeWindow = r.deletedAt < timeRange.start;

        if (deletedBeforeWindow) {
          return false;
        }
      }
      
      return matchesSearch && matchesNs && matchesKind && matchesStatus;
    });
  }, [resources, filters, timeRange]);

  // Derived selected resource object
  const selectedResource = useMemo(() => {
    if (!selectedPoint) return null;
    return resources.find(r => r.id === selectedPoint.resourceId) || null;
  }, [selectedPoint, resources]);

  const relevantEventIds = useMemo(() => {
    if (!selectedResource || !selectedPoint) return [];
    const segment = selectedResource.statusSegments[selectedPoint.index];
    if (!segment) return [];
    return selectedResource.events
      .filter(e => e.timestamp >= segment.start && e.timestamp <= segment.end)
      .map(e => e.id);
  }, [selectedResource, selectedPoint]);

  const handleSegmentClick = async (resource: K8sResource, index: number) => {
    setSelectedPoint({ resourceId: resource.id, index });
  };

  const handleClosePanel = () => {
    setSelectedPoint(null);
  };

  const handleAnalyzeRootCause = async () => {
    if (!selectedResource || selectedPoint === null) return;

    const segment = selectedResource.statusSegments[selectedPoint.index];
    if (!segment) return;

    console.log('[RootCause] Starting analysis for:', {
      resourceId: selectedResource.id,
      resourceName: selectedResource.name,
      segmentStart: segment.start,
      segmentEnd: segment.end,
    });

    setRootCauseLoading(true);
    // Prevent deep link effect from interfering
    deepLinkLoadedRef.current = true;

    const defaultLookbackMs = 600000; // 10 minutes

    try {
      console.log('[RootCause] Calling fetchRootCauseAnalysis...');
      const analysis = await fetchRootCauseAnalysis(selectedResource.id, segment.start, { lookbackMs: defaultLookbackMs });
      console.log('[RootCause] Analysis received:', analysis);

      // Set both states together
      setRootCauseAnalysis(analysis);
      setRootCauseParams({
        uid: selectedResource.id,
        timestamp: segment.start,
        lookbackMs: defaultLookbackMs,
      });
      setShowRootCause(true);
      setSelectedPoint(null); // Close detail panel

      console.log('[RootCause] State updated, adding URL params...');

      // Add root cause params to URL for deep linking
      const currentParams = Object.fromEntries(searchParams.entries());
      setSearchParams({
        ...currentParams,
        rcResourceUID: selectedResource.id,
        rcTimestamp: segment.start.getTime().toString(),
      }, { replace: true }); // Use replace to avoid triggering unnecessary navigation

      console.log('[RootCause] URL params added');
    } catch (err) {
      console.error('[RootCause] Failed to fetch:', err);
      deepLinkLoadedRef.current = false; // Allow retry on error
      toast.apiError(err, 'Failed to load root cause analysis');
    } finally {
      setRootCauseLoading(false);
      console.log('[RootCause] Loading state cleared');
    }
  };

  const handleRootCauseRefresh = async (
    uid: string,
    timestamp: Date,
    lookbackMs: number
  ) => {
    setRootCauseLoading(true);
    setRootCauseError(null); // Clear previous errors
    try {
      const analysis = await fetchRootCauseAnalysis(uid, timestamp, { lookbackMs });
      setRootCauseAnalysis(analysis);
      setRootCauseParams({ uid, timestamp, lookbackMs });
    } catch (err) {
      console.error('[RootCause] Failed to refresh:', err);
      const errorMessage = err instanceof Error ? err.message : 'Failed to fetch root cause analysis';
      setRootCauseError(errorMessage);
      toast.apiError(err, 'Failed to refresh root cause analysis');
    } finally {
      setRootCauseLoading(false);
    }
  };

  const handleCloseRootCause = () => {
    setShowRootCause(false);
    setRootCauseAnalysis(null);
    setRootCauseParams(null);
    setRootCauseError(null);
    deepLinkLoadedRef.current = false; // Reset so deep link can work again if needed

    // Remove root cause params from URL
    const currentParams = Object.fromEntries(searchParams.entries());
    const { rcResourceUID, rcTimestamp, ...remainingParams } = currentParams;
    setSearchParams(remainingParams);
  };

  // Keyboard Navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!selectedPoint || !selectedResource) return;

      if (e.key === 'ArrowLeft') {
        e.preventDefault();
        // Move to previous segment
        if (selectedPoint.index > 0) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index - 1 });
        }
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        // Move to next segment
        if (selectedPoint.index < selectedResource.statusSegments.length - 1) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index + 1 });
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedPoint, selectedResource]);

  // Show time range picker if no range is selected
  if (!timeRange) {
    return (
      <TimeRangePicker onConfirm={handleTimeRangeChange} />
    );
  }

  return (
    <div className="flex flex-col h-screen bg-[var(--color-app-bg)] text-[var(--color-text-primary)] overflow-hidden font-sans transition-colors duration-300">
      <FilterBar
        filters={filters}
        setFilters={setFilters}
        timeRange={displayTimeRange}
        onTimeRangeChange={handleTimeRangeChange}
        availableNamespaces={availableNamespaces}
        availableKinds={availableKinds}
        rawStart={rawTimeExpressions.start}
        rawEnd={rawTimeExpressions.end}
      />

      <div className="flex-1 flex overflow-hidden relative">
        <div className="flex-1 overflow-hidden flex flex-col">
          {error ? (
            <div className="flex-1 flex items-center justify-center text-red-400 flex-col gap-4">
              <svg className="w-16 h-16 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M12 9v2m0 4v2m0 0v2m0-6V9m0 0V7m0 2V5m0 0h-2m2 0h2m-6 0h2m-2 0h-2m0 6h2m-2 0h-2m0 0v2m0-2v-2m0 0h2m-2 0h-2m6 0h2m-2 0h-2" />
              </svg>
              <div className="text-center">
                <p className="text-lg font-semibold mb-2">Failed to load resources</p>
                <p className="text-sm text-gray-400 mb-4">{error.message}</p>
                <button
                  onClick={() => window.location.reload()}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm transition-colors"
                >
                  Retry
                </button>
              </div>
            </div>
          ) : loading ? (
            <div className="flex-1 flex items-center justify-center text-gray-400 flex-col gap-4">
              <div className="animate-spin">
                <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" />
                </svg>
              </div>
              <p>Loading resources...</p>
              {totalCount > 0 && (
                <div className="flex flex-col items-center gap-2">
                  <div className="w-64 h-2 bg-gray-700 rounded-full overflow-hidden">
                    <div
                      className="h-full bg-blue-500 transition-all duration-300"
                      style={{ width: `${(loadedCount / totalCount) * 100}%` }}
                    />
                  </div>
                  <p className="text-sm text-gray-500">{loadedCount} / {totalCount} resources</p>
                </div>
              )}
            </div>
          ) : filteredResources.length > 0 ? (
            <>
              <Timeline
                resources={filteredResources}
                onSegmentClick={handleSegmentClick}
                selectedPoint={selectedPoint}
                highlightedEventIds={relevantEventIds}
                sidebarWidth={selectedPoint ? 384 : 0}
                timeRange={timeRange}
                onVisibleTimeRangeChange={handleVisibleTimeRangeChange}
                onZoomDetected={handleZoomDetected}
              />
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center text-gray-500 flex-col">
                <svg className="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"></path></svg>
                <p>No resources match your filters.</p>
            </div>
          )}
        </div>

        {/* Detail Panel - Only visible when not in root cause view */}
        {!showRootCause && (
          <DetailPanel
            resource={selectedResource}
            selectedIndex={selectedPoint?.index}
            onClose={handleClosePanel}
            onAnalyzeRootCause={handleAnalyzeRootCause}
          />
        )}
      </div>

      {/* Root Cause View - Full screen overlay (outside main container) */}
      {showRootCause && rootCauseAnalysis && rootCauseParams ? (
        <>
          {console.log('[TimelinePage] Rendering RootCauseView - condition met')}
          <RootCauseView
            analysis={rootCauseAnalysis}
            initialResourceUID={rootCauseParams.uid}
            initialTimestamp={rootCauseParams.timestamp}
            initialLookbackMs={rootCauseParams.lookbackMs}
            onClose={handleCloseRootCause}
            onRefresh={handleRootCauseRefresh}
            error={rootCauseError}
          />
        </>
      ) : (
        showRootCause && console.warn('[TimelinePage] RootCauseView NOT rendering - missing analysis or params', { showRootCause, hasAnalysis: !!rootCauseAnalysis, hasParams: !!rootCauseParams })
      )}
    </div>
  );
}

export default TimelinePage;

