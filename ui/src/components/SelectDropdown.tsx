import React, { useState, useRef, useEffect, useMemo } from 'react';

interface SelectDropdownProps {
  /** Label shown when nothing is selected */
  label: string;
  /** Available options (will be sorted alphabetically) */
  options: string[];
  /** Selected value(s) - string | null for single-select, string[] for multi-select */
  selected: string | string[] | null;
  /** Callback when selection changes */
  onChange: (value: string | string[] | null) => void;
  /** Enable multi-select mode (default: false) */
  multiple?: boolean;
  /** Show search input (default: true) */
  searchable?: boolean;
  /** Minimum width of the dropdown trigger (default: '160px') */
  minWidth?: string;
  /** Custom formatter for displaying option values (default: identity) */
  formatOption?: (value: string) => string;
  /** Whether to sort options alphabetically (default: true) */
  sortOptions?: boolean;
  /**
   * Visual style of the trigger:
   * - 'control': standalone 32px control (default)
   * - 'token': compact filter token for use inside a query bar
   */
  variant?: 'control' | 'token';
  /** Short key shown in front of the value in token mode (e.g. "ns", "kind") */
  tokenLabel?: string;
  /** Optional leading element inside the trigger (e.g. an icon) */
  prefix?: React.ReactNode;
}

/**
 * Reusable dropdown component supporting both single and multi-select modes.
 * 
 * Features:
 * - Searchable options
 * - Keyboard navigation (arrow keys, enter, escape)
 * - Click outside to close
 * - Options sorted alphabetically
 * - Multi-select: checkboxes, clear button
 * - Single-select: no checkboxes, closes on selection
 */
