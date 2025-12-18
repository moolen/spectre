import React, { useState, useRef, useEffect } from 'react';

interface TimeInputWithCalendarProps {
  value: string;
  onChange: (value: string) => void;
  onDateSelected?: (date: Date) => void;
  onEnter?: () => void;
  placeholder?: string;
  label?: string;
  className?: string;
}

export const TimeInputWithCalendar: React.FC<TimeInputWithCalendarProps> = ({
  value,
  onChange,
  onDateSelected,
  onEnter,
  placeholder = 'e.g., now, 2h ago, 2024-01-01',
  label,
  className = '',
}) => {
  const [isCalendarOpen, setIsCalendarOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const calendarRef = useRef<HTMLDivElement>(null);

  // Handle click outside to close calendar
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        calendarRef.current &&
        !calendarRef.current.contains(event.target as Node) &&
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsCalendarOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsCalendarOpen(false);
      }
    };

    if (isCalendarOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      document.addEventListener('keydown', handleEscape);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isCalendarOpen]);

  const handleCalendarSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const dateValue = e.target.value;
    if (dateValue) {
      const date = new Date(dateValue);
      if (!isNaN(date.getTime())) {
        // Format as YYYY-MM-DD HH:mm for display
        const formatted = formatDateTimeForDisplay(date);
        onChange(formatted);
        if (onDateSelected) {
          onDateSelected(date);
        }
        setIsCalendarOpen(false);
      }
    }
  };

  const formatDateTimeForDisplay = (date: Date): string => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day} ${hours}:${minutes}`;
  };

  const formatDateTimeLocal = (date: Date): string => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  };

  // Try to parse current value to pre-populate calendar
  const getCalendarValue = (): string => {
    try {
      const date = new Date(value);
      if (!isNaN(date.getTime())) {
        return formatDateTimeLocal(date);
      }
    } catch {
      // ignore
    }
    return formatDateTimeLocal(new Date());
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && onEnter) {
      e.preventDefault();
      onEnter();
    }
  };

  return (
    <div ref={containerRef} className="relative">
      {label && (
        <label className="block text-xs text-[var(--color-text-muted)] mb-1">
          {label}
        </label>
      )}
      <div className="relative flex items-center">
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className={`w-full px-3 py-1.5 pr-10 text-sm border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 ${className}`}
        />
        <button
          type="button"
          onClick={() => setIsCalendarOpen(!isCalendarOpen)}
          className="absolute right-2 p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors"
          aria-label="Open calendar"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
        </button>
      </div>

      {isCalendarOpen && (
        <div
          ref={calendarRef}
          className="absolute z-50 mt-1 p-3 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl"
        >
          <div className="text-xs text-[var(--color-text-muted)] mb-2">
            Select date and time
          </div>
          <input
            type="datetime-local"
            defaultValue={getCalendarValue()}
            onChange={handleCalendarSelect}
            className="w-full px-3 py-2 text-sm border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
          />
        </div>
      )}
    </div>
  );
};
