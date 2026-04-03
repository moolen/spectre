import { describe, expect, it } from 'vitest';
import {
  CLUSTER_SCOPED_NAMESPACE_LABEL,
  CLUSTER_SCOPED_NAMESPACE_VALUE,
  formatNamespaceFilterOption,
  fromNamespaceFilterValue,
  sortNamespaceFilterOptions,
  toNamespaceFilterValue,
} from './namespaceFilters';

describe('namespaceFilters', () => {
  it('maps empty namespaces to the cluster-scoped sentinel', () => {
    expect(toNamespaceFilterValue('')).toBe(CLUSTER_SCOPED_NAMESPACE_VALUE);
    expect(fromNamespaceFilterValue(CLUSTER_SCOPED_NAMESPACE_VALUE)).toBe('');
  });

  it('formats cluster-scoped namespaces with an explicit label', () => {
    expect(formatNamespaceFilterOption('')).toBe(CLUSTER_SCOPED_NAMESPACE_LABEL);
    expect(formatNamespaceFilterOption(CLUSTER_SCOPED_NAMESPACE_VALUE)).toBe(CLUSTER_SCOPED_NAMESPACE_LABEL);
    expect(formatNamespaceFilterOption('default')).toBe('default');
  });

  it('sorts named namespaces alphabetically and keeps cluster-scoped last', () => {
    expect(sortNamespaceFilterOptions(['zeta', '', 'alpha'])).toEqual([
      'alpha',
      'zeta',
      CLUSTER_SCOPED_NAMESPACE_VALUE,
    ]);
  });
});
