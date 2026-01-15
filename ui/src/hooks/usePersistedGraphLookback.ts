import { useState, useCallback } from 'react';

const STORAGE_KEY = 'spectre-graph-lookback';
const DEFAULT_LOOKBACK = '10m';

/**
 * Available lookback options for the graph view
 */
export const LOOKBACK_OPTIONS = [
  { value: '5m', label: '5 minutes' },
  { value: '10m', label: '10 minutes' },
  { value: '30m', label: '30 minutes' },
  { value: '1h', label: '1 hour' },
  { value: '2h', label: '2 hours' },
  { value: '6h', label: '6 hours' },
  { value: '12h', label: '12 hours' },
  { value: '24h', label: '24 hours' },
];

/**
 * Hook to persist the lookback period for spec changes in the graph view.
 * The lookback is stored in localStorage and restored on page load.
 *
 * @returns The current lookback and a setter function
 */
export const usePersistedGraphLookback = () => {
  const [lookback, setLookbackState] = useState<string>(() => {
    if (typeof window === 'undefined') return DEFAULT_LOOKBACK;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      return stored || DEFAULT_LOOKBACK;
    } catch {
      return DEFAULT_LOOKBACK;
    }
  });

  const setLookback = useCallback((value: string) => {
    setLookbackState(value);
    if (typeof window === 'undefined') return;

    try {
      if (value && value !== DEFAULT_LOOKBACK) {
        window.localStorage.setItem(STORAGE_KEY, value);
      } else {
        // Remove from storage if it's the default value
        window.localStorage.removeItem(STORAGE_KEY);
      }
    } catch (error) {
      console.error('Failed to save lookback to localStorage:', error);
    }
  }, []);

  return { lookback, setLookback };
};
