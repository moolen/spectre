import { beforeAll, afterAll, describe, expect, it, vi } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { Timeline } from './Timeline';
import { ResourceStatus } from '../types';

vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({
    compactMode: false,
    formatTime: (date: Date) => date.toISOString(),
    theme: 'dark',
  }),
}));

describe('Timeline', () => {
  const originalGetBBox = SVGElement.prototype.getBBox;
  const originalRAF = globalThis.requestAnimationFrame;
  const originalCancelRAF = globalThis.cancelAnimationFrame;

  beforeAll(() => {
    SVGElement.prototype.getBBox = () =>
      ({
        x: 0,
        y: 0,
        width: 80,
        height: 16,
        top: 0,
        left: 0,
        right: 80,
        bottom: 16,
        toJSON: () => ({}),
      }) as DOMRect;

    globalThis.requestAnimationFrame = (callback: FrameRequestCallback) => {
      callback(0);
      return 1;
    };
    globalThis.cancelAnimationFrame = () => {};
  });

  afterAll(() => {
    SVGElement.prototype.getBBox = originalGetBBox;
    globalThis.requestAnimationFrame = originalRAF;
    globalThis.cancelAnimationFrame = originalCancelRAF;
  });

  it('renders a continuation cue without dimming the whole segment for pre-existing resources', async () => {
    const rangeStart = new Date('2026-04-02T21:15:00Z');
    const rangeEnd = new Date('2026-04-02T22:45:00Z');

    const { container } = render(
      <Timeline
        resources={[
          {
            id: 'deployment-1',
            group: 'apps',
            version: 'v1',
            kind: 'Deployment',
            namespace: 'default',
            name: 'baufi-optimierer',
            preExisting: true,
            statusSegments: [
              {
                start: rangeStart,
                end: new Date('2026-04-02T21:51:53Z'),
                status: ResourceStatus.Ready,
                message: 'Ready',
              },
            ],
            events: [],
          },
        ]}
        onSegmentClick={() => {}}
        selectedPoint={null}
        timeRange={{ start: rangeStart, end: rangeEnd }}
      />
    );

    await waitFor(() => {
      expect(container.querySelectorAll('.segment').length).toBe(1);
    });

    const mainSegment = container.querySelector<SVGRectElement>('.segment');
    expect(mainSegment).not.toBeNull();
    expect(mainSegment?.getAttribute('opacity')).toBe('1');

    const continuationCue = container.querySelector('.segment-continuation');
    expect(continuationCue).not.toBeNull();
  });
});
