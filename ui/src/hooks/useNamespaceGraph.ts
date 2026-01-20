import { useState, useEffect, useCallback, useRef } from 'react';
import { apiClient } from '../services/api';
import { NamespaceGraphResponse, mergeNamespaceGraphResponses } from '../types/namespaceGraph';

export interface UseNamespaceGraphOptions {
  /** Namespace to fetch graph for (null to disable fetching) */
  namespace: string | null;
  /** Point in time for the graph snapshot */
  timestamp: Date;
  /** Include anomaly detection results */
  includeAnomalies?: boolean;
  /** Include causal path analysis */
  includeCausalPaths?: boolean;
  /** Lookback duration (e.g., "10m", "1h") */
  lookback?: string;
  /** Max relationship traversal depth */
  maxDepth?: number;
  /** Enable/disable data fetching */
  enabled?: boolean;
  /** Page size for pagination (default: 50) */
  pageSize?: number;
  /** Auto-load more pages when available (default: true) */
  autoLoad?: boolean;
  /** Delay between auto-load requests in ms (default: 100) */
  autoLoadDelay?: number;
}

export interface UseNamespaceGraphResult {
  /** Accumulated graph data */
  data: NamespaceGraphResponse | null;
  /** Loading initial page */
  isLoading: boolean;
  /** Loading additional pages */
  isLoadingMore: boolean;
  /** More pages available */
  hasMore: boolean;
  /** Total pages loaded */
  pagesLoaded: number;
  /** Error if any fetch failed */
  error: Error | null;
  /** Manually load next page */
  loadMore: () => void;
  /** Reset and refetch from scratch */
  refetch: () => void;
}

const DEFAULT_PAGE_SIZE = 50;
const DEFAULT_AUTO_LOAD_DELAY = 100;

/**
 * Hook to fetch namespace graph data from the API with pagination support
 *
 * @example
 * ```tsx
 * const { data, isLoading, isLoadingMore, hasMore, refetch } = useNamespaceGraph({
 *   namespace: 'production',
 *   timestamp: new Date(),
 *   includeAnomalies: true,
 *   autoLoad: true, // Automatically loads all pages
 * });
 * ```
 */
