import React, { useState, useRef, useEffect } from 'react';
import { FilterState, TimeRange } from '../types';
import { TimeRangeDropdown } from './TimeRangeDropdown';
import { SettingsMenu } from './SettingsMenu';

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

const MultiSelectDropdown = ({
    label,
    options,
    selected,
    onToggle,
    onClear
}: {
    label: string,
    options: string[],
    selected: string[],
    onToggle: (val: string) => void,
    onClear: () => void
}) => {
    const [isOpen, setIsOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState('');
    const [focusedIndex, setFocusedIndex] = useState(-1);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    const triggerRef = useRef<HTMLButtonElement>(null);
    const searchInputRef = useRef<HTMLInputElement>(null);

    // Filter options based on search query
    const filteredOptions = React.useMemo(() => {
        if (!searchQuery.trim()) {
            return options;
        }
        const query = searchQuery.toLowerCase();
        return options.filter(option => option.toLowerCase().includes(query));
    }, [options, searchQuery]);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setIsOpen(false);
                setFocusedIndex(-1);
                setSearchQuery('');
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    // Focus search input when dropdown opens
    useEffect(() => {
        if (isOpen && searchInputRef.current) {
            searchInputRef.current.focus();
        }
    }, [isOpen]);

    // Ensure focused item is visible
    useEffect(() => {
        if (isOpen && focusedIndex >= 0 && listRef.current) {
            const list = listRef.current;
            const element = list.children[focusedIndex] as HTMLElement;
            if (element) {
                const listTop = list.scrollTop;
                const listBottom = listTop + list.clientHeight;
                const elementTop = element.offsetTop;
                const elementBottom = elementTop + element.clientHeight;

                if (elementTop < listTop) {
                    list.scrollTop = elementTop;
                } else if (elementBottom > listBottom) {
                    list.scrollTop = elementBottom - list.clientHeight;
                }
            }
        }
    }, [focusedIndex, isOpen, filteredOptions]);

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (!isOpen) {
            if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                setIsOpen(true);
                setFocusedIndex(0);
            }
            return;
        }

        // Handle keys when typing in search input
        const target = e.target as HTMLElement;
        if (target.tagName === 'INPUT') {
            if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                e.preventDefault();
                // Move focus to first/last item when arrow keys pressed in search
                if (e.key === 'ArrowDown' && filteredOptions.length > 0) {
                    e.preventDefault();
                    setFocusedIndex(0);
                    searchInputRef.current?.blur();
                    // Focus the container so it can receive subsequent keyboard events
                    requestAnimationFrame(() => {
                        dropdownRef.current?.focus();
                    });
                } else if (e.key === 'ArrowUp' && filteredOptions.length > 0) {
                    e.preventDefault();
                    setFocusedIndex(filteredOptions.length - 1);
                    searchInputRef.current?.blur();
                    // Focus the container so it can receive subsequent keyboard events
                    requestAnimationFrame(() => {
                        dropdownRef.current?.focus();
                    });
                }
                return;
            } else if (e.key === 'Escape') {
                e.preventDefault();
                setIsOpen(false);
                setFocusedIndex(-1);
                setSearchQuery('');
                triggerRef.current?.focus();
                return;
            }
            // Allow other keys (typing) to work normally in the input
            return;
        }

        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                setFocusedIndex(prev => (prev + 1) % filteredOptions.length);
                break;
            case 'ArrowUp':
                e.preventDefault();
                setFocusedIndex(prev => (prev - 1 + filteredOptions.length) % filteredOptions.length);
                break;
            case ' ':
                // Spacebar: select highlighted item (keep dropdown open for multi-selection)
                e.preventDefault();
                if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
                    onToggle(filteredOptions[focusedIndex]);
                }
                break;
            case 'Enter':
                // Enter: apply filter and close dropdown
                e.preventDefault();
                if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
                    onToggle(filteredOptions[focusedIndex]);
                }
                setIsOpen(false);
                setFocusedIndex(-1);
                setSearchQuery('');
                triggerRef.current?.focus();
                break;
            case 'Escape':
                e.preventDefault();
                setIsOpen(false);
                setFocusedIndex(-1);
                setSearchQuery('');
                triggerRef.current?.focus();
                break;
            case 'Tab':
                setIsOpen(false);
                setFocusedIndex(-1);
                setSearchQuery('');
                break;
        }
    };

    const handleToggle = (option: string) => {
        onToggle(option);
        // Keep focus on the option to allow multi-selection
        triggerRef.current?.focus();
    };

    return (
        <div className="relative" ref={dropdownRef} onKeyDown={handleKeyDown} tabIndex={-1}>
            <button
                ref={triggerRef}
                onClick={() => {
                    setIsOpen(!isOpen);
                    if (!isOpen) {
                        setFocusedIndex(0);
                        setSearchQuery('');
                    }
                }}
                className={`flex items-center justify-between gap-2 px-4 py-2 text-sm font-medium rounded-md border transition-all min-w-[160px] outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 ${
                    selected.length > 0
                        ? 'bg-[var(--color-surface-active)] border-brand-500 text-brand-300 ring-1 ring-brand-500/20'
                        : 'bg-[var(--color-surface-muted)] border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)]'
                }`}
                aria-haspopup="listbox"
                aria-expanded={isOpen}
                title={selected.length > 0 ? selected.join(', ') : ''}
            >
                <span className="truncate max-w-[120px]">
                    {selected.length === 0
                        ? label
                        : selected.length === 1
                        ? selected[0]
                        : `${selected.join(', ')}`}
                </span>
                <svg
                    className={`w-4 h-4 text-gray-500 transition-transform duration-200 ${isOpen ? 'rotate-180' : ''}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                >
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
                </svg>
            </button>

            {isOpen && (
                <div className="absolute top-full left-0 mt-2 w-56 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl z-50 overflow-hidden animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10">
                    {/* Search Input and Clear Button */}
                    <div className="p-2 border-b border-[var(--color-border-soft)] space-y-2">
                        <div className="relative">
                            <div className="absolute inset-y-0 left-0 pl-2 flex items-center pointer-events-none">
                                <svg className="h-3.5 w-3.5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                </svg>
                            </div>
                            <input
                                ref={searchInputRef}
                                type="text"
                                value={searchQuery}
                                onChange={(e) => {
                                    setSearchQuery(e.target.value);
                                    setFocusedIndex(-1); // Reset focus when searching
                                }}
                                placeholder="Search..."
                                className="w-full pl-7 pr-2 py-1.5 text-xs border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                                onClick={(e) => e.stopPropagation()}
                            />
                        </div>
                        {selected.length > 0 && (
                            <button
                                onClick={(e) => {
                                    e.preventDefault();
                                    e.stopPropagation();
                                    onClear();
                                }}
                                className="w-full px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] bg-[var(--color-surface-muted)] hover:bg-[var(--color-surface-active)] border border-[var(--color-border-soft)] rounded-md transition-colors flex items-center justify-center gap-1.5"
                            >
                                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                                </svg>
                                Clear filter ({selected.length})
                            </button>
                        )}
                    </div>

                    {/* Options List */}
                    <div
                        className="max-h-64 overflow-y-auto p-2 space-y-1 custom-scrollbar outline-none"
                        role="listbox"
                        ref={listRef}
                        aria-multiselectable="true"
                        tabIndex={0}
                    >
                        {filteredOptions.length === 0 ? (
                            <div className="px-3 py-2 text-xs text-[var(--color-text-muted)] text-center">
                                No matches found
                            </div>
                        ) : (
                            filteredOptions.map((option, index) => {
                                const isSelected = selected.includes(option);
                                const isFocused = index === focusedIndex;
                                return (
                                    <div
                                        key={option}
                                        role="option"
                                        aria-selected={isSelected}
                                        onClick={() => onToggle(option)}
                                        onMouseEnter={() => setFocusedIndex(index)}
                                        className={`flex items-center px-3 py-2 rounded-md cursor-pointer transition-colors ${
                                            isFocused ? 'bg-[var(--color-surface-muted)]' : ''
                                        } ${
                                            isSelected
                                                ? 'bg-brand-900/30 text-brand-200'
                                                : 'text-[var(--color-text-muted)] hover:bg-[var(--color-surface-muted)] hover:text-[var(--color-text-primary)]'
                                        }`}
                                    >
                                        <div className={`w-4 h-4 rounded border mr-3 flex items-center justify-center transition-all ${
                                            isSelected
                                                ? 'bg-brand-600 border-brand-600'
                                                : 'border-[var(--color-border-soft)]'
                                        }`}>
                                            {isSelected && (
                                                <svg className="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                                                </svg>
                                            )}
                                        </div>
                                        <span className="text-sm select-none">{option}</span>
                                    </div>
                                );
                            })
                        )}
                    </div>
                </div>
            )}
        </div>
    );
};

export const FilterBar: React.FC<FilterBarProps> = ({ filters, setFilters, timeRange, onTimeRangeChange, availableNamespaces, availableKinds, rawStart, rawEnd }) => {

  const handleSearchChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setFilters(prev => ({ ...prev, search: e.target.value }));
  };

  const toggleNamespace = (ns: string) => {
    setFilters(prev => {
      const newNs = prev.namespaces.includes(ns)
        ? prev.namespaces.filter(n => n !== ns)
        : [...prev.namespaces, ns];
      return { ...prev, namespaces: newNs };
    });
  };

  const toggleKind = (kind: string) => {
      setFilters(prev => {
          const newKinds = prev.kinds.includes(kind)
            ? prev.kinds.filter(k => k !== kind)
            : [...prev.kinds, kind];
          return { ...prev, kinds: newKinds };
      });
  }

  const clearNamespaces = () => {
    setFilters(prev => ({ ...prev, namespaces: [] }));
  };

  const clearKinds = () => {
    setFilters(prev => ({ ...prev, kinds: [] }));
  };

  return (
    <div className="w-full bg-[var(--color-surface-elevated)]/95 backdrop-blur border-b border-[var(--color-border-soft)] p-4 flex flex-row gap-6 items-center shadow-lg z-30 text-[var(--color-text-primary)] transition-colors duration-300">
        {/* Keyframes for animations */}
        <style>{`
          @keyframes gradient-shift {
            0%, 100% { background-position: 0% 0%; }
            50% { background-position: 100% 100%; }
          }
          @keyframes levitate {
            0% { transform: translate(0px, 0px); }
            15% { transform: translate(0.5px, -1.5px); }
            30% { transform: translate(-0.3px, -2.2px); }
            45% { transform: translate(0.8px, -1px); }
            60% { transform: translate(-0.5px, -2.5px); }
            75% { transform: translate(0.2px, -0.8px); }
            90% { transform: translate(-0.4px, -1.8px); }
            100% { transform: translate(0px, 0px); }
          }
        `}</style>
        {/* Brand */}
        <div className="flex items-center gap-3 min-w-max">
            <div
                className="w-10 h-10 rounded-xl flex items-center justify-center text-white shadow-lg shadow-indigo-500/20 ring-1 ring-white/10"
                style={{
                    background: 'linear-gradient(135deg, #6366f1 0%, #8b5cf6 25%, #a855f7 50%, #7c3aed 75%, #6366f1 100%)',
                    backgroundSize: '200% 200%',
                    animation: 'gradient-shift 15s ease-in-out infinite',
                }}
            >
                <svg
                    className="w-6 h-6"
                    viewBox="0 0 24 24"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                    style={{ animation: 'levitate 10s ease-in-out infinite' }}
                >
                    <path d="M12 2C7.58172 2 4 5.58172 4 10V19C4 20.6569 5.34315 22 7 22C7.63228 22 8.21952 21.7909 8.70773 21.4312L10.5858 19.5531C10.9609 19.1781 11.4696 18.9674 12 18.9674C12.5304 18.9674 13.0391 19.1781 13.4142 19.5531L15.2923 21.4312C15.7805 21.7909 16.3677 22 17 22C18.6569 22 20 20.6569 20 19V10C20 5.58172 16.4183 2 12 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    <path d="M9 10H9.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    <path d="M15 10H15.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
            </div>
            <div>
                <h1 className="text-xl font-bold tracking-tight text-[var(--color-text-primary)] leading-none">Spectre</h1>
            </div>
        </div>

        {/* Divider */}
        <div className="h-8 w-px bg-gray-800 hidden md:block"></div>

        {/* Search */}
        <div className="relative group min-w-[300px] flex-grow max-w-2xl">
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
            <MultiSelectDropdown
                label="All Namespaces"
                options={availableNamespaces}
                selected={filters.namespaces}
                onToggle={toggleNamespace}
                onClear={clearNamespaces}
            />

            <MultiSelectDropdown
                label="All Kinds"
                options={availableKinds}
                selected={filters.kinds}
                onToggle={toggleKind}
                onClear={clearKinds}
            />
        </div>

        {/* Time Range Picker */}
        <div className="ml-auto flex items-center gap-3">
            <TimeRangeDropdown
                currentRange={timeRange}
                onConfirm={onTimeRangeChange}
                rawStart={rawStart}
                rawEnd={rawEnd}
            />
            <SettingsMenu />
        </div>
    </div>
  );
};