import React, { useState, useEffect, useRef, useMemo } from 'react';
import { TimeRange } from '../types';
import { useSettings } from '../hooks/useSettings';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { TimeInputWithCalendar } from './TimeInputWithCalendar';
import { validateTimeRange, formatDateTimeForInput } from '../utils/timeParsing';

interface TimeRangeDropdownProps {
  currentRange: TimeRange;
  onConfirm: (range: TimeRange, rawStart?: string, rawEnd?: string) => void;
  rawStart?: string;
  rawEnd?: string;
}

const PRESETS = [
  { label: 'Last 15min', minutes: 15, relative: 'now-15m' },
  { label: 'Last 30min', minutes: 30, relative: 'now-30m' },
  { label: 'Last 60min', minutes: 60, relative: 'now-60m' },
  { label: 'Last 3h', minutes: 180, relative: 'now-3h' },
];

export const TimeRangeDropdown: React.FC<TimeRangeDropdownProps> = ({ currentRange, onConfirm, rawStart, rawEnd }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [startInput, setStartInput] = useState<string>('');
  const [endInput, setEndInput] = useState<string>('');
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const { timeFormat } = useSettings();
  const { preset: persistedPreset, savePreset } = usePersistedQuickPreset();

  const rangeLabel = useMemo(() => {
    const formatDate = (date: Date) => {
      const month = date.toLocaleString('default', { month: 'short' });
      const day = date.getDate();
      const formatter = new Intl.DateTimeFormat(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        hour12: timeFormat === '12h'
      });
      return `${month} ${day}, ${formatter.format(date)}`;
    };

    // If raw expressions are available, use them for the label
    if (rawStart && rawEnd) {
      return `${rawStart} to ${rawEnd}`;
    }
    if (rawStart) {
      return `${rawStart} to ${formatDate(currentRange.end)}`;
    }
    if (rawEnd) {
      return `${formatDate(currentRange.start)} to ${rawEnd}`;
    }

    // Fall back to formatted dates if no raw expressions
    return `${formatDate(currentRange.start)} - ${formatDate(currentRange.end)}`;
  }, [currentRange, timeFormat, rawStart, rawEnd]);

  useEffect(() => {
    // Use raw expressions if available, otherwise format the Date objects
    setStartInput(rawStart || formatDateTimeForInput(currentRange.start));
    setEndInput(rawEnd || formatDateTimeForInput(currentRange.end));
    setValidationError(null);
    // Use persisted preset if available, otherwise check if current range matches a preset
    if (persistedPreset) {
      setSelectedPreset(persistedPreset);
    } else {
      const now = new Date();
      const rangeDuration = now.getTime() - currentRange.start.getTime();
      const matchingPreset = PRESETS.find(p => {
        const presetDuration = p.minutes * 60 * 1000;
        // Allow 1 second tolerance for matching
        return Math.abs(rangeDuration - presetDuration) < 1000;
      });
      setSelectedPreset(matchingPreset ? matchingPreset.minutes : null);
    }
  }, [currentRange, persistedPreset, rawStart, rawEnd]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handlePreset = (minutes: number, relative: string) => {
    const startExpr = relative;
    const endExpr = 'now';

    setStartInput(startExpr);
    setEndInput(endExpr);
    setSelectedPreset(minutes);
    setValidationError(null);

    // Save preset to localStorage
    savePreset(minutes);

    // Parse to Date objects for onConfirm
    const validation = validateTimeRange(startExpr, endExpr);
    if (validation.valid === false) {
      setValidationError(validation.error);
      return;
    }

    // Immediately confirm the preset selection and close dropdown
    onConfirm({ start: validation.start, end: validation.end }, startExpr, endExpr);
    setIsOpen(false);
  };

  const handleCustomInputChange = () => {
    setSelectedPreset(null); // Clear preset selection when custom inputs are changed
    setValidationError(null);
  };

  const handleConfirm = () => {
    const validation = validateTimeRange(startInput, endInput);

    if (validation.valid === false) {
      setValidationError(validation.error);
      return;
    }

    onConfirm({ start: validation.start, end: validation.end }, startInput, endInput);
    setIsOpen(false);
  };

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md border border-[var(--color-border-soft)] bg-[var(--color-input-bg)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200 transition-all min-w-[200px] justify-between"
      >
        <div className="flex items-center gap-2">
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="truncate text-xs">{rangeLabel}</span>
        </div>
        <svg
          className={`w-4 h-4 text-[var(--color-text-muted)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-96 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl z-50 p-4 animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10">
          {/* Presets */}
          <div className="mb-4">
            <label className="block text-xs font-medium text-[var(--color-text-muted)] mb-2">
              Quick Presets
            </label>
            <div className="grid grid-cols-2 gap-2">
              {PRESETS.map((preset) => {
                const isSelected = selectedPreset === preset.minutes;
                return (
                  <button
                    key={preset.minutes}
                    onClick={() => handlePreset(preset.minutes, preset.relative)}
                    className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-all ${
                      isSelected
                        ? 'bg-brand-600 border-brand-500 text-white ring-1 ring-brand-500/20'
                        : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200'
                    }`}
                  >
                    {preset.label}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Custom Date/Time Inputs */}
          <div className="mb-4">
            <label className="block text-xs font-medium text-[var(--color-text-muted)] mb-2">
              Custom Range
            </label>
            <div className="space-y-2">
              <TimeInputWithCalendar
                value={startInput}
                onChange={(value) => {
                  setStartInput(value);
                  handleCustomInputChange();
                }}
                label="Start Time"
                placeholder="e.g., 2h ago, 2025-12-02 13:00"
              />
              <TimeInputWithCalendar
                value={endInput}
                onChange={(value) => {
                  setEndInput(value);
                  handleCustomInputChange();
                }}
                label="End Time"
                placeholder="e.g., now, 2025-12-02 15:00"
              />
            </div>
            {validationError && (
              <div className="mt-2 text-xs text-red-400 bg-red-900/20 border border-red-500/40 rounded px-2 py-1">
                {validationError}
              </div>
            )}
          </div>

          {/* Confirm Button */}
          <div className="flex justify-end">
            <button
              onClick={handleConfirm}
              className="px-4 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white font-medium rounded-md transition-colors"
            >
              Apply
            </button>
          </div>
        </div>
      )}
    </div>
  );
};

