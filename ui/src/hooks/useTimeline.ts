import { useState, useEffect, useCallback } from 'react';
import { K8sResource } from '../types';
import { apiClient } from '../services/api';

interface UseTimelineResult {
  resources: K8sResource[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

interface UseTimelineOptions {
  filters?: {
    namespace?: string;
    kind?: string;
    group?: string;
    version?: string;
  };
}

export const useTimeline = (options?: UseTimelineOptions): UseTimelineResult => {
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // Calculate default time range: last 2 hours
      const now = Date.now();
      const twoHoursAgo = now - (2 * 60 * 60 * 1000);

      // Fetch resources from backend API
      const data = await apiClient.searchResources(
        twoHoursAgo,
        now,
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
  }, [options?.filters]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  return {
    resources,
    loading,
    error,
    refresh: fetchData
  };
};
