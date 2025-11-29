import React, { useEffect, useRef, useState } from 'react';
import { AutoRefreshOption, TimeFormat, useSettings } from '../hooks/useSettings';

const AUTO_REFRESH_LABELS: Record<AutoRefreshOption, string> = {
  off: 'Off',
  '30s': 'Every 30s',
  '60s': 'Every 1m',
  '300s': 'Every 5m'
};

export const SettingsMenu: React.FC = () => {
  const {
    theme,
    setTheme,
    timeFormat,
    setTimeFormat,
    compactMode,
    setCompactMode,
    autoRefresh,
    setAutoRefresh
  } = useSettings();

  const [isOpen, setIsOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, []);

  const toggleTheme = () => setTheme(theme === 'dark' ? 'light' : 'dark');

  const themeLabel = theme === 'dark' ? 'Dark mode' : 'Light mode';

  const renderToggle = (checked: boolean, onToggle: () => void, label: string, description: string) => (
    <button
      onClick={onToggle}
      className={`flex items-center justify-between w-full px-3 py-2 rounded-lg border transition-colors ${
        checked
          ? 'border-brand-500 bg-[var(--color-surface-active)] text-[var(--color-text-primary)]'
          : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
      }`}
    >
      <div className="text-left">
        <div className="text-sm font-semibold">{label}</div>
        <div className="text-xs text-[var(--color-text-muted)]">{description}</div>
      </div>
      <span
        className={`inline-flex h-5 w-10 items-center rounded-full p-0.5 transition-colors ${
          checked ? 'bg-brand-500' : 'bg-[var(--color-border-soft)]'
        }`}
      >
        <span
          className={`h-4 w-4 rounded-full bg-white shadow transform transition-transform ${
            checked ? 'translate-x-5' : 'translate-x-0'
          }`}
        />
      </span>
    </button>
  );

  return (
    <div ref={menuRef} className="relative">
      <button
        onClick={() => setIsOpen((prev) => !prev)}
        aria-label="Open settings"
        className="p-2 rounded-full border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200 transition-all shadow-sm"
      >
        <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="1.8" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.324.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 011.37.49l1.296 2.247a1.125 1.125 0 01-.26 1.431l-1.003.827c-.293.24-.438.613-.431.992a6.759 6.759 0 010 .255c-.007.378.138.75.43.99l1.005.828c.424.35.534.954.26 1.43l-1.298 2.247a1.125 1.125 0 01-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.57 6.57 0 01-.22.128c-.331.183-.581.495-.644.869l-.213 1.28c-.09.543-.56.941-1.11.941h-2.594c-.55 0-1.02-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 01-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 01-1.369-.49l-1.297-2.247a1.125 1.125 0 01.26-1.431l1.004-.827c.292-.24.437-.613.43-.992a6.932 6.932 0 010-.255c.007-.378-.138-.75-.43-.99l-1.004-.828a1.125 1.125 0 01-.26-1.43l1.297-2.247a1.125 1.125 0 011.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.087.22-.128.332-.183.582-.495.644-.869l.214-1.281z"
          />
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute right-0 mt-3 w-72 rounded-xl border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] shadow-2xl z-50 p-4 space-y-4">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-2">
              Theme & Layout
            </div>
            <div className="space-y-3">
              {renderToggle(
                theme === 'light',
                toggleTheme,
                'Enable light mode',
                'Bright interface that matches the mascot card background.'
              )}
              {renderToggle(
                compactMode,
                () => setCompactMode(!compactMode),
                'Compact resource rows',
                'Reduce row height and spacing for dense overviews.'
              )}
            </div>
          </div>

          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-2">
              Time Display
            </div>
            <div className="flex items-center gap-2">
              {(['24h', '12h'] as TimeFormat[]).map((format) => (
                <button
                  key={format}
                  onClick={() => setTimeFormat(format)}
                  className={`flex-1 px-3 py-2 rounded-lg border text-sm font-medium transition-colors ${
                    timeFormat === format
                      ? 'border-brand-500 bg-[var(--color-surface-active)] text-[var(--color-text-primary)]'
                      : 'border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
                  }`}
                >
                  {format === '24h' ? '24-hour' : '12-hour'}
                </button>
              ))}
            </div>
          </div>

          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-2">
              Auto-refresh
            </div>
            <div className="space-y-2">
              {(['off', '30s', '60s', '300s'] as AutoRefreshOption[]).map((option) => (
                <button
                  key={option}
                  onClick={() => setAutoRefresh(option)}
                  className={`w-full text-left px-3 py-2 rounded-lg border text-sm transition-colors ${
                    autoRefresh === option
                      ? 'border-brand-500 bg-[var(--color-surface-active)] text-[var(--color-text-primary)]'
                      : 'border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
                  }`}
                >
                  {AUTO_REFRESH_LABELS[option]}
                </button>
              ))}
            </div>
          </div>

          <p className="text-xs text-[var(--color-text-muted)]">
            Settings are stored locally so your preferences persist across sessions.
          </p>
        </div>
      )}
    </div>
  );
};

