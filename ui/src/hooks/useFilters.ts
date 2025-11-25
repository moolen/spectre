import { useMemo, useCallback } from 'react';
import { K8sResource, FilterState } from '../types';

interface UseFiltersResult {
  filteredResources: K8sResource[];
  filters: FilterState;
  setFilters: (filters: FilterState) => void;
  updateNamespaceFilter: (namespace: string, selected: boolean) => void;
  updateKindFilter: (kind: string, selected: boolean) => void;
  setSearchFilter: (search: string) => void;
}

export const useFilters = (
  resources: K8sResource[],
  initialFilters: FilterState,
  onFiltersChange: (filters: FilterState) => void
): UseFiltersResult => {
  const filters = initialFilters;

  // Compute filtered resources based on current filters
  const filteredResources = useMemo(() => {
    return resources.filter(resource => {
      // Namespace filter
      if (filters.namespaces.length > 0 && !filters.namespaces.includes(resource.namespace)) {
        return false;
      }

      // Kind filter
      if (filters.kinds.length > 0 && !filters.kinds.includes(resource.kind)) {
        return false;
      }

      // Search filter (case-insensitive)
      if (filters.search) {
        const searchLower = filters.search.toLowerCase();
        if (!resource.name.toLowerCase().includes(searchLower)) {
          return false;
        }
      }

      return true;
    });
  }, [resources, filters]);

  const updateNamespaceFilter = useCallback((namespace: string, selected: boolean) => {
    const newNamespaces = selected
      ? [...filters.namespaces, namespace]
      : filters.namespaces.filter(ns => ns !== namespace);

    onFiltersChange({
      ...filters,
      namespaces: newNamespaces
    });
  }, [filters, onFiltersChange]);

  const updateKindFilter = useCallback((kind: string, selected: boolean) => {
    const newKinds = selected
      ? [...filters.kinds, kind]
      : filters.kinds.filter(k => k !== kind);

    onFiltersChange({
      ...filters,
      kinds: newKinds
    });
  }, [filters, onFiltersChange]);

  const setSearchFilter = useCallback((search: string) => {
    onFiltersChange({
      ...filters,
      search
    });
  }, [filters, onFiltersChange]);

  return {
    filteredResources,
    filters,
    setFilters: onFiltersChange,
    updateNamespaceFilter,
    updateKindFilter,
    setSearchFilter
  };
};
