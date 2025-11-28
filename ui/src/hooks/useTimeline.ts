import { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { K8sResource } from '../types';
import { apiClient } from '../services/api';

interface UseTimelineResult {
  resources: K8sResource[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

interface UseTimelineOptions {
  startTime?: Date;
  endTime?: Date;
  filters?: {
    namespace?: string;
    kind?: string;
    group?: string;
    version?: string;
  };
  refreshToken?: number;
}

/**
 * Hook to fetch and manage timeline data from the backend API.
 *
 * @param options - Time range (startTime, endTime) and optional filters for resource selection
 * @returns Object containing resources, loading state, error state, and refresh function
 *
 * Features:
 * - Automatically fetches data on mount and when time range or filters change
 * - Requires startTime and endTime to be provided
 * - Provides refresh callback for manual data reloads
 * - Handles errors gracefully with logging
 * - Only re-fetches when filters actually change (namespace or kind)
 */
export const useTimeline = (options?: UseTimelineOptions): UseTimelineResult => {
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);

  // Serialize filters for stable dependency tracking
  const filtersKey = useMemo(() => {
    if (!options?.filters) return '';
    return JSON.stringify({
      namespace: options.filters.namespace || null,
      kind: options.filters.kind || null,
      group: options.filters.group || null,
      version: options.filters.version || null,
    });
  }, [options?.filters?.namespace, options?.filters?.kind, options?.filters?.group, options?.filters?.version]);

  const startTimeMs = options?.startTime?.getTime() ?? null;
  const endTimeMs = options?.endTime?.getTime() ?? null;

  // Track previous values to detect actual changes
  const prevFiltersRef = useRef<string>('');
  const prevStartTimeRef = useRef<number | null>(null);
  const prevEndTimeRef = useRef<number | null>(null);

  const refreshToken = options?.refreshToken ?? 0;

  const prevRefreshTokenRef = useRef<number>(refreshToken);

  const fetchData = useCallback(async (force = false) => {
    if (!options?.startTime || !options?.endTime) {
      setLoading(false);
      return;
    }

    // Only fetch if time range or filters actually changed
    const timeRangeChanged =
      prevStartTimeRef.current !== startTimeMs ||
      prevEndTimeRef.current !== endTimeMs;
    const filtersChanged = prevFiltersRef.current !== filtersKey;
    const refreshChanged = prevRefreshTokenRef.current !== refreshToken;

    if (!force && !timeRangeChanged && !filtersChanged && !refreshChanged) {
      return; // No changes, skip fetch
    }

    // Update refs
    prevStartTimeRef.current = startTimeMs;
    prevEndTimeRef.current = endTimeMs;
    prevFiltersRef.current = filtersKey;
    prevRefreshTokenRef.current = refreshToken;

    try {
      setLoading(true);
      setError(null);

      // Fetch timeline data from backend API using provided time range
      // This endpoint returns full resource data with statusSegments and events
      const data = await apiClient.getTimeline(
        startTimeMs!,
        endTimeMs!,
        options?.filters
      );

      setResources(data);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to fetch timeline data';
      setError(new Error(errorMessage));
      console.error('Timeline fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, [startTimeMs, endTimeMs, filtersKey, options?.filters, refreshToken]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return {
    resources,
    loading,
    error,
    refresh: () => fetchData(true)
  };
};