export function useNamespaceGraph(options: UseNamespaceGraphOptions): UseNamespaceGraphResult {
  const {
    namespace,
    timestamp,
    includeAnomalies = true,
    includeCausalPaths = false,
    lookback,
    maxDepth,
    enabled = true,
    pageSize = DEFAULT_PAGE_SIZE,
    autoLoad = true,
    autoLoadDelay = DEFAULT_AUTO_LOAD_DELAY,
  } = options;

  // Accumulated data state
  const [data, setData] = useState<NamespaceGraphResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [pagesLoaded, setPagesLoaded] = useState(0);

  // Pagination state
  const [hasMore, setHasMore] = useState(false);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  // Ref to track current fetch session to avoid race conditions
  const fetchSessionRef = useRef(0);
  // Ref to track if component is mounted
  const mountedRef = useRef(true);
  // Ref for auto-load timer
  const autoLoadTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Ref to track if auto-load is in progress (prevents duplicate scheduling)
  const isAutoLoadingRef = useRef(false);

  const timestampNanos = timestamp.getTime() * 1_000_000;

  // Store options in refs to avoid callback recreation
  const optionsRef = useRef({
    namespace,
    timestampNanos,
    includeAnomalies,
    includeCausalPaths,
    lookback,
    maxDepth,
    pageSize,
    autoLoadDelay,
  });
  optionsRef.current = {
    namespace,
    timestampNanos,
    includeAnomalies,
    includeCausalPaths,
    lookback,
    maxDepth,
    pageSize,
    autoLoadDelay,
  };

  // Fetch a single page - uses refs to avoid recreation
  const fetchPage = useCallback(
    async (cursor: string | undefined, sessionId: number, isFirstPage: boolean): Promise<boolean> => {
      const opts = optionsRef.current;

      const response = await apiClient.getNamespaceGraph({
        namespace: opts.namespace!,
        timestamp: opts.timestampNanos,
        includeAnomalies: opts.includeAnomalies,
        includeCausalPaths: opts.includeCausalPaths,
        lookback: opts.lookback,
        maxDepth: opts.maxDepth,
        limit: opts.pageSize,
        cursor,
      });

      // Check if this fetch is still relevant
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        return false;
      }

      if (isFirstPage) {
        setData(response);
      } else {
        setData(prev => (prev ? mergeNamespaceGraphResponses(prev, response) : response));
      }

      setHasMore(response.metadata.hasMore);
      setNextCursor(response.metadata.nextCursor);
      setPagesLoaded(prev => prev + 1);

      return response.metadata.hasMore;
    },
    [] // No dependencies - uses refs
  );

  // Clear auto-load timer
  const clearAutoLoadTimer = useCallback(() => {
    if (autoLoadTimerRef.current) {
      clearTimeout(autoLoadTimerRef.current);
      autoLoadTimerRef.current = null;
    }
  }, []);

  // Reset function - clears all state
  const reset = useCallback(() => {
    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(undefined);
    setPagesLoaded(0);
    setIsLoading(false);
    setIsLoadingMore(false);
  }, [clearAutoLoadTimer]);

  // Initial fetch effect - triggers when namespace/timestamp/enabled changes
  useEffect(() => {
    mountedRef.current = true;
    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;

    if (!namespace || !enabled) {
      reset();
      return;
    }

    // Start new fetch session
    const sessionId = ++fetchSessionRef.current;

    // Reset state for new fetch
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(undefined);
    setPagesLoaded(0);
    setIsLoading(true);
    setIsLoadingMore(false);

    const runInitialFetch = async () => {
      try {
        await fetchPage(undefined, sessionId, true);

        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setIsLoading(false);
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setIsLoading(false);
        }
      }
    };

    runInitialFetch();

    return () => {
      mountedRef.current = false;
      clearAutoLoadTimer();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace, timestampNanos, lookback, enabled]);

  // Auto-load effect - watches for cursor changes and loads more
  useEffect(() => {
    // Skip if auto-load is disabled or conditions not met
    if (!autoLoad || !hasMore || !nextCursor || isLoading) {
      return;
    }

    // Skip if already auto-loading (prevents duplicate fetches)
    if (isAutoLoadingRef.current) {
      return;
    }

    const sessionId = fetchSessionRef.current;
    const cursor = nextCursor;
    const delay = optionsRef.current.autoLoadDelay;

    // Mark as auto-loading
    isAutoLoadingRef.current = true;

    // Clear any existing timer
    clearAutoLoadTimer();

    // Schedule the next fetch
    autoLoadTimerRef.current = setTimeout(async () => {
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        isAutoLoadingRef.current = false;
        return;
      }

      setIsLoadingMore(true);

      try {
        const moreAvailable = await fetchPage(cursor, sessionId, false);

        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          // Reset auto-loading flag - if more available, this effect will re-run
          isAutoLoadingRef.current = false;

          if (!moreAvailable) {
            setIsLoadingMore(false);
          }
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          console.error('[useNamespaceGraph] Auto-load error:', err);
          isAutoLoadingRef.current = false;
          setIsLoadingMore(false);
        }
      }
    }, delay);

    return () => {
      clearAutoLoadTimer();
    };
  }, [autoLoad, hasMore, nextCursor, isLoading, fetchPage, clearAutoLoadTimer]);

  // Manual load more function
  const loadMore = useCallback(() => {
    if (!hasMore || isLoadingMore || isLoading || !nextCursor) return;

    const sessionId = fetchSessionRef.current;
    setIsLoadingMore(true);

    fetchPage(nextCursor, sessionId, false)
      .then(moreAvailable => {
        if (mountedRef.current && sessionId === fetchSessionRef.current && !moreAvailable) {
          setIsLoadingMore(false);
        }
      })
      .catch(err => {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setIsLoadingMore(false);
        }
      });
  }, [hasMore, isLoadingMore, isLoading, nextCursor, fetchPage]);

  // Refetch function - resets and starts fresh
  const refetch = useCallback(() => {
    if (!namespace || !enabled) return;

    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;

    // Start new fetch session
    const sessionId = ++fetchSessionRef.current;

    // Reset state
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(undefined);
    setPagesLoaded(0);
    setIsLoading(true);
    setIsLoadingMore(false);

    const runFetch = async () => {
      try {
        await fetchPage(undefined, sessionId, true);

        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setIsLoading(false);
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setIsLoading(false);
        }
      }
    };

    runFetch();
  }, [namespace, enabled, fetchPage, clearAutoLoadTimer]);

  return {
    data,
    isLoading,
    isLoadingMore,
    hasMore,
    pagesLoaded,
    error,
    loadMore,
    refetch,
  };
}
