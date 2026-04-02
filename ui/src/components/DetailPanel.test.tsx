import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DetailPanel } from './DetailPanel';
import { SettingsProvider } from '../hooks/useSettings';
import { K8sResource, ResourceStatus } from '../types';

const resource: K8sResource = {
  id: 'pod-uid-1',
  group: '',
  version: 'v1',
  kind: 'Pod',
  namespace: 'default',
  name: 'example-pod',
  statusSegments: [
    {
      start: new Date('2026-01-01T00:00:00Z'),
      end: new Date('2026-01-01T00:05:00Z'),
      status: ResourceStatus.Error,
      message: 'ImagePullBackOff',
      resourceData: {
        spec: {
          containers: [{ image: 'broken:tag' }],
        },
      },
    },
  ],
  events: [
    {
      id: 'event-1',
      timestamp: new Date('2026-01-01T00:01:00Z'),
      reason: 'Failed',
      message: 'Failed to pull image',
      type: 'Warning',
      count: 1,
      source: 'kubelet',
    },
  ],
};

describe('DetailPanel', () => {
  it('does not render the root cause action button', () => {
    render(
      <SettingsProvider>
        <DetailPanel
          {...({
            resource,
            selectedIndex: 0,
            onClose: vi.fn(),
            onAnalyzeRootCause: vi.fn(),
          } as any)}
        />
      </SettingsProvider>
    );

    expect(screen.queryByRole('button', { name: 'Analyze Root Cause' })).not.toBeInTheDocument();
  });
});
