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
    <div className="relative" ref={dropdownRef}>
      {/* Trigger Button */}
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`ui-control ${isOpen ? '!border-[var(--color-accent-line)] !text-[var(--color-text-primary)]' : ''}`}
      >
        <svg className="w-3.5 h-3.5 text-[var(--color-text-faint)]" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24" strokeLinecap="round">
          <circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" />
        </svg>
        <span className="truncate tabular-nums">{displayLabel}</span>
        {/* Live indicator */}
        {isLive && (
          <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] shadow-[0_0_0_3px_var(--color-accent-soft)]" title="Live" />
        )}
        <svg className={`w-3.5 h-3.5 text-[var(--color-text-faint)] transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
             fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {/* Dropdown Panel */}
      {isOpen && (
        <div className="ui-popover absolute top-full right-0 mt-2 w-72 z-[60] p-4">
          {/* Presets */}
          <div className="mb-4">
            <label className="block ui-section-label mb-2">
              Quick Select
            </label>
            <div className="grid grid-cols-2 gap-2">
              {TIMESTAMP_PRESETS.map((preset) => {
                const isSelected = selectedPreset?.expression === preset.expression;
                return (
                  <button
                    key={preset.expression}
                    onClick={() => handlePresetClick(preset)}
                    className={`h-8 px-3 text-xs font-medium rounded-md border transition-colors text-left ${
                      isSelected
                        ? 'border-[var(--color-accent-line)] bg-[var(--color-accent-soft)] text-[var(--color-text-primary)]'
                        : 'border-[var(--color-border-strong)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)]'
                    }`}
                  >
                    {preset.label}
                    {preset.isLive && isSelected && (
                      <span className="ml-1.5 w-1.5 h-1.5 inline-block rounded-full bg-[var(--color-accent)]" />
                    )}
                  </button>
                );
              })}
            </div>
          </div>

          {/* Custom Input */}
          <div className="mb-4">
            <label className="block ui-section-label mb-2">
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
              <div className="mt-2 text-xs rounded-md px-2.5 py-1.5 bg-[var(--color-status-err-soft)] text-[var(--color-status-err-text)]">
                {validationError}
              </div>
            )}
          </div>

          {/* Apply Button */}
          <div className="flex justify-end">
            <button type="button" onClick={handleCustomApply} className="ui-btn-primary">
              Apply
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
