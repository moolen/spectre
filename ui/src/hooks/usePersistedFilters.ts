import { useState, useEffect, useCallback, useRef } from 'react';

const STORAGE_KEY_KINDS = 'spectre-filters-kinds';
const STORAGE_KEY_NAMESPACES = 'spectre-filters-namespaces';
const STORAGE_KEY_DEFAULT_KINDS_HASH = 'spectre-filters-default-kinds-hash';

export interface PersistedFilters {
  kinds: string[];
  namespaces: string[];
}

// Simple hash function to detect when default kinds change
const hashArray = (arr: string[]): string => {
  return arr.slice().sort().join(',');
};

const loadFilters = (defaultKinds: string[]): PersistedFilters => {
  if (typeof window === 'undefined') {
    return { kinds: defaultKinds, namespaces: [] };
  }

  try {
    const kindsStr = window.localStorage.getItem(STORAGE_KEY_KINDS);
    const namespacesStr = window.localStorage.getItem(STORAGE_KEY_NAMESPACES);

    // Parse kinds from localStorage, but treat empty array as "use defaults"
    // An empty array means no explicit user selection, so fall back to defaults
    let kinds = defaultKinds;
    if (kindsStr) {
      const parsed = JSON.parse(kindsStr);
      if (Array.isArray(parsed) && parsed.length > 0) {
        kinds = parsed;
      }
    }

    const namespaces = namespacesStr ? JSON.parse(namespacesStr) : [];

    return {
      kinds,
      namespaces: Array.isArray(namespaces) ? namespaces : []
    };
  } catch {
    return { kinds: defaultKinds, namespaces: [] };
  }
};

const saveFilters = (filters: PersistedFilters) => {
  if (typeof window === 'undefined') return;

  try {
    window.localStorage.setItem(STORAGE_KEY_KINDS, JSON.stringify(filters.kinds));
    window.localStorage.setItem(STORAGE_KEY_NAMESPACES, JSON.stringify(filters.namespaces));
  } catch (error) {
    console.error('Failed to save filters to localStorage:', error);
  }
};

export const usePersistedFilters = (
  availableKinds: string[],
  availableNamespaces: string[],
  defaultKinds: string[]
) => {
  const [filters, setFilters] = useState<PersistedFilters>(() => loadFilters(defaultKinds));
  const previousDefaultKindsHashRef = useRef<string | null>(null);
  const initializedRef = useRef(false);

  // Ensure we have kinds on initial load - if empty, use defaults
  // This handles the case where localStorage had an empty array from a previous bug
  useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      if (filters.kinds.length === 0 && defaultKinds.length > 0) {
        console.log('[usePersistedFilters] Empty kinds on init, applying defaults:', defaultKinds);
        setFilters(prev => ({ ...prev, kinds: defaultKinds }));
      }
    }
  }, [defaultKinds, filters.kinds.length]);

  // Reset filters when default kinds setting changes
  useEffect(() => {
    const currentHash = hashArray(defaultKinds);
    const storedHash = window.localStorage.getItem(STORAGE_KEY_DEFAULT_KINDS_HASH);
    
    // Initialize the ref on first render
    if (previousDefaultKindsHashRef.current === null) {
      previousDefaultKindsHashRef.current = storedHash || currentHash;
      // Store the hash if not already stored
      if (!storedHash) {
        window.localStorage.setItem(STORAGE_KEY_DEFAULT_KINDS_HASH, currentHash);
      }
      return;
    }

    // Check if defaults have changed since last time
    if (storedHash !== currentHash) {
      console.log('[usePersistedFilters] Default kinds changed, resetting filters');
      // Reset to new defaults
      setFilters(prev => ({ ...prev, kinds: defaultKinds }));
      // Update the stored hash
      window.localStorage.setItem(STORAGE_KEY_DEFAULT_KINDS_HASH, currentHash);
      previousDefaultKindsHashRef.current = currentHash;
    }
  }, [defaultKinds]);

  // Validate filters against available options when metadata changes
  // Use functional update to avoid dependency on filters state
  useEffect(() => {
    // Only validate if we have available options (metadata has loaded)
    // Skip validation if metadata arrays are empty (not loaded yet)
    if (availableKinds.length === 0 && availableNamespaces.length === 0) {
      return;
    }

    setFilters(prev => {
      // Validate kinds: keep if in availableKinds OR in defaultKinds
      // This preserves default kinds even when not in current metadata
      // (e.g., no Pods exist in the time range, but Pod is a valid default)
      const validatedKinds = availableKinds.length > 0
        ? prev.kinds.filter(kind => availableKinds.includes(kind) || defaultKinds.includes(kind))
        : prev.kinds;

      // Only validate namespaces if we have available namespaces, otherwise keep current
      const validatedNamespaces = availableNamespaces.length > 0
        ? prev.namespaces.filter(ns => availableNamespaces.includes(ns))
        : prev.namespaces;

      // Only update if validation removed any values
      if (validatedKinds.length !== prev.kinds.length ||
          validatedNamespaces.length !== prev.namespaces.length) {
        const validated = {
          kinds: validatedKinds,
          namespaces: validatedNamespaces
        };
        // Save will happen in the effect below
        return validated;
      }
      return prev;
    });
  }, [availableKinds, availableNamespaces, defaultKinds]);

  // Save to localStorage whenever filters change
  useEffect(() => {
    saveFilters(filters);
  }, [filters]);

  const setKinds = useCallback((kinds: string[]) => {
    setFilters(prev => ({ ...prev, kinds }));
  }, []);

  const setNamespaces = useCallback((namespaces: string[]) => {
    setFilters(prev => ({ ...prev, namespaces }));
  }, []);

  return {
    kinds: filters.kinds,
    namespaces: filters.namespaces,
    setKinds,
    setNamespaces
  };
};

