import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import TimelinePage from './TimelinePage';

const {
  detectWatcherEnabledMock,
  getWatcherEnabledMock,
  getMetadataMock,
} = vi.hoisted(() => ({
  detectWatcherEnabledMock: vi.fn(),
  getWatcherEnabledMock: vi.fn(),
  getMetadataMock: vi.fn(),
}));

vi.mock('../services/api', () => ({
  apiClient: {
    getMetadata: getMetadataMock,
  },
  detectWatcherEnabled: detectWatcherEnabledMock,
  getWatcherEnabled: getWatcherEnabledMock,
}));

vi.mock('../hooks/useTimeline', () => ({
  useTimeline: () => ({
    resources: [],
    loading: false,
    loadingMore: false,
    error: null,
    refresh: vi.fn(),
    totalCount: 0,
    loadedCount: 0,
    hasMore: false,
    loadMore: vi.fn(),
  }),
}));

vi.mock('../hooks/useMetadata', () => ({
  useMetadata: () => ({
    namespaces: [],
    kinds: [],
    loading: false,
    error: null,
  }),
}));

vi.mock('../hooks/usePersistedFilters', () => ({
  usePersistedFilters: () => ({
    kinds: [],
    namespaces: [],
    setKinds: vi.fn(),
    setNamespaces: vi.fn(),
  }),
}));

vi.mock('../hooks/usePersistedQuickPreset', () => ({
  usePersistedQuickPreset: () => ({
    preset: null,
    savePreset: vi.fn(),
    clearPreset: vi.fn(),
  }),
}));

vi.mock('../hooks/useSettings', () => ({
  useSettings: () => ({
    theme: 'dark',
    timeFormat: '24h',
    compactMode: false,
    autoRefresh: 'off',
    defaultKinds: [],
    hideInactiveReplicaSets: true,
    setTheme: vi.fn(),
    setTimeFormat: vi.fn(),
    setCompactMode: vi.fn(),
    setAutoRefresh: vi.fn(),
    setDefaultKinds: vi.fn(),
    setHideInactiveReplicaSets: vi.fn(),
    formatTime: vi.fn(),
  }),
}));

vi.mock('../components/FilterBar', () => ({
  FilterBar: () => <div data-testid="filter-bar" />,
}));

vi.mock('../components/Timeline', () => ({
  Timeline: () => <div data-testid="timeline" />,
}));

vi.mock('../components/DetailPanel', () => ({
  DetailPanel: () => <div data-testid="detail-panel" />,
}));

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.search}</div>;
}

function renderTimelinePage(initialEntry: string) {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route
          path="/"
          element={(
            <>
              <TimelinePage />
              <LocationProbe />
            </>
          )}
        />
      </Routes>
    </MemoryRouter>,
  );
}

describe('TimelinePage startup behavior', () => {
  beforeEach(() => {
    detectWatcherEnabledMock.mockResolvedValue(true);
    getWatcherEnabledMock.mockReturnValue(true);
    getMetadataMock.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('initializes the URL from metadata bounds when watcher is disabled and no range is set', async () => {
    detectWatcherEnabledMock.mockResolvedValue(false);
    getWatcherEnabledMock.mockReturnValue(false);
    getMetadataMock.mockResolvedValue({
      namespaces: [],
      kinds: [],
      timeRange: {
        earliest: 1735689600,
        latest: 1735693200,
      },
    });

    renderTimelinePage('/');

    await waitFor(() => {
      const params = new URLSearchParams(screen.getByTestId('location').textContent ?? '');
      expect(params.get('start')).toBe('2025-01-01T00:00:00.000Z');
      expect(params.get('end')).toBe('2025-01-01T01:00:00.000Z');
    });

    expect(getMetadataMock).toHaveBeenCalledTimes(1);
    expect(getMetadataMock).toHaveBeenCalledWith();
  });

  it('preserves explicit URL range parameters when watcher is disabled', async () => {
    detectWatcherEnabledMock.mockResolvedValue(false);
    getWatcherEnabledMock.mockReturnValue(false);

    renderTimelinePage('/?start=2025-01-01T00:00:00.000Z&end=2025-01-01T01:00:00.000Z');

    await waitFor(() => {
      expect(screen.getByTestId('filter-bar')).toBeInTheDocument();
    });

    const params = new URLSearchParams(screen.getByTestId('location').textContent ?? '');
    expect(params.get('start')).toBe('2025-01-01T00:00:00.000Z');
    expect(params.get('end')).toBe('2025-01-01T01:00:00.000Z');
    expect(getMetadataMock).not.toHaveBeenCalled();
  });

  it('keeps the relative default range when watcher is enabled', async () => {
    renderTimelinePage('/');

    await waitFor(() => {
      const params = new URLSearchParams(screen.getByTestId('location').textContent ?? '');
      expect(params.get('start')).toBe('now-30m');
      expect(params.get('end')).toBe('now');
    });

    expect(getMetadataMock).not.toHaveBeenCalled();
  });
});
