import React from 'react';
import { SelectDropdown } from '../SelectDropdown';
import { TimestampPicker } from '../TimestampPicker';
import { LOOKBACK_OPTIONS } from '../../hooks/usePersistedGraphLookback';
import { formatNamespaceFilterOption } from '../../utils/namespaceFilters';
import { PageHeader } from '../Common/PageHeader';
import { Icon } from '../Common/Icon';

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
    if (typeof value === 'string') {
      onNamespaceChange(value);
    }
  };

  const handleKindsChange = (value: string | string[] | null) => {
    onKindsChange((value as string[]) || []);
  };

  return (
    <PageHeader
      title="Graph"
      actions={
        <>
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
            prefix={<span className="text-[var(--color-text-faint)]">Lookback</span>}
            minWidth="0"
          />
        </>
      }
    >
      {/* Query bar with the namespace and kind tokens */}
      <div className="flex items-center gap-1 h-8 flex-1 max-w-[560px] ml-3 pl-2.5 pr-1.5 rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-surface-elevated)]">
        <Icon name="filter" size={14} className="text-[var(--color-text-faint)] mr-1" />

        {/* Namespace Dropdown */}
        <SelectDropdown
          label="Select Namespace"
          tokenLabel="ns"
          variant="token"
          options={namespaces}
          selected={namespace}
          onChange={handleNamespaceChange}
          multiple={false}
          formatOption={formatNamespaceFilterOption}
        />

        {/* Kind Filter Dropdown */}
        <SelectDropdown
          label="All Kinds"
          tokenLabel="kind"
          variant="token"
          options={availableKinds}
          selected={kinds}
          onChange={handleKindsChange}
          multiple={true}
        />
      </div>
    </PageHeader>
  );
};

export default NamespaceGraphControls;
