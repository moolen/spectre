import React, { useState, useEffect, useRef, useMemo } from 'react';
import { TimeRange } from '../types';
import { useSettings } from '../hooks/useSettings';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { TimeInputWithCalendar } from './TimeInputWithCalendar';
import { validateTimeRange, formatDateTimeForInput } from '../utils/timeParsing';
import { TIME_RANGE_PRESETS } from '../constants/timeRangePresets';

interface TimeRangeDropdownProps {
  currentRange: TimeRange;
  onConfirm: (range: TimeRange, rawStart?: string, rawEnd?: string) => void;
  rawStart?: string;
  rawEnd?: string;
}

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
      const matchingPreset = TIME_RANGE_PRESETS.find(p => {
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
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`ui-control max-w-[300px] ${isOpen ? '!border-[var(--color-accent-line)] !text-[var(--color-text-primary)]' : ''}`}
        title={rangeLabel}
      >
        <svg className="w-3.5 h-3.5 flex-shrink-0 text-[var(--color-text-faint)]" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
          <rect x="3" y="4" width="18" height="18" rx="2" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M16 2v4M8 2v4M3 10h18" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <span className="truncate tabular-nums">{rangeLabel}</span>
        <svg
          className={`w-3.5 h-3.5 flex-shrink-0 text-[var(--color-text-faint)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="ui-popover absolute top-full right-0 mt-2 w-[360px] z-[60] p-4">
          {/* Presets */}
          <div className="mb-4">
            <label className="block ui-section-label mb-2">
              Quick Presets
            </label>
            <div className="grid grid-cols-2 gap-1.5">
              {TIME_RANGE_PRESETS.map((preset) => {
                const isSelected = selectedPreset === preset.minutes;
                return (
                  <button
                    key={preset.minutes}
                    type="button"
                    onClick={() => handlePreset(preset.minutes, preset.relative)}
                    className={`h-8 px-3 text-xs font-medium rounded-md border transition-colors text-left ${
                      isSelected
                        ? 'border-[var(--color-accent-line)] bg-[var(--color-accent-soft)] text-[var(--color-text-primary)]'
                        : 'border-[var(--color-border-strong)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)]'
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
            <label className="block ui-section-label mb-2">
              Custom Range
            </label>
            <div className="space-y-2.5">
              <TimeInputWithCalendar
                value={startInput}
                onChange={(value) => {
                  setStartInput(value);
                  handleCustomInputChange();
                }}
                onEnter={handleConfirm}
                label="Start Time"
                placeholder="e.g., 2h ago, 2025-12-02 13:00"
              />
              <TimeInputWithCalendar
                value={endInput}
                onChange={(value) => {
                  setEndInput(value);
                  handleCustomInputChange();
                }}
                onEnter={handleConfirm}
                label="End Time"
                placeholder="e.g., now, 2025-12-02 15:00"
              />
            </div>
            {validationError && (
              <div className="mt-2 text-xs rounded-md px-2.5 py-1.5 bg-[var(--color-status-err-soft)] text-[var(--color-status-err-text)]">
                {validationError}
              </div>
            )}
          </div>

          {/* Confirm Button */}
          <div className="flex justify-end">
            <button type="button" onClick={handleConfirm} className="ui-btn-primary">
              Apply
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
