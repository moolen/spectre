/**
 * Root Cause Analysis Service
 * Fetches root cause analysis from /v1/root-cause endpoint
 */

import { RootCauseAnalysisV2, AnomalyResponse, CausalPathsResponse } from '../types/rootCause';
import { withRetry } from '../utils/retry';

const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

/**
 * Strip the /v1/Kind/ prefix from resource UID if present
 * Converts "/v1/Pod/abc-123" to "abc-123"
 */
function stripResourceUIDPrefix(uid: string): string {
  // If UID has the format /v1/Kind/UUID, extract just the UUID
  const parts = uid.split('/');
  if (parts.length >= 3 && parts[0] === '' && parts[1] === 'v1') {
    // Return the last part (UUID)
    return parts[parts.length - 1];
  }
  // Return as-is if format doesn't match
  return uid;
}

interface CacheEntry {
  data: RootCauseAnalysisV2;
  timestamp: number;
}

const cache = new Map<string, CacheEntry>();

function getCacheKey(resourceUID: string, timestamp: Date, lookbackMs?: number, format?: ResponseFormat): string {
  // Round to nearest second to improve cache hit rate
  const timestampSec = Math.floor(timestamp.getTime() / 1000);
  const lookbackKey = lookbackMs ? `:${lookbackMs}` : '';
  const formatKey = format ? `:${format}` : ':diff';
  return `${resourceUID}:${timestampSec}${lookbackKey}${formatKey}`;
}

function isCacheValid(entry: CacheEntry): boolean {
  return Date.now() - entry.timestamp < CACHE_TTL_MS;
}

export type ResponseFormat = 'legacy' | 'diff';

export interface RootCauseOptions {
  maxDepth?: number;
  minConfidence?: number;
  lookbackMs?: number;  // Lookback in milliseconds
  format?: ResponseFormat; // Response format: 'legacy' or 'diff' (default: 'diff')
}

/**
 * Fetch root cause analysis for a failing resource
 */
export async function fetchRootCauseAnalysis(
  resourceUID: string,
  failureTimestamp: Date,
  options?: RootCauseOptions
): Promise<RootCauseAnalysisV2> {
  // Check cache first
  const cacheKey = getCacheKey(resourceUID, failureTimestamp, options?.lookbackMs, options?.format);
  const cached = cache.get(cacheKey);

  if (cached && isCacheValid(cached)) {
    console.log('[RootCause] Cache hit:', cacheKey);
    return cached.data;
  }

  // Strip the /v1/Kind/ prefix from resourceUID if present
  const cleanUID = stripResourceUIDPrefix(resourceUID);

  // Build query parameters
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    failureTimestamp: failureTimestamp.getTime().toString(),
  });

  if (options?.maxDepth !== undefined) {
    params.set('maxDepth', options.maxDepth.toString());
  }

  if (options?.minConfidence !== undefined) {
    params.set('minConfidence', options.minConfidence.toString());
  }

  if (options?.lookbackMs !== undefined) {
    params.set('lookbackMs', options.lookbackMs.toString());
  }

  // Default to 'diff' format for new format with significance scoring
  params.set('format', options?.format || 'diff');

  // Fetch from API with retry logic for network failures
  const url = `/v1/causal-graph?${params.toString()}`;
  console.log('[RootCause] Fetching:', url);

  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      console.log('[RootCause] Response status:', response.status, response.statusText);

      if (!response.ok) {
        if (response.status === 404) {
          throw new Error('No root cause analysis available for this resource');
        }
        const errorText = await response.text();
        console.error('[RootCause] Error response:', errorText);
        throw new Error(`Root cause analysis failed: ${response.status} ${errorText}`);
      }

      const data: RootCauseAnalysisV2 = await response.json();
      console.log('[RootCause] Received data:', data);
      return data;
    },
    {
      maxAttempts: 3,
      initialDelay: 1000,
      onRetry: (attempt, error) => {
        console.log(`[RootCause] Retry attempt ${attempt} after error:`, error);
      },
    }
  );

  // Cache the result
  cache.set(cacheKey, {
    data,
    timestamp: Date.now(),
  });

  return data;
}

/**
 * Clear the cache (e.g., when time range changes significantly)
 */
export function clearRootCauseCache(): void {
  cache.clear();
  console.log('[RootCause] Cache cleared');
}

