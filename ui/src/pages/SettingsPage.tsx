import React, { useState, useRef } from 'react';
import { Theme, TimeFormat, useSettings, COMMON_KINDS, DEFAULT_KINDS } from '../hooks/useSettings';
import { TimeInputWithCalendar } from '../components/TimeInputWithCalendar';
import { validateTimeRange } from '../utils/timeParsing';
import { apiClient } from '../services/api';
import { toast } from '../utils/toast';

interface ExportFormData {
  from: string;
  to: string;
  clusterId: string;
  instanceId: string;
}

const SettingsPage: React.FC = () => {
  const {
    theme,
    setTheme,
    timeFormat,
    setTimeFormat,
    compactMode,
    setCompactMode,
    defaultKinds,
    setDefaultKinds,
  } = useSettings();

  // Export state
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const [exportForm, setExportForm] = useState<ExportFormData>({
    from: '1d ago',
    to: 'now',
    clusterId: '',
    instanceId: ''
  });

  // Import state
  const [isImporting, setIsImporting] = useState(false);
  const [importMessage, setImportMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleExport = async () => {
    setIsExporting(true);
    setExportError(null);

    try {
      const validation = validateTimeRange(exportForm.from, exportForm.to);
      if (validation.valid === false) {
        throw new Error(validation.error);
      }

      const { start, end } = validation;
      if (start >= end) {
        throw new Error('Start time must be before end time');
      }

      const blob = await apiClient.exportData({
        from: exportForm.from,
        to: exportForm.to,
        clusterId: exportForm.clusterId || undefined,
        instanceId: exportForm.instanceId || undefined,
      });

      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `spectre-export-${Date.now()}.json.gz`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);

      toast.success('Data exported successfully', 'Your timeline data has been downloaded');
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Export failed';
      setExportError(errorMessage);
      toast.error('Export failed', errorMessage);
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
      const result = await apiClient.importData(file, {
        validate: true,
        overwrite: true,
      });

      const successMessage = `Successfully imported ${result.total_events || 0} events from ${result.imported_files || 0} file(s)`;
      setImportMessage({
        type: 'success',
        text: successMessage
      });
      toast.success('Data imported successfully', successMessage);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Import failed';
      setImportMessage({
        type: 'error',
        text: errorMessage
      });
      toast.error('Import failed', errorMessage);
    } finally {
      setIsImporting(false);
      if (event.target) {
        event.target.value = '';
      }
    }
  };

  return (
    <div className="h-screen overflow-auto bg-[var(--color-app-bg)] text-[var(--color-text-primary)]">
      <div className="max-w-4xl mx-auto p-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold mb-2">Settings</h1>
          <p className="text-[var(--color-text-muted)]">
            Configure your Spectre preferences. Settings are stored locally and persist across sessions.
          </p>
        </div>

        {/* Settings Sections */}
        <div className="space-y-8">
          {/* Appearance Section */}
          <section className="bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)] p-6">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-6">
              Appearance
            </h2>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {/* Theme Setting */}
              <div className="space-y-3">
                <div>
                  <h3 className="text-base font-medium text-[var(--color-text-primary)]">Theme</h3>
                  <p className="text-sm text-[var(--color-text-muted)]">
                    Choose between dark and light mode
                  </p>
                </div>
                <div className="flex gap-2">
                  {(['dark', 'light'] as Theme[]).map((t) => (
                    <button
                      key={t}
                      onClick={() => setTheme(t)}
                      className={`flex-1 px-4 py-3 rounded-lg border text-sm font-medium transition-all ${
                        theme === t
                          ? 'border-brand-500 bg-brand-500/20 text-brand-300 ring-1 ring-brand-500/30'
                          : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] hover:border-brand-500/50 hover:text-[var(--color-text-primary)]'
                      }`}
                    >
                      <div className="flex items-center justify-center gap-2">
                        {t === 'dark' ? (
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                          </svg>
                        ) : (
                          <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                          </svg>
                        )}
                        <span className="capitalize">{t}</span>
                      </div>
                    </button>
                  ))}
                </div>
              </div>

              {/* Compact Mode Setting */}
              <div className="space-y-3">
                <div>
                  <h3 className="text-base font-medium text-[var(--color-text-primary)]">Compact Mode</h3>
                  <p className="text-sm text-[var(--color-text-muted)]">
                    Reduce row height and spacing for dense overviews
                  </p>
                </div>
                <button
                  onClick={() => setCompactMode(!compactMode)}
                  className={`w-full flex items-center justify-between px-4 py-3 rounded-lg border transition-all ${
                    compactMode
                      ? 'border-brand-500 bg-brand-500/20 text-[var(--color-text-primary)]'
                      : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-muted)]'
                  }`}
                >
                  <span className="text-sm font-medium">{compactMode ? 'Enabled' : 'Disabled'}</span>
                  <span
                    className={`inline-flex h-6 w-11 items-center rounded-full p-0.5 transition-colors ${
                      compactMode ? 'bg-brand-500 justify-end' : 'bg-[var(--color-border-soft)] justify-start'
                    }`}
                  >
                    <span className="h-5 w-5 rounded-full bg-white shadow transition-transform" />
                  </span>
                </button>
              </div>
            </div>
          </section>

          {/* Time Display Section */}
          <section className="bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)] p-6">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-6">
              Time Display
            </h2>
            <div className="max-w-md">
              <div className="space-y-3">
                <div>
                  <h3 className="text-base font-medium text-[var(--color-text-primary)]">Time Format</h3>
                  <p className="text-sm text-[var(--color-text-muted)]">
                    Choose how times are displayed throughout the application
                  </p>
                </div>
                <div className="flex gap-2">
                  {(['24h', '12h'] as TimeFormat[]).map((format) => (
                    <button
                      key={format}
                      onClick={() => setTimeFormat(format)}
                      className={`flex-1 px-4 py-3 rounded-lg border text-sm font-medium transition-all ${
                        timeFormat === format
                          ? 'border-brand-500 bg-brand-500/20 text-brand-300 ring-1 ring-brand-500/30'
                          : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] hover:border-brand-500/50 hover:text-[var(--color-text-primary)]'
                      }`}
                    >
                      {format === '24h' ? '24-hour' : '12-hour'}
                    </button>
                  ))}
                </div>
                <p className="text-xs text-[var(--color-text-muted)]">
                  Example: {timeFormat === '24h' ? '14:30:00' : '2:30:00 PM'}
                </p>
              </div>
            </div>
          </section>

          {/* Filters Section */}
          <section className="bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)] p-6">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-6">
              Filters
            </h2>
            <div className="space-y-4">
              <div>
                <h3 className="text-base font-medium text-[var(--color-text-primary)]">Default Resource Kinds</h3>
                <p className="text-sm text-[var(--color-text-muted)] mb-4">
                  Select which resource kinds are shown by default in Timeline and Graph views. 
                  Changing these defaults will reset your current filters.
                </p>
              </div>

              {/* Kind checkboxes grid */}
              <div className="bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)] p-4">
                <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-3">
                  {COMMON_KINDS.map((kind) => (
                    <label
                      key={kind}
                      className="flex items-center gap-2 cursor-pointer group"
                    >
                      <input
                        type="checkbox"
                        checked={defaultKinds.includes(kind)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setDefaultKinds([...defaultKinds, kind]);
                          } else {
                            setDefaultKinds(defaultKinds.filter(k => k !== kind));
                          }
                        }}
                        className="w-4 h-4 rounded border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] 
                                   text-brand-500 focus:ring-brand-500 focus:ring-offset-0 cursor-pointer"
                      />
                      <span className="text-sm text-[var(--color-text-primary)] group-hover:text-brand-400 transition-colors">
                        {kind}
                      </span>
                    </label>
                  ))}
                </div>

                {/* Actions */}
                <div className="mt-4 pt-4 border-t border-[var(--color-border-soft)] flex items-center gap-4">
                  <button
                    onClick={() => setDefaultKinds(DEFAULT_KINDS)}
                    className="px-3 py-1.5 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] 
                               text-sm text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)] transition-colors"
                  >
                    Reset to Defaults
                  </button>
                  <button
                    onClick={() => setDefaultKinds(COMMON_KINDS)}
                    className="px-3 py-1.5 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] 
                               text-sm text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)] transition-colors"
                  >
                    Select All
                  </button>
                  <button
                    onClick={() => setDefaultKinds([])}
                    className="px-3 py-1.5 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] 
                               text-sm text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)] transition-colors"
                  >
                    Clear All
                  </button>
                  <span className="ml-auto text-xs text-[var(--color-text-muted)]">
                    {defaultKinds.length} of {COMMON_KINDS.length} selected
                  </span>
                </div>
              </div>
            </div>
          </section>

          {/* Data Management Section */}
          <section className="bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)] p-6">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-[var(--color-text-muted)] mb-6">
              Data Management
            </h2>
            <div className="space-y-8">
              {/* Export Data */}
              <div className="space-y-4">
                <div>
                  <h3 className="text-base font-medium text-[var(--color-text-primary)] flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                    </svg>
                    Export Data
                  </h3>
                  <p className="text-sm text-[var(--color-text-muted)]">
                    Download timeline data for offline analysis or backup
                  </p>
                </div>

                <div className="bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)] p-4 space-y-4">
                  {/* Time Range */}
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <TimeInputWithCalendar
                        value={exportForm.from}
                        onChange={(value) => setExportForm({ ...exportForm, from: value })}
                        label="Start Date & Time"
                        placeholder="e.g., 1d ago, 2h ago"
                        className="w-full"
                      />
                      <p className="text-xs text-[var(--color-text-muted)] mt-1">Data from this time onwards</p>
                    </div>
                    <div>
                      <TimeInputWithCalendar
                        value={exportForm.to}
                        onChange={(value) => setExportForm({ ...exportForm, to: value })}
                        label="End Date & Time"
                        placeholder="e.g., now, 1h ago"
                        className="w-full"
                      />
                      <p className="text-xs text-[var(--color-text-muted)] mt-1">Data up to this time</p>
                    </div>
                  </div>

                  {/* Metadata (Optional) */}
                  <div className="pt-2 border-t border-[var(--color-border-soft)]">
                    <p className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wide mb-3">
                      Optional Metadata
                    </p>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      <div>
                        <label className="block text-sm font-medium text-[var(--color-text-primary)] mb-2">
                          Cluster ID
                        </label>
                        <input
                          type="text"
                          value={exportForm.clusterId}
                          onChange={(e) => setExportForm({ ...exportForm, clusterId: e.target.value })}
                          placeholder="e.g., production"
                          className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 placeholder-[var(--color-text-muted)]"
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
                          className="w-full px-3 py-2 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:border-brand-500 focus:outline-none focus:ring-1 focus:ring-brand-500 placeholder-[var(--color-text-muted)]"
                        />
                      </div>
                    </div>
                  </div>

                  {/* Export Error */}
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

                  {/* Export Button */}
                  <button
                    onClick={handleExport}
                    disabled={isExporting}
                    className="w-full md:w-auto px-6 py-2.5 rounded-lg bg-brand-500 text-white hover:bg-brand-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 font-medium"
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

              {/* Import Data */}
              <div className="space-y-4">
                <div>
                  <h3 className="text-base font-medium text-[var(--color-text-primary)] flex items-center gap-2">
                    <svg className="w-5 h-5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
                    </svg>
                    Import Data
                  </h3>
                  <p className="text-sm text-[var(--color-text-muted)]">
                    Upload previously exported data to restore or merge timeline events
                  </p>
                </div>

                <div className="bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)] p-4">
                  <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4">
                    <button
                      onClick={handleImportClick}
                      disabled={isImporting}
                      className="px-6 py-2.5 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] text-[var(--color-text-primary)] hover:border-brand-500 hover:bg-[var(--color-surface-active)] transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 font-medium"
                    >
                      {isImporting ? (
                        <>
                          <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                          </svg>
                          <span>Importing...</span>
                        </>
                      ) : (
                        <>
                          <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5" />
                          </svg>
                          <span>Choose File</span>
                        </>
                      )}
                    </button>
                    <input
                      ref={fileInputRef}
                      type="file"
                      accept=".tar,.tar.gz,.gz,application/gzip"
                      onChange={handleFileChange}
                      className="hidden"
                    />
                    <p className="text-sm text-[var(--color-text-muted)]">
                      Accepts .tar, .tar.gz, or .gz files
                    </p>
                  </div>

                  {importMessage && (
                    <div className={`mt-4 text-sm px-3 py-2 rounded-lg ${
                      importMessage.type === 'success'
                        ? 'text-green-400 bg-green-900/20 border border-green-500/40'
                        : 'text-red-400 bg-red-900/20 border border-red-500/40'
                    }`}>
                      {importMessage.text}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </section>
        </div>

        {/* Footer Note */}
        <div className="mt-8 text-center">
          <p className="text-sm text-[var(--color-text-muted)]">
            All settings are stored in your browser's local storage and will persist across sessions.
          </p>
        </div>
      </div>
    </div>
  );
};

export default SettingsPage;
