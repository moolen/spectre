import { useState, useEffect, useCallback, useRef } from 'react';
import { apiClient } from '../services/api';
import { ObservatoryGraphResponse, ObservatoryGraphRequest } from '../types/observatoryGraph';

export interface UseObservatoryGraphOptions {
  /** Integration name to filter (optional) */
  integration?: string;
  /** Namespace to filter SignalAnchors by workload (optional) */
  namespace?: string;
  /** Workload name to filter SignalAnchors (optional) */
  workload?: string;
  /** Include SignalBaseline nodes (optional) */
  includeBaselines?: boolean;
  /** Maximum number of SignalAnchor nodes (default: 100) */
  limit?: number;
  /** Enable/disable data fetching */
  enabled?: boolean;
}

export interface UseObservatoryGraphResult {
  /** Graph data */
  data: ObservatoryGraphResponse | null;
  /** Loading state */
  isLoading: boolean;
  /** Error if any fetch failed */
  error: Error | null;
  /** Refetch the data */
  refetch: () => void;
}

const DEFAULT_LIMIT = 100;

/**
 * Hook to fetch observatory graph data from the API
 *
 * @example
 * ```tsx
 * const { data, isLoading, error, refetch } = useObservatoryGraph({
 *   integration: 'grafana-prod',
 *   namespace: 'production',
 * });
 * ```
 */
export function useObservatoryGraph(options: UseObservatoryGraphOptions): UseObservatoryGraphResult {
  const {
    integration,
    namespace,
    workload,
    includeBaselines = false,
    limit = DEFAULT_LIMIT,
    enabled = true,
  } = options;

  const [data, setData] = useState<ObservatoryGraphResponse | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Ref to track current fetch session to avoid race conditions
  const fetchSessionRef = useRef(0);
  // Ref to track if component is mounted
  const mountedRef = useRef(true);

  // Store options in refs to avoid callback recreation
  const optionsRef = useRef<ObservatoryGraphRequest>({
    integration,
    namespace,
    workload,
    includeBaselines,
    limit,
  });
  optionsRef.current = {
    integration,
    namespace,
    workload,
    includeBaselines,
    limit,
  };

  // Fetch data function
  const fetchData = useCallback(async (sessionId: number) => {
    const opts = optionsRef.current;

    try {
      const response = await apiClient.getObservatoryGraph(opts);

      // Check if this fetch is still relevant
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        return;
      }

      setData(response);
      setError(null);
    } catch (err) {
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        return;
      }
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      if (mountedRef.current && sessionId === fetchSessionRef.current) {
        setIsLoading(false);
      }
    }
  }, []);

  // Initial fetch effect
  useEffect(() => {
    mountedRef.current = true;

    if (!enabled) {
      setData(null);
      setError(null);
      setIsLoading(false);
      return;
    }

    // Start new fetch session
    const sessionId = ++fetchSessionRef.current;

    setIsLoading(true);
    setError(null);

    fetchData(sessionId);

    return () => {
      mountedRef.current = false;
    };
  }, [integration, namespace, workload, includeBaselines, limit, enabled, fetchData]);

  // Refetch function
  const refetch = useCallback(() => {
    if (!enabled) return;

    const sessionId = ++fetchSessionRef.current;

    setIsLoading(true);
    setError(null);

    fetchData(sessionId);
  }, [enabled, fetchData]);

  return {
    data,
    isLoading,
    error,
    refetch,
  };
}
