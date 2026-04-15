import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  detectWatcherEnabled,
  getWatcherEnabled,
  resetRuntimeModeForTests,
} from './api';

describe('api runtime mode detection', () => {
  afterEach(() => {
    resetRuntimeModeForTests();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('caches watcherEnabled from /health', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ watcherEnabled: false }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const watcherEnabled = await detectWatcherEnabled();
    const secondWatcherEnabled = await detectWatcherEnabled();

    expect(watcherEnabled).toBe(false);
    expect(secondWatcherEnabled).toBe(false);
    expect(getWatcherEnabled()).toBe(false);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('defaults watcherEnabled to true when the health response omits it', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ status: 'healthy' }),
      }),
    );

    const watcherEnabled = await detectWatcherEnabled();

    expect(watcherEnabled).toBe(true);
    expect(getWatcherEnabled()).toBe(true);
  });

  it('caches watcherEnabled=true when /health returns non-ok', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      json: async () => ({ watcherEnabled: false }),
    });
    vi.stubGlobal('fetch', fetchMock);

    const watcherEnabled = await detectWatcherEnabled();
    const secondWatcherEnabled = await detectWatcherEnabled();

    expect(watcherEnabled).toBe(true);
    expect(secondWatcherEnabled).toBe(true);
    expect(getWatcherEnabled()).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it('caches watcherEnabled=true when /health fetch fails', async () => {
    const fetchMock = vi.fn().mockRejectedValue(new Error('network down'));
    vi.stubGlobal('fetch', fetchMock);

    const watcherEnabled = await detectWatcherEnabled();
    const secondWatcherEnabled = await detectWatcherEnabled();

    expect(watcherEnabled).toBe(true);
    expect(secondWatcherEnabled).toBe(true);
    expect(getWatcherEnabled()).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
