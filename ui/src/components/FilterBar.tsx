import React from 'react';
import { FilterState, TimeRange } from '../types';
import { TimeRangeDropdown } from './TimeRangeDropdown';
import { SelectDropdown } from './SelectDropdown';

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
    <div className="w-full bg-[var(--color-surface-elevated)]/95 backdrop-blur border-b border-[var(--color-border-soft)] p-4 flex flex-row gap-6 items-center shadow-lg z-30 text-[var(--color-text-primary)] transition-colors duration-300">
        {/* Search */}
        <div className="relative group min-w-[150px] flex-grow max-w-xl">
            <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <svg className="h-4 w-4 text-gray-500 group-focus-within:text-brand-500 transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path></svg>
            </div>
            <input
                type="text"
                placeholder="Search resources by name..."
                value={filters.search}
                onChange={handleSearchChange}
                className="block w-full pl-10 pr-3 py-2 border border-[var(--color-border-soft)] rounded-md leading-5 bg-[var(--color-input-bg)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:bg-[var(--color-surface-muted)] focus:border-brand-500 focus:ring-1 focus:ring-brand-500 sm:text-sm transition-all shadow-sm"
            />
        </div>

        {/* Filters - Dropdowns */}
        <div className="flex gap-3">
            <SelectDropdown
                label="All Namespaces"
                options={availableNamespaces}
                selected={filters.namespaces}
                onChange={handleNamespacesChange}
                multiple={true}
            />

            <SelectDropdown
                label="All Kinds"
                options={availableKinds}
                selected={filters.kinds}
                onChange={handleKindsChange}
                multiple={true}
            />

            <button
                onClick={() => setFilters(prev => ({ ...prev, hasProblematicStatus: !prev.hasProblematicStatus }))}
                className={`flex items-center justify-between gap-2 px-4 py-2 text-sm font-medium rounded-md border transition-all outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 ${
                    filters.hasProblematicStatus
                        ? 'bg-[var(--color-surface-active)] border-brand-500 text-brand-300 ring-1 ring-brand-500/20'
                        : 'bg-[var(--color-surface-muted)] border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)]'
                }`}
                title="Show only resources with problematic status (Warning, Error, Terminating, or Unknown)"
            >
                <div className="flex items-center gap-2">
                    <div className={`w-4 h-4 rounded border flex items-center justify-center transition-all ${
                        filters.hasProblematicStatus
                            ? 'bg-brand-600 border-brand-600'
                            : 'border-[var(--color-border-soft)]'
                    }`}>
                        {filters.hasProblematicStatus && (
                            <svg className="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                            </svg>
                        )}
                    </div>
                    <span className="truncate">Problematic Only</span>
                </div>
            </button>
        </div>

        {/* Time Range Picker */}
        <div className="ml-auto flex items-center gap-3">
            <TimeRangeDropdown
                currentRange={timeRange}
                onConfirm={onTimeRangeChange}
                rawStart={rawStart}
                rawEnd={rawEnd}
            />
        </div>
    </div>
  );
};
