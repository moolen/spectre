import React, { useMemo } from 'react';
import { SelectDropdown } from '../SelectDropdown';
import { TimestampPicker } from '../TimestampPicker';
import { LOOKBACK_OPTIONS } from '../../hooks/usePersistedGraphLookback';

// Create a lookup map for formatting lookback values to labels
const LOOKBACK_LABELS: Record<string, string> = Object.fromEntries(
  LOOKBACK_OPTIONS.map(opt => [opt.value, opt.label])
);

interface NamespaceGraphControlsProps {
  /** Current namespace */
  namespace: string;
  /** Available namespaces */
  namespaces: string[];
  /** Callback when namespace changes */
  onNamespaceChange: (namespace: string) => void;
  /** Currently selected kinds */
  kinds: string[];
  /** Available kinds */
  availableKinds: string[];
  /** Callback when kinds change */
  onKindsChange: (kinds: string[]) => void;
  /** Current timestamp expression (e.g., "now", "2h ago") */
  timestampExpression: string;
  /** Callback when timestamp changes */
  onTimestampChange: (expression: string) => void;
  /** Resolved timestamp for display */
  resolvedTimestamp?: Date;
  /** Current lookback period for spec changes */
  lookback: string;
  /** Callback when lookback changes */
  onLookbackChange: (lookback: string) => void;
}

/**
 * Control bar for the namespace graph view
 * Contains namespace selector dropdown and kind filter
 */
export const NamespaceGraphControls: React.FC<NamespaceGraphControlsProps> = ({
  namespace,
  namespaces,
  onNamespaceChange,
  kinds,
  availableKinds,
  onKindsChange,
  timestampExpression,
  onTimestampChange,
  resolvedTimestamp,
  lookback,
  onLookbackChange,
}) => {
  const handleNamespaceChange = (value: string | string[] | null) => {
    if (value && typeof value === 'string') {
      onNamespaceChange(value);
    }
  };

  const handleKindsChange = (value: string | string[] | null) => {
    onKindsChange((value as string[]) || []);
  };

  return (
    <div className="w-full bg-[var(--color-surface-elevated)]/95 backdrop-blur border-b border-[var(--color-border-soft)] p-4 flex flex-row gap-6 items-center shadow-lg z-30 text-[var(--color-text-primary)] transition-colors duration-300">
      {/* Namespace Dropdown */}
      <SelectDropdown
        label="Select Namespace"
        options={namespaces}
        selected={namespace}
        onChange={handleNamespaceChange}
        multiple={false}
        minWidth="200px"
      />

      {/* Kind Filter Dropdown */}
      <SelectDropdown
        label="All Kinds"
        options={availableKinds}
        selected={kinds}
        onChange={handleKindsChange}
        multiple={true}
        minWidth="200px"
      />

      {/* Timestamp Picker */}
      <TimestampPicker
        expression={timestampExpression}
        onChange={onTimestampChange}
        resolvedTimestamp={resolvedTimestamp}
      />

      {/* Lookback Dropdown */}
      <SelectDropdown
        label="Lookback"
        options={LOOKBACK_OPTIONS.map(opt => opt.value)}
        selected={lookback}
        onChange={(value) => {
          if (value && typeof value === 'string') {
            onLookbackChange(value);
          }
        }}
        multiple={false}
        searchable={false}
        sortOptions={false}
        formatOption={(value) => LOOKBACK_LABELS[value] || value}
        minWidth="140px"
      />
    </div>
  );
};

export default NamespaceGraphControls;
