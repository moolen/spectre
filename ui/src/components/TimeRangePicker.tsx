import React, { useState, useEffect } from 'react';
import { TimeRange } from '../types';
import spectreMascot from '../spectre-mascot.png';

interface TimeRangePickerProps {
  onConfirm: (range: TimeRange) => void;
  initialRange?: TimeRange;
}

const PRESETS = [
  { label: 'Last 15min', minutes: 15 },
  { label: 'Last 30min', minutes: 30 },
  { label: 'Last 60min', minutes: 60 },
  { label: 'Last 3h', minutes: 180 },
];

export const TimeRangePicker: React.FC<TimeRangePickerProps> = ({ onConfirm, initialRange }) => {
  const [startTime, setStartTime] = useState<string>('');
  const [endTime, setEndTime] = useState<string>('');
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null);

  // Initialize with default (last 2 hours) or provided range
  useEffect(() => {
    if (initialRange) {
      setStartTime(formatDateTimeLocal(initialRange.start));
      setEndTime(formatDateTimeLocal(initialRange.end));
    } else {
      const now = new Date();
      const twoHoursAgo = new Date(now.getTime() - 2 * 60 * 60 * 1000);
      setStartTime(formatDateTimeLocal(twoHoursAgo));
      setEndTime(formatDateTimeLocal(now));
    }
    setSelectedPreset(null); // Reset preset selection when range changes externally
  }, [initialRange]);

  const formatDateTimeLocal = (date: Date): string => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  };

  const handlePreset = (minutes: number) => {
    const end = new Date();
    const start = new Date(end.getTime() - minutes * 60 * 1000);
    setStartTime(formatDateTimeLocal(start));
    setEndTime(formatDateTimeLocal(end));
    setSelectedPreset(minutes);
    // Immediately confirm the preset selection
    onConfirm({ start, end });
  };

  const handleCustomInputChange = () => {
    setSelectedPreset(null); // Clear preset selection when custom inputs are changed
  };

  const handleConfirm = () => {
    const start = new Date(startTime);
    const end = new Date(endTime);

    if (isNaN(start.getTime()) || isNaN(end.getTime())) {
      alert('Please select valid start and end times');
      return;
    }

    if (start >= end) {
      alert('Start time must be before end time');
      return;
    }

    onConfirm({ start, end });
  };

  return (
    <div className="flex items-center justify-center min-h-screen bg-[var(--color-mascot-bg)] transition-colors duration-300 p-4 test-TimeRange">
      <div className="w-full max-w-2xl bg-[var(--color-surface-elevated)] rounded-2xl border border-[var(--color-border-soft)] shadow-2xl p-8 text-[var(--color-text-primary)]">
        {/* Mascot */}
        <div className="flex justify-center mb-6">
          <img
            src={spectreMascot}
            alt="Spectre Mascot"
            style={{ borderRadius: '20px' }}
            className="w-32 h-32 object-contain"
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
                  onClick={() => handlePreset(preset.minutes)}
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
            <div>
              <label className="block text-xs text-[var(--color-text-muted)] mb-1">Start Time</label>
              <input
                type="datetime-local"
                value={startTime}
                onChange={(e) => {
                  setStartTime(e.target.value);
                  handleCustomInputChange();
                }}
                className="w-full px-4 py-2 border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
              />
            </div>
            <div>
              <label className="block text-xs text-[var(--color-text-muted)] mb-1">End Time</label>
              <input
                type="datetime-local"
                value={endTime}
                onChange={(e) => {
                  setEndTime(e.target.value);
                  handleCustomInputChange();
                }}
                className="w-full px-4 py-2 border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
              />
            </div>
          </div>
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

