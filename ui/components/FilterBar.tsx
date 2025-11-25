import React, { useState, useRef, useEffect } from 'react';
import { FilterState } from '../types';
import { MOCK_NAMESPACES, MOCK_KINDS } from '../constants';

interface FilterBarProps {
  filters: FilterState;
  setFilters: React.Dispatch<React.SetStateAction<FilterState>>;
}

const MultiSelectDropdown = ({ 
    label, 
    options, 
    selected, 
    onToggle 
}: { 
    label: string, 
    options: string[], 
    selected: string[], 
    onToggle: (val: string) => void 
}) => {
    const [isOpen, setIsOpen] = useState(false);
    const [focusedIndex, setFocusedIndex] = useState(-1);
    const dropdownRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<HTMLDivElement>(null);
    const triggerRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setIsOpen(false);
                setFocusedIndex(-1);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

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
    }, [focusedIndex, isOpen]);

    const handleKeyDown = (e: React.KeyboardEvent) => {
        if (!isOpen) {
            if (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                setIsOpen(true);
                setFocusedIndex(0);
            }
            return;
        }

        switch (e.key) {
            case 'ArrowDown':
                e.preventDefault();
                setFocusedIndex(prev => (prev + 1) % options.length);
                break;
            case 'ArrowUp':
                e.preventDefault();
                setFocusedIndex(prev => (prev - 1 + options.length) % options.length);
                break;
            case 'Enter':
            case ' ':
                e.preventDefault();
                if (focusedIndex >= 0 && focusedIndex < options.length) {
                    onToggle(options[focusedIndex]);
                }
                break;
            case 'Escape':
                e.preventDefault();
                setIsOpen(false);
                setFocusedIndex(-1);
                triggerRef.current?.focus();
                break;
            case 'Tab':
                setIsOpen(false);
                setFocusedIndex(-1);
                break;
        }
    };

    const handleToggle = (option: string) => {
        onToggle(option);
        // Keep focus on the option to allow multi-selection
        triggerRef.current?.focus();
    };

    return (
        <div className="relative" ref={dropdownRef} onKeyDown={handleKeyDown}>
            <button
                ref={triggerRef}
                onClick={() => {
                    setIsOpen(!isOpen);
                    if (!isOpen) setFocusedIndex(0);
                }}
                className={`flex items-center justify-between gap-2 px-4 py-2 text-sm font-medium rounded-md border transition-all min-w-[160px] outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 ${
                    selected.length > 0
                        ? 'bg-gray-800 border-brand-500 text-brand-300 ring-1 ring-brand-500/20'
                        : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600 hover:text-gray-300'
                }`}
                aria-haspopup="listbox"
                aria-expanded={isOpen}
            >
                <span className="truncate max-w-[120px]">
                    {selected.length === 0 
                        ? label 
                        : `${label} (${selected.length})`}
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
                <div className="absolute top-full left-0 mt-2 w-56 bg-gray-850 border border-gray-700 rounded-lg shadow-xl z-50 overflow-hidden animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black ring-opacity-5">
                    <div 
                        className="max-h-64 overflow-y-auto p-2 space-y-1 custom-scrollbar"
                        role="listbox"
                        ref={listRef}
                        aria-multiselectable="true"
                    >
                        {options.map((option, index) => {
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
                                        isFocused ? 'bg-gray-700' : ''
                                    } ${
                                        isSelected 
                                            ? 'bg-brand-900/30 text-brand-200' 
                                            : 'text-gray-400 hover:bg-gray-800 hover:text-gray-200'
                                    }`}
                                >
                                    <div className={`w-4 h-4 rounded border mr-3 flex items-center justify-center transition-all ${
                                        isSelected
                                            ? 'bg-brand-600 border-brand-600'
                                            : 'border-gray-600'
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
                        })}
                    </div>
                </div>
            )}
        </div>
    );
};

export const FilterBar: React.FC<FilterBarProps> = ({ filters, setFilters }) => {
  
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

  return (
    <div className="w-full bg-gray-900/95 backdrop-blur border-b border-gray-800 p-4 flex flex-row gap-6 items-center shadow-lg z-30">
        {/* Brand */}
        <div className="flex items-center gap-3 min-w-max">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-white shadow-lg shadow-indigo-500/20 ring-1 ring-white/10">
                <svg className="w-6 h-6" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M12 2C7.58172 2 4 5.58172 4 10V19C4 20.6569 5.34315 22 7 22C7.63228 22 8.21952 21.7909 8.70773 21.4312L10.5858 19.5531C10.9609 19.1781 11.4696 18.9674 12 18.9674C12.5304 18.9674 13.0391 19.1781 13.4142 19.5531L15.2923 21.4312C15.7805 21.7909 16.3677 22 17 22C18.6569 22 20 20.6569 20 19V10C20 5.58172 16.4183 2 12 2Z" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    <path d="M9 10H9.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                    <path d="M15 10H15.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
                </svg>
            </div>
            <div>
                <h1 className="text-xl font-bold tracking-tight text-white leading-none">Spectre</h1>
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
                className="block w-full pl-10 pr-3 py-2 border border-gray-700 rounded-md leading-5 bg-gray-800 text-gray-300 placeholder-gray-500 focus:outline-none focus:bg-gray-900 focus:border-brand-500 focus:ring-1 focus:ring-brand-500 sm:text-sm transition-all shadow-sm"
            />
        </div>

        {/* Filters - Dropdowns */}
        <div className="flex gap-3">
            <MultiSelectDropdown 
                label="All Namespaces" 
                options={MOCK_NAMESPACES} 
                selected={filters.namespaces} 
                onToggle={toggleNamespace} 
            />
            
            <MultiSelectDropdown 
                label="All Kinds" 
                options={MOCK_KINDS} 
                selected={filters.kinds} 
                onToggle={toggleKind} 
            />
        </div>
    </div>
  );
};