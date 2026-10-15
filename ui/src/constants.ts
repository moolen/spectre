import { ResourceStatus } from './types';

// Status colours resolve through theme tokens (src/theme.css) so they follow
// the active light/dark theme without re-rendering.
export const STATUS_COLORS: Record<ResourceStatus, string> = {
  [ResourceStatus.Ready]: 'var(--color-status-ok)',
  [ResourceStatus.Warning]: 'var(--color-status-warn)',
  [ResourceStatus.Error]: 'var(--color-status-err)',
  [ResourceStatus.Terminating]: 'var(--color-status-neutral)',
  [ResourceStatus.Unknown]: 'var(--color-border-strong)',
};

export const MOCK_NAMESPACES = ['default', 'kube-system', 'production', 'staging', 'monitoring'];
export const MOCK_KINDS = ['Deployment', 'Pod', 'Service', 'ConfigMap', 'Ingress'];