export const SelectDropdown: React.FC<SelectDropdownProps> = ({
  label,
  options,
  selected,
  onChange,
  multiple = false,
  searchable = true,
  minWidth = '160px',
  formatOption = (v) => v,
  sortOptions = true,
  variant = 'control',
  tokenLabel,
  prefix,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [focusedIndex, setFocusedIndex] = useState(-1);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Normalize selected to array for internal use
  const selectedArray = useMemo(() => {
    if (multiple) {
      return (selected as string[]) || [];
    }
    return selected !== null ? [selected as string] : [];
  }, [selected, multiple]);

  // Optionally sort options alphabetically and filter by search query
  const filteredOptions = useMemo(() => {
    const processed = sortOptions
      ? [...options].sort((a, b) => a.localeCompare(b))
      : options;
    if (!searchQuery.trim()) {
      return processed;
    }
    const query = searchQuery.toLowerCase();
    // Search against both the raw value and formatted display value
    return processed.filter(option =>
      option.toLowerCase().includes(query) ||
      formatOption(option).toLowerCase().includes(query)
    );
  }, [options, searchQuery, sortOptions, formatOption]);

  // Handle click outside - use capture phase to catch events before D3/SVG handlers
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        setFocusedIndex(-1);
        setSearchQuery('');
      }
    };
    document.addEventListener('mousedown', handleClickOutside, true);
    return () => document.removeEventListener('mousedown', handleClickOutside, true);
  }, []);

  // Focus search input when dropdown opens
  useEffect(() => {
    if (isOpen && searchable && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [isOpen, searchable]);

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

  // Handle option selection
  const handleSelect = (option: string, closeAfter: boolean) => {
    if (multiple) {
      const currentSelected = selectedArray;
      const newSelected = currentSelected.includes(option)
        ? currentSelected.filter(v => v !== option)
        : [...currentSelected, option];
      onChange(newSelected);
    } else {
      // Single-select: toggle off if clicking the already-selected option
      const newValue = selectedArray.includes(option) ? null : option;
      onChange(newValue);
    }

    if (closeAfter) {
      setIsOpen(false);
      setFocusedIndex(-1);
      setSearchQuery('');
      triggerRef.current?.focus();
    }
  };

  // Handle clear (multi-select only)
  const handleClear = () => {
    onChange(multiple ? [] : null);
  };

  // Keyboard navigation
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
        if (e.key === 'ArrowDown' && filteredOptions.length > 0) {
          setFocusedIndex(0);
          searchInputRef.current?.blur();
          requestAnimationFrame(() => {
            dropdownRef.current?.focus();
          });
        } else if (e.key === 'ArrowUp' && filteredOptions.length > 0) {
          setFocusedIndex(filteredOptions.length - 1);
          searchInputRef.current?.blur();
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
      } else if (e.key === 'Enter') {
        e.preventDefault();
        // Select first filtered option on Enter in search
        if (filteredOptions.length > 0) {
          handleSelect(filteredOptions[0], true);
        }
        return;
      }
      // Allow other keys (typing) to work normally
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
        e.preventDefault();
        if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
          // Space: toggle in multi-select (stay open), select in single-select (close)
          handleSelect(filteredOptions[focusedIndex], !multiple);
        }
        break;
      case 'Enter':
        e.preventDefault();
        if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
          // Enter: always close after selection
          handleSelect(filteredOptions[focusedIndex], true);
        }
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

  // Display text for trigger button
  const displayText = useMemo(() => {
    if (selectedArray.length === 0) {
      return label;
    }
    if (selectedArray.length === 1) {
      return formatOption(selectedArray[0]);
    }
    if (selectedArray.length > 2) {
      return `${selectedArray.slice(0, 2).map(formatOption).join(', ')} +${selectedArray.length - 2}`;
    }
    return selectedArray.map(formatOption).join(', ');
  }, [selectedArray, label, formatOption]);

  const hasSelection = selectedArray.length > 0;

  return (
    <div className="relative" ref={dropdownRef} onKeyDown={handleKeyDown} tabIndex={-1}>
      <button
        ref={triggerRef}
        type="button"
        onClick={() => {
          setIsOpen(!isOpen);
          if (!isOpen) {
            setFocusedIndex(0);
            setSearchQuery('');
          }
        }}
        style={variant === 'control' ? { minWidth } : undefined}
        className={
          variant === 'token'
            ? `inline-flex items-center gap-1.5 h-[22px] pl-[7px] pr-1.5 rounded-[5px] text-xs whitespace-nowrap outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[var(--color-accent-soft)] ${
                hasSelection
                  ? 'bg-[var(--color-surface-active)] text-[var(--color-text-primary)] hover:bg-[var(--color-border-strong)]'
                  : 'text-[var(--color-text-faint)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)]'
              }`
            : `ui-control justify-between ${hasSelection ? '!text-[var(--color-text-primary)]' : ''}`
        }
        aria-haspopup="listbox"
        aria-expanded={isOpen}
        title={hasSelection ? selectedArray.map(formatOption).join(', ') : ''}
      >
        {prefix}
        {variant === 'token' && hasSelection && tokenLabel && (
          <span className="text-[var(--color-text-faint)] font-medium">{tokenLabel}</span>
        )}
        <span className={`truncate ${variant === 'token' ? 'max-w-[260px]' : 'max-w-[200px]'}`}>
          {displayText}
        </span>
        <svg
          className={`flex-shrink-0 text-[var(--color-text-faint)] transition-transform duration-200 ${variant === 'token' ? 'w-3 h-3' : 'w-3.5 h-3.5'} ${isOpen ? 'rotate-180' : ''}`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 9l-7 7-7-7" />
        </svg>
      </button>

      {isOpen && (
        <div className="ui-popover absolute top-full left-0 mt-2 w-64 z-[60] overflow-hidden">
          {/* Search Input and Clear Button */}
          {(searchable || hasSelection) && (
            <div className="p-2 border-b border-[var(--color-border-soft)] space-y-2">
              {searchable && (
                <div className="relative">
                  <div className="absolute inset-y-0 left-0 pl-2 flex items-center pointer-events-none">
                    <svg className="h-3.5 w-3.5 text-[var(--color-text-faint)]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                    </svg>
                  </div>
                  <input
                    ref={searchInputRef}
                    type="text"
                    value={searchQuery}
                    onChange={(e) => {
                      setSearchQuery(e.target.value);
                      setFocusedIndex(-1);
                    }}
                    placeholder="Search..."
                    className="ui-input !h-[28px] !pl-7 !text-xs"
                    onClick={(e) => e.stopPropagation()}
                  />
                </div>
              )}
              {hasSelection && (
                <button
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    handleClear();
                  }}
                  className="w-full h-7 px-3 text-xs font-medium text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] rounded-md transition-colors flex items-center justify-center gap-1.5"
                >
                  <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                  Clear{multiple ? ` filter (${selectedArray.length})` : ''}
                </button>
              )}
            </div>
          )}

          {/* Options List */}
          <div
            className="max-h-72 overflow-y-auto p-1 custom-scrollbar outline-none"
            role="listbox"
            ref={listRef}
            aria-multiselectable={multiple}
            tabIndex={0}
          >
            {filteredOptions.length === 0 ? (
              <div className="px-3 py-3 text-xs text-[var(--color-text-faint)] text-center">
                No matches found
              </div>
            ) : (
              filteredOptions.map((option, index) => {
                const isSelected = selectedArray.includes(option);
                const isFocused = index === focusedIndex;
                return (
                  <div
                    key={option}
                    role="option"
                    aria-selected={isSelected}
                    onClick={() => handleSelect(option, !multiple)}
                    onMouseEnter={() => setFocusedIndex(index)}
                    className={`flex items-center h-8 px-2.5 rounded-md cursor-pointer transition-colors ${
                      isFocused ? 'bg-[var(--color-surface-muted)]' : ''
                    } ${
                      isSelected
                        ? 'text-[var(--color-text-primary)]'
                        : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
                    }`}
                  >
                    {multiple && (
                      <div className={`w-3.5 h-3.5 rounded-[4px] border mr-2.5 flex items-center justify-center transition-all ${
                        isSelected
                          ? 'bg-[var(--color-accent)] border-[var(--color-accent)]'
                          : 'border-[var(--color-border-strong)]'
                      }`}>
                        {isSelected && (
                          <svg className="w-2.5 h-2.5 text-[var(--color-on-accent)]" fill="none" stroke="currentColor" strokeWidth={3} viewBox="0 0 24 24">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                          </svg>
                        )}
                      </div>
                    )}
                    <span className="text-[13px] select-none truncate">{formatOption(option)}</span>
                    {!multiple && isSelected && (
                      <svg className="w-3.5 h-3.5 ml-auto text-[var(--color-accent)] flex-shrink-0" fill="none" stroke="currentColor" strokeWidth={2.5} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" /></svg>
                    )}
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

export default SelectDropdown;
