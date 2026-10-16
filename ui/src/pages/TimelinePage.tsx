import React, { useState, useMemo, useEffect, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { FilterBar } from '../components/FilterBar';
import { Timeline } from '../components/Timeline';
import { DetailPanel, DEFAULT_DETAIL_PANEL_WIDTH } from '../components/DetailPanel';
import { useTimeline } from '../hooks/useTimeline';
import { useMetadata } from '../hooks/useMetadata';
import { usePersistedFilters } from '../hooks/usePersistedFilters';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { K8sResource, FilterState, SelectedPoint, TimeRange, ResourceStatus } from '../types';
import { useSettings } from '../hooks/useSettings';
import { apiClient, detectWatcherEnabled } from '../services/api';
import { parseTimeExpression } from '../utils/timeParsing';
import { fromNamespaceFilterValue, sortNamespaceFilterOptions } from '../utils/namespaceFilters';
import { buildWatcherDisabledInitialRange } from './timelineStartup';

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

  useEffect(() => {
    let cancelled = false;

    const initializeStartupRange = async () => {
      const startParam = searchParams.get('start');
      const endParam = searchParams.get('end');

      // Only auto-set if no time range is specified in URL
      if (startParam || endParam) {
        return;
      }

      const defaultStart = 'now-30m';
      const defaultEnd = 'now';

      try {
        const watcherEnabled = await detectWatcherEnabled();
        if (cancelled) {
          return;
        }

        if (!watcherEnabled) {
          const metadata = await apiClient.getMetadata();
          if (cancelled) {
            return;
          }

          const initialRange = buildWatcherDisabledInitialRange(metadata.timeRange);
          if (initialRange) {
            setSearchParams(
              {
                start: initialRange.start.toISOString(),
                end: initialRange.end.toISOString(),
              },
              { replace: true },
            );
            return;
          }
        }
      } catch (error) {
        console.error('Failed to initialize startup time range:', error);
      }

      if (!cancelled) {
        setSearchParams(
          {
            start: defaultStart,
            end: defaultEnd,
          },
          { replace: true },
        );
      }
    };

    initializeStartupRange();

    return () => {
      cancelled = true;
    };
  }, [searchParams, setSearchParams]);

  // Parse the time range from the URL
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
  const sortedAvailableNamespaces = useMemo(
    () => sortNamespaceFilterOptions(availableNamespaces),
    [availableNamespaces]
  );

  // Get default kinds from settings
  const { defaultKinds } = useSettings();

  // Load persisted filters from localStorage
  const { kinds: persistedKinds, namespaces: persistedNamespaces, setKinds, setNamespaces } =
    usePersistedFilters(availableKinds, availableNamespaces, defaultKinds);

  // Selection State
  const [selectedPoint, setSelectedPoint] = useState<SelectedPoint | null>(null);

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

  // Pass filter arrays to API for server-side filtering with pagination
  const apiFilters = useMemo(() => ({
    namespaces: filters.namespaces.map(fromNamespaceFilterValue),
    kinds: filters.kinds,
  }), [filters.namespaces, filters.kinds]);

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

  // Fetch timeline data from backend API with pagination support
  const { resources, loading, loadingMore, error, totalCount, loadedCount, hasMore, loadMore } = useTimeline({
    startTime: timeRange?.start,
    endTime: timeRange?.end,
    rawStart: rawTimeExpressions.start,
    rawEnd: rawTimeExpressions.end,
    filters: apiFilters,
    pageSize: 100, // Load 100 resources per page
    refreshToken
  });

  // Track when we're loading all resources for the problematic filter
  const [loadingAllForProblematic, setLoadingAllForProblematic] = useState(false);

  // Show loading indicator when "Problematic Only" filter is enabled
  // Keep it visible during initial load, loading more, or while there are more resources to load
  useEffect(() => {
    if (hasProblematicStatus && timeRange) {
      // Show loading indicator if:
      // 1. Initial load is in progress
      // 2. Loading more pages
      // 3. There are more resources to load
      if (loading || loadingMore || hasMore) {
        setLoadingAllForProblematic(true);
      } else {
        // All resources loaded, hide loading indicator
        setLoadingAllForProblematic(false);
      }
    } else if (!hasProblematicStatus) {
      // Filter disabled, hide loading indicator
      setLoadingAllForProblematic(false);
    }
  }, [hasProblematicStatus, timeRange, loading, loadingMore, hasMore]);

  // When "Problematic Only" filter is enabled, automatically load all remaining resources
  // This ensures we have all data available for client-side filtering
  useEffect(() => {
    if (hasProblematicStatus && hasMore && !loading && !loadingMore && timeRange) {
      // Load more resources when problematic filter is enabled and more are available
      loadMore();
    }
  }, [hasProblematicStatus, hasMore, loading, loadingMore, loadMore, timeRange]);

  // Filter Logic - now only for client-side filters (search and problematic status)
  // Kinds and namespaces are filtered server-side via API
  const filteredResources = useMemo(() => {
    return resources.filter(r => {
      const matchesSearch = r.name.toLowerCase().includes(filters.search.toLowerCase());
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

      return matchesSearch && matchesStatus;
    });
  }, [resources, filters.search, filters.hasProblematicStatus, timeRange]);

  // Counts for the status bar, based on each resource's latest status
  const statusSummary = useMemo(() => {
    let failing = 0;
    let degraded = 0;
    let events = 0;
    for (const r of filteredResources) {
      const latest = r.statusSegments[r.statusSegments.length - 1]?.status;
      if (latest === ResourceStatus.Error) failing++;
      else if (latest === ResourceStatus.Warning) degraded++;
      events += r.events.length;
    }
    return { failing, degraded, events };
  }, [filteredResources]);

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

  // Show loading state if time range hasn't been set yet (waiting for auto-set)
  if (!timeRange) {
    return (
      <div className="flex flex-col h-screen bg-[var(--color-app-bg)] text-[var(--color-text-primary)] overflow-hidden items-center justify-center gap-3">
        <Spinner />
        <p className="text-[12.5px] text-[var(--color-text-faint)]">Loading timeline...</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-screen bg-[var(--color-app-bg)] text-[var(--color-text-primary)] overflow-hidden">
      <FilterBar
        filters={filters}
        setFilters={setFilters}
        timeRange={displayTimeRange}
        onTimeRangeChange={handleTimeRangeChange}
        availableNamespaces={sortedAvailableNamespaces}
        availableKinds={availableKinds}
        rawStart={rawTimeExpressions.start}
        rawEnd={rawTimeExpressions.end}
      />

      <div className="flex-1 flex overflow-hidden relative">
        <div className="flex-1 overflow-hidden flex flex-col relative">
          {/* Loading indicator overlay when loading all resources for problematic filter */}
          {loadingAllForProblematic && (
            <div className="absolute inset-0 bg-[var(--color-app-bg)]/80 backdrop-blur-sm flex items-center justify-center z-50">
              <div className="flex flex-col items-center gap-3 text-[var(--color-text-primary)]">
                <Spinner />
                <p className="text-[13.5px] font-medium">Loading all resources...</p>
                <p className="text-xs text-[var(--color-text-faint)]">
                  Fetching remaining resources for problematic filter
                </p>
                <div className="w-56 h-1 bg-[var(--color-border-strong)] rounded-full overflow-hidden mt-1">
                  <div
                    className={`h-full bg-[var(--color-accent)] transition-all duration-300 ${hasMore ? 'animate-pulse' : ''}`}
                    style={{ width: hasMore ? '90%' : '100%' }}
                  />
                </div>
                <p className="text-xs text-[var(--color-text-faint)] tabular-nums">{loadedCount} resources loaded</p>
              </div>
            </div>
          )}
          {error ? (
            <div className="flex-1 flex items-center justify-center flex-col gap-3 text-center">
              <div className="w-10 h-10 rounded-full grid place-items-center bg-[var(--color-status-err-soft)] text-[var(--color-status-err)]">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24" strokeLinecap="round">
                  <circle cx="12" cy="12" r="9" /><path d="M12 8v4M12 16h.01" />
                </svg>
              </div>
              <p className="text-[13.5px] font-medium">Failed to load resources</p>
              <p className="text-xs text-[var(--color-text-faint)] max-w-sm">{error.message}</p>
              <button onClick={() => window.location.reload()} className="ui-control mt-1">
                Retry
              </button>
            </div>
          ) : loading ? (
            <div className="flex-1 flex items-center justify-center flex-col gap-3 text-[var(--color-text-faint)]">
              <Spinner />
              <p className="text-[12.5px]">Loading resources...</p>
              {totalCount > 0 && (
                <div className="flex flex-col items-center gap-2">
                  <div className="w-56 h-1 bg-[var(--color-border-strong)] rounded-full overflow-hidden">
                    <div
                      className="h-full bg-[var(--color-accent)] transition-all duration-300"
                      style={{ width: `${(loadedCount / totalCount) * 100}%` }}
                    />
                  </div>
                  <p className="text-xs tabular-nums">{loadedCount} / {totalCount} resources</p>
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
                sidebarWidth={selectedPoint ? DEFAULT_DETAIL_PANEL_WIDTH : 0}
                timeRange={timeRange}
                onVisibleTimeRangeChange={handleVisibleTimeRangeChange}
                onZoomDetected={handleZoomDetected}
                hasMore={hasMore}
                loadingMore={loadingMore}
                onLoadMore={loadMore}
              />
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center flex-col gap-3 text-[var(--color-text-faint)]">
              <div className="w-10 h-10 rounded-full grid place-items-center bg-[var(--color-surface-muted)]">
                <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24" strokeLinecap="round">
                  <path d="M3 5h18M6 12h12M10 19h4" />
                </svg>
              </div>
              <p className="text-[12.5px]">No resources match your filters.</p>
            </div>
          )}
        </div>

        <DetailPanel
          resource={selectedResource}
          selectedIndex={selectedPoint?.index}
          onClose={handleClosePanel}
        />
      </div>

      <footer className="ui-statusbar">
        <span className="tabular-nums">{filteredResources.length}{hasMore ? '+' : ''} resources</span>
        <span className="tabular-nums"><i className="dot" style={{ background: 'var(--color-status-err)' }} />{statusSummary.failing} failing</span>
        <span className="tabular-nums"><i className="dot" style={{ background: 'var(--color-status-warn)' }} />{statusSummary.degraded} degraded</span>
        <span className="tabular-nums">{statusSummary.events} events</span>
        <span className="ml-auto">
          {autoRefresh !== 'off' ? `Auto-refresh every ${autoRefresh}` : 'Auto-refresh off'}
        </span>
      </footer>
    </div>
  );
}

function Spinner() {
  return (
    <div className="w-6 h-6 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
  );
}

export default TimelinePage;
