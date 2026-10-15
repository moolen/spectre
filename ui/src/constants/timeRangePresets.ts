export interface TimeRangePreset {
  label: string;
  /** Compact label for the segmented control in the top bar */
  shortLabel: string;
  minutes: number;
  relative: string;
}

export const TIME_RANGE_PRESETS: TimeRangePreset[] = [
  { label: 'Last 30min', shortLabel: '30m', minutes: 30, relative: 'now-30m' },
  { label: 'Last 6h', shortLabel: '6h', minutes: 360, relative: 'now-6h' },
  { label: 'Last 24h', shortLabel: '24h', minutes: 1440, relative: 'now-24h' },
  { label: 'Last 48h', shortLabel: '48h', minutes: 2880, relative: 'now-48h' },
];
