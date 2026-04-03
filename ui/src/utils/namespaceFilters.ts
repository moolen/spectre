export const CLUSTER_SCOPED_NAMESPACE_VALUE = '__CLUSTER_SCOPED__';
export const CLUSTER_SCOPED_NAMESPACE_LABEL = 'Cluster-scoped';

export const toNamespaceFilterValue = (namespace: string): string =>
  namespace === '' ? CLUSTER_SCOPED_NAMESPACE_VALUE : namespace;

export const fromNamespaceFilterValue = (namespace: string): string =>
  namespace === CLUSTER_SCOPED_NAMESPACE_VALUE ? '' : namespace;

export const formatNamespaceFilterOption = (namespace: string): string =>
  fromNamespaceFilterValue(namespace) === '' ? CLUSTER_SCOPED_NAMESPACE_LABEL : namespace;

export const sortNamespaceFilterOptions = (namespaces: string[]): string[] => {
  const normalized = Array.from(new Set(namespaces.map(toNamespaceFilterValue)));
  const namedNamespaces = normalized
    .filter(namespace => namespace !== CLUSTER_SCOPED_NAMESPACE_VALUE)
    .sort((a, b) => a.localeCompare(b));

  if (normalized.includes(CLUSTER_SCOPED_NAMESPACE_VALUE)) {
    namedNamespaces.push(CLUSTER_SCOPED_NAMESPACE_VALUE);
  }

  return namedNamespaces;
};
