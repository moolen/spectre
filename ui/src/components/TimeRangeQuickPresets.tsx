import React from 'react';
import { TimeRange } from '../types';
import { usePersistedQuickPreset } from '../hooks/usePersistedQuickPreset';
import { validateTimeRange } from '../utils/timeParsing';
import { TIME_RANGE_PRESETS } from '../constants/timeRangePresets';

interface TimeRangeQuickPresetsProps {
  onConfirm: (range: TimeRange, rawStart?: string, rawEnd?: string) => void;
  rawStart?: string;
  rawEnd?: string;
}

/**
 * Segmented control with the quick relative presets (30m / 6h / 24h / 48h).
 * A preset is highlighted when the current range is exactly "now-X → now".
 */
export const TimeRangeQuickPresets: React.FC<TimeRangeQuickPresetsProps> = ({ onConfirm, rawStart, rawEnd }) => {
  const { savePreset } = usePersistedQuickPreset();

  const activeRelative = rawEnd === 'now' ? rawStart : undefined;

  const handleClick = (minutes: number, relative: string) => {
    const validation = validateTimeRange(relative, 'now');
    if (validation.valid === false) return;
    savePreset(minutes);
    onConfirm({ start: validation.start, end: validation.end }, relative, 'now');
  };

  return (
    <div className="ui-segmented" role="group" aria-label="Quick time range">
      {TIME_RANGE_PRESETS.map((preset) => (
        <button
          key={preset.minutes}
          type="button"
          aria-pressed={activeRelative === preset.relative}
          title={preset.label}
          onClick={() => handleClick(preset.minutes, preset.relative)}
        >
          {preset.shortLabel}
        </button>
      ))}
    </div>
  );
};
