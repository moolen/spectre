import React, { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { AutoRefreshOption, TimeFormat, useSettings } from '../hooks/useSettings';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';

const AUTO_REFRESH_LABELS: Record<AutoRefreshOption, string> = {
  off: 'Off',
  '30s': '30s',
  '60s': '1m',
  '300s': '5m'
};

interface ExportFormData {
  from: string; // ISO datetime string for datetime-local input
  to: string;   // ISO datetime string for datetime-local input
  includeOpenHour: boolean;
  compression: boolean;
  clusterId: string;
  instanceId: string;
}

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

  const { preset } = usePersistedQuickPreset();

  const [isOpen, setIsOpen] = useState(false);
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const [isImporting, setIsImporting] = useState(false);
  const [importMessage, setImportMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  const menuRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [exportForm, setExportForm] = useState<ExportFormData>(() => {
    const now = new Date();
    const yesterday = new Date(now.getTime() - 86400000); // 24 hours ago

    // Format as datetime-local string (YYYY-MM-DDTHH:mm)
    const formatDatetimeLocal = (date: Date) => {
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      return `${year}-${month}-${day}T${hours}:${minutes}`;
    };

    return {
      from: formatDatetimeLocal(yesterday),
      to: formatDatetimeLocal(now),
      includeOpenHour: true,
      compression: true,
      clusterId: '',
      instanceId: ''
    };
  });

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

  const handleExport = async () => {
    setIsExporting(true);
    setExportError(null);

    try {
      // Convert datetime-local strings to Unix timestamps (seconds)
      const fromTimestamp = Math.floor(new Date(exportForm.from).getTime() / 1000);
      const toTimestamp = Math.floor(new Date(exportForm.to).getTime() / 1000);

      // Validate timestamps
      if (isNaN(fromTimestamp) || isNaN(toTimestamp)) {
        throw new Error('Invalid date/time values');
      }

      if (fromTimestamp >= toTimestamp) {
        throw new Error('Start time must be before end time');
      }

      const params = new URLSearchParams();
      params.append('from', fromTimestamp.toString());
      params.append('to', toTimestamp.toString());
      params.append('include_open_hour', exportForm.includeOpenHour.toString());
      params.append('compression', exportForm.compression.toString());
      if (exportForm.clusterId) params.append('cluster_id', exportForm.clusterId);
      if (exportForm.instanceId) params.append('instance_id', exportForm.instanceId);

      const baseUrl = window.location.origin;
      const response = await fetch(`${baseUrl}/v1/storage/export?${params.toString()}`);

      if (!response.ok) {
        throw new Error(`Export failed: ${response.status} ${response.statusText}`);
      }

      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `spectre-export-${Date.now()}.tar${exportForm.compression ? '.gz' : ''}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      setIsExportModalOpen(false);
    } catch (error) {
      setExportError(error instanceof Error ? error.message : 'Export failed');
    } finally {
      setIsExporting(false);
    }
  };

  const handleImportClick = () => {
    fileInputRef.current?.click();
  };

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) return;

    setIsImporting(true);
    setImportMessage(null);

    try {
      const baseUrl = window.location.origin;
      const response = await fetch(`${baseUrl}/v1/storage/import?validate=true&overwrite=true`, {
        method: 'POST',
        body: file,
        headers: {
          'Content-Type': 'application/gzip'
        }
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(`Import failed: ${response.status} - ${errorText}`);
      }

      const result = await response.json();
      setImportMessage({
        type: 'success',
        text: `Successfully imported ${result.total_events || 0} events from ${result.imported_files || 0} file(s)`
      });
    } catch (error) {
      setImportMessage({
        type: 'error',
        text: error instanceof Error ? error.message : 'Import failed'
      });
    } finally {
      setIsImporting(false);
      // Reset file input
      if (event.target) {
        event.target.value = '';
      }
    }
  };

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
          checked ? 'bg-brand-500 justify-end' : 'bg-[var(--color-border-soft)] justify-start'
        }`}
      >
        <span
          className="h-4 w-4 rounded-full bg-white shadow transition-transform"
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
            <div className="flex gap-2">
              {(['off', '30s', '60s', '300s'] as AutoRefreshOption[]).map((option) => (
                <button
                  key={option}
                  onClick={() => setAutoRefresh(option)}
                  className={`flex-1 px-3 py-1.5 rounded-lg border text-sm font-medium transition-colors ${
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

          <div>
            <div className="text-xs font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-2">
              Data Management
            </div>
            <div className="space-y-2">
              <button
                onClick={() => setIsExportModalOpen(true)}
                className="w-full text-left px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:bg-[var(--color-surface-active)] transition-colors text-sm"
              >
                <div className="flex items-center gap-2">
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                  </svg>
                  <span>Export data</span>
                </div>
              </button>

              <button
                onClick={handleImportClick}
                disabled={isImporting}
                className="w-full text-left px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:bg-[var(--color-surface-active)] transition-colors text-sm disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <div className="flex items-center gap-2">
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
                  </svg>
                  <span>{isImporting ? 'Importing...' : 'Import data'}</span>
                </div>
              </button>

              <input
                ref={fileInputRef}
                type="file"
                accept=".tar,.tar.gz,.gz,application/gzip"
                onChange={handleFileChange}
                className="hidden"
              />

              {importMessage && (
                <div className={`text-xs px-2 py-1 rounded ${
                  importMessage.type === 'success'
                    ? 'text-green-400 bg-green-900/20'
                    : 'text-red-400 bg-red-900/20'
                }`}>
                  {importMessage.text}
                </div>
              )}
            </div>
          </div>

          <p className="text-xs text-[var(--color-text-muted)]">
            Settings are stored locally so your preferences persist across sessions.
          </p>
        </div>
      )}

      {isExportModalOpen && createPortal(
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 bg-black/50 z-[100]"
            onClick={() => setIsExportModalOpen(false)}
          />

          {/* Right-side panel */}
          <div
            className="fixed right-0 bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)] shadow-2xl transform transition-transform duration-300 ease-in-out z-[101] overflow-y-auto flex flex-col text-[var(--color-text-primary)]"
            style={{ width: '480px', top: '73px', bottom: 0 }}
          >
            <div className="p-6 flex-1">
              {/* Header */}
              <div className="flex justify-between items-start mb-6">
                <div>
                  <h2 className="text-xl font-bold">Export Data</h2>
                  <p className="text-sm text-[var(--color-text-muted)] mt-1">
                    Export storage data for offline analysis
                  </p>
                </div>
                <button
                  onClick={() => setIsExportModalOpen(false)}
                  className="text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors p-1 hover:bg-[var(--color-surface-muted)] rounded"
                >
                  <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* Form */}
              <div className="space-y-6">
                {/* Time Range Section */}
                <div>
                  <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold mb-3">
                    Time Range
                  </div>

                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">
                        Start Date & Time
                      </label>
                      <input
                        type="datetime-local"
                        value={exportForm.from}
                        onChange={(e) => setExportForm({ ...exportForm, from: e.target.value })}
                        className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none"
                      />
                      <div className="text-xs text-[var(--color-text-muted)] mt-1">
                        Data from this time onwards
                      </div>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">
                        End Date & Time
                      </label>
                      <input
                        type="datetime-local"
                        value={exportForm.to}
                        onChange={(e) => setExportForm({ ...exportForm, to: e.target.value })}
                        className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none"
                      />
                      <div className="text-xs text-[var(--color-text-muted)] mt-1">
                        Data up to this time
                      </div>
                    </div>
                  </div>
                </div>

                {/* Options Section */}
                <div>
                  <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold mb-3">
                    Options
                  </div>

                  <div className="bg-[var(--color-surface-muted)] rounded-lg p-4 border border-[var(--color-border-soft)] space-y-3">
                    <label className="flex items-center gap-3 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={exportForm.includeOpenHour}
                        onChange={(e) => setExportForm({ ...exportForm, includeOpenHour: e.target.checked })}
                        className="w-4 h-4 rounded border-[var(--color-border-soft)] text-brand-500 focus:ring-brand-500"
                      />
                      <div className="flex-1">
                        <div className="text-sm text-[var(--color-text-primary)]">Include open hour</div>
                        <div className="text-xs text-[var(--color-text-muted)]">Include data from the current hour</div>
                      </div>
                    </label>

                    <label className="flex items-center gap-3 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={exportForm.compression}
                        onChange={(e) => setExportForm({ ...exportForm, compression: e.target.checked })}
                        className="w-4 h-4 rounded border-[var(--color-border-soft)] text-brand-500 focus:ring-brand-500"
                      />
                      <div className="flex-1">
                        <div className="text-sm text-[var(--color-text-primary)]">Enable compression</div>
                        <div className="text-xs text-[var(--color-text-muted)]">Compress archive with gzip</div>
                      </div>
                    </label>
                  </div>
                </div>

                {/* Metadata Section */}
                <div>
                  <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold mb-3">
                    Metadata (Optional)
                  </div>

                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">
                        Cluster ID
                      </label>
                      <input
                        type="text"
                        value={exportForm.clusterId}
                        onChange={(e) => setExportForm({ ...exportForm, clusterId: e.target.value })}
                        placeholder="e.g., production"
                        className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none placeholder-[var(--color-text-muted)]"
                      />
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">
                        Instance ID
                      </label>
                      <input
                        type="text"
                        value={exportForm.instanceId}
                        onChange={(e) => setExportForm({ ...exportForm, instanceId: e.target.value })}
                        placeholder="e.g., instance-1"
                        className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none placeholder-[var(--color-text-muted)]"
                      />
                    </div>
                  </div>
                </div>

                {/* Error Display */}
                {exportError && (
                  <div className="bg-red-900/20 border border-red-500/40 rounded-lg p-3">
                    <div className="flex items-start gap-2">
                      <svg className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <div className="text-sm text-red-400">{exportError}</div>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Footer with actions */}
            <div className="border-t border-[var(--color-border-soft)] p-4 bg-[var(--color-surface-muted)]">
              <div className="flex gap-3">
                <button
                  onClick={() => setIsExportModalOpen(false)}
                  className="flex-1 px-4 py-2 rounded-lg border border-[var(--color-border-soft)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors"
                >
                  Cancel
                </button>
                <button
                  onClick={handleExport}
                  disabled={isExporting}
                  className="flex-1 px-4 py-2 rounded-lg bg-brand-500 text-white hover:bg-brand-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
                >
                  {isExporting ? (
                    <>
                      <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                      </svg>
                      <span>Exporting...</span>
                    </>
                  ) : (
                    <>
                      <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                      </svg>
                      <span>Download Export</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </>,
        document.body
      )}
    </div>
  );
};

