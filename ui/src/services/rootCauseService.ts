/**
 * Root Cause Analysis Service
 * Fetches root cause analysis from /v1/root-cause endpoint
 */

import { RootCauseAnalysisV2 } from '../types/rootCause';
import { withRetry } from '../utils/retry';

const CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

interface CacheEntry {
  data: RootCauseAnalysisV2;
  timestamp: number;
}

const cache = new Map<string, CacheEntry>();

function getCacheKey(resourceUID: string, timestamp: Date, lookbackMs?: number): string {
  // Round to nearest second to improve cache hit rate
  const timestampSec = Math.floor(timestamp.getTime() / 1000);
  const lookbackKey = lookbackMs ? `:${lookbackMs}` : '';
  return `${resourceUID}:${timestampSec}${lookbackKey}`;
}

function isCacheValid(entry: CacheEntry): boolean {
  return Date.now() - entry.timestamp < CACHE_TTL_MS;
}

export interface RootCauseOptions {
  maxDepth?: number;
  minConfidence?: number;
  lookbackMs?: number;  // Lookback in milliseconds
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
  const cacheKey = getCacheKey(resourceUID, failureTimestamp, options?.lookbackMs);
  const cached = cache.get(cacheKey);

  if (cached && isCacheValid(cached)) {
    console.log('[RootCause] Cache hit:', cacheKey);
    return cached.data;
  }

  // Build query parameters
  const params = new URLSearchParams({
    resourceUID,
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

  // Fetch from API with retry logic for network failures
  const url = `/v1/root-cause?${params.toString()}`;
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
