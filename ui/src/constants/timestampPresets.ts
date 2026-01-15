/**
 * Preset options for quick timestamp selection in the graph page
 */
export interface TimestampPreset {
  /** Display label for the preset button */
  label: string;
  /** Time expression (e.g., "now", "now-1h") */
  expression: string;
  /** If true, shows a live indicator (green pulse) */
  isLive?: boolean;
}

export const TIMESTAMP_PRESETS: TimestampPreset[] = [
  { label: 'Now', expression: 'now', isLive: true },
  { label: '1h ago', expression: 'now-1h' },
  { label: '6h ago', expression: 'now-6h' },
  { label: '24h ago', expression: 'now-24h' },
];
