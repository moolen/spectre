import { useState, useEffect, useCallback } from 'react';
import { K8sResource } from '../types';
import { generateMockData } from '../services/mockData';

interface UseTimelineResult {
  resources: K8sResource[];
  loading: boolean;
  error: Error | null;
  refresh: () => void;
}

export const useTimeline = (): UseTimelineResult => {
  const [resources, setResources] = useState<K8sResource[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // TODO: Replace with actual API call once backend is available
      // const response = await fetch('/api/resources');
      // const data = await response.json();
      // setResources(data);

      // For now, use mock data
      const mockData = generateMockData(50);
      setResources(mockData);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch timeline data'));
      console.error('Timeline fetch error:', err);
    } finally {
      setLoading(false);
    }
  }, []);

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
