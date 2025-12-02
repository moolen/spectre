import { useState, useEffect } from 'react';

const STORAGE_KEY = 'spectre-quick-preset';

/**
 * Hook for persisting quick preset selection in localStorage
 * Quick presets are relative time ranges like "Last 15m", "Last 30m", etc.
 */
export const usePersistedQuickPreset = () => {
  const [preset, setPreset] = useState<number | null>(null);

  // Load preset from localStorage on mount
  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      if (stored) {
        const minutes = parseInt(stored, 10);
        if (!isNaN(minutes)) {
          setPreset(minutes);
        }
      }
    } catch (error) {
      console.error('Failed to load preset from localStorage:', error);
    }
  }, []);

  const savePreset = (minutes: number) => {
    try {
      window.localStorage.setItem(STORAGE_KEY, minutes.toString());
      setPreset(minutes);
    } catch (error) {
      console.error('Failed to save preset to localStorage:', error);
    }
  };

  const clearPreset = () => {
    try {
      window.localStorage.removeItem(STORAGE_KEY);
      setPreset(null);
    } catch (error) {
      console.error('Failed to clear preset from localStorage:', error);
    }
  };

  return {
    preset,
    savePreset,
    clearPreset
  };
};
