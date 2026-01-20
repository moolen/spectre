import React, { useState, useRef, useEffect, useMemo } from 'react';
import { TimeInputWithCalendar } from './TimeInputWithCalendar';
import { parseTimeExpression, formatDateTimeForDisplay, isHumanFriendlyExpression } from '../utils/timeParsing';
import { TIMESTAMP_PRESETS, TimestampPreset } from '../constants/timestampPresets';

interface TimestampPickerProps {
  /** Current timestamp expression (e.g., "now", "2h ago", "2025-01-14 10:00") */
  expression: string;
  /** Callback when expression changes */
  onChange: (expression: string) => void;
  /** The resolved Date object (for display) */
  resolvedTimestamp?: Date;
}

/**
 * Timestamp picker component with preset buttons and custom time input.
 * Used to select a point-in-time for viewing historical graph snapshots.
 */
export const TimestampPicker: React.FC<TimestampPickerProps> = ({
  expression,
  onChange,
  resolvedTimestamp,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [customInput, setCustomInput] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Sync custom input with expression when dropdown opens
  useEffect(() => {
    if (isOpen) {
      setCustomInput(expression);
      setValidationError(null);
    }
  }, [isOpen, expression]);

  // Handle click outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Check if current expression matches a preset
  const selectedPreset = useMemo(() => {
    return TIMESTAMP_PRESETS.find(p => p.expression === expression.toLowerCase().trim());
  }, [expression]);

  const isLive = selectedPreset?.isLive ?? false;

  // Display label logic
  const displayLabel = useMemo(() => {
    if (selectedPreset) {
      return selectedPreset.label;
    }
    if (isHumanFriendlyExpression(expression)) {
      return expression;
    }
    if (resolvedTimestamp) {
      return formatDateTimeForDisplay(resolvedTimestamp);
    }
    return expression;
  }, [selectedPreset, expression, resolvedTimestamp]);

  const handlePresetClick = (preset: TimestampPreset) => {
    onChange(preset.expression);
    setIsOpen(false);
  };

  const handleCustomApply = () => {
    const parsed = parseTimeExpression(customInput);
    if (!parsed) {
      setValidationError(`Invalid time: "${customInput}"`);
      return;
    }
    onChange(customInput);
    setIsOpen(false);
  };

  return (
    <div className="relative ml-auto" ref={dropdownRef}>
      {/* Trigger Button */}
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md border
                   border-[var(--color-border-soft)] bg-[var(--color-input-bg)]
                   text-[var(--color-text-primary)] hover:border-brand-500
                   hover:text-brand-200 transition-all min-w-[140px] justify-between"
      >
        <div className="flex items-center gap-2">
          {/* Clock icon */}
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2"
                  d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="truncate text-xs">{displayLabel}</span>
          {/* Live indicator */}
          {isLive && (
            <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" title="Live" />
          )}
        </div>
        {/* Chevron */}
        <svg className={`w-4 h-4 text-[var(--color-text-muted)] transition-transform duration-200
                        ${isOpen ? 'rotate-180' : ''}`}
             fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* Dropdown Panel */}
      {isOpen && (
        <div className="absolute top-full right-0 mt-2 w-72 bg-[var(--color-surface-elevated)]
                        border border-[var(--color-border-soft)] rounded-lg shadow-xl z-[60] p-4
                        animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10">
          {/* Presets */}
          <div className="mb-4">
            <label className="block text-xs font-medium text-[var(--color-text-muted)] mb-2">
              Quick Select
            </label>
            <div className="grid grid-cols-2 gap-2">
              {TIMESTAMP_PRESETS.map((preset) => {
                const isSelected = selectedPreset?.expression === preset.expression;
                return (
                  <button
                    key={preset.expression}
                    onClick={() => handlePresetClick(preset)}
                    className={`px-3 py-1.5 text-xs font-medium rounded-md border transition-all ${
                      isSelected
                        ? 'bg-brand-600 border-brand-500 text-white ring-1 ring-brand-500/20'
                        : 'border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200'
                    }`}
                  >
                    {preset.label}
                    {preset.isLive && isSelected && (
                      <span className="ml-1 w-1.5 h-1.5 inline-block rounded-full bg-green-300" />
                    )}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Custom Input */}
          <div className="mb-4">
            <label className="block text-xs font-medium text-[var(--color-text-muted)] mb-2">
              Custom Time
            </label>
            <TimeInputWithCalendar
              value={customInput}
              onChange={(value) => {
                setCustomInput(value);
                setValidationError(null);
              }}
              onEnter={handleCustomApply}
              placeholder="e.g., 2h ago, 2025-01-14 10:00"
            />
            {validationError && (
              <div className="mt-2 text-xs text-red-400 bg-red-900/20 border border-red-500/40
                              rounded px-2 py-1">
                {validationError}
              </div>
            )}
          </div>

          {/* Apply Button */}
          <div className="flex justify-end">
            <button
              onClick={handleCustomApply}
              className="px-4 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white
                         font-medium rounded-md transition-colors"
            >
              Apply
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
