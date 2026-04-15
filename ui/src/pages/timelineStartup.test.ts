import { describe, expect, it } from 'vitest';
import { buildWatcherDisabledInitialRange } from './timelineStartup';

describe('buildWatcherDisabledInitialRange', () => {
  it('returns the metadata bounds when earliest is before latest', () => {
    const range = buildWatcherDisabledInitialRange({ earliest: 1735689600, latest: 1735693200 });

    expect(range).not.toBeNull();
    expect(range?.start.toISOString()).toBe('2025-01-01T00:00:00.000Z');
    expect(range?.end.toISOString()).toBe('2025-01-01T01:00:00.000Z');
  });

  it('pads single-event bounds by one second on each side', () => {
    const range = buildWatcherDisabledInitialRange({ earliest: 1735689600, latest: 1735689600 });

    expect(range?.start.toISOString()).toBe('2024-12-31T23:59:59.000Z');
    expect(range?.end.toISOString()).toBe('2025-01-01T00:00:01.000Z');
  });

  it('returns null when the bounds are missing or invalid', () => {
    expect(buildWatcherDisabledInitialRange({ earliest: 0, latest: 0 })).toBeNull();
    expect(buildWatcherDisabledInitialRange({ earliest: -1, latest: 10 })).toBeNull();
    expect(buildWatcherDisabledInitialRange({ earliest: 20, latest: 10 })).toBeNull();
  });
});
