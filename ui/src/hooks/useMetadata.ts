import { useState, useEffect } from 'react';
import { apiClient } from '../services/api';
import { TimeRange } from '../types';

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

  useEffect(() => {
    if (!timeRange) {
      setLoading(false);
      return;
    }

    const fetchMetadata = async () => {
      try {
        setLoading(true);
        setError(null);

        const metadata = await apiClient.getMetadata(
          timeRange.start.getTime(),
          timeRange.end.getTime()
        );

        setNamespaces(metadata.namespaces || []);
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
  }, [timeRange?.start.getTime(), timeRange?.end.getTime()]);

  return {
    namespaces,
    kinds,
    loading,
    error,
  };
};