/**
 * Prune expired entries from cache
 */
export function pruneRootCauseCache(): void {
  const now = Date.now();
  for (const [key, entry] of cache.entries()) {
    if (now - entry.timestamp >= CACHE_TTL_MS) {
      cache.delete(key);
    }
  }
}

// Auto-prune every minute
setInterval(pruneRootCauseCache, 60_000);

/**
 * Fetch anomalies for a resource and time window
 * @param resourceUID - The UID of the symptom resource
 * @param startUnixSeconds - Start of time window in Unix seconds
 * @param endUnixSeconds - End of time window in Unix seconds
 */
export async function fetchAnomalies(
  resourceUID: string,
  startUnixSeconds: number,
  endUnixSeconds: number
): Promise<AnomalyResponse> {
  // Strip the /v1/Kind/ prefix from resourceUID if present
  const cleanUID = stripResourceUIDPrefix(resourceUID);

  // Build query parameters
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    start: startUnixSeconds.toString(),
    end: endUnixSeconds.toString(),
  });

  const url = `/v1/anomalies?${params.toString()}`;
  console.log('[Anomalies] Fetching:', url);

  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      console.log('[Anomalies] Response status:', response.status, response.statusText);

      if (!response.ok) {
        if (response.status === 404) {
          throw new Error('No anomalies available for this resource');
        }
        const errorText = await response.text();
        console.error('[Anomalies] Error response:', errorText);
        throw new Error(`Anomaly detection failed: ${response.status} ${errorText}`);
      }

      const data: AnomalyResponse = await response.json();
      console.log('[Anomalies] Received data:', data);
      return data;
    },
    {
      maxAttempts: 3,
      initialDelay: 1000,
      onRetry: (attempt, error) => {
        console.log(`[Anomalies] Retry attempt ${attempt} after error:`, error);
      },
    }
  );

  return data;
}

/**
 * Options for causal paths API
 */
export interface CausalPathsOptions {
  maxDepth?: number;      // Maximum traversal depth (default: 5)
  maxPaths?: number;      // Maximum paths to return (default: 5)
  lookbackMs?: number;    // Lookback window in milliseconds
}

/**
 * Fetch causal paths for a resource and time window
 * @param resourceUID - The UID of the symptom resource
 * @param failureTimestamp - Timestamp of the failure in milliseconds
 * @param options - Optional parameters for the query
 */
export async function fetchCausalPaths(
  resourceUID: string,
  failureTimestamp: number,
  options?: CausalPathsOptions
): Promise<CausalPathsResponse> {
  // Strip the /v1/Kind/ prefix from resourceUID if present
  const cleanUID = stripResourceUIDPrefix(resourceUID);

  // Build query parameters
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    failureTimestamp: failureTimestamp.toString(),
  });

  if (options?.maxDepth !== undefined) {
    params.set('maxDepth', options.maxDepth.toString());
  }

  if (options?.maxPaths !== undefined) {
    params.set('maxPaths', options.maxPaths.toString());
  }

  if (options?.lookbackMs !== undefined) {
    params.set('lookbackMs', options.lookbackMs.toString());
  }

  const url = `/v1/causal-paths?${params.toString()}`;
  console.log('[CausalPaths] Fetching:', url);

  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      console.log('[CausalPaths] Response status:', response.status, response.statusText);

      if (!response.ok) {
        if (response.status === 404) {
          // Return empty response for 404 (no paths found)
          return {
            paths: [],
            metadata: {
              queryExecutionMs: 0,
              algorithmVersion: 'unknown',
              executedAt: new Date().toISOString(),
              nodesExplored: 0,
              pathsDiscovered: 0,
              pathsReturned: 0,
            },
          } as CausalPathsResponse;
        }
        const errorText = await response.text();
        console.error('[CausalPaths] Error response:', errorText);
        throw new Error(`Causal paths fetch failed: ${response.status} ${errorText}`);
      }

      const data: CausalPathsResponse = await response.json();
      console.log('[CausalPaths] Received data:', data.paths.length, 'paths');
      return data;
    },
    {
      maxAttempts: 3,
      initialDelay: 1000,
      onRetry: (attempt, error) => {
        console.log(`[CausalPaths] Retry attempt ${attempt} after error:`, error);
      },
    }
  );

  return data;
}
