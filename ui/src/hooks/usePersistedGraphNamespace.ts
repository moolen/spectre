import { useState, useEffect, useCallback } from 'react';

const STORAGE_KEY = 'spectre-graph-namespace';

/**
 * Hook to persist the selected namespace in the graph view.
 * The namespace is stored in localStorage and restored on page load.
 * 
 * @param availableNamespaces - List of available namespaces from metadata
 * @returns The current namespace and a setter function
 */
export const usePersistedGraphNamespace = (availableNamespaces: string[]) => {
  const [namespace, setNamespaceState] = useState<string | null>(() => {
    if (typeof window === 'undefined') return null;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      return stored || null;
    } catch {
      return null;
    }
  });

  // Validate namespace against available namespaces when they load
  useEffect(() => {
    if (availableNamespaces.length === 0) return;
    
    if (namespace && !availableNamespaces.includes(namespace)) {
      // Stored namespace no longer exists, clear it
      console.log('[usePersistedGraphNamespace] Stored namespace not available, clearing');
      setNamespaceState(null);
      try {
        window.localStorage.removeItem(STORAGE_KEY);
      } catch {
        // Ignore localStorage errors
      }
    }
  }, [namespace, availableNamespaces]);

  // Save to localStorage whenever namespace changes
  const setNamespace = useCallback((ns: string | null) => {
    setNamespaceState(ns);
    if (typeof window === 'undefined') return;
    
    try {
      if (ns) {
        window.localStorage.setItem(STORAGE_KEY, ns);
      } else {
        window.localStorage.removeItem(STORAGE_KEY);
      }
    } catch (error) {
      console.error('Failed to save namespace to localStorage:', error);
    }
  }, []);

  return { namespace, setNamespace };
};
