import React, { useState, useRef, useCallback } from 'react';
import { Theme, TimeFormat, useSettings, COMMON_KINDS, DEFAULT_KINDS } from '../hooks/useSettings';
import { TimeInputWithCalendar } from '../components/TimeInputWithCalendar';
import { validateTimeRange } from '../utils/timeParsing';
import { apiClient } from '../services/api';
import { toast } from '../utils/toast';
import { PageHeader } from '../components/Common/PageHeader';
import { Icon, IconName } from '../components/Common/Icon';

const SECTIONS: Array<{ id: string; label: string; icon: IconName }> = [
  { id: 'appearance', label: 'Appearance', icon: 'sliders' },
  { id: 'filters', label: 'Default filters', icon: 'filter' },
  { id: 'data', label: 'Data', icon: 'database' },
];

/** Label + description on the left, control on the right, separated by hairlines. */
const SettingRow: React.FC<{ title: string; description?: React.ReactNode; children?: React.ReactNode }> = ({
  title,
  description,
  children,
}) => (
  <div className="flex items-center gap-6 py-4 border-t border-[var(--color-border-soft)]">
    <div className="flex-1 min-w-0">
      <div className="text-[13px] font-medium text-[var(--color-text-primary)]">{title}</div>
      {description && <div className="text-xs text-[var(--color-text-faint)] mt-0.5 leading-relaxed">{description}</div>}
    </div>
    {children}
  </div>
);

const Switch: React.FC<{ checked: boolean; onChange: (checked: boolean) => void; label: string }> = ({ checked, onChange, label }) => (
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    onClick={() => onChange(!checked)}
    className="ui-switch"
  />
);

