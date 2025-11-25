import { ResourceStatus } from './types';

export const STATUS_COLORS: Record<ResourceStatus, string> = {
  [ResourceStatus.Ready]: '#10b981', // emerald-500
  [ResourceStatus.Warning]: '#f59e0b', // amber-500
  [ResourceStatus.Error]: '#ef4444', // red-500
  [ResourceStatus.Terminating]: '#6b7280', // gray-500
  [ResourceStatus.Unknown]: '#374151', // gray-700
};

export const MOCK_NAMESPACES = ['default', 'kube-system', 'production', 'staging', 'monitoring'];
export const MOCK_KINDS = ['Deployment', 'Pod', 'Service', 'ConfigMap', 'Ingress'];
