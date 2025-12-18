import { useState, useEffect, useCallback, useRef, useMemo, startTransition } from 'react';
import { K8sResource } from '../types';
import { apiClient } from '../services/api';

interface UseTimelineResult {
  resources: K8sResource[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
  totalCount: number;
  loadedCount: number;
}

interface UseTimelineOptions {
  startTime?: Date;
  endTime?: Date;
  rawStart?: string;
  rawEnd?: string;
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
  const [totalCount, setTotalCount] = useState<number>(0);
  const [loadedCount, setLoadedCount] = useState<number>(0);
  const abortControllerRef = useRef<AbortController | null>(null);
  const accumulatedResourcesRef = useRef<K8sResource[]>([]);
  const batchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

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

    // Update refs BEFORE fetching to prevent duplicate calls
    prevStartTimeRef.current = startTimeMs;
    prevEndTimeRef.current = endTimeMs;
    prevFiltersRef.current = filtersKey;
    prevRefreshTokenRef.current = refreshToken;

    // Cancel any previous request and pending batches
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    if (batchTimeoutRef.current) {
      clearTimeout(batchTimeoutRef.current);
      batchTimeoutRef.current = null;
    }
    abortControllerRef.current = new AbortController();
    accumulatedResourcesRef.current = [];
    let isFirstBatch = true;

    // Flush batch function - applies accumulated resources to state
    const flushBatch = () => {
      if (accumulatedResourcesRef.current.length > 0) {
        const batchedResources = [...accumulatedResourcesRef.current];
        const batchSize = batchedResources.length;

        if (isFirstBatch) {
          // First batch: urgent update to show data immediately
          setResources(prev => [...prev, ...batchedResources]);
          setLoadedCount(prev => prev + batchSize);
          setLoading(false); // Stop showing spinner after first batch
          isFirstBatch = false;
        } else {
          // Subsequent batches: use startTransition for non-critical updates
          startTransition(() => {
            setResources(prev => [...prev, ...batchedResources]);
            setLoadedCount(prev => prev + batchSize);
          });
        }

        accumulatedResourcesRef.current = [];
      }
    };

    try {
      setLoading(true);
      setError(null);
      setResources([]);
      setTotalCount(0);
      setLoadedCount(0);

      // Fetch timeline data from backend using gRPC streaming
      // Use raw string expressions if provided, otherwise use millisecond timestamps
      const startParam = options?.rawStart || startTimeMs;
      const endParam = options?.rawEnd || endTimeMs;

      if (!startParam || !endParam) {
        setLoading(false);
        return;
      }

      const BATCH_SIZE = 50; // Flush every 50 resources
      const BATCH_DELAY_MS = 150; // Or after 150ms of no new data

      // Use gRPC streaming with batched progressive rendering
      const result = await apiClient.getTimelineGrpc(
        startParam,
        endParam,
        options?.filters,
        (chunk) => {
          console.log("getTimeline Grpc: chunk.resources", chunk.resources);
          // Update total count from metadata (first chunk) - this is critical
          if (chunk.metadata?.totalCount !== undefined) {
            setTotalCount(chunk.metadata.totalCount);
          }

          // Accumulate resources for batching
          accumulatedResourcesRef.current.push(...chunk.resources);

          // Flush if we hit batch size or it's the last chunk
          if (accumulatedResourcesRef.current.length >= BATCH_SIZE || chunk.isComplete) {
            if (batchTimeoutRef.current) {
              clearTimeout(batchTimeoutRef.current);
              batchTimeoutRef.current = null;
            }
            flushBatch();
          } else {
            // Debounce: flush after delay if no new data arrives
            if (batchTimeoutRef.current) {
              clearTimeout(batchTimeoutRef.current);
            }
            batchTimeoutRef.current = setTimeout(() => {
              flushBatch();
              batchTimeoutRef.current = null;
            }, BATCH_DELAY_MS);
          }
        }
      );

      // Final cleanup: flush any remaining resources
      if (batchTimeoutRef.current) {
        clearTimeout(batchTimeoutRef.current);
        batchTimeoutRef.current = null;
      }
      flushBatch();
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        // Request was cancelled, ignore
        return;
      }
      const errorMessage = err instanceof Error ? err.message : 'Failed to fetch timeline data';
      setError(new Error(errorMessage));
      setLoading(false);
    }
  }, [startTimeMs, endTimeMs, filtersKey, options?.rawStart, options?.rawEnd, refreshToken]);

  useEffect(() => {
    fetchData();

    // Cleanup: abort ongoing request and clear batch timeout on unmount or when dependencies change
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      if (batchTimeoutRef.current) {
        clearTimeout(batchTimeoutRef.current);
        batchTimeoutRef.current = null;
      }
    };
  }, [fetchData]);

  return {
    resources,
    loading,
    error,
    refresh: () => fetchData(true),
    totalCount,
    loadedCount
  };
};