const SectionHeading: React.FC<{ id: string; title: string; description: string; sectionRef: (el: HTMLElement | null) => void; children: React.ReactNode }> = ({
  id,
  title,
  description,
  sectionRef,
  children,
}) => (
  <section id={`settings-${id}`} ref={sectionRef} className="scroll-mt-8 [&+&]:mt-10">
    <h2 className="text-[15px] font-semibold tracking-[-0.01em]">{title}</h2>
    <p className="text-[12.5px] text-[var(--color-text-faint)] mt-1 mb-5">{description}</p>
    <div className="border-b border-[var(--color-border-soft)]">{children}</div>
  </section>
);

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
    hideInactiveReplicaSets,
    setHideInactiveReplicaSets,
  } = useSettings();

  // Export state
  const [isExporting, setIsExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const [exportForm, setExportForm] = useState<ExportFormData>({
    from: '1d ago',
    to: 'now',
    clusterId: '',
    instanceId: '',
  });

  // Import state
  const [isImporting, setIsImporting] = useState(false);
  const [importMessage, setImportMessage] = useState<{
    type: 'success' | 'error';
    text: string;
  } | null>(null);
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
        text: successMessage,
      });
      toast.success('Data imported successfully', successMessage);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Import failed';
      setImportMessage({
        type: 'error',
        text: errorMessage,
      });
      toast.error('Import failed', errorMessage);
    } finally {
      setIsImporting(false);
      if (event.target) {
        event.target.value = '';
      }
    }
  };

  const scrollRef = useRef<HTMLDivElement>(null);
  const sectionRefs = useRef<Record<string, HTMLElement | null>>({});
  const [activeSection, setActiveSection] = useState(SECTIONS[0].id);

  const scrollToSection = (id: string) => {
    setActiveSection(id);
    sectionRefs.current[id]?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  // Highlight the section closest to the top of the scroll area
  const handleScroll = useCallback(() => {
    const container = scrollRef.current;
    if (!container) return;
    const top = container.getBoundingClientRect().top;
    let current = SECTIONS[0].id;
    for (const section of SECTIONS) {
      const el = sectionRefs.current[section.id];
      if (el && el.getBoundingClientRect().top - top <= 80) {
        current = section.id;
      }
    }
    setActiveSection(current);
  }, []);

  return (
    <div className="h-screen flex flex-col bg-[var(--color-app-bg)] text-[var(--color-text-primary)]">
      <PageHeader
        title="Settings"
        actions={<span className="text-xs text-[var(--color-text-faint)]">Stored in this browser</span>}
      />

      <div className="flex-1 flex min-h-0">
        {/* Section navigation */}
        <nav className="w-[220px] flex-none border-r border-[var(--color-border-soft)] px-3 py-5 space-y-0.5" aria-label="Settings sections">
          {SECTIONS.map(section => {
            const isActive = activeSection === section.id;
            return (
              <button
                key={section.id}
                type="button"
                onClick={() => scrollToSection(section.id)}
                aria-current={isActive ? 'true' : undefined}
                className={`w-full h-8 px-2.5 flex items-center gap-2.5 rounded-md text-[13px] text-left transition-colors ${
                  isActive
                    ? 'bg-[var(--color-surface-active)] text-[var(--color-text-primary)]'
                    : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)]'
                }`}
              >
                <Icon
                  name={section.icon}
                  size={15}
                  className={isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-faint)]'}
                />
                {section.label}
              </button>
            );
          })}
        </nav>

        {/* Settings content */}
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          data-testid="settings-scroll"
          className="flex-1 overflow-auto"
        >
          <div className="max-w-[760px] px-12 pt-8 pb-16">
            <SectionHeading
              id="appearance"
              title="Appearance"
              description="How Spectre looks on this device."
              sectionRef={el => { sectionRefs.current.appearance = el; }}
            >
              <SettingRow title="Theme" description="Dark is the default; light is available for bright rooms.">
                <div className="ui-segmented" role="group" aria-label="Theme">
                  {(['dark', 'light'] as Theme[]).map((t) => (
                    <button key={t} type="button" aria-pressed={theme === t} onClick={() => setTheme(t)}>
                      <Icon name={t === 'dark' ? 'moon' : 'sun'} size={13} />
                      <span className="capitalize">{t}</span>
                    </button>
                  ))}
                </div>
              </SettingRow>
              <SettingRow title="Compact rows" description="Reduce row height on the timeline for dense overviews.">
                <Switch checked={compactMode} onChange={setCompactMode} label="Compact rows" />
              </SettingRow>
              <SettingRow
                title="Time format"
                description={`Example: ${timeFormat === '24h' ? '14:30:00' : '2:30:00 PM'}`}
              >
                <div className="ui-segmented" role="group" aria-label="Time format">
                  {(['24h', '12h'] as TimeFormat[]).map((format) => (
                    <button key={format} type="button" aria-pressed={timeFormat === format} onClick={() => setTimeFormat(format)}>
                      {format === '24h' ? '24-hour' : '12-hour'}
                    </button>
                  ))}
                </div>
              </SettingRow>
            </SectionHeading>

            <SectionHeading
              id="filters"
              title="Default filters"
              description="Applied when you open the Timeline or Graph. Changing them resets your current filters."
              sectionRef={el => { sectionRefs.current.filters = el; }}
            >
              <div className="py-4 border-t border-[var(--color-border-soft)]">
                <div className="flex items-center gap-4">
                  <div className="flex-1">
                    <div className="text-[13px] font-medium">Resource kinds</div>
                    <div className="text-xs text-[var(--color-text-faint)] mt-0.5 tabular-nums">
                      {defaultKinds.length} of {COMMON_KINDS.length} selected
                    </div>
                  </div>
                  <button type="button" onClick={() => setDefaultKinds(COMMON_KINDS)} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
                    Select all
                  </button>
                  <button type="button" onClick={() => setDefaultKinds([])} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
                    Clear all
                  </button>
                  <button type="button" onClick={() => setDefaultKinds(DEFAULT_KINDS)} className="text-xs text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]">
                    Reset to defaults
                  </button>
                </div>
                <div className="flex flex-wrap gap-1.5 mt-3">
                  {COMMON_KINDS.map((kind) => {
                    const checked = defaultKinds.includes(kind);
                    return (
                      <button
                        key={kind}
                        type="button"
                        role="checkbox"
                        aria-checked={checked}
                        onClick={() => {
                          if (checked) {
                            setDefaultKinds(defaultKinds.filter((k) => k !== kind));
                          } else {
                            setDefaultKinds([...defaultKinds, kind]);
                          }
                        }}
                        className={`h-[26px] inline-flex items-center gap-1.5 px-2.5 rounded-md border text-xs transition-colors ${
                          checked
                            ? 'border-[var(--color-accent-line)] bg-[var(--color-accent-soft)] text-[var(--color-text-primary)]'
                            : 'border-[var(--color-border-strong)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
                        }`}
                      >
                        {checked && <Icon name="check" size={12} strokeWidth={2.4} className="text-[var(--color-accent-strong)]" />}
                        {kind}
                      </button>
                    );
                  })}
                </div>
              </div>
              <SettingRow
                title="Hide inactive ReplicaSets"
                description="Hide ReplicaSets with spec.replicas = 0 in the Graph view. These are old ReplicaSets kept for rollback that no longer run pods."
              >
                <Switch checked={hideInactiveReplicaSets} onChange={setHideInactiveReplicaSets} label="Hide inactive ReplicaSets" />
              </SettingRow>
            </SectionHeading>

            <SectionHeading
              id="data"
              title="Data"
              description="Export timeline data for offline analysis or backup, or import a previous export."
              sectionRef={el => { sectionRefs.current.data = el; }}
            >
              {/* Export */}
              <div className="py-4 border-t border-[var(--color-border-soft)]">
                <div className="text-[13px] font-medium">Export data</div>
                <div className="text-xs text-[var(--color-text-faint)] mt-0.5">Download timeline data for the selected range.</div>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5 mt-4">
                  <TimeInputWithCalendar
                    value={exportForm.from}
                    onChange={(value) => setExportForm({ ...exportForm, from: value })}
                    label="From"
                    placeholder="e.g., 1d ago, 2h ago"
                    className="w-full"
                  />
                  <TimeInputWithCalendar
                    value={exportForm.to}
                    onChange={(value) => setExportForm({ ...exportForm, to: value })}
                    label="Until"
                    placeholder="e.g., now, 1h ago"
                    className="w-full"
                  />
                  <div>
                    <label htmlFor="export-cluster-id" className="block text-xs text-[var(--color-text-secondary)] mb-1.5">
                      Cluster ID <span className="text-[var(--color-text-faint)]">optional</span>
                    </label>
                    <input
                      id="export-cluster-id"
                      type="text"
                      value={exportForm.clusterId}
                      onChange={(e) =>
                        setExportForm({ ...exportForm, clusterId: e.target.value })
                      }
                      placeholder="e.g., production"
                      className="ui-input"
                    />
                  </div>
                  <div>
                    <label htmlFor="export-instance-id" className="block text-xs text-[var(--color-text-secondary)] mb-1.5">
                      Instance ID <span className="text-[var(--color-text-faint)]">optional</span>
                    </label>
                    <input
                      id="export-instance-id"
                      type="text"
                      value={exportForm.instanceId}
                      onChange={(e) =>
                        setExportForm({ ...exportForm, instanceId: e.target.value })
                      }
                      placeholder="e.g., instance-1"
                      className="ui-input"
                    />
                  </div>
                </div>

                {/* Export Error */}
                {exportError && (
                  <div className="mt-3 flex items-start gap-2 rounded-lg px-3 py-2 text-xs bg-[var(--color-status-err-soft)] text-[var(--color-status-err-text)]">
                    <Icon name="alert" size={14} />
                    <span>{exportError}</span>
                  </div>
                )}

                <button
                  type="button"
                  onClick={handleExport}
                  disabled={isExporting}
                  className="ui-btn-primary mt-4"
                >
                  {isExporting ? (
                    <>
                      <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                      <span>Exporting...</span>
                    </>
                  ) : (
                    <>
                      <Icon name="download" size={14} strokeWidth={2} />
                      <span>Download Export</span>
                    </>
                  )}
                </button>
              </div>

              {/* Import */}
              <div className="py-4 border-t border-[var(--color-border-soft)]">
                <div className="flex items-center gap-6">
                  <div className="flex-1">
                    <div className="text-[13px] font-medium">Import data</div>
                    <div className="text-xs text-[var(--color-text-faint)] mt-0.5">
                      Restore or merge previously exported events. Accepts .tar, .tar.gz, or .gz files.
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={handleImportClick}
                    disabled={isImporting}
                    className="ui-control disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isImporting ? (
                      <>
                        <span className="w-3.5 h-3.5 rounded-full border-2 border-current border-t-transparent animate-spin" />
                        <span>Importing...</span>
                      </>
                    ) : (
                      <>
                        <Icon name="upload" size={14} />
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
                </div>

                {importMessage && (
                  <div
                    className={`mt-3 text-xs px-3 py-2 rounded-lg ${
                      importMessage.type === 'success'
                        ? 'bg-[var(--color-status-ok-soft)] text-[var(--color-status-ok-text)]'
                        : 'bg-[var(--color-status-err-soft)] text-[var(--color-status-err-text)]'
                    }`}
                  >
                    {importMessage.text}
                  </div>
                )}
              </div>
            </SectionHeading>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SettingsPage;
