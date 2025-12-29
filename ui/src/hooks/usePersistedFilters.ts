import { useState, useEffect, useCallback } from 'react';

const STORAGE_KEY_KINDS = 'spectre-filters-kinds';
const STORAGE_KEY_NAMESPACES = 'spectre-filters-namespaces';

export interface PersistedFilters {
  kinds: string[];
  namespaces: string[];
}

const DEFAULT_RUNTIME_KINDS = [
  'Deployment',
  'StatefulSet',
  'ReplicaSet',
  'Pod',
  'Job',
  'CronJob',
  'Service'
];

const loadFilters = (): PersistedFilters => {
  if (typeof window === 'undefined') {
    return { kinds: DEFAULT_RUNTIME_KINDS, namespaces: [] };
  }

  try {
    const kindsStr = window.localStorage.getItem(STORAGE_KEY_KINDS);
    const namespacesStr = window.localStorage.getItem(STORAGE_KEY_NAMESPACES);

    const kinds = kindsStr ? JSON.parse(kindsStr) : DEFAULT_RUNTIME_KINDS;
    const namespaces = namespacesStr ? JSON.parse(namespacesStr) : [];

    return {
      kinds: Array.isArray(kinds) ? kinds : DEFAULT_RUNTIME_KINDS,
      namespaces: Array.isArray(namespaces) ? namespaces : []
    };
  } catch {
    return { kinds: DEFAULT_RUNTIME_KINDS, namespaces: [] };
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
  availableNamespaces: string[]
) => {
  const [filters, setFilters] = useState<PersistedFilters>(() => loadFilters());

  // Validate filters against available options when metadata changes
  // Use functional update to avoid dependency on filters state
  useEffect(() => {
    // Only validate if we have available options (metadata has loaded)
    // Skip validation if metadata arrays are empty (not loaded yet)
    if (availableKinds.length === 0 && availableNamespaces.length === 0) {
      return;
    }

    setFilters(prev => {
      // Only validate kinds if we have available kinds, otherwise keep current
      const validatedKinds = availableKinds.length > 0
        ? prev.kinds.filter(kind => availableKinds.includes(kind))
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
  }, [availableKinds, availableNamespaces]);

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

