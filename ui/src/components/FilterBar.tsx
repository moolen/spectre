import React from 'react';
import { FilterState, TimeRange } from '../types';
import { TimeRangeDropdown } from './TimeRangeDropdown';
import { TimeRangeQuickPresets } from './TimeRangeQuickPresets';
import { SelectDropdown } from './SelectDropdown';
import { PageHeader } from './Common/PageHeader';
import { Icon } from './Common/Icon';
import { formatNamespaceFilterOption } from '../utils/namespaceFilters';

interface FilterBarProps {
  filters: FilterState;
  setFilters: React.Dispatch<React.SetStateAction<FilterState>>;
  timeRange: TimeRange;
  onTimeRangeChange: (range: TimeRange, rawStart?: string, rawEnd?: string) => void;
  availableNamespaces: string[];
  availableKinds: string[];
  rawStart?: string;
  rawEnd?: string;
}

/**
 * Timeline top bar: page title, a single query bar holding the filter tokens
 * and free-text search, and the time range controls on the right.
 */
export const FilterBar: React.FC<FilterBarProps> = ({ filters, setFilters, timeRange, onTimeRangeChange, availableNamespaces, availableKinds, rawStart, rawEnd }) => {

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilters(prev => ({ ...prev, search: e.target.value }));
  };

  const handleNamespacesChange = (values: string | string[] | null) => {
    setFilters(prev => ({ ...prev, namespaces: (values as string[]) || [] }));
  };

  const handleKindsChange = (values: string | string[] | null) => {
    setFilters(prev => ({ ...prev, kinds: (values as string[]) || [] }));
  };

  return (
    <PageHeader
      title="Timeline"
      actions={
        <>
          <TimeRangeQuickPresets onConfirm={onTimeRangeChange} rawStart={rawStart} rawEnd={rawEnd} />
          <TimeRangeDropdown
            currentRange={timeRange}
            onConfirm={onTimeRangeChange}
            rawStart={rawStart}
            rawEnd={rawEnd}
          />
        </>
      }
    >
      {/* Query bar: filter tokens + search in one field */}
      <div className="group flex items-center gap-1 h-8 flex-1 max-w-[760px] ml-3 pl-2.5 pr-1.5 rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-surface-elevated)] focus-within:border-[var(--color-accent-line)] focus-within:shadow-[0_0_0_3px_var(--color-accent-soft)] transition-colors">
        <Icon name="search" size={14} className="text-[var(--color-text-faint)] mr-1" />

        <SelectDropdown
          label="All Namespaces"
          tokenLabel="ns"
          variant="token"
          options={availableNamespaces}
          selected={filters.namespaces}
          onChange={handleNamespacesChange}
          multiple={true}
          formatOption={formatNamespaceFilterOption}
        />

        <SelectDropdown
          label="All Kinds"
          tokenLabel="kind"
          variant="token"
          options={availableKinds}
          selected={filters.kinds}
          onChange={handleKindsChange}
          multiple={true}
        />

        <button
          type="button"
          onClick={() => setFilters(prev => ({ ...prev, hasProblematicStatus: !prev.hasProblematicStatus }))}
          aria-pressed={filters.hasProblematicStatus}
          className={`inline-flex items-center gap-1.5 h-[22px] px-[7px] rounded-[5px] text-xs whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent-soft)] ${
            filters.hasProblematicStatus
              ? 'bg-[var(--color-accent-soft)] text-[var(--color-accent-strong)]'
              : 'text-[var(--color-text-faint)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)]'
          }`}
          title="Show only resources with problematic status (Warning, Error, Terminating, or Unknown)"
        >
          <span
            className={`w-1.5 h-1.5 rounded-full ${
              filters.hasProblematicStatus ? 'bg-[var(--color-accent)]' : 'bg-[var(--color-status-err)]'
            }`}
          />
          <span>Problematic only</span>
        </button>

        <span className="w-px h-4 bg-[var(--color-border-strong)] mx-1.5" aria-hidden="true" />

        <input
          type="text"
          placeholder="Search resources by name..."
          value={filters.search}
          onChange={handleSearchChange}
          className="flex-1 min-w-[120px] h-full bg-transparent text-[12.5px] text-[var(--color-text-primary)] placeholder-[var(--color-text-faint)] outline-none border-none"
        />
      </div>
    </PageHeader>
  );
};
