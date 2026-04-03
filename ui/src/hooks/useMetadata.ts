import { useState, useEffect } from 'react';
import { apiClient } from '../services/api';
import { TimeRange } from '../types';
import { toNamespaceFilterValue } from '../utils/namespaceFilters';

interface UseMetadataResult {
  namespaces: string[];
  kinds: string[];
  loading: boolean;
  error: Error | null;
}

/**
 * Hook to fetch metadata (namespaces, kinds) for a given time range
 */
export const useMetadata = (timeRange: TimeRange | null): UseMetadataResult => {
  const [namespaces, setNamespaces] = useState<string[]>([]);
  const [kinds, setKinds] = useState<string[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);
  const startTimeMs = timeRange?.start.getTime();
  const endTimeMs = timeRange?.end.getTime();

  useEffect(() => {
    if (startTimeMs === undefined || endTimeMs === undefined) {
      setLoading(false);
      return;
    }

    const fetchMetadata = async () => {
      try {
        setLoading(true);
        setError(null);

        let metadata = await apiClient.getMetadata(
          startTimeMs,
          endTimeMs
        );
        setNamespaces((metadata.namespaces || []).map(toNamespaceFilterValue));
        setKinds(metadata.kinds || []);
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : 'Failed to fetch metadata';
        setError(new Error(errorMessage));
        console.error('Metadata fetch error:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchMetadata();
  }, [startTimeMs, endTimeMs]);

  return {
    namespaces,
    kinds,
    loading,
    error,
  };
};
