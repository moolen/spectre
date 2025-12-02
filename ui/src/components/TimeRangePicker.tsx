import React, { useState, useEffect } from 'react';
import { TimeRange } from '../types';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { TimeInputWithCalendar } from './TimeInputWithCalendar';
import { validateTimeRange, formatDateTimeForInput } from '../utils/timeParsing';

interface TimeRangePickerProps {
  onConfirm: (range: TimeRange, rawStart?: string, rawEnd?: string) => void;
  initialRange?: TimeRange;
}

const PRESETS = [
  { label: 'Last 15min', minutes: 15, relative: 'now-15m' },
  { label: 'Last 30min', minutes: 30, relative: 'now-30m' },
  { label: 'Last 60min', minutes: 60, relative: 'now-60m' },
  { label: 'Last 3h', minutes: 180, relative: 'now-3h' },
];

export const TimeRangePicker: React.FC<TimeRangePickerProps> = ({ onConfirm, initialRange }) => {
  const [startInput, setStartInput] = useState<string>('');
  const [endInput, setEndInput] = useState<string>('');
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const { savePreset } = usePersistedQuickPreset();

  // Initialize with default (last 2 hours) or provided range
  useEffect(() => {
    if (initialRange) {
      setStartInput(formatDateTimeForInput(initialRange.start));
      setEndInput(formatDateTimeForInput(initialRange.end));
    } else {
      const now = new Date();
      const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000);
      setStartInput(formatDateTimeForInput(twoHoursAgo));
      setEndInput(formatDateTimeForInput(now));
    }
    setSelectedPreset(null); // Reset preset selection when range changes externally
    setValidationError(null);
  }, [initialRange]);

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
    if (validation.error || !validation.start || !validation.end) {
      setValidationError(validation.error || 'Failed to parse time range');
      return;
    }

    // Immediately confirm the preset selection
    onConfirm({ start: validation.start, end: validation.end }, startExpr, endExpr);
  };

  const handleCustomInputChange = () => {
    setSelectedPreset(null); // Clear preset selection when custom inputs are changed
    setValidationError(null);
  };

  const handleConfirm = () => {
    const validation = validateTimeRange(startInput, endInput);

    if (validation.error || !validation.start || !validation.end) {
      setValidationError(validation.error || 'Invalid time range');
      return;
    }

    onConfirm({ start: validation.start, end: validation.end }, startInput, endInput);
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-[var(--color-mascot-bg)] transition-colors duration-300 p-4 test-TimeRange">
      <div className="w-full max-w-2xl bg-[var(--color-surface-elevated)] rounded-2xl border border-[var(--color-border-soft)] shadow-2xl p-8 text-[var(--color-text-primary)]">
        {/* Mascot */}
        <div className="flex justify-center mb-6">
          <img
            src="/ghost.svg"
            alt="Spectre Ghost"
            className="w-32 h-32"
          />
        </div>

        {/* Title and Description */}
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold mb-2">Select Time Range</h1>
          <p className="text-[var(--color-text-muted)] text-sm">
            Choose a time range to explore your Kubernetes resources and their audit timeline.
          </p>
        </div>

        {/* Presets */}
        <div className="mb-6">
          <label className="block text-sm font-medium text-[var(--color-text-muted)] mb-3">
            Quick Presets
          </label>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
              {PRESETS.map((preset) => {
                const isSelected = selectedPreset === preset.minutes;
                return (
                  <button
                    key={preset.minutes}
                    onClick={() => handlePreset(preset.minutes, preset.relative)}
                    className={`px-4 py-2 text-sm font-medium rounded-md border transition-all ${
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
        <div className="mb-6">
          <label className="block text-sm font-medium text-[var(--color-text-muted)] mb-3">
            Custom Range
          </label>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <TimeInputWithCalendar
              value={startInput}
              onChange={(value) => {
                setStartInput(value);
                handleCustomInputChange();
              }}
              label="Start Time"
              placeholder="e.g., 2h ago, 2025-12-02 13:00"
              className="w-full"
            />
            <TimeInputWithCalendar
              value={endInput}
              onChange={(value) => {
                setEndInput(value);
                handleCustomInputChange();
              }}
              label="End Time"
              placeholder="e.g., now, 2025-12-02 15:00"
              className="w-full"
            />
          </div>
          {validationError && (
            <div className="mt-3 text-sm text-red-400 bg-red-900/20 border border-red-500/40 rounded px-3 py-2">
              {validationError}
            </div>
          )}
        </div>

        {/* Confirm Button */}
        <div className="flex justify-end">
          <button
            onClick={handleConfirm}
            className="px-6 py-2 bg-brand-600 hover:bg-brand-700 text-white font-medium rounded-md transition-colors shadow-lg shadow-brand-500/20"
          >
            View Timeline
          </button>
        </div>
      </div>
    </div>
  );
};

