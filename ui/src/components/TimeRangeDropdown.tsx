import React, { useState, useEffect, useRef, useMemo } from 'react';
import { TimeRange } from '../types';
import { useSettings } from '../hooks/useSettings';

interface TimeRangeDropdownProps {
  currentRange: TimeRange;
  onConfirm: (range: TimeRange) => void;
}

const PRESETS = [
  { label: 'Last 15min', minutes: 15 },
  { label: 'Last 30min', minutes: 30 },
  { label: 'Last 60min', minutes: 60 },
  { label: 'Last 3h', minutes: 180 },
];

const formatDateTimeLocal = (date: Date): string => {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day}T${hours}:${minutes}`;
};

export const TimeRangeDropdown: React.FC<TimeRangeDropdownProps> = ({ currentRange, onConfirm }) => {
  const [isOpen, setIsOpen] = useState(false);
  const [startTime, setStartTime] = useState<string>('');
  const [endTime, setEndTime] = useState<string>('');
  const [selectedPreset, setSelectedPreset] = useState<number | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const { timeFormat } = useSettings();

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
    return `${formatDate(currentRange.start)} - ${formatDate(currentRange.end)}`;
  }, [currentRange, timeFormat]);

  useEffect(() => {
    setStartTime(formatDateTimeLocal(currentRange.start));
    setEndTime(formatDateTimeLocal(currentRange.end));
    // Check if current range matches a preset
    const now = new Date();
    const rangeDuration = now.getTime() - currentRange.start.getTime();
    const matchingPreset = PRESETS.find(p => {
      const presetDuration = p.minutes * 60 * 1000;
      // Allow 1 second tolerance for matching
      return Math.abs(rangeDuration - presetDuration) < 1000;
    });
    setSelectedPreset(matchingPreset ? matchingPreset.minutes : null);
  }, [currentRange]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handlePreset = (minutes: number) => {
    const end = new Date();
    const start = new Date(end.getTime() - minutes * 60 * 1000);
    setStartTime(formatDateTimeLocal(start));
    setEndTime(formatDateTimeLocal(end));
    setSelectedPreset(minutes);
    // Immediately confirm the preset selection and close dropdown
    onConfirm({ start, end });
    setIsOpen(false);
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
                    onClick={() => handlePreset(preset.minutes)}
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
              <div>
                <label className="block text-xs text-[var(--color-text-muted)] mb-1">Start Time</label>
                <input
                  type="datetime-local"
                  value={startTime}
                  onChange={(e) => {
                    setStartTime(e.target.value);
                    handleCustomInputChange();
                  }}
                  className="w-full px-3 py-1.5 text-sm border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
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
                  className="w-full px-3 py-1.5 text-sm border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                />
              </div>
            </div>
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

