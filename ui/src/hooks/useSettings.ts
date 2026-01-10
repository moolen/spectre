import React, { createContext, useContext, useEffect, useMemo, useState, useCallback } from 'react';

export type Theme = 'dark' | 'light';
export type TimeFormat = '24h' | '12h';
export type AutoRefreshOption = 'off' | '30s' | '60s' | '300s';

// Default kinds to show in timeline and graph views
export const DEFAULT_KINDS = [
  'Pod',
  'Deployment',
  'ReplicaSet',
  'StatefulSet',
  'HelmRelease'
];

// Common kinds available for selection in settings
export const COMMON_KINDS = [
  // Workloads
  'Pod', 'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob',
  // Networking
  'Service', 'Ingress', 'NetworkPolicy',
  // Config
  'ConfigMap', 'Secret',
  // Storage
  'PersistentVolumeClaim',
  // Flux
  'HelmRelease', 'Kustomization', 'GitRepository', 'HelmRepository',
  // ArgoCD
  'Application',
  // RBAC
  'ServiceAccount', 'Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding',
  // Other
  'Node', 'Namespace', 'Event'
];

export interface SettingsState {
  theme: Theme;
  timeFormat: TimeFormat;
  compactMode: boolean;
  autoRefresh: AutoRefreshOption;
  defaultKinds: string[];
}

interface SettingsContextValue extends SettingsState {
  setTheme: (theme: Theme) => void;
  setTimeFormat: (format: TimeFormat) => void;
  setCompactMode: (enabled: boolean) => void;
  setAutoRefresh: (value: AutoRefreshOption) => void;
  setDefaultKinds: (kinds: string[]) => void;
  formatTime: (date: Date) => string;
}

const DEFAULT_SETTINGS: SettingsState = {
  theme: 'dark',
  timeFormat: '24h',
  compactMode: false,
  autoRefresh: 'off',
  defaultKinds: DEFAULT_KINDS
};

const STORAGE_KEY = 'spectre-settings';

const SettingsContext = createContext<SettingsContextValue | undefined>(undefined);

const loadSettings = (): SettingsState => {
  if (typeof window === 'undefined') {
    return DEFAULT_SETTINGS;
  }

  try {
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (!stored) return DEFAULT_SETTINGS;
    const parsed = JSON.parse(stored);
    return {
      ...DEFAULT_SETTINGS,
      ...parsed
    };
  } catch {
    return DEFAULT_SETTINGS;
  }
};

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettings] = useState<SettingsState>(() => loadSettings());

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
  }, [settings]);

  useEffect(() => {
    const root = document.documentElement;
    root.setAttribute('data-theme', settings.theme);
  }, [settings.theme]);

  const formatter = useMemo(() => {
    return new Intl.DateTimeFormat(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: settings.timeFormat === '12h'
    });
  }, [settings.timeFormat]);

  const formatTime = useCallback((date: Date) => formatter.format(date), [formatter]);

  const value: SettingsContextValue = {
    ...settings,
    setTheme: (theme) => setSettings((prev) => ({ ...prev, theme })),
    setTimeFormat: (format) => setSettings((prev) => ({ ...prev, timeFormat: format })),
    setCompactMode: (enabled) => setSettings((prev) => ({ ...prev, compactMode: enabled })),
    setAutoRefresh: (value) => setSettings((prev) => ({ ...prev, autoRefresh: value })),
    setDefaultKinds: (kinds) => setSettings((prev) => ({ ...prev, defaultKinds: kinds })),
    formatTime
  };

  return React.createElement(SettingsContext.Provider, { value }, children);
};

export const useSettings = (): SettingsContextValue => {
  const context = useContext(SettingsContext);
  if (!context) {
    throw new Error('useSettings must be used within SettingsProvider');
  }
  return context;
};

