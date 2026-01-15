import { r as reactExports, u as useSettings, j as jsxRuntimeExports, R as React, a as useSearchParams, N as NavLink, b as Routes, c as Route } from './index-CPSjW-or.js';
import { f as formatDateTimeForInput, v as validateTimeRange, T as TimeInputWithCalendar, a as apiClient, p as parseTimeExpression, t as toast, i as isHumanFriendlyExpression, b as formatDateTimeForDisplay, c as Toaster, S as SettingsPage } from './SettingsPage-Bo0M5phn.js';

const STORAGE_KEY$3 = "spectre-quick-preset";
const usePersistedQuickPreset = () => {
  const [preset, setPreset] = reactExports.useState(null);
  reactExports.useEffect(() => {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY$3);
      if (stored) {
        const minutes = parseInt(stored, 10);
        if (!isNaN(minutes)) {
          setPreset(minutes);
        }
      }
    } catch (error) {
      console.error("Failed to load preset from localStorage:", error);
    }
  }, []);
  const savePreset = (minutes) => {
    try {
      window.localStorage.setItem(STORAGE_KEY$3, minutes.toString());
      setPreset(minutes);
    } catch (error) {
      console.error("Failed to save preset to localStorage:", error);
    }
  };
  const clearPreset = () => {
    try {
      window.localStorage.removeItem(STORAGE_KEY$3);
      setPreset(null);
    } catch (error) {
      console.error("Failed to clear preset from localStorage:", error);
    }
  };
  return {
    preset,
    savePreset,
    clearPreset
  };
};

const TIME_RANGE_PRESETS = [
  { label: "Last 30min", minutes: 30, relative: "now-30m" },
  { label: "Last 6h", minutes: 360, relative: "now-6h" },
  { label: "Last 24h", minutes: 1440, relative: "now-24h" },
  { label: "Last 48h", minutes: 2880, relative: "now-48h" }
];

const TimeRangeDropdown = ({ currentRange, onConfirm, rawStart, rawEnd }) => {
  const [isOpen, setIsOpen] = reactExports.useState(false);
  const [startInput, setStartInput] = reactExports.useState("");
  const [endInput, setEndInput] = reactExports.useState("");
  const [selectedPreset, setSelectedPreset] = reactExports.useState(null);
  const [validationError, setValidationError] = reactExports.useState(null);
  const dropdownRef = reactExports.useRef(null);
  const { timeFormat } = useSettings();
  const { preset: persistedPreset, savePreset } = usePersistedQuickPreset();
  const rangeLabel = reactExports.useMemo(() => {
    const formatDate = (date) => {
      const month = date.toLocaleString("default", { month: "short" });
      const day = date.getDate();
      const formatter = new Intl.DateTimeFormat(void 0, {
        hour: "2-digit",
        minute: "2-digit",
        hour12: timeFormat === "12h"
      });
      return `${month} ${day}, ${formatter.format(date)}`;
    };
    if (rawStart && rawEnd) {
      return `${rawStart} to ${rawEnd}`;
    }
    if (rawStart) {
      return `${rawStart} to ${formatDate(currentRange.end)}`;
    }
    if (rawEnd) {
      return `${formatDate(currentRange.start)} to ${rawEnd}`;
    }
    return `${formatDate(currentRange.start)} - ${formatDate(currentRange.end)}`;
  }, [currentRange, timeFormat, rawStart, rawEnd]);
  reactExports.useEffect(() => {
    setStartInput(rawStart || formatDateTimeForInput(currentRange.start));
    setEndInput(rawEnd || formatDateTimeForInput(currentRange.end));
    setValidationError(null);
    if (persistedPreset) {
      setSelectedPreset(persistedPreset);
    } else {
      const now = /* @__PURE__ */ new Date();
      const rangeDuration = now.getTime() - currentRange.start.getTime();
      const matchingPreset = TIME_RANGE_PRESETS.find((p) => {
        const presetDuration = p.minutes * 60 * 1e3;
        return Math.abs(rangeDuration - presetDuration) < 1e3;
      });
      setSelectedPreset(matchingPreset ? matchingPreset.minutes : null);
    }
  }, [currentRange, persistedPreset, rawStart, rawEnd]);
  reactExports.useEffect(() => {
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);
  const handlePreset = (minutes, relative) => {
    const startExpr = relative;
    const endExpr = "now";
    setStartInput(startExpr);
    setEndInput(endExpr);
    setSelectedPreset(minutes);
    setValidationError(null);
    savePreset(minutes);
    const validation = validateTimeRange(startExpr, endExpr);
    if (validation.valid === false) {
      setValidationError(validation.error);
      return;
    }
    onConfirm({ start: validation.start, end: validation.end }, startExpr, endExpr);
    setIsOpen(false);
  };
  const handleCustomInputChange = () => {
    setSelectedPreset(null);
    setValidationError(null);
  };
  const handleConfirm = () => {
    const validation = validateTimeRange(startInput, endInput);
    if (validation.valid === false) {
      setValidationError(validation.error);
      return;
    }
    onConfirm({ start: validation.start, end: validation.end }, startInput, endInput);
    setIsOpen(false);
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative", ref: dropdownRef, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs(
      "button",
      {
        onClick: () => setIsOpen(!isOpen),
        className: "flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md border border-[var(--color-border-soft)] bg-[var(--color-input-bg)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200 transition-all min-w-[200px] justify-between",
        children: [
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" }) }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "truncate text-xs", children: rangeLabel })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "svg",
            {
              className: `w-4 h-4 text-[var(--color-text-muted)] transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`,
              fill: "none",
              stroke: "currentColor",
              viewBox: "0 0 24 24",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M19 9l-7 7-7-7" })
            }
          )
        ]
      }
    ),
    isOpen && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute top-full right-0 mt-2 w-96 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl z-[60] p-4 animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-4", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("label", { className: "block text-xs font-medium text-[var(--color-text-muted)] mb-2", children: "Quick Presets" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "grid grid-cols-2 gap-2", children: TIME_RANGE_PRESETS.map((preset) => {
          const isSelected = selectedPreset === preset.minutes;
          return /* @__PURE__ */ jsxRuntimeExports.jsx(
            "button",
            {
              onClick: () => handlePreset(preset.minutes, preset.relative),
              className: `px-3 py-1.5 text-xs font-medium rounded-md border transition-all ${isSelected ? "bg-brand-600 border-brand-500 text-white ring-1 ring-brand-500/20" : "border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200"}`,
              children: preset.label
            },
            preset.minutes
          );
        }) })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-4", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("label", { className: "block text-xs font-medium text-[var(--color-text-muted)] mb-2", children: "Custom Range" }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "space-y-2", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            TimeInputWithCalendar,
            {
              value: startInput,
              onChange: (value) => {
                setStartInput(value);
                handleCustomInputChange();
              },
              onEnter: handleConfirm,
              label: "Start Time",
              placeholder: "e.g., 2h ago, 2025-12-02 13:00"
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            TimeInputWithCalendar,
            {
              value: endInput,
              onChange: (value) => {
                setEndInput(value);
                handleCustomInputChange();
              },
              onEnter: handleConfirm,
              label: "End Time",
              placeholder: "e.g., now, 2025-12-02 15:00"
            }
          )
        ] }),
        validationError && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "mt-2 text-xs text-red-400 bg-red-900/20 border border-red-500/40 rounded px-2 py-1", children: validationError })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex justify-end", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: handleConfirm,
          className: "px-4 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white font-medium rounded-md transition-colors",
          children: "Apply"
        }
      ) })
    ] })
  ] });
};

const SelectDropdown = ({
  label,
  options,
  selected,
  onChange,
  multiple = false,
  searchable = true,
  minWidth = "160px",
  formatOption = (v) => v,
  sortOptions = true
}) => {
  const [isOpen, setIsOpen] = reactExports.useState(false);
  const [searchQuery, setSearchQuery] = reactExports.useState("");
  const [focusedIndex, setFocusedIndex] = reactExports.useState(-1);
  const dropdownRef = reactExports.useRef(null);
  const listRef = reactExports.useRef(null);
  const triggerRef = reactExports.useRef(null);
  const searchInputRef = reactExports.useRef(null);
  const selectedArray = reactExports.useMemo(() => {
    if (multiple) {
      return selected || [];
    }
    return selected ? [selected] : [];
  }, [selected, multiple]);
  const filteredOptions = reactExports.useMemo(() => {
    const processed = sortOptions ? [...options].sort((a, b) => a.localeCompare(b)) : options;
    if (!searchQuery.trim()) {
      return processed;
    }
    const query = searchQuery.toLowerCase();
    return processed.filter(
      (option) => option.toLowerCase().includes(query) || formatOption(option).toLowerCase().includes(query)
    );
  }, [options, searchQuery, sortOptions, formatOption]);
  reactExports.useEffect(() => {
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsOpen(false);
        setFocusedIndex(-1);
        setSearchQuery("");
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);
  reactExports.useEffect(() => {
    if (isOpen && searchable && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [isOpen, searchable]);
  reactExports.useEffect(() => {
    if (isOpen && focusedIndex >= 0 && listRef.current) {
      const list = listRef.current;
      const element = list.children[focusedIndex];
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
  const handleSelect = (option, closeAfter) => {
    if (multiple) {
      const currentSelected = selectedArray;
      const newSelected = currentSelected.includes(option) ? currentSelected.filter((v) => v !== option) : [...currentSelected, option];
      onChange(newSelected);
    } else {
      onChange(option);
    }
    if (closeAfter) {
      setIsOpen(false);
      setFocusedIndex(-1);
      setSearchQuery("");
      triggerRef.current?.focus();
    }
  };
  const handleClear = () => {
    onChange(multiple ? [] : null);
  };
  const handleKeyDown = (e) => {
    if (!isOpen) {
      if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        setIsOpen(true);
        setFocusedIndex(0);
      }
      return;
    }
    const target = e.target;
    if (target.tagName === "INPUT") {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        if (e.key === "ArrowDown" && filteredOptions.length > 0) {
          setFocusedIndex(0);
          searchInputRef.current?.blur();
          requestAnimationFrame(() => {
            dropdownRef.current?.focus();
          });
        } else if (e.key === "ArrowUp" && filteredOptions.length > 0) {
          setFocusedIndex(filteredOptions.length - 1);
          searchInputRef.current?.blur();
          requestAnimationFrame(() => {
            dropdownRef.current?.focus();
          });
        }
        return;
      } else if (e.key === "Escape") {
        e.preventDefault();
        setIsOpen(false);
        setFocusedIndex(-1);
        setSearchQuery("");
        triggerRef.current?.focus();
        return;
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (filteredOptions.length > 0) {
          handleSelect(filteredOptions[0], true);
        }
        return;
      }
      return;
    }
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setFocusedIndex((prev) => (prev + 1) % filteredOptions.length);
        break;
      case "ArrowUp":
        e.preventDefault();
        setFocusedIndex((prev) => (prev - 1 + filteredOptions.length) % filteredOptions.length);
        break;
      case " ":
        e.preventDefault();
        if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
          handleSelect(filteredOptions[focusedIndex], !multiple);
        }
        break;
      case "Enter":
        e.preventDefault();
        if (focusedIndex >= 0 && focusedIndex < filteredOptions.length) {
          handleSelect(filteredOptions[focusedIndex], true);
        }
        break;
      case "Escape":
        e.preventDefault();
        setIsOpen(false);
        setFocusedIndex(-1);
        setSearchQuery("");
        triggerRef.current?.focus();
        break;
      case "Tab":
        setIsOpen(false);
        setFocusedIndex(-1);
        setSearchQuery("");
        break;
    }
  };
  const displayText = reactExports.useMemo(() => {
    if (selectedArray.length === 0) {
      return label;
    }
    if (selectedArray.length === 1) {
      return formatOption(selectedArray[0]);
    }
    return selectedArray.map(formatOption).join(", ");
  }, [selectedArray, label, formatOption]);
  const hasSelection = selectedArray.length > 0;
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative", ref: dropdownRef, onKeyDown: handleKeyDown, tabIndex: -1, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs(
      "button",
      {
        ref: triggerRef,
        onClick: () => {
          setIsOpen(!isOpen);
          if (!isOpen) {
            setFocusedIndex(0);
            setSearchQuery("");
          }
        },
        style: { minWidth },
        className: `flex items-center justify-between gap-2 px-4 py-2 text-sm font-medium rounded-md border transition-all outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 ${hasSelection ? "bg-[var(--color-surface-active)] border-brand-500 text-brand-300 ring-1 ring-brand-500/20" : "bg-[var(--color-surface-muted)] border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)]"}`,
        "aria-haspopup": "listbox",
        "aria-expanded": isOpen,
        title: hasSelection ? selectedArray.map(formatOption).join(", ") : "",
        children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "truncate max-w-[200px]", children: displayText }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "svg",
            {
              className: `w-4 h-4 text-gray-500 transition-transform duration-200 flex-shrink-0 ${isOpen ? "rotate-180" : ""}`,
              fill: "none",
              stroke: "currentColor",
              viewBox: "0 0 24 24",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M19 9l-7 7-7-7" })
            }
          )
        ]
      }
    ),
    isOpen && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute top-full left-0 mt-2 w-56 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl z-[60] overflow-hidden animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10", children: [
      (searchable || multiple && hasSelection) && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-2 border-b border-[var(--color-border-soft)] space-y-2", children: [
        searchable && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-y-0 left-0 pl-2 flex items-center pointer-events-none", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "h-3.5 w-3.5 text-gray-500", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "input",
            {
              ref: searchInputRef,
              type: "text",
              value: searchQuery,
              onChange: (e) => {
                setSearchQuery(e.target.value);
                setFocusedIndex(-1);
              },
              placeholder: "Search...",
              className: "w-full pl-7 pr-2 py-1.5 text-xs border border-[var(--color-border-soft)] rounded-md bg-[var(--color-input-bg)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500",
              onClick: (e) => e.stopPropagation()
            }
          )
        ] }),
        multiple && hasSelection && /* @__PURE__ */ jsxRuntimeExports.jsxs(
          "button",
          {
            onClick: (e) => {
              e.preventDefault();
              e.stopPropagation();
              handleClear();
            },
            className: "w-full px-3 py-1.5 text-xs font-medium text-[var(--color-text-primary)] bg-[var(--color-surface-muted)] hover:bg-[var(--color-surface-active)] border border-[var(--color-border-soft)] rounded-md transition-colors flex items-center justify-center gap-1.5",
            children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-3.5 h-3.5", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M6 18L18 6M6 6l12 12" }) }),
              "Clear filter (",
              selectedArray.length,
              ")"
            ]
          }
        )
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "div",
        {
          className: "max-h-64 overflow-y-auto p-2 space-y-1 custom-scrollbar outline-none",
          role: "listbox",
          ref: listRef,
          "aria-multiselectable": multiple,
          tabIndex: 0,
          children: filteredOptions.length === 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "px-3 py-2 text-xs text-[var(--color-text-muted)] text-center", children: "No matches found" }) : filteredOptions.map((option, index) => {
            const isSelected = selectedArray.includes(option);
            const isFocused = index === focusedIndex;
            return /* @__PURE__ */ jsxRuntimeExports.jsxs(
              "div",
              {
                role: "option",
                "aria-selected": isSelected,
                onClick: () => handleSelect(option, !multiple),
                onMouseEnter: () => setFocusedIndex(index),
                className: `flex items-center px-3 py-2 rounded-md cursor-pointer transition-colors ${isFocused ? "bg-[var(--color-surface-muted)]" : ""} ${isSelected ? "bg-brand-900/30 text-brand-200" : "text-[var(--color-text-muted)] hover:bg-[var(--color-surface-muted)] hover:text-[var(--color-text-primary)]"}`,
                children: [
                  multiple && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `w-4 h-4 rounded border mr-3 flex items-center justify-center transition-all ${isSelected ? "bg-brand-600 border-brand-600" : "border-[var(--color-border-soft)]"}`, children: isSelected && /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-3 h-3 text-white", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M5 13l4 4L19 7" }) }) }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-sm select-none", children: formatOption(option) })
                ]
              },
              option
            );
          })
        }
      )
    ] })
  ] });
};

const FilterBar = ({ filters, setFilters, timeRange, onTimeRangeChange, availableNamespaces, availableKinds, rawStart, rawEnd }) => {
  const handleSearchChange = (e) => {
    setFilters((prev) => ({ ...prev, search: e.target.value }));
  };
  const handleNamespacesChange = (values) => {
    setFilters((prev) => ({ ...prev, namespaces: values || [] }));
  };
  const handleKindsChange = (values) => {
    setFilters((prev) => ({ ...prev, kinds: values || [] }));
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "w-full bg-[var(--color-surface-elevated)]/95 backdrop-blur border-b border-[var(--color-border-soft)] p-4 flex flex-row gap-6 items-center shadow-lg z-30 text-[var(--color-text-primary)] transition-colors duration-300", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative group min-w-[150px] flex-grow max-w-xl", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "h-4 w-4 text-gray-500 group-focus-within:text-brand-500 transition-colors", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" }) }) }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "input",
        {
          type: "text",
          placeholder: "Search resources by name...",
          value: filters.search,
          onChange: handleSearchChange,
          className: "block w-full pl-10 pr-3 py-2 border border-[var(--color-border-soft)] rounded-md leading-5 bg-[var(--color-input-bg)] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] focus:outline-none focus:bg-[var(--color-surface-muted)] focus:border-brand-500 focus:ring-1 focus:ring-brand-500 sm:text-sm transition-all shadow-sm"
        }
      )
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex gap-3", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        SelectDropdown,
        {
          label: "All Namespaces",
          options: availableNamespaces,
          selected: filters.namespaces,
          onChange: handleNamespacesChange,
          multiple: true
        }
      ),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        SelectDropdown,
        {
          label: "All Kinds",
          options: availableKinds,
          selected: filters.kinds,
          onChange: handleKindsChange,
          multiple: true
        }
      ),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: () => setFilters((prev) => ({ ...prev, hasProblematicStatus: !prev.hasProblematicStatus })),
          className: `flex items-center justify-between gap-2 px-4 py-2 text-sm font-medium rounded-md border transition-all outline-none focus:ring-2 focus:ring-brand-500 focus:border-brand-500 ${filters.hasProblematicStatus ? "bg-[var(--color-surface-active)] border-brand-500 text-brand-300 ring-1 ring-brand-500/20" : "bg-[var(--color-surface-muted)] border-[var(--color-border-soft)] text-[var(--color-text-muted)] hover:border-brand-500 hover:text-[var(--color-text-primary)]"}`,
          title: "Show only resources with problematic status (Warning, Error, Terminating, or Unknown)",
          children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `w-4 h-4 rounded border flex items-center justify-center transition-all ${filters.hasProblematicStatus ? "bg-brand-600 border-brand-600" : "border-[var(--color-border-soft)]"}`, children: filters.hasProblematicStatus && /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-3 h-3 text-white", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M5 13l4 4L19 7" }) }) }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "truncate", children: "Problematic Only" })
          ] })
        }
      )
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "ml-auto flex items-center gap-3", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
      TimeRangeDropdown,
      {
        currentRange: timeRange,
        onConfirm: onTimeRangeChange,
        rawStart,
        rawEnd
      }
    ) })
  ] });
};

function ascending$3(a, b) {
  return a == null || b == null ? NaN : a < b ? -1 : a > b ? 1 : a >= b ? 0 : NaN;
}

function descending$2(a, b) {
  return a == null || b == null ? NaN
    : b < a ? -1
    : b > a ? 1
    : b >= a ? 0
    : NaN;
}

function bisector(f) {
  let compare1, compare2, delta;

  // If an accessor is specified, promote it to a comparator. In this case we
  // can test whether the search value is (self-) comparable. We can’t do this
  // for a comparator (except for specific, known comparators) because we can’t
  // tell if the comparator is symmetric, and an asymmetric comparator can’t be
  // used to test whether a single value is comparable.
  if (f.length !== 2) {
    compare1 = ascending$3;
    compare2 = (d, x) => ascending$3(f(d), x);
    delta = (d, x) => f(d) - x;
  } else {
    compare1 = f === ascending$3 || f === descending$2 ? f : zero$1;
    compare2 = f;
    delta = f;
  }

  function left(a, x, lo = 0, hi = a.length) {
    if (lo < hi) {
      if (compare1(x, x) !== 0) return hi;
      do {
        const mid = (lo + hi) >>> 1;
        if (compare2(a[mid], x) < 0) lo = mid + 1;
        else hi = mid;
      } while (lo < hi);
    }
    return lo;
  }

  function right(a, x, lo = 0, hi = a.length) {
    if (lo < hi) {
      if (compare1(x, x) !== 0) return hi;
      do {
        const mid = (lo + hi) >>> 1;
        if (compare2(a[mid], x) <= 0) lo = mid + 1;
        else hi = mid;
      } while (lo < hi);
    }
    return lo;
  }

  function center(a, x, lo = 0, hi = a.length) {
    const i = left(a, x, lo, hi - 1);
    return i > lo && delta(a[i - 1], x) > -delta(a[i], x) ? i - 1 : i;
  }

  return {left, center, right};
}

function zero$1() {
  return 0;
}

function number$3(x) {
  return x === null ? NaN : +x;
}

function* numbers(values, valueof) {
  if (valueof === undefined) {
    for (let value of values) {
      if (value != null && (value = +value) >= value) {
        yield value;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null && (value = +value) >= value) {
        yield value;
      }
    }
  }
}

const ascendingBisect = bisector(ascending$3);
const bisectRight = ascendingBisect.right;
const bisectLeft = ascendingBisect.left;
const bisectCenter = bisector(number$3).center;

function blur(values, r) {
  if (!((r = +r) >= 0)) throw new RangeError("invalid r");
  let length = values.length;
  if (!((length = Math.floor(length)) >= 0)) throw new RangeError("invalid length");
  if (!length || !r) return values;
  const blur = blurf(r);
  const temp = values.slice();
  blur(values, temp, 0, length, 1);
  blur(temp, values, 0, length, 1);
  blur(values, temp, 0, length, 1);
  return values;
}

const blur2 = Blur2(blurf);

const blurImage = Blur2(blurfImage);

function Blur2(blur) {
  return function(data, rx, ry = rx) {
    if (!((rx = +rx) >= 0)) throw new RangeError("invalid rx");
    if (!((ry = +ry) >= 0)) throw new RangeError("invalid ry");
    let {data: values, width, height} = data;
    if (!((width = Math.floor(width)) >= 0)) throw new RangeError("invalid width");
    if (!((height = Math.floor(height !== undefined ? height : values.length / width)) >= 0)) throw new RangeError("invalid height");
    if (!width || !height || (!rx && !ry)) return data;
    const blurx = rx && blur(rx);
    const blury = ry && blur(ry);
    const temp = values.slice();
    if (blurx && blury) {
      blurh(blurx, temp, values, width, height);
      blurh(blurx, values, temp, width, height);
      blurh(blurx, temp, values, width, height);
      blurv(blury, values, temp, width, height);
      blurv(blury, temp, values, width, height);
      blurv(blury, values, temp, width, height);
    } else if (blurx) {
      blurh(blurx, values, temp, width, height);
      blurh(blurx, temp, values, width, height);
      blurh(blurx, values, temp, width, height);
    } else if (blury) {
      blurv(blury, values, temp, width, height);
      blurv(blury, temp, values, width, height);
      blurv(blury, values, temp, width, height);
    }
    return data;
  };
}

function blurh(blur, T, S, w, h) {
  for (let y = 0, n = w * h; y < n;) {
    blur(T, S, y, y += w, 1);
  }
}

function blurv(blur, T, S, w, h) {
  for (let x = 0, n = w * h; x < w; ++x) {
    blur(T, S, x, x + n, w);
  }
}

function blurfImage(radius) {
  const blur = blurf(radius);
  return (T, S, start, stop, step) => {
    start <<= 2, stop <<= 2, step <<= 2;
    blur(T, S, start + 0, stop + 0, step);
    blur(T, S, start + 1, stop + 1, step);
    blur(T, S, start + 2, stop + 2, step);
    blur(T, S, start + 3, stop + 3, step);
  };
}

// Given a target array T, a source array S, sets each value T[i] to the average
// of {S[i - r], …, S[i], …, S[i + r]}, where r = ⌊radius⌋, start <= i < stop,
// for each i, i + step, i + 2 * step, etc., and where S[j] is clamped between
// S[start] (inclusive) and S[stop] (exclusive). If the given radius is not an
// integer, S[i - r - 1] and S[i + r + 1] are added to the sum, each weighted
// according to r - ⌊radius⌋.
function blurf(radius) {
  const radius0 = Math.floor(radius);
  if (radius0 === radius) return bluri(radius);
  const t = radius - radius0;
  const w = 2 * radius + 1;
  return (T, S, start, stop, step) => { // stop must be aligned!
    if (!((stop -= step) >= start)) return; // inclusive stop
    let sum = radius0 * S[start];
    const s0 = step * radius0;
    const s1 = s0 + step;
    for (let i = start, j = start + s0; i < j; i += step) {
      sum += S[Math.min(stop, i)];
    }
    for (let i = start, j = stop; i <= j; i += step) {
      sum += S[Math.min(stop, i + s0)];
      T[i] = (sum + t * (S[Math.max(start, i - s1)] + S[Math.min(stop, i + s1)])) / w;
      sum -= S[Math.max(start, i - s0)];
    }
  };
}

// Like blurf, but optimized for integer radius.
function bluri(radius) {
  const w = 2 * radius + 1;
  return (T, S, start, stop, step) => { // stop must be aligned!
    if (!((stop -= step) >= start)) return; // inclusive stop
    let sum = radius * S[start];
    const s = step * radius;
    for (let i = start, j = start + s; i < j; i += step) {
      sum += S[Math.min(stop, i)];
    }
    for (let i = start, j = stop; i <= j; i += step) {
      sum += S[Math.min(stop, i + s)];
      T[i] = sum / w;
      sum -= S[Math.max(start, i - s)];
    }
  };
}

function count$1(values, valueof) {
  let count = 0;
  if (valueof === undefined) {
    for (let value of values) {
      if (value != null && (value = +value) >= value) {
        ++count;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null && (value = +value) >= value) {
        ++count;
      }
    }
  }
  return count;
}

function length$3(array) {
  return array.length | 0;
}

function empty$2(length) {
  return !(length > 0);
}

function arrayify(values) {
  return typeof values !== "object" || "length" in values ? values : Array.from(values);
}

function reducer(reduce) {
  return values => reduce(...values);
}

function cross$2(...values) {
  const reduce = typeof values[values.length - 1] === "function" && reducer(values.pop());
  values = values.map(arrayify);
  const lengths = values.map(length$3);
  const j = values.length - 1;
  const index = new Array(j + 1).fill(0);
  const product = [];
  if (j < 0 || lengths.some(empty$2)) return product;
  while (true) {
    product.push(index.map((j, i) => values[i][j]));
    let i = j;
    while (++index[i] === lengths[i]) {
      if (i === 0) return reduce ? product.map(reduce) : product;
      index[i--] = 0;
    }
  }
}

function cumsum(values, valueof) {
  var sum = 0, index = 0;
  return Float64Array.from(values, valueof === undefined
    ? v => (sum += +v || 0)
    : v => (sum += +valueof(v, index++, values) || 0));
}

function variance(values, valueof) {
  let count = 0;
  let delta;
  let mean = 0;
  let sum = 0;
  if (valueof === undefined) {
    for (let value of values) {
      if (value != null && (value = +value) >= value) {
        delta = value - mean;
        mean += delta / ++count;
        sum += delta * (value - mean);
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null && (value = +value) >= value) {
        delta = value - mean;
        mean += delta / ++count;
        sum += delta * (value - mean);
      }
    }
  }
  if (count > 1) return sum / (count - 1);
}

function deviation(values, valueof) {
  const v = variance(values, valueof);
  return v ? Math.sqrt(v) : v;
}

function extent$1(values, valueof) {
  let min;
  let max;
  if (valueof === undefined) {
    for (const value of values) {
      if (value != null) {
        if (min === undefined) {
          if (value >= value) min = max = value;
        } else {
          if (min > value) min = value;
          if (max < value) max = value;
        }
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null) {
        if (min === undefined) {
          if (value >= value) min = max = value;
        } else {
          if (min > value) min = value;
          if (max < value) max = value;
        }
      }
    }
  }
  return [min, max];
}

// https://github.com/python/cpython/blob/a74eea238f5baba15797e2e8b570d153bc8690a7/Modules/mathmodule.c#L1423
class Adder {
  constructor() {
    this._partials = new Float64Array(32);
    this._n = 0;
  }
  add(x) {
    const p = this._partials;
    let i = 0;
    for (let j = 0; j < this._n && j < 32; j++) {
      const y = p[j],
        hi = x + y,
        lo = Math.abs(x) < Math.abs(y) ? x - (hi - y) : y - (hi - x);
      if (lo) p[i++] = lo;
      x = hi;
    }
    p[i] = x;
    this._n = i + 1;
    return this;
  }
  valueOf() {
    const p = this._partials;
    let n = this._n, x, y, lo, hi = 0;
    if (n > 0) {
      hi = p[--n];
      while (n > 0) {
        x = hi;
        y = p[--n];
        hi = x + y;
        lo = y - (hi - x);
        if (lo) break;
      }
      if (n > 0 && ((lo < 0 && p[n - 1] < 0) || (lo > 0 && p[n - 1] > 0))) {
        y = lo * 2;
        x = hi + y;
        if (y == x - hi) hi = x;
      }
    }
    return hi;
  }
}

function fsum(values, valueof) {
  const adder = new Adder();
  if (valueof === undefined) {
    for (let value of values) {
      if (value = +value) {
        adder.add(value);
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if (value = +valueof(value, ++index, values)) {
        adder.add(value);
      }
    }
  }
  return +adder;
}

function fcumsum(values, valueof) {
  const adder = new Adder();
  let index = -1;
  return Float64Array.from(values, valueof === undefined
      ? v => adder.add(+v || 0)
      : v => adder.add(+valueof(v, ++index, values) || 0)
  );
}

class InternMap extends Map {
  constructor(entries, key = keyof) {
    super();
    Object.defineProperties(this, {_intern: {value: new Map()}, _key: {value: key}});
    if (entries != null) for (const [key, value] of entries) this.set(key, value);
  }
  get(key) {
    return super.get(intern_get(this, key));
  }
  has(key) {
    return super.has(intern_get(this, key));
  }
  set(key, value) {
    return super.set(intern_set(this, key), value);
  }
  delete(key) {
    return super.delete(intern_delete(this, key));
  }
}

class InternSet extends Set {
  constructor(values, key = keyof) {
    super();
    Object.defineProperties(this, {_intern: {value: new Map()}, _key: {value: key}});
    if (values != null) for (const value of values) this.add(value);
  }
  has(value) {
    return super.has(intern_get(this, value));
  }
  add(value) {
    return super.add(intern_set(this, value));
  }
  delete(value) {
    return super.delete(intern_delete(this, value));
  }
}

function intern_get({_intern, _key}, value) {
  const key = _key(value);
  return _intern.has(key) ? _intern.get(key) : value;
}

function intern_set({_intern, _key}, value) {
  const key = _key(value);
  if (_intern.has(key)) return _intern.get(key);
  _intern.set(key, value);
  return value;
}

function intern_delete({_intern, _key}, value) {
  const key = _key(value);
  if (_intern.has(key)) {
    value = _intern.get(key);
    _intern.delete(key);
  }
  return value;
}

function keyof(value) {
  return value !== null && typeof value === "object" ? value.valueOf() : value;
}

function identity$9(x) {
  return x;
}

function group(values, ...keys) {
  return nest(values, identity$9, identity$9, keys);
}

function groups(values, ...keys) {
  return nest(values, Array.from, identity$9, keys);
}

function flatten$1(groups, keys) {
  for (let i = 1, n = keys.length; i < n; ++i) {
    groups = groups.flatMap(g => g.pop().map(([key, value]) => [...g, key, value]));
  }
  return groups;
}

function flatGroup(values, ...keys) {
  return flatten$1(groups(values, ...keys), keys);
}

function flatRollup(values, reduce, ...keys) {
  return flatten$1(rollups(values, reduce, ...keys), keys);
}

function rollup(values, reduce, ...keys) {
  return nest(values, identity$9, reduce, keys);
}

function rollups(values, reduce, ...keys) {
  return nest(values, Array.from, reduce, keys);
}

function index$4(values, ...keys) {
  return nest(values, identity$9, unique, keys);
}

function indexes(values, ...keys) {
  return nest(values, Array.from, unique, keys);
}

function unique(values) {
  if (values.length !== 1) throw new Error("duplicate key");
  return values[0];
}

function nest(values, map, reduce, keys) {
  return (function regroup(values, i) {
    if (i >= keys.length) return reduce(values);
    const groups = new InternMap();
    const keyof = keys[i++];
    let index = -1;
    for (const value of values) {
      const key = keyof(value, ++index, values);
      const group = groups.get(key);
      if (group) group.push(value);
      else groups.set(key, [value]);
    }
    for (const [key, values] of groups) {
      groups.set(key, regroup(values, i));
    }
    return map(groups);
  })(values, 0);
}

function permute(source, keys) {
  return Array.from(keys, key => source[key]);
}

function sort(values, ...F) {
  if (typeof values[Symbol.iterator] !== "function") throw new TypeError("values is not iterable");
  values = Array.from(values);
  let [f] = F;
  if ((f && f.length !== 2) || F.length > 1) {
    const index = Uint32Array.from(values, (d, i) => i);
    if (F.length > 1) {
      F = F.map(f => values.map(f));
      index.sort((i, j) => {
        for (const f of F) {
          const c = ascendingDefined(f[i], f[j]);
          if (c) return c;
        }
      });
    } else {
      f = values.map(f);
      index.sort((i, j) => ascendingDefined(f[i], f[j]));
    }
    return permute(values, index);
  }
  return values.sort(compareDefined(f));
}

function compareDefined(compare = ascending$3) {
  if (compare === ascending$3) return ascendingDefined;
  if (typeof compare !== "function") throw new TypeError("compare is not a function");
  return (a, b) => {
    const x = compare(a, b);
    if (x || x === 0) return x;
    return (compare(b, b) === 0) - (compare(a, a) === 0);
  };
}

function ascendingDefined(a, b) {
  return (a == null || !(a >= a)) - (b == null || !(b >= b)) || (a < b ? -1 : a > b ? 1 : 0);
}

function groupSort(values, reduce, key) {
  return (reduce.length !== 2
    ? sort(rollup(values, reduce, key), (([ak, av], [bk, bv]) => ascending$3(av, bv) || ascending$3(ak, bk)))
    : sort(group(values, key), (([ak, av], [bk, bv]) => reduce(av, bv) || ascending$3(ak, bk))))
    .map(([key]) => key);
}

var array$5 = Array.prototype;

var slice$3 = array$5.slice;
var map$2 = array$5.map;

function constant$b(x) {
  return () => x;
}

const e10 = Math.sqrt(50),
    e5 = Math.sqrt(10),
    e2 = Math.sqrt(2);

function tickSpec(start, stop, count) {
  const step = (stop - start) / Math.max(0, count),
      power = Math.floor(Math.log10(step)),
      error = step / Math.pow(10, power),
      factor = error >= e10 ? 10 : error >= e5 ? 5 : error >= e2 ? 2 : 1;
  let i1, i2, inc;
  if (power < 0) {
    inc = Math.pow(10, -power) / factor;
    i1 = Math.round(start * inc);
    i2 = Math.round(stop * inc);
    if (i1 / inc < start) ++i1;
    if (i2 / inc > stop) --i2;
    inc = -inc;
  } else {
    inc = Math.pow(10, power) * factor;
    i1 = Math.round(start / inc);
    i2 = Math.round(stop / inc);
    if (i1 * inc < start) ++i1;
    if (i2 * inc > stop) --i2;
  }
  if (i2 < i1 && 0.5 <= count && count < 2) return tickSpec(start, stop, count * 2);
  return [i1, i2, inc];
}

function ticks(start, stop, count) {
  stop = +stop, start = +start, count = +count;
  if (!(count > 0)) return [];
  if (start === stop) return [start];
  const reverse = stop < start, [i1, i2, inc] = reverse ? tickSpec(stop, start, count) : tickSpec(start, stop, count);
  if (!(i2 >= i1)) return [];
  const n = i2 - i1 + 1, ticks = new Array(n);
  if (reverse) {
    if (inc < 0) for (let i = 0; i < n; ++i) ticks[i] = (i2 - i) / -inc;
    else for (let i = 0; i < n; ++i) ticks[i] = (i2 - i) * inc;
  } else {
    if (inc < 0) for (let i = 0; i < n; ++i) ticks[i] = (i1 + i) / -inc;
    else for (let i = 0; i < n; ++i) ticks[i] = (i1 + i) * inc;
  }
  return ticks;
}

function tickIncrement(start, stop, count) {
  stop = +stop, start = +start, count = +count;
  return tickSpec(start, stop, count)[2];
}

function tickStep(start, stop, count) {
  stop = +stop, start = +start, count = +count;
  const reverse = stop < start, inc = reverse ? tickIncrement(stop, start, count) : tickIncrement(start, stop, count);
  return (reverse ? -1 : 1) * (inc < 0 ? 1 / -inc : inc);
}

function nice$1(start, stop, count) {
  let prestep;
  while (true) {
    const step = tickIncrement(start, stop, count);
    if (step === prestep || step === 0 || !isFinite(step)) {
      return [start, stop];
    } else if (step > 0) {
      start = Math.floor(start / step) * step;
      stop = Math.ceil(stop / step) * step;
    } else if (step < 0) {
      start = Math.ceil(start * step) / step;
      stop = Math.floor(stop * step) / step;
    }
    prestep = step;
  }
}

function thresholdSturges(values) {
  return Math.max(1, Math.ceil(Math.log(count$1(values)) / Math.LN2) + 1);
}

function bin() {
  var value = identity$9,
      domain = extent$1,
      threshold = thresholdSturges;

  function histogram(data) {
    if (!Array.isArray(data)) data = Array.from(data);

    var i,
        n = data.length,
        x,
        step,
        values = new Array(n);

    for (i = 0; i < n; ++i) {
      values[i] = value(data[i], i, data);
    }

    var xz = domain(values),
        x0 = xz[0],
        x1 = xz[1],
        tz = threshold(values, x0, x1);

    // Convert number of thresholds into uniform thresholds, and nice the
    // default domain accordingly.
    if (!Array.isArray(tz)) {
      const max = x1, tn = +tz;
      if (domain === extent$1) [x0, x1] = nice$1(x0, x1, tn);
      tz = ticks(x0, x1, tn);

      // If the domain is aligned with the first tick (which it will by
      // default), then we can use quantization rather than bisection to bin
      // values, which is substantially faster.
      if (tz[0] <= x0) step = tickIncrement(x0, x1, tn);

      // If the last threshold is coincident with the domain’s upper bound, the
      // last bin will be zero-width. If the default domain is used, and this
      // last threshold is coincident with the maximum input value, we can
      // extend the niced upper bound by one tick to ensure uniform bin widths;
      // otherwise, we simply remove the last threshold. Note that we don’t
      // coerce values or the domain to numbers, and thus must be careful to
      // compare order (>=) rather than strict equality (===)!
      if (tz[tz.length - 1] >= x1) {
        if (max >= x1 && domain === extent$1) {
          const step = tickIncrement(x0, x1, tn);
          if (isFinite(step)) {
            if (step > 0) {
              x1 = (Math.floor(x1 / step) + 1) * step;
            } else if (step < 0) {
              x1 = (Math.ceil(x1 * -step) + 1) / -step;
            }
          }
        } else {
          tz.pop();
        }
      }
    }

    // Remove any thresholds outside the domain.
    // Be careful not to mutate an array owned by the user!
    var m = tz.length, a = 0, b = m;
    while (tz[a] <= x0) ++a;
    while (tz[b - 1] > x1) --b;
    if (a || b < m) tz = tz.slice(a, b), m = b - a;

    var bins = new Array(m + 1),
        bin;

    // Initialize bins.
    for (i = 0; i <= m; ++i) {
      bin = bins[i] = [];
      bin.x0 = i > 0 ? tz[i - 1] : x0;
      bin.x1 = i < m ? tz[i] : x1;
    }

    // Assign data to bins by value, ignoring any outside the domain.
    if (isFinite(step)) {
      if (step > 0) {
        for (i = 0; i < n; ++i) {
          if ((x = values[i]) != null && x0 <= x && x <= x1) {
            bins[Math.min(m, Math.floor((x - x0) / step))].push(data[i]);
          }
        }
      } else if (step < 0) {
        for (i = 0; i < n; ++i) {
          if ((x = values[i]) != null && x0 <= x && x <= x1) {
            const j = Math.floor((x0 - x) * step);
            bins[Math.min(m, j + (tz[j] <= x))].push(data[i]); // handle off-by-one due to rounding
          }
        }
      }
    } else {
      for (i = 0; i < n; ++i) {
        if ((x = values[i]) != null && x0 <= x && x <= x1) {
          bins[bisectRight(tz, x, 0, m)].push(data[i]);
        }
      }
    }

    return bins;
  }

  histogram.value = function(_) {
    return arguments.length ? (value = typeof _ === "function" ? _ : constant$b(_), histogram) : value;
  };

  histogram.domain = function(_) {
    return arguments.length ? (domain = typeof _ === "function" ? _ : constant$b([_[0], _[1]]), histogram) : domain;
  };

  histogram.thresholds = function(_) {
    return arguments.length ? (threshold = typeof _ === "function" ? _ : constant$b(Array.isArray(_) ? slice$3.call(_) : _), histogram) : threshold;
  };

  return histogram;
}

function max$3(values, valueof) {
  let max;
  if (valueof === undefined) {
    for (const value of values) {
      if (value != null
          && (max < value || (max === undefined && value >= value))) {
        max = value;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null
          && (max < value || (max === undefined && value >= value))) {
        max = value;
      }
    }
  }
  return max;
}

function maxIndex(values, valueof) {
  let max;
  let maxIndex = -1;
  let index = -1;
  if (valueof === undefined) {
    for (const value of values) {
      ++index;
      if (value != null
          && (max < value || (max === undefined && value >= value))) {
        max = value, maxIndex = index;
      }
    }
  } else {
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null
          && (max < value || (max === undefined && value >= value))) {
        max = value, maxIndex = index;
      }
    }
  }
  return maxIndex;
}

function min$2(values, valueof) {
  let min;
  if (valueof === undefined) {
    for (const value of values) {
      if (value != null
          && (min > value || (min === undefined && value >= value))) {
        min = value;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null
          && (min > value || (min === undefined && value >= value))) {
        min = value;
      }
    }
  }
  return min;
}

function minIndex(values, valueof) {
  let min;
  let minIndex = -1;
  let index = -1;
  if (valueof === undefined) {
    for (const value of values) {
      ++index;
      if (value != null
          && (min > value || (min === undefined && value >= value))) {
        min = value, minIndex = index;
      }
    }
  } else {
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null
          && (min > value || (min === undefined && value >= value))) {
        min = value, minIndex = index;
      }
    }
  }
  return minIndex;
}

// Based on https://github.com/mourner/quickselect
// ISC license, Copyright 2018 Vladimir Agafonkin.
function quickselect(array, k, left = 0, right = Infinity, compare) {
  k = Math.floor(k);
  left = Math.floor(Math.max(0, left));
  right = Math.floor(Math.min(array.length - 1, right));

  if (!(left <= k && k <= right)) return array;

  compare = compare === undefined ? ascendingDefined : compareDefined(compare);

  while (right > left) {
    if (right - left > 600) {
      const n = right - left + 1;
      const m = k - left + 1;
      const z = Math.log(n);
      const s = 0.5 * Math.exp(2 * z / 3);
      const sd = 0.5 * Math.sqrt(z * s * (n - s) / n) * (m - n / 2 < 0 ? -1 : 1);
      const newLeft = Math.max(left, Math.floor(k - m * s / n + sd));
      const newRight = Math.min(right, Math.floor(k + (n - m) * s / n + sd));
      quickselect(array, k, newLeft, newRight, compare);
    }

    const t = array[k];
    let i = left;
    let j = right;

    swap$1(array, left, k);
    if (compare(array[right], t) > 0) swap$1(array, left, right);

    while (i < j) {
      swap$1(array, i, j), ++i, --j;
      while (compare(array[i], t) < 0) ++i;
      while (compare(array[j], t) > 0) --j;
    }

    if (compare(array[left], t) === 0) swap$1(array, left, j);
    else ++j, swap$1(array, j, right);

    if (j <= k) left = j + 1;
    if (k <= j) right = j - 1;
  }

  return array;
}

function swap$1(array, i, j) {
  const t = array[i];
  array[i] = array[j];
  array[j] = t;
}

function greatest(values, compare = ascending$3) {
  let max;
  let defined = false;
  if (compare.length === 1) {
    let maxValue;
    for (const element of values) {
      const value = compare(element);
      if (defined
          ? ascending$3(value, maxValue) > 0
          : ascending$3(value, value) === 0) {
        max = element;
        maxValue = value;
        defined = true;
      }
    }
  } else {
    for (const value of values) {
      if (defined
          ? compare(value, max) > 0
          : compare(value, value) === 0) {
        max = value;
        defined = true;
      }
    }
  }
  return max;
}

function quantile$1(values, p, valueof) {
  values = Float64Array.from(numbers(values, valueof));
  if (!(n = values.length) || isNaN(p = +p)) return;
  if (p <= 0 || n < 2) return min$2(values);
  if (p >= 1) return max$3(values);
  var n,
      i = (n - 1) * p,
      i0 = Math.floor(i),
      value0 = max$3(quickselect(values, i0).subarray(0, i0 + 1)),
      value1 = min$2(values.subarray(i0 + 1));
  return value0 + (value1 - value0) * (i - i0);
}

function quantileSorted(values, p, valueof = number$3) {
  if (!(n = values.length) || isNaN(p = +p)) return;
  if (p <= 0 || n < 2) return +valueof(values[0], 0, values);
  if (p >= 1) return +valueof(values[n - 1], n - 1, values);
  var n,
      i = (n - 1) * p,
      i0 = Math.floor(i),
      value0 = +valueof(values[i0], i0, values),
      value1 = +valueof(values[i0 + 1], i0 + 1, values);
  return value0 + (value1 - value0) * (i - i0);
}

function quantileIndex(values, p, valueof = number$3) {
  if (isNaN(p = +p)) return;
  numbers = Float64Array.from(values, (_, i) => number$3(valueof(values[i], i, values)));
  if (p <= 0) return minIndex(numbers);
  if (p >= 1) return maxIndex(numbers);
  var numbers,
      index = Uint32Array.from(values, (_, i) => i),
      j = numbers.length - 1,
      i = Math.floor(j * p);
  quickselect(index, i, 0, j, (i, j) => ascendingDefined(numbers[i], numbers[j]));
  i = greatest(index.subarray(0, i + 1), (i) => numbers[i]);
  return i >= 0 ? i : -1;
}

function thresholdFreedmanDiaconis(values, min, max) {
  const c = count$1(values), d = quantile$1(values, 0.75) - quantile$1(values, 0.25);
  return c && d ? Math.ceil((max - min) / (2 * d * Math.pow(c, -1 / 3))) : 1;
}

function thresholdScott(values, min, max) {
  const c = count$1(values), d = deviation(values);
  return c && d ? Math.ceil((max - min) * Math.cbrt(c) / (3.49 * d)) : 1;
}

function mean(values, valueof) {
  let count = 0;
  let sum = 0;
  if (valueof === undefined) {
    for (let value of values) {
      if (value != null && (value = +value) >= value) {
        ++count, sum += value;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null && (value = +value) >= value) {
        ++count, sum += value;
      }
    }
  }
  if (count) return sum / count;
}

function median(values, valueof) {
  return quantile$1(values, 0.5, valueof);
}

function medianIndex(values, valueof) {
  return quantileIndex(values, 0.5, valueof);
}

function* flatten(arrays) {
  for (const array of arrays) {
    yield* array;
  }
}

function merge(arrays) {
  return Array.from(flatten(arrays));
}

function mode(values, valueof) {
  const counts = new InternMap();
  if (valueof === undefined) {
    for (let value of values) {
      if (value != null && value >= value) {
        counts.set(value, (counts.get(value) || 0) + 1);
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if ((value = valueof(value, ++index, values)) != null && value >= value) {
        counts.set(value, (counts.get(value) || 0) + 1);
      }
    }
  }
  let modeValue;
  let modeCount = 0;
  for (const [value, count] of counts) {
    if (count > modeCount) {
      modeCount = count;
      modeValue = value;
    }
  }
  return modeValue;
}

function pairs(values, pairof = pair) {
  const pairs = [];
  let previous;
  let first = false;
  for (const value of values) {
    if (first) pairs.push(pairof(previous, value));
    previous = value;
    first = true;
  }
  return pairs;
}

function pair(a, b) {
  return [a, b];
}

function range$2(start, stop, step) {
  start = +start, stop = +stop, step = (n = arguments.length) < 2 ? (stop = start, start = 0, 1) : n < 3 ? 1 : +step;

  var i = -1,
      n = Math.max(0, Math.ceil((stop - start) / step)) | 0,
      range = new Array(n);

  while (++i < n) {
    range[i] = start + i * step;
  }

  return range;
}

function rank(values, valueof = ascending$3) {
  if (typeof values[Symbol.iterator] !== "function") throw new TypeError("values is not iterable");
  let V = Array.from(values);
  const R = new Float64Array(V.length);
  if (valueof.length !== 2) V = V.map(valueof), valueof = ascending$3;
  const compareIndex = (i, j) => valueof(V[i], V[j]);
  let k, r;
  values = Uint32Array.from(V, (_, i) => i);
  // Risky chaining due to Safari 14 https://github.com/d3/d3-array/issues/123
  values.sort(valueof === ascending$3 ? (i, j) => ascendingDefined(V[i], V[j]) : compareDefined(compareIndex));
  values.forEach((j, i) => {
      const c = compareIndex(j, k === undefined ? j : k);
      if (c >= 0) {
        if (k === undefined || c > 0) k = j, r = i;
        R[j] = r;
      } else {
        R[j] = NaN;
      }
    });
  return R;
}

function least(values, compare = ascending$3) {
  let min;
  let defined = false;
  if (compare.length === 1) {
    let minValue;
    for (const element of values) {
      const value = compare(element);
      if (defined
          ? ascending$3(value, minValue) < 0
          : ascending$3(value, value) === 0) {
        min = element;
        minValue = value;
        defined = true;
      }
    }
  } else {
    for (const value of values) {
      if (defined
          ? compare(value, min) < 0
          : compare(value, value) === 0) {
        min = value;
        defined = true;
      }
    }
  }
  return min;
}

function leastIndex(values, compare = ascending$3) {
  if (compare.length === 1) return minIndex(values, compare);
  let minValue;
  let min = -1;
  let index = -1;
  for (const value of values) {
    ++index;
    if (min < 0
        ? compare(value, value) === 0
        : compare(value, minValue) < 0) {
      minValue = value;
      min = index;
    }
  }
  return min;
}

function greatestIndex(values, compare = ascending$3) {
  if (compare.length === 1) return maxIndex(values, compare);
  let maxValue;
  let max = -1;
  let index = -1;
  for (const value of values) {
    ++index;
    if (max < 0
        ? compare(value, value) === 0
        : compare(value, maxValue) > 0) {
      maxValue = value;
      max = index;
    }
  }
  return max;
}

function scan(values, compare) {
  const index = leastIndex(values, compare);
  return index < 0 ? undefined : index;
}

const shuffle$1 = shuffler(Math.random);

function shuffler(random) {
  return function shuffle(array, i0 = 0, i1 = array.length) {
    let m = i1 - (i0 = +i0);
    while (m) {
      const i = random() * m-- | 0, t = array[m + i0];
      array[m + i0] = array[i + i0];
      array[i + i0] = t;
    }
    return array;
  };
}

function sum$2(values, valueof) {
  let sum = 0;
  if (valueof === undefined) {
    for (let value of values) {
      if (value = +value) {
        sum += value;
      }
    }
  } else {
    let index = -1;
    for (let value of values) {
      if (value = +valueof(value, ++index, values)) {
        sum += value;
      }
    }
  }
  return sum;
}

function transpose(matrix) {
  if (!(n = matrix.length)) return [];
  for (var i = -1, m = min$2(matrix, length$2), transpose = new Array(m); ++i < m;) {
    for (var j = -1, n, row = transpose[i] = new Array(n); ++j < n;) {
      row[j] = matrix[j][i];
    }
  }
  return transpose;
}

function length$2(d) {
  return d.length;
}

function zip() {
  return transpose(arguments);
}

function every(values, test) {
  if (typeof test !== "function") throw new TypeError("test is not a function");
  let index = -1;
  for (const value of values) {
    if (!test(value, ++index, values)) {
      return false;
    }
  }
  return true;
}

function some(values, test) {
  if (typeof test !== "function") throw new TypeError("test is not a function");
  let index = -1;
  for (const value of values) {
    if (test(value, ++index, values)) {
      return true;
    }
  }
  return false;
}

function filter$1(values, test) {
  if (typeof test !== "function") throw new TypeError("test is not a function");
  const array = [];
  let index = -1;
  for (const value of values) {
    if (test(value, ++index, values)) {
      array.push(value);
    }
  }
  return array;
}

function map$1(values, mapper) {
  if (typeof values[Symbol.iterator] !== "function") throw new TypeError("values is not iterable");
  if (typeof mapper !== "function") throw new TypeError("mapper is not a function");
  return Array.from(values, (value, index) => mapper(value, index, values));
}

function reduce(values, reducer, value) {
  if (typeof reducer !== "function") throw new TypeError("reducer is not a function");
  const iterator = values[Symbol.iterator]();
  let done, next, index = -1;
  if (arguments.length < 3) {
    ({done, value} = iterator.next());
    if (done) return;
    ++index;
  }
  while (({done, value: next} = iterator.next()), !done) {
    value = reducer(value, next, ++index, values);
  }
  return value;
}

function reverse$1(values) {
  if (typeof values[Symbol.iterator] !== "function") throw new TypeError("values is not iterable");
  return Array.from(values).reverse();
}

function difference(values, ...others) {
  values = new InternSet(values);
  for (const other of others) {
    for (const value of other) {
      values.delete(value);
    }
  }
  return values;
}

function disjoint(values, other) {
  const iterator = other[Symbol.iterator](), set = new InternSet();
  for (const v of values) {
    if (set.has(v)) return false;
    let value, done;
    while (({value, done} = iterator.next())) {
      if (done) break;
      if (Object.is(v, value)) return false;
      set.add(value);
    }
  }
  return true;
}

function intersection(values, ...others) {
  values = new InternSet(values);
  others = others.map(set$2);
  out: for (const value of values) {
    for (const other of others) {
      if (!other.has(value)) {
        values.delete(value);
        continue out;
      }
    }
  }
  return values;
}

function set$2(values) {
  return values instanceof InternSet ? values : new InternSet(values);
}

function superset(values, other) {
  const iterator = values[Symbol.iterator](), set = new Set();
  for (const o of other) {
    const io = intern(o);
    if (set.has(io)) continue;
    let value, done;
    while (({value, done} = iterator.next())) {
      if (done) return false;
      const ivalue = intern(value);
      set.add(ivalue);
      if (Object.is(io, ivalue)) break;
    }
  }
  return true;
}

function intern(value) {
  return value !== null && typeof value === "object" ? value.valueOf() : value;
}

function subset(values, other) {
  return superset(other, values);
}

function union(...others) {
  const set = new InternSet();
  for (const other of others) {
    for (const o of other) {
      set.add(o);
    }
  }
  return set;
}

function identity$8(x) {
  return x;
}

var top = 1,
    right = 2,
    bottom = 3,
    left = 4,
    epsilon$6 = 1e-6;

function translateX(x) {
  return "translate(" + x + ",0)";
}

function translateY(y) {
  return "translate(0," + y + ")";
}

function number$2(scale) {
  return d => +scale(d);
}

function center$1(scale, offset) {
  offset = Math.max(0, scale.bandwidth() - offset * 2) / 2;
  if (scale.round()) offset = Math.round(offset);
  return d => +scale(d) + offset;
}

function entering() {
  return !this.__axis;
}

function axis(orient, scale) {
  var tickArguments = [],
      tickValues = null,
      tickFormat = null,
      tickSizeInner = 6,
      tickSizeOuter = 6,
      tickPadding = 3,
      offset = typeof window !== "undefined" && window.devicePixelRatio > 1 ? 0 : 0.5,
      k = orient === top || orient === left ? -1 : 1,
      x = orient === left || orient === right ? "x" : "y",
      transform = orient === top || orient === bottom ? translateX : translateY;

  function axis(context) {
    var values = tickValues == null ? (scale.ticks ? scale.ticks.apply(scale, tickArguments) : scale.domain()) : tickValues,
        format = tickFormat == null ? (scale.tickFormat ? scale.tickFormat.apply(scale, tickArguments) : identity$8) : tickFormat,
        spacing = Math.max(tickSizeInner, 0) + tickPadding,
        range = scale.range(),
        range0 = +range[0] + offset,
        range1 = +range[range.length - 1] + offset,
        position = (scale.bandwidth ? center$1 : number$2)(scale.copy(), offset),
        selection = context.selection ? context.selection() : context,
        path = selection.selectAll(".domain").data([null]),
        tick = selection.selectAll(".tick").data(values, scale).order(),
        tickExit = tick.exit(),
        tickEnter = tick.enter().append("g").attr("class", "tick"),
        line = tick.select("line"),
        text = tick.select("text");

    path = path.merge(path.enter().insert("path", ".tick")
        .attr("class", "domain")
        .attr("stroke", "currentColor"));

    tick = tick.merge(tickEnter);

    line = line.merge(tickEnter.append("line")
        .attr("stroke", "currentColor")
        .attr(x + "2", k * tickSizeInner));

    text = text.merge(tickEnter.append("text")
        .attr("fill", "currentColor")
        .attr(x, k * spacing)
        .attr("dy", orient === top ? "0em" : orient === bottom ? "0.71em" : "0.32em"));

    if (context !== selection) {
      path = path.transition(context);
      tick = tick.transition(context);
      line = line.transition(context);
      text = text.transition(context);

      tickExit = tickExit.transition(context)
          .attr("opacity", epsilon$6)
          .attr("transform", function(d) { return isFinite(d = position(d)) ? transform(d + offset) : this.getAttribute("transform"); });

      tickEnter
          .attr("opacity", epsilon$6)
          .attr("transform", function(d) { var p = this.parentNode.__axis; return transform((p && isFinite(p = p(d)) ? p : position(d)) + offset); });
    }

    tickExit.remove();

    path
        .attr("d", orient === left || orient === right
            ? (tickSizeOuter ? "M" + k * tickSizeOuter + "," + range0 + "H" + offset + "V" + range1 + "H" + k * tickSizeOuter : "M" + offset + "," + range0 + "V" + range1)
            : (tickSizeOuter ? "M" + range0 + "," + k * tickSizeOuter + "V" + offset + "H" + range1 + "V" + k * tickSizeOuter : "M" + range0 + "," + offset + "H" + range1));

    tick
        .attr("opacity", 1)
        .attr("transform", function(d) { return transform(position(d) + offset); });

    line
        .attr(x + "2", k * tickSizeInner);

    text
        .attr(x, k * spacing)
        .text(format);

    selection.filter(entering)
        .attr("fill", "none")
        .attr("font-size", 10)
        .attr("font-family", "sans-serif")
        .attr("text-anchor", orient === right ? "start" : orient === left ? "end" : "middle");

    selection
        .each(function() { this.__axis = position; });
  }

  axis.scale = function(_) {
    return arguments.length ? (scale = _, axis) : scale;
  };

  axis.ticks = function() {
    return tickArguments = Array.from(arguments), axis;
  };

  axis.tickArguments = function(_) {
    return arguments.length ? (tickArguments = _ == null ? [] : Array.from(_), axis) : tickArguments.slice();
  };

  axis.tickValues = function(_) {
    return arguments.length ? (tickValues = _ == null ? null : Array.from(_), axis) : tickValues && tickValues.slice();
  };

  axis.tickFormat = function(_) {
    return arguments.length ? (tickFormat = _, axis) : tickFormat;
  };

  axis.tickSize = function(_) {
    return arguments.length ? (tickSizeInner = tickSizeOuter = +_, axis) : tickSizeInner;
  };

  axis.tickSizeInner = function(_) {
    return arguments.length ? (tickSizeInner = +_, axis) : tickSizeInner;
  };

  axis.tickSizeOuter = function(_) {
    return arguments.length ? (tickSizeOuter = +_, axis) : tickSizeOuter;
  };

  axis.tickPadding = function(_) {
    return arguments.length ? (tickPadding = +_, axis) : tickPadding;
  };

  axis.offset = function(_) {
    return arguments.length ? (offset = +_, axis) : offset;
  };

  return axis;
}

function axisTop(scale) {
  return axis(top, scale);
}

function axisRight(scale) {
  return axis(right, scale);
}

function axisBottom(scale) {
  return axis(bottom, scale);
}

function axisLeft(scale) {
  return axis(left, scale);
}

var noop$3 = {value: () => {}};

function dispatch() {
  for (var i = 0, n = arguments.length, _ = {}, t; i < n; ++i) {
    if (!(t = arguments[i] + "") || (t in _) || /[\s.]/.test(t)) throw new Error("illegal type: " + t);
    _[t] = [];
  }
  return new Dispatch(_);
}

function Dispatch(_) {
  this._ = _;
}

function parseTypenames$1(typenames, types) {
  return typenames.trim().split(/^|\s+/).map(function(t) {
    var name = "", i = t.indexOf(".");
    if (i >= 0) name = t.slice(i + 1), t = t.slice(0, i);
    if (t && !types.hasOwnProperty(t)) throw new Error("unknown type: " + t);
    return {type: t, name: name};
  });
}

Dispatch.prototype = dispatch.prototype = {
  constructor: Dispatch,
  on: function(typename, callback) {
    var _ = this._,
        T = parseTypenames$1(typename + "", _),
        t,
        i = -1,
        n = T.length;

    // If no callback was specified, return the callback of the given type and name.
    if (arguments.length < 2) {
      while (++i < n) if ((t = (typename = T[i]).type) && (t = get$1(_[t], typename.name))) return t;
      return;
    }

    // If a type was specified, set the callback for the given type and name.
    // Otherwise, if a null callback was specified, remove callbacks of the given name.
    if (callback != null && typeof callback !== "function") throw new Error("invalid callback: " + callback);
    while (++i < n) {
      if (t = (typename = T[i]).type) _[t] = set$1(_[t], typename.name, callback);
      else if (callback == null) for (t in _) _[t] = set$1(_[t], typename.name, null);
    }

    return this;
  },
  copy: function() {
    var copy = {}, _ = this._;
    for (var t in _) copy[t] = _[t].slice();
    return new Dispatch(copy);
  },
  call: function(type, that) {
    if ((n = arguments.length - 2) > 0) for (var args = new Array(n), i = 0, n, t; i < n; ++i) args[i] = arguments[i + 2];
    if (!this._.hasOwnProperty(type)) throw new Error("unknown type: " + type);
    for (t = this._[type], i = 0, n = t.length; i < n; ++i) t[i].value.apply(that, args);
  },
  apply: function(type, that, args) {
    if (!this._.hasOwnProperty(type)) throw new Error("unknown type: " + type);
    for (var t = this._[type], i = 0, n = t.length; i < n; ++i) t[i].value.apply(that, args);
  }
};

function get$1(type, name) {
  for (var i = 0, n = type.length, c; i < n; ++i) {
    if ((c = type[i]).name === name) {
      return c.value;
    }
  }
}

function set$1(type, name, callback) {
  for (var i = 0, n = type.length; i < n; ++i) {
    if (type[i].name === name) {
      type[i] = noop$3, type = type.slice(0, i).concat(type.slice(i + 1));
      break;
    }
  }
  if (callback != null) type.push({name: name, value: callback});
  return type;
}

var xhtml = "http://www.w3.org/1999/xhtml";

const namespaces = {
  svg: "http://www.w3.org/2000/svg",
  xhtml: xhtml,
  xlink: "http://www.w3.org/1999/xlink",
  xml: "http://www.w3.org/XML/1998/namespace",
  xmlns: "http://www.w3.org/2000/xmlns/"
};

function namespace(name) {
  var prefix = name += "", i = prefix.indexOf(":");
  if (i >= 0 && (prefix = name.slice(0, i)) !== "xmlns") name = name.slice(i + 1);
  return namespaces.hasOwnProperty(prefix) ? {space: namespaces[prefix], local: name} : name; // eslint-disable-line no-prototype-builtins
}

function creatorInherit(name) {
  return function() {
    var document = this.ownerDocument,
        uri = this.namespaceURI;
    return uri === xhtml && document.documentElement.namespaceURI === xhtml
        ? document.createElement(name)
        : document.createElementNS(uri, name);
  };
}

function creatorFixed(fullname) {
  return function() {
    return this.ownerDocument.createElementNS(fullname.space, fullname.local);
  };
}

function creator(name) {
  var fullname = namespace(name);
  return (fullname.local
      ? creatorFixed
      : creatorInherit)(fullname);
}

function none$2() {}

function selector(selector) {
  return selector == null ? none$2 : function() {
    return this.querySelector(selector);
  };
}

function selection_select(select) {
  if (typeof select !== "function") select = selector(select);

  for (var groups = this._groups, m = groups.length, subgroups = new Array(m), j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, subgroup = subgroups[j] = new Array(n), node, subnode, i = 0; i < n; ++i) {
      if ((node = group[i]) && (subnode = select.call(node, node.__data__, i, group))) {
        if ("__data__" in node) subnode.__data__ = node.__data__;
        subgroup[i] = subnode;
      }
    }
  }

  return new Selection$1(subgroups, this._parents);
}

// Given something array like (or null), returns something that is strictly an
// array. This is used to ensure that array-like objects passed to d3.selectAll
// or selection.selectAll are converted into proper arrays when creating a
// selection; we don’t ever want to create a selection backed by a live
// HTMLCollection or NodeList. However, note that selection.selectAll will use a
// static NodeList as a group, since it safely derived from querySelectorAll.
function array$4(x) {
  return x == null ? [] : Array.isArray(x) ? x : Array.from(x);
}

function empty$1() {
  return [];
}

function selectorAll(selector) {
  return selector == null ? empty$1 : function() {
    return this.querySelectorAll(selector);
  };
}

function arrayAll(select) {
  return function() {
    return array$4(select.apply(this, arguments));
  };
}

function selection_selectAll(select) {
  if (typeof select === "function") select = arrayAll(select);
  else select = selectorAll(select);

  for (var groups = this._groups, m = groups.length, subgroups = [], parents = [], j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, node, i = 0; i < n; ++i) {
      if (node = group[i]) {
        subgroups.push(select.call(node, node.__data__, i, group));
        parents.push(node);
      }
    }
  }

  return new Selection$1(subgroups, parents);
}

function matcher(selector) {
  return function() {
    return this.matches(selector);
  };
}

function childMatcher(selector) {
  return function(node) {
    return node.matches(selector);
  };
}

var find$1 = Array.prototype.find;

function childFind(match) {
  return function() {
    return find$1.call(this.children, match);
  };
}

function childFirst() {
  return this.firstElementChild;
}

function selection_selectChild(match) {
  return this.select(match == null ? childFirst
      : childFind(typeof match === "function" ? match : childMatcher(match)));
}

var filter = Array.prototype.filter;

function children() {
  return Array.from(this.children);
}

function childrenFilter(match) {
  return function() {
    return filter.call(this.children, match);
  };
}

function selection_selectChildren(match) {
  return this.selectAll(match == null ? children
      : childrenFilter(typeof match === "function" ? match : childMatcher(match)));
}

function selection_filter(match) {
  if (typeof match !== "function") match = matcher(match);

  for (var groups = this._groups, m = groups.length, subgroups = new Array(m), j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, subgroup = subgroups[j] = [], node, i = 0; i < n; ++i) {
      if ((node = group[i]) && match.call(node, node.__data__, i, group)) {
        subgroup.push(node);
      }
    }
  }

  return new Selection$1(subgroups, this._parents);
}

function sparse(update) {
  return new Array(update.length);
}

function selection_enter() {
  return new Selection$1(this._enter || this._groups.map(sparse), this._parents);
}

function EnterNode(parent, datum) {
  this.ownerDocument = parent.ownerDocument;
  this.namespaceURI = parent.namespaceURI;
  this._next = null;
  this._parent = parent;
  this.__data__ = datum;
}

EnterNode.prototype = {
  constructor: EnterNode,
  appendChild: function(child) { return this._parent.insertBefore(child, this._next); },
  insertBefore: function(child, next) { return this._parent.insertBefore(child, next); },
  querySelector: function(selector) { return this._parent.querySelector(selector); },
  querySelectorAll: function(selector) { return this._parent.querySelectorAll(selector); }
};

function constant$a(x) {
  return function() {
    return x;
  };
}

function bindIndex(parent, group, enter, update, exit, data) {
  var i = 0,
      node,
      groupLength = group.length,
      dataLength = data.length;

  // Put any non-null nodes that fit into update.
  // Put any null nodes into enter.
  // Put any remaining data into enter.
  for (; i < dataLength; ++i) {
    if (node = group[i]) {
      node.__data__ = data[i];
      update[i] = node;
    } else {
      enter[i] = new EnterNode(parent, data[i]);
    }
  }

  // Put any non-null nodes that don’t fit into exit.
  for (; i < groupLength; ++i) {
    if (node = group[i]) {
      exit[i] = node;
    }
  }
}

function bindKey(parent, group, enter, update, exit, data, key) {
  var i,
      node,
      nodeByKeyValue = new Map,
      groupLength = group.length,
      dataLength = data.length,
      keyValues = new Array(groupLength),
      keyValue;

  // Compute the key for each node.
  // If multiple nodes have the same key, the duplicates are added to exit.
  for (i = 0; i < groupLength; ++i) {
    if (node = group[i]) {
      keyValues[i] = keyValue = key.call(node, node.__data__, i, group) + "";
      if (nodeByKeyValue.has(keyValue)) {
        exit[i] = node;
      } else {
        nodeByKeyValue.set(keyValue, node);
      }
    }
  }

  // Compute the key for each datum.
  // If there a node associated with this key, join and add it to update.
  // If there is not (or the key is a duplicate), add it to enter.
  for (i = 0; i < dataLength; ++i) {
    keyValue = key.call(parent, data[i], i, data) + "";
    if (node = nodeByKeyValue.get(keyValue)) {
      update[i] = node;
      node.__data__ = data[i];
      nodeByKeyValue.delete(keyValue);
    } else {
      enter[i] = new EnterNode(parent, data[i]);
    }
  }

  // Add any remaining nodes that were not bound to data to exit.
  for (i = 0; i < groupLength; ++i) {
    if ((node = group[i]) && (nodeByKeyValue.get(keyValues[i]) === node)) {
      exit[i] = node;
    }
  }
}

function datum(node) {
  return node.__data__;
}

function selection_data(value, key) {
  if (!arguments.length) return Array.from(this, datum);

  var bind = key ? bindKey : bindIndex,
      parents = this._parents,
      groups = this._groups;

  if (typeof value !== "function") value = constant$a(value);

  for (var m = groups.length, update = new Array(m), enter = new Array(m), exit = new Array(m), j = 0; j < m; ++j) {
    var parent = parents[j],
        group = groups[j],
        groupLength = group.length,
        data = arraylike(value.call(parent, parent && parent.__data__, j, parents)),
        dataLength = data.length,
        enterGroup = enter[j] = new Array(dataLength),
        updateGroup = update[j] = new Array(dataLength),
        exitGroup = exit[j] = new Array(groupLength);

    bind(parent, group, enterGroup, updateGroup, exitGroup, data, key);

    // Now connect the enter nodes to their following update node, such that
    // appendChild can insert the materialized enter node before this node,
    // rather than at the end of the parent node.
    for (var i0 = 0, i1 = 0, previous, next; i0 < dataLength; ++i0) {
      if (previous = enterGroup[i0]) {
        if (i0 >= i1) i1 = i0 + 1;
        while (!(next = updateGroup[i1]) && ++i1 < dataLength);
        previous._next = next || null;
      }
    }
  }

  update = new Selection$1(update, parents);
  update._enter = enter;
  update._exit = exit;
  return update;
}

// Given some data, this returns an array-like view of it: an object that
// exposes a length property and allows numeric indexing. Note that unlike
// selectAll, this isn’t worried about “live” collections because the resulting
// array will only be used briefly while data is being bound. (It is possible to
// cause the data to change while iterating by using a key function, but please
// don’t; we’d rather avoid a gratuitous copy.)
function arraylike(data) {
  return typeof data === "object" && "length" in data
    ? data // Array, TypedArray, NodeList, array-like
    : Array.from(data); // Map, Set, iterable, string, or anything else
}

function selection_exit() {
  return new Selection$1(this._exit || this._groups.map(sparse), this._parents);
}

function selection_join(onenter, onupdate, onexit) {
  var enter = this.enter(), update = this, exit = this.exit();
  if (typeof onenter === "function") {
    enter = onenter(enter);
    if (enter) enter = enter.selection();
  } else {
    enter = enter.append(onenter + "");
  }
  if (onupdate != null) {
    update = onupdate(update);
    if (update) update = update.selection();
  }
  if (onexit == null) exit.remove(); else onexit(exit);
  return enter && update ? enter.merge(update).order() : update;
}

function selection_merge(context) {
  var selection = context.selection ? context.selection() : context;

  for (var groups0 = this._groups, groups1 = selection._groups, m0 = groups0.length, m1 = groups1.length, m = Math.min(m0, m1), merges = new Array(m0), j = 0; j < m; ++j) {
    for (var group0 = groups0[j], group1 = groups1[j], n = group0.length, merge = merges[j] = new Array(n), node, i = 0; i < n; ++i) {
      if (node = group0[i] || group1[i]) {
        merge[i] = node;
      }
    }
  }

  for (; j < m0; ++j) {
    merges[j] = groups0[j];
  }

  return new Selection$1(merges, this._parents);
}

function selection_order() {

  for (var groups = this._groups, j = -1, m = groups.length; ++j < m;) {
    for (var group = groups[j], i = group.length - 1, next = group[i], node; --i >= 0;) {
      if (node = group[i]) {
        if (next && node.compareDocumentPosition(next) ^ 4) next.parentNode.insertBefore(node, next);
        next = node;
      }
    }
  }

  return this;
}

function selection_sort(compare) {
  if (!compare) compare = ascending$2;

  function compareNode(a, b) {
    return a && b ? compare(a.__data__, b.__data__) : !a - !b;
  }

  for (var groups = this._groups, m = groups.length, sortgroups = new Array(m), j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, sortgroup = sortgroups[j] = new Array(n), node, i = 0; i < n; ++i) {
      if (node = group[i]) {
        sortgroup[i] = node;
      }
    }
    sortgroup.sort(compareNode);
  }

  return new Selection$1(sortgroups, this._parents).order();
}

function ascending$2(a, b) {
  return a < b ? -1 : a > b ? 1 : a >= b ? 0 : NaN;
}

function selection_call() {
  var callback = arguments[0];
  arguments[0] = this;
  callback.apply(null, arguments);
  return this;
}

function selection_nodes() {
  return Array.from(this);
}

function selection_node() {

  for (var groups = this._groups, j = 0, m = groups.length; j < m; ++j) {
    for (var group = groups[j], i = 0, n = group.length; i < n; ++i) {
      var node = group[i];
      if (node) return node;
    }
  }

  return null;
}

function selection_size() {
  let size = 0;
  for (const node of this) ++size; // eslint-disable-line no-unused-vars
  return size;
}

function selection_empty() {
  return !this.node();
}

function selection_each(callback) {

  for (var groups = this._groups, j = 0, m = groups.length; j < m; ++j) {
    for (var group = groups[j], i = 0, n = group.length, node; i < n; ++i) {
      if (node = group[i]) callback.call(node, node.__data__, i, group);
    }
  }

  return this;
}

function attrRemove$1(name) {
  return function() {
    this.removeAttribute(name);
  };
}

function attrRemoveNS$1(fullname) {
  return function() {
    this.removeAttributeNS(fullname.space, fullname.local);
  };
}

function attrConstant$1(name, value) {
  return function() {
    this.setAttribute(name, value);
  };
}

function attrConstantNS$1(fullname, value) {
  return function() {
    this.setAttributeNS(fullname.space, fullname.local, value);
  };
}

function attrFunction$1(name, value) {
  return function() {
    var v = value.apply(this, arguments);
    if (v == null) this.removeAttribute(name);
    else this.setAttribute(name, v);
  };
}

function attrFunctionNS$1(fullname, value) {
  return function() {
    var v = value.apply(this, arguments);
    if (v == null) this.removeAttributeNS(fullname.space, fullname.local);
    else this.setAttributeNS(fullname.space, fullname.local, v);
  };
}

function selection_attr(name, value) {
  var fullname = namespace(name);

  if (arguments.length < 2) {
    var node = this.node();
    return fullname.local
        ? node.getAttributeNS(fullname.space, fullname.local)
        : node.getAttribute(fullname);
  }

  return this.each((value == null
      ? (fullname.local ? attrRemoveNS$1 : attrRemove$1) : (typeof value === "function"
      ? (fullname.local ? attrFunctionNS$1 : attrFunction$1)
      : (fullname.local ? attrConstantNS$1 : attrConstant$1)))(fullname, value));
}

function defaultView(node) {
  return (node.ownerDocument && node.ownerDocument.defaultView) // node is a Node
      || (node.document && node) // node is a Window
      || node.defaultView; // node is a Document
}

function styleRemove$1(name) {
  return function() {
    this.style.removeProperty(name);
  };
}

function styleConstant$1(name, value, priority) {
  return function() {
    this.style.setProperty(name, value, priority);
  };
}

function styleFunction$1(name, value, priority) {
  return function() {
    var v = value.apply(this, arguments);
    if (v == null) this.style.removeProperty(name);
    else this.style.setProperty(name, v, priority);
  };
}

function selection_style(name, value, priority) {
  return arguments.length > 1
      ? this.each((value == null
            ? styleRemove$1 : typeof value === "function"
            ? styleFunction$1
            : styleConstant$1)(name, value, priority == null ? "" : priority))
      : styleValue(this.node(), name);
}

function styleValue(node, name) {
  return node.style.getPropertyValue(name)
      || defaultView(node).getComputedStyle(node, null).getPropertyValue(name);
}

function propertyRemove(name) {
  return function() {
    delete this[name];
  };
}

function propertyConstant(name, value) {
  return function() {
    this[name] = value;
  };
}

function propertyFunction(name, value) {
  return function() {
    var v = value.apply(this, arguments);
    if (v == null) delete this[name];
    else this[name] = v;
  };
}

function selection_property(name, value) {
  return arguments.length > 1
      ? this.each((value == null
          ? propertyRemove : typeof value === "function"
          ? propertyFunction
          : propertyConstant)(name, value))
      : this.node()[name];
}

function classArray(string) {
  return string.trim().split(/^|\s+/);
}

function classList(node) {
  return node.classList || new ClassList(node);
}

function ClassList(node) {
  this._node = node;
  this._names = classArray(node.getAttribute("class") || "");
}

ClassList.prototype = {
  add: function(name) {
    var i = this._names.indexOf(name);
    if (i < 0) {
      this._names.push(name);
      this._node.setAttribute("class", this._names.join(" "));
    }
  },
  remove: function(name) {
    var i = this._names.indexOf(name);
    if (i >= 0) {
      this._names.splice(i, 1);
      this._node.setAttribute("class", this._names.join(" "));
    }
  },
  contains: function(name) {
    return this._names.indexOf(name) >= 0;
  }
};

function classedAdd(node, names) {
  var list = classList(node), i = -1, n = names.length;
  while (++i < n) list.add(names[i]);
}

function classedRemove(node, names) {
  var list = classList(node), i = -1, n = names.length;
  while (++i < n) list.remove(names[i]);
}

function classedTrue(names) {
  return function() {
    classedAdd(this, names);
  };
}

function classedFalse(names) {
  return function() {
    classedRemove(this, names);
  };
}

function classedFunction(names, value) {
  return function() {
    (value.apply(this, arguments) ? classedAdd : classedRemove)(this, names);
  };
}

function selection_classed(name, value) {
  var names = classArray(name + "");

  if (arguments.length < 2) {
    var list = classList(this.node()), i = -1, n = names.length;
    while (++i < n) if (!list.contains(names[i])) return false;
    return true;
  }

  return this.each((typeof value === "function"
      ? classedFunction : value
      ? classedTrue
      : classedFalse)(names, value));
}

function textRemove() {
  this.textContent = "";
}

function textConstant$1(value) {
  return function() {
    this.textContent = value;
  };
}

function textFunction$1(value) {
  return function() {
    var v = value.apply(this, arguments);
    this.textContent = v == null ? "" : v;
  };
}

function selection_text(value) {
  return arguments.length
      ? this.each(value == null
          ? textRemove : (typeof value === "function"
          ? textFunction$1
          : textConstant$1)(value))
      : this.node().textContent;
}

function htmlRemove() {
  this.innerHTML = "";
}

function htmlConstant(value) {
  return function() {
    this.innerHTML = value;
  };
}

function htmlFunction(value) {
  return function() {
    var v = value.apply(this, arguments);
    this.innerHTML = v == null ? "" : v;
  };
}

function selection_html(value) {
  return arguments.length
      ? this.each(value == null
          ? htmlRemove : (typeof value === "function"
          ? htmlFunction
          : htmlConstant)(value))
      : this.node().innerHTML;
}

function raise() {
  if (this.nextSibling) this.parentNode.appendChild(this);
}

function selection_raise() {
  return this.each(raise);
}

function lower() {
  if (this.previousSibling) this.parentNode.insertBefore(this, this.parentNode.firstChild);
}

function selection_lower() {
  return this.each(lower);
}

function selection_append(name) {
  var create = typeof name === "function" ? name : creator(name);
  return this.select(function() {
    return this.appendChild(create.apply(this, arguments));
  });
}

function constantNull() {
  return null;
}

function selection_insert(name, before) {
  var create = typeof name === "function" ? name : creator(name),
      select = before == null ? constantNull : typeof before === "function" ? before : selector(before);
  return this.select(function() {
    return this.insertBefore(create.apply(this, arguments), select.apply(this, arguments) || null);
  });
}

function remove() {
  var parent = this.parentNode;
  if (parent) parent.removeChild(this);
}

function selection_remove() {
  return this.each(remove);
}

function selection_cloneShallow() {
  var clone = this.cloneNode(false), parent = this.parentNode;
  return parent ? parent.insertBefore(clone, this.nextSibling) : clone;
}

function selection_cloneDeep() {
  var clone = this.cloneNode(true), parent = this.parentNode;
  return parent ? parent.insertBefore(clone, this.nextSibling) : clone;
}

function selection_clone(deep) {
  return this.select(deep ? selection_cloneDeep : selection_cloneShallow);
}

function selection_datum(value) {
  return arguments.length
      ? this.property("__data__", value)
      : this.node().__data__;
}

function contextListener(listener) {
  return function(event) {
    listener.call(this, event, this.__data__);
  };
}

function parseTypenames(typenames) {
  return typenames.trim().split(/^|\s+/).map(function(t) {
    var name = "", i = t.indexOf(".");
    if (i >= 0) name = t.slice(i + 1), t = t.slice(0, i);
    return {type: t, name: name};
  });
}

function onRemove(typename) {
  return function() {
    var on = this.__on;
    if (!on) return;
    for (var j = 0, i = -1, m = on.length, o; j < m; ++j) {
      if (o = on[j], (!typename.type || o.type === typename.type) && o.name === typename.name) {
        this.removeEventListener(o.type, o.listener, o.options);
      } else {
        on[++i] = o;
      }
    }
    if (++i) on.length = i;
    else delete this.__on;
  };
}

function onAdd(typename, value, options) {
  return function() {
    var on = this.__on, o, listener = contextListener(value);
    if (on) for (var j = 0, m = on.length; j < m; ++j) {
      if ((o = on[j]).type === typename.type && o.name === typename.name) {
        this.removeEventListener(o.type, o.listener, o.options);
        this.addEventListener(o.type, o.listener = listener, o.options = options);
        o.value = value;
        return;
      }
    }
    this.addEventListener(typename.type, listener, options);
    o = {type: typename.type, name: typename.name, value: value, listener: listener, options: options};
    if (!on) this.__on = [o];
    else on.push(o);
  };
}

function selection_on(typename, value, options) {
  var typenames = parseTypenames(typename + ""), i, n = typenames.length, t;

  if (arguments.length < 2) {
    var on = this.node().__on;
    if (on) for (var j = 0, m = on.length, o; j < m; ++j) {
      for (i = 0, o = on[j]; i < n; ++i) {
        if ((t = typenames[i]).type === o.type && t.name === o.name) {
          return o.value;
        }
      }
    }
    return;
  }

  on = value ? onAdd : onRemove;
  for (i = 0; i < n; ++i) this.each(on(typenames[i], value, options));
  return this;
}

function dispatchEvent(node, type, params) {
  var window = defaultView(node),
      event = window.CustomEvent;

  if (typeof event === "function") {
    event = new event(type, params);
  } else {
    event = window.document.createEvent("Event");
    if (params) event.initEvent(type, params.bubbles, params.cancelable), event.detail = params.detail;
    else event.initEvent(type, false, false);
  }

  node.dispatchEvent(event);
}

function dispatchConstant(type, params) {
  return function() {
    return dispatchEvent(this, type, params);
  };
}

function dispatchFunction(type, params) {
  return function() {
    return dispatchEvent(this, type, params.apply(this, arguments));
  };
}

function selection_dispatch(type, params) {
  return this.each((typeof params === "function"
      ? dispatchFunction
      : dispatchConstant)(type, params));
}

function* selection_iterator() {
  for (var groups = this._groups, j = 0, m = groups.length; j < m; ++j) {
    for (var group = groups[j], i = 0, n = group.length, node; i < n; ++i) {
      if (node = group[i]) yield node;
    }
  }
}

var root$1 = [null];

function Selection$1(groups, parents) {
  this._groups = groups;
  this._parents = parents;
}

function selection() {
  return new Selection$1([[document.documentElement]], root$1);
}

function selection_selection() {
  return this;
}

Selection$1.prototype = selection.prototype = {
  constructor: Selection$1,
  select: selection_select,
  selectAll: selection_selectAll,
  selectChild: selection_selectChild,
  selectChildren: selection_selectChildren,
  filter: selection_filter,
  data: selection_data,
  enter: selection_enter,
  exit: selection_exit,
  join: selection_join,
  merge: selection_merge,
  selection: selection_selection,
  order: selection_order,
  sort: selection_sort,
  call: selection_call,
  nodes: selection_nodes,
  node: selection_node,
  size: selection_size,
  empty: selection_empty,
  each: selection_each,
  attr: selection_attr,
  style: selection_style,
  property: selection_property,
  classed: selection_classed,
  text: selection_text,
  html: selection_html,
  raise: selection_raise,
  lower: selection_lower,
  append: selection_append,
  insert: selection_insert,
  remove: selection_remove,
  clone: selection_clone,
  datum: selection_datum,
  on: selection_on,
  dispatch: selection_dispatch,
  [Symbol.iterator]: selection_iterator
};

function select(selector) {
  return typeof selector === "string"
      ? new Selection$1([[document.querySelector(selector)]], [document.documentElement])
      : new Selection$1([[selector]], root$1);
}

function create$1(name) {
  return select(creator(name).call(document.documentElement));
}

var nextId = 0;

function local$1() {
  return new Local;
}

function Local() {
  this._ = "@" + (++nextId).toString(36);
}

Local.prototype = local$1.prototype = {
  constructor: Local,
  get: function(node) {
    var id = this._;
    while (!(id in node)) if (!(node = node.parentNode)) return;
    return node[id];
  },
  set: function(node, value) {
    return node[this._] = value;
  },
  remove: function(node) {
    return this._ in node && delete node[this._];
  },
  toString: function() {
    return this._;
  }
};

function sourceEvent(event) {
  let sourceEvent;
  while (sourceEvent = event.sourceEvent) event = sourceEvent;
  return event;
}

function pointer(event, node) {
  event = sourceEvent(event);
  if (node === undefined) node = event.currentTarget;
  if (node) {
    var svg = node.ownerSVGElement || node;
    if (svg.createSVGPoint) {
      var point = svg.createSVGPoint();
      point.x = event.clientX, point.y = event.clientY;
      point = point.matrixTransform(node.getScreenCTM().inverse());
      return [point.x, point.y];
    }
    if (node.getBoundingClientRect) {
      var rect = node.getBoundingClientRect();
      return [event.clientX - rect.left - node.clientLeft, event.clientY - rect.top - node.clientTop];
    }
  }
  return [event.pageX, event.pageY];
}

function pointers(events, node) {
  if (events.target) { // i.e., instanceof Event, not TouchList or iterable
    events = sourceEvent(events);
    if (node === undefined) node = events.currentTarget;
    events = events.touches || [events];
  }
  return Array.from(events, event => pointer(event, node));
}

function selectAll(selector) {
  return typeof selector === "string"
      ? new Selection$1([document.querySelectorAll(selector)], [document.documentElement])
      : new Selection$1([array$4(selector)], root$1);
}

// These are typically used in conjunction with noevent to ensure that we can
// preventDefault on the event.
const nonpassive = {passive: false};
const nonpassivecapture = {capture: true, passive: false};

function nopropagation$2(event) {
  event.stopImmediatePropagation();
}

function noevent$2(event) {
  event.preventDefault();
  event.stopImmediatePropagation();
}

function dragDisable(view) {
  var root = view.document.documentElement,
      selection = select(view).on("dragstart.drag", noevent$2, nonpassivecapture);
  if ("onselectstart" in root) {
    selection.on("selectstart.drag", noevent$2, nonpassivecapture);
  } else {
    root.__noselect = root.style.MozUserSelect;
    root.style.MozUserSelect = "none";
  }
}

function yesdrag(view, noclick) {
  var root = view.document.documentElement,
      selection = select(view).on("dragstart.drag", null);
  if (noclick) {
    selection.on("click.drag", noevent$2, nonpassivecapture);
    setTimeout(function() { selection.on("click.drag", null); }, 0);
  }
  if ("onselectstart" in root) {
    selection.on("selectstart.drag", null);
  } else {
    root.style.MozUserSelect = root.__noselect;
    delete root.__noselect;
  }
}

const constant$9 = x => () => x;

function DragEvent(type, {
  sourceEvent,
  subject,
  target,
  identifier,
  active,
  x, y, dx, dy,
  dispatch
}) {
  Object.defineProperties(this, {
    type: {value: type, enumerable: true, configurable: true},
    sourceEvent: {value: sourceEvent, enumerable: true, configurable: true},
    subject: {value: subject, enumerable: true, configurable: true},
    target: {value: target, enumerable: true, configurable: true},
    identifier: {value: identifier, enumerable: true, configurable: true},
    active: {value: active, enumerable: true, configurable: true},
    x: {value: x, enumerable: true, configurable: true},
    y: {value: y, enumerable: true, configurable: true},
    dx: {value: dx, enumerable: true, configurable: true},
    dy: {value: dy, enumerable: true, configurable: true},
    _: {value: dispatch}
  });
}

DragEvent.prototype.on = function() {
  var value = this._.on.apply(this._, arguments);
  return value === this._ ? this : value;
};

// Ignore right-click, since that should open the context menu.
function defaultFilter$2(event) {
  return !event.ctrlKey && !event.button;
}

function defaultContainer() {
  return this.parentNode;
}

function defaultSubject(event, d) {
  return d == null ? {x: event.x, y: event.y} : d;
}

function defaultTouchable$2() {
  return navigator.maxTouchPoints || ("ontouchstart" in this);
}

function drag() {
  var filter = defaultFilter$2,
      container = defaultContainer,
      subject = defaultSubject,
      touchable = defaultTouchable$2,
      gestures = {},
      listeners = dispatch("start", "drag", "end"),
      active = 0,
      mousedownx,
      mousedowny,
      mousemoving,
      touchending,
      clickDistance2 = 0;

  function drag(selection) {
    selection
        .on("mousedown.drag", mousedowned)
      .filter(touchable)
        .on("touchstart.drag", touchstarted)
        .on("touchmove.drag", touchmoved, nonpassive)
        .on("touchend.drag touchcancel.drag", touchended)
        .style("touch-action", "none")
        .style("-webkit-tap-highlight-color", "rgba(0,0,0,0)");
  }

  function mousedowned(event, d) {
    if (touchending || !filter.call(this, event, d)) return;
    var gesture = beforestart(this, container.call(this, event, d), event, d, "mouse");
    if (!gesture) return;
    select(event.view)
      .on("mousemove.drag", mousemoved, nonpassivecapture)
      .on("mouseup.drag", mouseupped, nonpassivecapture);
    dragDisable(event.view);
    nopropagation$2(event);
    mousemoving = false;
    mousedownx = event.clientX;
    mousedowny = event.clientY;
    gesture("start", event);
  }

  function mousemoved(event) {
    noevent$2(event);
    if (!mousemoving) {
      var dx = event.clientX - mousedownx, dy = event.clientY - mousedowny;
      mousemoving = dx * dx + dy * dy > clickDistance2;
    }
    gestures.mouse("drag", event);
  }

  function mouseupped(event) {
    select(event.view).on("mousemove.drag mouseup.drag", null);
    yesdrag(event.view, mousemoving);
    noevent$2(event);
    gestures.mouse("end", event);
  }

  function touchstarted(event, d) {
    if (!filter.call(this, event, d)) return;
    var touches = event.changedTouches,
        c = container.call(this, event, d),
        n = touches.length, i, gesture;

    for (i = 0; i < n; ++i) {
      if (gesture = beforestart(this, c, event, d, touches[i].identifier, touches[i])) {
        nopropagation$2(event);
        gesture("start", event, touches[i]);
      }
    }
  }

  function touchmoved(event) {
    var touches = event.changedTouches,
        n = touches.length, i, gesture;

    for (i = 0; i < n; ++i) {
      if (gesture = gestures[touches[i].identifier]) {
        noevent$2(event);
        gesture("drag", event, touches[i]);
      }
    }
  }

  function touchended(event) {
    var touches = event.changedTouches,
        n = touches.length, i, gesture;

    if (touchending) clearTimeout(touchending);
    touchending = setTimeout(function() { touchending = null; }, 500); // Ghost clicks are delayed!
    for (i = 0; i < n; ++i) {
      if (gesture = gestures[touches[i].identifier]) {
        nopropagation$2(event);
        gesture("end", event, touches[i]);
      }
    }
  }

  function beforestart(that, container, event, d, identifier, touch) {
    var dispatch = listeners.copy(),
        p = pointer(touch || event, container), dx, dy,
        s;

    if ((s = subject.call(that, new DragEvent("beforestart", {
        sourceEvent: event,
        target: drag,
        identifier,
        active,
        x: p[0],
        y: p[1],
        dx: 0,
        dy: 0,
        dispatch
      }), d)) == null) return;

    dx = s.x - p[0] || 0;
    dy = s.y - p[1] || 0;

    return function gesture(type, event, touch) {
      var p0 = p, n;
      switch (type) {
        case "start": gestures[identifier] = gesture, n = active++; break;
        case "end": delete gestures[identifier], --active; // falls through
        case "drag": p = pointer(touch || event, container), n = active; break;
      }
      dispatch.call(
        type,
        that,
        new DragEvent(type, {
          sourceEvent: event,
          subject: s,
          target: drag,
          identifier,
          active: n,
          x: p[0] + dx,
          y: p[1] + dy,
          dx: p[0] - p0[0],
          dy: p[1] - p0[1],
          dispatch
        }),
        d
      );
    };
  }

  drag.filter = function(_) {
    return arguments.length ? (filter = typeof _ === "function" ? _ : constant$9(!!_), drag) : filter;
  };

  drag.container = function(_) {
    return arguments.length ? (container = typeof _ === "function" ? _ : constant$9(_), drag) : container;
  };

  drag.subject = function(_) {
    return arguments.length ? (subject = typeof _ === "function" ? _ : constant$9(_), drag) : subject;
  };

  drag.touchable = function(_) {
    return arguments.length ? (touchable = typeof _ === "function" ? _ : constant$9(!!_), drag) : touchable;
  };

  drag.on = function() {
    var value = listeners.on.apply(listeners, arguments);
    return value === listeners ? drag : value;
  };

  drag.clickDistance = function(_) {
    return arguments.length ? (clickDistance2 = (_ = +_) * _, drag) : Math.sqrt(clickDistance2);
  };

  return drag;
}

function define(constructor, factory, prototype) {
  constructor.prototype = factory.prototype = prototype;
  prototype.constructor = constructor;
}

function extend(parent, definition) {
  var prototype = Object.create(parent.prototype);
  for (var key in definition) prototype[key] = definition[key];
  return prototype;
}

function Color() {}

var darker = 0.7;
var brighter = 1 / darker;

var reI = "\\s*([+-]?\\d+)\\s*",
    reN = "\\s*([+-]?(?:\\d*\\.)?\\d+(?:[eE][+-]?\\d+)?)\\s*",
    reP = "\\s*([+-]?(?:\\d*\\.)?\\d+(?:[eE][+-]?\\d+)?)%\\s*",
    reHex = /^#([0-9a-f]{3,8})$/,
    reRgbInteger = new RegExp(`^rgb\\(${reI},${reI},${reI}\\)$`),
    reRgbPercent = new RegExp(`^rgb\\(${reP},${reP},${reP}\\)$`),
    reRgbaInteger = new RegExp(`^rgba\\(${reI},${reI},${reI},${reN}\\)$`),
    reRgbaPercent = new RegExp(`^rgba\\(${reP},${reP},${reP},${reN}\\)$`),
    reHslPercent = new RegExp(`^hsl\\(${reN},${reP},${reP}\\)$`),
    reHslaPercent = new RegExp(`^hsla\\(${reN},${reP},${reP},${reN}\\)$`);

var named = {
  aliceblue: 0xf0f8ff,
  antiquewhite: 0xfaebd7,
  aqua: 0x00ffff,
  aquamarine: 0x7fffd4,
  azure: 0xf0ffff,
  beige: 0xf5f5dc,
  bisque: 0xffe4c4,
  black: 0x000000,
  blanchedalmond: 0xffebcd,
  blue: 0x0000ff,
  blueviolet: 0x8a2be2,
  brown: 0xa52a2a,
  burlywood: 0xdeb887,
  cadetblue: 0x5f9ea0,
  chartreuse: 0x7fff00,
  chocolate: 0xd2691e,
  coral: 0xff7f50,
  cornflowerblue: 0x6495ed,
  cornsilk: 0xfff8dc,
  crimson: 0xdc143c,
  cyan: 0x00ffff,
  darkblue: 0x00008b,
  darkcyan: 0x008b8b,
  darkgoldenrod: 0xb8860b,
  darkgray: 0xa9a9a9,
  darkgreen: 0x006400,
  darkgrey: 0xa9a9a9,
  darkkhaki: 0xbdb76b,
  darkmagenta: 0x8b008b,
  darkolivegreen: 0x556b2f,
  darkorange: 0xff8c00,
  darkorchid: 0x9932cc,
  darkred: 0x8b0000,
  darksalmon: 0xe9967a,
  darkseagreen: 0x8fbc8f,
  darkslateblue: 0x483d8b,
  darkslategray: 0x2f4f4f,
  darkslategrey: 0x2f4f4f,
  darkturquoise: 0x00ced1,
  darkviolet: 0x9400d3,
  deeppink: 0xff1493,
  deepskyblue: 0x00bfff,
  dimgray: 0x696969,
  dimgrey: 0x696969,
  dodgerblue: 0x1e90ff,
  firebrick: 0xb22222,
  floralwhite: 0xfffaf0,
  forestgreen: 0x228b22,
  fuchsia: 0xff00ff,
  gainsboro: 0xdcdcdc,
  ghostwhite: 0xf8f8ff,
  gold: 0xffd700,
  goldenrod: 0xdaa520,
  gray: 0x808080,
  green: 0x008000,
  greenyellow: 0xadff2f,
  grey: 0x808080,
  honeydew: 0xf0fff0,
  hotpink: 0xff69b4,
  indianred: 0xcd5c5c,
  indigo: 0x4b0082,
  ivory: 0xfffff0,
  khaki: 0xf0e68c,
  lavender: 0xe6e6fa,
  lavenderblush: 0xfff0f5,
  lawngreen: 0x7cfc00,
  lemonchiffon: 0xfffacd,
  lightblue: 0xadd8e6,
  lightcoral: 0xf08080,
  lightcyan: 0xe0ffff,
  lightgoldenrodyellow: 0xfafad2,
  lightgray: 0xd3d3d3,
  lightgreen: 0x90ee90,
  lightgrey: 0xd3d3d3,
  lightpink: 0xffb6c1,
  lightsalmon: 0xffa07a,
  lightseagreen: 0x20b2aa,
  lightskyblue: 0x87cefa,
  lightslategray: 0x778899,
  lightslategrey: 0x778899,
  lightsteelblue: 0xb0c4de,
  lightyellow: 0xffffe0,
  lime: 0x00ff00,
  limegreen: 0x32cd32,
  linen: 0xfaf0e6,
  magenta: 0xff00ff,
  maroon: 0x800000,
  mediumaquamarine: 0x66cdaa,
  mediumblue: 0x0000cd,
  mediumorchid: 0xba55d3,
  mediumpurple: 0x9370db,
  mediumseagreen: 0x3cb371,
  mediumslateblue: 0x7b68ee,
  mediumspringgreen: 0x00fa9a,
  mediumturquoise: 0x48d1cc,
  mediumvioletred: 0xc71585,
  midnightblue: 0x191970,
  mintcream: 0xf5fffa,
  mistyrose: 0xffe4e1,
  moccasin: 0xffe4b5,
  navajowhite: 0xffdead,
  navy: 0x000080,
  oldlace: 0xfdf5e6,
  olive: 0x808000,
  olivedrab: 0x6b8e23,
  orange: 0xffa500,
  orangered: 0xff4500,
  orchid: 0xda70d6,
  palegoldenrod: 0xeee8aa,
  palegreen: 0x98fb98,
  paleturquoise: 0xafeeee,
  palevioletred: 0xdb7093,
  papayawhip: 0xffefd5,
  peachpuff: 0xffdab9,
  peru: 0xcd853f,
  pink: 0xffc0cb,
  plum: 0xdda0dd,
  powderblue: 0xb0e0e6,
  purple: 0x800080,
  rebeccapurple: 0x663399,
  red: 0xff0000,
  rosybrown: 0xbc8f8f,
  royalblue: 0x4169e1,
  saddlebrown: 0x8b4513,
  salmon: 0xfa8072,
  sandybrown: 0xf4a460,
  seagreen: 0x2e8b57,
  seashell: 0xfff5ee,
  sienna: 0xa0522d,
  silver: 0xc0c0c0,
  skyblue: 0x87ceeb,
  slateblue: 0x6a5acd,
  slategray: 0x708090,
  slategrey: 0x708090,
  snow: 0xfffafa,
  springgreen: 0x00ff7f,
  steelblue: 0x4682b4,
  tan: 0xd2b48c,
  teal: 0x008080,
  thistle: 0xd8bfd8,
  tomato: 0xff6347,
  turquoise: 0x40e0d0,
  violet: 0xee82ee,
  wheat: 0xf5deb3,
  white: 0xffffff,
  whitesmoke: 0xf5f5f5,
  yellow: 0xffff00,
  yellowgreen: 0x9acd32
};

define(Color, color, {
  copy(channels) {
    return Object.assign(new this.constructor, this, channels);
  },
  displayable() {
    return this.rgb().displayable();
  },
  hex: color_formatHex, // Deprecated! Use color.formatHex.
  formatHex: color_formatHex,
  formatHex8: color_formatHex8,
  formatHsl: color_formatHsl,
  formatRgb: color_formatRgb,
  toString: color_formatRgb
});

function color_formatHex() {
  return this.rgb().formatHex();
}

function color_formatHex8() {
  return this.rgb().formatHex8();
}

function color_formatHsl() {
  return hslConvert(this).formatHsl();
}

function color_formatRgb() {
  return this.rgb().formatRgb();
}

function color(format) {
  var m, l;
  format = (format + "").trim().toLowerCase();
  return (m = reHex.exec(format)) ? (l = m[1].length, m = parseInt(m[1], 16), l === 6 ? rgbn(m) // #ff0000
      : l === 3 ? new Rgb((m >> 8 & 0xf) | (m >> 4 & 0xf0), (m >> 4 & 0xf) | (m & 0xf0), ((m & 0xf) << 4) | (m & 0xf), 1) // #f00
      : l === 8 ? rgba(m >> 24 & 0xff, m >> 16 & 0xff, m >> 8 & 0xff, (m & 0xff) / 0xff) // #ff000000
      : l === 4 ? rgba((m >> 12 & 0xf) | (m >> 8 & 0xf0), (m >> 8 & 0xf) | (m >> 4 & 0xf0), (m >> 4 & 0xf) | (m & 0xf0), (((m & 0xf) << 4) | (m & 0xf)) / 0xff) // #f000
      : null) // invalid hex
      : (m = reRgbInteger.exec(format)) ? new Rgb(m[1], m[2], m[3], 1) // rgb(255, 0, 0)
      : (m = reRgbPercent.exec(format)) ? new Rgb(m[1] * 255 / 100, m[2] * 255 / 100, m[3] * 255 / 100, 1) // rgb(100%, 0%, 0%)
      : (m = reRgbaInteger.exec(format)) ? rgba(m[1], m[2], m[3], m[4]) // rgba(255, 0, 0, 1)
      : (m = reRgbaPercent.exec(format)) ? rgba(m[1] * 255 / 100, m[2] * 255 / 100, m[3] * 255 / 100, m[4]) // rgb(100%, 0%, 0%, 1)
      : (m = reHslPercent.exec(format)) ? hsla(m[1], m[2] / 100, m[3] / 100, 1) // hsl(120, 50%, 50%)
      : (m = reHslaPercent.exec(format)) ? hsla(m[1], m[2] / 100, m[3] / 100, m[4]) // hsla(120, 50%, 50%, 1)
      : named.hasOwnProperty(format) ? rgbn(named[format]) // eslint-disable-line no-prototype-builtins
      : format === "transparent" ? new Rgb(NaN, NaN, NaN, 0)
      : null;
}

function rgbn(n) {
  return new Rgb(n >> 16 & 0xff, n >> 8 & 0xff, n & 0xff, 1);
}

function rgba(r, g, b, a) {
  if (a <= 0) r = g = b = NaN;
  return new Rgb(r, g, b, a);
}

function rgbConvert(o) {
  if (!(o instanceof Color)) o = color(o);
  if (!o) return new Rgb;
  o = o.rgb();
  return new Rgb(o.r, o.g, o.b, o.opacity);
}

function rgb(r, g, b, opacity) {
  return arguments.length === 1 ? rgbConvert(r) : new Rgb(r, g, b, opacity == null ? 1 : opacity);
}

function Rgb(r, g, b, opacity) {
  this.r = +r;
  this.g = +g;
  this.b = +b;
  this.opacity = +opacity;
}

define(Rgb, rgb, extend(Color, {
  brighter(k) {
    k = k == null ? brighter : Math.pow(brighter, k);
    return new Rgb(this.r * k, this.g * k, this.b * k, this.opacity);
  },
  darker(k) {
    k = k == null ? darker : Math.pow(darker, k);
    return new Rgb(this.r * k, this.g * k, this.b * k, this.opacity);
  },
  rgb() {
    return this;
  },
  clamp() {
    return new Rgb(clampi(this.r), clampi(this.g), clampi(this.b), clampa(this.opacity));
  },
  displayable() {
    return (-0.5 <= this.r && this.r < 255.5)
        && (-0.5 <= this.g && this.g < 255.5)
        && (-0.5 <= this.b && this.b < 255.5)
        && (0 <= this.opacity && this.opacity <= 1);
  },
  hex: rgb_formatHex, // Deprecated! Use color.formatHex.
  formatHex: rgb_formatHex,
  formatHex8: rgb_formatHex8,
  formatRgb: rgb_formatRgb,
  toString: rgb_formatRgb
}));

function rgb_formatHex() {
  return `#${hex(this.r)}${hex(this.g)}${hex(this.b)}`;
}

function rgb_formatHex8() {
  return `#${hex(this.r)}${hex(this.g)}${hex(this.b)}${hex((isNaN(this.opacity) ? 1 : this.opacity) * 255)}`;
}

function rgb_formatRgb() {
  const a = clampa(this.opacity);
  return `${a === 1 ? "rgb(" : "rgba("}${clampi(this.r)}, ${clampi(this.g)}, ${clampi(this.b)}${a === 1 ? ")" : `, ${a})`}`;
}

function clampa(opacity) {
  return isNaN(opacity) ? 1 : Math.max(0, Math.min(1, opacity));
}

function clampi(value) {
  return Math.max(0, Math.min(255, Math.round(value) || 0));
}

function hex(value) {
  value = clampi(value);
  return (value < 16 ? "0" : "") + value.toString(16);
}

function hsla(h, s, l, a) {
  if (a <= 0) h = s = l = NaN;
  else if (l <= 0 || l >= 1) h = s = NaN;
  else if (s <= 0) h = NaN;
  return new Hsl(h, s, l, a);
}

function hslConvert(o) {
  if (o instanceof Hsl) return new Hsl(o.h, o.s, o.l, o.opacity);
  if (!(o instanceof Color)) o = color(o);
  if (!o) return new Hsl;
  if (o instanceof Hsl) return o;
  o = o.rgb();
  var r = o.r / 255,
      g = o.g / 255,
      b = o.b / 255,
      min = Math.min(r, g, b),
      max = Math.max(r, g, b),
      h = NaN,
      s = max - min,
      l = (max + min) / 2;
  if (s) {
    if (r === max) h = (g - b) / s + (g < b) * 6;
    else if (g === max) h = (b - r) / s + 2;
    else h = (r - g) / s + 4;
    s /= l < 0.5 ? max + min : 2 - max - min;
    h *= 60;
  } else {
    s = l > 0 && l < 1 ? 0 : h;
  }
  return new Hsl(h, s, l, o.opacity);
}

function hsl$1(h, s, l, opacity) {
  return arguments.length === 1 ? hslConvert(h) : new Hsl(h, s, l, opacity == null ? 1 : opacity);
}

function Hsl(h, s, l, opacity) {
  this.h = +h;
  this.s = +s;
  this.l = +l;
  this.opacity = +opacity;
}

define(Hsl, hsl$1, extend(Color, {
  brighter(k) {
    k = k == null ? brighter : Math.pow(brighter, k);
    return new Hsl(this.h, this.s, this.l * k, this.opacity);
  },
  darker(k) {
    k = k == null ? darker : Math.pow(darker, k);
    return new Hsl(this.h, this.s, this.l * k, this.opacity);
  },
  rgb() {
    var h = this.h % 360 + (this.h < 0) * 360,
        s = isNaN(h) || isNaN(this.s) ? 0 : this.s,
        l = this.l,
        m2 = l + (l < 0.5 ? l : 1 - l) * s,
        m1 = 2 * l - m2;
    return new Rgb(
      hsl2rgb(h >= 240 ? h - 240 : h + 120, m1, m2),
      hsl2rgb(h, m1, m2),
      hsl2rgb(h < 120 ? h + 240 : h - 120, m1, m2),
      this.opacity
    );
  },
  clamp() {
    return new Hsl(clamph(this.h), clampt(this.s), clampt(this.l), clampa(this.opacity));
  },
  displayable() {
    return (0 <= this.s && this.s <= 1 || isNaN(this.s))
        && (0 <= this.l && this.l <= 1)
        && (0 <= this.opacity && this.opacity <= 1);
  },
  formatHsl() {
    const a = clampa(this.opacity);
    return `${a === 1 ? "hsl(" : "hsla("}${clamph(this.h)}, ${clampt(this.s) * 100}%, ${clampt(this.l) * 100}%${a === 1 ? ")" : `, ${a})`}`;
  }
}));

function clamph(value) {
  value = (value || 0) % 360;
  return value < 0 ? value + 360 : value;
}

function clampt(value) {
  return Math.max(0, Math.min(1, value || 0));
}

/* From FvD 13.37, CSS Color Module Level 3 */
function hsl2rgb(h, m1, m2) {
  return (h < 60 ? m1 + (m2 - m1) * h / 60
      : h < 180 ? m2
      : h < 240 ? m1 + (m2 - m1) * (240 - h) / 60
      : m1) * 255;
}

const radians$1 = Math.PI / 180;
const degrees$2 = 180 / Math.PI;

// https://observablehq.com/@mbostock/lab-and-rgb
const K = 18,
    Xn = 0.96422,
    Yn = 1,
    Zn = 0.82521,
    t0$1 = 4 / 29,
    t1$1 = 6 / 29,
    t2 = 3 * t1$1 * t1$1,
    t3 = t1$1 * t1$1 * t1$1;

function labConvert(o) {
  if (o instanceof Lab) return new Lab(o.l, o.a, o.b, o.opacity);
  if (o instanceof Hcl) return hcl2lab(o);
  if (!(o instanceof Rgb)) o = rgbConvert(o);
  var r = rgb2lrgb(o.r),
      g = rgb2lrgb(o.g),
      b = rgb2lrgb(o.b),
      y = xyz2lab((0.2225045 * r + 0.7168786 * g + 0.0606169 * b) / Yn), x, z;
  if (r === g && g === b) x = z = y; else {
    x = xyz2lab((0.4360747 * r + 0.3850649 * g + 0.1430804 * b) / Xn);
    z = xyz2lab((0.0139322 * r + 0.0971045 * g + 0.7141733 * b) / Zn);
  }
  return new Lab(116 * y - 16, 500 * (x - y), 200 * (y - z), o.opacity);
}

function gray(l, opacity) {
  return new Lab(l, 0, 0, opacity == null ? 1 : opacity);
}

function lab$1(l, a, b, opacity) {
  return arguments.length === 1 ? labConvert(l) : new Lab(l, a, b, opacity == null ? 1 : opacity);
}

function Lab(l, a, b, opacity) {
  this.l = +l;
  this.a = +a;
  this.b = +b;
  this.opacity = +opacity;
}

define(Lab, lab$1, extend(Color, {
  brighter(k) {
    return new Lab(this.l + K * (k == null ? 1 : k), this.a, this.b, this.opacity);
  },
  darker(k) {
    return new Lab(this.l - K * (k == null ? 1 : k), this.a, this.b, this.opacity);
  },
  rgb() {
    var y = (this.l + 16) / 116,
        x = isNaN(this.a) ? y : y + this.a / 500,
        z = isNaN(this.b) ? y : y - this.b / 200;
    x = Xn * lab2xyz(x);
    y = Yn * lab2xyz(y);
    z = Zn * lab2xyz(z);
    return new Rgb(
      lrgb2rgb( 3.1338561 * x - 1.6168667 * y - 0.4906146 * z),
      lrgb2rgb(-0.9787684 * x + 1.9161415 * y + 0.0334540 * z),
      lrgb2rgb( 0.0719453 * x - 0.2289914 * y + 1.4052427 * z),
      this.opacity
    );
  }
}));

function xyz2lab(t) {
  return t > t3 ? Math.pow(t, 1 / 3) : t / t2 + t0$1;
}

function lab2xyz(t) {
  return t > t1$1 ? t * t * t : t2 * (t - t0$1);
}

function lrgb2rgb(x) {
  return 255 * (x <= 0.0031308 ? 12.92 * x : 1.055 * Math.pow(x, 1 / 2.4) - 0.055);
}

function rgb2lrgb(x) {
  return (x /= 255) <= 0.04045 ? x / 12.92 : Math.pow((x + 0.055) / 1.055, 2.4);
}

function hclConvert(o) {
  if (o instanceof Hcl) return new Hcl(o.h, o.c, o.l, o.opacity);
  if (!(o instanceof Lab)) o = labConvert(o);
  if (o.a === 0 && o.b === 0) return new Hcl(NaN, 0 < o.l && o.l < 100 ? 0 : NaN, o.l, o.opacity);
  var h = Math.atan2(o.b, o.a) * degrees$2;
  return new Hcl(h < 0 ? h + 360 : h, Math.sqrt(o.a * o.a + o.b * o.b), o.l, o.opacity);
}

function lch(l, c, h, opacity) {
  return arguments.length === 1 ? hclConvert(l) : new Hcl(h, c, l, opacity == null ? 1 : opacity);
}

function hcl$1(h, c, l, opacity) {
  return arguments.length === 1 ? hclConvert(h) : new Hcl(h, c, l, opacity == null ? 1 : opacity);
}

function Hcl(h, c, l, opacity) {
  this.h = +h;
  this.c = +c;
  this.l = +l;
  this.opacity = +opacity;
}

function hcl2lab(o) {
  if (isNaN(o.h)) return new Lab(o.l, 0, 0, o.opacity);
  var h = o.h * radians$1;
  return new Lab(o.l, Math.cos(h) * o.c, Math.sin(h) * o.c, o.opacity);
}

define(Hcl, hcl$1, extend(Color, {
  brighter(k) {
    return new Hcl(this.h, this.c, this.l + K * (k == null ? 1 : k), this.opacity);
  },
  darker(k) {
    return new Hcl(this.h, this.c, this.l - K * (k == null ? 1 : k), this.opacity);
  },
  rgb() {
    return hcl2lab(this).rgb();
  }
}));

var A = -0.14861,
    B$1 = +1.78277,
    C = -0.29227,
    D$1 = -0.90649,
    E = +1.97294,
    ED = E * D$1,
    EB = E * B$1,
    BC_DA = B$1 * C - D$1 * A;

function cubehelixConvert(o) {
  if (o instanceof Cubehelix) return new Cubehelix(o.h, o.s, o.l, o.opacity);
  if (!(o instanceof Rgb)) o = rgbConvert(o);
  var r = o.r / 255,
      g = o.g / 255,
      b = o.b / 255,
      l = (BC_DA * b + ED * r - EB * g) / (BC_DA + ED - EB),
      bl = b - l,
      k = (E * (g - l) - C * bl) / D$1,
      s = Math.sqrt(k * k + bl * bl) / (E * l * (1 - l)), // NaN if l=0 or l=1
      h = s ? Math.atan2(k, bl) * degrees$2 - 120 : NaN;
  return new Cubehelix(h < 0 ? h + 360 : h, s, l, o.opacity);
}

function cubehelix$2(h, s, l, opacity) {
  return arguments.length === 1 ? cubehelixConvert(h) : new Cubehelix(h, s, l, opacity == null ? 1 : opacity);
}

function Cubehelix(h, s, l, opacity) {
  this.h = +h;
  this.s = +s;
  this.l = +l;
  this.opacity = +opacity;
}

define(Cubehelix, cubehelix$2, extend(Color, {
  brighter(k) {
    k = k == null ? brighter : Math.pow(brighter, k);
    return new Cubehelix(this.h, this.s, this.l * k, this.opacity);
  },
  darker(k) {
    k = k == null ? darker : Math.pow(darker, k);
    return new Cubehelix(this.h, this.s, this.l * k, this.opacity);
  },
  rgb() {
    var h = isNaN(this.h) ? 0 : (this.h + 120) * radians$1,
        l = +this.l,
        a = isNaN(this.s) ? 0 : this.s * l * (1 - l),
        cosh = Math.cos(h),
        sinh = Math.sin(h);
    return new Rgb(
      255 * (l + a * (A * cosh + B$1 * sinh)),
      255 * (l + a * (C * cosh + D$1 * sinh)),
      255 * (l + a * (E * cosh)),
      this.opacity
    );
  }
}));

function basis$1(t1, v0, v1, v2, v3) {
  var t2 = t1 * t1, t3 = t2 * t1;
  return ((1 - 3 * t1 + 3 * t2 - t3) * v0
      + (4 - 6 * t2 + 3 * t3) * v1
      + (1 + 3 * t1 + 3 * t2 - 3 * t3) * v2
      + t3 * v3) / 6;
}

function basis$2(values) {
  var n = values.length - 1;
  return function(t) {
    var i = t <= 0 ? (t = 0) : t >= 1 ? (t = 1, n - 1) : Math.floor(t * n),
        v1 = values[i],
        v2 = values[i + 1],
        v0 = i > 0 ? values[i - 1] : 2 * v1 - v2,
        v3 = i < n - 1 ? values[i + 2] : 2 * v2 - v1;
    return basis$1((t - i / n) * n, v0, v1, v2, v3);
  };
}

function basisClosed$1(values) {
  var n = values.length;
  return function(t) {
    var i = Math.floor(((t %= 1) < 0 ? ++t : t) * n),
        v0 = values[(i + n - 1) % n],
        v1 = values[i % n],
        v2 = values[(i + 1) % n],
        v3 = values[(i + 2) % n];
    return basis$1((t - i / n) * n, v0, v1, v2, v3);
  };
}

const constant$8 = x => () => x;

function linear$2(a, d) {
  return function(t) {
    return a + t * d;
  };
}

function exponential$1(a, b, y) {
  return a = Math.pow(a, y), b = Math.pow(b, y) - a, y = 1 / y, function(t) {
    return Math.pow(a + t * b, y);
  };
}

function hue$1(a, b) {
  var d = b - a;
  return d ? linear$2(a, d > 180 || d < -180 ? d - 360 * Math.round(d / 360) : d) : constant$8(isNaN(a) ? b : a);
}

function gamma$1(y) {
  return (y = +y) === 1 ? nogamma : function(a, b) {
    return b - a ? exponential$1(a, b, y) : constant$8(isNaN(a) ? b : a);
  };
}

function nogamma(a, b) {
  var d = b - a;
  return d ? linear$2(a, d) : constant$8(isNaN(a) ? b : a);
}

const interpolateRgb = (function rgbGamma(y) {
  var color = gamma$1(y);

  function rgb$1(start, end) {
    var r = color((start = rgb(start)).r, (end = rgb(end)).r),
        g = color(start.g, end.g),
        b = color(start.b, end.b),
        opacity = nogamma(start.opacity, end.opacity);
    return function(t) {
      start.r = r(t);
      start.g = g(t);
      start.b = b(t);
      start.opacity = opacity(t);
      return start + "";
    };
  }

  rgb$1.gamma = rgbGamma;

  return rgb$1;
})(1);

function rgbSpline(spline) {
  return function(colors) {
    var n = colors.length,
        r = new Array(n),
        g = new Array(n),
        b = new Array(n),
        i, color;
    for (i = 0; i < n; ++i) {
      color = rgb(colors[i]);
      r[i] = color.r || 0;
      g[i] = color.g || 0;
      b[i] = color.b || 0;
    }
    r = spline(r);
    g = spline(g);
    b = spline(b);
    color.opacity = 1;
    return function(t) {
      color.r = r(t);
      color.g = g(t);
      color.b = b(t);
      return color + "";
    };
  };
}

var rgbBasis = rgbSpline(basis$2);
var rgbBasisClosed = rgbSpline(basisClosed$1);

function numberArray(a, b) {
  if (!b) b = [];
  var n = a ? Math.min(b.length, a.length) : 0,
      c = b.slice(),
      i;
  return function(t) {
    for (i = 0; i < n; ++i) c[i] = a[i] * (1 - t) + b[i] * t;
    return c;
  };
}

function isNumberArray(x) {
  return ArrayBuffer.isView(x) && !(x instanceof DataView);
}

function array$3(a, b) {
  return (isNumberArray(b) ? numberArray : genericArray)(a, b);
}

function genericArray(a, b) {
  var nb = b ? b.length : 0,
      na = a ? Math.min(nb, a.length) : 0,
      x = new Array(na),
      c = new Array(nb),
      i;

  for (i = 0; i < na; ++i) x[i] = interpolate$2(a[i], b[i]);
  for (; i < nb; ++i) c[i] = b[i];

  return function(t) {
    for (i = 0; i < na; ++i) c[i] = x[i](t);
    return c;
  };
}

function date$1(a, b) {
  var d = new Date;
  return a = +a, b = +b, function(t) {
    return d.setTime(a * (1 - t) + b * t), d;
  };
}

function interpolateNumber(a, b) {
  return a = +a, b = +b, function(t) {
    return a * (1 - t) + b * t;
  };
}

function object$1(a, b) {
  var i = {},
      c = {},
      k;

  if (a === null || typeof a !== "object") a = {};
  if (b === null || typeof b !== "object") b = {};

  for (k in b) {
    if (k in a) {
      i[k] = interpolate$2(a[k], b[k]);
    } else {
      c[k] = b[k];
    }
  }

  return function(t) {
    for (k in i) c[k] = i[k](t);
    return c;
  };
}

var reA = /[-+]?(?:\d+\.?\d*|\.?\d+)(?:[eE][-+]?\d+)?/g,
    reB = new RegExp(reA.source, "g");

function zero(b) {
  return function() {
    return b;
  };
}

function one(b) {
  return function(t) {
    return b(t) + "";
  };
}

function interpolateString(a, b) {
  var bi = reA.lastIndex = reB.lastIndex = 0, // scan index for next number in b
      am, // current match in a
      bm, // current match in b
      bs, // string preceding current number in b, if any
      i = -1, // index in s
      s = [], // string constants and placeholders
      q = []; // number interpolators

  // Coerce inputs to strings.
  a = a + "", b = b + "";

  // Interpolate pairs of numbers in a & b.
  while ((am = reA.exec(a))
      && (bm = reB.exec(b))) {
    if ((bs = bm.index) > bi) { // a string precedes the next number in b
      bs = b.slice(bi, bs);
      if (s[i]) s[i] += bs; // coalesce with previous string
      else s[++i] = bs;
    }
    if ((am = am[0]) === (bm = bm[0])) { // numbers in a & b match
      if (s[i]) s[i] += bm; // coalesce with previous string
      else s[++i] = bm;
    } else { // interpolate non-matching numbers
      s[++i] = null;
      q.push({i: i, x: interpolateNumber(am, bm)});
    }
    bi = reB.lastIndex;
  }

  // Add remains of b.
  if (bi < b.length) {
    bs = b.slice(bi);
    if (s[i]) s[i] += bs; // coalesce with previous string
    else s[++i] = bs;
  }

  // Special optimization for only a single match.
  // Otherwise, interpolate each of the numbers and rejoin the string.
  return s.length < 2 ? (q[0]
      ? one(q[0].x)
      : zero(b))
      : (b = q.length, function(t) {
          for (var i = 0, o; i < b; ++i) s[(o = q[i]).i] = o.x(t);
          return s.join("");
        });
}

function interpolate$2(a, b) {
  var t = typeof b, c;
  return b == null || t === "boolean" ? constant$8(b)
      : (t === "number" ? interpolateNumber
      : t === "string" ? ((c = color(b)) ? (b = c, interpolateRgb) : interpolateString)
      : b instanceof color ? interpolateRgb
      : b instanceof Date ? date$1
      : isNumberArray(b) ? numberArray
      : Array.isArray(b) ? genericArray
      : typeof b.valueOf !== "function" && typeof b.toString !== "function" || isNaN(b) ? object$1
      : interpolateNumber)(a, b);
}

function discrete(range) {
  var n = range.length;
  return function(t) {
    return range[Math.max(0, Math.min(n - 1, Math.floor(t * n)))];
  };
}

function hue(a, b) {
  var i = hue$1(+a, +b);
  return function(t) {
    var x = i(t);
    return x - 360 * Math.floor(x / 360);
  };
}

function interpolateRound(a, b) {
  return a = +a, b = +b, function(t) {
    return Math.round(a * (1 - t) + b * t);
  };
}

var degrees$1 = 180 / Math.PI;

var identity$7 = {
  translateX: 0,
  translateY: 0,
  rotate: 0,
  skewX: 0,
  scaleX: 1,
  scaleY: 1
};

function decompose(a, b, c, d, e, f) {
  var scaleX, scaleY, skewX;
  if (scaleX = Math.sqrt(a * a + b * b)) a /= scaleX, b /= scaleX;
  if (skewX = a * c + b * d) c -= a * skewX, d -= b * skewX;
  if (scaleY = Math.sqrt(c * c + d * d)) c /= scaleY, d /= scaleY, skewX /= scaleY;
  if (a * d < b * c) a = -a, b = -b, skewX = -skewX, scaleX = -scaleX;
  return {
    translateX: e,
    translateY: f,
    rotate: Math.atan2(b, a) * degrees$1,
    skewX: Math.atan(skewX) * degrees$1,
    scaleX: scaleX,
    scaleY: scaleY
  };
}

var svgNode;

/* eslint-disable no-undef */
function parseCss(value) {
  const m = new (typeof DOMMatrix === "function" ? DOMMatrix : WebKitCSSMatrix)(value + "");
  return m.isIdentity ? identity$7 : decompose(m.a, m.b, m.c, m.d, m.e, m.f);
}

function parseSvg(value) {
  if (value == null) return identity$7;
  if (!svgNode) svgNode = document.createElementNS("http://www.w3.org/2000/svg", "g");
  svgNode.setAttribute("transform", value);
  if (!(value = svgNode.transform.baseVal.consolidate())) return identity$7;
  value = value.matrix;
  return decompose(value.a, value.b, value.c, value.d, value.e, value.f);
}

function interpolateTransform(parse, pxComma, pxParen, degParen) {

  function pop(s) {
    return s.length ? s.pop() + " " : "";
  }

  function translate(xa, ya, xb, yb, s, q) {
    if (xa !== xb || ya !== yb) {
      var i = s.push("translate(", null, pxComma, null, pxParen);
      q.push({i: i - 4, x: interpolateNumber(xa, xb)}, {i: i - 2, x: interpolateNumber(ya, yb)});
    } else if (xb || yb) {
      s.push("translate(" + xb + pxComma + yb + pxParen);
    }
  }

  function rotate(a, b, s, q) {
    if (a !== b) {
      if (a - b > 180) b += 360; else if (b - a > 180) a += 360; // shortest path
      q.push({i: s.push(pop(s) + "rotate(", null, degParen) - 2, x: interpolateNumber(a, b)});
    } else if (b) {
      s.push(pop(s) + "rotate(" + b + degParen);
    }
  }

  function skewX(a, b, s, q) {
    if (a !== b) {
      q.push({i: s.push(pop(s) + "skewX(", null, degParen) - 2, x: interpolateNumber(a, b)});
    } else if (b) {
      s.push(pop(s) + "skewX(" + b + degParen);
    }
  }

  function scale(xa, ya, xb, yb, s, q) {
    if (xa !== xb || ya !== yb) {
      var i = s.push(pop(s) + "scale(", null, ",", null, ")");
      q.push({i: i - 4, x: interpolateNumber(xa, xb)}, {i: i - 2, x: interpolateNumber(ya, yb)});
    } else if (xb !== 1 || yb !== 1) {
      s.push(pop(s) + "scale(" + xb + "," + yb + ")");
    }
  }

  return function(a, b) {
    var s = [], // string constants and placeholders
        q = []; // number interpolators
    a = parse(a), b = parse(b);
    translate(a.translateX, a.translateY, b.translateX, b.translateY, s, q);
    rotate(a.rotate, b.rotate, s, q);
    skewX(a.skewX, b.skewX, s, q);
    scale(a.scaleX, a.scaleY, b.scaleX, b.scaleY, s, q);
    a = b = null; // gc
    return function(t) {
      var i = -1, n = q.length, o;
      while (++i < n) s[(o = q[i]).i] = o.x(t);
      return s.join("");
    };
  };
}

var interpolateTransformCss = interpolateTransform(parseCss, "px, ", "px)", "deg)");
var interpolateTransformSvg = interpolateTransform(parseSvg, ", ", ")", ")");

var epsilon2$1 = 1e-12;

function cosh(x) {
  return ((x = Math.exp(x)) + 1 / x) / 2;
}

function sinh(x) {
  return ((x = Math.exp(x)) - 1 / x) / 2;
}

function tanh(x) {
  return ((x = Math.exp(2 * x)) - 1) / (x + 1);
}

const interpolateZoom = (function zoomRho(rho, rho2, rho4) {

  // p0 = [ux0, uy0, w0]
  // p1 = [ux1, uy1, w1]
  function zoom(p0, p1) {
    var ux0 = p0[0], uy0 = p0[1], w0 = p0[2],
        ux1 = p1[0], uy1 = p1[1], w1 = p1[2],
        dx = ux1 - ux0,
        dy = uy1 - uy0,
        d2 = dx * dx + dy * dy,
        i,
        S;

    // Special case for u0 ≅ u1.
    if (d2 < epsilon2$1) {
      S = Math.log(w1 / w0) / rho;
      i = function(t) {
        return [
          ux0 + t * dx,
          uy0 + t * dy,
          w0 * Math.exp(rho * t * S)
        ];
      };
    }

    // General case.
    else {
      var d1 = Math.sqrt(d2),
          b0 = (w1 * w1 - w0 * w0 + rho4 * d2) / (2 * w0 * rho2 * d1),
          b1 = (w1 * w1 - w0 * w0 - rho4 * d2) / (2 * w1 * rho2 * d1),
          r0 = Math.log(Math.sqrt(b0 * b0 + 1) - b0),
          r1 = Math.log(Math.sqrt(b1 * b1 + 1) - b1);
      S = (r1 - r0) / rho;
      i = function(t) {
        var s = t * S,
            coshr0 = cosh(r0),
            u = w0 / (rho2 * d1) * (coshr0 * tanh(rho * s + r0) - sinh(r0));
        return [
          ux0 + u * dx,
          uy0 + u * dy,
          w0 * coshr0 / cosh(rho * s + r0)
        ];
      };
    }

    i.duration = S * 1000 * rho / Math.SQRT2;

    return i;
  }

  zoom.rho = function(_) {
    var _1 = Math.max(1e-3, +_), _2 = _1 * _1, _4 = _2 * _2;
    return zoomRho(_1, _2, _4);
  };

  return zoom;
})(Math.SQRT2, 2, 4);

function hsl(hue) {
  return function(start, end) {
    var h = hue((start = hsl$1(start)).h, (end = hsl$1(end)).h),
        s = nogamma(start.s, end.s),
        l = nogamma(start.l, end.l),
        opacity = nogamma(start.opacity, end.opacity);
    return function(t) {
      start.h = h(t);
      start.s = s(t);
      start.l = l(t);
      start.opacity = opacity(t);
      return start + "";
    };
  }
}

const hsl_default = hsl(hue$1);
var hslLong = hsl(nogamma);

function lab(start, end) {
  var l = nogamma((start = lab$1(start)).l, (end = lab$1(end)).l),
      a = nogamma(start.a, end.a),
      b = nogamma(start.b, end.b),
      opacity = nogamma(start.opacity, end.opacity);
  return function(t) {
    start.l = l(t);
    start.a = a(t);
    start.b = b(t);
    start.opacity = opacity(t);
    return start + "";
  };
}

function hcl(hue) {
  return function(start, end) {
    var h = hue((start = hcl$1(start)).h, (end = hcl$1(end)).h),
        c = nogamma(start.c, end.c),
        l = nogamma(start.l, end.l),
        opacity = nogamma(start.opacity, end.opacity);
    return function(t) {
      start.h = h(t);
      start.c = c(t);
      start.l = l(t);
      start.opacity = opacity(t);
      return start + "";
    };
  }
}

const hcl_default = hcl(hue$1);
var hclLong = hcl(nogamma);

function cubehelix$1(hue) {
  return (function cubehelixGamma(y) {
    y = +y;

    function cubehelix(start, end) {
      var h = hue((start = cubehelix$2(start)).h, (end = cubehelix$2(end)).h),
          s = nogamma(start.s, end.s),
          l = nogamma(start.l, end.l),
          opacity = nogamma(start.opacity, end.opacity);
      return function(t) {
        start.h = h(t);
        start.s = s(t);
        start.l = l(Math.pow(t, y));
        start.opacity = opacity(t);
        return start + "";
      };
    }

    cubehelix.gamma = cubehelixGamma;

    return cubehelix;
  })(1);
}

const cubehelix_default = cubehelix$1(hue$1);
var cubehelixLong = cubehelix$1(nogamma);

function piecewise(interpolate, values) {
  if (values === undefined) values = interpolate, interpolate = interpolate$2;
  var i = 0, n = values.length - 1, v = values[0], I = new Array(n < 0 ? 0 : n);
  while (i < n) I[i] = interpolate(v, v = values[++i]);
  return function(t) {
    var i = Math.max(0, Math.min(n - 1, Math.floor(t *= n)));
    return I[i](t - i);
  };
}

function quantize$1(interpolator, n) {
  var samples = new Array(n);
  for (var i = 0; i < n; ++i) samples[i] = interpolator(i / (n - 1));
  return samples;
}

var frame = 0, // is an animation frame pending?
    timeout$1 = 0, // is a timeout pending?
    interval$1 = 0, // are any timers active?
    pokeDelay = 1000, // how frequently we check for clock skew
    taskHead,
    taskTail,
    clockLast = 0,
    clockNow = 0,
    clockSkew = 0,
    clock = typeof performance === "object" && performance.now ? performance : Date,
    setFrame = typeof window === "object" && window.requestAnimationFrame ? window.requestAnimationFrame.bind(window) : function(f) { setTimeout(f, 17); };

function now() {
  return clockNow || (setFrame(clearNow), clockNow = clock.now() + clockSkew);
}

function clearNow() {
  clockNow = 0;
}

function Timer() {
  this._call =
  this._time =
  this._next = null;
}

Timer.prototype = timer.prototype = {
  constructor: Timer,
  restart: function(callback, delay, time) {
    if (typeof callback !== "function") throw new TypeError("callback is not a function");
    time = (time == null ? now() : +time) + (delay == null ? 0 : +delay);
    if (!this._next && taskTail !== this) {
      if (taskTail) taskTail._next = this;
      else taskHead = this;
      taskTail = this;
    }
    this._call = callback;
    this._time = time;
    sleep$1();
  },
  stop: function() {
    if (this._call) {
      this._call = null;
      this._time = Infinity;
      sleep$1();
    }
  }
};

function timer(callback, delay, time) {
  var t = new Timer;
  t.restart(callback, delay, time);
  return t;
}

function timerFlush() {
  now(); // Get the current time, if not already set.
  ++frame; // Pretend we’ve set an alarm, if we haven’t already.
  var t = taskHead, e;
  while (t) {
    if ((e = clockNow - t._time) >= 0) t._call.call(undefined, e);
    t = t._next;
  }
  --frame;
}

function wake() {
  clockNow = (clockLast = clock.now()) + clockSkew;
  frame = timeout$1 = 0;
  try {
    timerFlush();
  } finally {
    frame = 0;
    nap();
    clockNow = 0;
  }
}

function poke() {
  var now = clock.now(), delay = now - clockLast;
  if (delay > pokeDelay) clockSkew -= delay, clockLast = now;
}

function nap() {
  var t0, t1 = taskHead, t2, time = Infinity;
  while (t1) {
    if (t1._call) {
      if (time > t1._time) time = t1._time;
      t0 = t1, t1 = t1._next;
    } else {
      t2 = t1._next, t1._next = null;
      t1 = t0 ? t0._next = t2 : taskHead = t2;
    }
  }
  taskTail = t0;
  sleep$1(time);
}

function sleep$1(time) {
  if (frame) return; // Soonest alarm already set, or will be.
  if (timeout$1) timeout$1 = clearTimeout(timeout$1);
  var delay = time - clockNow; // Strictly less than if we recomputed clockNow.
  if (delay > 24) {
    if (time < Infinity) timeout$1 = setTimeout(wake, time - clock.now() - clockSkew);
    if (interval$1) interval$1 = clearInterval(interval$1);
  } else {
    if (!interval$1) clockLast = clock.now(), interval$1 = setInterval(poke, pokeDelay);
    frame = 1, setFrame(wake);
  }
}

function timeout(callback, delay, time) {
  var t = new Timer;
  delay = delay == null ? 0 : +delay;
  t.restart(elapsed => {
    t.stop();
    callback(elapsed + delay);
  }, delay, time);
  return t;
}

function interval(callback, delay, time) {
  var t = new Timer, total = delay;
  if (delay == null) return t.restart(callback, delay, time), t;
  t._restart = t.restart;
  t.restart = function(callback, delay, time) {
    delay = +delay, time = time == null ? now() : +time;
    t._restart(function tick(elapsed) {
      elapsed += total;
      t._restart(tick, total += delay, time);
      callback(elapsed);
    }, delay, time);
  };
  t.restart(callback, delay, time);
  return t;
}

var emptyOn = dispatch("start", "end", "cancel", "interrupt");
var emptyTween = [];

var CREATED = 0;
var SCHEDULED = 1;
var STARTING = 2;
var STARTED = 3;
var RUNNING = 4;
var ENDING = 5;
var ENDED = 6;

function schedule(node, name, id, index, group, timing) {
  var schedules = node.__transition;
  if (!schedules) node.__transition = {};
  else if (id in schedules) return;
  create(node, id, {
    name: name,
    index: index, // For context during callback.
    group: group, // For context during callback.
    on: emptyOn,
    tween: emptyTween,
    time: timing.time,
    delay: timing.delay,
    duration: timing.duration,
    ease: timing.ease,
    timer: null,
    state: CREATED
  });
}

function init(node, id) {
  var schedule = get(node, id);
  if (schedule.state > CREATED) throw new Error("too late; already scheduled");
  return schedule;
}

function set(node, id) {
  var schedule = get(node, id);
  if (schedule.state > STARTED) throw new Error("too late; already running");
  return schedule;
}

function get(node, id) {
  var schedule = node.__transition;
  if (!schedule || !(schedule = schedule[id])) throw new Error("transition not found");
  return schedule;
}

function create(node, id, self) {
  var schedules = node.__transition,
      tween;

  // Initialize the self timer when the transition is created.
  // Note the actual delay is not known until the first callback!
  schedules[id] = self;
  self.timer = timer(schedule, 0, self.time);

  function schedule(elapsed) {
    self.state = SCHEDULED;
    self.timer.restart(start, self.delay, self.time);

    // If the elapsed delay is less than our first sleep, start immediately.
    if (self.delay <= elapsed) start(elapsed - self.delay);
  }

  function start(elapsed) {
    var i, j, n, o;

    // If the state is not SCHEDULED, then we previously errored on start.
    if (self.state !== SCHEDULED) return stop();

    for (i in schedules) {
      o = schedules[i];
      if (o.name !== self.name) continue;

      // While this element already has a starting transition during this frame,
      // defer starting an interrupting transition until that transition has a
      // chance to tick (and possibly end); see d3/d3-transition#54!
      if (o.state === STARTED) return timeout(start);

      // Interrupt the active transition, if any.
      if (o.state === RUNNING) {
        o.state = ENDED;
        o.timer.stop();
        o.on.call("interrupt", node, node.__data__, o.index, o.group);
        delete schedules[i];
      }

      // Cancel any pre-empted transitions.
      else if (+i < id) {
        o.state = ENDED;
        o.timer.stop();
        o.on.call("cancel", node, node.__data__, o.index, o.group);
        delete schedules[i];
      }
    }

    // Defer the first tick to end of the current frame; see d3/d3#1576.
    // Note the transition may be canceled after start and before the first tick!
    // Note this must be scheduled before the start event; see d3/d3-transition#16!
    // Assuming this is successful, subsequent callbacks go straight to tick.
    timeout(function() {
      if (self.state === STARTED) {
        self.state = RUNNING;
        self.timer.restart(tick, self.delay, self.time);
        tick(elapsed);
      }
    });

    // Dispatch the start event.
    // Note this must be done before the tween are initialized.
    self.state = STARTING;
    self.on.call("start", node, node.__data__, self.index, self.group);
    if (self.state !== STARTING) return; // interrupted
    self.state = STARTED;

    // Initialize the tween, deleting null tween.
    tween = new Array(n = self.tween.length);
    for (i = 0, j = -1; i < n; ++i) {
      if (o = self.tween[i].value.call(node, node.__data__, self.index, self.group)) {
        tween[++j] = o;
      }
    }
    tween.length = j + 1;
  }

  function tick(elapsed) {
    var t = elapsed < self.duration ? self.ease.call(null, elapsed / self.duration) : (self.timer.restart(stop), self.state = ENDING, 1),
        i = -1,
        n = tween.length;

    while (++i < n) {
      tween[i].call(node, t);
    }

    // Dispatch the end event.
    if (self.state === ENDING) {
      self.on.call("end", node, node.__data__, self.index, self.group);
      stop();
    }
  }

  function stop() {
    self.state = ENDED;
    self.timer.stop();
    delete schedules[id];
    for (var i in schedules) return; // eslint-disable-line no-unused-vars
    delete node.__transition;
  }
}

function interrupt(node, name) {
  var schedules = node.__transition,
      schedule,
      active,
      empty = true,
      i;

  if (!schedules) return;

  name = name == null ? null : name + "";

  for (i in schedules) {
    if ((schedule = schedules[i]).name !== name) { empty = false; continue; }
    active = schedule.state > STARTING && schedule.state < ENDING;
    schedule.state = ENDED;
    schedule.timer.stop();
    schedule.on.call(active ? "interrupt" : "cancel", node, node.__data__, schedule.index, schedule.group);
    delete schedules[i];
  }

  if (empty) delete node.__transition;
}

function selection_interrupt(name) {
  return this.each(function() {
    interrupt(this, name);
  });
}

function tweenRemove(id, name) {
  var tween0, tween1;
  return function() {
    var schedule = set(this, id),
        tween = schedule.tween;

    // If this node shared tween with the previous node,
    // just assign the updated shared tween and we’re done!
    // Otherwise, copy-on-write.
    if (tween !== tween0) {
      tween1 = tween0 = tween;
      for (var i = 0, n = tween1.length; i < n; ++i) {
        if (tween1[i].name === name) {
          tween1 = tween1.slice();
          tween1.splice(i, 1);
          break;
        }
      }
    }

    schedule.tween = tween1;
  };
}

function tweenFunction(id, name, value) {
  var tween0, tween1;
  if (typeof value !== "function") throw new Error;
  return function() {
    var schedule = set(this, id),
        tween = schedule.tween;

    // If this node shared tween with the previous node,
    // just assign the updated shared tween and we’re done!
    // Otherwise, copy-on-write.
    if (tween !== tween0) {
      tween1 = (tween0 = tween).slice();
      for (var t = {name: name, value: value}, i = 0, n = tween1.length; i < n; ++i) {
        if (tween1[i].name === name) {
          tween1[i] = t;
          break;
        }
      }
      if (i === n) tween1.push(t);
    }

    schedule.tween = tween1;
  };
}

function transition_tween(name, value) {
  var id = this._id;

  name += "";

  if (arguments.length < 2) {
    var tween = get(this.node(), id).tween;
    for (var i = 0, n = tween.length, t; i < n; ++i) {
      if ((t = tween[i]).name === name) {
        return t.value;
      }
    }
    return null;
  }

  return this.each((value == null ? tweenRemove : tweenFunction)(id, name, value));
}

function tweenValue(transition, name, value) {
  var id = transition._id;

  transition.each(function() {
    var schedule = set(this, id);
    (schedule.value || (schedule.value = {}))[name] = value.apply(this, arguments);
  });

  return function(node) {
    return get(node, id).value[name];
  };
}

function interpolate$1(a, b) {
  var c;
  return (typeof b === "number" ? interpolateNumber
      : b instanceof color ? interpolateRgb
      : (c = color(b)) ? (b = c, interpolateRgb)
      : interpolateString)(a, b);
}

function attrRemove(name) {
  return function() {
    this.removeAttribute(name);
  };
}

function attrRemoveNS(fullname) {
  return function() {
    this.removeAttributeNS(fullname.space, fullname.local);
  };
}

function attrConstant(name, interpolate, value1) {
  var string00,
      string1 = value1 + "",
      interpolate0;
  return function() {
    var string0 = this.getAttribute(name);
    return string0 === string1 ? null
        : string0 === string00 ? interpolate0
        : interpolate0 = interpolate(string00 = string0, value1);
  };
}

function attrConstantNS(fullname, interpolate, value1) {
  var string00,
      string1 = value1 + "",
      interpolate0;
  return function() {
    var string0 = this.getAttributeNS(fullname.space, fullname.local);
    return string0 === string1 ? null
        : string0 === string00 ? interpolate0
        : interpolate0 = interpolate(string00 = string0, value1);
  };
}

function attrFunction(name, interpolate, value) {
  var string00,
      string10,
      interpolate0;
  return function() {
    var string0, value1 = value(this), string1;
    if (value1 == null) return void this.removeAttribute(name);
    string0 = this.getAttribute(name);
    string1 = value1 + "";
    return string0 === string1 ? null
        : string0 === string00 && string1 === string10 ? interpolate0
        : (string10 = string1, interpolate0 = interpolate(string00 = string0, value1));
  };
}

function attrFunctionNS(fullname, interpolate, value) {
  var string00,
      string10,
      interpolate0;
  return function() {
    var string0, value1 = value(this), string1;
    if (value1 == null) return void this.removeAttributeNS(fullname.space, fullname.local);
    string0 = this.getAttributeNS(fullname.space, fullname.local);
    string1 = value1 + "";
    return string0 === string1 ? null
        : string0 === string00 && string1 === string10 ? interpolate0
        : (string10 = string1, interpolate0 = interpolate(string00 = string0, value1));
  };
}

function transition_attr(name, value) {
  var fullname = namespace(name), i = fullname === "transform" ? interpolateTransformSvg : interpolate$1;
  return this.attrTween(name, typeof value === "function"
      ? (fullname.local ? attrFunctionNS : attrFunction)(fullname, i, tweenValue(this, "attr." + name, value))
      : value == null ? (fullname.local ? attrRemoveNS : attrRemove)(fullname)
      : (fullname.local ? attrConstantNS : attrConstant)(fullname, i, value));
}

function attrInterpolate(name, i) {
  return function(t) {
    this.setAttribute(name, i.call(this, t));
  };
}

function attrInterpolateNS(fullname, i) {
  return function(t) {
    this.setAttributeNS(fullname.space, fullname.local, i.call(this, t));
  };
}

function attrTweenNS(fullname, value) {
  var t0, i0;
  function tween() {
    var i = value.apply(this, arguments);
    if (i !== i0) t0 = (i0 = i) && attrInterpolateNS(fullname, i);
    return t0;
  }
  tween._value = value;
  return tween;
}

function attrTween(name, value) {
  var t0, i0;
  function tween() {
    var i = value.apply(this, arguments);
    if (i !== i0) t0 = (i0 = i) && attrInterpolate(name, i);
    return t0;
  }
  tween._value = value;
  return tween;
}

function transition_attrTween(name, value) {
  var key = "attr." + name;
  if (arguments.length < 2) return (key = this.tween(key)) && key._value;
  if (value == null) return this.tween(key, null);
  if (typeof value !== "function") throw new Error;
  var fullname = namespace(name);
  return this.tween(key, (fullname.local ? attrTweenNS : attrTween)(fullname, value));
}

function delayFunction(id, value) {
  return function() {
    init(this, id).delay = +value.apply(this, arguments);
  };
}

function delayConstant(id, value) {
  return value = +value, function() {
    init(this, id).delay = value;
  };
}

function transition_delay(value) {
  var id = this._id;

  return arguments.length
      ? this.each((typeof value === "function"
          ? delayFunction
          : delayConstant)(id, value))
      : get(this.node(), id).delay;
}

function durationFunction(id, value) {
  return function() {
    set(this, id).duration = +value.apply(this, arguments);
  };
}

function durationConstant(id, value) {
  return value = +value, function() {
    set(this, id).duration = value;
  };
}

function transition_duration(value) {
  var id = this._id;

  return arguments.length
      ? this.each((typeof value === "function"
          ? durationFunction
          : durationConstant)(id, value))
      : get(this.node(), id).duration;
}

function easeConstant(id, value) {
  if (typeof value !== "function") throw new Error;
  return function() {
    set(this, id).ease = value;
  };
}

function transition_ease(value) {
  var id = this._id;

  return arguments.length
      ? this.each(easeConstant(id, value))
      : get(this.node(), id).ease;
}

function easeVarying(id, value) {
  return function() {
    var v = value.apply(this, arguments);
    if (typeof v !== "function") throw new Error;
    set(this, id).ease = v;
  };
}

function transition_easeVarying(value) {
  if (typeof value !== "function") throw new Error;
  return this.each(easeVarying(this._id, value));
}

function transition_filter(match) {
  if (typeof match !== "function") match = matcher(match);

  for (var groups = this._groups, m = groups.length, subgroups = new Array(m), j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, subgroup = subgroups[j] = [], node, i = 0; i < n; ++i) {
      if ((node = group[i]) && match.call(node, node.__data__, i, group)) {
        subgroup.push(node);
      }
    }
  }

  return new Transition(subgroups, this._parents, this._name, this._id);
}

function transition_merge(transition) {
  if (transition._id !== this._id) throw new Error;

  for (var groups0 = this._groups, groups1 = transition._groups, m0 = groups0.length, m1 = groups1.length, m = Math.min(m0, m1), merges = new Array(m0), j = 0; j < m; ++j) {
    for (var group0 = groups0[j], group1 = groups1[j], n = group0.length, merge = merges[j] = new Array(n), node, i = 0; i < n; ++i) {
      if (node = group0[i] || group1[i]) {
        merge[i] = node;
      }
    }
  }

  for (; j < m0; ++j) {
    merges[j] = groups0[j];
  }

  return new Transition(merges, this._parents, this._name, this._id);
}

function start(name) {
  return (name + "").trim().split(/^|\s+/).every(function(t) {
    var i = t.indexOf(".");
    if (i >= 0) t = t.slice(0, i);
    return !t || t === "start";
  });
}

function onFunction(id, name, listener) {
  var on0, on1, sit = start(name) ? init : set;
  return function() {
    var schedule = sit(this, id),
        on = schedule.on;

    // If this node shared a dispatch with the previous node,
    // just assign the updated shared dispatch and we’re done!
    // Otherwise, copy-on-write.
    if (on !== on0) (on1 = (on0 = on).copy()).on(name, listener);

    schedule.on = on1;
  };
}

function transition_on(name, listener) {
  var id = this._id;

  return arguments.length < 2
      ? get(this.node(), id).on.on(name)
      : this.each(onFunction(id, name, listener));
}

function removeFunction(id) {
  return function() {
    var parent = this.parentNode;
    for (var i in this.__transition) if (+i !== id) return;
    if (parent) parent.removeChild(this);
  };
}

function transition_remove() {
  return this.on("end.remove", removeFunction(this._id));
}

function transition_select(select) {
  var name = this._name,
      id = this._id;

  if (typeof select !== "function") select = selector(select);

  for (var groups = this._groups, m = groups.length, subgroups = new Array(m), j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, subgroup = subgroups[j] = new Array(n), node, subnode, i = 0; i < n; ++i) {
      if ((node = group[i]) && (subnode = select.call(node, node.__data__, i, group))) {
        if ("__data__" in node) subnode.__data__ = node.__data__;
        subgroup[i] = subnode;
        schedule(subgroup[i], name, id, i, subgroup, get(node, id));
      }
    }
  }

  return new Transition(subgroups, this._parents, name, id);
}

function transition_selectAll(select) {
  var name = this._name,
      id = this._id;

  if (typeof select !== "function") select = selectorAll(select);

  for (var groups = this._groups, m = groups.length, subgroups = [], parents = [], j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, node, i = 0; i < n; ++i) {
      if (node = group[i]) {
        for (var children = select.call(node, node.__data__, i, group), child, inherit = get(node, id), k = 0, l = children.length; k < l; ++k) {
          if (child = children[k]) {
            schedule(child, name, id, k, children, inherit);
          }
        }
        subgroups.push(children);
        parents.push(node);
      }
    }
  }

  return new Transition(subgroups, parents, name, id);
}

var Selection = selection.prototype.constructor;

function transition_selection() {
  return new Selection(this._groups, this._parents);
}

function styleNull(name, interpolate) {
  var string00,
      string10,
      interpolate0;
  return function() {
    var string0 = styleValue(this, name),
        string1 = (this.style.removeProperty(name), styleValue(this, name));
    return string0 === string1 ? null
        : string0 === string00 && string1 === string10 ? interpolate0
        : interpolate0 = interpolate(string00 = string0, string10 = string1);
  };
}

function styleRemove(name) {
  return function() {
    this.style.removeProperty(name);
  };
}

function styleConstant(name, interpolate, value1) {
  var string00,
      string1 = value1 + "",
      interpolate0;
  return function() {
    var string0 = styleValue(this, name);
    return string0 === string1 ? null
        : string0 === string00 ? interpolate0
        : interpolate0 = interpolate(string00 = string0, value1);
  };
}

function styleFunction(name, interpolate, value) {
  var string00,
      string10,
      interpolate0;
  return function() {
    var string0 = styleValue(this, name),
        value1 = value(this),
        string1 = value1 + "";
    if (value1 == null) string1 = value1 = (this.style.removeProperty(name), styleValue(this, name));
    return string0 === string1 ? null
        : string0 === string00 && string1 === string10 ? interpolate0
        : (string10 = string1, interpolate0 = interpolate(string00 = string0, value1));
  };
}

function styleMaybeRemove(id, name) {
  var on0, on1, listener0, key = "style." + name, event = "end." + key, remove;
  return function() {
    var schedule = set(this, id),
        on = schedule.on,
        listener = schedule.value[key] == null ? remove || (remove = styleRemove(name)) : undefined;

    // If this node shared a dispatch with the previous node,
    // just assign the updated shared dispatch and we’re done!
    // Otherwise, copy-on-write.
    if (on !== on0 || listener0 !== listener) (on1 = (on0 = on).copy()).on(event, listener0 = listener);

    schedule.on = on1;
  };
}

function transition_style(name, value, priority) {
  var i = (name += "") === "transform" ? interpolateTransformCss : interpolate$1;
  return value == null ? this
      .styleTween(name, styleNull(name, i))
      .on("end.style." + name, styleRemove(name))
    : typeof value === "function" ? this
      .styleTween(name, styleFunction(name, i, tweenValue(this, "style." + name, value)))
      .each(styleMaybeRemove(this._id, name))
    : this
      .styleTween(name, styleConstant(name, i, value), priority)
      .on("end.style." + name, null);
}

function styleInterpolate(name, i, priority) {
  return function(t) {
    this.style.setProperty(name, i.call(this, t), priority);
  };
}

function styleTween(name, value, priority) {
  var t, i0;
  function tween() {
    var i = value.apply(this, arguments);
    if (i !== i0) t = (i0 = i) && styleInterpolate(name, i, priority);
    return t;
  }
  tween._value = value;
  return tween;
}

function transition_styleTween(name, value, priority) {
  var key = "style." + (name += "");
  if (arguments.length < 2) return (key = this.tween(key)) && key._value;
  if (value == null) return this.tween(key, null);
  if (typeof value !== "function") throw new Error;
  return this.tween(key, styleTween(name, value, priority == null ? "" : priority));
}

function textConstant(value) {
  return function() {
    this.textContent = value;
  };
}

function textFunction(value) {
  return function() {
    var value1 = value(this);
    this.textContent = value1 == null ? "" : value1;
  };
}

function transition_text(value) {
  return this.tween("text", typeof value === "function"
      ? textFunction(tweenValue(this, "text", value))
      : textConstant(value == null ? "" : value + ""));
}

function textInterpolate(i) {
  return function(t) {
    this.textContent = i.call(this, t);
  };
}

function textTween(value) {
  var t0, i0;
  function tween() {
    var i = value.apply(this, arguments);
    if (i !== i0) t0 = (i0 = i) && textInterpolate(i);
    return t0;
  }
  tween._value = value;
  return tween;
}

function transition_textTween(value) {
  var key = "text";
  if (arguments.length < 1) return (key = this.tween(key)) && key._value;
  if (value == null) return this.tween(key, null);
  if (typeof value !== "function") throw new Error;
  return this.tween(key, textTween(value));
}

function transition_transition() {
  var name = this._name,
      id0 = this._id,
      id1 = newId();

  for (var groups = this._groups, m = groups.length, j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, node, i = 0; i < n; ++i) {
      if (node = group[i]) {
        var inherit = get(node, id0);
        schedule(node, name, id1, i, group, {
          time: inherit.time + inherit.delay + inherit.duration,
          delay: 0,
          duration: inherit.duration,
          ease: inherit.ease
        });
      }
    }
  }

  return new Transition(groups, this._parents, name, id1);
}

function transition_end() {
  var on0, on1, that = this, id = that._id, size = that.size();
  return new Promise(function(resolve, reject) {
    var cancel = {value: reject},
        end = {value: function() { if (--size === 0) resolve(); }};

    that.each(function() {
      var schedule = set(this, id),
          on = schedule.on;

      // If this node shared a dispatch with the previous node,
      // just assign the updated shared dispatch and we’re done!
      // Otherwise, copy-on-write.
      if (on !== on0) {
        on1 = (on0 = on).copy();
        on1._.cancel.push(cancel);
        on1._.interrupt.push(cancel);
        on1._.end.push(end);
      }

      schedule.on = on1;
    });

    // The selection was empty, resolve end immediately
    if (size === 0) resolve();
  });
}

var id = 0;

function Transition(groups, parents, name, id) {
  this._groups = groups;
  this._parents = parents;
  this._name = name;
  this._id = id;
}

function transition(name) {
  return selection().transition(name);
}

function newId() {
  return ++id;
}

var selection_prototype = selection.prototype;

Transition.prototype = transition.prototype = {
  constructor: Transition,
  select: transition_select,
  selectAll: transition_selectAll,
  selectChild: selection_prototype.selectChild,
  selectChildren: selection_prototype.selectChildren,
  filter: transition_filter,
  merge: transition_merge,
  selection: transition_selection,
  transition: transition_transition,
  call: selection_prototype.call,
  nodes: selection_prototype.nodes,
  node: selection_prototype.node,
  size: selection_prototype.size,
  empty: selection_prototype.empty,
  each: selection_prototype.each,
  on: transition_on,
  attr: transition_attr,
  attrTween: transition_attrTween,
  style: transition_style,
  styleTween: transition_styleTween,
  text: transition_text,
  textTween: transition_textTween,
  remove: transition_remove,
  tween: transition_tween,
  delay: transition_delay,
  duration: transition_duration,
  ease: transition_ease,
  easeVarying: transition_easeVarying,
  end: transition_end,
  [Symbol.iterator]: selection_prototype[Symbol.iterator]
};

const linear$1 = t => +t;

function quadIn(t) {
  return t * t;
}

function quadOut(t) {
  return t * (2 - t);
}

function quadInOut(t) {
  return ((t *= 2) <= 1 ? t * t : --t * (2 - t) + 1) / 2;
}

function cubicIn(t) {
  return t * t * t;
}

function cubicOut(t) {
  return --t * t * t + 1;
}

function cubicInOut(t) {
  return ((t *= 2) <= 1 ? t * t * t : (t -= 2) * t * t + 2) / 2;
}

var exponent$1 = 3;

var polyIn = (function custom(e) {
  e = +e;

  function polyIn(t) {
    return Math.pow(t, e);
  }

  polyIn.exponent = custom;

  return polyIn;
})(exponent$1);

var polyOut = (function custom(e) {
  e = +e;

  function polyOut(t) {
    return 1 - Math.pow(1 - t, e);
  }

  polyOut.exponent = custom;

  return polyOut;
})(exponent$1);

var polyInOut = (function custom(e) {
  e = +e;

  function polyInOut(t) {
    return ((t *= 2) <= 1 ? Math.pow(t, e) : 2 - Math.pow(2 - t, e)) / 2;
  }

  polyInOut.exponent = custom;

  return polyInOut;
})(exponent$1);

var pi$4 = Math.PI,
    halfPi$3 = pi$4 / 2;

function sinIn(t) {
  return (+t === 1) ? 1 : 1 - Math.cos(t * halfPi$3);
}

function sinOut(t) {
  return Math.sin(t * halfPi$3);
}

function sinInOut(t) {
  return (1 - Math.cos(pi$4 * t)) / 2;
}

// tpmt is two power minus ten times t scaled to [0,1]
function tpmt(x) {
  return (Math.pow(2, -10 * x) - 0.0009765625) * 1.0009775171065494;
}

function expIn(t) {
  return tpmt(1 - +t);
}

function expOut(t) {
  return 1 - tpmt(t);
}

function expInOut(t) {
  return ((t *= 2) <= 1 ? tpmt(1 - t) : 2 - tpmt(t - 1)) / 2;
}

function circleIn(t) {
  return 1 - Math.sqrt(1 - t * t);
}

function circleOut(t) {
  return Math.sqrt(1 - --t * t);
}

function circleInOut(t) {
  return ((t *= 2) <= 1 ? 1 - Math.sqrt(1 - t * t) : Math.sqrt(1 - (t -= 2) * t) + 1) / 2;
}

var b1 = 4 / 11,
    b2 = 6 / 11,
    b3 = 8 / 11,
    b4 = 3 / 4,
    b5 = 9 / 11,
    b6 = 10 / 11,
    b7 = 15 / 16,
    b8 = 21 / 22,
    b9 = 63 / 64,
    b0 = 1 / b1 / b1;

function bounceIn(t) {
  return 1 - bounceOut(1 - t);
}

function bounceOut(t) {
  return (t = +t) < b1 ? b0 * t * t : t < b3 ? b0 * (t -= b2) * t + b4 : t < b6 ? b0 * (t -= b5) * t + b7 : b0 * (t -= b8) * t + b9;
}

function bounceInOut(t) {
  return ((t *= 2) <= 1 ? 1 - bounceOut(1 - t) : bounceOut(t - 1) + 1) / 2;
}

var overshoot = 1.70158;

var backIn = (function custom(s) {
  s = +s;

  function backIn(t) {
    return (t = +t) * t * (s * (t - 1) + t);
  }

  backIn.overshoot = custom;

  return backIn;
})(overshoot);

var backOut = (function custom(s) {
  s = +s;

  function backOut(t) {
    return --t * t * ((t + 1) * s + t) + 1;
  }

  backOut.overshoot = custom;

  return backOut;
})(overshoot);

var backInOut = (function custom(s) {
  s = +s;

  function backInOut(t) {
    return ((t *= 2) < 1 ? t * t * ((s + 1) * t - s) : (t -= 2) * t * ((s + 1) * t + s) + 2) / 2;
  }

  backInOut.overshoot = custom;

  return backInOut;
})(overshoot);

var tau$5 = 2 * Math.PI,
    amplitude = 1,
    period = 0.3;

var elasticIn = (function custom(a, p) {
  var s = Math.asin(1 / (a = Math.max(1, a))) * (p /= tau$5);

  function elasticIn(t) {
    return a * tpmt(-(--t)) * Math.sin((s - t) / p);
  }

  elasticIn.amplitude = function(a) { return custom(a, p * tau$5); };
  elasticIn.period = function(p) { return custom(a, p); };

  return elasticIn;
})(amplitude, period);

var elasticOut = (function custom(a, p) {
  var s = Math.asin(1 / (a = Math.max(1, a))) * (p /= tau$5);

  function elasticOut(t) {
    return 1 - a * tpmt(t = +t) * Math.sin((t + s) / p);
  }

  elasticOut.amplitude = function(a) { return custom(a, p * tau$5); };
  elasticOut.period = function(p) { return custom(a, p); };

  return elasticOut;
})(amplitude, period);

var elasticInOut = (function custom(a, p) {
  var s = Math.asin(1 / (a = Math.max(1, a))) * (p /= tau$5);

  function elasticInOut(t) {
    return ((t = t * 2 - 1) < 0
        ? a * tpmt(-t) * Math.sin((s - t) / p)
        : 2 - a * tpmt(t) * Math.sin((s + t) / p)) / 2;
  }

  elasticInOut.amplitude = function(a) { return custom(a, p * tau$5); };
  elasticInOut.period = function(p) { return custom(a, p); };

  return elasticInOut;
})(amplitude, period);

var defaultTiming = {
  time: null, // Set on use.
  delay: 0,
  duration: 250,
  ease: cubicInOut
};

function inherit(node, id) {
  var timing;
  while (!(timing = node.__transition) || !(timing = timing[id])) {
    if (!(node = node.parentNode)) {
      throw new Error(`transition ${id} not found`);
    }
  }
  return timing;
}

function selection_transition(name) {
  var id,
      timing;

  if (name instanceof Transition) {
    id = name._id, name = name._name;
  } else {
    id = newId(), (timing = defaultTiming).time = now(), name = name == null ? null : name + "";
  }

  for (var groups = this._groups, m = groups.length, j = 0; j < m; ++j) {
    for (var group = groups[j], n = group.length, node, i = 0; i < n; ++i) {
      if (node = group[i]) {
        schedule(node, name, id, i, group, timing || inherit(node, id));
      }
    }
  }

  return new Transition(groups, this._parents, name, id);
}

selection.prototype.interrupt = selection_interrupt;
selection.prototype.transition = selection_transition;

var root = [null];

function active(node, name) {
  var schedules = node.__transition,
      schedule,
      i;

  if (schedules) {
    name = name == null ? null : name + "";
    for (i in schedules) {
      if ((schedule = schedules[i]).state > SCHEDULED && schedule.name === name) {
        return new Transition([[node]], root, name, +i);
      }
    }
  }

  return null;
}

const constant$7 = x => () => x;

function BrushEvent(type, {
  sourceEvent,
  target,
  selection,
  mode,
  dispatch
}) {
  Object.defineProperties(this, {
    type: {value: type, enumerable: true, configurable: true},
    sourceEvent: {value: sourceEvent, enumerable: true, configurable: true},
    target: {value: target, enumerable: true, configurable: true},
    selection: {value: selection, enumerable: true, configurable: true},
    mode: {value: mode, enumerable: true, configurable: true},
    _: {value: dispatch}
  });
}

function nopropagation$1(event) {
  event.stopImmediatePropagation();
}

function noevent$1(event) {
  event.preventDefault();
  event.stopImmediatePropagation();
}

var MODE_DRAG = {name: "drag"},
    MODE_SPACE = {name: "space"},
    MODE_HANDLE = {name: "handle"},
    MODE_CENTER = {name: "center"};

const {abs: abs$3, max: max$2, min: min$1} = Math;

function number1(e) {
  return [+e[0], +e[1]];
}

function number2(e) {
  return [number1(e[0]), number1(e[1])];
}

var X = {
  name: "x",
  handles: ["w", "e"].map(type),
  input: function(x, e) { return x == null ? null : [[+x[0], e[0][1]], [+x[1], e[1][1]]]; },
  output: function(xy) { return xy && [xy[0][0], xy[1][0]]; }
};

var Y = {
  name: "y",
  handles: ["n", "s"].map(type),
  input: function(y, e) { return y == null ? null : [[e[0][0], +y[0]], [e[1][0], +y[1]]]; },
  output: function(xy) { return xy && [xy[0][1], xy[1][1]]; }
};

var XY = {
  name: "xy",
  handles: ["n", "w", "e", "s", "nw", "ne", "sw", "se"].map(type),
  input: function(xy) { return xy == null ? null : number2(xy); },
  output: function(xy) { return xy; }
};

var cursors = {
  overlay: "crosshair",
  selection: "move",
  n: "ns-resize",
  e: "ew-resize",
  s: "ns-resize",
  w: "ew-resize",
  nw: "nwse-resize",
  ne: "nesw-resize",
  se: "nwse-resize",
  sw: "nesw-resize"
};

var flipX = {
  e: "w",
  w: "e",
  nw: "ne",
  ne: "nw",
  se: "sw",
  sw: "se"
};

var flipY = {
  n: "s",
  s: "n",
  nw: "sw",
  ne: "se",
  se: "ne",
  sw: "nw"
};

var signsX = {
  overlay: +1,
  selection: +1,
  n: null,
  e: +1,
  s: null,
  w: -1,
  nw: -1,
  ne: +1,
  se: +1,
  sw: -1
};

var signsY = {
  overlay: +1,
  selection: +1,
  n: -1,
  e: null,
  s: +1,
  w: null,
  nw: -1,
  ne: -1,
  se: +1,
  sw: +1
};

function type(t) {
  return {type: t};
}

// Ignore right-click, since that should open the context menu.
function defaultFilter$1(event) {
  return !event.ctrlKey && !event.button;
}

function defaultExtent$1() {
  var svg = this.ownerSVGElement || this;
  if (svg.hasAttribute("viewBox")) {
    svg = svg.viewBox.baseVal;
    return [[svg.x, svg.y], [svg.x + svg.width, svg.y + svg.height]];
  }
  return [[0, 0], [svg.width.baseVal.value, svg.height.baseVal.value]];
}

function defaultTouchable$1() {
  return navigator.maxTouchPoints || ("ontouchstart" in this);
}

// Like d3.local, but with the name “__brush” rather than auto-generated.
function local(node) {
  while (!node.__brush) if (!(node = node.parentNode)) return;
  return node.__brush;
}

function empty(extent) {
  return extent[0][0] === extent[1][0]
      || extent[0][1] === extent[1][1];
}

function brushSelection(node) {
  var state = node.__brush;
  return state ? state.dim.output(state.selection) : null;
}

function brushX() {
  return brush(X);
}

function brushY() {
  return brush(Y);
}

function brush_default() {
  return brush(XY);
}

function brush(dim) {
  var extent = defaultExtent$1,
      filter = defaultFilter$1,
      touchable = defaultTouchable$1,
      keys = true,
      listeners = dispatch("start", "brush", "end"),
      handleSize = 6,
      touchending;

  function brush(group) {
    var overlay = group
        .property("__brush", initialize)
      .selectAll(".overlay")
      .data([type("overlay")]);

    overlay.enter().append("rect")
        .attr("class", "overlay")
        .attr("pointer-events", "all")
        .attr("cursor", cursors.overlay)
      .merge(overlay)
        .each(function() {
          var extent = local(this).extent;
          select(this)
              .attr("x", extent[0][0])
              .attr("y", extent[0][1])
              .attr("width", extent[1][0] - extent[0][0])
              .attr("height", extent[1][1] - extent[0][1]);
        });

    group.selectAll(".selection")
      .data([type("selection")])
      .enter().append("rect")
        .attr("class", "selection")
        .attr("cursor", cursors.selection)
        .attr("fill", "#777")
        .attr("fill-opacity", 0.3)
        .attr("stroke", "#fff")
        .attr("shape-rendering", "crispEdges");

    var handle = group.selectAll(".handle")
      .data(dim.handles, function(d) { return d.type; });

    handle.exit().remove();

    handle.enter().append("rect")
        .attr("class", function(d) { return "handle handle--" + d.type; })
        .attr("cursor", function(d) { return cursors[d.type]; });

    group
        .each(redraw)
        .attr("fill", "none")
        .attr("pointer-events", "all")
        .on("mousedown.brush", started)
      .filter(touchable)
        .on("touchstart.brush", started)
        .on("touchmove.brush", touchmoved)
        .on("touchend.brush touchcancel.brush", touchended)
        .style("touch-action", "none")
        .style("-webkit-tap-highlight-color", "rgba(0,0,0,0)");
  }

  brush.move = function(group, selection, event) {
    if (group.tween) {
      group
          .on("start.brush", function(event) { emitter(this, arguments).beforestart().start(event); })
          .on("interrupt.brush end.brush", function(event) { emitter(this, arguments).end(event); })
          .tween("brush", function() {
            var that = this,
                state = that.__brush,
                emit = emitter(that, arguments),
                selection0 = state.selection,
                selection1 = dim.input(typeof selection === "function" ? selection.apply(this, arguments) : selection, state.extent),
                i = interpolate$2(selection0, selection1);

            function tween(t) {
              state.selection = t === 1 && selection1 === null ? null : i(t);
              redraw.call(that);
              emit.brush();
            }

            return selection0 !== null && selection1 !== null ? tween : tween(1);
          });
    } else {
      group
          .each(function() {
            var that = this,
                args = arguments,
                state = that.__brush,
                selection1 = dim.input(typeof selection === "function" ? selection.apply(that, args) : selection, state.extent),
                emit = emitter(that, args).beforestart();

            interrupt(that);
            state.selection = selection1 === null ? null : selection1;
            redraw.call(that);
            emit.start(event).brush(event).end(event);
          });
    }
  };

  brush.clear = function(group, event) {
    brush.move(group, null, event);
  };

  function redraw() {
    var group = select(this),
        selection = local(this).selection;

    if (selection) {
      group.selectAll(".selection")
          .style("display", null)
          .attr("x", selection[0][0])
          .attr("y", selection[0][1])
          .attr("width", selection[1][0] - selection[0][0])
          .attr("height", selection[1][1] - selection[0][1]);

      group.selectAll(".handle")
          .style("display", null)
          .attr("x", function(d) { return d.type[d.type.length - 1] === "e" ? selection[1][0] - handleSize / 2 : selection[0][0] - handleSize / 2; })
          .attr("y", function(d) { return d.type[0] === "s" ? selection[1][1] - handleSize / 2 : selection[0][1] - handleSize / 2; })
          .attr("width", function(d) { return d.type === "n" || d.type === "s" ? selection[1][0] - selection[0][0] + handleSize : handleSize; })
          .attr("height", function(d) { return d.type === "e" || d.type === "w" ? selection[1][1] - selection[0][1] + handleSize : handleSize; });
    }

    else {
      group.selectAll(".selection,.handle")
          .style("display", "none")
          .attr("x", null)
          .attr("y", null)
          .attr("width", null)
          .attr("height", null);
    }
  }

  function emitter(that, args, clean) {
    var emit = that.__brush.emitter;
    return emit && (!clean || !emit.clean) ? emit : new Emitter(that, args, clean);
  }

  function Emitter(that, args, clean) {
    this.that = that;
    this.args = args;
    this.state = that.__brush;
    this.active = 0;
    this.clean = clean;
  }

  Emitter.prototype = {
    beforestart: function() {
      if (++this.active === 1) this.state.emitter = this, this.starting = true;
      return this;
    },
    start: function(event, mode) {
      if (this.starting) this.starting = false, this.emit("start", event, mode);
      else this.emit("brush", event);
      return this;
    },
    brush: function(event, mode) {
      this.emit("brush", event, mode);
      return this;
    },
    end: function(event, mode) {
      if (--this.active === 0) delete this.state.emitter, this.emit("end", event, mode);
      return this;
    },
    emit: function(type, event, mode) {
      var d = select(this.that).datum();
      listeners.call(
        type,
        this.that,
        new BrushEvent(type, {
          sourceEvent: event,
          target: brush,
          selection: dim.output(this.state.selection),
          mode,
          dispatch: listeners
        }),
        d
      );
    }
  };

  function started(event) {
    if (touchending && !event.touches) return;
    if (!filter.apply(this, arguments)) return;

    var that = this,
        type = event.target.__data__.type,
        mode = (keys && event.metaKey ? type = "overlay" : type) === "selection" ? MODE_DRAG : (keys && event.altKey ? MODE_CENTER : MODE_HANDLE),
        signX = dim === Y ? null : signsX[type],
        signY = dim === X ? null : signsY[type],
        state = local(that),
        extent = state.extent,
        selection = state.selection,
        W = extent[0][0], w0, w1,
        N = extent[0][1], n0, n1,
        E = extent[1][0], e0, e1,
        S = extent[1][1], s0, s1,
        dx = 0,
        dy = 0,
        moving,
        shifting = signX && signY && keys && event.shiftKey,
        lockX,
        lockY,
        points = Array.from(event.touches || [event], t => {
          const i = t.identifier;
          t = pointer(t, that);
          t.point0 = t.slice();
          t.identifier = i;
          return t;
        });

    interrupt(that);
    var emit = emitter(that, arguments, true).beforestart();

    if (type === "overlay") {
      if (selection) moving = true;
      const pts = [points[0], points[1] || points[0]];
      state.selection = selection = [[
          w0 = dim === Y ? W : min$1(pts[0][0], pts[1][0]),
          n0 = dim === X ? N : min$1(pts[0][1], pts[1][1])
        ], [
          e0 = dim === Y ? E : max$2(pts[0][0], pts[1][0]),
          s0 = dim === X ? S : max$2(pts[0][1], pts[1][1])
        ]];
      if (points.length > 1) move(event);
    } else {
      w0 = selection[0][0];
      n0 = selection[0][1];
      e0 = selection[1][0];
      s0 = selection[1][1];
    }

    w1 = w0;
    n1 = n0;
    e1 = e0;
    s1 = s0;

    var group = select(that)
        .attr("pointer-events", "none");

    var overlay = group.selectAll(".overlay")
        .attr("cursor", cursors[type]);

    if (event.touches) {
      emit.moved = moved;
      emit.ended = ended;
    } else {
      var view = select(event.view)
          .on("mousemove.brush", moved, true)
          .on("mouseup.brush", ended, true);
      if (keys) view
          .on("keydown.brush", keydowned, true)
          .on("keyup.brush", keyupped, true);

      dragDisable(event.view);
    }

    redraw.call(that);
    emit.start(event, mode.name);

    function moved(event) {
      for (const p of event.changedTouches || [event]) {
        for (const d of points)
          if (d.identifier === p.identifier) d.cur = pointer(p, that);
      }
      if (shifting && !lockX && !lockY && points.length === 1) {
        const point = points[0];
        if (abs$3(point.cur[0] - point[0]) > abs$3(point.cur[1] - point[1]))
          lockY = true;
        else
          lockX = true;
      }
      for (const point of points)
        if (point.cur) point[0] = point.cur[0], point[1] = point.cur[1];
      moving = true;
      noevent$1(event);
      move(event);
    }

    function move(event) {
      const point = points[0], point0 = point.point0;
      var t;

      dx = point[0] - point0[0];
      dy = point[1] - point0[1];

      switch (mode) {
        case MODE_SPACE:
        case MODE_DRAG: {
          if (signX) dx = max$2(W - w0, min$1(E - e0, dx)), w1 = w0 + dx, e1 = e0 + dx;
          if (signY) dy = max$2(N - n0, min$1(S - s0, dy)), n1 = n0 + dy, s1 = s0 + dy;
          break;
        }
        case MODE_HANDLE: {
          if (points[1]) {
            if (signX) w1 = max$2(W, min$1(E, points[0][0])), e1 = max$2(W, min$1(E, points[1][0])), signX = 1;
            if (signY) n1 = max$2(N, min$1(S, points[0][1])), s1 = max$2(N, min$1(S, points[1][1])), signY = 1;
          } else {
            if (signX < 0) dx = max$2(W - w0, min$1(E - w0, dx)), w1 = w0 + dx, e1 = e0;
            else if (signX > 0) dx = max$2(W - e0, min$1(E - e0, dx)), w1 = w0, e1 = e0 + dx;
            if (signY < 0) dy = max$2(N - n0, min$1(S - n0, dy)), n1 = n0 + dy, s1 = s0;
            else if (signY > 0) dy = max$2(N - s0, min$1(S - s0, dy)), n1 = n0, s1 = s0 + dy;
          }
          break;
        }
        case MODE_CENTER: {
          if (signX) w1 = max$2(W, min$1(E, w0 - dx * signX)), e1 = max$2(W, min$1(E, e0 + dx * signX));
          if (signY) n1 = max$2(N, min$1(S, n0 - dy * signY)), s1 = max$2(N, min$1(S, s0 + dy * signY));
          break;
        }
      }

      if (e1 < w1) {
        signX *= -1;
        t = w0, w0 = e0, e0 = t;
        t = w1, w1 = e1, e1 = t;
        if (type in flipX) overlay.attr("cursor", cursors[type = flipX[type]]);
      }

      if (s1 < n1) {
        signY *= -1;
        t = n0, n0 = s0, s0 = t;
        t = n1, n1 = s1, s1 = t;
        if (type in flipY) overlay.attr("cursor", cursors[type = flipY[type]]);
      }

      if (state.selection) selection = state.selection; // May be set by brush.move!
      if (lockX) w1 = selection[0][0], e1 = selection[1][0];
      if (lockY) n1 = selection[0][1], s1 = selection[1][1];

      if (selection[0][0] !== w1
          || selection[0][1] !== n1
          || selection[1][0] !== e1
          || selection[1][1] !== s1) {
        state.selection = [[w1, n1], [e1, s1]];
        redraw.call(that);
        emit.brush(event, mode.name);
      }
    }

    function ended(event) {
      nopropagation$1(event);
      if (event.touches) {
        if (event.touches.length) return;
        if (touchending) clearTimeout(touchending);
        touchending = setTimeout(function() { touchending = null; }, 500); // Ghost clicks are delayed!
      } else {
        yesdrag(event.view, moving);
        view.on("keydown.brush keyup.brush mousemove.brush mouseup.brush", null);
      }
      group.attr("pointer-events", "all");
      overlay.attr("cursor", cursors.overlay);
      if (state.selection) selection = state.selection; // May be set by brush.move (on start)!
      if (empty(selection)) state.selection = null, redraw.call(that);
      emit.end(event, mode.name);
    }

    function keydowned(event) {
      switch (event.keyCode) {
        case 16: { // SHIFT
          shifting = signX && signY;
          break;
        }
        case 18: { // ALT
          if (mode === MODE_HANDLE) {
            if (signX) e0 = e1 - dx * signX, w0 = w1 + dx * signX;
            if (signY) s0 = s1 - dy * signY, n0 = n1 + dy * signY;
            mode = MODE_CENTER;
            move(event);
          }
          break;
        }
        case 32: { // SPACE; takes priority over ALT
          if (mode === MODE_HANDLE || mode === MODE_CENTER) {
            if (signX < 0) e0 = e1 - dx; else if (signX > 0) w0 = w1 - dx;
            if (signY < 0) s0 = s1 - dy; else if (signY > 0) n0 = n1 - dy;
            mode = MODE_SPACE;
            overlay.attr("cursor", cursors.selection);
            move(event);
          }
          break;
        }
        default: return;
      }
      noevent$1(event);
    }

    function keyupped(event) {
      switch (event.keyCode) {
        case 16: { // SHIFT
          if (shifting) {
            lockX = lockY = shifting = false;
            move(event);
          }
          break;
        }
        case 18: { // ALT
          if (mode === MODE_CENTER) {
            if (signX < 0) e0 = e1; else if (signX > 0) w0 = w1;
            if (signY < 0) s0 = s1; else if (signY > 0) n0 = n1;
            mode = MODE_HANDLE;
            move(event);
          }
          break;
        }
        case 32: { // SPACE
          if (mode === MODE_SPACE) {
            if (event.altKey) {
              if (signX) e0 = e1 - dx * signX, w0 = w1 + dx * signX;
              if (signY) s0 = s1 - dy * signY, n0 = n1 + dy * signY;
              mode = MODE_CENTER;
            } else {
              if (signX < 0) e0 = e1; else if (signX > 0) w0 = w1;
              if (signY < 0) s0 = s1; else if (signY > 0) n0 = n1;
              mode = MODE_HANDLE;
            }
            overlay.attr("cursor", cursors[type]);
            move(event);
          }
          break;
        }
        default: return;
      }
      noevent$1(event);
    }
  }

  function touchmoved(event) {
    emitter(this, arguments).moved(event);
  }

  function touchended(event) {
    emitter(this, arguments).ended(event);
  }

  function initialize() {
    var state = this.__brush || {selection: null};
    state.extent = number2(extent.apply(this, arguments));
    state.dim = dim;
    return state;
  }

  brush.extent = function(_) {
    return arguments.length ? (extent = typeof _ === "function" ? _ : constant$7(number2(_)), brush) : extent;
  };

  brush.filter = function(_) {
    return arguments.length ? (filter = typeof _ === "function" ? _ : constant$7(!!_), brush) : filter;
  };

  brush.touchable = function(_) {
    return arguments.length ? (touchable = typeof _ === "function" ? _ : constant$7(!!_), brush) : touchable;
  };

  brush.handleSize = function(_) {
    return arguments.length ? (handleSize = +_, brush) : handleSize;
  };

  brush.keyModifiers = function(_) {
    return arguments.length ? (keys = !!_, brush) : keys;
  };

  brush.on = function() {
    var value = listeners.on.apply(listeners, arguments);
    return value === listeners ? brush : value;
  };

  return brush;
}

var abs$2 = Math.abs;
var cos$2 = Math.cos;
var sin$2 = Math.sin;
var pi$3 = Math.PI;
var halfPi$2 = pi$3 / 2;
var tau$4 = pi$3 * 2;
var max$1 = Math.max;
var epsilon$5 = 1e-12;

function range$1(i, j) {
  return Array.from({length: j - i}, (_, k) => i + k);
}

function compareValue(compare) {
  return function(a, b) {
    return compare(
      a.source.value + a.target.value,
      b.source.value + b.target.value
    );
  };
}

function chord_default() {
  return chord(false, false);
}

function chordTranspose() {
  return chord(false, true);
}

function chordDirected() {
  return chord(true, false);
}

function chord(directed, transpose) {
  var padAngle = 0,
      sortGroups = null,
      sortSubgroups = null,
      sortChords = null;

  function chord(matrix) {
    var n = matrix.length,
        groupSums = new Array(n),
        groupIndex = range$1(0, n),
        chords = new Array(n * n),
        groups = new Array(n),
        k = 0, dx;

    matrix = Float64Array.from({length: n * n}, transpose
        ? (_, i) => matrix[i % n][i / n | 0]
        : (_, i) => matrix[i / n | 0][i % n]);

    // Compute the scaling factor from value to angle in [0, 2pi].
    for (let i = 0; i < n; ++i) {
      let x = 0;
      for (let j = 0; j < n; ++j) x += matrix[i * n + j] + directed * matrix[j * n + i];
      k += groupSums[i] = x;
    }
    k = max$1(0, tau$4 - padAngle * n) / k;
    dx = k ? padAngle : tau$4 / n;

    // Compute the angles for each group and constituent chord.
    {
      let x = 0;
      if (sortGroups) groupIndex.sort((a, b) => sortGroups(groupSums[a], groupSums[b]));
      for (const i of groupIndex) {
        const x0 = x;
        if (directed) {
          const subgroupIndex = range$1(~n + 1, n).filter(j => j < 0 ? matrix[~j * n + i] : matrix[i * n + j]);
          if (sortSubgroups) subgroupIndex.sort((a, b) => sortSubgroups(a < 0 ? -matrix[~a * n + i] : matrix[i * n + a], b < 0 ? -matrix[~b * n + i] : matrix[i * n + b]));
          for (const j of subgroupIndex) {
            if (j < 0) {
              const chord = chords[~j * n + i] || (chords[~j * n + i] = {source: null, target: null});
              chord.target = {index: i, startAngle: x, endAngle: x += matrix[~j * n + i] * k, value: matrix[~j * n + i]};
            } else {
              const chord = chords[i * n + j] || (chords[i * n + j] = {source: null, target: null});
              chord.source = {index: i, startAngle: x, endAngle: x += matrix[i * n + j] * k, value: matrix[i * n + j]};
            }
          }
          groups[i] = {index: i, startAngle: x0, endAngle: x, value: groupSums[i]};
        } else {
          const subgroupIndex = range$1(0, n).filter(j => matrix[i * n + j] || matrix[j * n + i]);
          if (sortSubgroups) subgroupIndex.sort((a, b) => sortSubgroups(matrix[i * n + a], matrix[i * n + b]));
          for (const j of subgroupIndex) {
            let chord;
            if (i < j) {
              chord = chords[i * n + j] || (chords[i * n + j] = {source: null, target: null});
              chord.source = {index: i, startAngle: x, endAngle: x += matrix[i * n + j] * k, value: matrix[i * n + j]};
            } else {
              chord = chords[j * n + i] || (chords[j * n + i] = {source: null, target: null});
              chord.target = {index: i, startAngle: x, endAngle: x += matrix[i * n + j] * k, value: matrix[i * n + j]};
              if (i === j) chord.source = chord.target;
            }
            if (chord.source && chord.target && chord.source.value < chord.target.value) {
              const source = chord.source;
              chord.source = chord.target;
              chord.target = source;
            }
          }
          groups[i] = {index: i, startAngle: x0, endAngle: x, value: groupSums[i]};
        }
        x += dx;
      }
    }

    // Remove empty chords.
    chords = Object.values(chords);
    chords.groups = groups;
    return sortChords ? chords.sort(sortChords) : chords;
  }

  chord.padAngle = function(_) {
    return arguments.length ? (padAngle = max$1(0, _), chord) : padAngle;
  };

  chord.sortGroups = function(_) {
    return arguments.length ? (sortGroups = _, chord) : sortGroups;
  };

  chord.sortSubgroups = function(_) {
    return arguments.length ? (sortSubgroups = _, chord) : sortSubgroups;
  };

  chord.sortChords = function(_) {
    return arguments.length ? (_ == null ? sortChords = null : (sortChords = compareValue(_))._ = _, chord) : sortChords && sortChords._;
  };

  return chord;
}

const pi$2 = Math.PI,
    tau$3 = 2 * pi$2,
    epsilon$4 = 1e-6,
    tauEpsilon = tau$3 - epsilon$4;

function append$1(strings) {
  this._ += strings[0];
  for (let i = 1, n = strings.length; i < n; ++i) {
    this._ += arguments[i] + strings[i];
  }
}

function appendRound$1(digits) {
  let d = Math.floor(digits);
  if (!(d >= 0)) throw new Error(`invalid digits: ${digits}`);
  if (d > 15) return append$1;
  const k = 10 ** d;
  return function(strings) {
    this._ += strings[0];
    for (let i = 1, n = strings.length; i < n; ++i) {
      this._ += Math.round(arguments[i] * k) / k + strings[i];
    }
  };
}

let Path$1 = class Path {
  constructor(digits) {
    this._x0 = this._y0 = // start of current subpath
    this._x1 = this._y1 = null; // end of current subpath
    this._ = "";
    this._append = digits == null ? append$1 : appendRound$1(digits);
  }
  moveTo(x, y) {
    this._append`M${this._x0 = this._x1 = +x},${this._y0 = this._y1 = +y}`;
  }
  closePath() {
    if (this._x1 !== null) {
      this._x1 = this._x0, this._y1 = this._y0;
      this._append`Z`;
    }
  }
  lineTo(x, y) {
    this._append`L${this._x1 = +x},${this._y1 = +y}`;
  }
  quadraticCurveTo(x1, y1, x, y) {
    this._append`Q${+x1},${+y1},${this._x1 = +x},${this._y1 = +y}`;
  }
  bezierCurveTo(x1, y1, x2, y2, x, y) {
    this._append`C${+x1},${+y1},${+x2},${+y2},${this._x1 = +x},${this._y1 = +y}`;
  }
  arcTo(x1, y1, x2, y2, r) {
    x1 = +x1, y1 = +y1, x2 = +x2, y2 = +y2, r = +r;

    // Is the radius negative? Error.
    if (r < 0) throw new Error(`negative radius: ${r}`);

    let x0 = this._x1,
        y0 = this._y1,
        x21 = x2 - x1,
        y21 = y2 - y1,
        x01 = x0 - x1,
        y01 = y0 - y1,
        l01_2 = x01 * x01 + y01 * y01;

    // Is this path empty? Move to (x1,y1).
    if (this._x1 === null) {
      this._append`M${this._x1 = x1},${this._y1 = y1}`;
    }

    // Or, is (x1,y1) coincident with (x0,y0)? Do nothing.
    else if (!(l01_2 > epsilon$4));

    // Or, are (x0,y0), (x1,y1) and (x2,y2) collinear?
    // Equivalently, is (x1,y1) coincident with (x2,y2)?
    // Or, is the radius zero? Line to (x1,y1).
    else if (!(Math.abs(y01 * x21 - y21 * x01) > epsilon$4) || !r) {
      this._append`L${this._x1 = x1},${this._y1 = y1}`;
    }

    // Otherwise, draw an arc!
    else {
      let x20 = x2 - x0,
          y20 = y2 - y0,
          l21_2 = x21 * x21 + y21 * y21,
          l20_2 = x20 * x20 + y20 * y20,
          l21 = Math.sqrt(l21_2),
          l01 = Math.sqrt(l01_2),
          l = r * Math.tan((pi$2 - Math.acos((l21_2 + l01_2 - l20_2) / (2 * l21 * l01))) / 2),
          t01 = l / l01,
          t21 = l / l21;

      // If the start tangent is not coincident with (x0,y0), line to.
      if (Math.abs(t01 - 1) > epsilon$4) {
        this._append`L${x1 + t01 * x01},${y1 + t01 * y01}`;
      }

      this._append`A${r},${r},0,0,${+(y01 * x20 > x01 * y20)},${this._x1 = x1 + t21 * x21},${this._y1 = y1 + t21 * y21}`;
    }
  }
  arc(x, y, r, a0, a1, ccw) {
    x = +x, y = +y, r = +r, ccw = !!ccw;

    // Is the radius negative? Error.
    if (r < 0) throw new Error(`negative radius: ${r}`);

    let dx = r * Math.cos(a0),
        dy = r * Math.sin(a0),
        x0 = x + dx,
        y0 = y + dy,
        cw = 1 ^ ccw,
        da = ccw ? a0 - a1 : a1 - a0;

    // Is this path empty? Move to (x0,y0).
    if (this._x1 === null) {
      this._append`M${x0},${y0}`;
    }

    // Or, is (x0,y0) not coincident with the previous point? Line to (x0,y0).
    else if (Math.abs(this._x1 - x0) > epsilon$4 || Math.abs(this._y1 - y0) > epsilon$4) {
      this._append`L${x0},${y0}`;
    }

    // Is this arc empty? We’re done.
    if (!r) return;

    // Does the angle go the wrong way? Flip the direction.
    if (da < 0) da = da % tau$3 + tau$3;

    // Is this a complete circle? Draw two arcs to complete the circle.
    if (da > tauEpsilon) {
      this._append`A${r},${r},0,1,${cw},${x - dx},${y - dy}A${r},${r},0,1,${cw},${this._x1 = x0},${this._y1 = y0}`;
    }

    // Is this arc non-empty? Draw an arc!
    else if (da > epsilon$4) {
      this._append`A${r},${r},0,${+(da >= pi$2)},${cw},${this._x1 = x + r * Math.cos(a1)},${this._y1 = y + r * Math.sin(a1)}`;
    }
  }
  rect(x, y, w, h) {
    this._append`M${this._x0 = this._x1 = +x},${this._y0 = this._y1 = +y}h${w = +w}v${+h}h${-w}Z`;
  }
  toString() {
    return this._;
  }
};

function path() {
  return new Path$1;
}

// Allow instanceof d3.path
path.prototype = Path$1.prototype;

function pathRound(digits = 3) {
  return new Path$1(+digits);
}

var slice$2 = Array.prototype.slice;

function constant$6(x) {
  return function() {
    return x;
  };
}

function defaultSource$1(d) {
  return d.source;
}

function defaultTarget(d) {
  return d.target;
}

function defaultRadius$1(d) {
  return d.radius;
}

function defaultStartAngle(d) {
  return d.startAngle;
}

function defaultEndAngle(d) {
  return d.endAngle;
}

function defaultPadAngle() {
  return 0;
}

function defaultArrowheadRadius() {
  return 10;
}

function ribbon(headRadius) {
  var source = defaultSource$1,
      target = defaultTarget,
      sourceRadius = defaultRadius$1,
      targetRadius = defaultRadius$1,
      startAngle = defaultStartAngle,
      endAngle = defaultEndAngle,
      padAngle = defaultPadAngle,
      context = null;

  function ribbon() {
    var buffer,
        s = source.apply(this, arguments),
        t = target.apply(this, arguments),
        ap = padAngle.apply(this, arguments) / 2,
        argv = slice$2.call(arguments),
        sr = +sourceRadius.apply(this, (argv[0] = s, argv)),
        sa0 = startAngle.apply(this, argv) - halfPi$2,
        sa1 = endAngle.apply(this, argv) - halfPi$2,
        tr = +targetRadius.apply(this, (argv[0] = t, argv)),
        ta0 = startAngle.apply(this, argv) - halfPi$2,
        ta1 = endAngle.apply(this, argv) - halfPi$2;

    if (!context) context = buffer = path();

    if (ap > epsilon$5) {
      if (abs$2(sa1 - sa0) > ap * 2 + epsilon$5) sa1 > sa0 ? (sa0 += ap, sa1 -= ap) : (sa0 -= ap, sa1 += ap);
      else sa0 = sa1 = (sa0 + sa1) / 2;
      if (abs$2(ta1 - ta0) > ap * 2 + epsilon$5) ta1 > ta0 ? (ta0 += ap, ta1 -= ap) : (ta0 -= ap, ta1 += ap);
      else ta0 = ta1 = (ta0 + ta1) / 2;
    }

    context.moveTo(sr * cos$2(sa0), sr * sin$2(sa0));
    context.arc(0, 0, sr, sa0, sa1);
    if (sa0 !== ta0 || sa1 !== ta1) {
      if (headRadius) {
        var hr = +headRadius.apply(this, arguments), tr2 = tr - hr, ta2 = (ta0 + ta1) / 2;
        context.quadraticCurveTo(0, 0, tr2 * cos$2(ta0), tr2 * sin$2(ta0));
        context.lineTo(tr * cos$2(ta2), tr * sin$2(ta2));
        context.lineTo(tr2 * cos$2(ta1), tr2 * sin$2(ta1));
      } else {
        context.quadraticCurveTo(0, 0, tr * cos$2(ta0), tr * sin$2(ta0));
        context.arc(0, 0, tr, ta0, ta1);
      }
    }
    context.quadraticCurveTo(0, 0, sr * cos$2(sa0), sr * sin$2(sa0));
    context.closePath();

    if (buffer) return context = null, buffer + "" || null;
  }

  if (headRadius) ribbon.headRadius = function(_) {
    return arguments.length ? (headRadius = typeof _ === "function" ? _ : constant$6(+_), ribbon) : headRadius;
  };

  ribbon.radius = function(_) {
    return arguments.length ? (sourceRadius = targetRadius = typeof _ === "function" ? _ : constant$6(+_), ribbon) : sourceRadius;
  };

  ribbon.sourceRadius = function(_) {
    return arguments.length ? (sourceRadius = typeof _ === "function" ? _ : constant$6(+_), ribbon) : sourceRadius;
  };

  ribbon.targetRadius = function(_) {
    return arguments.length ? (targetRadius = typeof _ === "function" ? _ : constant$6(+_), ribbon) : targetRadius;
  };

  ribbon.startAngle = function(_) {
    return arguments.length ? (startAngle = typeof _ === "function" ? _ : constant$6(+_), ribbon) : startAngle;
  };

  ribbon.endAngle = function(_) {
    return arguments.length ? (endAngle = typeof _ === "function" ? _ : constant$6(+_), ribbon) : endAngle;
  };

  ribbon.padAngle = function(_) {
    return arguments.length ? (padAngle = typeof _ === "function" ? _ : constant$6(+_), ribbon) : padAngle;
  };

  ribbon.source = function(_) {
    return arguments.length ? (source = _, ribbon) : source;
  };

  ribbon.target = function(_) {
    return arguments.length ? (target = _, ribbon) : target;
  };

  ribbon.context = function(_) {
    return arguments.length ? ((context = _ == null ? null : _), ribbon) : context;
  };

  return ribbon;
}

function ribbon_default() {
  return ribbon();
}

function ribbonArrow() {
  return ribbon(defaultArrowheadRadius);
}

var array$2 = Array.prototype;

var slice$1 = array$2.slice;

function ascending$1(a, b) {
  return a - b;
}

function area$3(ring) {
  var i = 0, n = ring.length, area = ring[n - 1][1] * ring[0][0] - ring[n - 1][0] * ring[0][1];
  while (++i < n) area += ring[i - 1][1] * ring[i][0] - ring[i - 1][0] * ring[i][1];
  return area;
}

const constant$5 = x => () => x;

function contains$2(ring, hole) {
  var i = -1, n = hole.length, c;
  while (++i < n) if (c = ringContains(ring, hole[i])) return c;
  return 0;
}

function ringContains(ring, point) {
  var x = point[0], y = point[1], contains = -1;
  for (var i = 0, n = ring.length, j = n - 1; i < n; j = i++) {
    var pi = ring[i], xi = pi[0], yi = pi[1], pj = ring[j], xj = pj[0], yj = pj[1];
    if (segmentContains(pi, pj, point)) return 0;
    if (((yi > y) !== (yj > y)) && ((x < (xj - xi) * (y - yi) / (yj - yi) + xi))) contains = -contains;
  }
  return contains;
}

function segmentContains(a, b, c) {
  var i; return collinear$1(a, b, c) && within(a[i = +(a[0] === b[0])], c[i], b[i]);
}

function collinear$1(a, b, c) {
  return (b[0] - a[0]) * (c[1] - a[1]) === (c[0] - a[0]) * (b[1] - a[1]);
}

function within(p, q, r) {
  return p <= q && q <= r || r <= q && q <= p;
}

function noop$2() {}

var cases = [
  [],
  [[[1.0, 1.5], [0.5, 1.0]]],
  [[[1.5, 1.0], [1.0, 1.5]]],
  [[[1.5, 1.0], [0.5, 1.0]]],
  [[[1.0, 0.5], [1.5, 1.0]]],
  [[[1.0, 1.5], [0.5, 1.0]], [[1.0, 0.5], [1.5, 1.0]]],
  [[[1.0, 0.5], [1.0, 1.5]]],
  [[[1.0, 0.5], [0.5, 1.0]]],
  [[[0.5, 1.0], [1.0, 0.5]]],
  [[[1.0, 1.5], [1.0, 0.5]]],
  [[[0.5, 1.0], [1.0, 0.5]], [[1.5, 1.0], [1.0, 1.5]]],
  [[[1.5, 1.0], [1.0, 0.5]]],
  [[[0.5, 1.0], [1.5, 1.0]]],
  [[[1.0, 1.5], [1.5, 1.0]]],
  [[[0.5, 1.0], [1.0, 1.5]]],
  []
];

function Contours() {
  var dx = 1,
      dy = 1,
      threshold = thresholdSturges,
      smooth = smoothLinear;

  function contours(values) {
    var tz = threshold(values);

    // Convert number of thresholds into uniform thresholds.
    if (!Array.isArray(tz)) {
      const e = extent$1(values, finite);
      tz = ticks(...nice$1(e[0], e[1], tz), tz);
      while (tz[tz.length - 1] >= e[1]) tz.pop();
      while (tz[1] < e[0]) tz.shift();
    } else {
      tz = tz.slice().sort(ascending$1);
    }

    return tz.map(value => contour(values, value));
  }

  // Accumulate, smooth contour rings, assign holes to exterior rings.
  // Based on https://github.com/mbostock/shapefile/blob/v0.6.2/shp/polygon.js
  function contour(values, value) {
    const v = value == null ? NaN : +value;
    if (isNaN(v)) throw new Error(`invalid value: ${value}`);

    var polygons = [],
        holes = [];

    isorings(values, v, function(ring) {
      smooth(ring, values, v);
      if (area$3(ring) > 0) polygons.push([ring]);
      else holes.push(ring);
    });

    holes.forEach(function(hole) {
      for (var i = 0, n = polygons.length, polygon; i < n; ++i) {
        if (contains$2((polygon = polygons[i])[0], hole) !== -1) {
          polygon.push(hole);
          return;
        }
      }
    });

    return {
      type: "MultiPolygon",
      value: value,
      coordinates: polygons
    };
  }

  // Marching squares with isolines stitched into rings.
  // Based on https://github.com/topojson/topojson-client/blob/v3.0.0/src/stitch.js
  function isorings(values, value, callback) {
    var fragmentByStart = new Array,
        fragmentByEnd = new Array,
        x, y, t0, t1, t2, t3;

    // Special case for the first row (y = -1, t2 = t3 = 0).
    x = y = -1;
    t1 = above(values[0], value);
    cases[t1 << 1].forEach(stitch);
    while (++x < dx - 1) {
      t0 = t1, t1 = above(values[x + 1], value);
      cases[t0 | t1 << 1].forEach(stitch);
    }
    cases[t1 << 0].forEach(stitch);

    // General case for the intermediate rows.
    while (++y < dy - 1) {
      x = -1;
      t1 = above(values[y * dx + dx], value);
      t2 = above(values[y * dx], value);
      cases[t1 << 1 | t2 << 2].forEach(stitch);
      while (++x < dx - 1) {
        t0 = t1, t1 = above(values[y * dx + dx + x + 1], value);
        t3 = t2, t2 = above(values[y * dx + x + 1], value);
        cases[t0 | t1 << 1 | t2 << 2 | t3 << 3].forEach(stitch);
      }
      cases[t1 | t2 << 3].forEach(stitch);
    }

    // Special case for the last row (y = dy - 1, t0 = t1 = 0).
    x = -1;
    t2 = values[y * dx] >= value;
    cases[t2 << 2].forEach(stitch);
    while (++x < dx - 1) {
      t3 = t2, t2 = above(values[y * dx + x + 1], value);
      cases[t2 << 2 | t3 << 3].forEach(stitch);
    }
    cases[t2 << 3].forEach(stitch);

    function stitch(line) {
      var start = [line[0][0] + x, line[0][1] + y],
          end = [line[1][0] + x, line[1][1] + y],
          startIndex = index(start),
          endIndex = index(end),
          f, g;
      if (f = fragmentByEnd[startIndex]) {
        if (g = fragmentByStart[endIndex]) {
          delete fragmentByEnd[f.end];
          delete fragmentByStart[g.start];
          if (f === g) {
            f.ring.push(end);
            callback(f.ring);
          } else {
            fragmentByStart[f.start] = fragmentByEnd[g.end] = {start: f.start, end: g.end, ring: f.ring.concat(g.ring)};
          }
        } else {
          delete fragmentByEnd[f.end];
          f.ring.push(end);
          fragmentByEnd[f.end = endIndex] = f;
        }
      } else if (f = fragmentByStart[endIndex]) {
        if (g = fragmentByEnd[startIndex]) {
          delete fragmentByStart[f.start];
          delete fragmentByEnd[g.end];
          if (f === g) {
            f.ring.push(end);
            callback(f.ring);
          } else {
            fragmentByStart[g.start] = fragmentByEnd[f.end] = {start: g.start, end: f.end, ring: g.ring.concat(f.ring)};
          }
        } else {
          delete fragmentByStart[f.start];
          f.ring.unshift(start);
          fragmentByStart[f.start = startIndex] = f;
        }
      } else {
        fragmentByStart[startIndex] = fragmentByEnd[endIndex] = {start: startIndex, end: endIndex, ring: [start, end]};
      }
    }
  }

  function index(point) {
    return point[0] * 2 + point[1] * (dx + 1) * 4;
  }

  function smoothLinear(ring, values, value) {
    ring.forEach(function(point) {
      var x = point[0],
          y = point[1],
          xt = x | 0,
          yt = y | 0,
          v1 = valid(values[yt * dx + xt]);
      if (x > 0 && x < dx && xt === x) {
        point[0] = smooth1(x, valid(values[yt * dx + xt - 1]), v1, value);
      }
      if (y > 0 && y < dy && yt === y) {
        point[1] = smooth1(y, valid(values[(yt - 1) * dx + xt]), v1, value);
      }
    });
  }

  contours.contour = contour;

  contours.size = function(_) {
    if (!arguments.length) return [dx, dy];
    var _0 = Math.floor(_[0]), _1 = Math.floor(_[1]);
    if (!(_0 >= 0 && _1 >= 0)) throw new Error("invalid size");
    return dx = _0, dy = _1, contours;
  };

  contours.thresholds = function(_) {
    return arguments.length ? (threshold = typeof _ === "function" ? _ : Array.isArray(_) ? constant$5(slice$1.call(_)) : constant$5(_), contours) : threshold;
  };

  contours.smooth = function(_) {
    return arguments.length ? (smooth = _ ? smoothLinear : noop$2, contours) : smooth === smoothLinear;
  };

  return contours;
}

// When computing the extent, ignore infinite values (as well as invalid ones).
function finite(x) {
  return isFinite(x) ? x : NaN;
}

// Is the (possibly invalid) x greater than or equal to the (known valid) value?
// Treat any invalid value as below negative infinity.
function above(x, value) {
  return x == null ? false : +x >= value;
}

// During smoothing, treat any invalid value as negative infinity.
function valid(v) {
  return v == null || isNaN(v = +v) ? -Infinity : v;
}

function smooth1(x, v0, v1, value) {
  const a = value - v0;
  const b = v1 - v0;
  const d = isFinite(a) || isFinite(b) ? a / b : Math.sign(a) / Math.sign(b);
  return isNaN(d) ? x : x + d - 0.5;
}

function defaultX$1(d) {
  return d[0];
}

function defaultY$1(d) {
  return d[1];
}

function defaultWeight() {
  return 1;
}

function density() {
  var x = defaultX$1,
      y = defaultY$1,
      weight = defaultWeight,
      dx = 960,
      dy = 500,
      r = 20, // blur radius
      k = 2, // log2(grid cell size)
      o = r * 3, // grid offset, to pad for blur
      n = (dx + o * 2) >> k, // grid width
      m = (dy + o * 2) >> k, // grid height
      threshold = constant$5(20);

  function grid(data) {
    var values = new Float32Array(n * m),
        pow2k = Math.pow(2, -k),
        i = -1;

    for (const d of data) {
      var xi = (x(d, ++i, data) + o) * pow2k,
          yi = (y(d, i, data) + o) * pow2k,
          wi = +weight(d, i, data);
      if (wi && xi >= 0 && xi < n && yi >= 0 && yi < m) {
        var x0 = Math.floor(xi),
            y0 = Math.floor(yi),
            xt = xi - x0 - 0.5,
            yt = yi - y0 - 0.5;
        values[x0 + y0 * n] += (1 - xt) * (1 - yt) * wi;
        values[x0 + 1 + y0 * n] += xt * (1 - yt) * wi;
        values[x0 + 1 + (y0 + 1) * n] += xt * yt * wi;
        values[x0 + (y0 + 1) * n] += (1 - xt) * yt * wi;
      }
    }

    blur2({data: values, width: n, height: m}, r * pow2k);
    return values;
  }

  function density(data) {
    var values = grid(data),
        tz = threshold(values),
        pow4k = Math.pow(2, 2 * k);

    // Convert number of thresholds into uniform thresholds.
    if (!Array.isArray(tz)) {
      tz = ticks(Number.MIN_VALUE, max$3(values) / pow4k, tz);
    }

    return Contours()
        .size([n, m])
        .thresholds(tz.map(d => d * pow4k))
      (values)
        .map((c, i) => (c.value = +tz[i], transform(c)));
  }

  density.contours = function(data) {
    var values = grid(data),
        contours = Contours().size([n, m]),
        pow4k = Math.pow(2, 2 * k),
        contour = value => {
          value = +value;
          var c = transform(contours.contour(values, value * pow4k));
          c.value = value; // preserve exact threshold value
          return c;
        };
    Object.defineProperty(contour, "max", {get: () => max$3(values) / pow4k});
    return contour;
  };

  function transform(geometry) {
    geometry.coordinates.forEach(transformPolygon);
    return geometry;
  }

  function transformPolygon(coordinates) {
    coordinates.forEach(transformRing);
  }

  function transformRing(coordinates) {
    coordinates.forEach(transformPoint);
  }

  // TODO Optimize.
  function transformPoint(coordinates) {
    coordinates[0] = coordinates[0] * Math.pow(2, k) - o;
    coordinates[1] = coordinates[1] * Math.pow(2, k) - o;
  }

  function resize() {
    o = r * 3;
    n = (dx + o * 2) >> k;
    m = (dy + o * 2) >> k;
    return density;
  }

  density.x = function(_) {
    return arguments.length ? (x = typeof _ === "function" ? _ : constant$5(+_), density) : x;
  };

  density.y = function(_) {
    return arguments.length ? (y = typeof _ === "function" ? _ : constant$5(+_), density) : y;
  };

  density.weight = function(_) {
    return arguments.length ? (weight = typeof _ === "function" ? _ : constant$5(+_), density) : weight;
  };

  density.size = function(_) {
    if (!arguments.length) return [dx, dy];
    var _0 = +_[0], _1 = +_[1];
    if (!(_0 >= 0 && _1 >= 0)) throw new Error("invalid size");
    return dx = _0, dy = _1, resize();
  };

  density.cellSize = function(_) {
    if (!arguments.length) return 1 << k;
    if (!((_ = +_) >= 1)) throw new Error("invalid cell size");
    return k = Math.floor(Math.log(_) / Math.LN2), resize();
  };

  density.thresholds = function(_) {
    return arguments.length ? (threshold = typeof _ === "function" ? _ : Array.isArray(_) ? constant$5(slice$1.call(_)) : constant$5(_), density) : threshold;
  };

  density.bandwidth = function(_) {
    if (!arguments.length) return Math.sqrt(r * (r + 1));
    if (!((_ = +_) >= 0)) throw new Error("invalid bandwidth");
    return r = (Math.sqrt(4 * _ * _ + 1) - 1) / 2, resize();
  };

  return density;
}

const epsilon$3 = 1.1102230246251565e-16;
const splitter = 134217729;
const resulterrbound = (3 + 8 * epsilon$3) * epsilon$3;

// fast_expansion_sum_zeroelim routine from oritinal code
function sum$1(elen, e, flen, f, h) {
    let Q, Qnew, hh, bvirt;
    let enow = e[0];
    let fnow = f[0];
    let eindex = 0;
    let findex = 0;
    if ((fnow > enow) === (fnow > -enow)) {
        Q = enow;
        enow = e[++eindex];
    } else {
        Q = fnow;
        fnow = f[++findex];
    }
    let hindex = 0;
    if (eindex < elen && findex < flen) {
        if ((fnow > enow) === (fnow > -enow)) {
            Qnew = enow + Q;
            hh = Q - (Qnew - enow);
            enow = e[++eindex];
        } else {
            Qnew = fnow + Q;
            hh = Q - (Qnew - fnow);
            fnow = f[++findex];
        }
        Q = Qnew;
        if (hh !== 0) {
            h[hindex++] = hh;
        }
        while (eindex < elen && findex < flen) {
            if ((fnow > enow) === (fnow > -enow)) {
                Qnew = Q + enow;
                bvirt = Qnew - Q;
                hh = Q - (Qnew - bvirt) + (enow - bvirt);
                enow = e[++eindex];
            } else {
                Qnew = Q + fnow;
                bvirt = Qnew - Q;
                hh = Q - (Qnew - bvirt) + (fnow - bvirt);
                fnow = f[++findex];
            }
            Q = Qnew;
            if (hh !== 0) {
                h[hindex++] = hh;
            }
        }
    }
    while (eindex < elen) {
        Qnew = Q + enow;
        bvirt = Qnew - Q;
        hh = Q - (Qnew - bvirt) + (enow - bvirt);
        enow = e[++eindex];
        Q = Qnew;
        if (hh !== 0) {
            h[hindex++] = hh;
        }
    }
    while (findex < flen) {
        Qnew = Q + fnow;
        bvirt = Qnew - Q;
        hh = Q - (Qnew - bvirt) + (fnow - bvirt);
        fnow = f[++findex];
        Q = Qnew;
        if (hh !== 0) {
            h[hindex++] = hh;
        }
    }
    if (Q !== 0 || hindex === 0) {
        h[hindex++] = Q;
    }
    return hindex;
}

function sum_three(alen, a, blen, b, clen, c, tmp, out) {
    return sum$1(sum$1(alen, a, blen, b, tmp), tmp, clen, c, out);
}

// scale_expansion_zeroelim routine from oritinal code
function scale(elen, e, b, h) {
    let Q, sum, hh, product1, product0;
    let bvirt, c, ahi, alo, bhi, blo;

    c = splitter * b;
    bhi = c - (c - b);
    blo = b - bhi;
    let enow = e[0];
    Q = enow * b;
    c = splitter * enow;
    ahi = c - (c - enow);
    alo = enow - ahi;
    hh = alo * blo - (Q - ahi * bhi - alo * bhi - ahi * blo);
    let hindex = 0;
    if (hh !== 0) {
        h[hindex++] = hh;
    }
    for (let i = 1; i < elen; i++) {
        enow = e[i];
        product1 = enow * b;
        c = splitter * enow;
        ahi = c - (c - enow);
        alo = enow - ahi;
        product0 = alo * blo - (product1 - ahi * bhi - alo * bhi - ahi * blo);
        sum = Q + product0;
        bvirt = sum - Q;
        hh = Q - (sum - bvirt) + (product0 - bvirt);
        if (hh !== 0) {
            h[hindex++] = hh;
        }
        Q = product1 + sum;
        hh = sum - (Q - product1);
        if (hh !== 0) {
            h[hindex++] = hh;
        }
    }
    if (Q !== 0 || hindex === 0) {
        h[hindex++] = Q;
    }
    return hindex;
}

function negate(elen, e) {
    for (let i = 0; i < elen; i++) e[i] = -e[i];
    return elen;
}

function estimate(elen, e) {
    let Q = e[0];
    for (let i = 1; i < elen; i++) Q += e[i];
    return Q;
}

function vec(n) {
    return new Float64Array(n);
}

const ccwerrboundA = (3 + 16 * epsilon$3) * epsilon$3;
const ccwerrboundB = (2 + 12 * epsilon$3) * epsilon$3;
const ccwerrboundC = (9 + 64 * epsilon$3) * epsilon$3 * epsilon$3;

const B = vec(4);
const C1 = vec(8);
const C2 = vec(12);
const D = vec(16);
const u$2 = vec(4);

function orient2dadapt(ax, ay, bx, by, cx, cy, detsum) {
    let acxtail, acytail, bcxtail, bcytail;
    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _0, s1, s0, t1, t0, u3;

    const acx = ax - cx;
    const bcx = bx - cx;
    const acy = ay - cy;
    const bcy = by - cy;

    s1 = acx * bcy;
    c = splitter * acx;
    ahi = c - (c - acx);
    alo = acx - ahi;
    c = splitter * bcy;
    bhi = c - (c - bcy);
    blo = bcy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = acy * bcx;
    c = splitter * acy;
    ahi = c - (c - acy);
    alo = acy - ahi;
    c = splitter * bcx;
    bhi = c - (c - bcx);
    blo = bcx - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    B[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    B[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    B[2] = _j - (u3 - bvirt) + (_i - bvirt);
    B[3] = u3;

    let det = estimate(4, B);
    let errbound = ccwerrboundB * detsum;
    if (det >= errbound || -det >= errbound) {
        return det;
    }

    bvirt = ax - acx;
    acxtail = ax - (acx + bvirt) + (bvirt - cx);
    bvirt = bx - bcx;
    bcxtail = bx - (bcx + bvirt) + (bvirt - cx);
    bvirt = ay - acy;
    acytail = ay - (acy + bvirt) + (bvirt - cy);
    bvirt = by - bcy;
    bcytail = by - (bcy + bvirt) + (bvirt - cy);

    if (acxtail === 0 && acytail === 0 && bcxtail === 0 && bcytail === 0) {
        return det;
    }

    errbound = ccwerrboundC * detsum + resulterrbound * Math.abs(det);
    det += (acx * bcytail + bcy * acxtail) - (acy * bcxtail + bcx * acytail);
    if (det >= errbound || -det >= errbound) return det;

    s1 = acxtail * bcy;
    c = splitter * acxtail;
    ahi = c - (c - acxtail);
    alo = acxtail - ahi;
    c = splitter * bcy;
    bhi = c - (c - bcy);
    blo = bcy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = acytail * bcx;
    c = splitter * acytail;
    ahi = c - (c - acytail);
    alo = acytail - ahi;
    c = splitter * bcx;
    bhi = c - (c - bcx);
    blo = bcx - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    u$2[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    u$2[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    u$2[2] = _j - (u3 - bvirt) + (_i - bvirt);
    u$2[3] = u3;
    const C1len = sum$1(4, B, 4, u$2, C1);

    s1 = acx * bcytail;
    c = splitter * acx;
    ahi = c - (c - acx);
    alo = acx - ahi;
    c = splitter * bcytail;
    bhi = c - (c - bcytail);
    blo = bcytail - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = acy * bcxtail;
    c = splitter * acy;
    ahi = c - (c - acy);
    alo = acy - ahi;
    c = splitter * bcxtail;
    bhi = c - (c - bcxtail);
    blo = bcxtail - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    u$2[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    u$2[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    u$2[2] = _j - (u3 - bvirt) + (_i - bvirt);
    u$2[3] = u3;
    const C2len = sum$1(C1len, C1, 4, u$2, C2);

    s1 = acxtail * bcytail;
    c = splitter * acxtail;
    ahi = c - (c - acxtail);
    alo = acxtail - ahi;
    c = splitter * bcytail;
    bhi = c - (c - bcytail);
    blo = bcytail - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = acytail * bcxtail;
    c = splitter * acytail;
    ahi = c - (c - acytail);
    alo = acytail - ahi;
    c = splitter * bcxtail;
    bhi = c - (c - bcxtail);
    blo = bcxtail - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    u$2[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    u$2[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    u$2[2] = _j - (u3 - bvirt) + (_i - bvirt);
    u$2[3] = u3;
    const Dlen = sum$1(C2len, C2, 4, u$2, D);

    return D[Dlen - 1];
}

function orient2d(ax, ay, bx, by, cx, cy) {
    const detleft = (ay - cy) * (bx - cx);
    const detright = (ax - cx) * (by - cy);
    const det = detleft - detright;

    const detsum = Math.abs(detleft + detright);
    if (Math.abs(det) >= ccwerrboundA * detsum) return det;

    return -orient2dadapt(ax, ay, bx, by, cx, cy, detsum);
}

function orient2dfast(ax, ay, bx, by, cx, cy) {
    return (ay - cy) * (bx - cx) - (ax - cx) * (by - cy);
}

const o3derrboundA = (7 + 56 * epsilon$3) * epsilon$3;
const o3derrboundB = (3 + 28 * epsilon$3) * epsilon$3;
const o3derrboundC = (26 + 288 * epsilon$3) * epsilon$3 * epsilon$3;

const bc$2 = vec(4);
const ca$1 = vec(4);
const ab$2 = vec(4);
const at_b = vec(4);
const at_c = vec(4);
const bt_c = vec(4);
const bt_a = vec(4);
const ct_a = vec(4);
const ct_b = vec(4);
const bct$1 = vec(8);
const cat$1 = vec(8);
const abt$1 = vec(8);
const u$1 = vec(4);

const _8$2 = vec(8);
const _8b$1 = vec(8);
const _16$2 = vec(8);
const _12 = vec(12);

let fin$2 = vec(192);
let fin2$1 = vec(192);

function finadd$1(finlen, alen, a) {
    finlen = sum$1(finlen, fin$2, alen, a, fin2$1);
    const tmp = fin$2; fin$2 = fin2$1; fin2$1 = tmp;
    return finlen;
}

function tailinit(xtail, ytail, ax, ay, bx, by, a, b) {
    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _k, _0, s1, s0, t1, t0, u3, negate;
    if (xtail === 0) {
        if (ytail === 0) {
            a[0] = 0;
            b[0] = 0;
            return 1;
        } else {
            negate = -ytail;
            s1 = negate * ax;
            c = splitter * negate;
            ahi = c - (c - negate);
            alo = negate - ahi;
            c = splitter * ax;
            bhi = c - (c - ax);
            blo = ax - bhi;
            a[0] = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            a[1] = s1;
            s1 = ytail * bx;
            c = splitter * ytail;
            ahi = c - (c - ytail);
            alo = ytail - ahi;
            c = splitter * bx;
            bhi = c - (c - bx);
            blo = bx - bhi;
            b[0] = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            b[1] = s1;
            return 2;
        }
    } else {
        if (ytail === 0) {
            s1 = xtail * ay;
            c = splitter * xtail;
            ahi = c - (c - xtail);
            alo = xtail - ahi;
            c = splitter * ay;
            bhi = c - (c - ay);
            blo = ay - bhi;
            a[0] = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            a[1] = s1;
            negate = -xtail;
            s1 = negate * by;
            c = splitter * negate;
            ahi = c - (c - negate);
            alo = negate - ahi;
            c = splitter * by;
            bhi = c - (c - by);
            blo = by - bhi;
            b[0] = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            b[1] = s1;
            return 2;
        } else {
            s1 = xtail * ay;
            c = splitter * xtail;
            ahi = c - (c - xtail);
            alo = xtail - ahi;
            c = splitter * ay;
            bhi = c - (c - ay);
            blo = ay - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = ytail * ax;
            c = splitter * ytail;
            ahi = c - (c - ytail);
            alo = ytail - ahi;
            c = splitter * ax;
            bhi = c - (c - ax);
            blo = ax - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 - t0;
            bvirt = s0 - _i;
            a[0] = s0 - (_i + bvirt) + (bvirt - t0);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 - t1;
            bvirt = _0 - _i;
            a[1] = _0 - (_i + bvirt) + (bvirt - t1);
            u3 = _j + _i;
            bvirt = u3 - _j;
            a[2] = _j - (u3 - bvirt) + (_i - bvirt);
            a[3] = u3;
            s1 = ytail * bx;
            c = splitter * ytail;
            ahi = c - (c - ytail);
            alo = ytail - ahi;
            c = splitter * bx;
            bhi = c - (c - bx);
            blo = bx - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = xtail * by;
            c = splitter * xtail;
            ahi = c - (c - xtail);
            alo = xtail - ahi;
            c = splitter * by;
            bhi = c - (c - by);
            blo = by - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 - t0;
            bvirt = s0 - _i;
            b[0] = s0 - (_i + bvirt) + (bvirt - t0);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 - t1;
            bvirt = _0 - _i;
            b[1] = _0 - (_i + bvirt) + (bvirt - t1);
            u3 = _j + _i;
            bvirt = u3 - _j;
            b[2] = _j - (u3 - bvirt) + (_i - bvirt);
            b[3] = u3;
            return 4;
        }
    }
}

function tailadd(finlen, a, b, k, z) {
    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _k, _0, s1, s0, u3;
    s1 = a * b;
    c = splitter * a;
    ahi = c - (c - a);
    alo = a - ahi;
    c = splitter * b;
    bhi = c - (c - b);
    blo = b - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    c = splitter * k;
    bhi = c - (c - k);
    blo = k - bhi;
    _i = s0 * k;
    c = splitter * s0;
    ahi = c - (c - s0);
    alo = s0 - ahi;
    u$1[0] = alo * blo - (_i - ahi * bhi - alo * bhi - ahi * blo);
    _j = s1 * k;
    c = splitter * s1;
    ahi = c - (c - s1);
    alo = s1 - ahi;
    _0 = alo * blo - (_j - ahi * bhi - alo * bhi - ahi * blo);
    _k = _i + _0;
    bvirt = _k - _i;
    u$1[1] = _i - (_k - bvirt) + (_0 - bvirt);
    u3 = _j + _k;
    u$1[2] = _k - (u3 - _j);
    u$1[3] = u3;
    finlen = finadd$1(finlen, 4, u$1);
    if (z !== 0) {
        c = splitter * z;
        bhi = c - (c - z);
        blo = z - bhi;
        _i = s0 * z;
        c = splitter * s0;
        ahi = c - (c - s0);
        alo = s0 - ahi;
        u$1[0] = alo * blo - (_i - ahi * bhi - alo * bhi - ahi * blo);
        _j = s1 * z;
        c = splitter * s1;
        ahi = c - (c - s1);
        alo = s1 - ahi;
        _0 = alo * blo - (_j - ahi * bhi - alo * bhi - ahi * blo);
        _k = _i + _0;
        bvirt = _k - _i;
        u$1[1] = _i - (_k - bvirt) + (_0 - bvirt);
        u3 = _j + _k;
        u$1[2] = _k - (u3 - _j);
        u$1[3] = u3;
        finlen = finadd$1(finlen, 4, u$1);
    }
    return finlen;
}

function orient3dadapt(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, permanent) {
    let finlen;
    let adxtail, bdxtail, cdxtail;
    let adytail, bdytail, cdytail;
    let adztail, bdztail, cdztail;
    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _k, _0, s1, s0, t1, t0, u3;

    const adx = ax - dx;
    const bdx = bx - dx;
    const cdx = cx - dx;
    const ady = ay - dy;
    const bdy = by - dy;
    const cdy = cy - dy;
    const adz = az - dz;
    const bdz = bz - dz;
    const cdz = cz - dz;

    s1 = bdx * cdy;
    c = splitter * bdx;
    ahi = c - (c - bdx);
    alo = bdx - ahi;
    c = splitter * cdy;
    bhi = c - (c - cdy);
    blo = cdy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cdx * bdy;
    c = splitter * cdx;
    ahi = c - (c - cdx);
    alo = cdx - ahi;
    c = splitter * bdy;
    bhi = c - (c - bdy);
    blo = bdy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bc$2[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bc$2[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    bc$2[2] = _j - (u3 - bvirt) + (_i - bvirt);
    bc$2[3] = u3;
    s1 = cdx * ady;
    c = splitter * cdx;
    ahi = c - (c - cdx);
    alo = cdx - ahi;
    c = splitter * ady;
    bhi = c - (c - ady);
    blo = ady - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = adx * cdy;
    c = splitter * adx;
    ahi = c - (c - adx);
    alo = adx - ahi;
    c = splitter * cdy;
    bhi = c - (c - cdy);
    blo = cdy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ca$1[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ca$1[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ca$1[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ca$1[3] = u3;
    s1 = adx * bdy;
    c = splitter * adx;
    ahi = c - (c - adx);
    alo = adx - ahi;
    c = splitter * bdy;
    bhi = c - (c - bdy);
    blo = bdy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = bdx * ady;
    c = splitter * bdx;
    ahi = c - (c - bdx);
    alo = bdx - ahi;
    c = splitter * ady;
    bhi = c - (c - ady);
    blo = ady - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ab$2[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ab$2[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ab$2[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ab$2[3] = u3;

    finlen = sum$1(
        sum$1(
            scale(4, bc$2, adz, _8$2), _8$2,
            scale(4, ca$1, bdz, _8b$1), _8b$1, _16$2), _16$2,
        scale(4, ab$2, cdz, _8$2), _8$2, fin$2);

    let det = estimate(finlen, fin$2);
    let errbound = o3derrboundB * permanent;
    if (det >= errbound || -det >= errbound) {
        return det;
    }

    bvirt = ax - adx;
    adxtail = ax - (adx + bvirt) + (bvirt - dx);
    bvirt = bx - bdx;
    bdxtail = bx - (bdx + bvirt) + (bvirt - dx);
    bvirt = cx - cdx;
    cdxtail = cx - (cdx + bvirt) + (bvirt - dx);
    bvirt = ay - ady;
    adytail = ay - (ady + bvirt) + (bvirt - dy);
    bvirt = by - bdy;
    bdytail = by - (bdy + bvirt) + (bvirt - dy);
    bvirt = cy - cdy;
    cdytail = cy - (cdy + bvirt) + (bvirt - dy);
    bvirt = az - adz;
    adztail = az - (adz + bvirt) + (bvirt - dz);
    bvirt = bz - bdz;
    bdztail = bz - (bdz + bvirt) + (bvirt - dz);
    bvirt = cz - cdz;
    cdztail = cz - (cdz + bvirt) + (bvirt - dz);

    if (adxtail === 0 && bdxtail === 0 && cdxtail === 0 &&
        adytail === 0 && bdytail === 0 && cdytail === 0 &&
        adztail === 0 && bdztail === 0 && cdztail === 0) {
        return det;
    }

    errbound = o3derrboundC * permanent + resulterrbound * Math.abs(det);
    det +=
        adz * (bdx * cdytail + cdy * bdxtail - (bdy * cdxtail + cdx * bdytail)) + adztail * (bdx * cdy - bdy * cdx) +
        bdz * (cdx * adytail + ady * cdxtail - (cdy * adxtail + adx * cdytail)) + bdztail * (cdx * ady - cdy * adx) +
        cdz * (adx * bdytail + bdy * adxtail - (ady * bdxtail + bdx * adytail)) + cdztail * (adx * bdy - ady * bdx);
    if (det >= errbound || -det >= errbound) {
        return det;
    }

    const at_len = tailinit(adxtail, adytail, bdx, bdy, cdx, cdy, at_b, at_c);
    const bt_len = tailinit(bdxtail, bdytail, cdx, cdy, adx, ady, bt_c, bt_a);
    const ct_len = tailinit(cdxtail, cdytail, adx, ady, bdx, bdy, ct_a, ct_b);

    const bctlen = sum$1(bt_len, bt_c, ct_len, ct_b, bct$1);
    finlen = finadd$1(finlen, scale(bctlen, bct$1, adz, _16$2), _16$2);

    const catlen = sum$1(ct_len, ct_a, at_len, at_c, cat$1);
    finlen = finadd$1(finlen, scale(catlen, cat$1, bdz, _16$2), _16$2);

    const abtlen = sum$1(at_len, at_b, bt_len, bt_a, abt$1);
    finlen = finadd$1(finlen, scale(abtlen, abt$1, cdz, _16$2), _16$2);

    if (adztail !== 0) {
        finlen = finadd$1(finlen, scale(4, bc$2, adztail, _12), _12);
        finlen = finadd$1(finlen, scale(bctlen, bct$1, adztail, _16$2), _16$2);
    }
    if (bdztail !== 0) {
        finlen = finadd$1(finlen, scale(4, ca$1, bdztail, _12), _12);
        finlen = finadd$1(finlen, scale(catlen, cat$1, bdztail, _16$2), _16$2);
    }
    if (cdztail !== 0) {
        finlen = finadd$1(finlen, scale(4, ab$2, cdztail, _12), _12);
        finlen = finadd$1(finlen, scale(abtlen, abt$1, cdztail, _16$2), _16$2);
    }

    if (adxtail !== 0) {
        if (bdytail !== 0) {
            finlen = tailadd(finlen, adxtail, bdytail, cdz, cdztail);
        }
        if (cdytail !== 0) {
            finlen = tailadd(finlen, -adxtail, cdytail, bdz, bdztail);
        }
    }
    if (bdxtail !== 0) {
        if (cdytail !== 0) {
            finlen = tailadd(finlen, bdxtail, cdytail, adz, adztail);
        }
        if (adytail !== 0) {
            finlen = tailadd(finlen, -bdxtail, adytail, cdz, cdztail);
        }
    }
    if (cdxtail !== 0) {
        if (adytail !== 0) {
            finlen = tailadd(finlen, cdxtail, adytail, bdz, bdztail);
        }
        if (bdytail !== 0) {
            finlen = tailadd(finlen, -cdxtail, bdytail, adz, adztail);
        }
    }

    return fin$2[finlen - 1];
}

function orient3d(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz) {
    const adx = ax - dx;
    const bdx = bx - dx;
    const cdx = cx - dx;
    const ady = ay - dy;
    const bdy = by - dy;
    const cdy = cy - dy;
    const adz = az - dz;
    const bdz = bz - dz;
    const cdz = cz - dz;

    const bdxcdy = bdx * cdy;
    const cdxbdy = cdx * bdy;

    const cdxady = cdx * ady;
    const adxcdy = adx * cdy;

    const adxbdy = adx * bdy;
    const bdxady = bdx * ady;

    const det =
        adz * (bdxcdy - cdxbdy) +
        bdz * (cdxady - adxcdy) +
        cdz * (adxbdy - bdxady);

    const permanent =
        (Math.abs(bdxcdy) + Math.abs(cdxbdy)) * Math.abs(adz) +
        (Math.abs(cdxady) + Math.abs(adxcdy)) * Math.abs(bdz) +
        (Math.abs(adxbdy) + Math.abs(bdxady)) * Math.abs(cdz);

    const errbound = o3derrboundA * permanent;
    if (det > errbound || -det > errbound) {
        return det;
    }

    return orient3dadapt(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, permanent);
}

function orient3dfast(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz) {
    const adx = ax - dx;
    const bdx = bx - dx;
    const cdx = cx - dx;
    const ady = ay - dy;
    const bdy = by - dy;
    const cdy = cy - dy;
    const adz = az - dz;
    const bdz = bz - dz;
    const cdz = cz - dz;

    return adx * (bdy * cdz - bdz * cdy) +
        bdx * (cdy * adz - cdz * ady) +
        cdx * (ady * bdz - adz * bdy);
}

const iccerrboundA = (10 + 96 * epsilon$3) * epsilon$3;
const iccerrboundB = (4 + 48 * epsilon$3) * epsilon$3;
const iccerrboundC = (44 + 576 * epsilon$3) * epsilon$3 * epsilon$3;

const bc$1 = vec(4);
const ca = vec(4);
const ab$1 = vec(4);
const aa = vec(4);
const bb = vec(4);
const cc = vec(4);
const u = vec(4);
const v = vec(4);
const axtbc = vec(8);
const aytbc = vec(8);
const bxtca = vec(8);
const bytca = vec(8);
const cxtab = vec(8);
const cytab = vec(8);
const abt = vec(8);
const bct = vec(8);
const cat = vec(8);
const abtt = vec(4);
const bctt = vec(4);
const catt = vec(4);

const _8$1 = vec(8);
const _16$1 = vec(16);
const _16b = vec(16);
const _16c = vec(16);
const _32 = vec(32);
const _32b = vec(32);
const _48$1 = vec(48);
const _64 = vec(64);

let fin$1 = vec(1152);
let fin2 = vec(1152);

function finadd(finlen, a, alen) {
    finlen = sum$1(finlen, fin$1, a, alen, fin2);
    const tmp = fin$1; fin$1 = fin2; fin2 = tmp;
    return finlen;
}

function incircleadapt(ax, ay, bx, by, cx, cy, dx, dy, permanent) {
    let finlen;
    let adxtail, bdxtail, cdxtail, adytail, bdytail, cdytail;
    let axtbclen, aytbclen, bxtcalen, bytcalen, cxtablen, cytablen;
    let abtlen, bctlen, catlen;
    let abttlen, bcttlen, cattlen;
    let n1, n0;

    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _0, s1, s0, t1, t0, u3;

    const adx = ax - dx;
    const bdx = bx - dx;
    const cdx = cx - dx;
    const ady = ay - dy;
    const bdy = by - dy;
    const cdy = cy - dy;

    s1 = bdx * cdy;
    c = splitter * bdx;
    ahi = c - (c - bdx);
    alo = bdx - ahi;
    c = splitter * cdy;
    bhi = c - (c - cdy);
    blo = cdy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cdx * bdy;
    c = splitter * cdx;
    ahi = c - (c - cdx);
    alo = cdx - ahi;
    c = splitter * bdy;
    bhi = c - (c - bdy);
    blo = bdy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bc$1[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bc$1[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    bc$1[2] = _j - (u3 - bvirt) + (_i - bvirt);
    bc$1[3] = u3;
    s1 = cdx * ady;
    c = splitter * cdx;
    ahi = c - (c - cdx);
    alo = cdx - ahi;
    c = splitter * ady;
    bhi = c - (c - ady);
    blo = ady - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = adx * cdy;
    c = splitter * adx;
    ahi = c - (c - adx);
    alo = adx - ahi;
    c = splitter * cdy;
    bhi = c - (c - cdy);
    blo = cdy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ca[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ca[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ca[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ca[3] = u3;
    s1 = adx * bdy;
    c = splitter * adx;
    ahi = c - (c - adx);
    alo = adx - ahi;
    c = splitter * bdy;
    bhi = c - (c - bdy);
    blo = bdy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = bdx * ady;
    c = splitter * bdx;
    ahi = c - (c - bdx);
    alo = bdx - ahi;
    c = splitter * ady;
    bhi = c - (c - ady);
    blo = ady - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ab$1[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ab$1[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ab$1[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ab$1[3] = u3;

    finlen = sum$1(
        sum$1(
            sum$1(
                scale(scale(4, bc$1, adx, _8$1), _8$1, adx, _16$1), _16$1,
                scale(scale(4, bc$1, ady, _8$1), _8$1, ady, _16b), _16b, _32), _32,
            sum$1(
                scale(scale(4, ca, bdx, _8$1), _8$1, bdx, _16$1), _16$1,
                scale(scale(4, ca, bdy, _8$1), _8$1, bdy, _16b), _16b, _32b), _32b, _64), _64,
        sum$1(
            scale(scale(4, ab$1, cdx, _8$1), _8$1, cdx, _16$1), _16$1,
            scale(scale(4, ab$1, cdy, _8$1), _8$1, cdy, _16b), _16b, _32), _32, fin$1);

    let det = estimate(finlen, fin$1);
    let errbound = iccerrboundB * permanent;
    if (det >= errbound || -det >= errbound) {
        return det;
    }

    bvirt = ax - adx;
    adxtail = ax - (adx + bvirt) + (bvirt - dx);
    bvirt = ay - ady;
    adytail = ay - (ady + bvirt) + (bvirt - dy);
    bvirt = bx - bdx;
    bdxtail = bx - (bdx + bvirt) + (bvirt - dx);
    bvirt = by - bdy;
    bdytail = by - (bdy + bvirt) + (bvirt - dy);
    bvirt = cx - cdx;
    cdxtail = cx - (cdx + bvirt) + (bvirt - dx);
    bvirt = cy - cdy;
    cdytail = cy - (cdy + bvirt) + (bvirt - dy);
    if (adxtail === 0 && bdxtail === 0 && cdxtail === 0 && adytail === 0 && bdytail === 0 && cdytail === 0) {
        return det;
    }

    errbound = iccerrboundC * permanent + resulterrbound * Math.abs(det);
    det += ((adx * adx + ady * ady) * ((bdx * cdytail + cdy * bdxtail) - (bdy * cdxtail + cdx * bdytail)) +
        2 * (adx * adxtail + ady * adytail) * (bdx * cdy - bdy * cdx)) +
        ((bdx * bdx + bdy * bdy) * ((cdx * adytail + ady * cdxtail) - (cdy * adxtail + adx * cdytail)) +
        2 * (bdx * bdxtail + bdy * bdytail) * (cdx * ady - cdy * adx)) +
        ((cdx * cdx + cdy * cdy) * ((adx * bdytail + bdy * adxtail) - (ady * bdxtail + bdx * adytail)) +
        2 * (cdx * cdxtail + cdy * cdytail) * (adx * bdy - ady * bdx));

    if (det >= errbound || -det >= errbound) {
        return det;
    }

    if (bdxtail !== 0 || bdytail !== 0 || cdxtail !== 0 || cdytail !== 0) {
        s1 = adx * adx;
        c = splitter * adx;
        ahi = c - (c - adx);
        alo = adx - ahi;
        s0 = alo * alo - (s1 - ahi * ahi - (ahi + ahi) * alo);
        t1 = ady * ady;
        c = splitter * ady;
        ahi = c - (c - ady);
        alo = ady - ahi;
        t0 = alo * alo - (t1 - ahi * ahi - (ahi + ahi) * alo);
        _i = s0 + t0;
        bvirt = _i - s0;
        aa[0] = s0 - (_i - bvirt) + (t0 - bvirt);
        _j = s1 + _i;
        bvirt = _j - s1;
        _0 = s1 - (_j - bvirt) + (_i - bvirt);
        _i = _0 + t1;
        bvirt = _i - _0;
        aa[1] = _0 - (_i - bvirt) + (t1 - bvirt);
        u3 = _j + _i;
        bvirt = u3 - _j;
        aa[2] = _j - (u3 - bvirt) + (_i - bvirt);
        aa[3] = u3;
    }
    if (cdxtail !== 0 || cdytail !== 0 || adxtail !== 0 || adytail !== 0) {
        s1 = bdx * bdx;
        c = splitter * bdx;
        ahi = c - (c - bdx);
        alo = bdx - ahi;
        s0 = alo * alo - (s1 - ahi * ahi - (ahi + ahi) * alo);
        t1 = bdy * bdy;
        c = splitter * bdy;
        ahi = c - (c - bdy);
        alo = bdy - ahi;
        t0 = alo * alo - (t1 - ahi * ahi - (ahi + ahi) * alo);
        _i = s0 + t0;
        bvirt = _i - s0;
        bb[0] = s0 - (_i - bvirt) + (t0 - bvirt);
        _j = s1 + _i;
        bvirt = _j - s1;
        _0 = s1 - (_j - bvirt) + (_i - bvirt);
        _i = _0 + t1;
        bvirt = _i - _0;
        bb[1] = _0 - (_i - bvirt) + (t1 - bvirt);
        u3 = _j + _i;
        bvirt = u3 - _j;
        bb[2] = _j - (u3 - bvirt) + (_i - bvirt);
        bb[3] = u3;
    }
    if (adxtail !== 0 || adytail !== 0 || bdxtail !== 0 || bdytail !== 0) {
        s1 = cdx * cdx;
        c = splitter * cdx;
        ahi = c - (c - cdx);
        alo = cdx - ahi;
        s0 = alo * alo - (s1 - ahi * ahi - (ahi + ahi) * alo);
        t1 = cdy * cdy;
        c = splitter * cdy;
        ahi = c - (c - cdy);
        alo = cdy - ahi;
        t0 = alo * alo - (t1 - ahi * ahi - (ahi + ahi) * alo);
        _i = s0 + t0;
        bvirt = _i - s0;
        cc[0] = s0 - (_i - bvirt) + (t0 - bvirt);
        _j = s1 + _i;
        bvirt = _j - s1;
        _0 = s1 - (_j - bvirt) + (_i - bvirt);
        _i = _0 + t1;
        bvirt = _i - _0;
        cc[1] = _0 - (_i - bvirt) + (t1 - bvirt);
        u3 = _j + _i;
        bvirt = u3 - _j;
        cc[2] = _j - (u3 - bvirt) + (_i - bvirt);
        cc[3] = u3;
    }

    if (adxtail !== 0) {
        axtbclen = scale(4, bc$1, adxtail, axtbc);
        finlen = finadd(finlen, sum_three(
            scale(axtbclen, axtbc, 2 * adx, _16$1), _16$1,
            scale(scale(4, cc, adxtail, _8$1), _8$1, bdy, _16b), _16b,
            scale(scale(4, bb, adxtail, _8$1), _8$1, -cdy, _16c), _16c, _32, _48$1), _48$1);
    }
    if (adytail !== 0) {
        aytbclen = scale(4, bc$1, adytail, aytbc);
        finlen = finadd(finlen, sum_three(
            scale(aytbclen, aytbc, 2 * ady, _16$1), _16$1,
            scale(scale(4, bb, adytail, _8$1), _8$1, cdx, _16b), _16b,
            scale(scale(4, cc, adytail, _8$1), _8$1, -bdx, _16c), _16c, _32, _48$1), _48$1);
    }
    if (bdxtail !== 0) {
        bxtcalen = scale(4, ca, bdxtail, bxtca);
        finlen = finadd(finlen, sum_three(
            scale(bxtcalen, bxtca, 2 * bdx, _16$1), _16$1,
            scale(scale(4, aa, bdxtail, _8$1), _8$1, cdy, _16b), _16b,
            scale(scale(4, cc, bdxtail, _8$1), _8$1, -ady, _16c), _16c, _32, _48$1), _48$1);
    }
    if (bdytail !== 0) {
        bytcalen = scale(4, ca, bdytail, bytca);
        finlen = finadd(finlen, sum_three(
            scale(bytcalen, bytca, 2 * bdy, _16$1), _16$1,
            scale(scale(4, cc, bdytail, _8$1), _8$1, adx, _16b), _16b,
            scale(scale(4, aa, bdytail, _8$1), _8$1, -cdx, _16c), _16c, _32, _48$1), _48$1);
    }
    if (cdxtail !== 0) {
        cxtablen = scale(4, ab$1, cdxtail, cxtab);
        finlen = finadd(finlen, sum_three(
            scale(cxtablen, cxtab, 2 * cdx, _16$1), _16$1,
            scale(scale(4, bb, cdxtail, _8$1), _8$1, ady, _16b), _16b,
            scale(scale(4, aa, cdxtail, _8$1), _8$1, -bdy, _16c), _16c, _32, _48$1), _48$1);
    }
    if (cdytail !== 0) {
        cytablen = scale(4, ab$1, cdytail, cytab);
        finlen = finadd(finlen, sum_three(
            scale(cytablen, cytab, 2 * cdy, _16$1), _16$1,
            scale(scale(4, aa, cdytail, _8$1), _8$1, bdx, _16b), _16b,
            scale(scale(4, bb, cdytail, _8$1), _8$1, -adx, _16c), _16c, _32, _48$1), _48$1);
    }

    if (adxtail !== 0 || adytail !== 0) {
        if (bdxtail !== 0 || bdytail !== 0 || cdxtail !== 0 || cdytail !== 0) {
            s1 = bdxtail * cdy;
            c = splitter * bdxtail;
            ahi = c - (c - bdxtail);
            alo = bdxtail - ahi;
            c = splitter * cdy;
            bhi = c - (c - cdy);
            blo = cdy - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = bdx * cdytail;
            c = splitter * bdx;
            ahi = c - (c - bdx);
            alo = bdx - ahi;
            c = splitter * cdytail;
            bhi = c - (c - cdytail);
            blo = cdytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            u[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            u[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            u[2] = _j - (u3 - bvirt) + (_i - bvirt);
            u[3] = u3;
            s1 = cdxtail * -bdy;
            c = splitter * cdxtail;
            ahi = c - (c - cdxtail);
            alo = cdxtail - ahi;
            c = splitter * -bdy;
            bhi = c - (c - -bdy);
            blo = -bdy - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = cdx * -bdytail;
            c = splitter * cdx;
            ahi = c - (c - cdx);
            alo = cdx - ahi;
            c = splitter * -bdytail;
            bhi = c - (c - -bdytail);
            blo = -bdytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            v[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            v[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            v[2] = _j - (u3 - bvirt) + (_i - bvirt);
            v[3] = u3;
            bctlen = sum$1(4, u, 4, v, bct);
            s1 = bdxtail * cdytail;
            c = splitter * bdxtail;
            ahi = c - (c - bdxtail);
            alo = bdxtail - ahi;
            c = splitter * cdytail;
            bhi = c - (c - cdytail);
            blo = cdytail - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = cdxtail * bdytail;
            c = splitter * cdxtail;
            ahi = c - (c - cdxtail);
            alo = cdxtail - ahi;
            c = splitter * bdytail;
            bhi = c - (c - bdytail);
            blo = bdytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 - t0;
            bvirt = s0 - _i;
            bctt[0] = s0 - (_i + bvirt) + (bvirt - t0);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 - t1;
            bvirt = _0 - _i;
            bctt[1] = _0 - (_i + bvirt) + (bvirt - t1);
            u3 = _j + _i;
            bvirt = u3 - _j;
            bctt[2] = _j - (u3 - bvirt) + (_i - bvirt);
            bctt[3] = u3;
            bcttlen = 4;
        } else {
            bct[0] = 0;
            bctlen = 1;
            bctt[0] = 0;
            bcttlen = 1;
        }
        if (adxtail !== 0) {
            const len = scale(bctlen, bct, adxtail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(axtbclen, axtbc, adxtail, _16$1), _16$1,
                scale(len, _16c, 2 * adx, _32), _32, _48$1), _48$1);

            const len2 = scale(bcttlen, bctt, adxtail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * adx, _16$1), _16$1,
                scale(len2, _8$1, adxtail, _16b), _16b,
                scale(len, _16c, adxtail, _32), _32, _32b, _64), _64);

            if (bdytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, cc, adxtail, _8$1), _8$1, bdytail, _16$1), _16$1);
            }
            if (cdytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, bb, -adxtail, _8$1), _8$1, cdytail, _16$1), _16$1);
            }
        }
        if (adytail !== 0) {
            const len = scale(bctlen, bct, adytail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(aytbclen, aytbc, adytail, _16$1), _16$1,
                scale(len, _16c, 2 * ady, _32), _32, _48$1), _48$1);

            const len2 = scale(bcttlen, bctt, adytail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * ady, _16$1), _16$1,
                scale(len2, _8$1, adytail, _16b), _16b,
                scale(len, _16c, adytail, _32), _32, _32b, _64), _64);
        }
    }
    if (bdxtail !== 0 || bdytail !== 0) {
        if (cdxtail !== 0 || cdytail !== 0 || adxtail !== 0 || adytail !== 0) {
            s1 = cdxtail * ady;
            c = splitter * cdxtail;
            ahi = c - (c - cdxtail);
            alo = cdxtail - ahi;
            c = splitter * ady;
            bhi = c - (c - ady);
            blo = ady - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = cdx * adytail;
            c = splitter * cdx;
            ahi = c - (c - cdx);
            alo = cdx - ahi;
            c = splitter * adytail;
            bhi = c - (c - adytail);
            blo = adytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            u[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            u[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            u[2] = _j - (u3 - bvirt) + (_i - bvirt);
            u[3] = u3;
            n1 = -cdy;
            n0 = -cdytail;
            s1 = adxtail * n1;
            c = splitter * adxtail;
            ahi = c - (c - adxtail);
            alo = adxtail - ahi;
            c = splitter * n1;
            bhi = c - (c - n1);
            blo = n1 - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = adx * n0;
            c = splitter * adx;
            ahi = c - (c - adx);
            alo = adx - ahi;
            c = splitter * n0;
            bhi = c - (c - n0);
            blo = n0 - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            v[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            v[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            v[2] = _j - (u3 - bvirt) + (_i - bvirt);
            v[3] = u3;
            catlen = sum$1(4, u, 4, v, cat);
            s1 = cdxtail * adytail;
            c = splitter * cdxtail;
            ahi = c - (c - cdxtail);
            alo = cdxtail - ahi;
            c = splitter * adytail;
            bhi = c - (c - adytail);
            blo = adytail - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = adxtail * cdytail;
            c = splitter * adxtail;
            ahi = c - (c - adxtail);
            alo = adxtail - ahi;
            c = splitter * cdytail;
            bhi = c - (c - cdytail);
            blo = cdytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 - t0;
            bvirt = s0 - _i;
            catt[0] = s0 - (_i + bvirt) + (bvirt - t0);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 - t1;
            bvirt = _0 - _i;
            catt[1] = _0 - (_i + bvirt) + (bvirt - t1);
            u3 = _j + _i;
            bvirt = u3 - _j;
            catt[2] = _j - (u3 - bvirt) + (_i - bvirt);
            catt[3] = u3;
            cattlen = 4;
        } else {
            cat[0] = 0;
            catlen = 1;
            catt[0] = 0;
            cattlen = 1;
        }
        if (bdxtail !== 0) {
            const len = scale(catlen, cat, bdxtail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(bxtcalen, bxtca, bdxtail, _16$1), _16$1,
                scale(len, _16c, 2 * bdx, _32), _32, _48$1), _48$1);

            const len2 = scale(cattlen, catt, bdxtail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * bdx, _16$1), _16$1,
                scale(len2, _8$1, bdxtail, _16b), _16b,
                scale(len, _16c, bdxtail, _32), _32, _32b, _64), _64);

            if (cdytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, aa, bdxtail, _8$1), _8$1, cdytail, _16$1), _16$1);
            }
            if (adytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, cc, -bdxtail, _8$1), _8$1, adytail, _16$1), _16$1);
            }
        }
        if (bdytail !== 0) {
            const len = scale(catlen, cat, bdytail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(bytcalen, bytca, bdytail, _16$1), _16$1,
                scale(len, _16c, 2 * bdy, _32), _32, _48$1), _48$1);

            const len2 = scale(cattlen, catt, bdytail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * bdy, _16$1), _16$1,
                scale(len2, _8$1, bdytail, _16b), _16b,
                scale(len, _16c, bdytail, _32), _32,  _32b, _64), _64);
        }
    }
    if (cdxtail !== 0 || cdytail !== 0) {
        if (adxtail !== 0 || adytail !== 0 || bdxtail !== 0 || bdytail !== 0) {
            s1 = adxtail * bdy;
            c = splitter * adxtail;
            ahi = c - (c - adxtail);
            alo = adxtail - ahi;
            c = splitter * bdy;
            bhi = c - (c - bdy);
            blo = bdy - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = adx * bdytail;
            c = splitter * adx;
            ahi = c - (c - adx);
            alo = adx - ahi;
            c = splitter * bdytail;
            bhi = c - (c - bdytail);
            blo = bdytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            u[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            u[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            u[2] = _j - (u3 - bvirt) + (_i - bvirt);
            u[3] = u3;
            n1 = -ady;
            n0 = -adytail;
            s1 = bdxtail * n1;
            c = splitter * bdxtail;
            ahi = c - (c - bdxtail);
            alo = bdxtail - ahi;
            c = splitter * n1;
            bhi = c - (c - n1);
            blo = n1 - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = bdx * n0;
            c = splitter * bdx;
            ahi = c - (c - bdx);
            alo = bdx - ahi;
            c = splitter * n0;
            bhi = c - (c - n0);
            blo = n0 - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 + t0;
            bvirt = _i - s0;
            v[0] = s0 - (_i - bvirt) + (t0 - bvirt);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 + t1;
            bvirt = _i - _0;
            v[1] = _0 - (_i - bvirt) + (t1 - bvirt);
            u3 = _j + _i;
            bvirt = u3 - _j;
            v[2] = _j - (u3 - bvirt) + (_i - bvirt);
            v[3] = u3;
            abtlen = sum$1(4, u, 4, v, abt);
            s1 = adxtail * bdytail;
            c = splitter * adxtail;
            ahi = c - (c - adxtail);
            alo = adxtail - ahi;
            c = splitter * bdytail;
            bhi = c - (c - bdytail);
            blo = bdytail - bhi;
            s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
            t1 = bdxtail * adytail;
            c = splitter * bdxtail;
            ahi = c - (c - bdxtail);
            alo = bdxtail - ahi;
            c = splitter * adytail;
            bhi = c - (c - adytail);
            blo = adytail - bhi;
            t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
            _i = s0 - t0;
            bvirt = s0 - _i;
            abtt[0] = s0 - (_i + bvirt) + (bvirt - t0);
            _j = s1 + _i;
            bvirt = _j - s1;
            _0 = s1 - (_j - bvirt) + (_i - bvirt);
            _i = _0 - t1;
            bvirt = _0 - _i;
            abtt[1] = _0 - (_i + bvirt) + (bvirt - t1);
            u3 = _j + _i;
            bvirt = u3 - _j;
            abtt[2] = _j - (u3 - bvirt) + (_i - bvirt);
            abtt[3] = u3;
            abttlen = 4;
        } else {
            abt[0] = 0;
            abtlen = 1;
            abtt[0] = 0;
            abttlen = 1;
        }
        if (cdxtail !== 0) {
            const len = scale(abtlen, abt, cdxtail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(cxtablen, cxtab, cdxtail, _16$1), _16$1,
                scale(len, _16c, 2 * cdx, _32), _32, _48$1), _48$1);

            const len2 = scale(abttlen, abtt, cdxtail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * cdx, _16$1), _16$1,
                scale(len2, _8$1, cdxtail, _16b), _16b,
                scale(len, _16c, cdxtail, _32), _32, _32b, _64), _64);

            if (adytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, bb, cdxtail, _8$1), _8$1, adytail, _16$1), _16$1);
            }
            if (bdytail !== 0) {
                finlen = finadd(finlen, scale(scale(4, aa, -cdxtail, _8$1), _8$1, bdytail, _16$1), _16$1);
            }
        }
        if (cdytail !== 0) {
            const len = scale(abtlen, abt, cdytail, _16c);
            finlen = finadd(finlen, sum$1(
                scale(cytablen, cytab, cdytail, _16$1), _16$1,
                scale(len, _16c, 2 * cdy, _32), _32, _48$1), _48$1);

            const len2 = scale(abttlen, abtt, cdytail, _8$1);
            finlen = finadd(finlen, sum_three(
                scale(len2, _8$1, 2 * cdy, _16$1), _16$1,
                scale(len2, _8$1, cdytail, _16b), _16b,
                scale(len, _16c, cdytail, _32), _32, _32b, _64), _64);
        }
    }

    return fin$1[finlen - 1];
}

function incircle(ax, ay, bx, by, cx, cy, dx, dy) {
    const adx = ax - dx;
    const bdx = bx - dx;
    const cdx = cx - dx;
    const ady = ay - dy;
    const bdy = by - dy;
    const cdy = cy - dy;

    const bdxcdy = bdx * cdy;
    const cdxbdy = cdx * bdy;
    const alift = adx * adx + ady * ady;

    const cdxady = cdx * ady;
    const adxcdy = adx * cdy;
    const blift = bdx * bdx + bdy * bdy;

    const adxbdy = adx * bdy;
    const bdxady = bdx * ady;
    const clift = cdx * cdx + cdy * cdy;

    const det =
        alift * (bdxcdy - cdxbdy) +
        blift * (cdxady - adxcdy) +
        clift * (adxbdy - bdxady);

    const permanent =
        (Math.abs(bdxcdy) + Math.abs(cdxbdy)) * alift +
        (Math.abs(cdxady) + Math.abs(adxcdy)) * blift +
        (Math.abs(adxbdy) + Math.abs(bdxady)) * clift;

    const errbound = iccerrboundA * permanent;

    if (det > errbound || -det > errbound) {
        return det;
    }
    return incircleadapt(ax, ay, bx, by, cx, cy, dx, dy, permanent);
}

function incirclefast(ax, ay, bx, by, cx, cy, dx, dy) {
    const adx = ax - dx;
    const ady = ay - dy;
    const bdx = bx - dx;
    const bdy = by - dy;
    const cdx = cx - dx;
    const cdy = cy - dy;

    const abdet = adx * bdy - bdx * ady;
    const bcdet = bdx * cdy - cdx * bdy;
    const cadet = cdx * ady - adx * cdy;
    const alift = adx * adx + ady * ady;
    const blift = bdx * bdx + bdy * bdy;
    const clift = cdx * cdx + cdy * cdy;

    return alift * bcdet + blift * cadet + clift * abdet;
}

const isperrboundA = (16 + 224 * epsilon$3) * epsilon$3;
const isperrboundB = (5 + 72 * epsilon$3) * epsilon$3;
const isperrboundC = (71 + 1408 * epsilon$3) * epsilon$3 * epsilon$3;

const ab = vec(4);
const bc = vec(4);
const cd = vec(4);
const de = vec(4);
const ea = vec(4);
const ac = vec(4);
const bd = vec(4);
const ce = vec(4);
const da = vec(4);
const eb = vec(4);

const abc = vec(24);
const bcd = vec(24);
const cde = vec(24);
const dea = vec(24);
const eab = vec(24);
const abd = vec(24);
const bce = vec(24);
const cda = vec(24);
const deb = vec(24);
const eac = vec(24);

const adet = vec(1152);
const bdet = vec(1152);
const cdet = vec(1152);
const ddet = vec(1152);
const edet = vec(1152);
const abdet = vec(2304);
const cddet = vec(2304);
const cdedet = vec(3456);
const deter = vec(5760);

const _8 = vec(8);
const _8b = vec(8);
const _8c = vec(8);
const _16 = vec(16);
const _24 = vec(24);
const _48 = vec(48);
const _48b = vec(48);
const _96 = vec(96);
const _192 = vec(192);
const _384x = vec(384);
const _384y = vec(384);
const _384z = vec(384);
const _768 = vec(768);

function sum_three_scale(a, b, c, az, bz, cz, out) {
    return sum_three(
        scale(4, a, az, _8), _8,
        scale(4, b, bz, _8b), _8b,
        scale(4, c, cz, _8c), _8c, _16, out);
}

function liftexact(alen, a, blen, b, clen, c, dlen, d, x, y, z, out) {
    const len = sum$1(
        sum$1(alen, a, blen, b, _48), _48,
        negate(sum$1(clen, c, dlen, d, _48b), _48b), _48b, _96);

    return sum_three(
        scale(scale(len, _96, x, _192), _192, x, _384x), _384x,
        scale(scale(len, _96, y, _192), _192, y, _384y), _384y,
        scale(scale(len, _96, z, _192), _192, z, _384z), _384z, _768, out);
}

function insphereexact(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, ex, ey, ez) {
    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _0, s1, s0, t1, t0, u3;

    s1 = ax * by;
    c = splitter * ax;
    ahi = c - (c - ax);
    alo = ax - ahi;
    c = splitter * by;
    bhi = c - (c - by);
    blo = by - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = bx * ay;
    c = splitter * bx;
    ahi = c - (c - bx);
    alo = bx - ahi;
    c = splitter * ay;
    bhi = c - (c - ay);
    blo = ay - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ab[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ab[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ab[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ab[3] = u3;
    s1 = bx * cy;
    c = splitter * bx;
    ahi = c - (c - bx);
    alo = bx - ahi;
    c = splitter * cy;
    bhi = c - (c - cy);
    blo = cy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cx * by;
    c = splitter * cx;
    ahi = c - (c - cx);
    alo = cx - ahi;
    c = splitter * by;
    bhi = c - (c - by);
    blo = by - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bc[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bc[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    bc[2] = _j - (u3 - bvirt) + (_i - bvirt);
    bc[3] = u3;
    s1 = cx * dy;
    c = splitter * cx;
    ahi = c - (c - cx);
    alo = cx - ahi;
    c = splitter * dy;
    bhi = c - (c - dy);
    blo = dy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = dx * cy;
    c = splitter * dx;
    ahi = c - (c - dx);
    alo = dx - ahi;
    c = splitter * cy;
    bhi = c - (c - cy);
    blo = cy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    cd[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    cd[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    cd[2] = _j - (u3 - bvirt) + (_i - bvirt);
    cd[3] = u3;
    s1 = dx * ey;
    c = splitter * dx;
    ahi = c - (c - dx);
    alo = dx - ahi;
    c = splitter * ey;
    bhi = c - (c - ey);
    blo = ey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = ex * dy;
    c = splitter * ex;
    ahi = c - (c - ex);
    alo = ex - ahi;
    c = splitter * dy;
    bhi = c - (c - dy);
    blo = dy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    de[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    de[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    de[2] = _j - (u3 - bvirt) + (_i - bvirt);
    de[3] = u3;
    s1 = ex * ay;
    c = splitter * ex;
    ahi = c - (c - ex);
    alo = ex - ahi;
    c = splitter * ay;
    bhi = c - (c - ay);
    blo = ay - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = ax * ey;
    c = splitter * ax;
    ahi = c - (c - ax);
    alo = ax - ahi;
    c = splitter * ey;
    bhi = c - (c - ey);
    blo = ey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ea[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ea[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ea[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ea[3] = u3;
    s1 = ax * cy;
    c = splitter * ax;
    ahi = c - (c - ax);
    alo = ax - ahi;
    c = splitter * cy;
    bhi = c - (c - cy);
    blo = cy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cx * ay;
    c = splitter * cx;
    ahi = c - (c - cx);
    alo = cx - ahi;
    c = splitter * ay;
    bhi = c - (c - ay);
    blo = ay - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ac[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ac[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ac[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ac[3] = u3;
    s1 = bx * dy;
    c = splitter * bx;
    ahi = c - (c - bx);
    alo = bx - ahi;
    c = splitter * dy;
    bhi = c - (c - dy);
    blo = dy - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = dx * by;
    c = splitter * dx;
    ahi = c - (c - dx);
    alo = dx - ahi;
    c = splitter * by;
    bhi = c - (c - by);
    blo = by - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bd[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bd[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    bd[2] = _j - (u3 - bvirt) + (_i - bvirt);
    bd[3] = u3;
    s1 = cx * ey;
    c = splitter * cx;
    ahi = c - (c - cx);
    alo = cx - ahi;
    c = splitter * ey;
    bhi = c - (c - ey);
    blo = ey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = ex * cy;
    c = splitter * ex;
    ahi = c - (c - ex);
    alo = ex - ahi;
    c = splitter * cy;
    bhi = c - (c - cy);
    blo = cy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ce[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ce[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    ce[2] = _j - (u3 - bvirt) + (_i - bvirt);
    ce[3] = u3;
    s1 = dx * ay;
    c = splitter * dx;
    ahi = c - (c - dx);
    alo = dx - ahi;
    c = splitter * ay;
    bhi = c - (c - ay);
    blo = ay - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = ax * dy;
    c = splitter * ax;
    ahi = c - (c - ax);
    alo = ax - ahi;
    c = splitter * dy;
    bhi = c - (c - dy);
    blo = dy - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    da[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    da[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    da[2] = _j - (u3 - bvirt) + (_i - bvirt);
    da[3] = u3;
    s1 = ex * by;
    c = splitter * ex;
    ahi = c - (c - ex);
    alo = ex - ahi;
    c = splitter * by;
    bhi = c - (c - by);
    blo = by - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = bx * ey;
    c = splitter * bx;
    ahi = c - (c - bx);
    alo = bx - ahi;
    c = splitter * ey;
    bhi = c - (c - ey);
    blo = ey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    eb[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    eb[1] = _0 - (_i + bvirt) + (bvirt - t1);
    u3 = _j + _i;
    bvirt = u3 - _j;
    eb[2] = _j - (u3 - bvirt) + (_i - bvirt);
    eb[3] = u3;

    const abclen = sum_three_scale(ab, bc, ac, cz, az, -bz, abc);
    const bcdlen = sum_three_scale(bc, cd, bd, dz, bz, -cz, bcd);
    const cdelen = sum_three_scale(cd, de, ce, ez, cz, -dz, cde);
    const dealen = sum_three_scale(de, ea, da, az, dz, -ez, dea);
    const eablen = sum_three_scale(ea, ab, eb, bz, ez, -az, eab);
    const abdlen = sum_three_scale(ab, bd, da, dz, az, bz, abd);
    const bcelen = sum_three_scale(bc, ce, eb, ez, bz, cz, bce);
    const cdalen = sum_three_scale(cd, da, ac, az, cz, dz, cda);
    const deblen = sum_three_scale(de, eb, bd, bz, dz, ez, deb);
    const eaclen = sum_three_scale(ea, ac, ce, cz, ez, az, eac);

    const deterlen = sum_three(
        liftexact(cdelen, cde, bcelen, bce, deblen, deb, bcdlen, bcd, ax, ay, az, adet), adet,
        liftexact(dealen, dea, cdalen, cda, eaclen, eac, cdelen, cde, bx, by, bz, bdet), bdet,
        sum_three(
            liftexact(eablen, eab, deblen, deb, abdlen, abd, dealen, dea, cx, cy, cz, cdet), cdet,
            liftexact(abclen, abc, eaclen, eac, bcelen, bce, eablen, eab, dx, dy, dz, ddet), ddet,
            liftexact(bcdlen, bcd, abdlen, abd, cdalen, cda, abclen, abc, ex, ey, ez, edet), edet, cddet, cdedet), cdedet, abdet, deter);

    return deter[deterlen - 1];
}

const xdet = vec(96);
const ydet = vec(96);
const zdet = vec(96);
const fin = vec(1152);

function liftadapt(a, b, c, az, bz, cz, x, y, z, out) {
    const len = sum_three_scale(a, b, c, az, bz, cz, _24);
    return sum_three(
        scale(scale(len, _24, x, _48), _48, x, xdet), xdet,
        scale(scale(len, _24, y, _48), _48, y, ydet), ydet,
        scale(scale(len, _24, z, _48), _48, z, zdet), zdet, _192, out);
}

function insphereadapt(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, ex, ey, ez, permanent) {
    let ab3, bc3, cd3, da3, ac3, bd3;

    let aextail, bextail, cextail, dextail;
    let aeytail, beytail, ceytail, deytail;
    let aeztail, beztail, ceztail, deztail;

    let bvirt, c, ahi, alo, bhi, blo, _i, _j, _0, s1, s0, t1, t0;

    const aex = ax - ex;
    const bex = bx - ex;
    const cex = cx - ex;
    const dex = dx - ex;
    const aey = ay - ey;
    const bey = by - ey;
    const cey = cy - ey;
    const dey = dy - ey;
    const aez = az - ez;
    const bez = bz - ez;
    const cez = cz - ez;
    const dez = dz - ez;

    s1 = aex * bey;
    c = splitter * aex;
    ahi = c - (c - aex);
    alo = aex - ahi;
    c = splitter * bey;
    bhi = c - (c - bey);
    blo = bey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = bex * aey;
    c = splitter * bex;
    ahi = c - (c - bex);
    alo = bex - ahi;
    c = splitter * aey;
    bhi = c - (c - aey);
    blo = aey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ab[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ab[1] = _0 - (_i + bvirt) + (bvirt - t1);
    ab3 = _j + _i;
    bvirt = ab3 - _j;
    ab[2] = _j - (ab3 - bvirt) + (_i - bvirt);
    ab[3] = ab3;
    s1 = bex * cey;
    c = splitter * bex;
    ahi = c - (c - bex);
    alo = bex - ahi;
    c = splitter * cey;
    bhi = c - (c - cey);
    blo = cey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cex * bey;
    c = splitter * cex;
    ahi = c - (c - cex);
    alo = cex - ahi;
    c = splitter * bey;
    bhi = c - (c - bey);
    blo = bey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bc[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bc[1] = _0 - (_i + bvirt) + (bvirt - t1);
    bc3 = _j + _i;
    bvirt = bc3 - _j;
    bc[2] = _j - (bc3 - bvirt) + (_i - bvirt);
    bc[3] = bc3;
    s1 = cex * dey;
    c = splitter * cex;
    ahi = c - (c - cex);
    alo = cex - ahi;
    c = splitter * dey;
    bhi = c - (c - dey);
    blo = dey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = dex * cey;
    c = splitter * dex;
    ahi = c - (c - dex);
    alo = dex - ahi;
    c = splitter * cey;
    bhi = c - (c - cey);
    blo = cey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    cd[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    cd[1] = _0 - (_i + bvirt) + (bvirt - t1);
    cd3 = _j + _i;
    bvirt = cd3 - _j;
    cd[2] = _j - (cd3 - bvirt) + (_i - bvirt);
    cd[3] = cd3;
    s1 = dex * aey;
    c = splitter * dex;
    ahi = c - (c - dex);
    alo = dex - ahi;
    c = splitter * aey;
    bhi = c - (c - aey);
    blo = aey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = aex * dey;
    c = splitter * aex;
    ahi = c - (c - aex);
    alo = aex - ahi;
    c = splitter * dey;
    bhi = c - (c - dey);
    blo = dey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    da[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    da[1] = _0 - (_i + bvirt) + (bvirt - t1);
    da3 = _j + _i;
    bvirt = da3 - _j;
    da[2] = _j - (da3 - bvirt) + (_i - bvirt);
    da[3] = da3;
    s1 = aex * cey;
    c = splitter * aex;
    ahi = c - (c - aex);
    alo = aex - ahi;
    c = splitter * cey;
    bhi = c - (c - cey);
    blo = cey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = cex * aey;
    c = splitter * cex;
    ahi = c - (c - cex);
    alo = cex - ahi;
    c = splitter * aey;
    bhi = c - (c - aey);
    blo = aey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    ac[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    ac[1] = _0 - (_i + bvirt) + (bvirt - t1);
    ac3 = _j + _i;
    bvirt = ac3 - _j;
    ac[2] = _j - (ac3 - bvirt) + (_i - bvirt);
    ac[3] = ac3;
    s1 = bex * dey;
    c = splitter * bex;
    ahi = c - (c - bex);
    alo = bex - ahi;
    c = splitter * dey;
    bhi = c - (c - dey);
    blo = dey - bhi;
    s0 = alo * blo - (s1 - ahi * bhi - alo * bhi - ahi * blo);
    t1 = dex * bey;
    c = splitter * dex;
    ahi = c - (c - dex);
    alo = dex - ahi;
    c = splitter * bey;
    bhi = c - (c - bey);
    blo = bey - bhi;
    t0 = alo * blo - (t1 - ahi * bhi - alo * bhi - ahi * blo);
    _i = s0 - t0;
    bvirt = s0 - _i;
    bd[0] = s0 - (_i + bvirt) + (bvirt - t0);
    _j = s1 + _i;
    bvirt = _j - s1;
    _0 = s1 - (_j - bvirt) + (_i - bvirt);
    _i = _0 - t1;
    bvirt = _0 - _i;
    bd[1] = _0 - (_i + bvirt) + (bvirt - t1);
    bd3 = _j + _i;
    bvirt = bd3 - _j;
    bd[2] = _j - (bd3 - bvirt) + (_i - bvirt);
    bd[3] = bd3;

    const finlen = sum$1(
        sum$1(
            negate(liftadapt(bc, cd, bd, dez, bez, -cez, aex, aey, aez, adet), adet), adet,
            liftadapt(cd, da, ac, aez, cez, dez, bex, bey, bez, bdet), bdet, abdet), abdet,
        sum$1(
            negate(liftadapt(da, ab, bd, bez, dez, aez, cex, cey, cez, cdet), cdet), cdet,
            liftadapt(ab, bc, ac, cez, aez, -bez, dex, dey, dez, ddet), ddet, cddet), cddet, fin);

    let det = estimate(finlen, fin);
    let errbound = isperrboundB * permanent;
    if (det >= errbound || -det >= errbound) {
        return det;
    }

    bvirt = ax - aex;
    aextail = ax - (aex + bvirt) + (bvirt - ex);
    bvirt = ay - aey;
    aeytail = ay - (aey + bvirt) + (bvirt - ey);
    bvirt = az - aez;
    aeztail = az - (aez + bvirt) + (bvirt - ez);
    bvirt = bx - bex;
    bextail = bx - (bex + bvirt) + (bvirt - ex);
    bvirt = by - bey;
    beytail = by - (bey + bvirt) + (bvirt - ey);
    bvirt = bz - bez;
    beztail = bz - (bez + bvirt) + (bvirt - ez);
    bvirt = cx - cex;
    cextail = cx - (cex + bvirt) + (bvirt - ex);
    bvirt = cy - cey;
    ceytail = cy - (cey + bvirt) + (bvirt - ey);
    bvirt = cz - cez;
    ceztail = cz - (cez + bvirt) + (bvirt - ez);
    bvirt = dx - dex;
    dextail = dx - (dex + bvirt) + (bvirt - ex);
    bvirt = dy - dey;
    deytail = dy - (dey + bvirt) + (bvirt - ey);
    bvirt = dz - dez;
    deztail = dz - (dez + bvirt) + (bvirt - ez);
    if (aextail === 0 && aeytail === 0 && aeztail === 0 &&
        bextail === 0 && beytail === 0 && beztail === 0 &&
        cextail === 0 && ceytail === 0 && ceztail === 0 &&
        dextail === 0 && deytail === 0 && deztail === 0) {
        return det;
    }

    errbound = isperrboundC * permanent + resulterrbound * Math.abs(det);

    const abeps = (aex * beytail + bey * aextail) - (aey * bextail + bex * aeytail);
    const bceps = (bex * ceytail + cey * bextail) - (bey * cextail + cex * beytail);
    const cdeps = (cex * deytail + dey * cextail) - (cey * dextail + dex * ceytail);
    const daeps = (dex * aeytail + aey * dextail) - (dey * aextail + aex * deytail);
    const aceps = (aex * ceytail + cey * aextail) - (aey * cextail + cex * aeytail);
    const bdeps = (bex * deytail + dey * bextail) - (bey * dextail + dex * beytail);
    det +=
        (((bex * bex + bey * bey + bez * bez) * ((cez * daeps + dez * aceps + aez * cdeps) +
        (ceztail * da3 + deztail * ac3 + aeztail * cd3)) + (dex * dex + dey * dey + dez * dez) *
        ((aez * bceps - bez * aceps + cez * abeps) + (aeztail * bc3 - beztail * ac3 + ceztail * ab3))) -
        ((aex * aex + aey * aey + aez * aez) * ((bez * cdeps - cez * bdeps + dez * bceps) +
        (beztail * cd3 - ceztail * bd3 + deztail * bc3)) + (cex * cex + cey * cey + cez * cez) *
        ((dez * abeps + aez * bdeps + bez * daeps) + (deztail * ab3 + aeztail * bd3 + beztail * da3)))) +
        2 * (((bex * bextail + bey * beytail + bez * beztail) * (cez * da3 + dez * ac3 + aez * cd3) +
        (dex * dextail + dey * deytail + dez * deztail) * (aez * bc3 - bez * ac3 + cez * ab3)) -
        ((aex * aextail + aey * aeytail + aez * aeztail) * (bez * cd3 - cez * bd3 + dez * bc3) +
        (cex * cextail + cey * ceytail + cez * ceztail) * (dez * ab3 + aez * bd3 + bez * da3)));

    if (det >= errbound || -det >= errbound) {
        return det;
    }

    return insphereexact(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, ex, ey, ez);
}

function insphere(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, ex, ey, ez) {
    const aex = ax - ex;
    const bex = bx - ex;
    const cex = cx - ex;
    const dex = dx - ex;
    const aey = ay - ey;
    const bey = by - ey;
    const cey = cy - ey;
    const dey = dy - ey;
    const aez = az - ez;
    const bez = bz - ez;
    const cez = cz - ez;
    const dez = dz - ez;

    const aexbey = aex * bey;
    const bexaey = bex * aey;
    const ab = aexbey - bexaey;
    const bexcey = bex * cey;
    const cexbey = cex * bey;
    const bc = bexcey - cexbey;
    const cexdey = cex * dey;
    const dexcey = dex * cey;
    const cd = cexdey - dexcey;
    const dexaey = dex * aey;
    const aexdey = aex * dey;
    const da = dexaey - aexdey;
    const aexcey = aex * cey;
    const cexaey = cex * aey;
    const ac = aexcey - cexaey;
    const bexdey = bex * dey;
    const dexbey = dex * bey;
    const bd = bexdey - dexbey;

    const alift = aex * aex + aey * aey + aez * aez;
    const blift = bex * bex + bey * bey + bez * bez;
    const clift = cex * cex + cey * cey + cez * cez;
    const dlift = dex * dex + dey * dey + dez * dez;

    const det =
        (clift * (dez * ab + aez * bd + bez * da) - dlift * (aez * bc - bez * ac + cez * ab)) +
        (alift * (bez * cd - cez * bd + dez * bc) - blift * (cez * da + dez * ac + aez * cd));

    const aezplus = Math.abs(aez);
    const bezplus = Math.abs(bez);
    const cezplus = Math.abs(cez);
    const dezplus = Math.abs(dez);
    const aexbeyplus = Math.abs(aexbey) + Math.abs(bexaey);
    const bexceyplus = Math.abs(bexcey) + Math.abs(cexbey);
    const cexdeyplus = Math.abs(cexdey) + Math.abs(dexcey);
    const dexaeyplus = Math.abs(dexaey) + Math.abs(aexdey);
    const aexceyplus = Math.abs(aexcey) + Math.abs(cexaey);
    const bexdeyplus = Math.abs(bexdey) + Math.abs(dexbey);
    const permanent =
        (cexdeyplus * bezplus + bexdeyplus * cezplus + bexceyplus * dezplus) * alift +
        (dexaeyplus * cezplus + aexceyplus * dezplus + cexdeyplus * aezplus) * blift +
        (aexbeyplus * dezplus + bexdeyplus * aezplus + dexaeyplus * bezplus) * clift +
        (bexceyplus * aezplus + aexceyplus * bezplus + aexbeyplus * cezplus) * dlift;

    const errbound = isperrboundA * permanent;
    if (det > errbound || -det > errbound) {
        return det;
    }
    return -insphereadapt(ax, ay, az, bx, by, bz, cx, cy, cz, dx, dy, dz, ex, ey, ez, permanent);
}

function inspherefast(pax, pay, paz, pbx, pby, pbz, pcx, pcy, pcz, pdx, pdy, pdz, pex, pey, pez) {
    const aex = pax - pex;
    const bex = pbx - pex;
    const cex = pcx - pex;
    const dex = pdx - pex;
    const aey = pay - pey;
    const bey = pby - pey;
    const cey = pcy - pey;
    const dey = pdy - pey;
    const aez = paz - pez;
    const bez = pbz - pez;
    const cez = pcz - pez;
    const dez = pdz - pez;

    const ab = aex * bey - bex * aey;
    const bc = bex * cey - cex * bey;
    const cd = cex * dey - dex * cey;
    const da = dex * aey - aex * dey;
    const ac = aex * cey - cex * aey;
    const bd = bex * dey - dex * bey;

    const abc = aez * bc - bez * ac + cez * ab;
    const bcd = bez * cd - cez * bd + dez * bc;
    const cda = cez * da + dez * ac + aez * cd;
    const dab = dez * ab + aez * bd + bez * da;

    const alift = aex * aex + aey * aey + aez * aez;
    const blift = bex * bex + bey * bey + bez * bez;
    const clift = cex * cex + cey * cey + cez * cez;
    const dlift = dex * dex + dey * dey + dez * dez;

    return (clift * dab - dlift * abc) + (alift * bcd - blift * cda);
}

const EPSILON = Math.pow(2, -52);
const EDGE_STACK = new Uint32Array(512);

class Delaunator {

    static from(points, getX = defaultGetX, getY = defaultGetY) {
        const n = points.length;
        const coords = new Float64Array(n * 2);

        for (let i = 0; i < n; i++) {
            const p = points[i];
            coords[2 * i] = getX(p);
            coords[2 * i + 1] = getY(p);
        }

        return new Delaunator(coords);
    }

    constructor(coords) {
        const n = coords.length >> 1;
        if (n > 0 && typeof coords[0] !== 'number') throw new Error('Expected coords to contain numbers.');

        this.coords = coords;

        // arrays that will store the triangulation graph
        const maxTriangles = Math.max(2 * n - 5, 0);
        this._triangles = new Uint32Array(maxTriangles * 3);
        this._halfedges = new Int32Array(maxTriangles * 3);

        // temporary arrays for tracking the edges of the advancing convex hull
        this._hashSize = Math.ceil(Math.sqrt(n));
        this._hullPrev = new Uint32Array(n); // edge to prev edge
        this._hullNext = new Uint32Array(n); // edge to next edge
        this._hullTri = new Uint32Array(n); // edge to adjacent triangle
        this._hullHash = new Int32Array(this._hashSize); // angular edge hash

        // temporary arrays for sorting points
        this._ids = new Uint32Array(n);
        this._dists = new Float64Array(n);

        this.update();
    }

    update() {
        const {coords, _hullPrev: hullPrev, _hullNext: hullNext, _hullTri: hullTri, _hullHash: hullHash} =  this;
        const n = coords.length >> 1;

        // populate an array of point indices; calculate input data bbox
        let minX = Infinity;
        let minY = Infinity;
        let maxX = -Infinity;
        let maxY = -Infinity;

        for (let i = 0; i < n; i++) {
            const x = coords[2 * i];
            const y = coords[2 * i + 1];
            if (x < minX) minX = x;
            if (y < minY) minY = y;
            if (x > maxX) maxX = x;
            if (y > maxY) maxY = y;
            this._ids[i] = i;
        }
        const cx = (minX + maxX) / 2;
        const cy = (minY + maxY) / 2;

        let i0, i1, i2;

        // pick a seed point close to the center
        for (let i = 0, minDist = Infinity; i < n; i++) {
            const d = dist(cx, cy, coords[2 * i], coords[2 * i + 1]);
            if (d < minDist) {
                i0 = i;
                minDist = d;
            }
        }
        const i0x = coords[2 * i0];
        const i0y = coords[2 * i0 + 1];

        // find the point closest to the seed
        for (let i = 0, minDist = Infinity; i < n; i++) {
            if (i === i0) continue;
            const d = dist(i0x, i0y, coords[2 * i], coords[2 * i + 1]);
            if (d < minDist && d > 0) {
                i1 = i;
                minDist = d;
            }
        }
        let i1x = coords[2 * i1];
        let i1y = coords[2 * i1 + 1];

        let minRadius = Infinity;

        // find the third point which forms the smallest circumcircle with the first two
        for (let i = 0; i < n; i++) {
            if (i === i0 || i === i1) continue;
            const r = circumradius(i0x, i0y, i1x, i1y, coords[2 * i], coords[2 * i + 1]);
            if (r < minRadius) {
                i2 = i;
                minRadius = r;
            }
        }
        let i2x = coords[2 * i2];
        let i2y = coords[2 * i2 + 1];

        if (minRadius === Infinity) {
            // order collinear points by dx (or dy if all x are identical)
            // and return the list as a hull
            for (let i = 0; i < n; i++) {
                this._dists[i] = (coords[2 * i] - coords[0]) || (coords[2 * i + 1] - coords[1]);
            }
            quicksort(this._ids, this._dists, 0, n - 1);
            const hull = new Uint32Array(n);
            let j = 0;
            for (let i = 0, d0 = -Infinity; i < n; i++) {
                const id = this._ids[i];
                const d = this._dists[id];
                if (d > d0) {
                    hull[j++] = id;
                    d0 = d;
                }
            }
            this.hull = hull.subarray(0, j);
            this.triangles = new Uint32Array(0);
            this.halfedges = new Uint32Array(0);
            return;
        }

        // swap the order of the seed points for counter-clockwise orientation
        if (orient2d(i0x, i0y, i1x, i1y, i2x, i2y) < 0) {
            const i = i1;
            const x = i1x;
            const y = i1y;
            i1 = i2;
            i1x = i2x;
            i1y = i2y;
            i2 = i;
            i2x = x;
            i2y = y;
        }

        const center = circumcenter(i0x, i0y, i1x, i1y, i2x, i2y);
        this._cx = center.x;
        this._cy = center.y;

        for (let i = 0; i < n; i++) {
            this._dists[i] = dist(coords[2 * i], coords[2 * i + 1], center.x, center.y);
        }

        // sort the points by distance from the seed triangle circumcenter
        quicksort(this._ids, this._dists, 0, n - 1);

        // set up the seed triangle as the starting hull
        this._hullStart = i0;
        let hullSize = 3;

        hullNext[i0] = hullPrev[i2] = i1;
        hullNext[i1] = hullPrev[i0] = i2;
        hullNext[i2] = hullPrev[i1] = i0;

        hullTri[i0] = 0;
        hullTri[i1] = 1;
        hullTri[i2] = 2;

        hullHash.fill(-1);
        hullHash[this._hashKey(i0x, i0y)] = i0;
        hullHash[this._hashKey(i1x, i1y)] = i1;
        hullHash[this._hashKey(i2x, i2y)] = i2;

        this.trianglesLen = 0;
        this._addTriangle(i0, i1, i2, -1, -1, -1);

        for (let k = 0, xp, yp; k < this._ids.length; k++) {
            const i = this._ids[k];
            const x = coords[2 * i];
            const y = coords[2 * i + 1];

            // skip near-duplicate points
            if (k > 0 && Math.abs(x - xp) <= EPSILON && Math.abs(y - yp) <= EPSILON) continue;
            xp = x;
            yp = y;

            // skip seed triangle points
            if (i === i0 || i === i1 || i === i2) continue;

            // find a visible edge on the convex hull using edge hash
            let start = 0;
            for (let j = 0, key = this._hashKey(x, y); j < this._hashSize; j++) {
                start = hullHash[(key + j) % this._hashSize];
                if (start !== -1 && start !== hullNext[start]) break;
            }

            start = hullPrev[start];
            let e = start, q;
            while (q = hullNext[e], orient2d(x, y, coords[2 * e], coords[2 * e + 1], coords[2 * q], coords[2 * q + 1]) >= 0) {
                e = q;
                if (e === start) {
                    e = -1;
                    break;
                }
            }
            if (e === -1) continue; // likely a near-duplicate point; skip it

            // add the first triangle from the point
            let t = this._addTriangle(e, i, hullNext[e], -1, -1, hullTri[e]);

            // recursively flip triangles from the point until they satisfy the Delaunay condition
            hullTri[i] = this._legalize(t + 2);
            hullTri[e] = t; // keep track of boundary triangles on the hull
            hullSize++;

            // walk forward through the hull, adding more triangles and flipping recursively
            let n = hullNext[e];
            while (q = hullNext[n], orient2d(x, y, coords[2 * n], coords[2 * n + 1], coords[2 * q], coords[2 * q + 1]) < 0) {
                t = this._addTriangle(n, i, q, hullTri[i], -1, hullTri[n]);
                hullTri[i] = this._legalize(t + 2);
                hullNext[n] = n; // mark as removed
                hullSize--;
                n = q;
            }

            // walk backward from the other side, adding more triangles and flipping
            if (e === start) {
                while (q = hullPrev[e], orient2d(x, y, coords[2 * q], coords[2 * q + 1], coords[2 * e], coords[2 * e + 1]) < 0) {
                    t = this._addTriangle(q, i, e, -1, hullTri[e], hullTri[q]);
                    this._legalize(t + 2);
                    hullTri[q] = t;
                    hullNext[e] = e; // mark as removed
                    hullSize--;
                    e = q;
                }
            }

            // update the hull indices
            this._hullStart = hullPrev[i] = e;
            hullNext[e] = hullPrev[n] = i;
            hullNext[i] = n;

            // save the two new edges in the hash table
            hullHash[this._hashKey(x, y)] = i;
            hullHash[this._hashKey(coords[2 * e], coords[2 * e + 1])] = e;
        }

        this.hull = new Uint32Array(hullSize);
        for (let i = 0, e = this._hullStart; i < hullSize; i++) {
            this.hull[i] = e;
            e = hullNext[e];
        }

        // trim typed triangle mesh arrays
        this.triangles = this._triangles.subarray(0, this.trianglesLen);
        this.halfedges = this._halfedges.subarray(0, this.trianglesLen);
    }

    _hashKey(x, y) {
        return Math.floor(pseudoAngle(x - this._cx, y - this._cy) * this._hashSize) % this._hashSize;
    }

    _legalize(a) {
        const {_triangles: triangles, _halfedges: halfedges, coords} = this;

        let i = 0;
        let ar = 0;

        // recursion eliminated with a fixed-size stack
        while (true) {
            const b = halfedges[a];

            /* if the pair of triangles doesn't satisfy the Delaunay condition
             * (p1 is inside the circumcircle of [p0, pl, pr]), flip them,
             * then do the same check/flip recursively for the new pair of triangles
             *
             *           pl                    pl
             *          /||\                  /  \
             *       al/ || \bl            al/    \a
             *        /  ||  \              /      \
             *       /  a||b  \    flip    /___ar___\
             *     p0\   ||   /p1   =>   p0\---bl---/p1
             *        \  ||  /              \      /
             *       ar\ || /br             b\    /br
             *          \||/                  \  /
             *           pr                    pr
             */
            const a0 = a - a % 3;
            ar = a0 + (a + 2) % 3;

            if (b === -1) { // convex hull edge
                if (i === 0) break;
                a = EDGE_STACK[--i];
                continue;
            }

            const b0 = b - b % 3;
            const al = a0 + (a + 1) % 3;
            const bl = b0 + (b + 2) % 3;

            const p0 = triangles[ar];
            const pr = triangles[a];
            const pl = triangles[al];
            const p1 = triangles[bl];

            const illegal = inCircle(
                coords[2 * p0], coords[2 * p0 + 1],
                coords[2 * pr], coords[2 * pr + 1],
                coords[2 * pl], coords[2 * pl + 1],
                coords[2 * p1], coords[2 * p1 + 1]);

            if (illegal) {
                triangles[a] = p1;
                triangles[b] = p0;

                const hbl = halfedges[bl];

                // edge swapped on the other side of the hull (rare); fix the halfedge reference
                if (hbl === -1) {
                    let e = this._hullStart;
                    do {
                        if (this._hullTri[e] === bl) {
                            this._hullTri[e] = a;
                            break;
                        }
                        e = this._hullPrev[e];
                    } while (e !== this._hullStart);
                }
                this._link(a, hbl);
                this._link(b, halfedges[ar]);
                this._link(ar, bl);

                const br = b0 + (b + 1) % 3;

                // don't worry about hitting the cap: it can only happen on extremely degenerate input
                if (i < EDGE_STACK.length) {
                    EDGE_STACK[i++] = br;
                }
            } else {
                if (i === 0) break;
                a = EDGE_STACK[--i];
            }
        }

        return ar;
    }

    _link(a, b) {
        this._halfedges[a] = b;
        if (b !== -1) this._halfedges[b] = a;
    }

    // add a new triangle given vertex indices and adjacent half-edge ids
    _addTriangle(i0, i1, i2, a, b, c) {
        const t = this.trianglesLen;

        this._triangles[t] = i0;
        this._triangles[t + 1] = i1;
        this._triangles[t + 2] = i2;

        this._link(t, a);
        this._link(t + 1, b);
        this._link(t + 2, c);

        this.trianglesLen += 3;

        return t;
    }
}

// monotonically increases with real angle, but doesn't need expensive trigonometry
function pseudoAngle(dx, dy) {
    const p = dx / (Math.abs(dx) + Math.abs(dy));
    return (dy > 0 ? 3 - p : 1 + p) / 4; // [0..1]
}

function dist(ax, ay, bx, by) {
    const dx = ax - bx;
    const dy = ay - by;
    return dx * dx + dy * dy;
}

function inCircle(ax, ay, bx, by, cx, cy, px, py) {
    const dx = ax - px;
    const dy = ay - py;
    const ex = bx - px;
    const ey = by - py;
    const fx = cx - px;
    const fy = cy - py;

    const ap = dx * dx + dy * dy;
    const bp = ex * ex + ey * ey;
    const cp = fx * fx + fy * fy;

    return dx * (ey * cp - bp * fy) -
           dy * (ex * cp - bp * fx) +
           ap * (ex * fy - ey * fx) < 0;
}

function circumradius(ax, ay, bx, by, cx, cy) {
    const dx = bx - ax;
    const dy = by - ay;
    const ex = cx - ax;
    const ey = cy - ay;

    const bl = dx * dx + dy * dy;
    const cl = ex * ex + ey * ey;
    const d = 0.5 / (dx * ey - dy * ex);

    const x = (ey * bl - dy * cl) * d;
    const y = (dx * cl - ex * bl) * d;

    return x * x + y * y;
}

function circumcenter(ax, ay, bx, by, cx, cy) {
    const dx = bx - ax;
    const dy = by - ay;
    const ex = cx - ax;
    const ey = cy - ay;

    const bl = dx * dx + dy * dy;
    const cl = ex * ex + ey * ey;
    const d = 0.5 / (dx * ey - dy * ex);

    const x = ax + (ey * bl - dy * cl) * d;
    const y = ay + (dx * cl - ex * bl) * d;

    return {x, y};
}

function quicksort(ids, dists, left, right) {
    if (right - left <= 20) {
        for (let i = left + 1; i <= right; i++) {
            const temp = ids[i];
            const tempDist = dists[temp];
            let j = i - 1;
            while (j >= left && dists[ids[j]] > tempDist) ids[j + 1] = ids[j--];
            ids[j + 1] = temp;
        }
    } else {
        const median = (left + right) >> 1;
        let i = left + 1;
        let j = right;
        swap(ids, median, i);
        if (dists[ids[left]] > dists[ids[right]]) swap(ids, left, right);
        if (dists[ids[i]] > dists[ids[right]]) swap(ids, i, right);
        if (dists[ids[left]] > dists[ids[i]]) swap(ids, left, i);

        const temp = ids[i];
        const tempDist = dists[temp];
        while (true) {
            do i++; while (dists[ids[i]] < tempDist);
            do j--; while (dists[ids[j]] > tempDist);
            if (j < i) break;
            swap(ids, i, j);
        }
        ids[left + 1] = ids[j];
        ids[j] = temp;

        if (right - i + 1 >= j - left) {
            quicksort(ids, dists, i, right);
            quicksort(ids, dists, left, j - 1);
        } else {
            quicksort(ids, dists, left, j - 1);
            quicksort(ids, dists, i, right);
        }
    }
}

function swap(arr, i, j) {
    const tmp = arr[i];
    arr[i] = arr[j];
    arr[j] = tmp;
}

function defaultGetX(p) {
    return p[0];
}
function defaultGetY(p) {
    return p[1];
}

const epsilon$2 = 1e-6;

class Path {
  constructor() {
    this._x0 = this._y0 = // start of current subpath
    this._x1 = this._y1 = null; // end of current subpath
    this._ = "";
  }
  moveTo(x, y) {
    this._ += `M${this._x0 = this._x1 = +x},${this._y0 = this._y1 = +y}`;
  }
  closePath() {
    if (this._x1 !== null) {
      this._x1 = this._x0, this._y1 = this._y0;
      this._ += "Z";
    }
  }
  lineTo(x, y) {
    this._ += `L${this._x1 = +x},${this._y1 = +y}`;
  }
  arc(x, y, r) {
    x = +x, y = +y, r = +r;
    const x0 = x + r;
    const y0 = y;
    if (r < 0) throw new Error("negative radius");
    if (this._x1 === null) this._ += `M${x0},${y0}`;
    else if (Math.abs(this._x1 - x0) > epsilon$2 || Math.abs(this._y1 - y0) > epsilon$2) this._ += "L" + x0 + "," + y0;
    if (!r) return;
    this._ += `A${r},${r},0,1,1,${x - r},${y}A${r},${r},0,1,1,${this._x1 = x0},${this._y1 = y0}`;
  }
  rect(x, y, w, h) {
    this._ += `M${this._x0 = this._x1 = +x},${this._y0 = this._y1 = +y}h${+w}v${+h}h${-w}Z`;
  }
  value() {
    return this._ || null;
  }
}

class Polygon {
  constructor() {
    this._ = [];
  }
  moveTo(x, y) {
    this._.push([x, y]);
  }
  closePath() {
    this._.push(this._[0].slice());
  }
  lineTo(x, y) {
    this._.push([x, y]);
  }
  value() {
    return this._.length ? this._ : null;
  }
}

class Voronoi {
  constructor(delaunay, [xmin, ymin, xmax, ymax] = [0, 0, 960, 500]) {
    if (!((xmax = +xmax) >= (xmin = +xmin)) || !((ymax = +ymax) >= (ymin = +ymin))) throw new Error("invalid bounds");
    this.delaunay = delaunay;
    this._circumcenters = new Float64Array(delaunay.points.length * 2);
    this.vectors = new Float64Array(delaunay.points.length * 2);
    this.xmax = xmax, this.xmin = xmin;
    this.ymax = ymax, this.ymin = ymin;
    this._init();
  }
  update() {
    this.delaunay.update();
    this._init();
    return this;
  }
  _init() {
    const {delaunay: {points, hull, triangles}, vectors} = this;
    let bx, by; // lazily computed barycenter of the hull

    // Compute circumcenters.
    const circumcenters = this.circumcenters = this._circumcenters.subarray(0, triangles.length / 3 * 2);
    for (let i = 0, j = 0, n = triangles.length, x, y; i < n; i += 3, j += 2) {
      const t1 = triangles[i] * 2;
      const t2 = triangles[i + 1] * 2;
      const t3 = triangles[i + 2] * 2;
      const x1 = points[t1];
      const y1 = points[t1 + 1];
      const x2 = points[t2];
      const y2 = points[t2 + 1];
      const x3 = points[t3];
      const y3 = points[t3 + 1];

      const dx = x2 - x1;
      const dy = y2 - y1;
      const ex = x3 - x1;
      const ey = y3 - y1;
      const ab = (dx * ey - dy * ex) * 2;

      if (Math.abs(ab) < 1e-9) {
        // For a degenerate triangle, the circumcenter is at the infinity, in a
        // direction orthogonal to the halfedge and away from the “center” of
        // the diagram <bx, by>, defined as the hull’s barycenter.
        if (bx === undefined) {
          bx = by = 0;
          for (const i of hull) bx += points[i * 2], by += points[i * 2 + 1];
          bx /= hull.length, by /= hull.length;
        }
        const a = 1e9 * Math.sign((bx - x1) * ey - (by - y1) * ex);
        x = (x1 + x3) / 2 - a * ey;
        y = (y1 + y3) / 2 + a * ex;
      } else {
        const d = 1 / ab;
        const bl = dx * dx + dy * dy;
        const cl = ex * ex + ey * ey;
        x = x1 + (ey * bl - dy * cl) * d;
        y = y1 + (dx * cl - ex * bl) * d;
      }
      circumcenters[j] = x;
      circumcenters[j + 1] = y;
    }

    // Compute exterior cell rays.
    let h = hull[hull.length - 1];
    let p0, p1 = h * 4;
    let x0, x1 = points[2 * h];
    let y0, y1 = points[2 * h + 1];
    vectors.fill(0);
    for (let i = 0; i < hull.length; ++i) {
      h = hull[i];
      p0 = p1, x0 = x1, y0 = y1;
      p1 = h * 4, x1 = points[2 * h], y1 = points[2 * h + 1];
      vectors[p0 + 2] = vectors[p1] = y0 - y1;
      vectors[p0 + 3] = vectors[p1 + 1] = x1 - x0;
    }
  }
  render(context) {
    const buffer = context == null ? context = new Path : undefined;
    const {delaunay: {halfedges, inedges, hull}, circumcenters, vectors} = this;
    if (hull.length <= 1) return null;
    for (let i = 0, n = halfedges.length; i < n; ++i) {
      const j = halfedges[i];
      if (j < i) continue;
      const ti = Math.floor(i / 3) * 2;
      const tj = Math.floor(j / 3) * 2;
      const xi = circumcenters[ti];
      const yi = circumcenters[ti + 1];
      const xj = circumcenters[tj];
      const yj = circumcenters[tj + 1];
      this._renderSegment(xi, yi, xj, yj, context);
    }
    let h0, h1 = hull[hull.length - 1];
    for (let i = 0; i < hull.length; ++i) {
      h0 = h1, h1 = hull[i];
      const t = Math.floor(inedges[h1] / 3) * 2;
      const x = circumcenters[t];
      const y = circumcenters[t + 1];
      const v = h0 * 4;
      const p = this._project(x, y, vectors[v + 2], vectors[v + 3]);
      if (p) this._renderSegment(x, y, p[0], p[1], context);
    }
    return buffer && buffer.value();
  }
  renderBounds(context) {
    const buffer = context == null ? context = new Path : undefined;
    context.rect(this.xmin, this.ymin, this.xmax - this.xmin, this.ymax - this.ymin);
    return buffer && buffer.value();
  }
  renderCell(i, context) {
    const buffer = context == null ? context = new Path : undefined;
    const points = this._clip(i);
    if (points === null || !points.length) return;
    context.moveTo(points[0], points[1]);
    let n = points.length;
    while (points[0] === points[n-2] && points[1] === points[n-1] && n > 1) n -= 2;
    for (let i = 2; i < n; i += 2) {
      if (points[i] !== points[i-2] || points[i+1] !== points[i-1])
        context.lineTo(points[i], points[i + 1]);
    }
    context.closePath();
    return buffer && buffer.value();
  }
  *cellPolygons() {
    const {delaunay: {points}} = this;
    for (let i = 0, n = points.length / 2; i < n; ++i) {
      const cell = this.cellPolygon(i);
      if (cell) cell.index = i, yield cell;
    }
  }
  cellPolygon(i) {
    const polygon = new Polygon;
    this.renderCell(i, polygon);
    return polygon.value();
  }
  _renderSegment(x0, y0, x1, y1, context) {
    let S;
    const c0 = this._regioncode(x0, y0);
    const c1 = this._regioncode(x1, y1);
    if (c0 === 0 && c1 === 0) {
      context.moveTo(x0, y0);
      context.lineTo(x1, y1);
    } else if (S = this._clipSegment(x0, y0, x1, y1, c0, c1)) {
      context.moveTo(S[0], S[1]);
      context.lineTo(S[2], S[3]);
    }
  }
  contains(i, x, y) {
    if ((x = +x, x !== x) || (y = +y, y !== y)) return false;
    return this.delaunay._step(i, x, y) === i;
  }
  *neighbors(i) {
    const ci = this._clip(i);
    if (ci) for (const j of this.delaunay.neighbors(i)) {
      const cj = this._clip(j);
      // find the common edge
      if (cj) loop: for (let ai = 0, li = ci.length; ai < li; ai += 2) {
        for (let aj = 0, lj = cj.length; aj < lj; aj += 2) {
          if (ci[ai] === cj[aj]
              && ci[ai + 1] === cj[aj + 1]
              && ci[(ai + 2) % li] === cj[(aj + lj - 2) % lj]
              && ci[(ai + 3) % li] === cj[(aj + lj - 1) % lj]) {
            yield j;
            break loop;
          }
        }
      }
    }
  }
  _cell(i) {
    const {circumcenters, delaunay: {inedges, halfedges, triangles}} = this;
    const e0 = inedges[i];
    if (e0 === -1) return null; // coincident point
    const points = [];
    let e = e0;
    do {
      const t = Math.floor(e / 3);
      points.push(circumcenters[t * 2], circumcenters[t * 2 + 1]);
      e = e % 3 === 2 ? e - 2 : e + 1;
      if (triangles[e] !== i) break; // bad triangulation
      e = halfedges[e];
    } while (e !== e0 && e !== -1);
    return points;
  }
  _clip(i) {
    // degenerate case (1 valid point: return the box)
    if (i === 0 && this.delaunay.hull.length === 1) {
      return [this.xmax, this.ymin, this.xmax, this.ymax, this.xmin, this.ymax, this.xmin, this.ymin];
    }
    const points = this._cell(i);
    if (points === null) return null;
    const {vectors: V} = this;
    const v = i * 4;
    return this._simplify(V[v] || V[v + 1]
        ? this._clipInfinite(i, points, V[v], V[v + 1], V[v + 2], V[v + 3])
        : this._clipFinite(i, points));
  }
  _clipFinite(i, points) {
    const n = points.length;
    let P = null;
    let x0, y0, x1 = points[n - 2], y1 = points[n - 1];
    let c0, c1 = this._regioncode(x1, y1);
    let e0, e1 = 0;
    for (let j = 0; j < n; j += 2) {
      x0 = x1, y0 = y1, x1 = points[j], y1 = points[j + 1];
      c0 = c1, c1 = this._regioncode(x1, y1);
      if (c0 === 0 && c1 === 0) {
        e0 = e1, e1 = 0;
        if (P) P.push(x1, y1);
        else P = [x1, y1];
      } else {
        let S, sx0, sy0, sx1, sy1;
        if (c0 === 0) {
          if ((S = this._clipSegment(x0, y0, x1, y1, c0, c1)) === null) continue;
          [sx0, sy0, sx1, sy1] = S;
        } else {
          if ((S = this._clipSegment(x1, y1, x0, y0, c1, c0)) === null) continue;
          [sx1, sy1, sx0, sy0] = S;
          e0 = e1, e1 = this._edgecode(sx0, sy0);
          if (e0 && e1) this._edge(i, e0, e1, P, P.length);
          if (P) P.push(sx0, sy0);
          else P = [sx0, sy0];
        }
        e0 = e1, e1 = this._edgecode(sx1, sy1);
        if (e0 && e1) this._edge(i, e0, e1, P, P.length);
        if (P) P.push(sx1, sy1);
        else P = [sx1, sy1];
      }
    }
    if (P) {
      e0 = e1, e1 = this._edgecode(P[0], P[1]);
      if (e0 && e1) this._edge(i, e0, e1, P, P.length);
    } else if (this.contains(i, (this.xmin + this.xmax) / 2, (this.ymin + this.ymax) / 2)) {
      return [this.xmax, this.ymin, this.xmax, this.ymax, this.xmin, this.ymax, this.xmin, this.ymin];
    }
    return P;
  }
  _clipSegment(x0, y0, x1, y1, c0, c1) {
    // for more robustness, always consider the segment in the same order
    const flip = c0 < c1;
    if (flip) [x0, y0, x1, y1, c0, c1] = [x1, y1, x0, y0, c1, c0];
    while (true) {
      if (c0 === 0 && c1 === 0) return flip ? [x1, y1, x0, y0] : [x0, y0, x1, y1];
      if (c0 & c1) return null;
      let x, y, c = c0 || c1;
      if (c & 0b1000) x = x0 + (x1 - x0) * (this.ymax - y0) / (y1 - y0), y = this.ymax;
      else if (c & 0b0100) x = x0 + (x1 - x0) * (this.ymin - y0) / (y1 - y0), y = this.ymin;
      else if (c & 0b0010) y = y0 + (y1 - y0) * (this.xmax - x0) / (x1 - x0), x = this.xmax;
      else y = y0 + (y1 - y0) * (this.xmin - x0) / (x1 - x0), x = this.xmin;
      if (c0) x0 = x, y0 = y, c0 = this._regioncode(x0, y0);
      else x1 = x, y1 = y, c1 = this._regioncode(x1, y1);
    }
  }
  _clipInfinite(i, points, vx0, vy0, vxn, vyn) {
    let P = Array.from(points), p;
    if (p = this._project(P[0], P[1], vx0, vy0)) P.unshift(p[0], p[1]);
    if (p = this._project(P[P.length - 2], P[P.length - 1], vxn, vyn)) P.push(p[0], p[1]);
    if (P = this._clipFinite(i, P)) {
      for (let j = 0, n = P.length, c0, c1 = this._edgecode(P[n - 2], P[n - 1]); j < n; j += 2) {
        c0 = c1, c1 = this._edgecode(P[j], P[j + 1]);
        if (c0 && c1) j = this._edge(i, c0, c1, P, j), n = P.length;
      }
    } else if (this.contains(i, (this.xmin + this.xmax) / 2, (this.ymin + this.ymax) / 2)) {
      P = [this.xmin, this.ymin, this.xmax, this.ymin, this.xmax, this.ymax, this.xmin, this.ymax];
    }
    return P;
  }
  _edge(i, e0, e1, P, j) {
    while (e0 !== e1) {
      let x, y;
      switch (e0) {
        case 0b0101: e0 = 0b0100; continue; // top-left
        case 0b0100: e0 = 0b0110, x = this.xmax, y = this.ymin; break; // top
        case 0b0110: e0 = 0b0010; continue; // top-right
        case 0b0010: e0 = 0b1010, x = this.xmax, y = this.ymax; break; // right
        case 0b1010: e0 = 0b1000; continue; // bottom-right
        case 0b1000: e0 = 0b1001, x = this.xmin, y = this.ymax; break; // bottom
        case 0b1001: e0 = 0b0001; continue; // bottom-left
        case 0b0001: e0 = 0b0101, x = this.xmin, y = this.ymin; break; // left
      }
      // Note: this implicitly checks for out of bounds: if P[j] or P[j+1] are
      // undefined, the conditional statement will be executed.
      if ((P[j] !== x || P[j + 1] !== y) && this.contains(i, x, y)) {
        P.splice(j, 0, x, y), j += 2;
      }
    }
    return j;
  }
  _project(x0, y0, vx, vy) {
    let t = Infinity, c, x, y;
    if (vy < 0) { // top
      if (y0 <= this.ymin) return null;
      if ((c = (this.ymin - y0) / vy) < t) y = this.ymin, x = x0 + (t = c) * vx;
    } else if (vy > 0) { // bottom
      if (y0 >= this.ymax) return null;
      if ((c = (this.ymax - y0) / vy) < t) y = this.ymax, x = x0 + (t = c) * vx;
    }
    if (vx > 0) { // right
      if (x0 >= this.xmax) return null;
      if ((c = (this.xmax - x0) / vx) < t) x = this.xmax, y = y0 + (t = c) * vy;
    } else if (vx < 0) { // left
      if (x0 <= this.xmin) return null;
      if ((c = (this.xmin - x0) / vx) < t) x = this.xmin, y = y0 + (t = c) * vy;
    }
    return [x, y];
  }
  _edgecode(x, y) {
    return (x === this.xmin ? 0b0001
        : x === this.xmax ? 0b0010 : 0b0000)
        | (y === this.ymin ? 0b0100
        : y === this.ymax ? 0b1000 : 0b0000);
  }
  _regioncode(x, y) {
    return (x < this.xmin ? 0b0001
        : x > this.xmax ? 0b0010 : 0b0000)
        | (y < this.ymin ? 0b0100
        : y > this.ymax ? 0b1000 : 0b0000);
  }
  _simplify(P) {
    if (P && P.length > 4) {
      for (let i = 0; i < P.length; i+= 2) {
        const j = (i + 2) % P.length, k = (i + 4) % P.length;
        if (P[i] === P[j] && P[j] === P[k] || P[i + 1] === P[j + 1] && P[j + 1] === P[k + 1]) {
          P.splice(j, 2), i -= 2;
        }
      }
      if (!P.length) P = null;
    }
    return P;
  }
}

const tau$2 = 2 * Math.PI, pow$2 = Math.pow;

function pointX(p) {
  return p[0];
}

function pointY(p) {
  return p[1];
}

// A triangulation is collinear if all its triangles have a non-null area
function collinear(d) {
  const {triangles, coords} = d;
  for (let i = 0; i < triangles.length; i += 3) {
    const a = 2 * triangles[i],
          b = 2 * triangles[i + 1],
          c = 2 * triangles[i + 2],
          cross = (coords[c] - coords[a]) * (coords[b + 1] - coords[a + 1])
                - (coords[b] - coords[a]) * (coords[c + 1] - coords[a + 1]);
    if (cross > 1e-10) return false;
  }
  return true;
}

function jitter(x, y, r) {
  return [x + Math.sin(x + y) * r, y + Math.cos(x - y) * r];
}

class Delaunay {
  static from(points, fx = pointX, fy = pointY, that) {
    return new Delaunay("length" in points
        ? flatArray(points, fx, fy, that)
        : Float64Array.from(flatIterable(points, fx, fy, that)));
  }
  constructor(points) {
    this._delaunator = new Delaunator(points);
    this.inedges = new Int32Array(points.length / 2);
    this._hullIndex = new Int32Array(points.length / 2);
    this.points = this._delaunator.coords;
    this._init();
  }
  update() {
    this._delaunator.update();
    this._init();
    return this;
  }
  _init() {
    const d = this._delaunator, points = this.points;

    // check for collinear
    if (d.hull && d.hull.length > 2 && collinear(d)) {
      this.collinear = Int32Array.from({length: points.length/2}, (_,i) => i)
        .sort((i, j) => points[2 * i] - points[2 * j] || points[2 * i + 1] - points[2 * j + 1]); // for exact neighbors
      const e = this.collinear[0], f = this.collinear[this.collinear.length - 1],
        bounds = [ points[2 * e], points[2 * e + 1], points[2 * f], points[2 * f + 1] ],
        r = 1e-8 * Math.hypot(bounds[3] - bounds[1], bounds[2] - bounds[0]);
      for (let i = 0, n = points.length / 2; i < n; ++i) {
        const p = jitter(points[2 * i], points[2 * i + 1], r);
        points[2 * i] = p[0];
        points[2 * i + 1] = p[1];
      }
      this._delaunator = new Delaunator(points);
    } else {
      delete this.collinear;
    }

    const halfedges = this.halfedges = this._delaunator.halfedges;
    const hull = this.hull = this._delaunator.hull;
    const triangles = this.triangles = this._delaunator.triangles;
    const inedges = this.inedges.fill(-1);
    const hullIndex = this._hullIndex.fill(-1);

    // Compute an index from each point to an (arbitrary) incoming halfedge
    // Used to give the first neighbor of each point; for this reason,
    // on the hull we give priority to exterior halfedges
    for (let e = 0, n = halfedges.length; e < n; ++e) {
      const p = triangles[e % 3 === 2 ? e - 2 : e + 1];
      if (halfedges[e] === -1 || inedges[p] === -1) inedges[p] = e;
    }
    for (let i = 0, n = hull.length; i < n; ++i) {
      hullIndex[hull[i]] = i;
    }

    // degenerate case: 1 or 2 (distinct) points
    if (hull.length <= 2 && hull.length > 0) {
      this.triangles = new Int32Array(3).fill(-1);
      this.halfedges = new Int32Array(3).fill(-1);
      this.triangles[0] = hull[0];
      inedges[hull[0]] = 1;
      if (hull.length === 2) {
        inedges[hull[1]] = 0;
        this.triangles[1] = hull[1];
        this.triangles[2] = hull[1];
      }
    }
  }
  voronoi(bounds) {
    return new Voronoi(this, bounds);
  }
  *neighbors(i) {
    const {inedges, hull, _hullIndex, halfedges, triangles, collinear} = this;

    // degenerate case with several collinear points
    if (collinear) {
      const l = collinear.indexOf(i);
      if (l > 0) yield collinear[l - 1];
      if (l < collinear.length - 1) yield collinear[l + 1];
      return;
    }

    const e0 = inedges[i];
    if (e0 === -1) return; // coincident point
    let e = e0, p0 = -1;
    do {
      yield p0 = triangles[e];
      e = e % 3 === 2 ? e - 2 : e + 1;
      if (triangles[e] !== i) return; // bad triangulation
      e = halfedges[e];
      if (e === -1) {
        const p = hull[(_hullIndex[i] + 1) % hull.length];
        if (p !== p0) yield p;
        return;
      }
    } while (e !== e0);
  }
  find(x, y, i = 0) {
    if ((x = +x, x !== x) || (y = +y, y !== y)) return -1;
    const i0 = i;
    let c;
    while ((c = this._step(i, x, y)) >= 0 && c !== i && c !== i0) i = c;
    return c;
  }
  _step(i, x, y) {
    const {inedges, hull, _hullIndex, halfedges, triangles, points} = this;
    if (inedges[i] === -1 || !points.length) return (i + 1) % (points.length >> 1);
    let c = i;
    let dc = pow$2(x - points[i * 2], 2) + pow$2(y - points[i * 2 + 1], 2);
    const e0 = inedges[i];
    let e = e0;
    do {
      let t = triangles[e];
      const dt = pow$2(x - points[t * 2], 2) + pow$2(y - points[t * 2 + 1], 2);
      if (dt < dc) dc = dt, c = t;
      e = e % 3 === 2 ? e - 2 : e + 1;
      if (triangles[e] !== i) break; // bad triangulation
      e = halfedges[e];
      if (e === -1) {
        e = hull[(_hullIndex[i] + 1) % hull.length];
        if (e !== t) {
          if (pow$2(x - points[e * 2], 2) + pow$2(y - points[e * 2 + 1], 2) < dc) return e;
        }
        break;
      }
    } while (e !== e0);
    return c;
  }
  render(context) {
    const buffer = context == null ? context = new Path : undefined;
    const {points, halfedges, triangles} = this;
    for (let i = 0, n = halfedges.length; i < n; ++i) {
      const j = halfedges[i];
      if (j < i) continue;
      const ti = triangles[i] * 2;
      const tj = triangles[j] * 2;
      context.moveTo(points[ti], points[ti + 1]);
      context.lineTo(points[tj], points[tj + 1]);
    }
    this.renderHull(context);
    return buffer && buffer.value();
  }
  renderPoints(context, r) {
    if (r === undefined && (!context || typeof context.moveTo !== "function")) r = context, context = null;
    r = r == undefined ? 2 : +r;
    const buffer = context == null ? context = new Path : undefined;
    const {points} = this;
    for (let i = 0, n = points.length; i < n; i += 2) {
      const x = points[i], y = points[i + 1];
      context.moveTo(x + r, y);
      context.arc(x, y, r, 0, tau$2);
    }
    return buffer && buffer.value();
  }
  renderHull(context) {
    const buffer = context == null ? context = new Path : undefined;
    const {hull, points} = this;
    const h = hull[0] * 2, n = hull.length;
    context.moveTo(points[h], points[h + 1]);
    for (let i = 1; i < n; ++i) {
      const h = 2 * hull[i];
      context.lineTo(points[h], points[h + 1]);
    }
    context.closePath();
    return buffer && buffer.value();
  }
  hullPolygon() {
    const polygon = new Polygon;
    this.renderHull(polygon);
    return polygon.value();
  }
  renderTriangle(i, context) {
    const buffer = context == null ? context = new Path : undefined;
    const {points, triangles} = this;
    const t0 = triangles[i *= 3] * 2;
    const t1 = triangles[i + 1] * 2;
    const t2 = triangles[i + 2] * 2;
    context.moveTo(points[t0], points[t0 + 1]);
    context.lineTo(points[t1], points[t1 + 1]);
    context.lineTo(points[t2], points[t2 + 1]);
    context.closePath();
    return buffer && buffer.value();
  }
  *trianglePolygons() {
    const {triangles} = this;
    for (let i = 0, n = triangles.length / 3; i < n; ++i) {
      yield this.trianglePolygon(i);
    }
  }
  trianglePolygon(i) {
    const polygon = new Polygon;
    this.renderTriangle(i, polygon);
    return polygon.value();
  }
}

function flatArray(points, fx, fy, that) {
  const n = points.length;
  const array = new Float64Array(n * 2);
  for (let i = 0; i < n; ++i) {
    const p = points[i];
    array[i * 2] = fx.call(that, p, i, points);
    array[i * 2 + 1] = fy.call(that, p, i, points);
  }
  return array;
}

function* flatIterable(points, fx, fy, that) {
  let i = 0;
  for (const p of points) {
    yield fx.call(that, p, i, points);
    yield fy.call(that, p, i, points);
    ++i;
  }
}

var EOL = {},
    EOF = {},
    QUOTE = 34,
    NEWLINE = 10,
    RETURN = 13;

function objectConverter(columns) {
  return new Function("d", "return {" + columns.map(function(name, i) {
    return JSON.stringify(name) + ": d[" + i + "] || \"\"";
  }).join(",") + "}");
}

function customConverter(columns, f) {
  var object = objectConverter(columns);
  return function(row, i) {
    return f(object(row), i, columns);
  };
}

// Compute unique columns in order of discovery.
function inferColumns(rows) {
  var columnSet = Object.create(null),
      columns = [];

  rows.forEach(function(row) {
    for (var column in row) {
      if (!(column in columnSet)) {
        columns.push(columnSet[column] = column);
      }
    }
  });

  return columns;
}

function pad$1(value, width) {
  var s = value + "", length = s.length;
  return length < width ? new Array(width - length + 1).join(0) + s : s;
}

function formatYear$1(year) {
  return year < 0 ? "-" + pad$1(-year, 6)
    : year > 9999 ? "+" + pad$1(year, 6)
    : pad$1(year, 4);
}

function formatDate(date) {
  var hours = date.getUTCHours(),
      minutes = date.getUTCMinutes(),
      seconds = date.getUTCSeconds(),
      milliseconds = date.getUTCMilliseconds();
  return isNaN(date) ? "Invalid Date"
      : formatYear$1(date.getUTCFullYear(), 4) + "-" + pad$1(date.getUTCMonth() + 1, 2) + "-" + pad$1(date.getUTCDate(), 2)
      + (milliseconds ? "T" + pad$1(hours, 2) + ":" + pad$1(minutes, 2) + ":" + pad$1(seconds, 2) + "." + pad$1(milliseconds, 3) + "Z"
      : seconds ? "T" + pad$1(hours, 2) + ":" + pad$1(minutes, 2) + ":" + pad$1(seconds, 2) + "Z"
      : minutes || hours ? "T" + pad$1(hours, 2) + ":" + pad$1(minutes, 2) + "Z"
      : "");
}

function dsvFormat(delimiter) {
  var reFormat = new RegExp("[\"" + delimiter + "\n\r]"),
      DELIMITER = delimiter.charCodeAt(0);

  function parse(text, f) {
    var convert, columns, rows = parseRows(text, function(row, i) {
      if (convert) return convert(row, i - 1);
      columns = row, convert = f ? customConverter(row, f) : objectConverter(row);
    });
    rows.columns = columns || [];
    return rows;
  }

  function parseRows(text, f) {
    var rows = [], // output rows
        N = text.length,
        I = 0, // current character index
        n = 0, // current line number
        t, // current token
        eof = N <= 0, // current token followed by EOF?
        eol = false; // current token followed by EOL?

    // Strip the trailing newline.
    if (text.charCodeAt(N - 1) === NEWLINE) --N;
    if (text.charCodeAt(N - 1) === RETURN) --N;

    function token() {
      if (eof) return EOF;
      if (eol) return eol = false, EOL;

      // Unescape quotes.
      var i, j = I, c;
      if (text.charCodeAt(j) === QUOTE) {
        while (I++ < N && text.charCodeAt(I) !== QUOTE || text.charCodeAt(++I) === QUOTE);
        if ((i = I) >= N) eof = true;
        else if ((c = text.charCodeAt(I++)) === NEWLINE) eol = true;
        else if (c === RETURN) { eol = true; if (text.charCodeAt(I) === NEWLINE) ++I; }
        return text.slice(j + 1, i - 1).replace(/""/g, "\"");
      }

      // Find next delimiter or newline.
      while (I < N) {
        if ((c = text.charCodeAt(i = I++)) === NEWLINE) eol = true;
        else if (c === RETURN) { eol = true; if (text.charCodeAt(I) === NEWLINE) ++I; }
        else if (c !== DELIMITER) continue;
        return text.slice(j, i);
      }

      // Return last token before EOF.
      return eof = true, text.slice(j, N);
    }

    while ((t = token()) !== EOF) {
      var row = [];
      while (t !== EOL && t !== EOF) row.push(t), t = token();
      if (f && (row = f(row, n++)) == null) continue;
      rows.push(row);
    }

    return rows;
  }

  function preformatBody(rows, columns) {
    return rows.map(function(row) {
      return columns.map(function(column) {
        return formatValue(row[column]);
      }).join(delimiter);
    });
  }

  function format(rows, columns) {
    if (columns == null) columns = inferColumns(rows);
    return [columns.map(formatValue).join(delimiter)].concat(preformatBody(rows, columns)).join("\n");
  }

  function formatBody(rows, columns) {
    if (columns == null) columns = inferColumns(rows);
    return preformatBody(rows, columns).join("\n");
  }

  function formatRows(rows) {
    return rows.map(formatRow).join("\n");
  }

  function formatRow(row) {
    return row.map(formatValue).join(delimiter);
  }

  function formatValue(value) {
    return value == null ? ""
        : value instanceof Date ? formatDate(value)
        : reFormat.test(value += "") ? "\"" + value.replace(/"/g, "\"\"") + "\""
        : value;
  }

  return {
    parse: parse,
    parseRows: parseRows,
    format: format,
    formatBody: formatBody,
    formatRows: formatRows,
    formatRow: formatRow,
    formatValue: formatValue
  };
}

var csv$1 = dsvFormat(",");

var csvParse = csv$1.parse;
var csvParseRows = csv$1.parseRows;
var csvFormat = csv$1.format;
var csvFormatBody = csv$1.formatBody;
var csvFormatRows = csv$1.formatRows;
var csvFormatRow = csv$1.formatRow;
var csvFormatValue = csv$1.formatValue;

var tsv$1 = dsvFormat("\t");

var tsvParse = tsv$1.parse;
var tsvParseRows = tsv$1.parseRows;
var tsvFormat = tsv$1.format;
var tsvFormatBody = tsv$1.formatBody;
var tsvFormatRows = tsv$1.formatRows;
var tsvFormatRow = tsv$1.formatRow;
var tsvFormatValue = tsv$1.formatValue;

function autoType(object) {
  for (var key in object) {
    var value = object[key].trim(), number, m;
    if (!value) value = null;
    else if (value === "true") value = true;
    else if (value === "false") value = false;
    else if (value === "NaN") value = NaN;
    else if (!isNaN(number = +value)) value = number;
    else if (m = value.match(/^([-+]\d{2})?\d{4}(-\d{2}(-\d{2})?)?(T\d{2}:\d{2}(:\d{2}(\.\d{3})?)?(Z|[-+]\d{2}:\d{2})?)?$/)) {
      if (fixtz && !!m[4] && !m[7]) value = value.replace(/-/g, "/").replace(/T/, " ");
      value = new Date(value);
    }
    else continue;
    object[key] = value;
  }
  return object;
}

// https://github.com/d3/d3-dsv/issues/45
const fixtz = new Date("2019-01-01T00:00").getHours() || new Date("2019-07-01T00:00").getHours();

function responseBlob(response) {
  if (!response.ok) throw new Error(response.status + " " + response.statusText);
  return response.blob();
}

function blob(input, init) {
  return fetch(input, init).then(responseBlob);
}

function responseArrayBuffer(response) {
  if (!response.ok) throw new Error(response.status + " " + response.statusText);
  return response.arrayBuffer();
}

function buffer(input, init) {
  return fetch(input, init).then(responseArrayBuffer);
}

function responseText(response) {
  if (!response.ok) throw new Error(response.status + " " + response.statusText);
  return response.text();
}

function text(input, init) {
  return fetch(input, init).then(responseText);
}

function dsvParse(parse) {
  return function(input, init, row) {
    if (arguments.length === 2 && typeof init === "function") row = init, init = undefined;
    return text(input, init).then(function(response) {
      return parse(response, row);
    });
  };
}

function dsv(delimiter, input, init, row) {
  if (arguments.length === 3 && typeof init === "function") row = init, init = undefined;
  var format = dsvFormat(delimiter);
  return text(input, init).then(function(response) {
    return format.parse(response, row);
  });
}

var csv = dsvParse(csvParse);
var tsv = dsvParse(tsvParse);

function image(input, init) {
  return new Promise(function(resolve, reject) {
    var image = new Image;
    for (var key in init) image[key] = init[key];
    image.onerror = reject;
    image.onload = function() { resolve(image); };
    image.src = input;
  });
}

function responseJson(response) {
  if (!response.ok) throw new Error(response.status + " " + response.statusText);
  if (response.status === 204 || response.status === 205) return;
  return response.json();
}

function json(input, init) {
  return fetch(input, init).then(responseJson);
}

function parser(type) {
  return (input, init) => text(input, init)
    .then(text => (new DOMParser).parseFromString(text, type));
}

const xml = parser("application/xml");

var html = parser("text/html");

var svg = parser("image/svg+xml");

function center(x, y) {
  var nodes, strength = 1;

  if (x == null) x = 0;
  if (y == null) y = 0;

  function force() {
    var i,
        n = nodes.length,
        node,
        sx = 0,
        sy = 0;

    for (i = 0; i < n; ++i) {
      node = nodes[i], sx += node.x, sy += node.y;
    }

    for (sx = (sx / n - x) * strength, sy = (sy / n - y) * strength, i = 0; i < n; ++i) {
      node = nodes[i], node.x -= sx, node.y -= sy;
    }
  }

  force.initialize = function(_) {
    nodes = _;
  };

  force.x = function(_) {
    return arguments.length ? (x = +_, force) : x;
  };

  force.y = function(_) {
    return arguments.length ? (y = +_, force) : y;
  };

  force.strength = function(_) {
    return arguments.length ? (strength = +_, force) : strength;
  };

  return force;
}

function tree_add(d) {
  const x = +this._x.call(null, d),
      y = +this._y.call(null, d);
  return add(this.cover(x, y), x, y, d);
}

function add(tree, x, y, d) {
  if (isNaN(x) || isNaN(y)) return tree; // ignore invalid points

  var parent,
      node = tree._root,
      leaf = {data: d},
      x0 = tree._x0,
      y0 = tree._y0,
      x1 = tree._x1,
      y1 = tree._y1,
      xm,
      ym,
      xp,
      yp,
      right,
      bottom,
      i,
      j;

  // If the tree is empty, initialize the root as a leaf.
  if (!node) return tree._root = leaf, tree;

  // Find the existing leaf for the new point, or add it.
  while (node.length) {
    if (right = x >= (xm = (x0 + x1) / 2)) x0 = xm; else x1 = xm;
    if (bottom = y >= (ym = (y0 + y1) / 2)) y0 = ym; else y1 = ym;
    if (parent = node, !(node = node[i = bottom << 1 | right])) return parent[i] = leaf, tree;
  }

  // Is the new point is exactly coincident with the existing point?
  xp = +tree._x.call(null, node.data);
  yp = +tree._y.call(null, node.data);
  if (x === xp && y === yp) return leaf.next = node, parent ? parent[i] = leaf : tree._root = leaf, tree;

  // Otherwise, split the leaf node until the old and new point are separated.
  do {
    parent = parent ? parent[i] = new Array(4) : tree._root = new Array(4);
    if (right = x >= (xm = (x0 + x1) / 2)) x0 = xm; else x1 = xm;
    if (bottom = y >= (ym = (y0 + y1) / 2)) y0 = ym; else y1 = ym;
  } while ((i = bottom << 1 | right) === (j = (yp >= ym) << 1 | (xp >= xm)));
  return parent[j] = node, parent[i] = leaf, tree;
}

function addAll(data) {
  var d, i, n = data.length,
      x,
      y,
      xz = new Array(n),
      yz = new Array(n),
      x0 = Infinity,
      y0 = Infinity,
      x1 = -Infinity,
      y1 = -Infinity;

  // Compute the points and their extent.
  for (i = 0; i < n; ++i) {
    if (isNaN(x = +this._x.call(null, d = data[i])) || isNaN(y = +this._y.call(null, d))) continue;
    xz[i] = x;
    yz[i] = y;
    if (x < x0) x0 = x;
    if (x > x1) x1 = x;
    if (y < y0) y0 = y;
    if (y > y1) y1 = y;
  }

  // If there were no (valid) points, abort.
  if (x0 > x1 || y0 > y1) return this;

  // Expand the tree to cover the new points.
  this.cover(x0, y0).cover(x1, y1);

  // Add the new points.
  for (i = 0; i < n; ++i) {
    add(this, xz[i], yz[i], data[i]);
  }

  return this;
}

function tree_cover(x, y) {
  if (isNaN(x = +x) || isNaN(y = +y)) return this; // ignore invalid points

  var x0 = this._x0,
      y0 = this._y0,
      x1 = this._x1,
      y1 = this._y1;

  // If the quadtree has no extent, initialize them.
  // Integer extent are necessary so that if we later double the extent,
  // the existing quadrant boundaries don’t change due to floating point error!
  if (isNaN(x0)) {
    x1 = (x0 = Math.floor(x)) + 1;
    y1 = (y0 = Math.floor(y)) + 1;
  }

  // Otherwise, double repeatedly to cover.
  else {
    var z = x1 - x0 || 1,
        node = this._root,
        parent,
        i;

    while (x0 > x || x >= x1 || y0 > y || y >= y1) {
      i = (y < y0) << 1 | (x < x0);
      parent = new Array(4), parent[i] = node, node = parent, z *= 2;
      switch (i) {
        case 0: x1 = x0 + z, y1 = y0 + z; break;
        case 1: x0 = x1 - z, y1 = y0 + z; break;
        case 2: x1 = x0 + z, y0 = y1 - z; break;
        case 3: x0 = x1 - z, y0 = y1 - z; break;
      }
    }

    if (this._root && this._root.length) this._root = node;
  }

  this._x0 = x0;
  this._y0 = y0;
  this._x1 = x1;
  this._y1 = y1;
  return this;
}

function tree_data() {
  var data = [];
  this.visit(function(node) {
    if (!node.length) do data.push(node.data); while (node = node.next)
  });
  return data;
}

function tree_extent(_) {
  return arguments.length
      ? this.cover(+_[0][0], +_[0][1]).cover(+_[1][0], +_[1][1])
      : isNaN(this._x0) ? undefined : [[this._x0, this._y0], [this._x1, this._y1]];
}

function Quad(node, x0, y0, x1, y1) {
  this.node = node;
  this.x0 = x0;
  this.y0 = y0;
  this.x1 = x1;
  this.y1 = y1;
}

function tree_find(x, y, radius) {
  var data,
      x0 = this._x0,
      y0 = this._y0,
      x1,
      y1,
      x2,
      y2,
      x3 = this._x1,
      y3 = this._y1,
      quads = [],
      node = this._root,
      q,
      i;

  if (node) quads.push(new Quad(node, x0, y0, x3, y3));
  if (radius == null) radius = Infinity;
  else {
    x0 = x - radius, y0 = y - radius;
    x3 = x + radius, y3 = y + radius;
    radius *= radius;
  }

  while (q = quads.pop()) {

    // Stop searching if this quadrant can’t contain a closer node.
    if (!(node = q.node)
        || (x1 = q.x0) > x3
        || (y1 = q.y0) > y3
        || (x2 = q.x1) < x0
        || (y2 = q.y1) < y0) continue;

    // Bisect the current quadrant.
    if (node.length) {
      var xm = (x1 + x2) / 2,
          ym = (y1 + y2) / 2;

      quads.push(
        new Quad(node[3], xm, ym, x2, y2),
        new Quad(node[2], x1, ym, xm, y2),
        new Quad(node[1], xm, y1, x2, ym),
        new Quad(node[0], x1, y1, xm, ym)
      );

      // Visit the closest quadrant first.
      if (i = (y >= ym) << 1 | (x >= xm)) {
        q = quads[quads.length - 1];
        quads[quads.length - 1] = quads[quads.length - 1 - i];
        quads[quads.length - 1 - i] = q;
      }
    }

    // Visit this point. (Visiting coincident points isn’t necessary!)
    else {
      var dx = x - +this._x.call(null, node.data),
          dy = y - +this._y.call(null, node.data),
          d2 = dx * dx + dy * dy;
      if (d2 < radius) {
        var d = Math.sqrt(radius = d2);
        x0 = x - d, y0 = y - d;
        x3 = x + d, y3 = y + d;
        data = node.data;
      }
    }
  }

  return data;
}

function tree_remove(d) {
  if (isNaN(x = +this._x.call(null, d)) || isNaN(y = +this._y.call(null, d))) return this; // ignore invalid points

  var parent,
      node = this._root,
      retainer,
      previous,
      next,
      x0 = this._x0,
      y0 = this._y0,
      x1 = this._x1,
      y1 = this._y1,
      x,
      y,
      xm,
      ym,
      right,
      bottom,
      i,
      j;

  // If the tree is empty, initialize the root as a leaf.
  if (!node) return this;

  // Find the leaf node for the point.
  // While descending, also retain the deepest parent with a non-removed sibling.
  if (node.length) while (true) {
    if (right = x >= (xm = (x0 + x1) / 2)) x0 = xm; else x1 = xm;
    if (bottom = y >= (ym = (y0 + y1) / 2)) y0 = ym; else y1 = ym;
    if (!(parent = node, node = node[i = bottom << 1 | right])) return this;
    if (!node.length) break;
    if (parent[(i + 1) & 3] || parent[(i + 2) & 3] || parent[(i + 3) & 3]) retainer = parent, j = i;
  }

  // Find the point to remove.
  while (node.data !== d) if (!(previous = node, node = node.next)) return this;
  if (next = node.next) delete node.next;

  // If there are multiple coincident points, remove just the point.
  if (previous) return (next ? previous.next = next : delete previous.next), this;

  // If this is the root point, remove it.
  if (!parent) return this._root = next, this;

  // Remove this leaf.
  next ? parent[i] = next : delete parent[i];

  // If the parent now contains exactly one leaf, collapse superfluous parents.
  if ((node = parent[0] || parent[1] || parent[2] || parent[3])
      && node === (parent[3] || parent[2] || parent[1] || parent[0])
      && !node.length) {
    if (retainer) retainer[j] = node;
    else this._root = node;
  }

  return this;
}

function removeAll(data) {
  for (var i = 0, n = data.length; i < n; ++i) this.remove(data[i]);
  return this;
}

function tree_root() {
  return this._root;
}

function tree_size() {
  var size = 0;
  this.visit(function(node) {
    if (!node.length) do ++size; while (node = node.next)
  });
  return size;
}

function tree_visit(callback) {
  var quads = [], q, node = this._root, child, x0, y0, x1, y1;
  if (node) quads.push(new Quad(node, this._x0, this._y0, this._x1, this._y1));
  while (q = quads.pop()) {
    if (!callback(node = q.node, x0 = q.x0, y0 = q.y0, x1 = q.x1, y1 = q.y1) && node.length) {
      var xm = (x0 + x1) / 2, ym = (y0 + y1) / 2;
      if (child = node[3]) quads.push(new Quad(child, xm, ym, x1, y1));
      if (child = node[2]) quads.push(new Quad(child, x0, ym, xm, y1));
      if (child = node[1]) quads.push(new Quad(child, xm, y0, x1, ym));
      if (child = node[0]) quads.push(new Quad(child, x0, y0, xm, ym));
    }
  }
  return this;
}

function tree_visitAfter(callback) {
  var quads = [], next = [], q;
  if (this._root) quads.push(new Quad(this._root, this._x0, this._y0, this._x1, this._y1));
  while (q = quads.pop()) {
    var node = q.node;
    if (node.length) {
      var child, x0 = q.x0, y0 = q.y0, x1 = q.x1, y1 = q.y1, xm = (x0 + x1) / 2, ym = (y0 + y1) / 2;
      if (child = node[0]) quads.push(new Quad(child, x0, y0, xm, ym));
      if (child = node[1]) quads.push(new Quad(child, xm, y0, x1, ym));
      if (child = node[2]) quads.push(new Quad(child, x0, ym, xm, y1));
      if (child = node[3]) quads.push(new Quad(child, xm, ym, x1, y1));
    }
    next.push(q);
  }
  while (q = next.pop()) {
    callback(q.node, q.x0, q.y0, q.x1, q.y1);
  }
  return this;
}

function defaultX(d) {
  return d[0];
}

function tree_x(_) {
  return arguments.length ? (this._x = _, this) : this._x;
}

function defaultY(d) {
  return d[1];
}

function tree_y(_) {
  return arguments.length ? (this._y = _, this) : this._y;
}

function quadtree(nodes, x, y) {
  var tree = new Quadtree(x == null ? defaultX : x, y == null ? defaultY : y, NaN, NaN, NaN, NaN);
  return nodes == null ? tree : tree.addAll(nodes);
}

function Quadtree(x, y, x0, y0, x1, y1) {
  this._x = x;
  this._y = y;
  this._x0 = x0;
  this._y0 = y0;
  this._x1 = x1;
  this._y1 = y1;
  this._root = undefined;
}

function leaf_copy(leaf) {
  var copy = {data: leaf.data}, next = copy;
  while (leaf = leaf.next) next = next.next = {data: leaf.data};
  return copy;
}

var treeProto = quadtree.prototype = Quadtree.prototype;

treeProto.copy = function() {
  var copy = new Quadtree(this._x, this._y, this._x0, this._y0, this._x1, this._y1),
      node = this._root,
      nodes,
      child;

  if (!node) return copy;

  if (!node.length) return copy._root = leaf_copy(node), copy;

  nodes = [{source: node, target: copy._root = new Array(4)}];
  while (node = nodes.pop()) {
    for (var i = 0; i < 4; ++i) {
      if (child = node.source[i]) {
        if (child.length) nodes.push({source: child, target: node.target[i] = new Array(4)});
        else node.target[i] = leaf_copy(child);
      }
    }
  }

  return copy;
};

treeProto.add = tree_add;
treeProto.addAll = addAll;
treeProto.cover = tree_cover;
treeProto.data = tree_data;
treeProto.extent = tree_extent;
treeProto.find = tree_find;
treeProto.remove = tree_remove;
treeProto.removeAll = removeAll;
treeProto.root = tree_root;
treeProto.size = tree_size;
treeProto.visit = tree_visit;
treeProto.visitAfter = tree_visitAfter;
treeProto.x = tree_x;
treeProto.y = tree_y;

function constant$4(x) {
  return function() {
    return x;
  };
}

function jiggle(random) {
  return (random() - 0.5) * 1e-6;
}

function x$3(d) {
  return d.x + d.vx;
}

function y$3(d) {
  return d.y + d.vy;
}

function collide(radius) {
  var nodes,
      radii,
      random,
      strength = 1,
      iterations = 1;

  if (typeof radius !== "function") radius = constant$4(radius == null ? 1 : +radius);

  function force() {
    var i, n = nodes.length,
        tree,
        node,
        xi,
        yi,
        ri,
        ri2;

    for (var k = 0; k < iterations; ++k) {
      tree = quadtree(nodes, x$3, y$3).visitAfter(prepare);
      for (i = 0; i < n; ++i) {
        node = nodes[i];
        ri = radii[node.index], ri2 = ri * ri;
        xi = node.x + node.vx;
        yi = node.y + node.vy;
        tree.visit(apply);
      }
    }

    function apply(quad, x0, y0, x1, y1) {
      var data = quad.data, rj = quad.r, r = ri + rj;
      if (data) {
        if (data.index > node.index) {
          var x = xi - data.x - data.vx,
              y = yi - data.y - data.vy,
              l = x * x + y * y;
          if (l < r * r) {
            if (x === 0) x = jiggle(random), l += x * x;
            if (y === 0) y = jiggle(random), l += y * y;
            l = (r - (l = Math.sqrt(l))) / l * strength;
            node.vx += (x *= l) * (r = (rj *= rj) / (ri2 + rj));
            node.vy += (y *= l) * r;
            data.vx -= x * (r = 1 - r);
            data.vy -= y * r;
          }
        }
        return;
      }
      return x0 > xi + r || x1 < xi - r || y0 > yi + r || y1 < yi - r;
    }
  }

  function prepare(quad) {
    if (quad.data) return quad.r = radii[quad.data.index];
    for (var i = quad.r = 0; i < 4; ++i) {
      if (quad[i] && quad[i].r > quad.r) {
        quad.r = quad[i].r;
      }
    }
  }

  function initialize() {
    if (!nodes) return;
    var i, n = nodes.length, node;
    radii = new Array(n);
    for (i = 0; i < n; ++i) node = nodes[i], radii[node.index] = +radius(node, i, nodes);
  }

  force.initialize = function(_nodes, _random) {
    nodes = _nodes;
    random = _random;
    initialize();
  };

  force.iterations = function(_) {
    return arguments.length ? (iterations = +_, force) : iterations;
  };

  force.strength = function(_) {
    return arguments.length ? (strength = +_, force) : strength;
  };

  force.radius = function(_) {
    return arguments.length ? (radius = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : radius;
  };

  return force;
}

function index$3(d) {
  return d.index;
}

function find(nodeById, nodeId) {
  var node = nodeById.get(nodeId);
  if (!node) throw new Error("node not found: " + nodeId);
  return node;
}

function link$2(links) {
  var id = index$3,
      strength = defaultStrength,
      strengths,
      distance = constant$4(30),
      distances,
      nodes,
      count,
      bias,
      random,
      iterations = 1;

  if (links == null) links = [];

  function defaultStrength(link) {
    return 1 / Math.min(count[link.source.index], count[link.target.index]);
  }

  function force(alpha) {
    for (var k = 0, n = links.length; k < iterations; ++k) {
      for (var i = 0, link, source, target, x, y, l, b; i < n; ++i) {
        link = links[i], source = link.source, target = link.target;
        x = target.x + target.vx - source.x - source.vx || jiggle(random);
        y = target.y + target.vy - source.y - source.vy || jiggle(random);
        l = Math.sqrt(x * x + y * y);
        l = (l - distances[i]) / l * alpha * strengths[i];
        x *= l, y *= l;
        target.vx -= x * (b = bias[i]);
        target.vy -= y * b;
        source.vx += x * (b = 1 - b);
        source.vy += y * b;
      }
    }
  }

  function initialize() {
    if (!nodes) return;

    var i,
        n = nodes.length,
        m = links.length,
        nodeById = new Map(nodes.map((d, i) => [id(d, i, nodes), d])),
        link;

    for (i = 0, count = new Array(n); i < m; ++i) {
      link = links[i], link.index = i;
      if (typeof link.source !== "object") link.source = find(nodeById, link.source);
      if (typeof link.target !== "object") link.target = find(nodeById, link.target);
      count[link.source.index] = (count[link.source.index] || 0) + 1;
      count[link.target.index] = (count[link.target.index] || 0) + 1;
    }

    for (i = 0, bias = new Array(m); i < m; ++i) {
      link = links[i], bias[i] = count[link.source.index] / (count[link.source.index] + count[link.target.index]);
    }

    strengths = new Array(m), initializeStrength();
    distances = new Array(m), initializeDistance();
  }

  function initializeStrength() {
    if (!nodes) return;

    for (var i = 0, n = links.length; i < n; ++i) {
      strengths[i] = +strength(links[i], i, links);
    }
  }

  function initializeDistance() {
    if (!nodes) return;

    for (var i = 0, n = links.length; i < n; ++i) {
      distances[i] = +distance(links[i], i, links);
    }
  }

  force.initialize = function(_nodes, _random) {
    nodes = _nodes;
    random = _random;
    initialize();
  };

  force.links = function(_) {
    return arguments.length ? (links = _, initialize(), force) : links;
  };

  force.id = function(_) {
    return arguments.length ? (id = _, force) : id;
  };

  force.iterations = function(_) {
    return arguments.length ? (iterations = +_, force) : iterations;
  };

  force.strength = function(_) {
    return arguments.length ? (strength = typeof _ === "function" ? _ : constant$4(+_), initializeStrength(), force) : strength;
  };

  force.distance = function(_) {
    return arguments.length ? (distance = typeof _ === "function" ? _ : constant$4(+_), initializeDistance(), force) : distance;
  };

  return force;
}

// https://en.wikipedia.org/wiki/Linear_congruential_generator#Parameters_in_common_use
const a$2 = 1664525;
const c$4 = 1013904223;
const m$1 = 4294967296; // 2^32

function lcg$2() {
  let s = 1;
  return () => (s = (a$2 * s + c$4) % m$1) / m$1;
}

function x$2(d) {
  return d.x;
}

function y$2(d) {
  return d.y;
}

var initialRadius = 10,
    initialAngle = Math.PI * (3 - Math.sqrt(5));

function simulation(nodes) {
  var simulation,
      alpha = 1,
      alphaMin = 0.001,
      alphaDecay = 1 - Math.pow(alphaMin, 1 / 300),
      alphaTarget = 0,
      velocityDecay = 0.6,
      forces = new Map(),
      stepper = timer(step),
      event = dispatch("tick", "end"),
      random = lcg$2();

  if (nodes == null) nodes = [];

  function step() {
    tick();
    event.call("tick", simulation);
    if (alpha < alphaMin) {
      stepper.stop();
      event.call("end", simulation);
    }
  }

  function tick(iterations) {
    var i, n = nodes.length, node;

    if (iterations === undefined) iterations = 1;

    for (var k = 0; k < iterations; ++k) {
      alpha += (alphaTarget - alpha) * alphaDecay;

      forces.forEach(function(force) {
        force(alpha);
      });

      for (i = 0; i < n; ++i) {
        node = nodes[i];
        if (node.fx == null) node.x += node.vx *= velocityDecay;
        else node.x = node.fx, node.vx = 0;
        if (node.fy == null) node.y += node.vy *= velocityDecay;
        else node.y = node.fy, node.vy = 0;
      }
    }

    return simulation;
  }

  function initializeNodes() {
    for (var i = 0, n = nodes.length, node; i < n; ++i) {
      node = nodes[i], node.index = i;
      if (node.fx != null) node.x = node.fx;
      if (node.fy != null) node.y = node.fy;
      if (isNaN(node.x) || isNaN(node.y)) {
        var radius = initialRadius * Math.sqrt(0.5 + i), angle = i * initialAngle;
        node.x = radius * Math.cos(angle);
        node.y = radius * Math.sin(angle);
      }
      if (isNaN(node.vx) || isNaN(node.vy)) {
        node.vx = node.vy = 0;
      }
    }
  }

  function initializeForce(force) {
    if (force.initialize) force.initialize(nodes, random);
    return force;
  }

  initializeNodes();

  return simulation = {
    tick: tick,

    restart: function() {
      return stepper.restart(step), simulation;
    },

    stop: function() {
      return stepper.stop(), simulation;
    },

    nodes: function(_) {
      return arguments.length ? (nodes = _, initializeNodes(), forces.forEach(initializeForce), simulation) : nodes;
    },

    alpha: function(_) {
      return arguments.length ? (alpha = +_, simulation) : alpha;
    },

    alphaMin: function(_) {
      return arguments.length ? (alphaMin = +_, simulation) : alphaMin;
    },

    alphaDecay: function(_) {
      return arguments.length ? (alphaDecay = +_, simulation) : +alphaDecay;
    },

    alphaTarget: function(_) {
      return arguments.length ? (alphaTarget = +_, simulation) : alphaTarget;
    },

    velocityDecay: function(_) {
      return arguments.length ? (velocityDecay = 1 - _, simulation) : 1 - velocityDecay;
    },

    randomSource: function(_) {
      return arguments.length ? (random = _, forces.forEach(initializeForce), simulation) : random;
    },

    force: function(name, _) {
      return arguments.length > 1 ? ((_ == null ? forces.delete(name) : forces.set(name, initializeForce(_))), simulation) : forces.get(name);
    },

    find: function(x, y, radius) {
      var i = 0,
          n = nodes.length,
          dx,
          dy,
          d2,
          node,
          closest;

      if (radius == null) radius = Infinity;
      else radius *= radius;

      for (i = 0; i < n; ++i) {
        node = nodes[i];
        dx = x - node.x;
        dy = y - node.y;
        d2 = dx * dx + dy * dy;
        if (d2 < radius) closest = node, radius = d2;
      }

      return closest;
    },

    on: function(name, _) {
      return arguments.length > 1 ? (event.on(name, _), simulation) : event.on(name);
    }
  };
}

function manyBody() {
  var nodes,
      node,
      random,
      alpha,
      strength = constant$4(-30),
      strengths,
      distanceMin2 = 1,
      distanceMax2 = Infinity,
      theta2 = 0.81;

  function force(_) {
    var i, n = nodes.length, tree = quadtree(nodes, x$2, y$2).visitAfter(accumulate);
    for (alpha = _, i = 0; i < n; ++i) node = nodes[i], tree.visit(apply);
  }

  function initialize() {
    if (!nodes) return;
    var i, n = nodes.length, node;
    strengths = new Array(n);
    for (i = 0; i < n; ++i) node = nodes[i], strengths[node.index] = +strength(node, i, nodes);
  }

  function accumulate(quad) {
    var strength = 0, q, c, weight = 0, x, y, i;

    // For internal nodes, accumulate forces from child quadrants.
    if (quad.length) {
      for (x = y = i = 0; i < 4; ++i) {
        if ((q = quad[i]) && (c = Math.abs(q.value))) {
          strength += q.value, weight += c, x += c * q.x, y += c * q.y;
        }
      }
      quad.x = x / weight;
      quad.y = y / weight;
    }

    // For leaf nodes, accumulate forces from coincident quadrants.
    else {
      q = quad;
      q.x = q.data.x;
      q.y = q.data.y;
      do strength += strengths[q.data.index];
      while (q = q.next);
    }

    quad.value = strength;
  }

  function apply(quad, x1, _, x2) {
    if (!quad.value) return true;

    var x = quad.x - node.x,
        y = quad.y - node.y,
        w = x2 - x1,
        l = x * x + y * y;

    // Apply the Barnes-Hut approximation if possible.
    // Limit forces for very close nodes; randomize direction if coincident.
    if (w * w / theta2 < l) {
      if (l < distanceMax2) {
        if (x === 0) x = jiggle(random), l += x * x;
        if (y === 0) y = jiggle(random), l += y * y;
        if (l < distanceMin2) l = Math.sqrt(distanceMin2 * l);
        node.vx += x * quad.value * alpha / l;
        node.vy += y * quad.value * alpha / l;
      }
      return true;
    }

    // Otherwise, process points directly.
    else if (quad.length || l >= distanceMax2) return;

    // Limit forces for very close nodes; randomize direction if coincident.
    if (quad.data !== node || quad.next) {
      if (x === 0) x = jiggle(random), l += x * x;
      if (y === 0) y = jiggle(random), l += y * y;
      if (l < distanceMin2) l = Math.sqrt(distanceMin2 * l);
    }

    do if (quad.data !== node) {
      w = strengths[quad.data.index] * alpha / l;
      node.vx += x * w;
      node.vy += y * w;
    } while (quad = quad.next);
  }

  force.initialize = function(_nodes, _random) {
    nodes = _nodes;
    random = _random;
    initialize();
  };

  force.strength = function(_) {
    return arguments.length ? (strength = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : strength;
  };

  force.distanceMin = function(_) {
    return arguments.length ? (distanceMin2 = _ * _, force) : Math.sqrt(distanceMin2);
  };

  force.distanceMax = function(_) {
    return arguments.length ? (distanceMax2 = _ * _, force) : Math.sqrt(distanceMax2);
  };

  force.theta = function(_) {
    return arguments.length ? (theta2 = _ * _, force) : Math.sqrt(theta2);
  };

  return force;
}

function radial$1(radius, x, y) {
  var nodes,
      strength = constant$4(0.1),
      strengths,
      radiuses;

  if (typeof radius !== "function") radius = constant$4(+radius);
  if (x == null) x = 0;
  if (y == null) y = 0;

  function force(alpha) {
    for (var i = 0, n = nodes.length; i < n; ++i) {
      var node = nodes[i],
          dx = node.x - x || 1e-6,
          dy = node.y - y || 1e-6,
          r = Math.sqrt(dx * dx + dy * dy),
          k = (radiuses[i] - r) * strengths[i] * alpha / r;
      node.vx += dx * k;
      node.vy += dy * k;
    }
  }

  function initialize() {
    if (!nodes) return;
    var i, n = nodes.length;
    strengths = new Array(n);
    radiuses = new Array(n);
    for (i = 0; i < n; ++i) {
      radiuses[i] = +radius(nodes[i], i, nodes);
      strengths[i] = isNaN(radiuses[i]) ? 0 : +strength(nodes[i], i, nodes);
    }
  }

  force.initialize = function(_) {
    nodes = _, initialize();
  };

  force.strength = function(_) {
    return arguments.length ? (strength = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : strength;
  };

  force.radius = function(_) {
    return arguments.length ? (radius = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : radius;
  };

  force.x = function(_) {
    return arguments.length ? (x = +_, force) : x;
  };

  force.y = function(_) {
    return arguments.length ? (y = +_, force) : y;
  };

  return force;
}

function x$1(x) {
  var strength = constant$4(0.1),
      nodes,
      strengths,
      xz;

  if (typeof x !== "function") x = constant$4(x == null ? 0 : +x);

  function force(alpha) {
    for (var i = 0, n = nodes.length, node; i < n; ++i) {
      node = nodes[i], node.vx += (xz[i] - node.x) * strengths[i] * alpha;
    }
  }

  function initialize() {
    if (!nodes) return;
    var i, n = nodes.length;
    strengths = new Array(n);
    xz = new Array(n);
    for (i = 0; i < n; ++i) {
      strengths[i] = isNaN(xz[i] = +x(nodes[i], i, nodes)) ? 0 : +strength(nodes[i], i, nodes);
    }
  }

  force.initialize = function(_) {
    nodes = _;
    initialize();
  };

  force.strength = function(_) {
    return arguments.length ? (strength = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : strength;
  };

  force.x = function(_) {
    return arguments.length ? (x = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : x;
  };

  return force;
}

function y$1(y) {
  var strength = constant$4(0.1),
      nodes,
      strengths,
      yz;

  if (typeof y !== "function") y = constant$4(y == null ? 0 : +y);

  function force(alpha) {
    for (var i = 0, n = nodes.length, node; i < n; ++i) {
      node = nodes[i], node.vy += (yz[i] - node.y) * strengths[i] * alpha;
    }
  }

  function initialize() {
    if (!nodes) return;
    var i, n = nodes.length;
    strengths = new Array(n);
    yz = new Array(n);
    for (i = 0; i < n; ++i) {
      strengths[i] = isNaN(yz[i] = +y(nodes[i], i, nodes)) ? 0 : +strength(nodes[i], i, nodes);
    }
  }

  force.initialize = function(_) {
    nodes = _;
    initialize();
  };

  force.strength = function(_) {
    return arguments.length ? (strength = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : strength;
  };

  force.y = function(_) {
    return arguments.length ? (y = typeof _ === "function" ? _ : constant$4(+_), initialize(), force) : y;
  };

  return force;
}

function formatDecimal(x) {
  return Math.abs(x = Math.round(x)) >= 1e21
      ? x.toLocaleString("en").replace(/,/g, "")
      : x.toString(10);
}

// Computes the decimal coefficient and exponent of the specified number x with
// significant digits p, where x is positive and p is in [1, 21] or undefined.
// For example, formatDecimalParts(1.23) returns ["123", 0].
function formatDecimalParts(x, p) {
  if ((i = (x = p ? x.toExponential(p - 1) : x.toExponential()).indexOf("e")) < 0) return null; // NaN, ±Infinity
  var i, coefficient = x.slice(0, i);

  // The string returned by toExponential either has the form \d\.\d+e[-+]\d+
  // (e.g., 1.2e+3) or the form \de[-+]\d+ (e.g., 1e+3).
  return [
    coefficient.length > 1 ? coefficient[0] + coefficient.slice(2) : coefficient,
    +x.slice(i + 1)
  ];
}

function exponent(x) {
  return x = formatDecimalParts(Math.abs(x)), x ? x[1] : NaN;
}

function formatGroup(grouping, thousands) {
  return function(value, width) {
    var i = value.length,
        t = [],
        j = 0,
        g = grouping[0],
        length = 0;

    while (i > 0 && g > 0) {
      if (length + g + 1 > width) g = Math.max(1, width - length);
      t.push(value.substring(i -= g, i + g));
      if ((length += g + 1) > width) break;
      g = grouping[j = (j + 1) % grouping.length];
    }

    return t.reverse().join(thousands);
  };
}

function formatNumerals(numerals) {
  return function(value) {
    return value.replace(/[0-9]/g, function(i) {
      return numerals[+i];
    });
  };
}

// [[fill]align][sign][symbol][0][width][,][.precision][~][type]
var re = /^(?:(.)?([<>=^]))?([+\-( ])?([$#])?(0)?(\d+)?(,)?(\.\d+)?(~)?([a-z%])?$/i;

function formatSpecifier(specifier) {
  if (!(match = re.exec(specifier))) throw new Error("invalid format: " + specifier);
  var match;
  return new FormatSpecifier({
    fill: match[1],
    align: match[2],
    sign: match[3],
    symbol: match[4],
    zero: match[5],
    width: match[6],
    comma: match[7],
    precision: match[8] && match[8].slice(1),
    trim: match[9],
    type: match[10]
  });
}

formatSpecifier.prototype = FormatSpecifier.prototype; // instanceof

function FormatSpecifier(specifier) {
  this.fill = specifier.fill === undefined ? " " : specifier.fill + "";
  this.align = specifier.align === undefined ? ">" : specifier.align + "";
  this.sign = specifier.sign === undefined ? "-" : specifier.sign + "";
  this.symbol = specifier.symbol === undefined ? "" : specifier.symbol + "";
  this.zero = !!specifier.zero;
  this.width = specifier.width === undefined ? undefined : +specifier.width;
  this.comma = !!specifier.comma;
  this.precision = specifier.precision === undefined ? undefined : +specifier.precision;
  this.trim = !!specifier.trim;
  this.type = specifier.type === undefined ? "" : specifier.type + "";
}

FormatSpecifier.prototype.toString = function() {
  return this.fill
      + this.align
      + this.sign
      + this.symbol
      + (this.zero ? "0" : "")
      + (this.width === undefined ? "" : Math.max(1, this.width | 0))
      + (this.comma ? "," : "")
      + (this.precision === undefined ? "" : "." + Math.max(0, this.precision | 0))
      + (this.trim ? "~" : "")
      + this.type;
};

// Trims insignificant zeros, e.g., replaces 1.2000k with 1.2k.
function formatTrim(s) {
  out: for (var n = s.length, i = 1, i0 = -1, i1; i < n; ++i) {
    switch (s[i]) {
      case ".": i0 = i1 = i; break;
      case "0": if (i0 === 0) i0 = i; i1 = i; break;
      default: if (!+s[i]) break out; if (i0 > 0) i0 = 0; break;
    }
  }
  return i0 > 0 ? s.slice(0, i0) + s.slice(i1 + 1) : s;
}

var prefixExponent;

function formatPrefixAuto(x, p) {
  var d = formatDecimalParts(x, p);
  if (!d) return x + "";
  var coefficient = d[0],
      exponent = d[1],
      i = exponent - (prefixExponent = Math.max(-8, Math.min(8, Math.floor(exponent / 3))) * 3) + 1,
      n = coefficient.length;
  return i === n ? coefficient
      : i > n ? coefficient + new Array(i - n + 1).join("0")
      : i > 0 ? coefficient.slice(0, i) + "." + coefficient.slice(i)
      : "0." + new Array(1 - i).join("0") + formatDecimalParts(x, Math.max(0, p + i - 1))[0]; // less than 1y!
}

function formatRounded(x, p) {
  var d = formatDecimalParts(x, p);
  if (!d) return x + "";
  var coefficient = d[0],
      exponent = d[1];
  return exponent < 0 ? "0." + new Array(-exponent).join("0") + coefficient
      : coefficient.length > exponent + 1 ? coefficient.slice(0, exponent + 1) + "." + coefficient.slice(exponent + 1)
      : coefficient + new Array(exponent - coefficient.length + 2).join("0");
}

const formatTypes = {
  "%": (x, p) => (x * 100).toFixed(p),
  "b": (x) => Math.round(x).toString(2),
  "c": (x) => x + "",
  "d": formatDecimal,
  "e": (x, p) => x.toExponential(p),
  "f": (x, p) => x.toFixed(p),
  "g": (x, p) => x.toPrecision(p),
  "o": (x) => Math.round(x).toString(8),
  "p": (x, p) => formatRounded(x * 100, p),
  "r": formatRounded,
  "s": formatPrefixAuto,
  "X": (x) => Math.round(x).toString(16).toUpperCase(),
  "x": (x) => Math.round(x).toString(16)
};

function identity$6(x) {
  return x;
}

var map = Array.prototype.map,
    prefixes = ["y","z","a","f","p","n","µ","m","","k","M","G","T","P","E","Z","Y"];

function formatLocale$1(locale) {
  var group = locale.grouping === undefined || locale.thousands === undefined ? identity$6 : formatGroup(map.call(locale.grouping, Number), locale.thousands + ""),
      currencyPrefix = locale.currency === undefined ? "" : locale.currency[0] + "",
      currencySuffix = locale.currency === undefined ? "" : locale.currency[1] + "",
      decimal = locale.decimal === undefined ? "." : locale.decimal + "",
      numerals = locale.numerals === undefined ? identity$6 : formatNumerals(map.call(locale.numerals, String)),
      percent = locale.percent === undefined ? "%" : locale.percent + "",
      minus = locale.minus === undefined ? "−" : locale.minus + "",
      nan = locale.nan === undefined ? "NaN" : locale.nan + "";

  function newFormat(specifier) {
    specifier = formatSpecifier(specifier);

    var fill = specifier.fill,
        align = specifier.align,
        sign = specifier.sign,
        symbol = specifier.symbol,
        zero = specifier.zero,
        width = specifier.width,
        comma = specifier.comma,
        precision = specifier.precision,
        trim = specifier.trim,
        type = specifier.type;

    // The "n" type is an alias for ",g".
    if (type === "n") comma = true, type = "g";

    // The "" type, and any invalid type, is an alias for ".12~g".
    else if (!formatTypes[type]) precision === undefined && (precision = 12), trim = true, type = "g";

    // If zero fill is specified, padding goes after sign and before digits.
    if (zero || (fill === "0" && align === "=")) zero = true, fill = "0", align = "=";

    // Compute the prefix and suffix.
    // For SI-prefix, the suffix is lazily computed.
    var prefix = symbol === "$" ? currencyPrefix : symbol === "#" && /[boxX]/.test(type) ? "0" + type.toLowerCase() : "",
        suffix = symbol === "$" ? currencySuffix : /[%p]/.test(type) ? percent : "";

    // What format function should we use?
    // Is this an integer type?
    // Can this type generate exponential notation?
    var formatType = formatTypes[type],
        maybeSuffix = /[defgprs%]/.test(type);

    // Set the default precision if not specified,
    // or clamp the specified precision to the supported range.
    // For significant precision, it must be in [1, 21].
    // For fixed precision, it must be in [0, 20].
    precision = precision === undefined ? 6
        : /[gprs]/.test(type) ? Math.max(1, Math.min(21, precision))
        : Math.max(0, Math.min(20, precision));

    function format(value) {
      var valuePrefix = prefix,
          valueSuffix = suffix,
          i, n, c;

      if (type === "c") {
        valueSuffix = formatType(value) + valueSuffix;
        value = "";
      } else {
        value = +value;

        // Determine the sign. -0 is not less than 0, but 1 / -0 is!
        var valueNegative = value < 0 || 1 / value < 0;

        // Perform the initial formatting.
        value = isNaN(value) ? nan : formatType(Math.abs(value), precision);

        // Trim insignificant zeros.
        if (trim) value = formatTrim(value);

        // If a negative value rounds to zero after formatting, and no explicit positive sign is requested, hide the sign.
        if (valueNegative && +value === 0 && sign !== "+") valueNegative = false;

        // Compute the prefix and suffix.
        valuePrefix = (valueNegative ? (sign === "(" ? sign : minus) : sign === "-" || sign === "(" ? "" : sign) + valuePrefix;
        valueSuffix = (type === "s" ? prefixes[8 + prefixExponent / 3] : "") + valueSuffix + (valueNegative && sign === "(" ? ")" : "");

        // Break the formatted value into the integer “value” part that can be
        // grouped, and fractional or exponential “suffix” part that is not.
        if (maybeSuffix) {
          i = -1, n = value.length;
          while (++i < n) {
            if (c = value.charCodeAt(i), 48 > c || c > 57) {
              valueSuffix = (c === 46 ? decimal + value.slice(i + 1) : value.slice(i)) + valueSuffix;
              value = value.slice(0, i);
              break;
            }
          }
        }
      }

      // If the fill character is not "0", grouping is applied before padding.
      if (comma && !zero) value = group(value, Infinity);

      // Compute the padding.
      var length = valuePrefix.length + value.length + valueSuffix.length,
          padding = length < width ? new Array(width - length + 1).join(fill) : "";

      // If the fill character is "0", grouping is applied after padding.
      if (comma && zero) value = group(padding + value, padding.length ? width - valueSuffix.length : Infinity), padding = "";

      // Reconstruct the final output based on the desired alignment.
      switch (align) {
        case "<": value = valuePrefix + value + valueSuffix + padding; break;
        case "=": value = valuePrefix + padding + value + valueSuffix; break;
        case "^": value = padding.slice(0, length = padding.length >> 1) + valuePrefix + value + valueSuffix + padding.slice(length); break;
        default: value = padding + valuePrefix + value + valueSuffix; break;
      }

      return numerals(value);
    }

    format.toString = function() {
      return specifier + "";
    };

    return format;
  }

  function formatPrefix(specifier, value) {
    var f = newFormat((specifier = formatSpecifier(specifier), specifier.type = "f", specifier)),
        e = Math.max(-8, Math.min(8, Math.floor(exponent(value) / 3))) * 3,
        k = Math.pow(10, -e),
        prefix = prefixes[8 + e / 3];
    return function(value) {
      return f(k * value) + prefix;
    };
  }

  return {
    format: newFormat,
    formatPrefix: formatPrefix
  };
}

var locale$1;
var format;
var formatPrefix;

defaultLocale$1({
  thousands: ",",
  grouping: [3],
  currency: ["$", ""]
});

function defaultLocale$1(definition) {
  locale$1 = formatLocale$1(definition);
  format = locale$1.format;
  formatPrefix = locale$1.formatPrefix;
  return locale$1;
}

function precisionFixed(step) {
  return Math.max(0, -exponent(Math.abs(step)));
}

function precisionPrefix(step, value) {
  return Math.max(0, Math.max(-8, Math.min(8, Math.floor(exponent(value) / 3))) * 3 - exponent(Math.abs(step)));
}

function precisionRound(step, max) {
  step = Math.abs(step), max = Math.abs(max) - step;
  return Math.max(0, exponent(max) - exponent(step)) + 1;
}

var epsilon$1 = 1e-6;
var epsilon2 = 1e-12;
var pi$1 = Math.PI;
var halfPi$1 = pi$1 / 2;
var quarterPi = pi$1 / 4;
var tau$1 = pi$1 * 2;

var degrees = 180 / pi$1;
var radians = pi$1 / 180;

var abs$1 = Math.abs;
var atan = Math.atan;
var atan2$1 = Math.atan2;
var cos$1 = Math.cos;
var ceil = Math.ceil;
var exp = Math.exp;
var floor = Math.floor;
var hypot = Math.hypot;
var log$1 = Math.log;
var pow$1 = Math.pow;
var sin$1 = Math.sin;
var sign$1 = Math.sign || function(x) { return x > 0 ? 1 : x < 0 ? -1 : 0; };
var sqrt$2 = Math.sqrt;
var tan = Math.tan;

function acos$1(x) {
  return x > 1 ? 0 : x < -1 ? pi$1 : Math.acos(x);
}

function asin$1(x) {
  return x > 1 ? halfPi$1 : x < -1 ? -halfPi$1 : Math.asin(x);
}

function haversin(x) {
  return (x = sin$1(x / 2)) * x;
}

function noop$1() {}

function streamGeometry(geometry, stream) {
  if (geometry && streamGeometryType.hasOwnProperty(geometry.type)) {
    streamGeometryType[geometry.type](geometry, stream);
  }
}

var streamObjectType = {
  Feature: function(object, stream) {
    streamGeometry(object.geometry, stream);
  },
  FeatureCollection: function(object, stream) {
    var features = object.features, i = -1, n = features.length;
    while (++i < n) streamGeometry(features[i].geometry, stream);
  }
};

var streamGeometryType = {
  Sphere: function(object, stream) {
    stream.sphere();
  },
  Point: function(object, stream) {
    object = object.coordinates;
    stream.point(object[0], object[1], object[2]);
  },
  MultiPoint: function(object, stream) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) object = coordinates[i], stream.point(object[0], object[1], object[2]);
  },
  LineString: function(object, stream) {
    streamLine(object.coordinates, stream, 0);
  },
  MultiLineString: function(object, stream) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) streamLine(coordinates[i], stream, 0);
  },
  Polygon: function(object, stream) {
    streamPolygon(object.coordinates, stream);
  },
  MultiPolygon: function(object, stream) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) streamPolygon(coordinates[i], stream);
  },
  GeometryCollection: function(object, stream) {
    var geometries = object.geometries, i = -1, n = geometries.length;
    while (++i < n) streamGeometry(geometries[i], stream);
  }
};

function streamLine(coordinates, stream, closed) {
  var i = -1, n = coordinates.length - closed, coordinate;
  stream.lineStart();
  while (++i < n) coordinate = coordinates[i], stream.point(coordinate[0], coordinate[1], coordinate[2]);
  stream.lineEnd();
}

function streamPolygon(coordinates, stream) {
  var i = -1, n = coordinates.length;
  stream.polygonStart();
  while (++i < n) streamLine(coordinates[i], stream, 1);
  stream.polygonEnd();
}

function geoStream(object, stream) {
  if (object && streamObjectType.hasOwnProperty(object.type)) {
    streamObjectType[object.type](object, stream);
  } else {
    streamGeometry(object, stream);
  }
}

var areaRingSum$1 = new Adder();

// hello?

var areaSum$1 = new Adder(),
    lambda00$2,
    phi00$2,
    lambda0$2,
    cosPhi0$1,
    sinPhi0$1;

var areaStream$1 = {
  point: noop$1,
  lineStart: noop$1,
  lineEnd: noop$1,
  polygonStart: function() {
    areaRingSum$1 = new Adder();
    areaStream$1.lineStart = areaRingStart$1;
    areaStream$1.lineEnd = areaRingEnd$1;
  },
  polygonEnd: function() {
    var areaRing = +areaRingSum$1;
    areaSum$1.add(areaRing < 0 ? tau$1 + areaRing : areaRing);
    this.lineStart = this.lineEnd = this.point = noop$1;
  },
  sphere: function() {
    areaSum$1.add(tau$1);
  }
};

function areaRingStart$1() {
  areaStream$1.point = areaPointFirst$1;
}

function areaRingEnd$1() {
  areaPoint$1(lambda00$2, phi00$2);
}

function areaPointFirst$1(lambda, phi) {
  areaStream$1.point = areaPoint$1;
  lambda00$2 = lambda, phi00$2 = phi;
  lambda *= radians, phi *= radians;
  lambda0$2 = lambda, cosPhi0$1 = cos$1(phi = phi / 2 + quarterPi), sinPhi0$1 = sin$1(phi);
}

function areaPoint$1(lambda, phi) {
  lambda *= radians, phi *= radians;
  phi = phi / 2 + quarterPi; // half the angular distance from south pole

  // Spherical excess E for a spherical triangle with vertices: south pole,
  // previous point, current point.  Uses a formula derived from Cagnoli’s
  // theorem.  See Todhunter, Spherical Trig. (1871), Sec. 103, Eq. (2).
  var dLambda = lambda - lambda0$2,
      sdLambda = dLambda >= 0 ? 1 : -1,
      adLambda = sdLambda * dLambda,
      cosPhi = cos$1(phi),
      sinPhi = sin$1(phi),
      k = sinPhi0$1 * sinPhi,
      u = cosPhi0$1 * cosPhi + k * cos$1(adLambda),
      v = k * sdLambda * sin$1(adLambda);
  areaRingSum$1.add(atan2$1(v, u));

  // Advance the previous points.
  lambda0$2 = lambda, cosPhi0$1 = cosPhi, sinPhi0$1 = sinPhi;
}

function area$2(object) {
  areaSum$1 = new Adder();
  geoStream(object, areaStream$1);
  return areaSum$1 * 2;
}

function spherical(cartesian) {
  return [atan2$1(cartesian[1], cartesian[0]), asin$1(cartesian[2])];
}

function cartesian(spherical) {
  var lambda = spherical[0], phi = spherical[1], cosPhi = cos$1(phi);
  return [cosPhi * cos$1(lambda), cosPhi * sin$1(lambda), sin$1(phi)];
}

function cartesianDot(a, b) {
  return a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
}

function cartesianCross(a, b) {
  return [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
}

// TODO return a
function cartesianAddInPlace(a, b) {
  a[0] += b[0], a[1] += b[1], a[2] += b[2];
}

function cartesianScale(vector, k) {
  return [vector[0] * k, vector[1] * k, vector[2] * k];
}

// TODO return d
function cartesianNormalizeInPlace(d) {
  var l = sqrt$2(d[0] * d[0] + d[1] * d[1] + d[2] * d[2]);
  d[0] /= l, d[1] /= l, d[2] /= l;
}

var lambda0$1, phi0, lambda1, phi1, // bounds
    lambda2, // previous lambda-coordinate
    lambda00$1, phi00$1, // first point
    p0, // previous 3D point
    deltaSum,
    ranges,
    range;

var boundsStream$1 = {
  point: boundsPoint$1,
  lineStart: boundsLineStart,
  lineEnd: boundsLineEnd,
  polygonStart: function() {
    boundsStream$1.point = boundsRingPoint;
    boundsStream$1.lineStart = boundsRingStart;
    boundsStream$1.lineEnd = boundsRingEnd;
    deltaSum = new Adder();
    areaStream$1.polygonStart();
  },
  polygonEnd: function() {
    areaStream$1.polygonEnd();
    boundsStream$1.point = boundsPoint$1;
    boundsStream$1.lineStart = boundsLineStart;
    boundsStream$1.lineEnd = boundsLineEnd;
    if (areaRingSum$1 < 0) lambda0$1 = -(lambda1 = 180), phi0 = -(phi1 = 90);
    else if (deltaSum > epsilon$1) phi1 = 90;
    else if (deltaSum < -epsilon$1) phi0 = -90;
    range[0] = lambda0$1, range[1] = lambda1;
  },
  sphere: function() {
    lambda0$1 = -(lambda1 = 180), phi0 = -(phi1 = 90);
  }
};

function boundsPoint$1(lambda, phi) {
  ranges.push(range = [lambda0$1 = lambda, lambda1 = lambda]);
  if (phi < phi0) phi0 = phi;
  if (phi > phi1) phi1 = phi;
}

function linePoint(lambda, phi) {
  var p = cartesian([lambda * radians, phi * radians]);
  if (p0) {
    var normal = cartesianCross(p0, p),
        equatorial = [normal[1], -normal[0], 0],
        inflection = cartesianCross(equatorial, normal);
    cartesianNormalizeInPlace(inflection);
    inflection = spherical(inflection);
    var delta = lambda - lambda2,
        sign = delta > 0 ? 1 : -1,
        lambdai = inflection[0] * degrees * sign,
        phii,
        antimeridian = abs$1(delta) > 180;
    if (antimeridian ^ (sign * lambda2 < lambdai && lambdai < sign * lambda)) {
      phii = inflection[1] * degrees;
      if (phii > phi1) phi1 = phii;
    } else if (lambdai = (lambdai + 360) % 360 - 180, antimeridian ^ (sign * lambda2 < lambdai && lambdai < sign * lambda)) {
      phii = -inflection[1] * degrees;
      if (phii < phi0) phi0 = phii;
    } else {
      if (phi < phi0) phi0 = phi;
      if (phi > phi1) phi1 = phi;
    }
    if (antimeridian) {
      if (lambda < lambda2) {
        if (angle(lambda0$1, lambda) > angle(lambda0$1, lambda1)) lambda1 = lambda;
      } else {
        if (angle(lambda, lambda1) > angle(lambda0$1, lambda1)) lambda0$1 = lambda;
      }
    } else {
      if (lambda1 >= lambda0$1) {
        if (lambda < lambda0$1) lambda0$1 = lambda;
        if (lambda > lambda1) lambda1 = lambda;
      } else {
        if (lambda > lambda2) {
          if (angle(lambda0$1, lambda) > angle(lambda0$1, lambda1)) lambda1 = lambda;
        } else {
          if (angle(lambda, lambda1) > angle(lambda0$1, lambda1)) lambda0$1 = lambda;
        }
      }
    }
  } else {
    ranges.push(range = [lambda0$1 = lambda, lambda1 = lambda]);
  }
  if (phi < phi0) phi0 = phi;
  if (phi > phi1) phi1 = phi;
  p0 = p, lambda2 = lambda;
}

function boundsLineStart() {
  boundsStream$1.point = linePoint;
}

function boundsLineEnd() {
  range[0] = lambda0$1, range[1] = lambda1;
  boundsStream$1.point = boundsPoint$1;
  p0 = null;
}

function boundsRingPoint(lambda, phi) {
  if (p0) {
    var delta = lambda - lambda2;
    deltaSum.add(abs$1(delta) > 180 ? delta + (delta > 0 ? 360 : -360) : delta);
  } else {
    lambda00$1 = lambda, phi00$1 = phi;
  }
  areaStream$1.point(lambda, phi);
  linePoint(lambda, phi);
}

function boundsRingStart() {
  areaStream$1.lineStart();
}

function boundsRingEnd() {
  boundsRingPoint(lambda00$1, phi00$1);
  areaStream$1.lineEnd();
  if (abs$1(deltaSum) > epsilon$1) lambda0$1 = -(lambda1 = 180);
  range[0] = lambda0$1, range[1] = lambda1;
  p0 = null;
}

// Finds the left-right distance between two longitudes.
// This is almost the same as (lambda1 - lambda0 + 360°) % 360°, except that we want
// the distance between ±180° to be 360°.
function angle(lambda0, lambda1) {
  return (lambda1 -= lambda0) < 0 ? lambda1 + 360 : lambda1;
}

function rangeCompare(a, b) {
  return a[0] - b[0];
}

function rangeContains(range, x) {
  return range[0] <= range[1] ? range[0] <= x && x <= range[1] : x < range[0] || range[1] < x;
}

function bounds(feature) {
  var i, n, a, b, merged, deltaMax, delta;

  phi1 = lambda1 = -(lambda0$1 = phi0 = Infinity);
  ranges = [];
  geoStream(feature, boundsStream$1);

  // First, sort ranges by their minimum longitudes.
  if (n = ranges.length) {
    ranges.sort(rangeCompare);

    // Then, merge any ranges that overlap.
    for (i = 1, a = ranges[0], merged = [a]; i < n; ++i) {
      b = ranges[i];
      if (rangeContains(a, b[0]) || rangeContains(a, b[1])) {
        if (angle(a[0], b[1]) > angle(a[0], a[1])) a[1] = b[1];
        if (angle(b[0], a[1]) > angle(a[0], a[1])) a[0] = b[0];
      } else {
        merged.push(a = b);
      }
    }

    // Finally, find the largest gap between the merged ranges.
    // The final bounding box will be the inverse of this gap.
    for (deltaMax = -Infinity, n = merged.length - 1, i = 0, a = merged[n]; i <= n; a = b, ++i) {
      b = merged[i];
      if ((delta = angle(a[1], b[0])) > deltaMax) deltaMax = delta, lambda0$1 = b[0], lambda1 = a[1];
    }
  }

  ranges = range = null;

  return lambda0$1 === Infinity || phi0 === Infinity
      ? [[NaN, NaN], [NaN, NaN]]
      : [[lambda0$1, phi0], [lambda1, phi1]];
}

var W0, W1,
    X0$1, Y0$1, Z0$1,
    X1$1, Y1$1, Z1$1,
    X2$1, Y2$1, Z2$1,
    lambda00, phi00, // first point
    x0$4, y0$4, z0; // previous point

var centroidStream$1 = {
  sphere: noop$1,
  point: centroidPoint$1,
  lineStart: centroidLineStart$1,
  lineEnd: centroidLineEnd$1,
  polygonStart: function() {
    centroidStream$1.lineStart = centroidRingStart$1;
    centroidStream$1.lineEnd = centroidRingEnd$1;
  },
  polygonEnd: function() {
    centroidStream$1.lineStart = centroidLineStart$1;
    centroidStream$1.lineEnd = centroidLineEnd$1;
  }
};

// Arithmetic mean of Cartesian vectors.
function centroidPoint$1(lambda, phi) {
  lambda *= radians, phi *= radians;
  var cosPhi = cos$1(phi);
  centroidPointCartesian(cosPhi * cos$1(lambda), cosPhi * sin$1(lambda), sin$1(phi));
}

function centroidPointCartesian(x, y, z) {
  ++W0;
  X0$1 += (x - X0$1) / W0;
  Y0$1 += (y - Y0$1) / W0;
  Z0$1 += (z - Z0$1) / W0;
}

function centroidLineStart$1() {
  centroidStream$1.point = centroidLinePointFirst;
}

function centroidLinePointFirst(lambda, phi) {
  lambda *= radians, phi *= radians;
  var cosPhi = cos$1(phi);
  x0$4 = cosPhi * cos$1(lambda);
  y0$4 = cosPhi * sin$1(lambda);
  z0 = sin$1(phi);
  centroidStream$1.point = centroidLinePoint;
  centroidPointCartesian(x0$4, y0$4, z0);
}

function centroidLinePoint(lambda, phi) {
  lambda *= radians, phi *= radians;
  var cosPhi = cos$1(phi),
      x = cosPhi * cos$1(lambda),
      y = cosPhi * sin$1(lambda),
      z = sin$1(phi),
      w = atan2$1(sqrt$2((w = y0$4 * z - z0 * y) * w + (w = z0 * x - x0$4 * z) * w + (w = x0$4 * y - y0$4 * x) * w), x0$4 * x + y0$4 * y + z0 * z);
  W1 += w;
  X1$1 += w * (x0$4 + (x0$4 = x));
  Y1$1 += w * (y0$4 + (y0$4 = y));
  Z1$1 += w * (z0 + (z0 = z));
  centroidPointCartesian(x0$4, y0$4, z0);
}

function centroidLineEnd$1() {
  centroidStream$1.point = centroidPoint$1;
}

// See J. E. Brock, The Inertia Tensor for a Spherical Triangle,
// J. Applied Mechanics 42, 239 (1975).
function centroidRingStart$1() {
  centroidStream$1.point = centroidRingPointFirst;
}

function centroidRingEnd$1() {
  centroidRingPoint(lambda00, phi00);
  centroidStream$1.point = centroidPoint$1;
}

function centroidRingPointFirst(lambda, phi) {
  lambda00 = lambda, phi00 = phi;
  lambda *= radians, phi *= radians;
  centroidStream$1.point = centroidRingPoint;
  var cosPhi = cos$1(phi);
  x0$4 = cosPhi * cos$1(lambda);
  y0$4 = cosPhi * sin$1(lambda);
  z0 = sin$1(phi);
  centroidPointCartesian(x0$4, y0$4, z0);
}

function centroidRingPoint(lambda, phi) {
  lambda *= radians, phi *= radians;
  var cosPhi = cos$1(phi),
      x = cosPhi * cos$1(lambda),
      y = cosPhi * sin$1(lambda),
      z = sin$1(phi),
      cx = y0$4 * z - z0 * y,
      cy = z0 * x - x0$4 * z,
      cz = x0$4 * y - y0$4 * x,
      m = hypot(cx, cy, cz),
      w = asin$1(m), // line weight = angle
      v = m && -w / m; // area weight multiplier
  X2$1.add(v * cx);
  Y2$1.add(v * cy);
  Z2$1.add(v * cz);
  W1 += w;
  X1$1 += w * (x0$4 + (x0$4 = x));
  Y1$1 += w * (y0$4 + (y0$4 = y));
  Z1$1 += w * (z0 + (z0 = z));
  centroidPointCartesian(x0$4, y0$4, z0);
}

function centroid$1(object) {
  W0 = W1 =
  X0$1 = Y0$1 = Z0$1 =
  X1$1 = Y1$1 = Z1$1 = 0;
  X2$1 = new Adder();
  Y2$1 = new Adder();
  Z2$1 = new Adder();
  geoStream(object, centroidStream$1);

  var x = +X2$1,
      y = +Y2$1,
      z = +Z2$1,
      m = hypot(x, y, z);

  // If the area-weighted ccentroid is undefined, fall back to length-weighted ccentroid.
  if (m < epsilon2) {
    x = X1$1, y = Y1$1, z = Z1$1;
    // If the feature has zero length, fall back to arithmetic mean of point vectors.
    if (W1 < epsilon$1) x = X0$1, y = Y0$1, z = Z0$1;
    m = hypot(x, y, z);
    // If the feature still has an undefined ccentroid, then return.
    if (m < epsilon2) return [NaN, NaN];
  }

  return [atan2$1(y, x) * degrees, asin$1(z / m) * degrees];
}

function constant$3(x) {
  return function() {
    return x;
  };
}

function compose(a, b) {

  function compose(x, y) {
    return x = a(x, y), b(x[0], x[1]);
  }

  if (a.invert && b.invert) compose.invert = function(x, y) {
    return x = b.invert(x, y), x && a.invert(x[0], x[1]);
  };

  return compose;
}

function rotationIdentity(lambda, phi) {
  if (abs$1(lambda) > pi$1) lambda -= Math.round(lambda / tau$1) * tau$1;
  return [lambda, phi];
}

rotationIdentity.invert = rotationIdentity;

function rotateRadians(deltaLambda, deltaPhi, deltaGamma) {
  return (deltaLambda %= tau$1) ? (deltaPhi || deltaGamma ? compose(rotationLambda(deltaLambda), rotationPhiGamma(deltaPhi, deltaGamma))
    : rotationLambda(deltaLambda))
    : (deltaPhi || deltaGamma ? rotationPhiGamma(deltaPhi, deltaGamma)
    : rotationIdentity);
}

function forwardRotationLambda(deltaLambda) {
  return function(lambda, phi) {
    lambda += deltaLambda;
    if (abs$1(lambda) > pi$1) lambda -= Math.round(lambda / tau$1) * tau$1;
    return [lambda, phi];
  };
}

function rotationLambda(deltaLambda) {
  var rotation = forwardRotationLambda(deltaLambda);
  rotation.invert = forwardRotationLambda(-deltaLambda);
  return rotation;
}

function rotationPhiGamma(deltaPhi, deltaGamma) {
  var cosDeltaPhi = cos$1(deltaPhi),
      sinDeltaPhi = sin$1(deltaPhi),
      cosDeltaGamma = cos$1(deltaGamma),
      sinDeltaGamma = sin$1(deltaGamma);

  function rotation(lambda, phi) {
    var cosPhi = cos$1(phi),
        x = cos$1(lambda) * cosPhi,
        y = sin$1(lambda) * cosPhi,
        z = sin$1(phi),
        k = z * cosDeltaPhi + x * sinDeltaPhi;
    return [
      atan2$1(y * cosDeltaGamma - k * sinDeltaGamma, x * cosDeltaPhi - z * sinDeltaPhi),
      asin$1(k * cosDeltaGamma + y * sinDeltaGamma)
    ];
  }

  rotation.invert = function(lambda, phi) {
    var cosPhi = cos$1(phi),
        x = cos$1(lambda) * cosPhi,
        y = sin$1(lambda) * cosPhi,
        z = sin$1(phi),
        k = z * cosDeltaGamma - y * sinDeltaGamma;
    return [
      atan2$1(y * cosDeltaGamma + z * sinDeltaGamma, x * cosDeltaPhi + k * sinDeltaPhi),
      asin$1(k * cosDeltaPhi - x * sinDeltaPhi)
    ];
  };

  return rotation;
}

function rotation(rotate) {
  rotate = rotateRadians(rotate[0] * radians, rotate[1] * radians, rotate.length > 2 ? rotate[2] * radians : 0);

  function forward(coordinates) {
    coordinates = rotate(coordinates[0] * radians, coordinates[1] * radians);
    return coordinates[0] *= degrees, coordinates[1] *= degrees, coordinates;
  }

  forward.invert = function(coordinates) {
    coordinates = rotate.invert(coordinates[0] * radians, coordinates[1] * radians);
    return coordinates[0] *= degrees, coordinates[1] *= degrees, coordinates;
  };

  return forward;
}

// Generates a circle centered at [0°, 0°], with a given radius and precision.
function circleStream(stream, radius, delta, direction, t0, t1) {
  if (!delta) return;
  var cosRadius = cos$1(radius),
      sinRadius = sin$1(radius),
      step = direction * delta;
  if (t0 == null) {
    t0 = radius + direction * tau$1;
    t1 = radius - step / 2;
  } else {
    t0 = circleRadius(cosRadius, t0);
    t1 = circleRadius(cosRadius, t1);
    if (direction > 0 ? t0 < t1 : t0 > t1) t0 += direction * tau$1;
  }
  for (var point, t = t0; direction > 0 ? t > t1 : t < t1; t -= step) {
    point = spherical([cosRadius, -sinRadius * cos$1(t), -sinRadius * sin$1(t)]);
    stream.point(point[0], point[1]);
  }
}

// Returns the signed angle of a cartesian point relative to [cosRadius, 0, 0].
function circleRadius(cosRadius, point) {
  point = cartesian(point), point[0] -= cosRadius;
  cartesianNormalizeInPlace(point);
  var radius = acos$1(-point[1]);
  return ((-point[2] < 0 ? -radius : radius) + tau$1 - epsilon$1) % tau$1;
}

function circle$1() {
  var center = constant$3([0, 0]),
      radius = constant$3(90),
      precision = constant$3(2),
      ring,
      rotate,
      stream = {point: point};

  function point(x, y) {
    ring.push(x = rotate(x, y));
    x[0] *= degrees, x[1] *= degrees;
  }

  function circle() {
    var c = center.apply(this, arguments),
        r = radius.apply(this, arguments) * radians,
        p = precision.apply(this, arguments) * radians;
    ring = [];
    rotate = rotateRadians(-c[0] * radians, -c[1] * radians, 0).invert;
    circleStream(stream, r, p, 1);
    c = {type: "Polygon", coordinates: [ring]};
    ring = rotate = null;
    return c;
  }

  circle.center = function(_) {
    return arguments.length ? (center = typeof _ === "function" ? _ : constant$3([+_[0], +_[1]]), circle) : center;
  };

  circle.radius = function(_) {
    return arguments.length ? (radius = typeof _ === "function" ? _ : constant$3(+_), circle) : radius;
  };

  circle.precision = function(_) {
    return arguments.length ? (precision = typeof _ === "function" ? _ : constant$3(+_), circle) : precision;
  };

  return circle;
}

function clipBuffer() {
  var lines = [],
      line;
  return {
    point: function(x, y, m) {
      line.push([x, y, m]);
    },
    lineStart: function() {
      lines.push(line = []);
    },
    lineEnd: noop$1,
    rejoin: function() {
      if (lines.length > 1) lines.push(lines.pop().concat(lines.shift()));
    },
    result: function() {
      var result = lines;
      lines = [];
      line = null;
      return result;
    }
  };
}

function pointEqual(a, b) {
  return abs$1(a[0] - b[0]) < epsilon$1 && abs$1(a[1] - b[1]) < epsilon$1;
}

function Intersection(point, points, other, entry) {
  this.x = point;
  this.z = points;
  this.o = other; // another intersection
  this.e = entry; // is an entry?
  this.v = false; // visited
  this.n = this.p = null; // next & previous
}

// A generalized polygon clipping algorithm: given a polygon that has been cut
// into its visible line segments, and rejoins the segments by interpolating
// along the clip edge.
function clipRejoin(segments, compareIntersection, startInside, interpolate, stream) {
  var subject = [],
      clip = [],
      i,
      n;

  segments.forEach(function(segment) {
    if ((n = segment.length - 1) <= 0) return;
    var n, p0 = segment[0], p1 = segment[n], x;

    if (pointEqual(p0, p1)) {
      if (!p0[2] && !p1[2]) {
        stream.lineStart();
        for (i = 0; i < n; ++i) stream.point((p0 = segment[i])[0], p0[1]);
        stream.lineEnd();
        return;
      }
      // handle degenerate cases by moving the point
      p1[0] += 2 * epsilon$1;
    }

    subject.push(x = new Intersection(p0, segment, null, true));
    clip.push(x.o = new Intersection(p0, null, x, false));
    subject.push(x = new Intersection(p1, segment, null, false));
    clip.push(x.o = new Intersection(p1, null, x, true));
  });

  if (!subject.length) return;

  clip.sort(compareIntersection);
  link$1(subject);
  link$1(clip);

  for (i = 0, n = clip.length; i < n; ++i) {
    clip[i].e = startInside = !startInside;
  }

  var start = subject[0],
      points,
      point;

  while (1) {
    // Find first unvisited intersection.
    var current = start,
        isSubject = true;
    while (current.v) if ((current = current.n) === start) return;
    points = current.z;
    stream.lineStart();
    do {
      current.v = current.o.v = true;
      if (current.e) {
        if (isSubject) {
          for (i = 0, n = points.length; i < n; ++i) stream.point((point = points[i])[0], point[1]);
        } else {
          interpolate(current.x, current.n.x, 1, stream);
        }
        current = current.n;
      } else {
        if (isSubject) {
          points = current.p.z;
          for (i = points.length - 1; i >= 0; --i) stream.point((point = points[i])[0], point[1]);
        } else {
          interpolate(current.x, current.p.x, -1, stream);
        }
        current = current.p;
      }
      current = current.o;
      points = current.z;
      isSubject = !isSubject;
    } while (!current.v);
    stream.lineEnd();
  }
}

function link$1(array) {
  if (!(n = array.length)) return;
  var n,
      i = 0,
      a = array[0],
      b;
  while (++i < n) {
    a.n = b = array[i];
    b.p = a;
    a = b;
  }
  a.n = b = array[0];
  b.p = a;
}

function longitude(point) {
  return abs$1(point[0]) <= pi$1 ? point[0] : sign$1(point[0]) * ((abs$1(point[0]) + pi$1) % tau$1 - pi$1);
}

function polygonContains(polygon, point) {
  var lambda = longitude(point),
      phi = point[1],
      sinPhi = sin$1(phi),
      normal = [sin$1(lambda), -cos$1(lambda), 0],
      angle = 0,
      winding = 0;

  var sum = new Adder();

  if (sinPhi === 1) phi = halfPi$1 + epsilon$1;
  else if (sinPhi === -1) phi = -halfPi$1 - epsilon$1;

  for (var i = 0, n = polygon.length; i < n; ++i) {
    if (!(m = (ring = polygon[i]).length)) continue;
    var ring,
        m,
        point0 = ring[m - 1],
        lambda0 = longitude(point0),
        phi0 = point0[1] / 2 + quarterPi,
        sinPhi0 = sin$1(phi0),
        cosPhi0 = cos$1(phi0);

    for (var j = 0; j < m; ++j, lambda0 = lambda1, sinPhi0 = sinPhi1, cosPhi0 = cosPhi1, point0 = point1) {
      var point1 = ring[j],
          lambda1 = longitude(point1),
          phi1 = point1[1] / 2 + quarterPi,
          sinPhi1 = sin$1(phi1),
          cosPhi1 = cos$1(phi1),
          delta = lambda1 - lambda0,
          sign = delta >= 0 ? 1 : -1,
          absDelta = sign * delta,
          antimeridian = absDelta > pi$1,
          k = sinPhi0 * sinPhi1;

      sum.add(atan2$1(k * sign * sin$1(absDelta), cosPhi0 * cosPhi1 + k * cos$1(absDelta)));
      angle += antimeridian ? delta + sign * tau$1 : delta;

      // Are the longitudes either side of the point’s meridian (lambda),
      // and are the latitudes smaller than the parallel (phi)?
      if (antimeridian ^ lambda0 >= lambda ^ lambda1 >= lambda) {
        var arc = cartesianCross(cartesian(point0), cartesian(point1));
        cartesianNormalizeInPlace(arc);
        var intersection = cartesianCross(normal, arc);
        cartesianNormalizeInPlace(intersection);
        var phiArc = (antimeridian ^ delta >= 0 ? -1 : 1) * asin$1(intersection[2]);
        if (phi > phiArc || phi === phiArc && (arc[0] || arc[1])) {
          winding += antimeridian ^ delta >= 0 ? 1 : -1;
        }
      }
    }
  }

  // First, determine whether the South pole is inside or outside:
  //
  // It is inside if:
  // * the polygon winds around it in a clockwise direction.
  // * the polygon does not (cumulatively) wind around it, but has a negative
  //   (counter-clockwise) area.
  //
  // Second, count the (signed) number of times a segment crosses a lambda
  // from the point to the South pole.  If it is zero, then the point is the
  // same side as the South pole.

  return (angle < -epsilon$1 || angle < epsilon$1 && sum < -epsilon2) ^ (winding & 1);
}

function clip(pointVisible, clipLine, interpolate, start) {
  return function(sink) {
    var line = clipLine(sink),
        ringBuffer = clipBuffer(),
        ringSink = clipLine(ringBuffer),
        polygonStarted = false,
        polygon,
        segments,
        ring;

    var clip = {
      point: point,
      lineStart: lineStart,
      lineEnd: lineEnd,
      polygonStart: function() {
        clip.point = pointRing;
        clip.lineStart = ringStart;
        clip.lineEnd = ringEnd;
        segments = [];
        polygon = [];
      },
      polygonEnd: function() {
        clip.point = point;
        clip.lineStart = lineStart;
        clip.lineEnd = lineEnd;
        segments = merge(segments);
        var startInside = polygonContains(polygon, start);
        if (segments.length) {
          if (!polygonStarted) sink.polygonStart(), polygonStarted = true;
          clipRejoin(segments, compareIntersection, startInside, interpolate, sink);
        } else if (startInside) {
          if (!polygonStarted) sink.polygonStart(), polygonStarted = true;
          sink.lineStart();
          interpolate(null, null, 1, sink);
          sink.lineEnd();
        }
        if (polygonStarted) sink.polygonEnd(), polygonStarted = false;
        segments = polygon = null;
      },
      sphere: function() {
        sink.polygonStart();
        sink.lineStart();
        interpolate(null, null, 1, sink);
        sink.lineEnd();
        sink.polygonEnd();
      }
    };

    function point(lambda, phi) {
      if (pointVisible(lambda, phi)) sink.point(lambda, phi);
    }

    function pointLine(lambda, phi) {
      line.point(lambda, phi);
    }

    function lineStart() {
      clip.point = pointLine;
      line.lineStart();
    }

    function lineEnd() {
      clip.point = point;
      line.lineEnd();
    }

    function pointRing(lambda, phi) {
      ring.push([lambda, phi]);
      ringSink.point(lambda, phi);
    }

    function ringStart() {
      ringSink.lineStart();
      ring = [];
    }

    function ringEnd() {
      pointRing(ring[0][0], ring[0][1]);
      ringSink.lineEnd();

      var clean = ringSink.clean(),
          ringSegments = ringBuffer.result(),
          i, n = ringSegments.length, m,
          segment,
          point;

      ring.pop();
      polygon.push(ring);
      ring = null;

      if (!n) return;

      // No intersections.
      if (clean & 1) {
        segment = ringSegments[0];
        if ((m = segment.length - 1) > 0) {
          if (!polygonStarted) sink.polygonStart(), polygonStarted = true;
          sink.lineStart();
          for (i = 0; i < m; ++i) sink.point((point = segment[i])[0], point[1]);
          sink.lineEnd();
        }
        return;
      }

      // Rejoin connected segments.
      // TODO reuse ringBuffer.rejoin()?
      if (n > 1 && clean & 2) ringSegments.push(ringSegments.pop().concat(ringSegments.shift()));

      segments.push(ringSegments.filter(validSegment));
    }

    return clip;
  };
}

function validSegment(segment) {
  return segment.length > 1;
}

// Intersections are sorted along the clip edge. For both antimeridian cutting
// and circle clipping, the same comparison is used.
function compareIntersection(a, b) {
  return ((a = a.x)[0] < 0 ? a[1] - halfPi$1 - epsilon$1 : halfPi$1 - a[1])
       - ((b = b.x)[0] < 0 ? b[1] - halfPi$1 - epsilon$1 : halfPi$1 - b[1]);
}

const clipAntimeridian = clip(
  function() { return true; },
  clipAntimeridianLine,
  clipAntimeridianInterpolate,
  [-pi$1, -halfPi$1]
);

// Takes a line and cuts into visible segments. Return values: 0 - there were
// intersections or the line was empty; 1 - no intersections; 2 - there were
// intersections, and the first and last segments should be rejoined.
function clipAntimeridianLine(stream) {
  var lambda0 = NaN,
      phi0 = NaN,
      sign0 = NaN,
      clean; // no intersections

  return {
    lineStart: function() {
      stream.lineStart();
      clean = 1;
    },
    point: function(lambda1, phi1) {
      var sign1 = lambda1 > 0 ? pi$1 : -pi$1,
          delta = abs$1(lambda1 - lambda0);
      if (abs$1(delta - pi$1) < epsilon$1) { // line crosses a pole
        stream.point(lambda0, phi0 = (phi0 + phi1) / 2 > 0 ? halfPi$1 : -halfPi$1);
        stream.point(sign0, phi0);
        stream.lineEnd();
        stream.lineStart();
        stream.point(sign1, phi0);
        stream.point(lambda1, phi0);
        clean = 0;
      } else if (sign0 !== sign1 && delta >= pi$1) { // line crosses antimeridian
        if (abs$1(lambda0 - sign0) < epsilon$1) lambda0 -= sign0 * epsilon$1; // handle degeneracies
        if (abs$1(lambda1 - sign1) < epsilon$1) lambda1 -= sign1 * epsilon$1;
        phi0 = clipAntimeridianIntersect(lambda0, phi0, lambda1, phi1);
        stream.point(sign0, phi0);
        stream.lineEnd();
        stream.lineStart();
        stream.point(sign1, phi0);
        clean = 0;
      }
      stream.point(lambda0 = lambda1, phi0 = phi1);
      sign0 = sign1;
    },
    lineEnd: function() {
      stream.lineEnd();
      lambda0 = phi0 = NaN;
    },
    clean: function() {
      return 2 - clean; // if intersections, rejoin first and last segments
    }
  };
}

function clipAntimeridianIntersect(lambda0, phi0, lambda1, phi1) {
  var cosPhi0,
      cosPhi1,
      sinLambda0Lambda1 = sin$1(lambda0 - lambda1);
  return abs$1(sinLambda0Lambda1) > epsilon$1
      ? atan((sin$1(phi0) * (cosPhi1 = cos$1(phi1)) * sin$1(lambda1)
          - sin$1(phi1) * (cosPhi0 = cos$1(phi0)) * sin$1(lambda0))
          / (cosPhi0 * cosPhi1 * sinLambda0Lambda1))
      : (phi0 + phi1) / 2;
}

function clipAntimeridianInterpolate(from, to, direction, stream) {
  var phi;
  if (from == null) {
    phi = direction * halfPi$1;
    stream.point(-pi$1, phi);
    stream.point(0, phi);
    stream.point(pi$1, phi);
    stream.point(pi$1, 0);
    stream.point(pi$1, -phi);
    stream.point(0, -phi);
    stream.point(-pi$1, -phi);
    stream.point(-pi$1, 0);
    stream.point(-pi$1, phi);
  } else if (abs$1(from[0] - to[0]) > epsilon$1) {
    var lambda = from[0] < to[0] ? pi$1 : -pi$1;
    phi = direction * lambda / 2;
    stream.point(-lambda, phi);
    stream.point(0, phi);
    stream.point(lambda, phi);
  } else {
    stream.point(to[0], to[1]);
  }
}

function clipCircle(radius) {
  var cr = cos$1(radius),
      delta = 2 * radians,
      smallRadius = cr > 0,
      notHemisphere = abs$1(cr) > epsilon$1; // TODO optimise for this common case

  function interpolate(from, to, direction, stream) {
    circleStream(stream, radius, delta, direction, from, to);
  }

  function visible(lambda, phi) {
    return cos$1(lambda) * cos$1(phi) > cr;
  }

  // Takes a line and cuts into visible segments. Return values used for polygon
  // clipping: 0 - there were intersections or the line was empty; 1 - no
  // intersections 2 - there were intersections, and the first and last segments
  // should be rejoined.
  function clipLine(stream) {
    var point0, // previous point
        c0, // code for previous point
        v0, // visibility of previous point
        v00, // visibility of first point
        clean; // no intersections
    return {
      lineStart: function() {
        v00 = v0 = false;
        clean = 1;
      },
      point: function(lambda, phi) {
        var point1 = [lambda, phi],
            point2,
            v = visible(lambda, phi),
            c = smallRadius
              ? v ? 0 : code(lambda, phi)
              : v ? code(lambda + (lambda < 0 ? pi$1 : -pi$1), phi) : 0;
        if (!point0 && (v00 = v0 = v)) stream.lineStart();
        if (v !== v0) {
          point2 = intersect(point0, point1);
          if (!point2 || pointEqual(point0, point2) || pointEqual(point1, point2))
            point1[2] = 1;
        }
        if (v !== v0) {
          clean = 0;
          if (v) {
            // outside going in
            stream.lineStart();
            point2 = intersect(point1, point0);
            stream.point(point2[0], point2[1]);
          } else {
            // inside going out
            point2 = intersect(point0, point1);
            stream.point(point2[0], point2[1], 2);
            stream.lineEnd();
          }
          point0 = point2;
        } else if (notHemisphere && point0 && smallRadius ^ v) {
          var t;
          // If the codes for two points are different, or are both zero,
          // and there this segment intersects with the small circle.
          if (!(c & c0) && (t = intersect(point1, point0, true))) {
            clean = 0;
            if (smallRadius) {
              stream.lineStart();
              stream.point(t[0][0], t[0][1]);
              stream.point(t[1][0], t[1][1]);
              stream.lineEnd();
            } else {
              stream.point(t[1][0], t[1][1]);
              stream.lineEnd();
              stream.lineStart();
              stream.point(t[0][0], t[0][1], 3);
            }
          }
        }
        if (v && (!point0 || !pointEqual(point0, point1))) {
          stream.point(point1[0], point1[1]);
        }
        point0 = point1, v0 = v, c0 = c;
      },
      lineEnd: function() {
        if (v0) stream.lineEnd();
        point0 = null;
      },
      // Rejoin first and last segments if there were intersections and the first
      // and last points were visible.
      clean: function() {
        return clean | ((v00 && v0) << 1);
      }
    };
  }

  // Intersects the great circle between a and b with the clip circle.
  function intersect(a, b, two) {
    var pa = cartesian(a),
        pb = cartesian(b);

    // We have two planes, n1.p = d1 and n2.p = d2.
    // Find intersection line p(t) = c1 n1 + c2 n2 + t (n1 ⨯ n2).
    var n1 = [1, 0, 0], // normal
        n2 = cartesianCross(pa, pb),
        n2n2 = cartesianDot(n2, n2),
        n1n2 = n2[0], // cartesianDot(n1, n2),
        determinant = n2n2 - n1n2 * n1n2;

    // Two polar points.
    if (!determinant) return !two && a;

    var c1 =  cr * n2n2 / determinant,
        c2 = -cr * n1n2 / determinant,
        n1xn2 = cartesianCross(n1, n2),
        A = cartesianScale(n1, c1),
        B = cartesianScale(n2, c2);
    cartesianAddInPlace(A, B);

    // Solve |p(t)|^2 = 1.
    var u = n1xn2,
        w = cartesianDot(A, u),
        uu = cartesianDot(u, u),
        t2 = w * w - uu * (cartesianDot(A, A) - 1);

    if (t2 < 0) return;

    var t = sqrt$2(t2),
        q = cartesianScale(u, (-w - t) / uu);
    cartesianAddInPlace(q, A);
    q = spherical(q);

    if (!two) return q;

    // Two intersection points.
    var lambda0 = a[0],
        lambda1 = b[0],
        phi0 = a[1],
        phi1 = b[1],
        z;

    if (lambda1 < lambda0) z = lambda0, lambda0 = lambda1, lambda1 = z;

    var delta = lambda1 - lambda0,
        polar = abs$1(delta - pi$1) < epsilon$1,
        meridian = polar || delta < epsilon$1;

    if (!polar && phi1 < phi0) z = phi0, phi0 = phi1, phi1 = z;

    // Check that the first point is between a and b.
    if (meridian
        ? polar
          ? phi0 + phi1 > 0 ^ q[1] < (abs$1(q[0] - lambda0) < epsilon$1 ? phi0 : phi1)
          : phi0 <= q[1] && q[1] <= phi1
        : delta > pi$1 ^ (lambda0 <= q[0] && q[0] <= lambda1)) {
      var q1 = cartesianScale(u, (-w + t) / uu);
      cartesianAddInPlace(q1, A);
      return [q, spherical(q1)];
    }
  }

  // Generates a 4-bit vector representing the location of a point relative to
  // the small circle's bounding box.
  function code(lambda, phi) {
    var r = smallRadius ? radius : pi$1 - radius,
        code = 0;
    if (lambda < -r) code |= 1; // left
    else if (lambda > r) code |= 2; // right
    if (phi < -r) code |= 4; // below
    else if (phi > r) code |= 8; // above
    return code;
  }

  return clip(visible, clipLine, interpolate, smallRadius ? [0, -radius] : [-pi$1, radius - pi$1]);
}

function clipLine(a, b, x0, y0, x1, y1) {
  var ax = a[0],
      ay = a[1],
      bx = b[0],
      by = b[1],
      t0 = 0,
      t1 = 1,
      dx = bx - ax,
      dy = by - ay,
      r;

  r = x0 - ax;
  if (!dx && r > 0) return;
  r /= dx;
  if (dx < 0) {
    if (r < t0) return;
    if (r < t1) t1 = r;
  } else if (dx > 0) {
    if (r > t1) return;
    if (r > t0) t0 = r;
  }

  r = x1 - ax;
  if (!dx && r < 0) return;
  r /= dx;
  if (dx < 0) {
    if (r > t1) return;
    if (r > t0) t0 = r;
  } else if (dx > 0) {
    if (r < t0) return;
    if (r < t1) t1 = r;
  }

  r = y0 - ay;
  if (!dy && r > 0) return;
  r /= dy;
  if (dy < 0) {
    if (r < t0) return;
    if (r < t1) t1 = r;
  } else if (dy > 0) {
    if (r > t1) return;
    if (r > t0) t0 = r;
  }

  r = y1 - ay;
  if (!dy && r < 0) return;
  r /= dy;
  if (dy < 0) {
    if (r > t1) return;
    if (r > t0) t0 = r;
  } else if (dy > 0) {
    if (r < t0) return;
    if (r < t1) t1 = r;
  }

  if (t0 > 0) a[0] = ax + t0 * dx, a[1] = ay + t0 * dy;
  if (t1 < 1) b[0] = ax + t1 * dx, b[1] = ay + t1 * dy;
  return true;
}

var clipMax = 1e9, clipMin = -clipMax;

// TODO Use d3-polygon’s polygonContains here for the ring check?
// TODO Eliminate duplicate buffering in clipBuffer and polygon.push?

function clipRectangle(x0, y0, x1, y1) {

  function visible(x, y) {
    return x0 <= x && x <= x1 && y0 <= y && y <= y1;
  }

  function interpolate(from, to, direction, stream) {
    var a = 0, a1 = 0;
    if (from == null
        || (a = corner(from, direction)) !== (a1 = corner(to, direction))
        || comparePoint(from, to) < 0 ^ direction > 0) {
      do stream.point(a === 0 || a === 3 ? x0 : x1, a > 1 ? y1 : y0);
      while ((a = (a + direction + 4) % 4) !== a1);
    } else {
      stream.point(to[0], to[1]);
    }
  }

  function corner(p, direction) {
    return abs$1(p[0] - x0) < epsilon$1 ? direction > 0 ? 0 : 3
        : abs$1(p[0] - x1) < epsilon$1 ? direction > 0 ? 2 : 1
        : abs$1(p[1] - y0) < epsilon$1 ? direction > 0 ? 1 : 0
        : direction > 0 ? 3 : 2; // abs(p[1] - y1) < epsilon
  }

  function compareIntersection(a, b) {
    return comparePoint(a.x, b.x);
  }

  function comparePoint(a, b) {
    var ca = corner(a, 1),
        cb = corner(b, 1);
    return ca !== cb ? ca - cb
        : ca === 0 ? b[1] - a[1]
        : ca === 1 ? a[0] - b[0]
        : ca === 2 ? a[1] - b[1]
        : b[0] - a[0];
  }

  return function(stream) {
    var activeStream = stream,
        bufferStream = clipBuffer(),
        segments,
        polygon,
        ring,
        x__, y__, v__, // first point
        x_, y_, v_, // previous point
        first,
        clean;

    var clipStream = {
      point: point,
      lineStart: lineStart,
      lineEnd: lineEnd,
      polygonStart: polygonStart,
      polygonEnd: polygonEnd
    };

    function point(x, y) {
      if (visible(x, y)) activeStream.point(x, y);
    }

    function polygonInside() {
      var winding = 0;

      for (var i = 0, n = polygon.length; i < n; ++i) {
        for (var ring = polygon[i], j = 1, m = ring.length, point = ring[0], a0, a1, b0 = point[0], b1 = point[1]; j < m; ++j) {
          a0 = b0, a1 = b1, point = ring[j], b0 = point[0], b1 = point[1];
          if (a1 <= y1) { if (b1 > y1 && (b0 - a0) * (y1 - a1) > (b1 - a1) * (x0 - a0)) ++winding; }
          else { if (b1 <= y1 && (b0 - a0) * (y1 - a1) < (b1 - a1) * (x0 - a0)) --winding; }
        }
      }

      return winding;
    }

    // Buffer geometry within a polygon and then clip it en masse.
    function polygonStart() {
      activeStream = bufferStream, segments = [], polygon = [], clean = true;
    }

    function polygonEnd() {
      var startInside = polygonInside(),
          cleanInside = clean && startInside,
          visible = (segments = merge(segments)).length;
      if (cleanInside || visible) {
        stream.polygonStart();
        if (cleanInside) {
          stream.lineStart();
          interpolate(null, null, 1, stream);
          stream.lineEnd();
        }
        if (visible) {
          clipRejoin(segments, compareIntersection, startInside, interpolate, stream);
        }
        stream.polygonEnd();
      }
      activeStream = stream, segments = polygon = ring = null;
    }

    function lineStart() {
      clipStream.point = linePoint;
      if (polygon) polygon.push(ring = []);
      first = true;
      v_ = false;
      x_ = y_ = NaN;
    }

    // TODO rather than special-case polygons, simply handle them separately.
    // Ideally, coincident intersection points should be jittered to avoid
    // clipping issues.
    function lineEnd() {
      if (segments) {
        linePoint(x__, y__);
        if (v__ && v_) bufferStream.rejoin();
        segments.push(bufferStream.result());
      }
      clipStream.point = point;
      if (v_) activeStream.lineEnd();
    }

    function linePoint(x, y) {
      var v = visible(x, y);
      if (polygon) ring.push([x, y]);
      if (first) {
        x__ = x, y__ = y, v__ = v;
        first = false;
        if (v) {
          activeStream.lineStart();
          activeStream.point(x, y);
        }
      } else {
        if (v && v_) activeStream.point(x, y);
        else {
          var a = [x_ = Math.max(clipMin, Math.min(clipMax, x_)), y_ = Math.max(clipMin, Math.min(clipMax, y_))],
              b = [x = Math.max(clipMin, Math.min(clipMax, x)), y = Math.max(clipMin, Math.min(clipMax, y))];
          if (clipLine(a, b, x0, y0, x1, y1)) {
            if (!v_) {
              activeStream.lineStart();
              activeStream.point(a[0], a[1]);
            }
            activeStream.point(b[0], b[1]);
            if (!v) activeStream.lineEnd();
            clean = false;
          } else if (v) {
            activeStream.lineStart();
            activeStream.point(x, y);
            clean = false;
          }
        }
      }
      x_ = x, y_ = y, v_ = v;
    }

    return clipStream;
  };
}

function extent() {
  var x0 = 0,
      y0 = 0,
      x1 = 960,
      y1 = 500,
      cache,
      cacheStream,
      clip;

  return clip = {
    stream: function(stream) {
      return cache && cacheStream === stream ? cache : cache = clipRectangle(x0, y0, x1, y1)(cacheStream = stream);
    },
    extent: function(_) {
      return arguments.length ? (x0 = +_[0][0], y0 = +_[0][1], x1 = +_[1][0], y1 = +_[1][1], cache = cacheStream = null, clip) : [[x0, y0], [x1, y1]];
    }
  };
}

var lengthSum$1,
    lambda0,
    sinPhi0,
    cosPhi0;

var lengthStream$1 = {
  sphere: noop$1,
  point: noop$1,
  lineStart: lengthLineStart,
  lineEnd: noop$1,
  polygonStart: noop$1,
  polygonEnd: noop$1
};

function lengthLineStart() {
  lengthStream$1.point = lengthPointFirst$1;
  lengthStream$1.lineEnd = lengthLineEnd;
}

function lengthLineEnd() {
  lengthStream$1.point = lengthStream$1.lineEnd = noop$1;
}

function lengthPointFirst$1(lambda, phi) {
  lambda *= radians, phi *= radians;
  lambda0 = lambda, sinPhi0 = sin$1(phi), cosPhi0 = cos$1(phi);
  lengthStream$1.point = lengthPoint$1;
}

function lengthPoint$1(lambda, phi) {
  lambda *= radians, phi *= radians;
  var sinPhi = sin$1(phi),
      cosPhi = cos$1(phi),
      delta = abs$1(lambda - lambda0),
      cosDelta = cos$1(delta),
      sinDelta = sin$1(delta),
      x = cosPhi * sinDelta,
      y = cosPhi0 * sinPhi - sinPhi0 * cosPhi * cosDelta,
      z = sinPhi0 * sinPhi + cosPhi0 * cosPhi * cosDelta;
  lengthSum$1.add(atan2$1(sqrt$2(x * x + y * y), z));
  lambda0 = lambda, sinPhi0 = sinPhi, cosPhi0 = cosPhi;
}

function length$1(object) {
  lengthSum$1 = new Adder();
  geoStream(object, lengthStream$1);
  return +lengthSum$1;
}

var coordinates = [null, null],
    object = {type: "LineString", coordinates: coordinates};

function distance(a, b) {
  coordinates[0] = a;
  coordinates[1] = b;
  return length$1(object);
}

var containsObjectType = {
  Feature: function(object, point) {
    return containsGeometry(object.geometry, point);
  },
  FeatureCollection: function(object, point) {
    var features = object.features, i = -1, n = features.length;
    while (++i < n) if (containsGeometry(features[i].geometry, point)) return true;
    return false;
  }
};

var containsGeometryType = {
  Sphere: function() {
    return true;
  },
  Point: function(object, point) {
    return containsPoint(object.coordinates, point);
  },
  MultiPoint: function(object, point) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) if (containsPoint(coordinates[i], point)) return true;
    return false;
  },
  LineString: function(object, point) {
    return containsLine(object.coordinates, point);
  },
  MultiLineString: function(object, point) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) if (containsLine(coordinates[i], point)) return true;
    return false;
  },
  Polygon: function(object, point) {
    return containsPolygon(object.coordinates, point);
  },
  MultiPolygon: function(object, point) {
    var coordinates = object.coordinates, i = -1, n = coordinates.length;
    while (++i < n) if (containsPolygon(coordinates[i], point)) return true;
    return false;
  },
  GeometryCollection: function(object, point) {
    var geometries = object.geometries, i = -1, n = geometries.length;
    while (++i < n) if (containsGeometry(geometries[i], point)) return true;
    return false;
  }
};

function containsGeometry(geometry, point) {
  return geometry && containsGeometryType.hasOwnProperty(geometry.type)
      ? containsGeometryType[geometry.type](geometry, point)
      : false;
}

function containsPoint(coordinates, point) {
  return distance(coordinates, point) === 0;
}

function containsLine(coordinates, point) {
  var ao, bo, ab;
  for (var i = 0, n = coordinates.length; i < n; i++) {
    bo = distance(coordinates[i], point);
    if (bo === 0) return true;
    if (i > 0) {
      ab = distance(coordinates[i], coordinates[i - 1]);
      if (
        ab > 0 &&
        ao <= ab &&
        bo <= ab &&
        (ao + bo - ab) * (1 - Math.pow((ao - bo) / ab, 2)) < epsilon2 * ab
      )
        return true;
    }
    ao = bo;
  }
  return false;
}

function containsPolygon(coordinates, point) {
  return !!polygonContains(coordinates.map(ringRadians), pointRadians(point));
}

function ringRadians(ring) {
  return ring = ring.map(pointRadians), ring.pop(), ring;
}

function pointRadians(point) {
  return [point[0] * radians, point[1] * radians];
}

function contains$1(object, point) {
  return (object && containsObjectType.hasOwnProperty(object.type)
      ? containsObjectType[object.type]
      : containsGeometry)(object, point);
}

function graticuleX(y0, y1, dy) {
  var y = range$2(y0, y1 - epsilon$1, dy).concat(y1);
  return function(x) { return y.map(function(y) { return [x, y]; }); };
}

function graticuleY(x0, x1, dx) {
  var x = range$2(x0, x1 - epsilon$1, dx).concat(x1);
  return function(y) { return x.map(function(x) { return [x, y]; }); };
}

function graticule() {
  var x1, x0, X1, X0,
      y1, y0, Y1, Y0,
      dx = 10, dy = dx, DX = 90, DY = 360,
      x, y, X, Y,
      precision = 2.5;

  function graticule() {
    return {type: "MultiLineString", coordinates: lines()};
  }

  function lines() {
    return range$2(ceil(X0 / DX) * DX, X1, DX).map(X)
        .concat(range$2(ceil(Y0 / DY) * DY, Y1, DY).map(Y))
        .concat(range$2(ceil(x0 / dx) * dx, x1, dx).filter(function(x) { return abs$1(x % DX) > epsilon$1; }).map(x))
        .concat(range$2(ceil(y0 / dy) * dy, y1, dy).filter(function(y) { return abs$1(y % DY) > epsilon$1; }).map(y));
  }

  graticule.lines = function() {
    return lines().map(function(coordinates) { return {type: "LineString", coordinates: coordinates}; });
  };

  graticule.outline = function() {
    return {
      type: "Polygon",
      coordinates: [
        X(X0).concat(
        Y(Y1).slice(1),
        X(X1).reverse().slice(1),
        Y(Y0).reverse().slice(1))
      ]
    };
  };

  graticule.extent = function(_) {
    if (!arguments.length) return graticule.extentMinor();
    return graticule.extentMajor(_).extentMinor(_);
  };

  graticule.extentMajor = function(_) {
    if (!arguments.length) return [[X0, Y0], [X1, Y1]];
    X0 = +_[0][0], X1 = +_[1][0];
    Y0 = +_[0][1], Y1 = +_[1][1];
    if (X0 > X1) _ = X0, X0 = X1, X1 = _;
    if (Y0 > Y1) _ = Y0, Y0 = Y1, Y1 = _;
    return graticule.precision(precision);
  };

  graticule.extentMinor = function(_) {
    if (!arguments.length) return [[x0, y0], [x1, y1]];
    x0 = +_[0][0], x1 = +_[1][0];
    y0 = +_[0][1], y1 = +_[1][1];
    if (x0 > x1) _ = x0, x0 = x1, x1 = _;
    if (y0 > y1) _ = y0, y0 = y1, y1 = _;
    return graticule.precision(precision);
  };

  graticule.step = function(_) {
    if (!arguments.length) return graticule.stepMinor();
    return graticule.stepMajor(_).stepMinor(_);
  };

  graticule.stepMajor = function(_) {
    if (!arguments.length) return [DX, DY];
    DX = +_[0], DY = +_[1];
    return graticule;
  };

  graticule.stepMinor = function(_) {
    if (!arguments.length) return [dx, dy];
    dx = +_[0], dy = +_[1];
    return graticule;
  };

  graticule.precision = function(_) {
    if (!arguments.length) return precision;
    precision = +_;
    x = graticuleX(y0, y1, 90);
    y = graticuleY(x0, x1, precision);
    X = graticuleX(Y0, Y1, 90);
    Y = graticuleY(X0, X1, precision);
    return graticule;
  };

  return graticule
      .extentMajor([[-180, -90 + epsilon$1], [180, 90 - epsilon$1]])
      .extentMinor([[-180, -80 - epsilon$1], [180, 80 + epsilon$1]]);
}

function graticule10() {
  return graticule()();
}

function interpolate(a, b) {
  var x0 = a[0] * radians,
      y0 = a[1] * radians,
      x1 = b[0] * radians,
      y1 = b[1] * radians,
      cy0 = cos$1(y0),
      sy0 = sin$1(y0),
      cy1 = cos$1(y1),
      sy1 = sin$1(y1),
      kx0 = cy0 * cos$1(x0),
      ky0 = cy0 * sin$1(x0),
      kx1 = cy1 * cos$1(x1),
      ky1 = cy1 * sin$1(x1),
      d = 2 * asin$1(sqrt$2(haversin(y1 - y0) + cy0 * cy1 * haversin(x1 - x0))),
      k = sin$1(d);

  var interpolate = d ? function(t) {
    var B = sin$1(t *= d) / k,
        A = sin$1(d - t) / k,
        x = A * kx0 + B * kx1,
        y = A * ky0 + B * ky1,
        z = A * sy0 + B * sy1;
    return [
      atan2$1(y, x) * degrees,
      atan2$1(z, sqrt$2(x * x + y * y)) * degrees
    ];
  } : function() {
    return [x0 * degrees, y0 * degrees];
  };

  interpolate.distance = d;

  return interpolate;
}

const identity$5 = x => x;

var areaSum = new Adder(),
    areaRingSum = new Adder(),
    x00$2,
    y00$2,
    x0$3,
    y0$3;

var areaStream = {
  point: noop$1,
  lineStart: noop$1,
  lineEnd: noop$1,
  polygonStart: function() {
    areaStream.lineStart = areaRingStart;
    areaStream.lineEnd = areaRingEnd;
  },
  polygonEnd: function() {
    areaStream.lineStart = areaStream.lineEnd = areaStream.point = noop$1;
    areaSum.add(abs$1(areaRingSum));
    areaRingSum = new Adder();
  },
  result: function() {
    var area = areaSum / 2;
    areaSum = new Adder();
    return area;
  }
};

function areaRingStart() {
  areaStream.point = areaPointFirst;
}

function areaPointFirst(x, y) {
  areaStream.point = areaPoint;
  x00$2 = x0$3 = x, y00$2 = y0$3 = y;
}

function areaPoint(x, y) {
  areaRingSum.add(y0$3 * x - x0$3 * y);
  x0$3 = x, y0$3 = y;
}

function areaRingEnd() {
  areaPoint(x00$2, y00$2);
}

var x0$2 = Infinity,
    y0$2 = x0$2,
    x1 = -x0$2,
    y1 = x1;

var boundsStream = {
  point: boundsPoint,
  lineStart: noop$1,
  lineEnd: noop$1,
  polygonStart: noop$1,
  polygonEnd: noop$1,
  result: function() {
    var bounds = [[x0$2, y0$2], [x1, y1]];
    x1 = y1 = -(y0$2 = x0$2 = Infinity);
    return bounds;
  }
};

function boundsPoint(x, y) {
  if (x < x0$2) x0$2 = x;
  if (x > x1) x1 = x;
  if (y < y0$2) y0$2 = y;
  if (y > y1) y1 = y;
}

// TODO Enforce positive area for exterior, negative area for interior?

var X0 = 0,
    Y0 = 0,
    Z0 = 0,
    X1 = 0,
    Y1 = 0,
    Z1 = 0,
    X2 = 0,
    Y2 = 0,
    Z2 = 0,
    x00$1,
    y00$1,
    x0$1,
    y0$1;

var centroidStream = {
  point: centroidPoint,
  lineStart: centroidLineStart,
  lineEnd: centroidLineEnd,
  polygonStart: function() {
    centroidStream.lineStart = centroidRingStart;
    centroidStream.lineEnd = centroidRingEnd;
  },
  polygonEnd: function() {
    centroidStream.point = centroidPoint;
    centroidStream.lineStart = centroidLineStart;
    centroidStream.lineEnd = centroidLineEnd;
  },
  result: function() {
    var centroid = Z2 ? [X2 / Z2, Y2 / Z2]
        : Z1 ? [X1 / Z1, Y1 / Z1]
        : Z0 ? [X0 / Z0, Y0 / Z0]
        : [NaN, NaN];
    X0 = Y0 = Z0 =
    X1 = Y1 = Z1 =
    X2 = Y2 = Z2 = 0;
    return centroid;
  }
};

function centroidPoint(x, y) {
  X0 += x;
  Y0 += y;
  ++Z0;
}

function centroidLineStart() {
  centroidStream.point = centroidPointFirstLine;
}

function centroidPointFirstLine(x, y) {
  centroidStream.point = centroidPointLine;
  centroidPoint(x0$1 = x, y0$1 = y);
}

function centroidPointLine(x, y) {
  var dx = x - x0$1, dy = y - y0$1, z = sqrt$2(dx * dx + dy * dy);
  X1 += z * (x0$1 + x) / 2;
  Y1 += z * (y0$1 + y) / 2;
  Z1 += z;
  centroidPoint(x0$1 = x, y0$1 = y);
}

function centroidLineEnd() {
  centroidStream.point = centroidPoint;
}

function centroidRingStart() {
  centroidStream.point = centroidPointFirstRing;
}

function centroidRingEnd() {
  centroidPointRing(x00$1, y00$1);
}

function centroidPointFirstRing(x, y) {
  centroidStream.point = centroidPointRing;
  centroidPoint(x00$1 = x0$1 = x, y00$1 = y0$1 = y);
}

function centroidPointRing(x, y) {
  var dx = x - x0$1,
      dy = y - y0$1,
      z = sqrt$2(dx * dx + dy * dy);

  X1 += z * (x0$1 + x) / 2;
  Y1 += z * (y0$1 + y) / 2;
  Z1 += z;

  z = y0$1 * x - x0$1 * y;
  X2 += z * (x0$1 + x);
  Y2 += z * (y0$1 + y);
  Z2 += z * 3;
  centroidPoint(x0$1 = x, y0$1 = y);
}

function PathContext(context) {
  this._context = context;
}

PathContext.prototype = {
  _radius: 4.5,
  pointRadius: function(_) {
    return this._radius = _, this;
  },
  polygonStart: function() {
    this._line = 0;
  },
  polygonEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._point = 0;
  },
  lineEnd: function() {
    if (this._line === 0) this._context.closePath();
    this._point = NaN;
  },
  point: function(x, y) {
    switch (this._point) {
      case 0: {
        this._context.moveTo(x, y);
        this._point = 1;
        break;
      }
      case 1: {
        this._context.lineTo(x, y);
        break;
      }
      default: {
        this._context.moveTo(x + this._radius, y);
        this._context.arc(x, y, this._radius, 0, tau$1);
        break;
      }
    }
  },
  result: noop$1
};

var lengthSum = new Adder(),
    lengthRing,
    x00,
    y00,
    x0,
    y0;

var lengthStream = {
  point: noop$1,
  lineStart: function() {
    lengthStream.point = lengthPointFirst;
  },
  lineEnd: function() {
    if (lengthRing) lengthPoint(x00, y00);
    lengthStream.point = noop$1;
  },
  polygonStart: function() {
    lengthRing = true;
  },
  polygonEnd: function() {
    lengthRing = null;
  },
  result: function() {
    var length = +lengthSum;
    lengthSum = new Adder();
    return length;
  }
};

function lengthPointFirst(x, y) {
  lengthStream.point = lengthPoint;
  x00 = x0 = x, y00 = y0 = y;
}

function lengthPoint(x, y) {
  x0 -= x, y0 -= y;
  lengthSum.add(sqrt$2(x0 * x0 + y0 * y0));
  x0 = x, y0 = y;
}

// Simple caching for constant-radius points.
let cacheDigits, cacheAppend, cacheRadius, cacheCircle;

class PathString {
  constructor(digits) {
    this._append = digits == null ? append : appendRound(digits);
    this._radius = 4.5;
    this._ = "";
  }
  pointRadius(_) {
    this._radius = +_;
    return this;
  }
  polygonStart() {
    this._line = 0;
  }
  polygonEnd() {
    this._line = NaN;
  }
  lineStart() {
    this._point = 0;
  }
  lineEnd() {
    if (this._line === 0) this._ += "Z";
    this._point = NaN;
  }
  point(x, y) {
    switch (this._point) {
      case 0: {
        this._append`M${x},${y}`;
        this._point = 1;
        break;
      }
      case 1: {
        this._append`L${x},${y}`;
        break;
      }
      default: {
        this._append`M${x},${y}`;
        if (this._radius !== cacheRadius || this._append !== cacheAppend) {
          const r = this._radius;
          const s = this._;
          this._ = ""; // stash the old string so we can cache the circle path fragment
          this._append`m0,${r}a${r},${r} 0 1,1 0,${-2 * r}a${r},${r} 0 1,1 0,${2 * r}z`;
          cacheRadius = r;
          cacheAppend = this._append;
          cacheCircle = this._;
          this._ = s;
        }
        this._ += cacheCircle;
        break;
      }
    }
  }
  result() {
    const result = this._;
    this._ = "";
    return result.length ? result : null;
  }
}

function append(strings) {
  let i = 1;
  this._ += strings[0];
  for (const j = strings.length; i < j; ++i) {
    this._ += arguments[i] + strings[i];
  }
}

function appendRound(digits) {
  const d = Math.floor(digits);
  if (!(d >= 0)) throw new RangeError(`invalid digits: ${digits}`);
  if (d > 15) return append;
  if (d !== cacheDigits) {
    const k = 10 ** d;
    cacheDigits = d;
    cacheAppend = function append(strings) {
      let i = 1;
      this._ += strings[0];
      for (const j = strings.length; i < j; ++i) {
        this._ += Math.round(arguments[i] * k) / k + strings[i];
      }
    };
  }
  return cacheAppend;
}

function index$2(projection, context) {
  let digits = 3,
      pointRadius = 4.5,
      projectionStream,
      contextStream;

  function path(object) {
    if (object) {
      if (typeof pointRadius === "function") contextStream.pointRadius(+pointRadius.apply(this, arguments));
      geoStream(object, projectionStream(contextStream));
    }
    return contextStream.result();
  }

  path.area = function(object) {
    geoStream(object, projectionStream(areaStream));
    return areaStream.result();
  };

  path.measure = function(object) {
    geoStream(object, projectionStream(lengthStream));
    return lengthStream.result();
  };

  path.bounds = function(object) {
    geoStream(object, projectionStream(boundsStream));
    return boundsStream.result();
  };

  path.centroid = function(object) {
    geoStream(object, projectionStream(centroidStream));
    return centroidStream.result();
  };

  path.projection = function(_) {
    if (!arguments.length) return projection;
    projectionStream = _ == null ? (projection = null, identity$5) : (projection = _).stream;
    return path;
  };

  path.context = function(_) {
    if (!arguments.length) return context;
    contextStream = _ == null ? (context = null, new PathString(digits)) : new PathContext(context = _);
    if (typeof pointRadius !== "function") contextStream.pointRadius(pointRadius);
    return path;
  };

  path.pointRadius = function(_) {
    if (!arguments.length) return pointRadius;
    pointRadius = typeof _ === "function" ? _ : (contextStream.pointRadius(+_), +_);
    return path;
  };

  path.digits = function(_) {
    if (!arguments.length) return digits;
    if (_ == null) digits = null;
    else {
      const d = Math.floor(_);
      if (!(d >= 0)) throw new RangeError(`invalid digits: ${_}`);
      digits = d;
    }
    if (context === null) contextStream = new PathString(digits);
    return path;
  };

  return path.projection(projection).digits(digits).context(context);
}

function transform$1(methods) {
  return {
    stream: transformer$3(methods)
  };
}

function transformer$3(methods) {
  return function(stream) {
    var s = new TransformStream;
    for (var key in methods) s[key] = methods[key];
    s.stream = stream;
    return s;
  };
}

function TransformStream() {}

TransformStream.prototype = {
  constructor: TransformStream,
  point: function(x, y) { this.stream.point(x, y); },
  sphere: function() { this.stream.sphere(); },
  lineStart: function() { this.stream.lineStart(); },
  lineEnd: function() { this.stream.lineEnd(); },
  polygonStart: function() { this.stream.polygonStart(); },
  polygonEnd: function() { this.stream.polygonEnd(); }
};

function fit(projection, fitBounds, object) {
  var clip = projection.clipExtent && projection.clipExtent();
  projection.scale(150).translate([0, 0]);
  if (clip != null) projection.clipExtent(null);
  geoStream(object, projection.stream(boundsStream));
  fitBounds(boundsStream.result());
  if (clip != null) projection.clipExtent(clip);
  return projection;
}

function fitExtent(projection, extent, object) {
  return fit(projection, function(b) {
    var w = extent[1][0] - extent[0][0],
        h = extent[1][1] - extent[0][1],
        k = Math.min(w / (b[1][0] - b[0][0]), h / (b[1][1] - b[0][1])),
        x = +extent[0][0] + (w - k * (b[1][0] + b[0][0])) / 2,
        y = +extent[0][1] + (h - k * (b[1][1] + b[0][1])) / 2;
    projection.scale(150 * k).translate([x, y]);
  }, object);
}

function fitSize(projection, size, object) {
  return fitExtent(projection, [[0, 0], size], object);
}

function fitWidth(projection, width, object) {
  return fit(projection, function(b) {
    var w = +width,
        k = w / (b[1][0] - b[0][0]),
        x = (w - k * (b[1][0] + b[0][0])) / 2,
        y = -k * b[0][1];
    projection.scale(150 * k).translate([x, y]);
  }, object);
}

function fitHeight(projection, height, object) {
  return fit(projection, function(b) {
    var h = +height,
        k = h / (b[1][1] - b[0][1]),
        x = -k * b[0][0],
        y = (h - k * (b[1][1] + b[0][1])) / 2;
    projection.scale(150 * k).translate([x, y]);
  }, object);
}

var maxDepth = 16, // maximum depth of subdivision
    cosMinDistance = cos$1(30 * radians); // cos(minimum angular distance)

function resample(project, delta2) {
  return +delta2 ? resample$1(project, delta2) : resampleNone(project);
}

function resampleNone(project) {
  return transformer$3({
    point: function(x, y) {
      x = project(x, y);
      this.stream.point(x[0], x[1]);
    }
  });
}

function resample$1(project, delta2) {

  function resampleLineTo(x0, y0, lambda0, a0, b0, c0, x1, y1, lambda1, a1, b1, c1, depth, stream) {
    var dx = x1 - x0,
        dy = y1 - y0,
        d2 = dx * dx + dy * dy;
    if (d2 > 4 * delta2 && depth--) {
      var a = a0 + a1,
          b = b0 + b1,
          c = c0 + c1,
          m = sqrt$2(a * a + b * b + c * c),
          phi2 = asin$1(c /= m),
          lambda2 = abs$1(abs$1(c) - 1) < epsilon$1 || abs$1(lambda0 - lambda1) < epsilon$1 ? (lambda0 + lambda1) / 2 : atan2$1(b, a),
          p = project(lambda2, phi2),
          x2 = p[0],
          y2 = p[1],
          dx2 = x2 - x0,
          dy2 = y2 - y0,
          dz = dy * dx2 - dx * dy2;
      if (dz * dz / d2 > delta2 // perpendicular projected distance
          || abs$1((dx * dx2 + dy * dy2) / d2 - 0.5) > 0.3 // midpoint close to an end
          || a0 * a1 + b0 * b1 + c0 * c1 < cosMinDistance) { // angular distance
        resampleLineTo(x0, y0, lambda0, a0, b0, c0, x2, y2, lambda2, a /= m, b /= m, c, depth, stream);
        stream.point(x2, y2);
        resampleLineTo(x2, y2, lambda2, a, b, c, x1, y1, lambda1, a1, b1, c1, depth, stream);
      }
    }
  }
  return function(stream) {
    var lambda00, x00, y00, a00, b00, c00, // first point
        lambda0, x0, y0, a0, b0, c0; // previous point

    var resampleStream = {
      point: point,
      lineStart: lineStart,
      lineEnd: lineEnd,
      polygonStart: function() { stream.polygonStart(); resampleStream.lineStart = ringStart; },
      polygonEnd: function() { stream.polygonEnd(); resampleStream.lineStart = lineStart; }
    };

    function point(x, y) {
      x = project(x, y);
      stream.point(x[0], x[1]);
    }

    function lineStart() {
      x0 = NaN;
      resampleStream.point = linePoint;
      stream.lineStart();
    }

    function linePoint(lambda, phi) {
      var c = cartesian([lambda, phi]), p = project(lambda, phi);
      resampleLineTo(x0, y0, lambda0, a0, b0, c0, x0 = p[0], y0 = p[1], lambda0 = lambda, a0 = c[0], b0 = c[1], c0 = c[2], maxDepth, stream);
      stream.point(x0, y0);
    }

    function lineEnd() {
      resampleStream.point = point;
      stream.lineEnd();
    }

    function ringStart() {
      lineStart();
      resampleStream.point = ringPoint;
      resampleStream.lineEnd = ringEnd;
    }

    function ringPoint(lambda, phi) {
      linePoint(lambda00 = lambda, phi), x00 = x0, y00 = y0, a00 = a0, b00 = b0, c00 = c0;
      resampleStream.point = linePoint;
    }

    function ringEnd() {
      resampleLineTo(x0, y0, lambda0, a0, b0, c0, x00, y00, lambda00, a00, b00, c00, maxDepth, stream);
      resampleStream.lineEnd = lineEnd;
      lineEnd();
    }

    return resampleStream;
  };
}

var transformRadians = transformer$3({
  point: function(x, y) {
    this.stream.point(x * radians, y * radians);
  }
});

function transformRotate(rotate) {
  return transformer$3({
    point: function(x, y) {
      var r = rotate(x, y);
      return this.stream.point(r[0], r[1]);
    }
  });
}

function scaleTranslate(k, dx, dy, sx, sy) {
  function transform(x, y) {
    x *= sx; y *= sy;
    return [dx + k * x, dy - k * y];
  }
  transform.invert = function(x, y) {
    return [(x - dx) / k * sx, (dy - y) / k * sy];
  };
  return transform;
}

function scaleTranslateRotate(k, dx, dy, sx, sy, alpha) {
  if (!alpha) return scaleTranslate(k, dx, dy, sx, sy);
  var cosAlpha = cos$1(alpha),
      sinAlpha = sin$1(alpha),
      a = cosAlpha * k,
      b = sinAlpha * k,
      ai = cosAlpha / k,
      bi = sinAlpha / k,
      ci = (sinAlpha * dy - cosAlpha * dx) / k,
      fi = (sinAlpha * dx + cosAlpha * dy) / k;
  function transform(x, y) {
    x *= sx; y *= sy;
    return [a * x - b * y + dx, dy - b * x - a * y];
  }
  transform.invert = function(x, y) {
    return [sx * (ai * x - bi * y + ci), sy * (fi - bi * x - ai * y)];
  };
  return transform;
}

function projection(project) {
  return projectionMutator(function() { return project; })();
}

function projectionMutator(projectAt) {
  var project,
      k = 150, // scale
      x = 480, y = 250, // translate
      lambda = 0, phi = 0, // center
      deltaLambda = 0, deltaPhi = 0, deltaGamma = 0, rotate, // pre-rotate
      alpha = 0, // post-rotate angle
      sx = 1, // reflectX
      sy = 1, // reflectX
      theta = null, preclip = clipAntimeridian, // pre-clip angle
      x0 = null, y0, x1, y1, postclip = identity$5, // post-clip extent
      delta2 = 0.5, // precision
      projectResample,
      projectTransform,
      projectRotateTransform,
      cache,
      cacheStream;

  function projection(point) {
    return projectRotateTransform(point[0] * radians, point[1] * radians);
  }

  function invert(point) {
    point = projectRotateTransform.invert(point[0], point[1]);
    return point && [point[0] * degrees, point[1] * degrees];
  }

  projection.stream = function(stream) {
    return cache && cacheStream === stream ? cache : cache = transformRadians(transformRotate(rotate)(preclip(projectResample(postclip(cacheStream = stream)))));
  };

  projection.preclip = function(_) {
    return arguments.length ? (preclip = _, theta = undefined, reset()) : preclip;
  };

  projection.postclip = function(_) {
    return arguments.length ? (postclip = _, x0 = y0 = x1 = y1 = null, reset()) : postclip;
  };

  projection.clipAngle = function(_) {
    return arguments.length ? (preclip = +_ ? clipCircle(theta = _ * radians) : (theta = null, clipAntimeridian), reset()) : theta * degrees;
  };

  projection.clipExtent = function(_) {
    return arguments.length ? (postclip = _ == null ? (x0 = y0 = x1 = y1 = null, identity$5) : clipRectangle(x0 = +_[0][0], y0 = +_[0][1], x1 = +_[1][0], y1 = +_[1][1]), reset()) : x0 == null ? null : [[x0, y0], [x1, y1]];
  };

  projection.scale = function(_) {
    return arguments.length ? (k = +_, recenter()) : k;
  };

  projection.translate = function(_) {
    return arguments.length ? (x = +_[0], y = +_[1], recenter()) : [x, y];
  };

  projection.center = function(_) {
    return arguments.length ? (lambda = _[0] % 360 * radians, phi = _[1] % 360 * radians, recenter()) : [lambda * degrees, phi * degrees];
  };

  projection.rotate = function(_) {
    return arguments.length ? (deltaLambda = _[0] % 360 * radians, deltaPhi = _[1] % 360 * radians, deltaGamma = _.length > 2 ? _[2] % 360 * radians : 0, recenter()) : [deltaLambda * degrees, deltaPhi * degrees, deltaGamma * degrees];
  };

  projection.angle = function(_) {
    return arguments.length ? (alpha = _ % 360 * radians, recenter()) : alpha * degrees;
  };

  projection.reflectX = function(_) {
    return arguments.length ? (sx = _ ? -1 : 1, recenter()) : sx < 0;
  };

  projection.reflectY = function(_) {
    return arguments.length ? (sy = _ ? -1 : 1, recenter()) : sy < 0;
  };

  projection.precision = function(_) {
    return arguments.length ? (projectResample = resample(projectTransform, delta2 = _ * _), reset()) : sqrt$2(delta2);
  };

  projection.fitExtent = function(extent, object) {
    return fitExtent(projection, extent, object);
  };

  projection.fitSize = function(size, object) {
    return fitSize(projection, size, object);
  };

  projection.fitWidth = function(width, object) {
    return fitWidth(projection, width, object);
  };

  projection.fitHeight = function(height, object) {
    return fitHeight(projection, height, object);
  };

  function recenter() {
    var center = scaleTranslateRotate(k, 0, 0, sx, sy, alpha).apply(null, project(lambda, phi)),
        transform = scaleTranslateRotate(k, x - center[0], y - center[1], sx, sy, alpha);
    rotate = rotateRadians(deltaLambda, deltaPhi, deltaGamma);
    projectTransform = compose(project, transform);
    projectRotateTransform = compose(rotate, projectTransform);
    projectResample = resample(projectTransform, delta2);
    return reset();
  }

  function reset() {
    cache = cacheStream = null;
    return projection;
  }

  return function() {
    project = projectAt.apply(this, arguments);
    projection.invert = project.invert && invert;
    return recenter();
  };
}

function conicProjection(projectAt) {
  var phi0 = 0,
      phi1 = pi$1 / 3,
      m = projectionMutator(projectAt),
      p = m(phi0, phi1);

  p.parallels = function(_) {
    return arguments.length ? m(phi0 = _[0] * radians, phi1 = _[1] * radians) : [phi0 * degrees, phi1 * degrees];
  };

  return p;
}

function cylindricalEqualAreaRaw(phi0) {
  var cosPhi0 = cos$1(phi0);

  function forward(lambda, phi) {
    return [lambda * cosPhi0, sin$1(phi) / cosPhi0];
  }

  forward.invert = function(x, y) {
    return [x / cosPhi0, asin$1(y * cosPhi0)];
  };

  return forward;
}

function conicEqualAreaRaw(y0, y1) {
  var sy0 = sin$1(y0), n = (sy0 + sin$1(y1)) / 2;

  // Are the parallels symmetrical around the Equator?
  if (abs$1(n) < epsilon$1) return cylindricalEqualAreaRaw(y0);

  var c = 1 + sy0 * (2 * n - sy0), r0 = sqrt$2(c) / n;

  function project(x, y) {
    var r = sqrt$2(c - 2 * n * sin$1(y)) / n;
    return [r * sin$1(x *= n), r0 - r * cos$1(x)];
  }

  project.invert = function(x, y) {
    var r0y = r0 - y,
        l = atan2$1(x, abs$1(r0y)) * sign$1(r0y);
    if (r0y * n < 0)
      l -= pi$1 * sign$1(x) * sign$1(r0y);
    return [l / n, asin$1((c - (x * x + r0y * r0y) * n * n) / (2 * n))];
  };

  return project;
}

function conicEqualArea() {
  return conicProjection(conicEqualAreaRaw)
      .scale(155.424)
      .center([0, 33.6442]);
}

function albers() {
  return conicEqualArea()
      .parallels([29.5, 45.5])
      .scale(1070)
      .translate([480, 250])
      .rotate([96, 0])
      .center([-0.6, 38.7]);
}

// The projections must have mutually exclusive clip regions on the sphere,
// as this will avoid emitting interleaving lines and polygons.
function multiplex(streams) {
  var n = streams.length;
  return {
    point: function(x, y) { var i = -1; while (++i < n) streams[i].point(x, y); },
    sphere: function() { var i = -1; while (++i < n) streams[i].sphere(); },
    lineStart: function() { var i = -1; while (++i < n) streams[i].lineStart(); },
    lineEnd: function() { var i = -1; while (++i < n) streams[i].lineEnd(); },
    polygonStart: function() { var i = -1; while (++i < n) streams[i].polygonStart(); },
    polygonEnd: function() { var i = -1; while (++i < n) streams[i].polygonEnd(); }
  };
}

// A composite projection for the United States, configured by default for
// 960×500. The projection also works quite well at 960×600 if you change the
// scale to 1285 and adjust the translate accordingly. The set of standard
// parallels for each region comes from USGS, which is published here:
// http://egsc.usgs.gov/isb/pubs/MapProjections/projections.html#albers
function albersUsa() {
  var cache,
      cacheStream,
      lower48 = albers(), lower48Point,
      alaska = conicEqualArea().rotate([154, 0]).center([-2, 58.5]).parallels([55, 65]), alaskaPoint, // EPSG:3338
      hawaii = conicEqualArea().rotate([157, 0]).center([-3, 19.9]).parallels([8, 18]), hawaiiPoint, // ESRI:102007
      point, pointStream = {point: function(x, y) { point = [x, y]; }};

  function albersUsa(coordinates) {
    var x = coordinates[0], y = coordinates[1];
    return point = null,
        (lower48Point.point(x, y), point)
        || (alaskaPoint.point(x, y), point)
        || (hawaiiPoint.point(x, y), point);
  }

  albersUsa.invert = function(coordinates) {
    var k = lower48.scale(),
        t = lower48.translate(),
        x = (coordinates[0] - t[0]) / k,
        y = (coordinates[1] - t[1]) / k;
    return (y >= 0.120 && y < 0.234 && x >= -0.425 && x < -0.214 ? alaska
        : y >= 0.166 && y < 0.234 && x >= -0.214 && x < -0.115 ? hawaii
        : lower48).invert(coordinates);
  };

  albersUsa.stream = function(stream) {
    return cache && cacheStream === stream ? cache : cache = multiplex([lower48.stream(cacheStream = stream), alaska.stream(stream), hawaii.stream(stream)]);
  };

  albersUsa.precision = function(_) {
    if (!arguments.length) return lower48.precision();
    lower48.precision(_), alaska.precision(_), hawaii.precision(_);
    return reset();
  };

  albersUsa.scale = function(_) {
    if (!arguments.length) return lower48.scale();
    lower48.scale(_), alaska.scale(_ * 0.35), hawaii.scale(_);
    return albersUsa.translate(lower48.translate());
  };

  albersUsa.translate = function(_) {
    if (!arguments.length) return lower48.translate();
    var k = lower48.scale(), x = +_[0], y = +_[1];

    lower48Point = lower48
        .translate(_)
        .clipExtent([[x - 0.455 * k, y - 0.238 * k], [x + 0.455 * k, y + 0.238 * k]])
        .stream(pointStream);

    alaskaPoint = alaska
        .translate([x - 0.307 * k, y + 0.201 * k])
        .clipExtent([[x - 0.425 * k + epsilon$1, y + 0.120 * k + epsilon$1], [x - 0.214 * k - epsilon$1, y + 0.234 * k - epsilon$1]])
        .stream(pointStream);

    hawaiiPoint = hawaii
        .translate([x - 0.205 * k, y + 0.212 * k])
        .clipExtent([[x - 0.214 * k + epsilon$1, y + 0.166 * k + epsilon$1], [x - 0.115 * k - epsilon$1, y + 0.234 * k - epsilon$1]])
        .stream(pointStream);

    return reset();
  };

  albersUsa.fitExtent = function(extent, object) {
    return fitExtent(albersUsa, extent, object);
  };

  albersUsa.fitSize = function(size, object) {
    return fitSize(albersUsa, size, object);
  };

  albersUsa.fitWidth = function(width, object) {
    return fitWidth(albersUsa, width, object);
  };

  albersUsa.fitHeight = function(height, object) {
    return fitHeight(albersUsa, height, object);
  };

  function reset() {
    cache = cacheStream = null;
    return albersUsa;
  }

  return albersUsa.scale(1070);
}

function azimuthalRaw(scale) {
  return function(x, y) {
    var cx = cos$1(x),
        cy = cos$1(y),
        k = scale(cx * cy);
        if (k === Infinity) return [2, 0];
    return [
      k * cy * sin$1(x),
      k * sin$1(y)
    ];
  }
}

function azimuthalInvert(angle) {
  return function(x, y) {
    var z = sqrt$2(x * x + y * y),
        c = angle(z),
        sc = sin$1(c),
        cc = cos$1(c);
    return [
      atan2$1(x * sc, z * cc),
      asin$1(z && y * sc / z)
    ];
  }
}

var azimuthalEqualAreaRaw = azimuthalRaw(function(cxcy) {
  return sqrt$2(2 / (1 + cxcy));
});

azimuthalEqualAreaRaw.invert = azimuthalInvert(function(z) {
  return 2 * asin$1(z / 2);
});

function azimuthalEqualArea() {
  return projection(azimuthalEqualAreaRaw)
      .scale(124.75)
      .clipAngle(180 - 1e-3);
}

var azimuthalEquidistantRaw = azimuthalRaw(function(c) {
  return (c = acos$1(c)) && c / sin$1(c);
});

azimuthalEquidistantRaw.invert = azimuthalInvert(function(z) {
  return z;
});

function azimuthalEquidistant() {
  return projection(azimuthalEquidistantRaw)
      .scale(79.4188)
      .clipAngle(180 - 1e-3);
}

function mercatorRaw(lambda, phi) {
  return [lambda, log$1(tan((halfPi$1 + phi) / 2))];
}

mercatorRaw.invert = function(x, y) {
  return [x, 2 * atan(exp(y)) - halfPi$1];
};

function mercator() {
  return mercatorProjection(mercatorRaw)
      .scale(961 / tau$1);
}

function mercatorProjection(project) {
  var m = projection(project),
      center = m.center,
      scale = m.scale,
      translate = m.translate,
      clipExtent = m.clipExtent,
      x0 = null, y0, x1, y1; // clip extent

  m.scale = function(_) {
    return arguments.length ? (scale(_), reclip()) : scale();
  };

  m.translate = function(_) {
    return arguments.length ? (translate(_), reclip()) : translate();
  };

  m.center = function(_) {
    return arguments.length ? (center(_), reclip()) : center();
  };

  m.clipExtent = function(_) {
    return arguments.length ? ((_ == null ? x0 = y0 = x1 = y1 = null : (x0 = +_[0][0], y0 = +_[0][1], x1 = +_[1][0], y1 = +_[1][1])), reclip()) : x0 == null ? null : [[x0, y0], [x1, y1]];
  };

  function reclip() {
    var k = pi$1 * scale(),
        t = m(rotation(m.rotate()).invert([0, 0]));
    return clipExtent(x0 == null
        ? [[t[0] - k, t[1] - k], [t[0] + k, t[1] + k]] : project === mercatorRaw
        ? [[Math.max(t[0] - k, x0), y0], [Math.min(t[0] + k, x1), y1]]
        : [[x0, Math.max(t[1] - k, y0)], [x1, Math.min(t[1] + k, y1)]]);
  }

  return reclip();
}

function tany(y) {
  return tan((halfPi$1 + y) / 2);
}

function conicConformalRaw(y0, y1) {
  var cy0 = cos$1(y0),
      n = y0 === y1 ? sin$1(y0) : log$1(cy0 / cos$1(y1)) / log$1(tany(y1) / tany(y0)),
      f = cy0 * pow$1(tany(y0), n) / n;

  if (!n) return mercatorRaw;

  function project(x, y) {
    if (f > 0) { if (y < -halfPi$1 + epsilon$1) y = -halfPi$1 + epsilon$1; }
    else { if (y > halfPi$1 - epsilon$1) y = halfPi$1 - epsilon$1; }
    var r = f / pow$1(tany(y), n);
    return [r * sin$1(n * x), f - r * cos$1(n * x)];
  }

  project.invert = function(x, y) {
    var fy = f - y, r = sign$1(n) * sqrt$2(x * x + fy * fy),
      l = atan2$1(x, abs$1(fy)) * sign$1(fy);
    if (fy * n < 0)
      l -= pi$1 * sign$1(x) * sign$1(fy);
    return [l / n, 2 * atan(pow$1(f / r, 1 / n)) - halfPi$1];
  };

  return project;
}

function conicConformal() {
  return conicProjection(conicConformalRaw)
      .scale(109.5)
      .parallels([30, 30]);
}

function equirectangularRaw(lambda, phi) {
  return [lambda, phi];
}

equirectangularRaw.invert = equirectangularRaw;

function equirectangular() {
  return projection(equirectangularRaw)
      .scale(152.63);
}

function conicEquidistantRaw(y0, y1) {
  var cy0 = cos$1(y0),
      n = y0 === y1 ? sin$1(y0) : (cy0 - cos$1(y1)) / (y1 - y0),
      g = cy0 / n + y0;

  if (abs$1(n) < epsilon$1) return equirectangularRaw;

  function project(x, y) {
    var gy = g - y, nx = n * x;
    return [gy * sin$1(nx), g - gy * cos$1(nx)];
  }

  project.invert = function(x, y) {
    var gy = g - y,
        l = atan2$1(x, abs$1(gy)) * sign$1(gy);
    if (gy * n < 0)
      l -= pi$1 * sign$1(x) * sign$1(gy);
    return [l / n, g - sign$1(n) * sqrt$2(x * x + gy * gy)];
  };

  return project;
}

function conicEquidistant() {
  return conicProjection(conicEquidistantRaw)
      .scale(131.154)
      .center([0, 13.9389]);
}

var A1 = 1.340264,
    A2 = -0.081106,
    A3 = 0.000893,
    A4 = 0.003796,
    M = sqrt$2(3) / 2,
    iterations = 12;

function equalEarthRaw(lambda, phi) {
  var l = asin$1(M * sin$1(phi)), l2 = l * l, l6 = l2 * l2 * l2;
  return [
    lambda * cos$1(l) / (M * (A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2))),
    l * (A1 + A2 * l2 + l6 * (A3 + A4 * l2))
  ];
}

equalEarthRaw.invert = function(x, y) {
  var l = y, l2 = l * l, l6 = l2 * l2 * l2;
  for (var i = 0, delta, fy, fpy; i < iterations; ++i) {
    fy = l * (A1 + A2 * l2 + l6 * (A3 + A4 * l2)) - y;
    fpy = A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2);
    l -= delta = fy / fpy, l2 = l * l, l6 = l2 * l2 * l2;
    if (abs$1(delta) < epsilon2) break;
  }
  return [
    M * x * (A1 + 3 * A2 * l2 + l6 * (7 * A3 + 9 * A4 * l2)) / cos$1(l),
    asin$1(sin$1(l) / M)
  ];
};

function equalEarth() {
  return projection(equalEarthRaw)
      .scale(177.158);
}

function gnomonicRaw(x, y) {
  var cy = cos$1(y), k = cos$1(x) * cy;
  return [cy * sin$1(x) / k, sin$1(y) / k];
}

gnomonicRaw.invert = azimuthalInvert(atan);

function gnomonic() {
  return projection(gnomonicRaw)
      .scale(144.049)
      .clipAngle(60);
}

function identity$4() {
  var k = 1, tx = 0, ty = 0, sx = 1, sy = 1, // scale, translate and reflect
      alpha = 0, ca, sa, // angle
      x0 = null, y0, x1, y1, // clip extent
      kx = 1, ky = 1,
      transform = transformer$3({
        point: function(x, y) {
          var p = projection([x, y]);
          this.stream.point(p[0], p[1]);
        }
      }),
      postclip = identity$5,
      cache,
      cacheStream;

  function reset() {
    kx = k * sx;
    ky = k * sy;
    cache = cacheStream = null;
    return projection;
  }

  function projection (p) {
    var x = p[0] * kx, y = p[1] * ky;
    if (alpha) {
      var t = y * ca - x * sa;
      x = x * ca + y * sa;
      y = t;
    }    
    return [x + tx, y + ty];
  }
  projection.invert = function(p) {
    var x = p[0] - tx, y = p[1] - ty;
    if (alpha) {
      var t = y * ca + x * sa;
      x = x * ca - y * sa;
      y = t;
    }
    return [x / kx, y / ky];
  };
  projection.stream = function(stream) {
    return cache && cacheStream === stream ? cache : cache = transform(postclip(cacheStream = stream));
  };
  projection.postclip = function(_) {
    return arguments.length ? (postclip = _, x0 = y0 = x1 = y1 = null, reset()) : postclip;
  };
  projection.clipExtent = function(_) {
    return arguments.length ? (postclip = _ == null ? (x0 = y0 = x1 = y1 = null, identity$5) : clipRectangle(x0 = +_[0][0], y0 = +_[0][1], x1 = +_[1][0], y1 = +_[1][1]), reset()) : x0 == null ? null : [[x0, y0], [x1, y1]];
  };
  projection.scale = function(_) {
    return arguments.length ? (k = +_, reset()) : k;
  };
  projection.translate = function(_) {
    return arguments.length ? (tx = +_[0], ty = +_[1], reset()) : [tx, ty];
  };
  projection.angle = function(_) {
    return arguments.length ? (alpha = _ % 360 * radians, sa = sin$1(alpha), ca = cos$1(alpha), reset()) : alpha * degrees;
  };
  projection.reflectX = function(_) {
    return arguments.length ? (sx = _ ? -1 : 1, reset()) : sx < 0;
  };
  projection.reflectY = function(_) {
    return arguments.length ? (sy = _ ? -1 : 1, reset()) : sy < 0;
  };
  projection.fitExtent = function(extent, object) {
    return fitExtent(projection, extent, object);
  };
  projection.fitSize = function(size, object) {
    return fitSize(projection, size, object);
  };
  projection.fitWidth = function(width, object) {
    return fitWidth(projection, width, object);
  };
  projection.fitHeight = function(height, object) {
    return fitHeight(projection, height, object);
  };

  return projection;
}

function naturalEarth1Raw(lambda, phi) {
  var phi2 = phi * phi, phi4 = phi2 * phi2;
  return [
    lambda * (0.8707 - 0.131979 * phi2 + phi4 * (-0.013791 + phi4 * (0.003971 * phi2 - 0.001529 * phi4))),
    phi * (1.007226 + phi2 * (0.015085 + phi4 * (-0.044475 + 0.028874 * phi2 - 0.005916 * phi4)))
  ];
}

naturalEarth1Raw.invert = function(x, y) {
  var phi = y, i = 25, delta;
  do {
    var phi2 = phi * phi, phi4 = phi2 * phi2;
    phi -= delta = (phi * (1.007226 + phi2 * (0.015085 + phi4 * (-0.044475 + 0.028874 * phi2 - 0.005916 * phi4))) - y) /
        (1.007226 + phi2 * (0.015085 * 3 + phi4 * (-0.044475 * 7 + 0.028874 * 9 * phi2 - 0.005916 * 11 * phi4)));
  } while (abs$1(delta) > epsilon$1 && --i > 0);
  return [
    x / (0.8707 + (phi2 = phi * phi) * (-0.131979 + phi2 * (-0.013791 + phi2 * phi2 * phi2 * (0.003971 - 0.001529 * phi2)))),
    phi
  ];
};

function naturalEarth1() {
  return projection(naturalEarth1Raw)
      .scale(175.295);
}

function orthographicRaw(x, y) {
  return [cos$1(y) * sin$1(x), sin$1(y)];
}

orthographicRaw.invert = azimuthalInvert(asin$1);

function orthographic() {
  return projection(orthographicRaw)
      .scale(249.5)
      .clipAngle(90 + epsilon$1);
}

function stereographicRaw(x, y) {
  var cy = cos$1(y), k = 1 + cos$1(x) * cy;
  return [cy * sin$1(x) / k, sin$1(y) / k];
}

stereographicRaw.invert = azimuthalInvert(function(z) {
  return 2 * atan(z);
});

function stereographic() {
  return projection(stereographicRaw)
      .scale(250)
      .clipAngle(142);
}

function transverseMercatorRaw(lambda, phi) {
  return [log$1(tan((halfPi$1 + phi) / 2)), -lambda];
}

transverseMercatorRaw.invert = function(x, y) {
  return [-y, 2 * atan(exp(x)) - halfPi$1];
};

function transverseMercator() {
  var m = mercatorProjection(transverseMercatorRaw),
      center = m.center,
      rotate = m.rotate;

  m.center = function(_) {
    return arguments.length ? center([-_[1], _[0]]) : (_ = center(), [_[1], -_[0]]);
  };

  m.rotate = function(_) {
    return arguments.length ? rotate([_[0], _[1], _.length > 2 ? _[2] + 90 : 90]) : (_ = rotate(), [_[0], _[1], _[2] - 90]);
  };

  return rotate([0, 0, 90])
      .scale(159.155);
}

function defaultSeparation$1(a, b) {
  return a.parent === b.parent ? 1 : 2;
}

function meanX(children) {
  return children.reduce(meanXReduce, 0) / children.length;
}

function meanXReduce(x, c) {
  return x + c.x;
}

function maxY(children) {
  return 1 + children.reduce(maxYReduce, 0);
}

function maxYReduce(y, c) {
  return Math.max(y, c.y);
}

function leafLeft(node) {
  var children;
  while (children = node.children) node = children[0];
  return node;
}

function leafRight(node) {
  var children;
  while (children = node.children) node = children[children.length - 1];
  return node;
}

function cluster() {
  var separation = defaultSeparation$1,
      dx = 1,
      dy = 1,
      nodeSize = false;

  function cluster(root) {
    var previousNode,
        x = 0;

    // First walk, computing the initial x & y values.
    root.eachAfter(function(node) {
      var children = node.children;
      if (children) {
        node.x = meanX(children);
        node.y = maxY(children);
      } else {
        node.x = previousNode ? x += separation(node, previousNode) : 0;
        node.y = 0;
        previousNode = node;
      }
    });

    var left = leafLeft(root),
        right = leafRight(root),
        x0 = left.x - separation(left, right) / 2,
        x1 = right.x + separation(right, left) / 2;

    // Second walk, normalizing x & y to the desired size.
    return root.eachAfter(nodeSize ? function(node) {
      node.x = (node.x - root.x) * dx;
      node.y = (root.y - node.y) * dy;
    } : function(node) {
      node.x = (node.x - x0) / (x1 - x0) * dx;
      node.y = (1 - (root.y ? node.y / root.y : 1)) * dy;
    });
  }

  cluster.separation = function(x) {
    return arguments.length ? (separation = x, cluster) : separation;
  };

  cluster.size = function(x) {
    return arguments.length ? (nodeSize = false, dx = +x[0], dy = +x[1], cluster) : (nodeSize ? null : [dx, dy]);
  };

  cluster.nodeSize = function(x) {
    return arguments.length ? (nodeSize = true, dx = +x[0], dy = +x[1], cluster) : (nodeSize ? [dx, dy] : null);
  };

  return cluster;
}

function count(node) {
  var sum = 0,
      children = node.children,
      i = children && children.length;
  if (!i) sum = 1;
  else while (--i >= 0) sum += children[i].value;
  node.value = sum;
}

function node_count() {
  return this.eachAfter(count);
}

function node_each(callback, that) {
  let index = -1;
  for (const node of this) {
    callback.call(that, node, ++index, this);
  }
  return this;
}

function node_eachBefore(callback, that) {
  var node = this, nodes = [node], children, i, index = -1;
  while (node = nodes.pop()) {
    callback.call(that, node, ++index, this);
    if (children = node.children) {
      for (i = children.length - 1; i >= 0; --i) {
        nodes.push(children[i]);
      }
    }
  }
  return this;
}

function node_eachAfter(callback, that) {
  var node = this, nodes = [node], next = [], children, i, n, index = -1;
  while (node = nodes.pop()) {
    next.push(node);
    if (children = node.children) {
      for (i = 0, n = children.length; i < n; ++i) {
        nodes.push(children[i]);
      }
    }
  }
  while (node = next.pop()) {
    callback.call(that, node, ++index, this);
  }
  return this;
}

function node_find(callback, that) {
  let index = -1;
  for (const node of this) {
    if (callback.call(that, node, ++index, this)) {
      return node;
    }
  }
}

function node_sum(value) {
  return this.eachAfter(function(node) {
    var sum = +value(node.data) || 0,
        children = node.children,
        i = children && children.length;
    while (--i >= 0) sum += children[i].value;
    node.value = sum;
  });
}

function node_sort(compare) {
  return this.eachBefore(function(node) {
    if (node.children) {
      node.children.sort(compare);
    }
  });
}

function node_path(end) {
  var start = this,
      ancestor = leastCommonAncestor(start, end),
      nodes = [start];
  while (start !== ancestor) {
    start = start.parent;
    nodes.push(start);
  }
  var k = nodes.length;
  while (end !== ancestor) {
    nodes.splice(k, 0, end);
    end = end.parent;
  }
  return nodes;
}

function leastCommonAncestor(a, b) {
  if (a === b) return a;
  var aNodes = a.ancestors(),
      bNodes = b.ancestors(),
      c = null;
  a = aNodes.pop();
  b = bNodes.pop();
  while (a === b) {
    c = a;
    a = aNodes.pop();
    b = bNodes.pop();
  }
  return c;
}

function node_ancestors() {
  var node = this, nodes = [node];
  while (node = node.parent) {
    nodes.push(node);
  }
  return nodes;
}

function node_descendants() {
  return Array.from(this);
}

function node_leaves() {
  var leaves = [];
  this.eachBefore(function(node) {
    if (!node.children) {
      leaves.push(node);
    }
  });
  return leaves;
}

function node_links() {
  var root = this, links = [];
  root.each(function(node) {
    if (node !== root) { // Don’t include the root’s parent, if any.
      links.push({source: node.parent, target: node});
    }
  });
  return links;
}

function* node_iterator() {
  var node = this, current, next = [node], children, i, n;
  do {
    current = next.reverse(), next = [];
    while (node = current.pop()) {
      yield node;
      if (children = node.children) {
        for (i = 0, n = children.length; i < n; ++i) {
          next.push(children[i]);
        }
      }
    }
  } while (next.length);
}

function hierarchy(data, children) {
  if (data instanceof Map) {
    data = [undefined, data];
    if (children === undefined) children = mapChildren;
  } else if (children === undefined) {
    children = objectChildren;
  }

  var root = new Node$1(data),
      node,
      nodes = [root],
      child,
      childs,
      i,
      n;

  while (node = nodes.pop()) {
    if ((childs = children(node.data)) && (n = (childs = Array.from(childs)).length)) {
      node.children = childs;
      for (i = n - 1; i >= 0; --i) {
        nodes.push(child = childs[i] = new Node$1(childs[i]));
        child.parent = node;
        child.depth = node.depth + 1;
      }
    }
  }

  return root.eachBefore(computeHeight);
}

function node_copy() {
  return hierarchy(this).eachBefore(copyData);
}

function objectChildren(d) {
  return d.children;
}

function mapChildren(d) {
  return Array.isArray(d) ? d[1] : null;
}

function copyData(node) {
  if (node.data.value !== undefined) node.value = node.data.value;
  node.data = node.data.data;
}

function computeHeight(node) {
  var height = 0;
  do node.height = height;
  while ((node = node.parent) && (node.height < ++height));
}

function Node$1(data) {
  this.data = data;
  this.depth =
  this.height = 0;
  this.parent = null;
}

Node$1.prototype = hierarchy.prototype = {
  constructor: Node$1,
  count: node_count,
  each: node_each,
  eachAfter: node_eachAfter,
  eachBefore: node_eachBefore,
  find: node_find,
  sum: node_sum,
  sort: node_sort,
  path: node_path,
  ancestors: node_ancestors,
  descendants: node_descendants,
  leaves: node_leaves,
  links: node_links,
  copy: node_copy,
  [Symbol.iterator]: node_iterator
};

function optional(f) {
  return f == null ? null : required(f);
}

function required(f) {
  if (typeof f !== "function") throw new Error;
  return f;
}

function constantZero() {
  return 0;
}

function constant$2(x) {
  return function() {
    return x;
  };
}

// https://en.wikipedia.org/wiki/Linear_congruential_generator#Parameters_in_common_use
const a$1 = 1664525;
const c$3 = 1013904223;
const m = 4294967296; // 2^32

function lcg$1() {
  let s = 1;
  return () => (s = (a$1 * s + c$3) % m) / m;
}

function array$1(x) {
  return typeof x === "object" && "length" in x
    ? x // Array, TypedArray, NodeList, array-like
    : Array.from(x); // Map, Set, iterable, string, or anything else
}

function shuffle(array, random) {
  let m = array.length,
      t,
      i;

  while (m) {
    i = random() * m-- | 0;
    t = array[m];
    array[m] = array[i];
    array[i] = t;
  }

  return array;
}

function enclose(circles) {
  return packEncloseRandom(circles, lcg$1());
}

function packEncloseRandom(circles, random) {
  var i = 0, n = (circles = shuffle(Array.from(circles), random)).length, B = [], p, e;

  while (i < n) {
    p = circles[i];
    if (e && enclosesWeak(e, p)) ++i;
    else e = encloseBasis(B = extendBasis(B, p)), i = 0;
  }

  return e;
}

function extendBasis(B, p) {
  var i, j;

  if (enclosesWeakAll(p, B)) return [p];

  // If we get here then B must have at least one element.
  for (i = 0; i < B.length; ++i) {
    if (enclosesNot(p, B[i])
        && enclosesWeakAll(encloseBasis2(B[i], p), B)) {
      return [B[i], p];
    }
  }

  // If we get here then B must have at least two elements.
  for (i = 0; i < B.length - 1; ++i) {
    for (j = i + 1; j < B.length; ++j) {
      if (enclosesNot(encloseBasis2(B[i], B[j]), p)
          && enclosesNot(encloseBasis2(B[i], p), B[j])
          && enclosesNot(encloseBasis2(B[j], p), B[i])
          && enclosesWeakAll(encloseBasis3(B[i], B[j], p), B)) {
        return [B[i], B[j], p];
      }
    }
  }

  // If we get here then something is very wrong.
  throw new Error;
}

function enclosesNot(a, b) {
  var dr = a.r - b.r, dx = b.x - a.x, dy = b.y - a.y;
  return dr < 0 || dr * dr < dx * dx + dy * dy;
}

function enclosesWeak(a, b) {
  var dr = a.r - b.r + Math.max(a.r, b.r, 1) * 1e-9, dx = b.x - a.x, dy = b.y - a.y;
  return dr > 0 && dr * dr > dx * dx + dy * dy;
}

function enclosesWeakAll(a, B) {
  for (var i = 0; i < B.length; ++i) {
    if (!enclosesWeak(a, B[i])) {
      return false;
    }
  }
  return true;
}

function encloseBasis(B) {
  switch (B.length) {
    case 1: return encloseBasis1(B[0]);
    case 2: return encloseBasis2(B[0], B[1]);
    case 3: return encloseBasis3(B[0], B[1], B[2]);
  }
}

function encloseBasis1(a) {
  return {
    x: a.x,
    y: a.y,
    r: a.r
  };
}

function encloseBasis2(a, b) {
  var x1 = a.x, y1 = a.y, r1 = a.r,
      x2 = b.x, y2 = b.y, r2 = b.r,
      x21 = x2 - x1, y21 = y2 - y1, r21 = r2 - r1,
      l = Math.sqrt(x21 * x21 + y21 * y21);
  return {
    x: (x1 + x2 + x21 / l * r21) / 2,
    y: (y1 + y2 + y21 / l * r21) / 2,
    r: (l + r1 + r2) / 2
  };
}

function encloseBasis3(a, b, c) {
  var x1 = a.x, y1 = a.y, r1 = a.r,
      x2 = b.x, y2 = b.y, r2 = b.r,
      x3 = c.x, y3 = c.y, r3 = c.r,
      a2 = x1 - x2,
      a3 = x1 - x3,
      b2 = y1 - y2,
      b3 = y1 - y3,
      c2 = r2 - r1,
      c3 = r3 - r1,
      d1 = x1 * x1 + y1 * y1 - r1 * r1,
      d2 = d1 - x2 * x2 - y2 * y2 + r2 * r2,
      d3 = d1 - x3 * x3 - y3 * y3 + r3 * r3,
      ab = a3 * b2 - a2 * b3,
      xa = (b2 * d3 - b3 * d2) / (ab * 2) - x1,
      xb = (b3 * c2 - b2 * c3) / ab,
      ya = (a3 * d2 - a2 * d3) / (ab * 2) - y1,
      yb = (a2 * c3 - a3 * c2) / ab,
      A = xb * xb + yb * yb - 1,
      B = 2 * (r1 + xa * xb + ya * yb),
      C = xa * xa + ya * ya - r1 * r1,
      r = -(Math.abs(A) > 1e-6 ? (B + Math.sqrt(B * B - 4 * A * C)) / (2 * A) : C / B);
  return {
    x: x1 + xa + xb * r,
    y: y1 + ya + yb * r,
    r: r
  };
}

function place(b, a, c) {
  var dx = b.x - a.x, x, a2,
      dy = b.y - a.y, y, b2,
      d2 = dx * dx + dy * dy;
  if (d2) {
    a2 = a.r + c.r, a2 *= a2;
    b2 = b.r + c.r, b2 *= b2;
    if (a2 > b2) {
      x = (d2 + b2 - a2) / (2 * d2);
      y = Math.sqrt(Math.max(0, b2 / d2 - x * x));
      c.x = b.x - x * dx - y * dy;
      c.y = b.y - x * dy + y * dx;
    } else {
      x = (d2 + a2 - b2) / (2 * d2);
      y = Math.sqrt(Math.max(0, a2 / d2 - x * x));
      c.x = a.x + x * dx - y * dy;
      c.y = a.y + x * dy + y * dx;
    }
  } else {
    c.x = a.x + c.r;
    c.y = a.y;
  }
}

function intersects(a, b) {
  var dr = a.r + b.r - 1e-6, dx = b.x - a.x, dy = b.y - a.y;
  return dr > 0 && dr * dr > dx * dx + dy * dy;
}

function score(node) {
  var a = node._,
      b = node.next._,
      ab = a.r + b.r,
      dx = (a.x * b.r + b.x * a.r) / ab,
      dy = (a.y * b.r + b.y * a.r) / ab;
  return dx * dx + dy * dy;
}

function Node(circle) {
  this._ = circle;
  this.next = null;
  this.previous = null;
}

function packSiblingsRandom(circles, random) {
  if (!(n = (circles = array$1(circles)).length)) return 0;

  var a, b, c, n, aa, ca, i, j, k, sj, sk;

  // Place the first circle.
  a = circles[0], a.x = 0, a.y = 0;
  if (!(n > 1)) return a.r;

  // Place the second circle.
  b = circles[1], a.x = -b.r, b.x = a.r, b.y = 0;
  if (!(n > 2)) return a.r + b.r;

  // Place the third circle.
  place(b, a, c = circles[2]);

  // Initialize the front-chain using the first three circles a, b and c.
  a = new Node(a), b = new Node(b), c = new Node(c);
  a.next = c.previous = b;
  b.next = a.previous = c;
  c.next = b.previous = a;

  // Attempt to place each remaining circle…
  pack: for (i = 3; i < n; ++i) {
    place(a._, b._, c = circles[i]), c = new Node(c);

    // Find the closest intersecting circle on the front-chain, if any.
    // “Closeness” is determined by linear distance along the front-chain.
    // “Ahead” or “behind” is likewise determined by linear distance.
    j = b.next, k = a.previous, sj = b._.r, sk = a._.r;
    do {
      if (sj <= sk) {
        if (intersects(j._, c._)) {
          b = j, a.next = b, b.previous = a, --i;
          continue pack;
        }
        sj += j._.r, j = j.next;
      } else {
        if (intersects(k._, c._)) {
          a = k, a.next = b, b.previous = a, --i;
          continue pack;
        }
        sk += k._.r, k = k.previous;
      }
    } while (j !== k.next);

    // Success! Insert the new circle c between a and b.
    c.previous = a, c.next = b, a.next = b.previous = b = c;

    // Compute the new closest circle pair to the centroid.
    aa = score(a);
    while ((c = c.next) !== b) {
      if ((ca = score(c)) < aa) {
        a = c, aa = ca;
      }
    }
    b = a.next;
  }

  // Compute the enclosing circle of the front chain.
  a = [b._], c = b; while ((c = c.next) !== b) a.push(c._); c = packEncloseRandom(a, random);

  // Translate the circles to put the enclosing circle around the origin.
  for (i = 0; i < n; ++i) a = circles[i], a.x -= c.x, a.y -= c.y;

  return c.r;
}

function siblings(circles) {
  packSiblingsRandom(circles, lcg$1());
  return circles;
}

function defaultRadius(d) {
  return Math.sqrt(d.value);
}

function index$1() {
  var radius = null,
      dx = 1,
      dy = 1,
      padding = constantZero;

  function pack(root) {
    const random = lcg$1();
    root.x = dx / 2, root.y = dy / 2;
    if (radius) {
      root.eachBefore(radiusLeaf(radius))
          .eachAfter(packChildrenRandom(padding, 0.5, random))
          .eachBefore(translateChild(1));
    } else {
      root.eachBefore(radiusLeaf(defaultRadius))
          .eachAfter(packChildrenRandom(constantZero, 1, random))
          .eachAfter(packChildrenRandom(padding, root.r / Math.min(dx, dy), random))
          .eachBefore(translateChild(Math.min(dx, dy) / (2 * root.r)));
    }
    return root;
  }

  pack.radius = function(x) {
    return arguments.length ? (radius = optional(x), pack) : radius;
  };

  pack.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], pack) : [dx, dy];
  };

  pack.padding = function(x) {
    return arguments.length ? (padding = typeof x === "function" ? x : constant$2(+x), pack) : padding;
  };

  return pack;
}

function radiusLeaf(radius) {
  return function(node) {
    if (!node.children) {
      node.r = Math.max(0, +radius(node) || 0);
    }
  };
}

function packChildrenRandom(padding, k, random) {
  return function(node) {
    if (children = node.children) {
      var children,
          i,
          n = children.length,
          r = padding(node) * k || 0,
          e;

      if (r) for (i = 0; i < n; ++i) children[i].r += r;
      e = packSiblingsRandom(children, random);
      if (r) for (i = 0; i < n; ++i) children[i].r -= r;
      node.r = e + r;
    }
  };
}

function translateChild(k) {
  return function(node) {
    var parent = node.parent;
    node.r *= k;
    if (parent) {
      node.x = parent.x + k * node.x;
      node.y = parent.y + k * node.y;
    }
  };
}

function roundNode(node) {
  node.x0 = Math.round(node.x0);
  node.y0 = Math.round(node.y0);
  node.x1 = Math.round(node.x1);
  node.y1 = Math.round(node.y1);
}

function treemapDice(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      node,
      i = -1,
      n = nodes.length,
      k = parent.value && (x1 - x0) / parent.value;

  while (++i < n) {
    node = nodes[i], node.y0 = y0, node.y1 = y1;
    node.x0 = x0, node.x1 = x0 += node.value * k;
  }
}

function partition() {
  var dx = 1,
      dy = 1,
      padding = 0,
      round = false;

  function partition(root) {
    var n = root.height + 1;
    root.x0 =
    root.y0 = padding;
    root.x1 = dx;
    root.y1 = dy / n;
    root.eachBefore(positionNode(dy, n));
    if (round) root.eachBefore(roundNode);
    return root;
  }

  function positionNode(dy, n) {
    return function(node) {
      if (node.children) {
        treemapDice(node, node.x0, dy * (node.depth + 1) / n, node.x1, dy * (node.depth + 2) / n);
      }
      var x0 = node.x0,
          y0 = node.y0,
          x1 = node.x1 - padding,
          y1 = node.y1 - padding;
      if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
      if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
      node.x0 = x0;
      node.y0 = y0;
      node.x1 = x1;
      node.y1 = y1;
    };
  }

  partition.round = function(x) {
    return arguments.length ? (round = !!x, partition) : round;
  };

  partition.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], partition) : [dx, dy];
  };

  partition.padding = function(x) {
    return arguments.length ? (padding = +x, partition) : padding;
  };

  return partition;
}

var preroot = {depth: -1},
    ambiguous = {},
    imputed = {};

function defaultId(d) {
  return d.id;
}

function defaultParentId(d) {
  return d.parentId;
}

function stratify() {
  var id = defaultId,
      parentId = defaultParentId,
      path;

  function stratify(data) {
    var nodes = Array.from(data),
        currentId = id,
        currentParentId = parentId,
        n,
        d,
        i,
        root,
        parent,
        node,
        nodeId,
        nodeKey,
        nodeByKey = new Map;

    if (path != null) {
      const I = nodes.map((d, i) => normalize$1(path(d, i, data)));
      const P = I.map(parentof);
      const S = new Set(I).add("");
      for (const i of P) {
        if (!S.has(i)) {
          S.add(i);
          I.push(i);
          P.push(parentof(i));
          nodes.push(imputed);
        }
      }
      currentId = (_, i) => I[i];
      currentParentId = (_, i) => P[i];
    }

    for (i = 0, n = nodes.length; i < n; ++i) {
      d = nodes[i], node = nodes[i] = new Node$1(d);
      if ((nodeId = currentId(d, i, data)) != null && (nodeId += "")) {
        nodeKey = node.id = nodeId;
        nodeByKey.set(nodeKey, nodeByKey.has(nodeKey) ? ambiguous : node);
      }
      if ((nodeId = currentParentId(d, i, data)) != null && (nodeId += "")) {
        node.parent = nodeId;
      }
    }

    for (i = 0; i < n; ++i) {
      node = nodes[i];
      if (nodeId = node.parent) {
        parent = nodeByKey.get(nodeId);
        if (!parent) throw new Error("missing: " + nodeId);
        if (parent === ambiguous) throw new Error("ambiguous: " + nodeId);
        if (parent.children) parent.children.push(node);
        else parent.children = [node];
        node.parent = parent;
      } else {
        if (root) throw new Error("multiple roots");
        root = node;
      }
    }

    if (!root) throw new Error("no root");

    // When imputing internal nodes, only introduce roots if needed.
    // Then replace the imputed marker data with null.
    if (path != null) {
      while (root.data === imputed && root.children.length === 1) {
        root = root.children[0], --n;
      }
      for (let i = nodes.length - 1; i >= 0; --i) {
        node = nodes[i];
        if (node.data !== imputed) break;
        node.data = null;
      }
    }

    root.parent = preroot;
    root.eachBefore(function(node) { node.depth = node.parent.depth + 1; --n; }).eachBefore(computeHeight);
    root.parent = null;
    if (n > 0) throw new Error("cycle");

    return root;
  }

  stratify.id = function(x) {
    return arguments.length ? (id = optional(x), stratify) : id;
  };

  stratify.parentId = function(x) {
    return arguments.length ? (parentId = optional(x), stratify) : parentId;
  };

  stratify.path = function(x) {
    return arguments.length ? (path = optional(x), stratify) : path;
  };

  return stratify;
}

// To normalize a path, we coerce to a string, strip the trailing slash if any
// (as long as the trailing slash is not immediately preceded by another slash),
// and add leading slash if missing.
function normalize$1(path) {
  path = `${path}`;
  let i = path.length;
  if (slash(path, i - 1) && !slash(path, i - 2)) path = path.slice(0, -1);
  return path[0] === "/" ? path : `/${path}`;
}

// Walk backwards to find the first slash that is not the leading slash, e.g.:
// "/foo/bar" ⇥ "/foo", "/foo" ⇥ "/", "/" ↦ "". (The root is special-cased
// because the id of the root must be a truthy value.)
function parentof(path) {
  let i = path.length;
  if (i < 2) return "";
  while (--i > 1) if (slash(path, i)) break;
  return path.slice(0, i);
}

// Slashes can be escaped; to determine whether a slash is a path delimiter, we
// count the number of preceding backslashes escaping the forward slash: an odd
// number indicates an escaped forward slash.
function slash(path, i) {
  if (path[i] === "/") {
    let k = 0;
    while (i > 0 && path[--i] === "\\") ++k;
    if ((k & 1) === 0) return true;
  }
  return false;
}

function defaultSeparation(a, b) {
  return a.parent === b.parent ? 1 : 2;
}

// function radialSeparation(a, b) {
//   return (a.parent === b.parent ? 1 : 2) / a.depth;
// }

// This function is used to traverse the left contour of a subtree (or
// subforest). It returns the successor of v on this contour. This successor is
// either given by the leftmost child of v or by the thread of v. The function
// returns null if and only if v is on the highest level of its subtree.
function nextLeft(v) {
  var children = v.children;
  return children ? children[0] : v.t;
}

// This function works analogously to nextLeft.
function nextRight(v) {
  var children = v.children;
  return children ? children[children.length - 1] : v.t;
}

// Shifts the current subtree rooted at w+. This is done by increasing
// prelim(w+) and mod(w+) by shift.
function moveSubtree(wm, wp, shift) {
  var change = shift / (wp.i - wm.i);
  wp.c -= change;
  wp.s += shift;
  wm.c += change;
  wp.z += shift;
  wp.m += shift;
}

// All other shifts, applied to the smaller subtrees between w- and w+, are
// performed by this function. To prepare the shifts, we have to adjust
// change(w+), shift(w+), and change(w-).
function executeShifts(v) {
  var shift = 0,
      change = 0,
      children = v.children,
      i = children.length,
      w;
  while (--i >= 0) {
    w = children[i];
    w.z += shift;
    w.m += shift;
    shift += w.s + (change += w.c);
  }
}

// If vi-’s ancestor is a sibling of v, returns vi-’s ancestor. Otherwise,
// returns the specified (default) ancestor.
function nextAncestor(vim, v, ancestor) {
  return vim.a.parent === v.parent ? vim.a : ancestor;
}

function TreeNode(node, i) {
  this._ = node;
  this.parent = null;
  this.children = null;
  this.A = null; // default ancestor
  this.a = this; // ancestor
  this.z = 0; // prelim
  this.m = 0; // mod
  this.c = 0; // change
  this.s = 0; // shift
  this.t = null; // thread
  this.i = i; // number
}

TreeNode.prototype = Object.create(Node$1.prototype);

function treeRoot(root) {
  var tree = new TreeNode(root, 0),
      node,
      nodes = [tree],
      child,
      children,
      i,
      n;

  while (node = nodes.pop()) {
    if (children = node._.children) {
      node.children = new Array(n = children.length);
      for (i = n - 1; i >= 0; --i) {
        nodes.push(child = node.children[i] = new TreeNode(children[i], i));
        child.parent = node;
      }
    }
  }

  (tree.parent = new TreeNode(null, 0)).children = [tree];
  return tree;
}

// Node-link tree diagram using the Reingold-Tilford "tidy" algorithm
function tree() {
  var separation = defaultSeparation,
      dx = 1,
      dy = 1,
      nodeSize = null;

  function tree(root) {
    var t = treeRoot(root);

    // Compute the layout using Buchheim et al.’s algorithm.
    t.eachAfter(firstWalk), t.parent.m = -t.z;
    t.eachBefore(secondWalk);

    // If a fixed node size is specified, scale x and y.
    if (nodeSize) root.eachBefore(sizeNode);

    // If a fixed tree size is specified, scale x and y based on the extent.
    // Compute the left-most, right-most, and depth-most nodes for extents.
    else {
      var left = root,
          right = root,
          bottom = root;
      root.eachBefore(function(node) {
        if (node.x < left.x) left = node;
        if (node.x > right.x) right = node;
        if (node.depth > bottom.depth) bottom = node;
      });
      var s = left === right ? 1 : separation(left, right) / 2,
          tx = s - left.x,
          kx = dx / (right.x + s + tx),
          ky = dy / (bottom.depth || 1);
      root.eachBefore(function(node) {
        node.x = (node.x + tx) * kx;
        node.y = node.depth * ky;
      });
    }

    return root;
  }

  // Computes a preliminary x-coordinate for v. Before that, FIRST WALK is
  // applied recursively to the children of v, as well as the function
  // APPORTION. After spacing out the children by calling EXECUTE SHIFTS, the
  // node v is placed to the midpoint of its outermost children.
  function firstWalk(v) {
    var children = v.children,
        siblings = v.parent.children,
        w = v.i ? siblings[v.i - 1] : null;
    if (children) {
      executeShifts(v);
      var midpoint = (children[0].z + children[children.length - 1].z) / 2;
      if (w) {
        v.z = w.z + separation(v._, w._);
        v.m = v.z - midpoint;
      } else {
        v.z = midpoint;
      }
    } else if (w) {
      v.z = w.z + separation(v._, w._);
    }
    v.parent.A = apportion(v, w, v.parent.A || siblings[0]);
  }

  // Computes all real x-coordinates by summing up the modifiers recursively.
  function secondWalk(v) {
    v._.x = v.z + v.parent.m;
    v.m += v.parent.m;
  }

  // The core of the algorithm. Here, a new subtree is combined with the
  // previous subtrees. Threads are used to traverse the inside and outside
  // contours of the left and right subtree up to the highest common level. The
  // vertices used for the traversals are vi+, vi-, vo-, and vo+, where the
  // superscript o means outside and i means inside, the subscript - means left
  // subtree and + means right subtree. For summing up the modifiers along the
  // contour, we use respective variables si+, si-, so-, and so+. Whenever two
  // nodes of the inside contours conflict, we compute the left one of the
  // greatest uncommon ancestors using the function ANCESTOR and call MOVE
  // SUBTREE to shift the subtree and prepare the shifts of smaller subtrees.
  // Finally, we add a new thread (if necessary).
  function apportion(v, w, ancestor) {
    if (w) {
      var vip = v,
          vop = v,
          vim = w,
          vom = vip.parent.children[0],
          sip = vip.m,
          sop = vop.m,
          sim = vim.m,
          som = vom.m,
          shift;
      while (vim = nextRight(vim), vip = nextLeft(vip), vim && vip) {
        vom = nextLeft(vom);
        vop = nextRight(vop);
        vop.a = v;
        shift = vim.z + sim - vip.z - sip + separation(vim._, vip._);
        if (shift > 0) {
          moveSubtree(nextAncestor(vim, v, ancestor), v, shift);
          sip += shift;
          sop += shift;
        }
        sim += vim.m;
        sip += vip.m;
        som += vom.m;
        sop += vop.m;
      }
      if (vim && !nextRight(vop)) {
        vop.t = vim;
        vop.m += sim - sop;
      }
      if (vip && !nextLeft(vom)) {
        vom.t = vip;
        vom.m += sip - som;
        ancestor = v;
      }
    }
    return ancestor;
  }

  function sizeNode(node) {
    node.x *= dx;
    node.y = node.depth * dy;
  }

  tree.separation = function(x) {
    return arguments.length ? (separation = x, tree) : separation;
  };

  tree.size = function(x) {
    return arguments.length ? (nodeSize = false, dx = +x[0], dy = +x[1], tree) : (nodeSize ? null : [dx, dy]);
  };

  tree.nodeSize = function(x) {
    return arguments.length ? (nodeSize = true, dx = +x[0], dy = +x[1], tree) : (nodeSize ? [dx, dy] : null);
  };

  return tree;
}

function treemapSlice(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      node,
      i = -1,
      n = nodes.length,
      k = parent.value && (y1 - y0) / parent.value;

  while (++i < n) {
    node = nodes[i], node.x0 = x0, node.x1 = x1;
    node.y0 = y0, node.y1 = y0 += node.value * k;
  }
}

var phi = (1 + Math.sqrt(5)) / 2;

function squarifyRatio(ratio, parent, x0, y0, x1, y1) {
  var rows = [],
      nodes = parent.children,
      row,
      nodeValue,
      i0 = 0,
      i1 = 0,
      n = nodes.length,
      dx, dy,
      value = parent.value,
      sumValue,
      minValue,
      maxValue,
      newRatio,
      minRatio,
      alpha,
      beta;

  while (i0 < n) {
    dx = x1 - x0, dy = y1 - y0;

    // Find the next non-empty node.
    do sumValue = nodes[i1++].value; while (!sumValue && i1 < n);
    minValue = maxValue = sumValue;
    alpha = Math.max(dy / dx, dx / dy) / (value * ratio);
    beta = sumValue * sumValue * alpha;
    minRatio = Math.max(maxValue / beta, beta / minValue);

    // Keep adding nodes while the aspect ratio maintains or improves.
    for (; i1 < n; ++i1) {
      sumValue += nodeValue = nodes[i1].value;
      if (nodeValue < minValue) minValue = nodeValue;
      if (nodeValue > maxValue) maxValue = nodeValue;
      beta = sumValue * sumValue * alpha;
      newRatio = Math.max(maxValue / beta, beta / minValue);
      if (newRatio > minRatio) { sumValue -= nodeValue; break; }
      minRatio = newRatio;
    }

    // Position and record the row orientation.
    rows.push(row = {value: sumValue, dice: dx < dy, children: nodes.slice(i0, i1)});
    if (row.dice) treemapDice(row, x0, y0, x1, value ? y0 += dy * sumValue / value : y1);
    else treemapSlice(row, x0, y0, value ? x0 += dx * sumValue / value : x1, y1);
    value -= sumValue, i0 = i1;
  }

  return rows;
}

const squarify = (function custom(ratio) {

  function squarify(parent, x0, y0, x1, y1) {
    squarifyRatio(ratio, parent, x0, y0, x1, y1);
  }

  squarify.ratio = function(x) {
    return custom((x = +x) > 1 ? x : 1);
  };

  return squarify;
})(phi);

function index() {
  var tile = squarify,
      round = false,
      dx = 1,
      dy = 1,
      paddingStack = [0],
      paddingInner = constantZero,
      paddingTop = constantZero,
      paddingRight = constantZero,
      paddingBottom = constantZero,
      paddingLeft = constantZero;

  function treemap(root) {
    root.x0 =
    root.y0 = 0;
    root.x1 = dx;
    root.y1 = dy;
    root.eachBefore(positionNode);
    paddingStack = [0];
    if (round) root.eachBefore(roundNode);
    return root;
  }

  function positionNode(node) {
    var p = paddingStack[node.depth],
        x0 = node.x0 + p,
        y0 = node.y0 + p,
        x1 = node.x1 - p,
        y1 = node.y1 - p;
    if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
    if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
    node.x0 = x0;
    node.y0 = y0;
    node.x1 = x1;
    node.y1 = y1;
    if (node.children) {
      p = paddingStack[node.depth + 1] = paddingInner(node) / 2;
      x0 += paddingLeft(node) - p;
      y0 += paddingTop(node) - p;
      x1 -= paddingRight(node) - p;
      y1 -= paddingBottom(node) - p;
      if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
      if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
      tile(node, x0, y0, x1, y1);
    }
  }

  treemap.round = function(x) {
    return arguments.length ? (round = !!x, treemap) : round;
  };

  treemap.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], treemap) : [dx, dy];
  };

  treemap.tile = function(x) {
    return arguments.length ? (tile = required(x), treemap) : tile;
  };

  treemap.padding = function(x) {
    return arguments.length ? treemap.paddingInner(x).paddingOuter(x) : treemap.paddingInner();
  };

  treemap.paddingInner = function(x) {
    return arguments.length ? (paddingInner = typeof x === "function" ? x : constant$2(+x), treemap) : paddingInner;
  };

  treemap.paddingOuter = function(x) {
    return arguments.length ? treemap.paddingTop(x).paddingRight(x).paddingBottom(x).paddingLeft(x) : treemap.paddingTop();
  };

  treemap.paddingTop = function(x) {
    return arguments.length ? (paddingTop = typeof x === "function" ? x : constant$2(+x), treemap) : paddingTop;
  };

  treemap.paddingRight = function(x) {
    return arguments.length ? (paddingRight = typeof x === "function" ? x : constant$2(+x), treemap) : paddingRight;
  };

  treemap.paddingBottom = function(x) {
    return arguments.length ? (paddingBottom = typeof x === "function" ? x : constant$2(+x), treemap) : paddingBottom;
  };

  treemap.paddingLeft = function(x) {
    return arguments.length ? (paddingLeft = typeof x === "function" ? x : constant$2(+x), treemap) : paddingLeft;
  };

  return treemap;
}

function binary(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      i, n = nodes.length,
      sum, sums = new Array(n + 1);

  for (sums[0] = sum = i = 0; i < n; ++i) {
    sums[i + 1] = sum += nodes[i].value;
  }

  partition(0, n, parent.value, x0, y0, x1, y1);

  function partition(i, j, value, x0, y0, x1, y1) {
    if (i >= j - 1) {
      var node = nodes[i];
      node.x0 = x0, node.y0 = y0;
      node.x1 = x1, node.y1 = y1;
      return;
    }

    var valueOffset = sums[i],
        valueTarget = (value / 2) + valueOffset,
        k = i + 1,
        hi = j - 1;

    while (k < hi) {
      var mid = k + hi >>> 1;
      if (sums[mid] < valueTarget) k = mid + 1;
      else hi = mid;
    }

    if ((valueTarget - sums[k - 1]) < (sums[k] - valueTarget) && i + 1 < k) --k;

    var valueLeft = sums[k] - valueOffset,
        valueRight = value - valueLeft;

    if ((x1 - x0) > (y1 - y0)) {
      var xk = value ? (x0 * valueRight + x1 * valueLeft) / value : x1;
      partition(i, k, valueLeft, x0, y0, xk, y1);
      partition(k, j, valueRight, xk, y0, x1, y1);
    } else {
      var yk = value ? (y0 * valueRight + y1 * valueLeft) / value : y1;
      partition(i, k, valueLeft, x0, y0, x1, yk);
      partition(k, j, valueRight, x0, yk, x1, y1);
    }
  }
}

function sliceDice(parent, x0, y0, x1, y1) {
  (parent.depth & 1 ? treemapSlice : treemapDice)(parent, x0, y0, x1, y1);
}

const resquarify = (function custom(ratio) {

  function resquarify(parent, x0, y0, x1, y1) {
    if ((rows = parent._squarify) && (rows.ratio === ratio)) {
      var rows,
          row,
          nodes,
          i,
          j = -1,
          n,
          m = rows.length,
          value = parent.value;

      while (++j < m) {
        row = rows[j], nodes = row.children;
        for (i = row.value = 0, n = nodes.length; i < n; ++i) row.value += nodes[i].value;
        if (row.dice) treemapDice(row, x0, y0, x1, value ? y0 += (y1 - y0) * row.value / value : y1);
        else treemapSlice(row, x0, y0, value ? x0 += (x1 - x0) * row.value / value : x1, y1);
        value -= row.value;
      }
    } else {
      parent._squarify = rows = squarifyRatio(ratio, parent, x0, y0, x1, y1);
      rows.ratio = ratio;
    }
  }

  resquarify.ratio = function(x) {
    return custom((x = +x) > 1 ? x : 1);
  };

  return resquarify;
})(phi);

function area$1(polygon) {
  var i = -1,
      n = polygon.length,
      a,
      b = polygon[n - 1],
      area = 0;

  while (++i < n) {
    a = b;
    b = polygon[i];
    area += a[1] * b[0] - a[0] * b[1];
  }

  return area / 2;
}

function centroid(polygon) {
  var i = -1,
      n = polygon.length,
      x = 0,
      y = 0,
      a,
      b = polygon[n - 1],
      c,
      k = 0;

  while (++i < n) {
    a = b;
    b = polygon[i];
    k += c = a[0] * b[1] - b[0] * a[1];
    x += (a[0] + b[0]) * c;
    y += (a[1] + b[1]) * c;
  }

  return k *= 3, [x / k, y / k];
}

// Returns the 2D cross product of AB and AC vectors, i.e., the z-component of
// the 3D cross product in a quadrant I Cartesian coordinate system (+x is
// right, +y is up). Returns a positive value if ABC is counter-clockwise,
// negative if clockwise, and zero if the points are collinear.
function cross$1(a, b, c) {
  return (b[0] - a[0]) * (c[1] - a[1]) - (b[1] - a[1]) * (c[0] - a[0]);
}

function lexicographicOrder(a, b) {
  return a[0] - b[0] || a[1] - b[1];
}

// Computes the upper convex hull per the monotone chain algorithm.
// Assumes points.length >= 3, is sorted by x, unique in y.
// Returns an array of indices into points in left-to-right order.
function computeUpperHullIndexes(points) {
  const n = points.length,
      indexes = [0, 1];
  let size = 2, i;

  for (i = 2; i < n; ++i) {
    while (size > 1 && cross$1(points[indexes[size - 2]], points[indexes[size - 1]], points[i]) <= 0) --size;
    indexes[size++] = i;
  }

  return indexes.slice(0, size); // remove popped points
}

function hull(points) {
  if ((n = points.length) < 3) return null;

  var i,
      n,
      sortedPoints = new Array(n),
      flippedPoints = new Array(n);

  for (i = 0; i < n; ++i) sortedPoints[i] = [+points[i][0], +points[i][1], i];
  sortedPoints.sort(lexicographicOrder);
  for (i = 0; i < n; ++i) flippedPoints[i] = [sortedPoints[i][0], -sortedPoints[i][1]];

  var upperIndexes = computeUpperHullIndexes(sortedPoints),
      lowerIndexes = computeUpperHullIndexes(flippedPoints);

  // Construct the hull polygon, removing possible duplicate endpoints.
  var skipLeft = lowerIndexes[0] === upperIndexes[0],
      skipRight = lowerIndexes[lowerIndexes.length - 1] === upperIndexes[upperIndexes.length - 1],
      hull = [];

  // Add upper hull in right-to-l order.
  // Then add lower hull in left-to-right order.
  for (i = upperIndexes.length - 1; i >= 0; --i) hull.push(points[sortedPoints[upperIndexes[i]][2]]);
  for (i = +skipLeft; i < lowerIndexes.length - skipRight; ++i) hull.push(points[sortedPoints[lowerIndexes[i]][2]]);

  return hull;
}

function contains(polygon, point) {
  var n = polygon.length,
      p = polygon[n - 1],
      x = point[0], y = point[1],
      x0 = p[0], y0 = p[1],
      x1, y1,
      inside = false;

  for (var i = 0; i < n; ++i) {
    p = polygon[i], x1 = p[0], y1 = p[1];
    if (((y1 > y) !== (y0 > y)) && (x < (x0 - x1) * (y - y1) / (y0 - y1) + x1)) inside = !inside;
    x0 = x1, y0 = y1;
  }

  return inside;
}

function length(polygon) {
  var i = -1,
      n = polygon.length,
      b = polygon[n - 1],
      xa,
      ya,
      xb = b[0],
      yb = b[1],
      perimeter = 0;

  while (++i < n) {
    xa = xb;
    ya = yb;
    b = polygon[i];
    xb = b[0];
    yb = b[1];
    xa -= xb;
    ya -= yb;
    perimeter += Math.hypot(xa, ya);
  }

  return perimeter;
}

const defaultSource = Math.random;

const uniform = (function sourceRandomUniform(source) {
  function randomUniform(min, max) {
    min = min == null ? 0 : +min;
    max = max == null ? 1 : +max;
    if (arguments.length === 1) max = min, min = 0;
    else max -= min;
    return function() {
      return source() * max + min;
    };
  }

  randomUniform.source = sourceRandomUniform;

  return randomUniform;
})(defaultSource);

const int = (function sourceRandomInt(source) {
  function randomInt(min, max) {
    if (arguments.length < 2) max = min, min = 0;
    min = Math.floor(min);
    max = Math.floor(max) - min;
    return function() {
      return Math.floor(source() * max + min);
    };
  }

  randomInt.source = sourceRandomInt;

  return randomInt;
})(defaultSource);

const normal = (function sourceRandomNormal(source) {
  function randomNormal(mu, sigma) {
    var x, r;
    mu = mu == null ? 0 : +mu;
    sigma = sigma == null ? 1 : +sigma;
    return function() {
      var y;

      // If available, use the second previously-generated uniform random.
      if (x != null) y = x, x = null;

      // Otherwise, generate a new x and y.
      else do {
        x = source() * 2 - 1;
        y = source() * 2 - 1;
        r = x * x + y * y;
      } while (!r || r > 1);

      return mu + sigma * y * Math.sqrt(-2 * Math.log(r) / r);
    };
  }

  randomNormal.source = sourceRandomNormal;

  return randomNormal;
})(defaultSource);

const logNormal = (function sourceRandomLogNormal(source) {
  var N = normal.source(source);

  function randomLogNormal() {
    var randomNormal = N.apply(this, arguments);
    return function() {
      return Math.exp(randomNormal());
    };
  }

  randomLogNormal.source = sourceRandomLogNormal;

  return randomLogNormal;
})(defaultSource);

const irwinHall = (function sourceRandomIrwinHall(source) {
  function randomIrwinHall(n) {
    if ((n = +n) <= 0) return () => 0;
    return function() {
      for (var sum = 0, i = n; i > 1; --i) sum += source();
      return sum + i * source();
    };
  }

  randomIrwinHall.source = sourceRandomIrwinHall;

  return randomIrwinHall;
})(defaultSource);

const bates = (function sourceRandomBates(source) {
  var I = irwinHall.source(source);

  function randomBates(n) {
    // use limiting distribution at n === 0
    if ((n = +n) === 0) return source;
    var randomIrwinHall = I(n);
    return function() {
      return randomIrwinHall() / n;
    };
  }

  randomBates.source = sourceRandomBates;

  return randomBates;
})(defaultSource);

const exponential = (function sourceRandomExponential(source) {
  function randomExponential(lambda) {
    return function() {
      return -Math.log1p(-source()) / lambda;
    };
  }

  randomExponential.source = sourceRandomExponential;

  return randomExponential;
})(defaultSource);

const pareto = (function sourceRandomPareto(source) {
  function randomPareto(alpha) {
    if ((alpha = +alpha) < 0) throw new RangeError("invalid alpha");
    alpha = 1 / -alpha;
    return function() {
      return Math.pow(1 - source(), alpha);
    };
  }

  randomPareto.source = sourceRandomPareto;

  return randomPareto;
})(defaultSource);

const bernoulli = (function sourceRandomBernoulli(source) {
  function randomBernoulli(p) {
    if ((p = +p) < 0 || p > 1) throw new RangeError("invalid p");
    return function() {
      return Math.floor(source() + p);
    };
  }

  randomBernoulli.source = sourceRandomBernoulli;

  return randomBernoulli;
})(defaultSource);

const geometric = (function sourceRandomGeometric(source) {
  function randomGeometric(p) {
    if ((p = +p) < 0 || p > 1) throw new RangeError("invalid p");
    if (p === 0) return () => Infinity;
    if (p === 1) return () => 1;
    p = Math.log1p(-p);
    return function() {
      return 1 + Math.floor(Math.log1p(-source()) / p);
    };
  }

  randomGeometric.source = sourceRandomGeometric;

  return randomGeometric;
})(defaultSource);

const gamma = (function sourceRandomGamma(source) {
  var randomNormal = normal.source(source)();

  function randomGamma(k, theta) {
    if ((k = +k) < 0) throw new RangeError("invalid k");
    // degenerate distribution if k === 0
    if (k === 0) return () => 0;
    theta = theta == null ? 1 : +theta;
    // exponential distribution if k === 1
    if (k === 1) return () => -Math.log1p(-source()) * theta;

    var d = (k < 1 ? k + 1 : k) - 1 / 3,
        c = 1 / (3 * Math.sqrt(d)),
        multiplier = k < 1 ? () => Math.pow(source(), 1 / k) : () => 1;
    return function() {
      do {
        do {
          var x = randomNormal(),
              v = 1 + c * x;
        } while (v <= 0);
        v *= v * v;
        var u = 1 - source();
      } while (u >= 1 - 0.0331 * x * x * x * x && Math.log(u) >= 0.5 * x * x + d * (1 - v + Math.log(v)));
      return d * v * multiplier() * theta;
    };
  }

  randomGamma.source = sourceRandomGamma;

  return randomGamma;
})(defaultSource);

const beta = (function sourceRandomBeta(source) {
  var G = gamma.source(source);

  function randomBeta(alpha, beta) {
    var X = G(alpha),
        Y = G(beta);
    return function() {
      var x = X();
      return x === 0 ? 0 : x / (x + Y());
    };
  }

  randomBeta.source = sourceRandomBeta;

  return randomBeta;
})(defaultSource);

const binomial = (function sourceRandomBinomial(source) {
  var G = geometric.source(source),
      B = beta.source(source);

  function randomBinomial(n, p) {
    n = +n;
    if ((p = +p) >= 1) return () => n;
    if (p <= 0) return () => 0;
    return function() {
      var acc = 0, nn = n, pp = p;
      while (nn * pp > 16 && nn * (1 - pp) > 16) {
        var i = Math.floor((nn + 1) * pp),
            y = B(i, nn - i + 1)();
        if (y <= pp) {
          acc += i;
          nn -= i;
          pp = (pp - y) / (1 - y);
        } else {
          nn = i - 1;
          pp /= y;
        }
      }
      var sign = pp < 0.5,
          pFinal = sign ? pp : 1 - pp,
          g = G(pFinal);
      for (var s = g(), k = 0; s <= nn; ++k) s += g();
      return acc + (sign ? k : nn - k);
    };
  }

  randomBinomial.source = sourceRandomBinomial;

  return randomBinomial;
})(defaultSource);

const weibull = (function sourceRandomWeibull(source) {
  function randomWeibull(k, a, b) {
    var outerFunc;
    if ((k = +k) === 0) {
      outerFunc = x => -Math.log(x);
    } else {
      k = 1 / k;
      outerFunc = x => Math.pow(x, k);
    }
    a = a == null ? 0 : +a;
    b = b == null ? 1 : +b;
    return function() {
      return a + b * outerFunc(-Math.log1p(-source()));
    };
  }

  randomWeibull.source = sourceRandomWeibull;

  return randomWeibull;
})(defaultSource);

const cauchy = (function sourceRandomCauchy(source) {
  function randomCauchy(a, b) {
    a = a == null ? 0 : +a;
    b = b == null ? 1 : +b;
    return function() {
      return a + b * Math.tan(Math.PI * source());
    };
  }

  randomCauchy.source = sourceRandomCauchy;

  return randomCauchy;
})(defaultSource);

const logistic = (function sourceRandomLogistic(source) {
  function randomLogistic(a, b) {
    a = a == null ? 0 : +a;
    b = b == null ? 1 : +b;
    return function() {
      var u = source();
      return a + b * Math.log(u / (1 - u));
    };
  }

  randomLogistic.source = sourceRandomLogistic;

  return randomLogistic;
})(defaultSource);

const poisson = (function sourceRandomPoisson(source) {
  var G = gamma.source(source),
      B = binomial.source(source);

  function randomPoisson(lambda) {
    return function() {
      var acc = 0, l = lambda;
      while (l > 16) {
        var n = Math.floor(0.875 * l),
            t = G(n)();
        if (t > l) return acc + B(n - 1, l / t)();
        acc += n;
        l -= t;
      }
      for (var s = -Math.log1p(-source()), k = 0; s <= l; ++k) s -= Math.log1p(-source());
      return acc + k;
    };
  }

  randomPoisson.source = sourceRandomPoisson;

  return randomPoisson;
})(defaultSource);

// https://en.wikipedia.org/wiki/Linear_congruential_generator#Parameters_in_common_use
const mul = 0x19660D;
const inc = 0x3C6EF35F;
const eps = 1 / 0x100000000;

function lcg(seed = Math.random()) {
  let state = (0 <= seed && seed < 1 ? seed / eps : Math.abs(seed)) | 0;
  return () => (state = mul * state + inc | 0, eps * (state >>> 0));
}

function initRange(domain, range) {
  switch (arguments.length) {
    case 0: break;
    case 1: this.range(domain); break;
    default: this.range(range).domain(domain); break;
  }
  return this;
}

function initInterpolator(domain, interpolator) {
  switch (arguments.length) {
    case 0: break;
    case 1: {
      if (typeof domain === "function") this.interpolator(domain);
      else this.range(domain);
      break;
    }
    default: {
      this.domain(domain);
      if (typeof interpolator === "function") this.interpolator(interpolator);
      else this.range(interpolator);
      break;
    }
  }
  return this;
}

const implicit = Symbol("implicit");

function ordinal() {
  var index = new InternMap(),
      domain = [],
      range = [],
      unknown = implicit;

  function scale(d) {
    let i = index.get(d);
    if (i === undefined) {
      if (unknown !== implicit) return unknown;
      index.set(d, i = domain.push(d) - 1);
    }
    return range[i % range.length];
  }

  scale.domain = function(_) {
    if (!arguments.length) return domain.slice();
    domain = [], index = new InternMap();
    for (const value of _) {
      if (index.has(value)) continue;
      index.set(value, domain.push(value) - 1);
    }
    return scale;
  };

  scale.range = function(_) {
    return arguments.length ? (range = Array.from(_), scale) : range.slice();
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  scale.copy = function() {
    return ordinal(domain, range).unknown(unknown);
  };

  initRange.apply(scale, arguments);

  return scale;
}

function band() {
  var scale = ordinal().unknown(undefined),
      domain = scale.domain,
      ordinalRange = scale.range,
      r0 = 0,
      r1 = 1,
      step,
      bandwidth,
      round = false,
      paddingInner = 0,
      paddingOuter = 0,
      align = 0.5;

  delete scale.unknown;

  function rescale() {
    var n = domain().length,
        reverse = r1 < r0,
        start = reverse ? r1 : r0,
        stop = reverse ? r0 : r1;
    step = (stop - start) / Math.max(1, n - paddingInner + paddingOuter * 2);
    if (round) step = Math.floor(step);
    start += (stop - start - step * (n - paddingInner)) * align;
    bandwidth = step * (1 - paddingInner);
    if (round) start = Math.round(start), bandwidth = Math.round(bandwidth);
    var values = range$2(n).map(function(i) { return start + step * i; });
    return ordinalRange(reverse ? values.reverse() : values);
  }

  scale.domain = function(_) {
    return arguments.length ? (domain(_), rescale()) : domain();
  };

  scale.range = function(_) {
    return arguments.length ? ([r0, r1] = _, r0 = +r0, r1 = +r1, rescale()) : [r0, r1];
  };

  scale.rangeRound = function(_) {
    return [r0, r1] = _, r0 = +r0, r1 = +r1, round = true, rescale();
  };

  scale.bandwidth = function() {
    return bandwidth;
  };

  scale.step = function() {
    return step;
  };

  scale.round = function(_) {
    return arguments.length ? (round = !!_, rescale()) : round;
  };

  scale.padding = function(_) {
    return arguments.length ? (paddingInner = Math.min(1, paddingOuter = +_), rescale()) : paddingInner;
  };

  scale.paddingInner = function(_) {
    return arguments.length ? (paddingInner = Math.min(1, _), rescale()) : paddingInner;
  };

  scale.paddingOuter = function(_) {
    return arguments.length ? (paddingOuter = +_, rescale()) : paddingOuter;
  };

  scale.align = function(_) {
    return arguments.length ? (align = Math.max(0, Math.min(1, _)), rescale()) : align;
  };

  scale.copy = function() {
    return band(domain(), [r0, r1])
        .round(round)
        .paddingInner(paddingInner)
        .paddingOuter(paddingOuter)
        .align(align);
  };

  return initRange.apply(rescale(), arguments);
}

function pointish(scale) {
  var copy = scale.copy;

  scale.padding = scale.paddingOuter;
  delete scale.paddingInner;
  delete scale.paddingOuter;

  scale.copy = function() {
    return pointish(copy());
  };

  return scale;
}

function point$4() {
  return pointish(band.apply(null, arguments).paddingInner(1));
}

function constants(x) {
  return function() {
    return x;
  };
}

function number$1(x) {
  return +x;
}

var unit = [0, 1];

function identity$3(x) {
  return x;
}

function normalize(a, b) {
  return (b -= (a = +a))
      ? function(x) { return (x - a) / b; }
      : constants(isNaN(b) ? NaN : 0.5);
}

function clamper(a, b) {
  var t;
  if (a > b) t = a, a = b, b = t;
  return function(x) { return Math.max(a, Math.min(b, x)); };
}

// normalize(a, b)(x) takes a domain value x in [a,b] and returns the corresponding parameter t in [0,1].
// interpolate(a, b)(t) takes a parameter t in [0,1] and returns the corresponding range value x in [a,b].
function bimap(domain, range, interpolate) {
  var d0 = domain[0], d1 = domain[1], r0 = range[0], r1 = range[1];
  if (d1 < d0) d0 = normalize(d1, d0), r0 = interpolate(r1, r0);
  else d0 = normalize(d0, d1), r0 = interpolate(r0, r1);
  return function(x) { return r0(d0(x)); };
}

function polymap(domain, range, interpolate) {
  var j = Math.min(domain.length, range.length) - 1,
      d = new Array(j),
      r = new Array(j),
      i = -1;

  // Reverse descending domains.
  if (domain[j] < domain[0]) {
    domain = domain.slice().reverse();
    range = range.slice().reverse();
  }

  while (++i < j) {
    d[i] = normalize(domain[i], domain[i + 1]);
    r[i] = interpolate(range[i], range[i + 1]);
  }

  return function(x) {
    var i = bisectRight(domain, x, 1, j) - 1;
    return r[i](d[i](x));
  };
}

function copy$1(source, target) {
  return target
      .domain(source.domain())
      .range(source.range())
      .interpolate(source.interpolate())
      .clamp(source.clamp())
      .unknown(source.unknown());
}

function transformer$2() {
  var domain = unit,
      range = unit,
      interpolate = interpolate$2,
      transform,
      untransform,
      unknown,
      clamp = identity$3,
      piecewise,
      output,
      input;

  function rescale() {
    var n = Math.min(domain.length, range.length);
    if (clamp !== identity$3) clamp = clamper(domain[0], domain[n - 1]);
    piecewise = n > 2 ? polymap : bimap;
    output = input = null;
    return scale;
  }

  function scale(x) {
    return x == null || isNaN(x = +x) ? unknown : (output || (output = piecewise(domain.map(transform), range, interpolate)))(transform(clamp(x)));
  }

  scale.invert = function(y) {
    return clamp(untransform((input || (input = piecewise(range, domain.map(transform), interpolateNumber)))(y)));
  };

  scale.domain = function(_) {
    return arguments.length ? (domain = Array.from(_, number$1), rescale()) : domain.slice();
  };

  scale.range = function(_) {
    return arguments.length ? (range = Array.from(_), rescale()) : range.slice();
  };

  scale.rangeRound = function(_) {
    return range = Array.from(_), interpolate = interpolateRound, rescale();
  };

  scale.clamp = function(_) {
    return arguments.length ? (clamp = _ ? true : identity$3, rescale()) : clamp !== identity$3;
  };

  scale.interpolate = function(_) {
    return arguments.length ? (interpolate = _, rescale()) : interpolate;
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  return function(t, u) {
    transform = t, untransform = u;
    return rescale();
  };
}

function continuous() {
  return transformer$2()(identity$3, identity$3);
}

function tickFormat(start, stop, count, specifier) {
  var step = tickStep(start, stop, count),
      precision;
  specifier = formatSpecifier(specifier == null ? ",f" : specifier);
  switch (specifier.type) {
    case "s": {
      var value = Math.max(Math.abs(start), Math.abs(stop));
      if (specifier.precision == null && !isNaN(precision = precisionPrefix(step, value))) specifier.precision = precision;
      return formatPrefix(specifier, value);
    }
    case "":
    case "e":
    case "g":
    case "p":
    case "r": {
      if (specifier.precision == null && !isNaN(precision = precisionRound(step, Math.max(Math.abs(start), Math.abs(stop))))) specifier.precision = precision - (specifier.type === "e");
      break;
    }
    case "f":
    case "%": {
      if (specifier.precision == null && !isNaN(precision = precisionFixed(step))) specifier.precision = precision - (specifier.type === "%") * 2;
      break;
    }
  }
  return format(specifier);
}

function linearish(scale) {
  var domain = scale.domain;

  scale.ticks = function(count) {
    var d = domain();
    return ticks(d[0], d[d.length - 1], count == null ? 10 : count);
  };

  scale.tickFormat = function(count, specifier) {
    var d = domain();
    return tickFormat(d[0], d[d.length - 1], count == null ? 10 : count, specifier);
  };

  scale.nice = function(count) {
    if (count == null) count = 10;

    var d = domain();
    var i0 = 0;
    var i1 = d.length - 1;
    var start = d[i0];
    var stop = d[i1];
    var prestep;
    var step;
    var maxIter = 10;

    if (stop < start) {
      step = start, start = stop, stop = step;
      step = i0, i0 = i1, i1 = step;
    }
    
    while (maxIter-- > 0) {
      step = tickIncrement(start, stop, count);
      if (step === prestep) {
        d[i0] = start;
        d[i1] = stop;
        return domain(d);
      } else if (step > 0) {
        start = Math.floor(start / step) * step;
        stop = Math.ceil(stop / step) * step;
      } else if (step < 0) {
        start = Math.ceil(start * step) / step;
        stop = Math.floor(stop * step) / step;
      } else {
        break;
      }
      prestep = step;
    }

    return scale;
  };

  return scale;
}

function linear() {
  var scale = continuous();

  scale.copy = function() {
    return copy$1(scale, linear());
  };

  initRange.apply(scale, arguments);

  return linearish(scale);
}

function identity$2(domain) {
  var unknown;

  function scale(x) {
    return x == null || isNaN(x = +x) ? unknown : x;
  }

  scale.invert = scale;

  scale.domain = scale.range = function(_) {
    return arguments.length ? (domain = Array.from(_, number$1), scale) : domain.slice();
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  scale.copy = function() {
    return identity$2(domain).unknown(unknown);
  };

  domain = arguments.length ? Array.from(domain, number$1) : [0, 1];

  return linearish(scale);
}

function nice(domain, interval) {
  domain = domain.slice();

  var i0 = 0,
      i1 = domain.length - 1,
      x0 = domain[i0],
      x1 = domain[i1],
      t;

  if (x1 < x0) {
    t = i0, i0 = i1, i1 = t;
    t = x0, x0 = x1, x1 = t;
  }

  domain[i0] = interval.floor(x0);
  domain[i1] = interval.ceil(x1);
  return domain;
}

function transformLog(x) {
  return Math.log(x);
}

function transformExp(x) {
  return Math.exp(x);
}

function transformLogn(x) {
  return -Math.log(-x);
}

function transformExpn(x) {
  return -Math.exp(-x);
}

function pow10(x) {
  return isFinite(x) ? +("1e" + x) : x < 0 ? 0 : x;
}

function powp(base) {
  return base === 10 ? pow10
      : base === Math.E ? Math.exp
      : x => Math.pow(base, x);
}

function logp(base) {
  return base === Math.E ? Math.log
      : base === 10 && Math.log10
      || base === 2 && Math.log2
      || (base = Math.log(base), x => Math.log(x) / base);
}

function reflect(f) {
  return (x, k) => -f(-x, k);
}

function loggish(transform) {
  const scale = transform(transformLog, transformExp);
  const domain = scale.domain;
  let base = 10;
  let logs;
  let pows;

  function rescale() {
    logs = logp(base), pows = powp(base);
    if (domain()[0] < 0) {
      logs = reflect(logs), pows = reflect(pows);
      transform(transformLogn, transformExpn);
    } else {
      transform(transformLog, transformExp);
    }
    return scale;
  }

  scale.base = function(_) {
    return arguments.length ? (base = +_, rescale()) : base;
  };

  scale.domain = function(_) {
    return arguments.length ? (domain(_), rescale()) : domain();
  };

  scale.ticks = count => {
    const d = domain();
    let u = d[0];
    let v = d[d.length - 1];
    const r = v < u;

    if (r) ([u, v] = [v, u]);

    let i = logs(u);
    let j = logs(v);
    let k;
    let t;
    const n = count == null ? 10 : +count;
    let z = [];

    if (!(base % 1) && j - i < n) {
      i = Math.floor(i), j = Math.ceil(j);
      if (u > 0) for (; i <= j; ++i) {
        for (k = 1; k < base; ++k) {
          t = i < 0 ? k / pows(-i) : k * pows(i);
          if (t < u) continue;
          if (t > v) break;
          z.push(t);
        }
      } else for (; i <= j; ++i) {
        for (k = base - 1; k >= 1; --k) {
          t = i > 0 ? k / pows(-i) : k * pows(i);
          if (t < u) continue;
          if (t > v) break;
          z.push(t);
        }
      }
      if (z.length * 2 < n) z = ticks(u, v, n);
    } else {
      z = ticks(i, j, Math.min(j - i, n)).map(pows);
    }
    return r ? z.reverse() : z;
  };

  scale.tickFormat = (count, specifier) => {
    if (count == null) count = 10;
    if (specifier == null) specifier = base === 10 ? "s" : ",";
    if (typeof specifier !== "function") {
      if (!(base % 1) && (specifier = formatSpecifier(specifier)).precision == null) specifier.trim = true;
      specifier = format(specifier);
    }
    if (count === Infinity) return specifier;
    const k = Math.max(1, base * count / scale.ticks().length); // TODO fast estimate?
    return d => {
      let i = d / pows(Math.round(logs(d)));
      if (i * base < base - 0.5) i *= base;
      return i <= k ? specifier(d) : "";
    };
  };

  scale.nice = () => {
    return domain(nice(domain(), {
      floor: x => pows(Math.floor(logs(x))),
      ceil: x => pows(Math.ceil(logs(x)))
    }));
  };

  return scale;
}

function log() {
  const scale = loggish(transformer$2()).domain([1, 10]);
  scale.copy = () => copy$1(scale, log()).base(scale.base());
  initRange.apply(scale, arguments);
  return scale;
}

function transformSymlog(c) {
  return function(x) {
    return Math.sign(x) * Math.log1p(Math.abs(x / c));
  };
}

function transformSymexp(c) {
  return function(x) {
    return Math.sign(x) * Math.expm1(Math.abs(x)) * c;
  };
}

function symlogish(transform) {
  var c = 1, scale = transform(transformSymlog(c), transformSymexp(c));

  scale.constant = function(_) {
    return arguments.length ? transform(transformSymlog(c = +_), transformSymexp(c)) : c;
  };

  return linearish(scale);
}

function symlog() {
  var scale = symlogish(transformer$2());

  scale.copy = function() {
    return copy$1(scale, symlog()).constant(scale.constant());
  };

  return initRange.apply(scale, arguments);
}

function transformPow(exponent) {
  return function(x) {
    return x < 0 ? -Math.pow(-x, exponent) : Math.pow(x, exponent);
  };
}

function transformSqrt(x) {
  return x < 0 ? -Math.sqrt(-x) : Math.sqrt(x);
}

function transformSquare(x) {
  return x < 0 ? -x * x : x * x;
}

function powish(transform) {
  var scale = transform(identity$3, identity$3),
      exponent = 1;

  function rescale() {
    return exponent === 1 ? transform(identity$3, identity$3)
        : exponent === 0.5 ? transform(transformSqrt, transformSquare)
        : transform(transformPow(exponent), transformPow(1 / exponent));
  }

  scale.exponent = function(_) {
    return arguments.length ? (exponent = +_, rescale()) : exponent;
  };

  return linearish(scale);
}

function pow() {
  var scale = powish(transformer$2());

  scale.copy = function() {
    return copy$1(scale, pow()).exponent(scale.exponent());
  };

  initRange.apply(scale, arguments);

  return scale;
}

function sqrt$1() {
  return pow.apply(null, arguments).exponent(0.5);
}

function square$1(x) {
  return Math.sign(x) * x * x;
}

function unsquare(x) {
  return Math.sign(x) * Math.sqrt(Math.abs(x));
}

function radial() {
  var squared = continuous(),
      range = [0, 1],
      round = false,
      unknown;

  function scale(x) {
    var y = unsquare(squared(x));
    return isNaN(y) ? unknown : round ? Math.round(y) : y;
  }

  scale.invert = function(y) {
    return squared.invert(square$1(y));
  };

  scale.domain = function(_) {
    return arguments.length ? (squared.domain(_), scale) : squared.domain();
  };

  scale.range = function(_) {
    return arguments.length ? (squared.range((range = Array.from(_, number$1)).map(square$1)), scale) : range.slice();
  };

  scale.rangeRound = function(_) {
    return scale.range(_).round(true);
  };

  scale.round = function(_) {
    return arguments.length ? (round = !!_, scale) : round;
  };

  scale.clamp = function(_) {
    return arguments.length ? (squared.clamp(_), scale) : squared.clamp();
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  scale.copy = function() {
    return radial(squared.domain(), range)
        .round(round)
        .clamp(squared.clamp())
        .unknown(unknown);
  };

  initRange.apply(scale, arguments);

  return linearish(scale);
}

function quantile() {
  var domain = [],
      range = [],
      thresholds = [],
      unknown;

  function rescale() {
    var i = 0, n = Math.max(1, range.length);
    thresholds = new Array(n - 1);
    while (++i < n) thresholds[i - 1] = quantileSorted(domain, i / n);
    return scale;
  }

  function scale(x) {
    return x == null || isNaN(x = +x) ? unknown : range[bisectRight(thresholds, x)];
  }

  scale.invertExtent = function(y) {
    var i = range.indexOf(y);
    return i < 0 ? [NaN, NaN] : [
      i > 0 ? thresholds[i - 1] : domain[0],
      i < thresholds.length ? thresholds[i] : domain[domain.length - 1]
    ];
  };

  scale.domain = function(_) {
    if (!arguments.length) return domain.slice();
    domain = [];
    for (let d of _) if (d != null && !isNaN(d = +d)) domain.push(d);
    domain.sort(ascending$3);
    return rescale();
  };

  scale.range = function(_) {
    return arguments.length ? (range = Array.from(_), rescale()) : range.slice();
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  scale.quantiles = function() {
    return thresholds.slice();
  };

  scale.copy = function() {
    return quantile()
        .domain(domain)
        .range(range)
        .unknown(unknown);
  };

  return initRange.apply(scale, arguments);
}

function quantize() {
  var x0 = 0,
      x1 = 1,
      n = 1,
      domain = [0.5],
      range = [0, 1],
      unknown;

  function scale(x) {
    return x != null && x <= x ? range[bisectRight(domain, x, 0, n)] : unknown;
  }

  function rescale() {
    var i = -1;
    domain = new Array(n);
    while (++i < n) domain[i] = ((i + 1) * x1 - (i - n) * x0) / (n + 1);
    return scale;
  }

  scale.domain = function(_) {
    return arguments.length ? ([x0, x1] = _, x0 = +x0, x1 = +x1, rescale()) : [x0, x1];
  };

  scale.range = function(_) {
    return arguments.length ? (n = (range = Array.from(_)).length - 1, rescale()) : range.slice();
  };

  scale.invertExtent = function(y) {
    var i = range.indexOf(y);
    return i < 0 ? [NaN, NaN]
        : i < 1 ? [x0, domain[0]]
        : i >= n ? [domain[n - 1], x1]
        : [domain[i - 1], domain[i]];
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : scale;
  };

  scale.thresholds = function() {
    return domain.slice();
  };

  scale.copy = function() {
    return quantize()
        .domain([x0, x1])
        .range(range)
        .unknown(unknown);
  };

  return initRange.apply(linearish(scale), arguments);
}

function threshold() {
  var domain = [0.5],
      range = [0, 1],
      unknown,
      n = 1;

  function scale(x) {
    return x != null && x <= x ? range[bisectRight(domain, x, 0, n)] : unknown;
  }

  scale.domain = function(_) {
    return arguments.length ? (domain = Array.from(_), n = Math.min(domain.length, range.length - 1), scale) : domain.slice();
  };

  scale.range = function(_) {
    return arguments.length ? (range = Array.from(_), n = Math.min(domain.length, range.length - 1), scale) : range.slice();
  };

  scale.invertExtent = function(y) {
    var i = range.indexOf(y);
    return [domain[i - 1], domain[i]];
  };

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  scale.copy = function() {
    return threshold()
        .domain(domain)
        .range(range)
        .unknown(unknown);
  };

  return initRange.apply(scale, arguments);
}

const t0 = new Date, t1 = new Date;

function timeInterval(floori, offseti, count, field) {

  function interval(date) {
    return floori(date = arguments.length === 0 ? new Date : new Date(+date)), date;
  }

  interval.floor = (date) => {
    return floori(date = new Date(+date)), date;
  };

  interval.ceil = (date) => {
    return floori(date = new Date(date - 1)), offseti(date, 1), floori(date), date;
  };

  interval.round = (date) => {
    const d0 = interval(date), d1 = interval.ceil(date);
    return date - d0 < d1 - date ? d0 : d1;
  };

  interval.offset = (date, step) => {
    return offseti(date = new Date(+date), step == null ? 1 : Math.floor(step)), date;
  };

  interval.range = (start, stop, step) => {
    const range = [];
    start = interval.ceil(start);
    step = step == null ? 1 : Math.floor(step);
    if (!(start < stop) || !(step > 0)) return range; // also handles Invalid Date
    let previous;
    do range.push(previous = new Date(+start)), offseti(start, step), floori(start);
    while (previous < start && start < stop);
    return range;
  };

  interval.filter = (test) => {
    return timeInterval((date) => {
      if (date >= date) while (floori(date), !test(date)) date.setTime(date - 1);
    }, (date, step) => {
      if (date >= date) {
        if (step < 0) while (++step <= 0) {
          while (offseti(date, -1), !test(date)) {} // eslint-disable-line no-empty
        } else while (--step >= 0) {
          while (offseti(date, +1), !test(date)) {} // eslint-disable-line no-empty
        }
      }
    });
  };

  if (count) {
    interval.count = (start, end) => {
      t0.setTime(+start), t1.setTime(+end);
      floori(t0), floori(t1);
      return Math.floor(count(t0, t1));
    };

    interval.every = (step) => {
      step = Math.floor(step);
      return !isFinite(step) || !(step > 0) ? null
          : !(step > 1) ? interval
          : interval.filter(field
              ? (d) => field(d) % step === 0
              : (d) => interval.count(0, d) % step === 0);
    };
  }

  return interval;
}

const millisecond = timeInterval(() => {
  // noop
}, (date, step) => {
  date.setTime(+date + step);
}, (start, end) => {
  return end - start;
});

// An optimized implementation for this simple case.
millisecond.every = (k) => {
  k = Math.floor(k);
  if (!isFinite(k) || !(k > 0)) return null;
  if (!(k > 1)) return millisecond;
  return timeInterval((date) => {
    date.setTime(Math.floor(date / k) * k);
  }, (date, step) => {
    date.setTime(+date + step * k);
  }, (start, end) => {
    return (end - start) / k;
  });
};

const milliseconds = millisecond.range;

const durationSecond = 1000;
const durationMinute = durationSecond * 60;
const durationHour = durationMinute * 60;
const durationDay = durationHour * 24;
const durationWeek = durationDay * 7;
const durationMonth = durationDay * 30;
const durationYear = durationDay * 365;

const second = timeInterval((date) => {
  date.setTime(date - date.getMilliseconds());
}, (date, step) => {
  date.setTime(+date + step * durationSecond);
}, (start, end) => {
  return (end - start) / durationSecond;
}, (date) => {
  return date.getUTCSeconds();
});

const seconds = second.range;

const timeMinute = timeInterval((date) => {
  date.setTime(date - date.getMilliseconds() - date.getSeconds() * durationSecond);
}, (date, step) => {
  date.setTime(+date + step * durationMinute);
}, (start, end) => {
  return (end - start) / durationMinute;
}, (date) => {
  return date.getMinutes();
});

const timeMinutes = timeMinute.range;

const utcMinute = timeInterval((date) => {
  date.setUTCSeconds(0, 0);
}, (date, step) => {
  date.setTime(+date + step * durationMinute);
}, (start, end) => {
  return (end - start) / durationMinute;
}, (date) => {
  return date.getUTCMinutes();
});

const utcMinutes = utcMinute.range;

const timeHour = timeInterval((date) => {
  date.setTime(date - date.getMilliseconds() - date.getSeconds() * durationSecond - date.getMinutes() * durationMinute);
}, (date, step) => {
  date.setTime(+date + step * durationHour);
}, (start, end) => {
  return (end - start) / durationHour;
}, (date) => {
  return date.getHours();
});

const timeHours = timeHour.range;

const utcHour = timeInterval((date) => {
  date.setUTCMinutes(0, 0, 0);
}, (date, step) => {
  date.setTime(+date + step * durationHour);
}, (start, end) => {
  return (end - start) / durationHour;
}, (date) => {
  return date.getUTCHours();
});

const utcHours = utcHour.range;

const timeDay = timeInterval(
  date => date.setHours(0, 0, 0, 0),
  (date, step) => date.setDate(date.getDate() + step),
  (start, end) => (end - start - (end.getTimezoneOffset() - start.getTimezoneOffset()) * durationMinute) / durationDay,
  date => date.getDate() - 1
);

const timeDays = timeDay.range;

const utcDay = timeInterval((date) => {
  date.setUTCHours(0, 0, 0, 0);
}, (date, step) => {
  date.setUTCDate(date.getUTCDate() + step);
}, (start, end) => {
  return (end - start) / durationDay;
}, (date) => {
  return date.getUTCDate() - 1;
});

const utcDays = utcDay.range;

const unixDay = timeInterval((date) => {
  date.setUTCHours(0, 0, 0, 0);
}, (date, step) => {
  date.setUTCDate(date.getUTCDate() + step);
}, (start, end) => {
  return (end - start) / durationDay;
}, (date) => {
  return Math.floor(date / durationDay);
});

const unixDays = unixDay.range;

function timeWeekday(i) {
  return timeInterval((date) => {
    date.setDate(date.getDate() - (date.getDay() + 7 - i) % 7);
    date.setHours(0, 0, 0, 0);
  }, (date, step) => {
    date.setDate(date.getDate() + step * 7);
  }, (start, end) => {
    return (end - start - (end.getTimezoneOffset() - start.getTimezoneOffset()) * durationMinute) / durationWeek;
  });
}

const timeSunday = timeWeekday(0);
const timeMonday = timeWeekday(1);
const timeTuesday = timeWeekday(2);
const timeWednesday = timeWeekday(3);
const timeThursday = timeWeekday(4);
const timeFriday = timeWeekday(5);
const timeSaturday = timeWeekday(6);

const timeSundays = timeSunday.range;
const timeMondays = timeMonday.range;
const timeTuesdays = timeTuesday.range;
const timeWednesdays = timeWednesday.range;
const timeThursdays = timeThursday.range;
const timeFridays = timeFriday.range;
const timeSaturdays = timeSaturday.range;

function utcWeekday(i) {
  return timeInterval((date) => {
    date.setUTCDate(date.getUTCDate() - (date.getUTCDay() + 7 - i) % 7);
    date.setUTCHours(0, 0, 0, 0);
  }, (date, step) => {
    date.setUTCDate(date.getUTCDate() + step * 7);
  }, (start, end) => {
    return (end - start) / durationWeek;
  });
}

const utcSunday = utcWeekday(0);
const utcMonday = utcWeekday(1);
const utcTuesday = utcWeekday(2);
const utcWednesday = utcWeekday(3);
const utcThursday = utcWeekday(4);
const utcFriday = utcWeekday(5);
const utcSaturday = utcWeekday(6);

const utcSundays = utcSunday.range;
const utcMondays = utcMonday.range;
const utcTuesdays = utcTuesday.range;
const utcWednesdays = utcWednesday.range;
const utcThursdays = utcThursday.range;
const utcFridays = utcFriday.range;
const utcSaturdays = utcSaturday.range;

const timeMonth = timeInterval((date) => {
  date.setDate(1);
  date.setHours(0, 0, 0, 0);
}, (date, step) => {
  date.setMonth(date.getMonth() + step);
}, (start, end) => {
  return end.getMonth() - start.getMonth() + (end.getFullYear() - start.getFullYear()) * 12;
}, (date) => {
  return date.getMonth();
});

const timeMonths = timeMonth.range;

const utcMonth = timeInterval((date) => {
  date.setUTCDate(1);
  date.setUTCHours(0, 0, 0, 0);
}, (date, step) => {
  date.setUTCMonth(date.getUTCMonth() + step);
}, (start, end) => {
  return end.getUTCMonth() - start.getUTCMonth() + (end.getUTCFullYear() - start.getUTCFullYear()) * 12;
}, (date) => {
  return date.getUTCMonth();
});

const utcMonths = utcMonth.range;

const timeYear = timeInterval((date) => {
  date.setMonth(0, 1);
  date.setHours(0, 0, 0, 0);
}, (date, step) => {
  date.setFullYear(date.getFullYear() + step);
}, (start, end) => {
  return end.getFullYear() - start.getFullYear();
}, (date) => {
  return date.getFullYear();
});

// An optimized implementation for this simple case.
timeYear.every = (k) => {
  return !isFinite(k = Math.floor(k)) || !(k > 0) ? null : timeInterval((date) => {
    date.setFullYear(Math.floor(date.getFullYear() / k) * k);
    date.setMonth(0, 1);
    date.setHours(0, 0, 0, 0);
  }, (date, step) => {
    date.setFullYear(date.getFullYear() + step * k);
  });
};

const timeYears = timeYear.range;

const utcYear = timeInterval((date) => {
  date.setUTCMonth(0, 1);
  date.setUTCHours(0, 0, 0, 0);
}, (date, step) => {
  date.setUTCFullYear(date.getUTCFullYear() + step);
}, (start, end) => {
  return end.getUTCFullYear() - start.getUTCFullYear();
}, (date) => {
  return date.getUTCFullYear();
});

// An optimized implementation for this simple case.
utcYear.every = (k) => {
  return !isFinite(k = Math.floor(k)) || !(k > 0) ? null : timeInterval((date) => {
    date.setUTCFullYear(Math.floor(date.getUTCFullYear() / k) * k);
    date.setUTCMonth(0, 1);
    date.setUTCHours(0, 0, 0, 0);
  }, (date, step) => {
    date.setUTCFullYear(date.getUTCFullYear() + step * k);
  });
};

const utcYears = utcYear.range;

function ticker(year, month, week, day, hour, minute) {

  const tickIntervals = [
    [second,  1,      durationSecond],
    [second,  5,  5 * durationSecond],
    [second, 15, 15 * durationSecond],
    [second, 30, 30 * durationSecond],
    [minute,  1,      durationMinute],
    [minute,  5,  5 * durationMinute],
    [minute, 15, 15 * durationMinute],
    [minute, 30, 30 * durationMinute],
    [  hour,  1,      durationHour  ],
    [  hour,  3,  3 * durationHour  ],
    [  hour,  6,  6 * durationHour  ],
    [  hour, 12, 12 * durationHour  ],
    [   day,  1,      durationDay   ],
    [   day,  2,  2 * durationDay   ],
    [  week,  1,      durationWeek  ],
    [ month,  1,      durationMonth ],
    [ month,  3,  3 * durationMonth ],
    [  year,  1,      durationYear  ]
  ];

  function ticks(start, stop, count) {
    const reverse = stop < start;
    if (reverse) [start, stop] = [stop, start];
    const interval = count && typeof count.range === "function" ? count : tickInterval(start, stop, count);
    const ticks = interval ? interval.range(start, +stop + 1) : []; // inclusive stop
    return reverse ? ticks.reverse() : ticks;
  }

  function tickInterval(start, stop, count) {
    const target = Math.abs(stop - start) / count;
    const i = bisector(([,, step]) => step).right(tickIntervals, target);
    if (i === tickIntervals.length) return year.every(tickStep(start / durationYear, stop / durationYear, count));
    if (i === 0) return millisecond.every(Math.max(tickStep(start, stop, count), 1));
    const [t, step] = tickIntervals[target / tickIntervals[i - 1][2] < tickIntervals[i][2] / target ? i - 1 : i];
    return t.every(step);
  }

  return [ticks, tickInterval];
}

const [utcTicks, utcTickInterval] = ticker(utcYear, utcMonth, utcSunday, unixDay, utcHour, utcMinute);
const [timeTicks, timeTickInterval] = ticker(timeYear, timeMonth, timeSunday, timeDay, timeHour, timeMinute);

function localDate(d) {
  if (0 <= d.y && d.y < 100) {
    var date = new Date(-1, d.m, d.d, d.H, d.M, d.S, d.L);
    date.setFullYear(d.y);
    return date;
  }
  return new Date(d.y, d.m, d.d, d.H, d.M, d.S, d.L);
}

function utcDate(d) {
  if (0 <= d.y && d.y < 100) {
    var date = new Date(Date.UTC(-1, d.m, d.d, d.H, d.M, d.S, d.L));
    date.setUTCFullYear(d.y);
    return date;
  }
  return new Date(Date.UTC(d.y, d.m, d.d, d.H, d.M, d.S, d.L));
}

function newDate(y, m, d) {
  return {y: y, m: m, d: d, H: 0, M: 0, S: 0, L: 0};
}

function formatLocale(locale) {
  var locale_dateTime = locale.dateTime,
      locale_date = locale.date,
      locale_time = locale.time,
      locale_periods = locale.periods,
      locale_weekdays = locale.days,
      locale_shortWeekdays = locale.shortDays,
      locale_months = locale.months,
      locale_shortMonths = locale.shortMonths;

  var periodRe = formatRe(locale_periods),
      periodLookup = formatLookup(locale_periods),
      weekdayRe = formatRe(locale_weekdays),
      weekdayLookup = formatLookup(locale_weekdays),
      shortWeekdayRe = formatRe(locale_shortWeekdays),
      shortWeekdayLookup = formatLookup(locale_shortWeekdays),
      monthRe = formatRe(locale_months),
      monthLookup = formatLookup(locale_months),
      shortMonthRe = formatRe(locale_shortMonths),
      shortMonthLookup = formatLookup(locale_shortMonths);

  var formats = {
    "a": formatShortWeekday,
    "A": formatWeekday,
    "b": formatShortMonth,
    "B": formatMonth,
    "c": null,
    "d": formatDayOfMonth,
    "e": formatDayOfMonth,
    "f": formatMicroseconds,
    "g": formatYearISO,
    "G": formatFullYearISO,
    "H": formatHour24,
    "I": formatHour12,
    "j": formatDayOfYear,
    "L": formatMilliseconds,
    "m": formatMonthNumber,
    "M": formatMinutes,
    "p": formatPeriod,
    "q": formatQuarter,
    "Q": formatUnixTimestamp,
    "s": formatUnixTimestampSeconds,
    "S": formatSeconds,
    "u": formatWeekdayNumberMonday,
    "U": formatWeekNumberSunday,
    "V": formatWeekNumberISO,
    "w": formatWeekdayNumberSunday,
    "W": formatWeekNumberMonday,
    "x": null,
    "X": null,
    "y": formatYear,
    "Y": formatFullYear,
    "Z": formatZone,
    "%": formatLiteralPercent
  };

  var utcFormats = {
    "a": formatUTCShortWeekday,
    "A": formatUTCWeekday,
    "b": formatUTCShortMonth,
    "B": formatUTCMonth,
    "c": null,
    "d": formatUTCDayOfMonth,
    "e": formatUTCDayOfMonth,
    "f": formatUTCMicroseconds,
    "g": formatUTCYearISO,
    "G": formatUTCFullYearISO,
    "H": formatUTCHour24,
    "I": formatUTCHour12,
    "j": formatUTCDayOfYear,
    "L": formatUTCMilliseconds,
    "m": formatUTCMonthNumber,
    "M": formatUTCMinutes,
    "p": formatUTCPeriod,
    "q": formatUTCQuarter,
    "Q": formatUnixTimestamp,
    "s": formatUnixTimestampSeconds,
    "S": formatUTCSeconds,
    "u": formatUTCWeekdayNumberMonday,
    "U": formatUTCWeekNumberSunday,
    "V": formatUTCWeekNumberISO,
    "w": formatUTCWeekdayNumberSunday,
    "W": formatUTCWeekNumberMonday,
    "x": null,
    "X": null,
    "y": formatUTCYear,
    "Y": formatUTCFullYear,
    "Z": formatUTCZone,
    "%": formatLiteralPercent
  };

  var parses = {
    "a": parseShortWeekday,
    "A": parseWeekday,
    "b": parseShortMonth,
    "B": parseMonth,
    "c": parseLocaleDateTime,
    "d": parseDayOfMonth,
    "e": parseDayOfMonth,
    "f": parseMicroseconds,
    "g": parseYear,
    "G": parseFullYear,
    "H": parseHour24,
    "I": parseHour24,
    "j": parseDayOfYear,
    "L": parseMilliseconds,
    "m": parseMonthNumber,
    "M": parseMinutes,
    "p": parsePeriod,
    "q": parseQuarter,
    "Q": parseUnixTimestamp,
    "s": parseUnixTimestampSeconds,
    "S": parseSeconds,
    "u": parseWeekdayNumberMonday,
    "U": parseWeekNumberSunday,
    "V": parseWeekNumberISO,
    "w": parseWeekdayNumberSunday,
    "W": parseWeekNumberMonday,
    "x": parseLocaleDate,
    "X": parseLocaleTime,
    "y": parseYear,
    "Y": parseFullYear,
    "Z": parseZone,
    "%": parseLiteralPercent
  };

  // These recursive directive definitions must be deferred.
  formats.x = newFormat(locale_date, formats);
  formats.X = newFormat(locale_time, formats);
  formats.c = newFormat(locale_dateTime, formats);
  utcFormats.x = newFormat(locale_date, utcFormats);
  utcFormats.X = newFormat(locale_time, utcFormats);
  utcFormats.c = newFormat(locale_dateTime, utcFormats);

  function newFormat(specifier, formats) {
    return function(date) {
      var string = [],
          i = -1,
          j = 0,
          n = specifier.length,
          c,
          pad,
          format;

      if (!(date instanceof Date)) date = new Date(+date);

      while (++i < n) {
        if (specifier.charCodeAt(i) === 37) {
          string.push(specifier.slice(j, i));
          if ((pad = pads[c = specifier.charAt(++i)]) != null) c = specifier.charAt(++i);
          else pad = c === "e" ? " " : "0";
          if (format = formats[c]) c = format(date, pad);
          string.push(c);
          j = i + 1;
        }
      }

      string.push(specifier.slice(j, i));
      return string.join("");
    };
  }

  function newParse(specifier, Z) {
    return function(string) {
      var d = newDate(1900, undefined, 1),
          i = parseSpecifier(d, specifier, string += "", 0),
          week, day;
      if (i != string.length) return null;

      // If a UNIX timestamp is specified, return it.
      if ("Q" in d) return new Date(d.Q);
      if ("s" in d) return new Date(d.s * 1000 + ("L" in d ? d.L : 0));

      // If this is utcParse, never use the local timezone.
      if (Z && !("Z" in d)) d.Z = 0;

      // The am-pm flag is 0 for AM, and 1 for PM.
      if ("p" in d) d.H = d.H % 12 + d.p * 12;

      // If the month was not specified, inherit from the quarter.
      if (d.m === undefined) d.m = "q" in d ? d.q : 0;

      // Convert day-of-week and week-of-year to day-of-year.
      if ("V" in d) {
        if (d.V < 1 || d.V > 53) return null;
        if (!("w" in d)) d.w = 1;
        if ("Z" in d) {
          week = utcDate(newDate(d.y, 0, 1)), day = week.getUTCDay();
          week = day > 4 || day === 0 ? utcMonday.ceil(week) : utcMonday(week);
          week = utcDay.offset(week, (d.V - 1) * 7);
          d.y = week.getUTCFullYear();
          d.m = week.getUTCMonth();
          d.d = week.getUTCDate() + (d.w + 6) % 7;
        } else {
          week = localDate(newDate(d.y, 0, 1)), day = week.getDay();
          week = day > 4 || day === 0 ? timeMonday.ceil(week) : timeMonday(week);
          week = timeDay.offset(week, (d.V - 1) * 7);
          d.y = week.getFullYear();
          d.m = week.getMonth();
          d.d = week.getDate() + (d.w + 6) % 7;
        }
      } else if ("W" in d || "U" in d) {
        if (!("w" in d)) d.w = "u" in d ? d.u % 7 : "W" in d ? 1 : 0;
        day = "Z" in d ? utcDate(newDate(d.y, 0, 1)).getUTCDay() : localDate(newDate(d.y, 0, 1)).getDay();
        d.m = 0;
        d.d = "W" in d ? (d.w + 6) % 7 + d.W * 7 - (day + 5) % 7 : d.w + d.U * 7 - (day + 6) % 7;
      }

      // If a time zone is specified, all fields are interpreted as UTC and then
      // offset according to the specified time zone.
      if ("Z" in d) {
        d.H += d.Z / 100 | 0;
        d.M += d.Z % 100;
        return utcDate(d);
      }

      // Otherwise, all fields are in local time.
      return localDate(d);
    };
  }

  function parseSpecifier(d, specifier, string, j) {
    var i = 0,
        n = specifier.length,
        m = string.length,
        c,
        parse;

    while (i < n) {
      if (j >= m) return -1;
      c = specifier.charCodeAt(i++);
      if (c === 37) {
        c = specifier.charAt(i++);
        parse = parses[c in pads ? specifier.charAt(i++) : c];
        if (!parse || ((j = parse(d, string, j)) < 0)) return -1;
      } else if (c != string.charCodeAt(j++)) {
        return -1;
      }
    }

    return j;
  }

  function parsePeriod(d, string, i) {
    var n = periodRe.exec(string.slice(i));
    return n ? (d.p = periodLookup.get(n[0].toLowerCase()), i + n[0].length) : -1;
  }

  function parseShortWeekday(d, string, i) {
    var n = shortWeekdayRe.exec(string.slice(i));
    return n ? (d.w = shortWeekdayLookup.get(n[0].toLowerCase()), i + n[0].length) : -1;
  }

  function parseWeekday(d, string, i) {
    var n = weekdayRe.exec(string.slice(i));
    return n ? (d.w = weekdayLookup.get(n[0].toLowerCase()), i + n[0].length) : -1;
  }

  function parseShortMonth(d, string, i) {
    var n = shortMonthRe.exec(string.slice(i));
    return n ? (d.m = shortMonthLookup.get(n[0].toLowerCase()), i + n[0].length) : -1;
  }

  function parseMonth(d, string, i) {
    var n = monthRe.exec(string.slice(i));
    return n ? (d.m = monthLookup.get(n[0].toLowerCase()), i + n[0].length) : -1;
  }

  function parseLocaleDateTime(d, string, i) {
    return parseSpecifier(d, locale_dateTime, string, i);
  }

  function parseLocaleDate(d, string, i) {
    return parseSpecifier(d, locale_date, string, i);
  }

  function parseLocaleTime(d, string, i) {
    return parseSpecifier(d, locale_time, string, i);
  }

  function formatShortWeekday(d) {
    return locale_shortWeekdays[d.getDay()];
  }

  function formatWeekday(d) {
    return locale_weekdays[d.getDay()];
  }

  function formatShortMonth(d) {
    return locale_shortMonths[d.getMonth()];
  }

  function formatMonth(d) {
    return locale_months[d.getMonth()];
  }

  function formatPeriod(d) {
    return locale_periods[+(d.getHours() >= 12)];
  }

  function formatQuarter(d) {
    return 1 + ~~(d.getMonth() / 3);
  }

  function formatUTCShortWeekday(d) {
    return locale_shortWeekdays[d.getUTCDay()];
  }

  function formatUTCWeekday(d) {
    return locale_weekdays[d.getUTCDay()];
  }

  function formatUTCShortMonth(d) {
    return locale_shortMonths[d.getUTCMonth()];
  }

  function formatUTCMonth(d) {
    return locale_months[d.getUTCMonth()];
  }

  function formatUTCPeriod(d) {
    return locale_periods[+(d.getUTCHours() >= 12)];
  }

  function formatUTCQuarter(d) {
    return 1 + ~~(d.getUTCMonth() / 3);
  }

  return {
    format: function(specifier) {
      var f = newFormat(specifier += "", formats);
      f.toString = function() { return specifier; };
      return f;
    },
    parse: function(specifier) {
      var p = newParse(specifier += "", false);
      p.toString = function() { return specifier; };
      return p;
    },
    utcFormat: function(specifier) {
      var f = newFormat(specifier += "", utcFormats);
      f.toString = function() { return specifier; };
      return f;
    },
    utcParse: function(specifier) {
      var p = newParse(specifier += "", true);
      p.toString = function() { return specifier; };
      return p;
    }
  };
}

var pads = {"-": "", "_": " ", "0": "0"},
    numberRe = /^\s*\d+/, // note: ignores next directive
    percentRe = /^%/,
    requoteRe = /[\\^$*+?|[\]().{}]/g;

function pad(value, fill, width) {
  var sign = value < 0 ? "-" : "",
      string = (sign ? -value : value) + "",
      length = string.length;
  return sign + (length < width ? new Array(width - length + 1).join(fill) + string : string);
}

function requote(s) {
  return s.replace(requoteRe, "\\$&");
}

function formatRe(names) {
  return new RegExp("^(?:" + names.map(requote).join("|") + ")", "i");
}

function formatLookup(names) {
  return new Map(names.map((name, i) => [name.toLowerCase(), i]));
}

function parseWeekdayNumberSunday(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 1));
  return n ? (d.w = +n[0], i + n[0].length) : -1;
}

function parseWeekdayNumberMonday(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 1));
  return n ? (d.u = +n[0], i + n[0].length) : -1;
}

function parseWeekNumberSunday(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.U = +n[0], i + n[0].length) : -1;
}

function parseWeekNumberISO(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.V = +n[0], i + n[0].length) : -1;
}

function parseWeekNumberMonday(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.W = +n[0], i + n[0].length) : -1;
}

function parseFullYear(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 4));
  return n ? (d.y = +n[0], i + n[0].length) : -1;
}

function parseYear(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.y = +n[0] + (+n[0] > 68 ? 1900 : 2000), i + n[0].length) : -1;
}

function parseZone(d, string, i) {
  var n = /^(Z)|([+-]\d\d)(?::?(\d\d))?/.exec(string.slice(i, i + 6));
  return n ? (d.Z = n[1] ? 0 : -(n[2] + (n[3] || "00")), i + n[0].length) : -1;
}

function parseQuarter(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 1));
  return n ? (d.q = n[0] * 3 - 3, i + n[0].length) : -1;
}

function parseMonthNumber(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.m = n[0] - 1, i + n[0].length) : -1;
}

function parseDayOfMonth(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.d = +n[0], i + n[0].length) : -1;
}

function parseDayOfYear(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 3));
  return n ? (d.m = 0, d.d = +n[0], i + n[0].length) : -1;
}

function parseHour24(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.H = +n[0], i + n[0].length) : -1;
}

function parseMinutes(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.M = +n[0], i + n[0].length) : -1;
}

function parseSeconds(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 2));
  return n ? (d.S = +n[0], i + n[0].length) : -1;
}

function parseMilliseconds(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 3));
  return n ? (d.L = +n[0], i + n[0].length) : -1;
}

function parseMicroseconds(d, string, i) {
  var n = numberRe.exec(string.slice(i, i + 6));
  return n ? (d.L = Math.floor(n[0] / 1000), i + n[0].length) : -1;
}

function parseLiteralPercent(d, string, i) {
  var n = percentRe.exec(string.slice(i, i + 1));
  return n ? i + n[0].length : -1;
}

function parseUnixTimestamp(d, string, i) {
  var n = numberRe.exec(string.slice(i));
  return n ? (d.Q = +n[0], i + n[0].length) : -1;
}

function parseUnixTimestampSeconds(d, string, i) {
  var n = numberRe.exec(string.slice(i));
  return n ? (d.s = +n[0], i + n[0].length) : -1;
}

function formatDayOfMonth(d, p) {
  return pad(d.getDate(), p, 2);
}

function formatHour24(d, p) {
  return pad(d.getHours(), p, 2);
}

function formatHour12(d, p) {
  return pad(d.getHours() % 12 || 12, p, 2);
}

function formatDayOfYear(d, p) {
  return pad(1 + timeDay.count(timeYear(d), d), p, 3);
}

function formatMilliseconds(d, p) {
  return pad(d.getMilliseconds(), p, 3);
}

function formatMicroseconds(d, p) {
  return formatMilliseconds(d, p) + "000";
}

function formatMonthNumber(d, p) {
  return pad(d.getMonth() + 1, p, 2);
}

function formatMinutes(d, p) {
  return pad(d.getMinutes(), p, 2);
}

function formatSeconds(d, p) {
  return pad(d.getSeconds(), p, 2);
}

function formatWeekdayNumberMonday(d) {
  var day = d.getDay();
  return day === 0 ? 7 : day;
}

function formatWeekNumberSunday(d, p) {
  return pad(timeSunday.count(timeYear(d) - 1, d), p, 2);
}

function dISO(d) {
  var day = d.getDay();
  return (day >= 4 || day === 0) ? timeThursday(d) : timeThursday.ceil(d);
}

function formatWeekNumberISO(d, p) {
  d = dISO(d);
  return pad(timeThursday.count(timeYear(d), d) + (timeYear(d).getDay() === 4), p, 2);
}

function formatWeekdayNumberSunday(d) {
  return d.getDay();
}

function formatWeekNumberMonday(d, p) {
  return pad(timeMonday.count(timeYear(d) - 1, d), p, 2);
}

function formatYear(d, p) {
  return pad(d.getFullYear() % 100, p, 2);
}

function formatYearISO(d, p) {
  d = dISO(d);
  return pad(d.getFullYear() % 100, p, 2);
}

function formatFullYear(d, p) {
  return pad(d.getFullYear() % 10000, p, 4);
}

function formatFullYearISO(d, p) {
  var day = d.getDay();
  d = (day >= 4 || day === 0) ? timeThursday(d) : timeThursday.ceil(d);
  return pad(d.getFullYear() % 10000, p, 4);
}

function formatZone(d) {
  var z = d.getTimezoneOffset();
  return (z > 0 ? "-" : (z *= -1, "+"))
      + pad(z / 60 | 0, "0", 2)
      + pad(z % 60, "0", 2);
}

function formatUTCDayOfMonth(d, p) {
  return pad(d.getUTCDate(), p, 2);
}

function formatUTCHour24(d, p) {
  return pad(d.getUTCHours(), p, 2);
}

function formatUTCHour12(d, p) {
  return pad(d.getUTCHours() % 12 || 12, p, 2);
}

function formatUTCDayOfYear(d, p) {
  return pad(1 + utcDay.count(utcYear(d), d), p, 3);
}

function formatUTCMilliseconds(d, p) {
  return pad(d.getUTCMilliseconds(), p, 3);
}

function formatUTCMicroseconds(d, p) {
  return formatUTCMilliseconds(d, p) + "000";
}

function formatUTCMonthNumber(d, p) {
  return pad(d.getUTCMonth() + 1, p, 2);
}

function formatUTCMinutes(d, p) {
  return pad(d.getUTCMinutes(), p, 2);
}

function formatUTCSeconds(d, p) {
  return pad(d.getUTCSeconds(), p, 2);
}

function formatUTCWeekdayNumberMonday(d) {
  var dow = d.getUTCDay();
  return dow === 0 ? 7 : dow;
}

function formatUTCWeekNumberSunday(d, p) {
  return pad(utcSunday.count(utcYear(d) - 1, d), p, 2);
}

function UTCdISO(d) {
  var day = d.getUTCDay();
  return (day >= 4 || day === 0) ? utcThursday(d) : utcThursday.ceil(d);
}

function formatUTCWeekNumberISO(d, p) {
  d = UTCdISO(d);
  return pad(utcThursday.count(utcYear(d), d) + (utcYear(d).getUTCDay() === 4), p, 2);
}

function formatUTCWeekdayNumberSunday(d) {
  return d.getUTCDay();
}

function formatUTCWeekNumberMonday(d, p) {
  return pad(utcMonday.count(utcYear(d) - 1, d), p, 2);
}

function formatUTCYear(d, p) {
  return pad(d.getUTCFullYear() % 100, p, 2);
}

function formatUTCYearISO(d, p) {
  d = UTCdISO(d);
  return pad(d.getUTCFullYear() % 100, p, 2);
}

function formatUTCFullYear(d, p) {
  return pad(d.getUTCFullYear() % 10000, p, 4);
}

function formatUTCFullYearISO(d, p) {
  var day = d.getUTCDay();
  d = (day >= 4 || day === 0) ? utcThursday(d) : utcThursday.ceil(d);
  return pad(d.getUTCFullYear() % 10000, p, 4);
}

function formatUTCZone() {
  return "+0000";
}

function formatLiteralPercent() {
  return "%";
}

function formatUnixTimestamp(d) {
  return +d;
}

function formatUnixTimestampSeconds(d) {
  return Math.floor(+d / 1000);
}

var locale;
var timeFormat;
var timeParse;
var utcFormat;
var utcParse;

defaultLocale({
  dateTime: "%x, %X",
  date: "%-m/%-d/%Y",
  time: "%-I:%M:%S %p",
  periods: ["AM", "PM"],
  days: ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"],
  shortDays: ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"],
  months: ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"],
  shortMonths: ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"]
});

function defaultLocale(definition) {
  locale = formatLocale(definition);
  timeFormat = locale.format;
  timeParse = locale.parse;
  utcFormat = locale.utcFormat;
  utcParse = locale.utcParse;
  return locale;
}

var isoSpecifier = "%Y-%m-%dT%H:%M:%S.%LZ";

function formatIsoNative(date) {
  return date.toISOString();
}

var formatIso = Date.prototype.toISOString
    ? formatIsoNative
    : utcFormat(isoSpecifier);

function parseIsoNative(string) {
  var date = new Date(string);
  return isNaN(date) ? null : date;
}

var parseIso = +new Date("2000-01-01T00:00:00.000Z")
    ? parseIsoNative
    : utcParse(isoSpecifier);

function date(t) {
  return new Date(t);
}

function number(t) {
  return t instanceof Date ? +t : +new Date(+t);
}

function calendar(ticks, tickInterval, year, month, week, day, hour, minute, second, format) {
  var scale = continuous(),
      invert = scale.invert,
      domain = scale.domain;

  var formatMillisecond = format(".%L"),
      formatSecond = format(":%S"),
      formatMinute = format("%I:%M"),
      formatHour = format("%I %p"),
      formatDay = format("%a %d"),
      formatWeek = format("%b %d"),
      formatMonth = format("%B"),
      formatYear = format("%Y");

  function tickFormat(date) {
    return (second(date) < date ? formatMillisecond
        : minute(date) < date ? formatSecond
        : hour(date) < date ? formatMinute
        : day(date) < date ? formatHour
        : month(date) < date ? (week(date) < date ? formatDay : formatWeek)
        : year(date) < date ? formatMonth
        : formatYear)(date);
  }

  scale.invert = function(y) {
    return new Date(invert(y));
  };

  scale.domain = function(_) {
    return arguments.length ? domain(Array.from(_, number)) : domain().map(date);
  };

  scale.ticks = function(interval) {
    var d = domain();
    return ticks(d[0], d[d.length - 1], interval == null ? 10 : interval);
  };

  scale.tickFormat = function(count, specifier) {
    return specifier == null ? tickFormat : format(specifier);
  };

  scale.nice = function(interval) {
    var d = domain();
    if (!interval || typeof interval.range !== "function") interval = tickInterval(d[0], d[d.length - 1], interval == null ? 10 : interval);
    return interval ? domain(nice(d, interval)) : scale;
  };

  scale.copy = function() {
    return copy$1(scale, calendar(ticks, tickInterval, year, month, week, day, hour, minute, second, format));
  };

  return scale;
}

function time() {
  return initRange.apply(calendar(timeTicks, timeTickInterval, timeYear, timeMonth, timeSunday, timeDay, timeHour, timeMinute, second, timeFormat).domain([new Date(2000, 0, 1), new Date(2000, 0, 2)]), arguments);
}

function utcTime() {
  return initRange.apply(calendar(utcTicks, utcTickInterval, utcYear, utcMonth, utcSunday, utcDay, utcHour, utcMinute, second, utcFormat).domain([Date.UTC(2000, 0, 1), Date.UTC(2000, 0, 2)]), arguments);
}

function transformer$1() {
  var x0 = 0,
      x1 = 1,
      t0,
      t1,
      k10,
      transform,
      interpolator = identity$3,
      clamp = false,
      unknown;

  function scale(x) {
    return x == null || isNaN(x = +x) ? unknown : interpolator(k10 === 0 ? 0.5 : (x = (transform(x) - t0) * k10, clamp ? Math.max(0, Math.min(1, x)) : x));
  }

  scale.domain = function(_) {
    return arguments.length ? ([x0, x1] = _, t0 = transform(x0 = +x0), t1 = transform(x1 = +x1), k10 = t0 === t1 ? 0 : 1 / (t1 - t0), scale) : [x0, x1];
  };

  scale.clamp = function(_) {
    return arguments.length ? (clamp = !!_, scale) : clamp;
  };

  scale.interpolator = function(_) {
    return arguments.length ? (interpolator = _, scale) : interpolator;
  };

  function range(interpolate) {
    return function(_) {
      var r0, r1;
      return arguments.length ? ([r0, r1] = _, interpolator = interpolate(r0, r1), scale) : [interpolator(0), interpolator(1)];
    };
  }

  scale.range = range(interpolate$2);

  scale.rangeRound = range(interpolateRound);

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  return function(t) {
    transform = t, t0 = t(x0), t1 = t(x1), k10 = t0 === t1 ? 0 : 1 / (t1 - t0);
    return scale;
  };
}

function copy(source, target) {
  return target
      .domain(source.domain())
      .interpolator(source.interpolator())
      .clamp(source.clamp())
      .unknown(source.unknown());
}

function sequential() {
  var scale = linearish(transformer$1()(identity$3));

  scale.copy = function() {
    return copy(scale, sequential());
  };

  return initInterpolator.apply(scale, arguments);
}

function sequentialLog() {
  var scale = loggish(transformer$1()).domain([1, 10]);

  scale.copy = function() {
    return copy(scale, sequentialLog()).base(scale.base());
  };

  return initInterpolator.apply(scale, arguments);
}

function sequentialSymlog() {
  var scale = symlogish(transformer$1());

  scale.copy = function() {
    return copy(scale, sequentialSymlog()).constant(scale.constant());
  };

  return initInterpolator.apply(scale, arguments);
}

function sequentialPow() {
  var scale = powish(transformer$1());

  scale.copy = function() {
    return copy(scale, sequentialPow()).exponent(scale.exponent());
  };

  return initInterpolator.apply(scale, arguments);
}

function sequentialSqrt() {
  return sequentialPow.apply(null, arguments).exponent(0.5);
}

function sequentialQuantile() {
  var domain = [],
      interpolator = identity$3;

  function scale(x) {
    if (x != null && !isNaN(x = +x)) return interpolator((bisectRight(domain, x, 1) - 1) / (domain.length - 1));
  }

  scale.domain = function(_) {
    if (!arguments.length) return domain.slice();
    domain = [];
    for (let d of _) if (d != null && !isNaN(d = +d)) domain.push(d);
    domain.sort(ascending$3);
    return scale;
  };

  scale.interpolator = function(_) {
    return arguments.length ? (interpolator = _, scale) : interpolator;
  };

  scale.range = function() {
    return domain.map((d, i) => interpolator(i / (domain.length - 1)));
  };

  scale.quantiles = function(n) {
    return Array.from({length: n + 1}, (_, i) => quantile$1(domain, i / n));
  };

  scale.copy = function() {
    return sequentialQuantile(interpolator).domain(domain);
  };

  return initInterpolator.apply(scale, arguments);
}

function transformer() {
  var x0 = 0,
      x1 = 0.5,
      x2 = 1,
      s = 1,
      t0,
      t1,
      t2,
      k10,
      k21,
      interpolator = identity$3,
      transform,
      clamp = false,
      unknown;

  function scale(x) {
    return isNaN(x = +x) ? unknown : (x = 0.5 + ((x = +transform(x)) - t1) * (s * x < s * t1 ? k10 : k21), interpolator(clamp ? Math.max(0, Math.min(1, x)) : x));
  }

  scale.domain = function(_) {
    return arguments.length ? ([x0, x1, x2] = _, t0 = transform(x0 = +x0), t1 = transform(x1 = +x1), t2 = transform(x2 = +x2), k10 = t0 === t1 ? 0 : 0.5 / (t1 - t0), k21 = t1 === t2 ? 0 : 0.5 / (t2 - t1), s = t1 < t0 ? -1 : 1, scale) : [x0, x1, x2];
  };

  scale.clamp = function(_) {
    return arguments.length ? (clamp = !!_, scale) : clamp;
  };

  scale.interpolator = function(_) {
    return arguments.length ? (interpolator = _, scale) : interpolator;
  };

  function range(interpolate) {
    return function(_) {
      var r0, r1, r2;
      return arguments.length ? ([r0, r1, r2] = _, interpolator = piecewise(interpolate, [r0, r1, r2]), scale) : [interpolator(0), interpolator(0.5), interpolator(1)];
    };
  }

  scale.range = range(interpolate$2);

  scale.rangeRound = range(interpolateRound);

  scale.unknown = function(_) {
    return arguments.length ? (unknown = _, scale) : unknown;
  };

  return function(t) {
    transform = t, t0 = t(x0), t1 = t(x1), t2 = t(x2), k10 = t0 === t1 ? 0 : 0.5 / (t1 - t0), k21 = t1 === t2 ? 0 : 0.5 / (t2 - t1), s = t1 < t0 ? -1 : 1;
    return scale;
  };
}

function diverging$1() {
  var scale = linearish(transformer()(identity$3));

  scale.copy = function() {
    return copy(scale, diverging$1());
  };

  return initInterpolator.apply(scale, arguments);
}

function divergingLog() {
  var scale = loggish(transformer()).domain([0.1, 1, 10]);

  scale.copy = function() {
    return copy(scale, divergingLog()).base(scale.base());
  };

  return initInterpolator.apply(scale, arguments);
}

function divergingSymlog() {
  var scale = symlogish(transformer());

  scale.copy = function() {
    return copy(scale, divergingSymlog()).constant(scale.constant());
  };

  return initInterpolator.apply(scale, arguments);
}

function divergingPow() {
  var scale = powish(transformer());

  scale.copy = function() {
    return copy(scale, divergingPow()).exponent(scale.exponent());
  };

  return initInterpolator.apply(scale, arguments);
}

function divergingSqrt() {
  return divergingPow.apply(null, arguments).exponent(0.5);
}

function colors(specifier) {
  var n = specifier.length / 6 | 0, colors = new Array(n), i = 0;
  while (i < n) colors[i] = "#" + specifier.slice(i * 6, ++i * 6);
  return colors;
}

const category10 = colors("1f77b4ff7f0e2ca02cd627289467bd8c564be377c27f7f7fbcbd2217becf");

const Accent = colors("7fc97fbeaed4fdc086ffff99386cb0f0027fbf5b17666666");

const Dark2 = colors("1b9e77d95f027570b3e7298a66a61ee6ab02a6761d666666");

const observable10 = colors("4269d0efb118ff725c6cc5b03ca951ff8ab7a463f297bbf59c6b4e9498a0");

const Paired = colors("a6cee31f78b4b2df8a33a02cfb9a99e31a1cfdbf6fff7f00cab2d66a3d9affff99b15928");

const Pastel1 = colors("fbb4aeb3cde3ccebc5decbe4fed9a6ffffcce5d8bdfddaecf2f2f2");

const Pastel2 = colors("b3e2cdfdcdaccbd5e8f4cae4e6f5c9fff2aef1e2cccccccc");

const Set1 = colors("e41a1c377eb84daf4a984ea3ff7f00ffff33a65628f781bf999999");

const Set2 = colors("66c2a5fc8d628da0cbe78ac3a6d854ffd92fe5c494b3b3b3");

const Set3 = colors("8dd3c7ffffb3bebadafb807280b1d3fdb462b3de69fccde5d9d9d9bc80bdccebc5ffed6f");

const Tableau10 = colors("4e79a7f28e2ce1575976b7b259a14fedc949af7aa1ff9da79c755fbab0ab");

const ramp$1 = scheme => rgbBasis(scheme[scheme.length - 1]);

var scheme$q = new Array(3).concat(
  "d8b365f5f5f55ab4ac",
  "a6611adfc27d80cdc1018571",
  "a6611adfc27df5f5f580cdc1018571",
  "8c510ad8b365f6e8c3c7eae55ab4ac01665e",
  "8c510ad8b365f6e8c3f5f5f5c7eae55ab4ac01665e",
  "8c510abf812ddfc27df6e8c3c7eae580cdc135978f01665e",
  "8c510abf812ddfc27df6e8c3f5f5f5c7eae580cdc135978f01665e",
  "5430058c510abf812ddfc27df6e8c3c7eae580cdc135978f01665e003c30",
  "5430058c510abf812ddfc27df6e8c3f5f5f5c7eae580cdc135978f01665e003c30"
).map(colors);

const BrBG = ramp$1(scheme$q);

var scheme$p = new Array(3).concat(
  "af8dc3f7f7f77fbf7b",
  "7b3294c2a5cfa6dba0008837",
  "7b3294c2a5cff7f7f7a6dba0008837",
  "762a83af8dc3e7d4e8d9f0d37fbf7b1b7837",
  "762a83af8dc3e7d4e8f7f7f7d9f0d37fbf7b1b7837",
  "762a839970abc2a5cfe7d4e8d9f0d3a6dba05aae611b7837",
  "762a839970abc2a5cfe7d4e8f7f7f7d9f0d3a6dba05aae611b7837",
  "40004b762a839970abc2a5cfe7d4e8d9f0d3a6dba05aae611b783700441b",
  "40004b762a839970abc2a5cfe7d4e8f7f7f7d9f0d3a6dba05aae611b783700441b"
).map(colors);

const PRGn = ramp$1(scheme$p);

var scheme$o = new Array(3).concat(
  "e9a3c9f7f7f7a1d76a",
  "d01c8bf1b6dab8e1864dac26",
  "d01c8bf1b6daf7f7f7b8e1864dac26",
  "c51b7de9a3c9fde0efe6f5d0a1d76a4d9221",
  "c51b7de9a3c9fde0eff7f7f7e6f5d0a1d76a4d9221",
  "c51b7dde77aef1b6dafde0efe6f5d0b8e1867fbc414d9221",
  "c51b7dde77aef1b6dafde0eff7f7f7e6f5d0b8e1867fbc414d9221",
  "8e0152c51b7dde77aef1b6dafde0efe6f5d0b8e1867fbc414d9221276419",
  "8e0152c51b7dde77aef1b6dafde0eff7f7f7e6f5d0b8e1867fbc414d9221276419"
).map(colors);

const PiYG = ramp$1(scheme$o);

var scheme$n = new Array(3).concat(
  "998ec3f7f7f7f1a340",
  "5e3c99b2abd2fdb863e66101",
  "5e3c99b2abd2f7f7f7fdb863e66101",
  "542788998ec3d8daebfee0b6f1a340b35806",
  "542788998ec3d8daebf7f7f7fee0b6f1a340b35806",
  "5427888073acb2abd2d8daebfee0b6fdb863e08214b35806",
  "5427888073acb2abd2d8daebf7f7f7fee0b6fdb863e08214b35806",
  "2d004b5427888073acb2abd2d8daebfee0b6fdb863e08214b358067f3b08",
  "2d004b5427888073acb2abd2d8daebf7f7f7fee0b6fdb863e08214b358067f3b08"
).map(colors);

const PuOr = ramp$1(scheme$n);

var scheme$m = new Array(3).concat(
  "ef8a62f7f7f767a9cf",
  "ca0020f4a58292c5de0571b0",
  "ca0020f4a582f7f7f792c5de0571b0",
  "b2182bef8a62fddbc7d1e5f067a9cf2166ac",
  "b2182bef8a62fddbc7f7f7f7d1e5f067a9cf2166ac",
  "b2182bd6604df4a582fddbc7d1e5f092c5de4393c32166ac",
  "b2182bd6604df4a582fddbc7f7f7f7d1e5f092c5de4393c32166ac",
  "67001fb2182bd6604df4a582fddbc7d1e5f092c5de4393c32166ac053061",
  "67001fb2182bd6604df4a582fddbc7f7f7f7d1e5f092c5de4393c32166ac053061"
).map(colors);

const RdBu = ramp$1(scheme$m);

var scheme$l = new Array(3).concat(
  "ef8a62ffffff999999",
  "ca0020f4a582bababa404040",
  "ca0020f4a582ffffffbababa404040",
  "b2182bef8a62fddbc7e0e0e09999994d4d4d",
  "b2182bef8a62fddbc7ffffffe0e0e09999994d4d4d",
  "b2182bd6604df4a582fddbc7e0e0e0bababa8787874d4d4d",
  "b2182bd6604df4a582fddbc7ffffffe0e0e0bababa8787874d4d4d",
  "67001fb2182bd6604df4a582fddbc7e0e0e0bababa8787874d4d4d1a1a1a",
  "67001fb2182bd6604df4a582fddbc7ffffffe0e0e0bababa8787874d4d4d1a1a1a"
).map(colors);

const RdGy = ramp$1(scheme$l);

var scheme$k = new Array(3).concat(
  "fc8d59ffffbf91bfdb",
  "d7191cfdae61abd9e92c7bb6",
  "d7191cfdae61ffffbfabd9e92c7bb6",
  "d73027fc8d59fee090e0f3f891bfdb4575b4",
  "d73027fc8d59fee090ffffbfe0f3f891bfdb4575b4",
  "d73027f46d43fdae61fee090e0f3f8abd9e974add14575b4",
  "d73027f46d43fdae61fee090ffffbfe0f3f8abd9e974add14575b4",
  "a50026d73027f46d43fdae61fee090e0f3f8abd9e974add14575b4313695",
  "a50026d73027f46d43fdae61fee090ffffbfe0f3f8abd9e974add14575b4313695"
).map(colors);

const RdYlBu = ramp$1(scheme$k);

var scheme$j = new Array(3).concat(
  "fc8d59ffffbf91cf60",
  "d7191cfdae61a6d96a1a9641",
  "d7191cfdae61ffffbfa6d96a1a9641",
  "d73027fc8d59fee08bd9ef8b91cf601a9850",
  "d73027fc8d59fee08bffffbfd9ef8b91cf601a9850",
  "d73027f46d43fdae61fee08bd9ef8ba6d96a66bd631a9850",
  "d73027f46d43fdae61fee08bffffbfd9ef8ba6d96a66bd631a9850",
  "a50026d73027f46d43fdae61fee08bd9ef8ba6d96a66bd631a9850006837",
  "a50026d73027f46d43fdae61fee08bffffbfd9ef8ba6d96a66bd631a9850006837"
).map(colors);

const RdYlGn = ramp$1(scheme$j);

var scheme$i = new Array(3).concat(
  "fc8d59ffffbf99d594",
  "d7191cfdae61abdda42b83ba",
  "d7191cfdae61ffffbfabdda42b83ba",
  "d53e4ffc8d59fee08be6f59899d5943288bd",
  "d53e4ffc8d59fee08bffffbfe6f59899d5943288bd",
  "d53e4ff46d43fdae61fee08be6f598abdda466c2a53288bd",
  "d53e4ff46d43fdae61fee08bffffbfe6f598abdda466c2a53288bd",
  "9e0142d53e4ff46d43fdae61fee08be6f598abdda466c2a53288bd5e4fa2",
  "9e0142d53e4ff46d43fdae61fee08bffffbfe6f598abdda466c2a53288bd5e4fa2"
).map(colors);

const Spectral = ramp$1(scheme$i);

var scheme$h = new Array(3).concat(
  "e5f5f999d8c92ca25f",
  "edf8fbb2e2e266c2a4238b45",
  "edf8fbb2e2e266c2a42ca25f006d2c",
  "edf8fbccece699d8c966c2a42ca25f006d2c",
  "edf8fbccece699d8c966c2a441ae76238b45005824",
  "f7fcfde5f5f9ccece699d8c966c2a441ae76238b45005824",
  "f7fcfde5f5f9ccece699d8c966c2a441ae76238b45006d2c00441b"
).map(colors);

const BuGn = ramp$1(scheme$h);

var scheme$g = new Array(3).concat(
  "e0ecf49ebcda8856a7",
  "edf8fbb3cde38c96c688419d",
  "edf8fbb3cde38c96c68856a7810f7c",
  "edf8fbbfd3e69ebcda8c96c68856a7810f7c",
  "edf8fbbfd3e69ebcda8c96c68c6bb188419d6e016b",
  "f7fcfde0ecf4bfd3e69ebcda8c96c68c6bb188419d6e016b",
  "f7fcfde0ecf4bfd3e69ebcda8c96c68c6bb188419d810f7c4d004b"
).map(colors);

const BuPu = ramp$1(scheme$g);

var scheme$f = new Array(3).concat(
  "e0f3dba8ddb543a2ca",
  "f0f9e8bae4bc7bccc42b8cbe",
  "f0f9e8bae4bc7bccc443a2ca0868ac",
  "f0f9e8ccebc5a8ddb57bccc443a2ca0868ac",
  "f0f9e8ccebc5a8ddb57bccc44eb3d32b8cbe08589e",
  "f7fcf0e0f3dbccebc5a8ddb57bccc44eb3d32b8cbe08589e",
  "f7fcf0e0f3dbccebc5a8ddb57bccc44eb3d32b8cbe0868ac084081"
).map(colors);

const GnBu = ramp$1(scheme$f);

var scheme$e = new Array(3).concat(
  "fee8c8fdbb84e34a33",
  "fef0d9fdcc8afc8d59d7301f",
  "fef0d9fdcc8afc8d59e34a33b30000",
  "fef0d9fdd49efdbb84fc8d59e34a33b30000",
  "fef0d9fdd49efdbb84fc8d59ef6548d7301f990000",
  "fff7ecfee8c8fdd49efdbb84fc8d59ef6548d7301f990000",
  "fff7ecfee8c8fdd49efdbb84fc8d59ef6548d7301fb300007f0000"
).map(colors);

const OrRd = ramp$1(scheme$e);

var scheme$d = new Array(3).concat(
  "ece2f0a6bddb1c9099",
  "f6eff7bdc9e167a9cf02818a",
  "f6eff7bdc9e167a9cf1c9099016c59",
  "f6eff7d0d1e6a6bddb67a9cf1c9099016c59",
  "f6eff7d0d1e6a6bddb67a9cf3690c002818a016450",
  "fff7fbece2f0d0d1e6a6bddb67a9cf3690c002818a016450",
  "fff7fbece2f0d0d1e6a6bddb67a9cf3690c002818a016c59014636"
).map(colors);

const PuBuGn = ramp$1(scheme$d);

var scheme$c = new Array(3).concat(
  "ece7f2a6bddb2b8cbe",
  "f1eef6bdc9e174a9cf0570b0",
  "f1eef6bdc9e174a9cf2b8cbe045a8d",
  "f1eef6d0d1e6a6bddb74a9cf2b8cbe045a8d",
  "f1eef6d0d1e6a6bddb74a9cf3690c00570b0034e7b",
  "fff7fbece7f2d0d1e6a6bddb74a9cf3690c00570b0034e7b",
  "fff7fbece7f2d0d1e6a6bddb74a9cf3690c00570b0045a8d023858"
).map(colors);

const PuBu = ramp$1(scheme$c);

var scheme$b = new Array(3).concat(
  "e7e1efc994c7dd1c77",
  "f1eef6d7b5d8df65b0ce1256",
  "f1eef6d7b5d8df65b0dd1c77980043",
  "f1eef6d4b9dac994c7df65b0dd1c77980043",
  "f1eef6d4b9dac994c7df65b0e7298ace125691003f",
  "f7f4f9e7e1efd4b9dac994c7df65b0e7298ace125691003f",
  "f7f4f9e7e1efd4b9dac994c7df65b0e7298ace125698004367001f"
).map(colors);

const PuRd = ramp$1(scheme$b);

var scheme$a = new Array(3).concat(
  "fde0ddfa9fb5c51b8a",
  "feebe2fbb4b9f768a1ae017e",
  "feebe2fbb4b9f768a1c51b8a7a0177",
  "feebe2fcc5c0fa9fb5f768a1c51b8a7a0177",
  "feebe2fcc5c0fa9fb5f768a1dd3497ae017e7a0177",
  "fff7f3fde0ddfcc5c0fa9fb5f768a1dd3497ae017e7a0177",
  "fff7f3fde0ddfcc5c0fa9fb5f768a1dd3497ae017e7a017749006a"
).map(colors);

const RdPu = ramp$1(scheme$a);

var scheme$9 = new Array(3).concat(
  "edf8b17fcdbb2c7fb8",
  "ffffcca1dab441b6c4225ea8",
  "ffffcca1dab441b6c42c7fb8253494",
  "ffffccc7e9b47fcdbb41b6c42c7fb8253494",
  "ffffccc7e9b47fcdbb41b6c41d91c0225ea80c2c84",
  "ffffd9edf8b1c7e9b47fcdbb41b6c41d91c0225ea80c2c84",
  "ffffd9edf8b1c7e9b47fcdbb41b6c41d91c0225ea8253494081d58"
).map(colors);

const YlGnBu = ramp$1(scheme$9);

var scheme$8 = new Array(3).concat(
  "f7fcb9addd8e31a354",
  "ffffccc2e69978c679238443",
  "ffffccc2e69978c67931a354006837",
  "ffffccd9f0a3addd8e78c67931a354006837",
  "ffffccd9f0a3addd8e78c67941ab5d238443005a32",
  "ffffe5f7fcb9d9f0a3addd8e78c67941ab5d238443005a32",
  "ffffe5f7fcb9d9f0a3addd8e78c67941ab5d238443006837004529"
).map(colors);

const YlGn = ramp$1(scheme$8);

var scheme$7 = new Array(3).concat(
  "fff7bcfec44fd95f0e",
  "ffffd4fed98efe9929cc4c02",
  "ffffd4fed98efe9929d95f0e993404",
  "ffffd4fee391fec44ffe9929d95f0e993404",
  "ffffd4fee391fec44ffe9929ec7014cc4c028c2d04",
  "ffffe5fff7bcfee391fec44ffe9929ec7014cc4c028c2d04",
  "ffffe5fff7bcfee391fec44ffe9929ec7014cc4c02993404662506"
).map(colors);

const YlOrBr = ramp$1(scheme$7);

var scheme$6 = new Array(3).concat(
  "ffeda0feb24cf03b20",
  "ffffb2fecc5cfd8d3ce31a1c",
  "ffffb2fecc5cfd8d3cf03b20bd0026",
  "ffffb2fed976feb24cfd8d3cf03b20bd0026",
  "ffffb2fed976feb24cfd8d3cfc4e2ae31a1cb10026",
  "ffffccffeda0fed976feb24cfd8d3cfc4e2ae31a1cb10026",
  "ffffccffeda0fed976feb24cfd8d3cfc4e2ae31a1cbd0026800026"
).map(colors);

const YlOrRd = ramp$1(scheme$6);

var scheme$5 = new Array(3).concat(
  "deebf79ecae13182bd",
  "eff3ffbdd7e76baed62171b5",
  "eff3ffbdd7e76baed63182bd08519c",
  "eff3ffc6dbef9ecae16baed63182bd08519c",
  "eff3ffc6dbef9ecae16baed64292c62171b5084594",
  "f7fbffdeebf7c6dbef9ecae16baed64292c62171b5084594",
  "f7fbffdeebf7c6dbef9ecae16baed64292c62171b508519c08306b"
).map(colors);

const Blues = ramp$1(scheme$5);

var scheme$4 = new Array(3).concat(
  "e5f5e0a1d99b31a354",
  "edf8e9bae4b374c476238b45",
  "edf8e9bae4b374c47631a354006d2c",
  "edf8e9c7e9c0a1d99b74c47631a354006d2c",
  "edf8e9c7e9c0a1d99b74c47641ab5d238b45005a32",
  "f7fcf5e5f5e0c7e9c0a1d99b74c47641ab5d238b45005a32",
  "f7fcf5e5f5e0c7e9c0a1d99b74c47641ab5d238b45006d2c00441b"
).map(colors);

const Greens = ramp$1(scheme$4);

var scheme$3 = new Array(3).concat(
  "f0f0f0bdbdbd636363",
  "f7f7f7cccccc969696525252",
  "f7f7f7cccccc969696636363252525",
  "f7f7f7d9d9d9bdbdbd969696636363252525",
  "f7f7f7d9d9d9bdbdbd969696737373525252252525",
  "fffffff0f0f0d9d9d9bdbdbd969696737373525252252525",
  "fffffff0f0f0d9d9d9bdbdbd969696737373525252252525000000"
).map(colors);

const Greys = ramp$1(scheme$3);

var scheme$2 = new Array(3).concat(
  "efedf5bcbddc756bb1",
  "f2f0f7cbc9e29e9ac86a51a3",
  "f2f0f7cbc9e29e9ac8756bb154278f",
  "f2f0f7dadaebbcbddc9e9ac8756bb154278f",
  "f2f0f7dadaebbcbddc9e9ac8807dba6a51a34a1486",
  "fcfbfdefedf5dadaebbcbddc9e9ac8807dba6a51a34a1486",
  "fcfbfdefedf5dadaebbcbddc9e9ac8807dba6a51a354278f3f007d"
).map(colors);

const Purples = ramp$1(scheme$2);

var scheme$1 = new Array(3).concat(
  "fee0d2fc9272de2d26",
  "fee5d9fcae91fb6a4acb181d",
  "fee5d9fcae91fb6a4ade2d26a50f15",
  "fee5d9fcbba1fc9272fb6a4ade2d26a50f15",
  "fee5d9fcbba1fc9272fb6a4aef3b2ccb181d99000d",
  "fff5f0fee0d2fcbba1fc9272fb6a4aef3b2ccb181d99000d",
  "fff5f0fee0d2fcbba1fc9272fb6a4aef3b2ccb181da50f1567000d"
).map(colors);

const Reds = ramp$1(scheme$1);

var scheme = new Array(3).concat(
  "fee6cefdae6be6550d",
  "feeddefdbe85fd8d3cd94701",
  "feeddefdbe85fd8d3ce6550da63603",
  "feeddefdd0a2fdae6bfd8d3ce6550da63603",
  "feeddefdd0a2fdae6bfd8d3cf16913d948018c2d04",
  "fff5ebfee6cefdd0a2fdae6bfd8d3cf16913d948018c2d04",
  "fff5ebfee6cefdd0a2fdae6bfd8d3cf16913d94801a636037f2704"
).map(colors);

const Oranges = ramp$1(scheme);

function cividis(t) {
  t = Math.max(0, Math.min(1, t));
  return "rgb("
      + Math.max(0, Math.min(255, Math.round(-4.54 - t * (35.34 - t * (2381.73 - t * (6402.7 - t * (7024.72 - t * 2710.57))))))) + ", "
      + Math.max(0, Math.min(255, Math.round(32.49 + t * (170.73 + t * (52.82 - t * (131.46 - t * (176.58 - t * 67.37))))))) + ", "
      + Math.max(0, Math.min(255, Math.round(81.24 + t * (442.36 - t * (2482.43 - t * (6167.24 - t * (6614.94 - t * 2475.67)))))))
      + ")";
}

const cubehelix = cubehelixLong(cubehelix$2(300, 0.5, 0.0), cubehelix$2(-240, 0.5, 1.0));

var warm = cubehelixLong(cubehelix$2(-100, 0.75, 0.35), cubehelix$2(80, 1.50, 0.8));

var cool = cubehelixLong(cubehelix$2(260, 0.75, 0.35), cubehelix$2(80, 1.50, 0.8));

var c$2 = cubehelix$2();

function rainbow(t) {
  if (t < 0 || t > 1) t -= Math.floor(t);
  var ts = Math.abs(t - 0.5);
  c$2.h = 360 * t - 100;
  c$2.s = 1.5 - 1.5 * ts;
  c$2.l = 0.8 - 0.9 * ts;
  return c$2 + "";
}

var c$1 = rgb(),
    pi_1_3 = Math.PI / 3,
    pi_2_3 = Math.PI * 2 / 3;

function sinebow(t) {
  var x;
  t = (0.5 - t) * Math.PI;
  c$1.r = 255 * (x = Math.sin(t)) * x;
  c$1.g = 255 * (x = Math.sin(t + pi_1_3)) * x;
  c$1.b = 255 * (x = Math.sin(t + pi_2_3)) * x;
  return c$1 + "";
}

function turbo(t) {
  t = Math.max(0, Math.min(1, t));
  return "rgb("
      + Math.max(0, Math.min(255, Math.round(34.61 + t * (1172.33 - t * (10793.56 - t * (33300.12 - t * (38394.49 - t * 14825.05))))))) + ", "
      + Math.max(0, Math.min(255, Math.round(23.31 + t * (557.33 + t * (1225.33 - t * (3574.96 - t * (1073.77 + t * 707.56))))))) + ", "
      + Math.max(0, Math.min(255, Math.round(27.2 + t * (3211.1 - t * (15327.97 - t * (27814 - t * (22569.18 - t * 6838.66)))))))
      + ")";
}

function ramp(range) {
  var n = range.length;
  return function(t) {
    return range[Math.max(0, Math.min(n - 1, Math.floor(t * n)))];
  };
}

const viridis = ramp(colors("44015444025645045745055946075a46085c460a5d460b5e470d60470e6147106347116447136548146748166848176948186a481a6c481b6d481c6e481d6f481f70482071482173482374482475482576482677482878482979472a7a472c7a472d7b472e7c472f7d46307e46327e46337f463480453581453781453882443983443a83443b84433d84433e85423f854240864241864142874144874045884046883f47883f48893e49893e4a893e4c8a3d4d8a3d4e8a3c4f8a3c508b3b518b3b528b3a538b3a548c39558c39568c38588c38598c375a8c375b8d365c8d365d8d355e8d355f8d34608d34618d33628d33638d32648e32658e31668e31678e31688e30698e306a8e2f6b8e2f6c8e2e6d8e2e6e8e2e6f8e2d708e2d718e2c718e2c728e2c738e2b748e2b758e2a768e2a778e2a788e29798e297a8e297b8e287c8e287d8e277e8e277f8e27808e26818e26828e26828e25838e25848e25858e24868e24878e23888e23898e238a8d228b8d228c8d228d8d218e8d218f8d21908d21918c20928c20928c20938c1f948c1f958b1f968b1f978b1f988b1f998a1f9a8a1e9b8a1e9c891e9d891f9e891f9f881fa0881fa1881fa1871fa28720a38620a48621a58521a68522a78522a88423a98324aa8325ab8225ac8226ad8127ad8128ae8029af7f2ab07f2cb17e2db27d2eb37c2fb47c31b57b32b67a34b67935b77937b87838b9773aba763bbb753dbc743fbc7340bd7242be7144bf7046c06f48c16e4ac16d4cc26c4ec36b50c46a52c56954c56856c66758c7655ac8645cc8635ec96260ca6063cb5f65cb5e67cc5c69cd5b6ccd5a6ece5870cf5773d05675d05477d1537ad1517cd2507fd34e81d34d84d44b86d54989d5488bd6468ed64590d74393d74195d84098d83e9bd93c9dd93ba0da39a2da37a5db36a8db34aadc32addc30b0dd2fb2dd2db5de2bb8de29bade28bddf26c0df25c2df23c5e021c8e020cae11fcde11dd0e11cd2e21bd5e21ad8e219dae319dde318dfe318e2e418e5e419e7e419eae51aece51befe51cf1e51df4e61ef6e620f8e621fbe723fde725"));

var magma = ramp(colors("00000401000501010601010802010902020b02020d03030f03031204041405041606051806051a07061c08071e0907200a08220b09240c09260d0a290e0b2b100b2d110c2f120d31130d34140e36150e38160f3b180f3d19103f1a10421c10441d11471e114920114b21114e22115024125325125527125829115a2a115c2c115f2d11612f116331116533106734106936106b38106c390f6e3b0f703d0f713f0f72400f74420f75440f764510774710784910784a10794c117a4e117b4f127b51127c52137c54137d56147d57157e59157e5a167e5c167f5d177f5f187f601880621980641a80651a80671b80681c816a1c816b1d816d1d816e1e81701f81721f817320817521817621817822817922827b23827c23827e24828025828125818326818426818627818827818928818b29818c29818e2a81902a81912b81932b80942c80962c80982d80992d809b2e7f9c2e7f9e2f7fa02f7fa1307ea3307ea5317ea6317da8327daa337dab337cad347cae347bb0357bb2357bb3367ab5367ab73779b83779ba3878bc3978bd3977bf3a77c03a76c23b75c43c75c53c74c73d73c83e73ca3e72cc3f71cd4071cf4070d0416fd2426fd3436ed5446dd6456cd8456cd9466bdb476adc4869de4968df4a68e04c67e24d66e34e65e44f64e55064e75263e85362e95462ea5661eb5760ec5860ed5a5fee5b5eef5d5ef05f5ef1605df2625df2645cf3655cf4675cf4695cf56b5cf66c5cf66e5cf7705cf7725cf8745cf8765cf9785df9795df97b5dfa7d5efa7f5efa815ffb835ffb8560fb8761fc8961fc8a62fc8c63fc8e64fc9065fd9266fd9467fd9668fd9869fd9a6afd9b6bfe9d6cfe9f6dfea16efea36ffea571fea772fea973feaa74feac76feae77feb078feb27afeb47bfeb67cfeb77efeb97ffebb81febd82febf84fec185fec287fec488fec68afec88cfeca8dfecc8ffecd90fecf92fed194fed395fed597fed799fed89afdda9cfddc9efddea0fde0a1fde2a3fde3a5fde5a7fde7a9fde9aafdebacfcecaefceeb0fcf0b2fcf2b4fcf4b6fcf6b8fcf7b9fcf9bbfcfbbdfcfdbf"));

var inferno = ramp(colors("00000401000501010601010802010a02020c02020e03021004031204031405041706041907051b08051d09061f0a07220b07240c08260d08290e092b10092d110a30120a32140b34150b37160b39180c3c190c3e1b0c411c0c431e0c451f0c48210c4a230c4c240c4f260c51280b53290b552b0b572d0b592f0a5b310a5c320a5e340a5f3609613809623909633b09643d09653e0966400a67420a68440a68450a69470b6a490b6a4a0c6b4c0c6b4d0d6c4f0d6c510e6c520e6d540f6d550f6d57106e59106e5a116e5c126e5d126e5f136e61136e62146e64156e65156e67166e69166e6a176e6c186e6d186e6f196e71196e721a6e741a6e751b6e771c6d781c6d7a1d6d7c1d6d7d1e6d7f1e6c801f6c82206c84206b85216b87216b88226a8a226a8c23698d23698f24699025689225689326679526679727669827669a28659b29649d29649f2a63a02a63a22b62a32c61a52c60a62d60a82e5fa92e5eab2f5ead305dae305cb0315bb1325ab3325ab43359b63458b73557b93556ba3655bc3754bd3853bf3952c03a51c13a50c33b4fc43c4ec63d4dc73e4cc83f4bca404acb4149cc4248ce4347cf4446d04545d24644d34743d44842d54a41d74b3fd84c3ed94d3dda4e3cdb503bdd513ade5238df5337e05536e15635e25734e35933e45a31e55c30e65d2fe75e2ee8602de9612bea632aeb6429eb6628ec6726ed6925ee6a24ef6c23ef6e21f06f20f1711ff1731df2741cf3761bf37819f47918f57b17f57d15f67e14f68013f78212f78410f8850ff8870ef8890cf98b0bf98c0af98e09fa9008fa9207fa9407fb9606fb9706fb9906fb9b06fb9d07fc9f07fca108fca309fca50afca60cfca80dfcaa0ffcac11fcae12fcb014fcb216fcb418fbb61afbb81dfbba1ffbbc21fbbe23fac026fac228fac42afac62df9c72ff9c932f9cb35f8cd37f8cf3af7d13df7d340f6d543f6d746f5d949f5db4cf4dd4ff4df53f4e156f3e35af3e55df2e661f2e865f2ea69f1ec6df1ed71f1ef75f1f179f2f27df2f482f3f586f3f68af4f88ef5f992f6fa96f8fb9af9fc9dfafda1fcffa4"));

var plasma = ramp(colors("0d088710078813078916078a19068c1b068d1d068e20068f2206902406912605912805922a05932c05942e05952f059631059733059735049837049938049a3a049a3c049b3e049c3f049c41049d43039e44039e46039f48039f4903a04b03a14c02a14e02a25002a25102a35302a35502a45601a45801a45901a55b01a55c01a65e01a66001a66100a76300a76400a76600a76700a86900a86a00a86c00a86e00a86f00a87100a87201a87401a87501a87701a87801a87a02a87b02a87d03a87e03a88004a88104a78305a78405a78606a68707a68808a68a09a58b0aa58d0ba58e0ca48f0da4910ea3920fa39410a29511a19613a19814a099159f9a169f9c179e9d189d9e199da01a9ca11b9ba21d9aa31e9aa51f99a62098a72197a82296aa2395ab2494ac2694ad2793ae2892b02991b12a90b22b8fb32c8eb42e8db52f8cb6308bb7318ab83289ba3388bb3488bc3587bd3786be3885bf3984c03a83c13b82c23c81c33d80c43e7fc5407ec6417dc7427cc8437bc9447aca457acb4679cc4778cc4977cd4a76ce4b75cf4c74d04d73d14e72d24f71d35171d45270d5536fd5546ed6556dd7566cd8576bd9586ada5a6ada5b69db5c68dc5d67dd5e66de5f65de6164df6263e06363e16462e26561e26660e3685fe4695ee56a5de56b5de66c5ce76e5be76f5ae87059e97158e97257ea7457eb7556eb7655ec7754ed7953ed7a52ee7b51ef7c51ef7e50f07f4ff0804ef1814df1834cf2844bf3854bf3874af48849f48948f58b47f58c46f68d45f68f44f79044f79143f79342f89441f89540f9973ff9983ef99a3efa9b3dfa9c3cfa9e3bfb9f3afba139fba238fca338fca537fca636fca835fca934fdab33fdac33fdae32fdaf31fdb130fdb22ffdb42ffdb52efeb72dfeb82cfeba2cfebb2bfebd2afebe2afec029fdc229fdc328fdc527fdc627fdc827fdca26fdcb26fccd25fcce25fcd025fcd225fbd324fbd524fbd724fad824fada24f9dc24f9dd25f8df25f8e125f7e225f7e425f6e626f6e826f5e926f5eb27f4ed27f3ee27f3f027f2f227f1f426f1f525f0f724f0f921"));

function constant$1(x) {
  return function constant() {
    return x;
  };
}

const abs = Math.abs;
const atan2 = Math.atan2;
const cos = Math.cos;
const max = Math.max;
const min = Math.min;
const sin = Math.sin;
const sqrt = Math.sqrt;

const epsilon = 1e-12;
const pi = Math.PI;
const halfPi = pi / 2;
const tau = 2 * pi;

function acos(x) {
  return x > 1 ? 0 : x < -1 ? pi : Math.acos(x);
}

function asin(x) {
  return x >= 1 ? halfPi : x <= -1 ? -halfPi : Math.asin(x);
}

function withPath(shape) {
  let digits = 3;

  shape.digits = function(_) {
    if (!arguments.length) return digits;
    if (_ == null) {
      digits = null;
    } else {
      const d = Math.floor(_);
      if (!(d >= 0)) throw new RangeError(`invalid digits: ${_}`);
      digits = d;
    }
    return shape;
  };

  return () => new Path$1(digits);
}

function arcInnerRadius(d) {
  return d.innerRadius;
}

function arcOuterRadius(d) {
  return d.outerRadius;
}

function arcStartAngle(d) {
  return d.startAngle;
}

function arcEndAngle(d) {
  return d.endAngle;
}

function arcPadAngle(d) {
  return d && d.padAngle; // Note: optional!
}

function intersect(x0, y0, x1, y1, x2, y2, x3, y3) {
  var x10 = x1 - x0, y10 = y1 - y0,
      x32 = x3 - x2, y32 = y3 - y2,
      t = y32 * x10 - x32 * y10;
  if (t * t < epsilon) return;
  t = (x32 * (y0 - y2) - y32 * (x0 - x2)) / t;
  return [x0 + t * x10, y0 + t * y10];
}

// Compute perpendicular offset line of length rc.
// http://mathworld.wolfram.com/Circle-LineIntersection.html
function cornerTangents(x0, y0, x1, y1, r1, rc, cw) {
  var x01 = x0 - x1,
      y01 = y0 - y1,
      lo = (cw ? rc : -rc) / sqrt(x01 * x01 + y01 * y01),
      ox = lo * y01,
      oy = -lo * x01,
      x11 = x0 + ox,
      y11 = y0 + oy,
      x10 = x1 + ox,
      y10 = y1 + oy,
      x00 = (x11 + x10) / 2,
      y00 = (y11 + y10) / 2,
      dx = x10 - x11,
      dy = y10 - y11,
      d2 = dx * dx + dy * dy,
      r = r1 - rc,
      D = x11 * y10 - x10 * y11,
      d = (dy < 0 ? -1 : 1) * sqrt(max(0, r * r * d2 - D * D)),
      cx0 = (D * dy - dx * d) / d2,
      cy0 = (-D * dx - dy * d) / d2,
      cx1 = (D * dy + dx * d) / d2,
      cy1 = (-D * dx + dy * d) / d2,
      dx0 = cx0 - x00,
      dy0 = cy0 - y00,
      dx1 = cx1 - x00,
      dy1 = cy1 - y00;

  // Pick the closer of the two intersection points.
  // TODO Is there a faster way to determine which intersection to use?
  if (dx0 * dx0 + dy0 * dy0 > dx1 * dx1 + dy1 * dy1) cx0 = cx1, cy0 = cy1;

  return {
    cx: cx0,
    cy: cy0,
    x01: -ox,
    y01: -oy,
    x11: cx0 * (r1 / r - 1),
    y11: cy0 * (r1 / r - 1)
  };
}

function arc() {
  var innerRadius = arcInnerRadius,
      outerRadius = arcOuterRadius,
      cornerRadius = constant$1(0),
      padRadius = null,
      startAngle = arcStartAngle,
      endAngle = arcEndAngle,
      padAngle = arcPadAngle,
      context = null,
      path = withPath(arc);

  function arc() {
    var buffer,
        r,
        r0 = +innerRadius.apply(this, arguments),
        r1 = +outerRadius.apply(this, arguments),
        a0 = startAngle.apply(this, arguments) - halfPi,
        a1 = endAngle.apply(this, arguments) - halfPi,
        da = abs(a1 - a0),
        cw = a1 > a0;

    if (!context) context = buffer = path();

    // Ensure that the outer radius is always larger than the inner radius.
    if (r1 < r0) r = r1, r1 = r0, r0 = r;

    // Is it a point?
    if (!(r1 > epsilon)) context.moveTo(0, 0);

    // Or is it a circle or annulus?
    else if (da > tau - epsilon) {
      context.moveTo(r1 * cos(a0), r1 * sin(a0));
      context.arc(0, 0, r1, a0, a1, !cw);
      if (r0 > epsilon) {
        context.moveTo(r0 * cos(a1), r0 * sin(a1));
        context.arc(0, 0, r0, a1, a0, cw);
      }
    }

    // Or is it a circular or annular sector?
    else {
      var a01 = a0,
          a11 = a1,
          a00 = a0,
          a10 = a1,
          da0 = da,
          da1 = da,
          ap = padAngle.apply(this, arguments) / 2,
          rp = (ap > epsilon) && (padRadius ? +padRadius.apply(this, arguments) : sqrt(r0 * r0 + r1 * r1)),
          rc = min(abs(r1 - r0) / 2, +cornerRadius.apply(this, arguments)),
          rc0 = rc,
          rc1 = rc,
          t0,
          t1;

      // Apply padding? Note that since r1 ≥ r0, da1 ≥ da0.
      if (rp > epsilon) {
        var p0 = asin(rp / r0 * sin(ap)),
            p1 = asin(rp / r1 * sin(ap));
        if ((da0 -= p0 * 2) > epsilon) p0 *= (cw ? 1 : -1), a00 += p0, a10 -= p0;
        else da0 = 0, a00 = a10 = (a0 + a1) / 2;
        if ((da1 -= p1 * 2) > epsilon) p1 *= (cw ? 1 : -1), a01 += p1, a11 -= p1;
        else da1 = 0, a01 = a11 = (a0 + a1) / 2;
      }

      var x01 = r1 * cos(a01),
          y01 = r1 * sin(a01),
          x10 = r0 * cos(a10),
          y10 = r0 * sin(a10);

      // Apply rounded corners?
      if (rc > epsilon) {
        var x11 = r1 * cos(a11),
            y11 = r1 * sin(a11),
            x00 = r0 * cos(a00),
            y00 = r0 * sin(a00),
            oc;

        // Restrict the corner radius according to the sector angle. If this
        // intersection fails, it’s probably because the arc is too small, so
        // disable the corner radius entirely.
        if (da < pi) {
          if (oc = intersect(x01, y01, x00, y00, x11, y11, x10, y10)) {
            var ax = x01 - oc[0],
                ay = y01 - oc[1],
                bx = x11 - oc[0],
                by = y11 - oc[1],
                kc = 1 / sin(acos((ax * bx + ay * by) / (sqrt(ax * ax + ay * ay) * sqrt(bx * bx + by * by))) / 2),
                lc = sqrt(oc[0] * oc[0] + oc[1] * oc[1]);
            rc0 = min(rc, (r0 - lc) / (kc - 1));
            rc1 = min(rc, (r1 - lc) / (kc + 1));
          } else {
            rc0 = rc1 = 0;
          }
        }
      }

      // Is the sector collapsed to a line?
      if (!(da1 > epsilon)) context.moveTo(x01, y01);

      // Does the sector’s outer ring have rounded corners?
      else if (rc1 > epsilon) {
        t0 = cornerTangents(x00, y00, x01, y01, r1, rc1, cw);
        t1 = cornerTangents(x11, y11, x10, y10, r1, rc1, cw);

        context.moveTo(t0.cx + t0.x01, t0.cy + t0.y01);

        // Have the corners merged?
        if (rc1 < rc) context.arc(t0.cx, t0.cy, rc1, atan2(t0.y01, t0.x01), atan2(t1.y01, t1.x01), !cw);

        // Otherwise, draw the two corners and the ring.
        else {
          context.arc(t0.cx, t0.cy, rc1, atan2(t0.y01, t0.x01), atan2(t0.y11, t0.x11), !cw);
          context.arc(0, 0, r1, atan2(t0.cy + t0.y11, t0.cx + t0.x11), atan2(t1.cy + t1.y11, t1.cx + t1.x11), !cw);
          context.arc(t1.cx, t1.cy, rc1, atan2(t1.y11, t1.x11), atan2(t1.y01, t1.x01), !cw);
        }
      }

      // Or is the outer ring just a circular arc?
      else context.moveTo(x01, y01), context.arc(0, 0, r1, a01, a11, !cw);

      // Is there no inner ring, and it’s a circular sector?
      // Or perhaps it’s an annular sector collapsed due to padding?
      if (!(r0 > epsilon) || !(da0 > epsilon)) context.lineTo(x10, y10);

      // Does the sector’s inner ring (or point) have rounded corners?
      else if (rc0 > epsilon) {
        t0 = cornerTangents(x10, y10, x11, y11, r0, -rc0, cw);
        t1 = cornerTangents(x01, y01, x00, y00, r0, -rc0, cw);

        context.lineTo(t0.cx + t0.x01, t0.cy + t0.y01);

        // Have the corners merged?
        if (rc0 < rc) context.arc(t0.cx, t0.cy, rc0, atan2(t0.y01, t0.x01), atan2(t1.y01, t1.x01), !cw);

        // Otherwise, draw the two corners and the ring.
        else {
          context.arc(t0.cx, t0.cy, rc0, atan2(t0.y01, t0.x01), atan2(t0.y11, t0.x11), !cw);
          context.arc(0, 0, r0, atan2(t0.cy + t0.y11, t0.cx + t0.x11), atan2(t1.cy + t1.y11, t1.cx + t1.x11), cw);
          context.arc(t1.cx, t1.cy, rc0, atan2(t1.y11, t1.x11), atan2(t1.y01, t1.x01), !cw);
        }
      }

      // Or is the inner ring just a circular arc?
      else context.arc(0, 0, r0, a10, a00, cw);
    }

    context.closePath();

    if (buffer) return context = null, buffer + "" || null;
  }

  arc.centroid = function() {
    var r = (+innerRadius.apply(this, arguments) + +outerRadius.apply(this, arguments)) / 2,
        a = (+startAngle.apply(this, arguments) + +endAngle.apply(this, arguments)) / 2 - pi / 2;
    return [cos(a) * r, sin(a) * r];
  };

  arc.innerRadius = function(_) {
    return arguments.length ? (innerRadius = typeof _ === "function" ? _ : constant$1(+_), arc) : innerRadius;
  };

  arc.outerRadius = function(_) {
    return arguments.length ? (outerRadius = typeof _ === "function" ? _ : constant$1(+_), arc) : outerRadius;
  };

  arc.cornerRadius = function(_) {
    return arguments.length ? (cornerRadius = typeof _ === "function" ? _ : constant$1(+_), arc) : cornerRadius;
  };

  arc.padRadius = function(_) {
    return arguments.length ? (padRadius = _ == null ? null : typeof _ === "function" ? _ : constant$1(+_), arc) : padRadius;
  };

  arc.startAngle = function(_) {
    return arguments.length ? (startAngle = typeof _ === "function" ? _ : constant$1(+_), arc) : startAngle;
  };

  arc.endAngle = function(_) {
    return arguments.length ? (endAngle = typeof _ === "function" ? _ : constant$1(+_), arc) : endAngle;
  };

  arc.padAngle = function(_) {
    return arguments.length ? (padAngle = typeof _ === "function" ? _ : constant$1(+_), arc) : padAngle;
  };

  arc.context = function(_) {
    return arguments.length ? ((context = _ == null ? null : _), arc) : context;
  };

  return arc;
}

var slice = Array.prototype.slice;

function array(x) {
  return typeof x === "object" && "length" in x
    ? x // Array, TypedArray, NodeList, array-like
    : Array.from(x); // Map, Set, iterable, string, or anything else
}

function Linear(context) {
  this._context = context;
}

Linear.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._point = 0;
  },
  lineEnd: function() {
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; // falls through
      default: this._context.lineTo(x, y); break;
    }
  }
};

function curveLinear(context) {
  return new Linear(context);
}

function x(p) {
  return p[0];
}

function y(p) {
  return p[1];
}

function line(x$1, y$1) {
  var defined = constant$1(true),
      context = null,
      curve = curveLinear,
      output = null,
      path = withPath(line);

  x$1 = typeof x$1 === "function" ? x$1 : (x$1 === undefined) ? x : constant$1(x$1);
  y$1 = typeof y$1 === "function" ? y$1 : (y$1 === undefined) ? y : constant$1(y$1);

  function line(data) {
    var i,
        n = (data = array(data)).length,
        d,
        defined0 = false,
        buffer;

    if (context == null) output = curve(buffer = path());

    for (i = 0; i <= n; ++i) {
      if (!(i < n && defined(d = data[i], i, data)) === defined0) {
        if (defined0 = !defined0) output.lineStart();
        else output.lineEnd();
      }
      if (defined0) output.point(+x$1(d, i, data), +y$1(d, i, data));
    }

    if (buffer) return output = null, buffer + "" || null;
  }

  line.x = function(_) {
    return arguments.length ? (x$1 = typeof _ === "function" ? _ : constant$1(+_), line) : x$1;
  };

  line.y = function(_) {
    return arguments.length ? (y$1 = typeof _ === "function" ? _ : constant$1(+_), line) : y$1;
  };

  line.defined = function(_) {
    return arguments.length ? (defined = typeof _ === "function" ? _ : constant$1(!!_), line) : defined;
  };

  line.curve = function(_) {
    return arguments.length ? (curve = _, context != null && (output = curve(context)), line) : curve;
  };

  line.context = function(_) {
    return arguments.length ? (_ == null ? context = output = null : output = curve(context = _), line) : context;
  };

  return line;
}

function area(x0, y0, y1) {
  var x1 = null,
      defined = constant$1(true),
      context = null,
      curve = curveLinear,
      output = null,
      path = withPath(area);

  x0 = typeof x0 === "function" ? x0 : (x0 === undefined) ? x : constant$1(+x0);
  y0 = typeof y0 === "function" ? y0 : (y0 === undefined) ? constant$1(0) : constant$1(+y0);
  y1 = typeof y1 === "function" ? y1 : (y1 === undefined) ? y : constant$1(+y1);

  function area(data) {
    var i,
        j,
        k,
        n = (data = array(data)).length,
        d,
        defined0 = false,
        buffer,
        x0z = new Array(n),
        y0z = new Array(n);

    if (context == null) output = curve(buffer = path());

    for (i = 0; i <= n; ++i) {
      if (!(i < n && defined(d = data[i], i, data)) === defined0) {
        if (defined0 = !defined0) {
          j = i;
          output.areaStart();
          output.lineStart();
        } else {
          output.lineEnd();
          output.lineStart();
          for (k = i - 1; k >= j; --k) {
            output.point(x0z[k], y0z[k]);
          }
          output.lineEnd();
          output.areaEnd();
        }
      }
      if (defined0) {
        x0z[i] = +x0(d, i, data), y0z[i] = +y0(d, i, data);
        output.point(x1 ? +x1(d, i, data) : x0z[i], y1 ? +y1(d, i, data) : y0z[i]);
      }
    }

    if (buffer) return output = null, buffer + "" || null;
  }

  function arealine() {
    return line().defined(defined).curve(curve).context(context);
  }

  area.x = function(_) {
    return arguments.length ? (x0 = typeof _ === "function" ? _ : constant$1(+_), x1 = null, area) : x0;
  };

  area.x0 = function(_) {
    return arguments.length ? (x0 = typeof _ === "function" ? _ : constant$1(+_), area) : x0;
  };

  area.x1 = function(_) {
    return arguments.length ? (x1 = _ == null ? null : typeof _ === "function" ? _ : constant$1(+_), area) : x1;
  };

  area.y = function(_) {
    return arguments.length ? (y0 = typeof _ === "function" ? _ : constant$1(+_), y1 = null, area) : y0;
  };

  area.y0 = function(_) {
    return arguments.length ? (y0 = typeof _ === "function" ? _ : constant$1(+_), area) : y0;
  };

  area.y1 = function(_) {
    return arguments.length ? (y1 = _ == null ? null : typeof _ === "function" ? _ : constant$1(+_), area) : y1;
  };

  area.lineX0 =
  area.lineY0 = function() {
    return arealine().x(x0).y(y0);
  };

  area.lineY1 = function() {
    return arealine().x(x0).y(y1);
  };

  area.lineX1 = function() {
    return arealine().x(x1).y(y0);
  };

  area.defined = function(_) {
    return arguments.length ? (defined = typeof _ === "function" ? _ : constant$1(!!_), area) : defined;
  };

  area.curve = function(_) {
    return arguments.length ? (curve = _, context != null && (output = curve(context)), area) : curve;
  };

  area.context = function(_) {
    return arguments.length ? (_ == null ? context = output = null : output = curve(context = _), area) : context;
  };

  return area;
}

function descending$1(a, b) {
  return b < a ? -1 : b > a ? 1 : b >= a ? 0 : NaN;
}

function identity$1(d) {
  return d;
}

function pie() {
  var value = identity$1,
      sortValues = descending$1,
      sort = null,
      startAngle = constant$1(0),
      endAngle = constant$1(tau),
      padAngle = constant$1(0);

  function pie(data) {
    var i,
        n = (data = array(data)).length,
        j,
        k,
        sum = 0,
        index = new Array(n),
        arcs = new Array(n),
        a0 = +startAngle.apply(this, arguments),
        da = Math.min(tau, Math.max(-tau, endAngle.apply(this, arguments) - a0)),
        a1,
        p = Math.min(Math.abs(da) / n, padAngle.apply(this, arguments)),
        pa = p * (da < 0 ? -1 : 1),
        v;

    for (i = 0; i < n; ++i) {
      if ((v = arcs[index[i] = i] = +value(data[i], i, data)) > 0) {
        sum += v;
      }
    }

    // Optionally sort the arcs by previously-computed values or by data.
    if (sortValues != null) index.sort(function(i, j) { return sortValues(arcs[i], arcs[j]); });
    else if (sort != null) index.sort(function(i, j) { return sort(data[i], data[j]); });

    // Compute the arcs! They are stored in the original data's order.
    for (i = 0, k = sum ? (da - n * pa) / sum : 0; i < n; ++i, a0 = a1) {
      j = index[i], v = arcs[j], a1 = a0 + (v > 0 ? v * k : 0) + pa, arcs[j] = {
        data: data[j],
        index: i,
        value: v,
        startAngle: a0,
        endAngle: a1,
        padAngle: p
      };
    }

    return arcs;
  }

  pie.value = function(_) {
    return arguments.length ? (value = typeof _ === "function" ? _ : constant$1(+_), pie) : value;
  };

  pie.sortValues = function(_) {
    return arguments.length ? (sortValues = _, sort = null, pie) : sortValues;
  };

  pie.sort = function(_) {
    return arguments.length ? (sort = _, sortValues = null, pie) : sort;
  };

  pie.startAngle = function(_) {
    return arguments.length ? (startAngle = typeof _ === "function" ? _ : constant$1(+_), pie) : startAngle;
  };

  pie.endAngle = function(_) {
    return arguments.length ? (endAngle = typeof _ === "function" ? _ : constant$1(+_), pie) : endAngle;
  };

  pie.padAngle = function(_) {
    return arguments.length ? (padAngle = typeof _ === "function" ? _ : constant$1(+_), pie) : padAngle;
  };

  return pie;
}

var curveRadialLinear = curveRadial(curveLinear);

function Radial(curve) {
  this._curve = curve;
}

Radial.prototype = {
  areaStart: function() {
    this._curve.areaStart();
  },
  areaEnd: function() {
    this._curve.areaEnd();
  },
  lineStart: function() {
    this._curve.lineStart();
  },
  lineEnd: function() {
    this._curve.lineEnd();
  },
  point: function(a, r) {
    this._curve.point(r * Math.sin(a), r * -Math.cos(a));
  }
};

function curveRadial(curve) {

  function radial(context) {
    return new Radial(curve(context));
  }

  radial._curve = curve;

  return radial;
}

function lineRadial(l) {
  var c = l.curve;

  l.angle = l.x, delete l.x;
  l.radius = l.y, delete l.y;

  l.curve = function(_) {
    return arguments.length ? c(curveRadial(_)) : c()._curve;
  };

  return l;
}

function lineRadial_default() {
  return lineRadial(line().curve(curveRadialLinear));
}

function areaRadial() {
  var a = area().curve(curveRadialLinear),
      c = a.curve,
      x0 = a.lineX0,
      x1 = a.lineX1,
      y0 = a.lineY0,
      y1 = a.lineY1;

  a.angle = a.x, delete a.x;
  a.startAngle = a.x0, delete a.x0;
  a.endAngle = a.x1, delete a.x1;
  a.radius = a.y, delete a.y;
  a.innerRadius = a.y0, delete a.y0;
  a.outerRadius = a.y1, delete a.y1;
  a.lineStartAngle = function() { return lineRadial(x0()); }, delete a.lineX0;
  a.lineEndAngle = function() { return lineRadial(x1()); }, delete a.lineX1;
  a.lineInnerRadius = function() { return lineRadial(y0()); }, delete a.lineY0;
  a.lineOuterRadius = function() { return lineRadial(y1()); }, delete a.lineY1;

  a.curve = function(_) {
    return arguments.length ? c(curveRadial(_)) : c()._curve;
  };

  return a;
}

function pointRadial(x, y) {
  return [(y = +y) * Math.cos(x -= Math.PI / 2), y * Math.sin(x)];
}

class Bump {
  constructor(context, x) {
    this._context = context;
    this._x = x;
  }
  areaStart() {
    this._line = 0;
  }
  areaEnd() {
    this._line = NaN;
  }
  lineStart() {
    this._point = 0;
  }
  lineEnd() {
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  }
  point(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: {
        this._point = 1;
        if (this._line) this._context.lineTo(x, y);
        else this._context.moveTo(x, y);
        break;
      }
      case 1: this._point = 2; // falls through
      default: {
        if (this._x) this._context.bezierCurveTo(this._x0 = (this._x0 + x) / 2, this._y0, this._x0, y, x, y);
        else this._context.bezierCurveTo(this._x0, this._y0 = (this._y0 + y) / 2, x, this._y0, x, y);
        break;
      }
    }
    this._x0 = x, this._y0 = y;
  }
}

class BumpRadial {
  constructor(context) {
    this._context = context;
  }
  lineStart() {
    this._point = 0;
  }
  lineEnd() {}
  point(x, y) {
    x = +x, y = +y;
    if (this._point === 0) {
      this._point = 1;
    } else {
      const p0 = pointRadial(this._x0, this._y0);
      const p1 = pointRadial(this._x0, this._y0 = (this._y0 + y) / 2);
      const p2 = pointRadial(x, this._y0);
      const p3 = pointRadial(x, y);
      this._context.moveTo(...p0);
      this._context.bezierCurveTo(...p1, ...p2, ...p3);
    }
    this._x0 = x, this._y0 = y;
  }
}

function bumpX(context) {
  return new Bump(context, true);
}

function bumpY(context) {
  return new Bump(context, false);
}

function bumpRadial(context) {
  return new BumpRadial(context);
}

function linkSource(d) {
  return d.source;
}

function linkTarget(d) {
  return d.target;
}

function link(curve) {
  let source = linkSource,
      target = linkTarget,
      x$1 = x,
      y$1 = y,
      context = null,
      output = null,
      path = withPath(link);

  function link() {
    let buffer;
    const argv = slice.call(arguments);
    const s = source.apply(this, argv);
    const t = target.apply(this, argv);
    if (context == null) output = curve(buffer = path());
    output.lineStart();
    argv[0] = s, output.point(+x$1.apply(this, argv), +y$1.apply(this, argv));
    argv[0] = t, output.point(+x$1.apply(this, argv), +y$1.apply(this, argv));
    output.lineEnd();
    if (buffer) return output = null, buffer + "" || null;
  }

  link.source = function(_) {
    return arguments.length ? (source = _, link) : source;
  };

  link.target = function(_) {
    return arguments.length ? (target = _, link) : target;
  };

  link.x = function(_) {
    return arguments.length ? (x$1 = typeof _ === "function" ? _ : constant$1(+_), link) : x$1;
  };

  link.y = function(_) {
    return arguments.length ? (y$1 = typeof _ === "function" ? _ : constant$1(+_), link) : y$1;
  };

  link.context = function(_) {
    return arguments.length ? (_ == null ? context = output = null : output = curve(context = _), link) : context;
  };

  return link;
}

function linkHorizontal() {
  return link(bumpX);
}

function linkVertical() {
  return link(bumpY);
}

function linkRadial() {
  const l = link(bumpRadial);
  l.angle = l.x, delete l.x;
  l.radius = l.y, delete l.y;
  return l;
}

const sqrt3$2 = sqrt(3);

const asterisk = {
  draw(context, size) {
    const r = sqrt(size + min(size / 28, 0.75)) * 0.59436;
    const t = r / 2;
    const u = t * sqrt3$2;
    context.moveTo(0, r);
    context.lineTo(0, -r);
    context.moveTo(-u, -t);
    context.lineTo(u, t);
    context.moveTo(-u, t);
    context.lineTo(u, -t);
  }
};

const circle = {
  draw(context, size) {
    const r = sqrt(size / pi);
    context.moveTo(r, 0);
    context.arc(0, 0, r, 0, tau);
  }
};

const cross = {
  draw(context, size) {
    const r = sqrt(size / 5) / 2;
    context.moveTo(-3 * r, -r);
    context.lineTo(-r, -r);
    context.lineTo(-r, -3 * r);
    context.lineTo(r, -3 * r);
    context.lineTo(r, -r);
    context.lineTo(3 * r, -r);
    context.lineTo(3 * r, r);
    context.lineTo(r, r);
    context.lineTo(r, 3 * r);
    context.lineTo(-r, 3 * r);
    context.lineTo(-r, r);
    context.lineTo(-3 * r, r);
    context.closePath();
  }
};

const tan30 = sqrt(1 / 3);
const tan30_2 = tan30 * 2;

const diamond = {
  draw(context, size) {
    const y = sqrt(size / tan30_2);
    const x = y * tan30;
    context.moveTo(0, -y);
    context.lineTo(x, 0);
    context.lineTo(0, y);
    context.lineTo(-x, 0);
    context.closePath();
  }
};

const diamond2 = {
  draw(context, size) {
    const r = sqrt(size) * 0.62625;
    context.moveTo(0, -r);
    context.lineTo(r, 0);
    context.lineTo(0, r);
    context.lineTo(-r, 0);
    context.closePath();
  }
};

const plus = {
  draw(context, size) {
    const r = sqrt(size - min(size / 7, 2)) * 0.87559;
    context.moveTo(-r, 0);
    context.lineTo(r, 0);
    context.moveTo(0, r);
    context.lineTo(0, -r);
  }
};

const square = {
  draw(context, size) {
    const w = sqrt(size);
    const x = -w / 2;
    context.rect(x, x, w, w);
  }
};

const square2 = {
  draw(context, size) {
    const r = sqrt(size) * 0.4431;
    context.moveTo(r, r);
    context.lineTo(r, -r);
    context.lineTo(-r, -r);
    context.lineTo(-r, r);
    context.closePath();
  }
};

const ka = 0.89081309152928522810;
const kr = sin(pi / 10) / sin(7 * pi / 10);
const kx = sin(tau / 10) * kr;
const ky = -cos(tau / 10) * kr;

const star = {
  draw(context, size) {
    const r = sqrt(size * ka);
    const x = kx * r;
    const y = ky * r;
    context.moveTo(0, -r);
    context.lineTo(x, y);
    for (let i = 1; i < 5; ++i) {
      const a = tau * i / 5;
      const c = cos(a);
      const s = sin(a);
      context.lineTo(s * r, -c * r);
      context.lineTo(c * x - s * y, s * x + c * y);
    }
    context.closePath();
  }
};

const sqrt3$1 = sqrt(3);

const triangle = {
  draw(context, size) {
    const y = -sqrt(size / (sqrt3$1 * 3));
    context.moveTo(0, y * 2);
    context.lineTo(-sqrt3$1 * y, -y);
    context.lineTo(sqrt3$1 * y, -y);
    context.closePath();
  }
};

const sqrt3 = sqrt(3);

const triangle2 = {
  draw(context, size) {
    const s = sqrt(size) * 0.6824;
    const t = s  / 2;
    const u = (s * sqrt3) / 2; // cos(Math.PI / 6)
    context.moveTo(0, -s);
    context.lineTo(u, t);
    context.lineTo(-u, t);
    context.closePath();
  }
};

const c = -0.5;
const s = sqrt(3) / 2;
const k = 1 / sqrt(12);
const a = (k / 2 + 1) * 3;

const wye = {
  draw(context, size) {
    const r = sqrt(size / a);
    const x0 = r / 2, y0 = r * k;
    const x1 = x0, y1 = r * k + r;
    const x2 = -x1, y2 = y1;
    context.moveTo(x0, y0);
    context.lineTo(x1, y1);
    context.lineTo(x2, y2);
    context.lineTo(c * x0 - s * y0, s * x0 + c * y0);
    context.lineTo(c * x1 - s * y1, s * x1 + c * y1);
    context.lineTo(c * x2 - s * y2, s * x2 + c * y2);
    context.lineTo(c * x0 + s * y0, c * y0 - s * x0);
    context.lineTo(c * x1 + s * y1, c * y1 - s * x1);
    context.lineTo(c * x2 + s * y2, c * y2 - s * x2);
    context.closePath();
  }
};

const times = {
  draw(context, size) {
    const r = sqrt(size - min(size / 6, 1.7)) * 0.6189;
    context.moveTo(-r, -r);
    context.lineTo(r, r);
    context.moveTo(-r, r);
    context.lineTo(r, -r);
  }
};

// These symbols are designed to be filled.
const symbolsFill = [
  circle,
  cross,
  diamond,
  square,
  star,
  triangle,
  wye
];

// These symbols are designed to be stroked (with a width of 1.5px and round caps).
const symbolsStroke = [
  circle,
  plus,
  times,
  triangle2,
  asterisk,
  square2,
  diamond2
];

function Symbol$1(type, size) {
  let context = null,
      path = withPath(symbol);

  type = typeof type === "function" ? type : constant$1(type || circle);
  size = typeof size === "function" ? size : constant$1(size === undefined ? 64 : +size);

  function symbol() {
    let buffer;
    if (!context) context = buffer = path();
    type.apply(this, arguments).draw(context, +size.apply(this, arguments));
    if (buffer) return context = null, buffer + "" || null;
  }

  symbol.type = function(_) {
    return arguments.length ? (type = typeof _ === "function" ? _ : constant$1(_), symbol) : type;
  };

  symbol.size = function(_) {
    return arguments.length ? (size = typeof _ === "function" ? _ : constant$1(+_), symbol) : size;
  };

  symbol.context = function(_) {
    return arguments.length ? (context = _ == null ? null : _, symbol) : context;
  };

  return symbol;
}

function noop() {}

function point$3(that, x, y) {
  that._context.bezierCurveTo(
    (2 * that._x0 + that._x1) / 3,
    (2 * that._y0 + that._y1) / 3,
    (that._x0 + 2 * that._x1) / 3,
    (that._y0 + 2 * that._y1) / 3,
    (that._x0 + 4 * that._x1 + x) / 6,
    (that._y0 + 4 * that._y1 + y) / 6
  );
}

function Basis(context) {
  this._context = context;
}

Basis.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 =
    this._y0 = this._y1 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 3: point$3(this, this._x1, this._y1); // falls through
      case 2: this._context.lineTo(this._x1, this._y1); break;
    }
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; this._context.lineTo((5 * this._x0 + this._x1) / 6, (5 * this._y0 + this._y1) / 6); // falls through
      default: point$3(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = x;
    this._y0 = this._y1, this._y1 = y;
  }
};

function basis(context) {
  return new Basis(context);
}

function BasisClosed(context) {
  this._context = context;
}

BasisClosed.prototype = {
  areaStart: noop,
  areaEnd: noop,
  lineStart: function() {
    this._x0 = this._x1 = this._x2 = this._x3 = this._x4 =
    this._y0 = this._y1 = this._y2 = this._y3 = this._y4 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 1: {
        this._context.moveTo(this._x2, this._y2);
        this._context.closePath();
        break;
      }
      case 2: {
        this._context.moveTo((this._x2 + 2 * this._x3) / 3, (this._y2 + 2 * this._y3) / 3);
        this._context.lineTo((this._x3 + 2 * this._x2) / 3, (this._y3 + 2 * this._y2) / 3);
        this._context.closePath();
        break;
      }
      case 3: {
        this.point(this._x2, this._y2);
        this.point(this._x3, this._y3);
        this.point(this._x4, this._y4);
        break;
      }
    }
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._x2 = x, this._y2 = y; break;
      case 1: this._point = 2; this._x3 = x, this._y3 = y; break;
      case 2: this._point = 3; this._x4 = x, this._y4 = y; this._context.moveTo((this._x0 + 4 * this._x1 + x) / 6, (this._y0 + 4 * this._y1 + y) / 6); break;
      default: point$3(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = x;
    this._y0 = this._y1, this._y1 = y;
  }
};

function basisClosed(context) {
  return new BasisClosed(context);
}

function BasisOpen(context) {
  this._context = context;
}

BasisOpen.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 =
    this._y0 = this._y1 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    if (this._line || (this._line !== 0 && this._point === 3)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; var x0 = (this._x0 + 4 * this._x1 + x) / 6, y0 = (this._y0 + 4 * this._y1 + y) / 6; this._line ? this._context.lineTo(x0, y0) : this._context.moveTo(x0, y0); break;
      case 3: this._point = 4; // falls through
      default: point$3(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = x;
    this._y0 = this._y1, this._y1 = y;
  }
};

function basisOpen(context) {
  return new BasisOpen(context);
}

function Bundle(context, beta) {
  this._basis = new Basis(context);
  this._beta = beta;
}

Bundle.prototype = {
  lineStart: function() {
    this._x = [];
    this._y = [];
    this._basis.lineStart();
  },
  lineEnd: function() {
    var x = this._x,
        y = this._y,
        j = x.length - 1;

    if (j > 0) {
      var x0 = x[0],
          y0 = y[0],
          dx = x[j] - x0,
          dy = y[j] - y0,
          i = -1,
          t;

      while (++i <= j) {
        t = i / j;
        this._basis.point(
          this._beta * x[i] + (1 - this._beta) * (x0 + t * dx),
          this._beta * y[i] + (1 - this._beta) * (y0 + t * dy)
        );
      }
    }

    this._x = this._y = null;
    this._basis.lineEnd();
  },
  point: function(x, y) {
    this._x.push(+x);
    this._y.push(+y);
  }
};

const bundle = (function custom(beta) {

  function bundle(context) {
    return beta === 1 ? new Basis(context) : new Bundle(context, beta);
  }

  bundle.beta = function(beta) {
    return custom(+beta);
  };

  return bundle;
})(0.85);

function point$2(that, x, y) {
  that._context.bezierCurveTo(
    that._x1 + that._k * (that._x2 - that._x0),
    that._y1 + that._k * (that._y2 - that._y0),
    that._x2 + that._k * (that._x1 - x),
    that._y2 + that._k * (that._y1 - y),
    that._x2,
    that._y2
  );
}

function Cardinal(context, tension) {
  this._context = context;
  this._k = (1 - tension) / 6;
}

Cardinal.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 = this._x2 =
    this._y0 = this._y1 = this._y2 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 2: this._context.lineTo(this._x2, this._y2); break;
      case 3: point$2(this, this._x1, this._y1); break;
    }
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; this._x1 = x, this._y1 = y; break;
      case 2: this._point = 3; // falls through
      default: point$2(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const cardinal = (function custom(tension) {

  function cardinal(context) {
    return new Cardinal(context, tension);
  }

  cardinal.tension = function(tension) {
    return custom(+tension);
  };

  return cardinal;
})(0);

function CardinalClosed(context, tension) {
  this._context = context;
  this._k = (1 - tension) / 6;
}

CardinalClosed.prototype = {
  areaStart: noop,
  areaEnd: noop,
  lineStart: function() {
    this._x0 = this._x1 = this._x2 = this._x3 = this._x4 = this._x5 =
    this._y0 = this._y1 = this._y2 = this._y3 = this._y4 = this._y5 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 1: {
        this._context.moveTo(this._x3, this._y3);
        this._context.closePath();
        break;
      }
      case 2: {
        this._context.lineTo(this._x3, this._y3);
        this._context.closePath();
        break;
      }
      case 3: {
        this.point(this._x3, this._y3);
        this.point(this._x4, this._y4);
        this.point(this._x5, this._y5);
        break;
      }
    }
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._x3 = x, this._y3 = y; break;
      case 1: this._point = 2; this._context.moveTo(this._x4 = x, this._y4 = y); break;
      case 2: this._point = 3; this._x5 = x, this._y5 = y; break;
      default: point$2(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const cardinalClosed = (function custom(tension) {

  function cardinal(context) {
    return new CardinalClosed(context, tension);
  }

  cardinal.tension = function(tension) {
    return custom(+tension);
  };

  return cardinal;
})(0);

function CardinalOpen(context, tension) {
  this._context = context;
  this._k = (1 - tension) / 6;
}

CardinalOpen.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 = this._x2 =
    this._y0 = this._y1 = this._y2 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    if (this._line || (this._line !== 0 && this._point === 3)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; this._line ? this._context.lineTo(this._x2, this._y2) : this._context.moveTo(this._x2, this._y2); break;
      case 3: this._point = 4; // falls through
      default: point$2(this, x, y); break;
    }
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const cardinalOpen = (function custom(tension) {

  function cardinal(context) {
    return new CardinalOpen(context, tension);
  }

  cardinal.tension = function(tension) {
    return custom(+tension);
  };

  return cardinal;
})(0);

function point$1(that, x, y) {
  var x1 = that._x1,
      y1 = that._y1,
      x2 = that._x2,
      y2 = that._y2;

  if (that._l01_a > epsilon) {
    var a = 2 * that._l01_2a + 3 * that._l01_a * that._l12_a + that._l12_2a,
        n = 3 * that._l01_a * (that._l01_a + that._l12_a);
    x1 = (x1 * a - that._x0 * that._l12_2a + that._x2 * that._l01_2a) / n;
    y1 = (y1 * a - that._y0 * that._l12_2a + that._y2 * that._l01_2a) / n;
  }

  if (that._l23_a > epsilon) {
    var b = 2 * that._l23_2a + 3 * that._l23_a * that._l12_a + that._l12_2a,
        m = 3 * that._l23_a * (that._l23_a + that._l12_a);
    x2 = (x2 * b + that._x1 * that._l23_2a - x * that._l12_2a) / m;
    y2 = (y2 * b + that._y1 * that._l23_2a - y * that._l12_2a) / m;
  }

  that._context.bezierCurveTo(x1, y1, x2, y2, that._x2, that._y2);
}

function CatmullRom(context, alpha) {
  this._context = context;
  this._alpha = alpha;
}

CatmullRom.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 = this._x2 =
    this._y0 = this._y1 = this._y2 = NaN;
    this._l01_a = this._l12_a = this._l23_a =
    this._l01_2a = this._l12_2a = this._l23_2a =
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 2: this._context.lineTo(this._x2, this._y2); break;
      case 3: this.point(this._x2, this._y2); break;
    }
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;

    if (this._point) {
      var x23 = this._x2 - x,
          y23 = this._y2 - y;
      this._l23_a = Math.sqrt(this._l23_2a = Math.pow(x23 * x23 + y23 * y23, this._alpha));
    }

    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; // falls through
      default: point$1(this, x, y); break;
    }

    this._l01_a = this._l12_a, this._l12_a = this._l23_a;
    this._l01_2a = this._l12_2a, this._l12_2a = this._l23_2a;
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const catmullRom = (function custom(alpha) {

  function catmullRom(context) {
    return alpha ? new CatmullRom(context, alpha) : new Cardinal(context, 0);
  }

  catmullRom.alpha = function(alpha) {
    return custom(+alpha);
  };

  return catmullRom;
})(0.5);

function CatmullRomClosed(context, alpha) {
  this._context = context;
  this._alpha = alpha;
}

CatmullRomClosed.prototype = {
  areaStart: noop,
  areaEnd: noop,
  lineStart: function() {
    this._x0 = this._x1 = this._x2 = this._x3 = this._x4 = this._x5 =
    this._y0 = this._y1 = this._y2 = this._y3 = this._y4 = this._y5 = NaN;
    this._l01_a = this._l12_a = this._l23_a =
    this._l01_2a = this._l12_2a = this._l23_2a =
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 1: {
        this._context.moveTo(this._x3, this._y3);
        this._context.closePath();
        break;
      }
      case 2: {
        this._context.lineTo(this._x3, this._y3);
        this._context.closePath();
        break;
      }
      case 3: {
        this.point(this._x3, this._y3);
        this.point(this._x4, this._y4);
        this.point(this._x5, this._y5);
        break;
      }
    }
  },
  point: function(x, y) {
    x = +x, y = +y;

    if (this._point) {
      var x23 = this._x2 - x,
          y23 = this._y2 - y;
      this._l23_a = Math.sqrt(this._l23_2a = Math.pow(x23 * x23 + y23 * y23, this._alpha));
    }

    switch (this._point) {
      case 0: this._point = 1; this._x3 = x, this._y3 = y; break;
      case 1: this._point = 2; this._context.moveTo(this._x4 = x, this._y4 = y); break;
      case 2: this._point = 3; this._x5 = x, this._y5 = y; break;
      default: point$1(this, x, y); break;
    }

    this._l01_a = this._l12_a, this._l12_a = this._l23_a;
    this._l01_2a = this._l12_2a, this._l12_2a = this._l23_2a;
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const catmullRomClosed = (function custom(alpha) {

  function catmullRom(context) {
    return alpha ? new CatmullRomClosed(context, alpha) : new CardinalClosed(context, 0);
  }

  catmullRom.alpha = function(alpha) {
    return custom(+alpha);
  };

  return catmullRom;
})(0.5);

function CatmullRomOpen(context, alpha) {
  this._context = context;
  this._alpha = alpha;
}

CatmullRomOpen.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 = this._x2 =
    this._y0 = this._y1 = this._y2 = NaN;
    this._l01_a = this._l12_a = this._l23_a =
    this._l01_2a = this._l12_2a = this._l23_2a =
    this._point = 0;
  },
  lineEnd: function() {
    if (this._line || (this._line !== 0 && this._point === 3)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;

    if (this._point) {
      var x23 = this._x2 - x,
          y23 = this._y2 - y;
      this._l23_a = Math.sqrt(this._l23_2a = Math.pow(x23 * x23 + y23 * y23, this._alpha));
    }

    switch (this._point) {
      case 0: this._point = 1; break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; this._line ? this._context.lineTo(this._x2, this._y2) : this._context.moveTo(this._x2, this._y2); break;
      case 3: this._point = 4; // falls through
      default: point$1(this, x, y); break;
    }

    this._l01_a = this._l12_a, this._l12_a = this._l23_a;
    this._l01_2a = this._l12_2a, this._l12_2a = this._l23_2a;
    this._x0 = this._x1, this._x1 = this._x2, this._x2 = x;
    this._y0 = this._y1, this._y1 = this._y2, this._y2 = y;
  }
};

const catmullRomOpen = (function custom(alpha) {

  function catmullRom(context) {
    return alpha ? new CatmullRomOpen(context, alpha) : new CardinalOpen(context, 0);
  }

  catmullRom.alpha = function(alpha) {
    return custom(+alpha);
  };

  return catmullRom;
})(0.5);

function LinearClosed(context) {
  this._context = context;
}

LinearClosed.prototype = {
  areaStart: noop,
  areaEnd: noop,
  lineStart: function() {
    this._point = 0;
  },
  lineEnd: function() {
    if (this._point) this._context.closePath();
  },
  point: function(x, y) {
    x = +x, y = +y;
    if (this._point) this._context.lineTo(x, y);
    else this._point = 1, this._context.moveTo(x, y);
  }
};

function linearClosed(context) {
  return new LinearClosed(context);
}

function sign(x) {
  return x < 0 ? -1 : 1;
}

// Calculate the slopes of the tangents (Hermite-type interpolation) based on
// the following paper: Steffen, M. 1990. A Simple Method for Monotonic
// Interpolation in One Dimension. Astronomy and Astrophysics, Vol. 239, NO.
// NOV(II), P. 443, 1990.
function slope3(that, x2, y2) {
  var h0 = that._x1 - that._x0,
      h1 = x2 - that._x1,
      s0 = (that._y1 - that._y0) / (h0 || h1 < 0 && -0),
      s1 = (y2 - that._y1) / (h1 || h0 < 0 && -0),
      p = (s0 * h1 + s1 * h0) / (h0 + h1);
  return (sign(s0) + sign(s1)) * Math.min(Math.abs(s0), Math.abs(s1), 0.5 * Math.abs(p)) || 0;
}

// Calculate a one-sided slope.
function slope2(that, t) {
  var h = that._x1 - that._x0;
  return h ? (3 * (that._y1 - that._y0) / h - t) / 2 : t;
}

// According to https://en.wikipedia.org/wiki/Cubic_Hermite_spline#Representations
// "you can express cubic Hermite interpolation in terms of cubic Bézier curves
// with respect to the four values p0, p0 + m0 / 3, p1 - m1 / 3, p1".
function point(that, t0, t1) {
  var x0 = that._x0,
      y0 = that._y0,
      x1 = that._x1,
      y1 = that._y1,
      dx = (x1 - x0) / 3;
  that._context.bezierCurveTo(x0 + dx, y0 + dx * t0, x1 - dx, y1 - dx * t1, x1, y1);
}

function MonotoneX(context) {
  this._context = context;
}

MonotoneX.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x0 = this._x1 =
    this._y0 = this._y1 =
    this._t0 = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    switch (this._point) {
      case 2: this._context.lineTo(this._x1, this._y1); break;
      case 3: point(this, this._t0, slope2(this, this._t0)); break;
    }
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    this._line = 1 - this._line;
  },
  point: function(x, y) {
    var t1 = NaN;

    x = +x, y = +y;
    if (x === this._x1 && y === this._y1) return; // Ignore coincident points.
    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; break;
      case 2: this._point = 3; point(this, slope2(this, t1 = slope3(this, x, y)), t1); break;
      default: point(this, this._t0, t1 = slope3(this, x, y)); break;
    }

    this._x0 = this._x1, this._x1 = x;
    this._y0 = this._y1, this._y1 = y;
    this._t0 = t1;
  }
};

function MonotoneY(context) {
  this._context = new ReflectContext(context);
}

(MonotoneY.prototype = Object.create(MonotoneX.prototype)).point = function(x, y) {
  MonotoneX.prototype.point.call(this, y, x);
};

function ReflectContext(context) {
  this._context = context;
}

ReflectContext.prototype = {
  moveTo: function(x, y) { this._context.moveTo(y, x); },
  closePath: function() { this._context.closePath(); },
  lineTo: function(x, y) { this._context.lineTo(y, x); },
  bezierCurveTo: function(x1, y1, x2, y2, x, y) { this._context.bezierCurveTo(y1, x1, y2, x2, y, x); }
};

function monotoneX(context) {
  return new MonotoneX(context);
}

function monotoneY(context) {
  return new MonotoneY(context);
}

function Natural(context) {
  this._context = context;
}

Natural.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x = [];
    this._y = [];
  },
  lineEnd: function() {
    var x = this._x,
        y = this._y,
        n = x.length;

    if (n) {
      this._line ? this._context.lineTo(x[0], y[0]) : this._context.moveTo(x[0], y[0]);
      if (n === 2) {
        this._context.lineTo(x[1], y[1]);
      } else {
        var px = controlPoints(x),
            py = controlPoints(y);
        for (var i0 = 0, i1 = 1; i1 < n; ++i0, ++i1) {
          this._context.bezierCurveTo(px[0][i0], py[0][i0], px[1][i0], py[1][i0], x[i1], y[i1]);
        }
      }
    }

    if (this._line || (this._line !== 0 && n === 1)) this._context.closePath();
    this._line = 1 - this._line;
    this._x = this._y = null;
  },
  point: function(x, y) {
    this._x.push(+x);
    this._y.push(+y);
  }
};

// See https://www.particleincell.com/2012/bezier-splines/ for derivation.
function controlPoints(x) {
  var i,
      n = x.length - 1,
      m,
      a = new Array(n),
      b = new Array(n),
      r = new Array(n);
  a[0] = 0, b[0] = 2, r[0] = x[0] + 2 * x[1];
  for (i = 1; i < n - 1; ++i) a[i] = 1, b[i] = 4, r[i] = 4 * x[i] + 2 * x[i + 1];
  a[n - 1] = 2, b[n - 1] = 7, r[n - 1] = 8 * x[n - 1] + x[n];
  for (i = 1; i < n; ++i) m = a[i] / b[i - 1], b[i] -= m, r[i] -= m * r[i - 1];
  a[n - 1] = r[n - 1] / b[n - 1];
  for (i = n - 2; i >= 0; --i) a[i] = (r[i] - a[i + 1]) / b[i];
  b[n - 1] = (x[n] + a[n - 1]) / 2;
  for (i = 0; i < n - 1; ++i) b[i] = 2 * x[i + 1] - a[i + 1];
  return [a, b];
}

function natural(context) {
  return new Natural(context);
}

function Step(context, t) {
  this._context = context;
  this._t = t;
}

Step.prototype = {
  areaStart: function() {
    this._line = 0;
  },
  areaEnd: function() {
    this._line = NaN;
  },
  lineStart: function() {
    this._x = this._y = NaN;
    this._point = 0;
  },
  lineEnd: function() {
    if (0 < this._t && this._t < 1 && this._point === 2) this._context.lineTo(this._x, this._y);
    if (this._line || (this._line !== 0 && this._point === 1)) this._context.closePath();
    if (this._line >= 0) this._t = 1 - this._t, this._line = 1 - this._line;
  },
  point: function(x, y) {
    x = +x, y = +y;
    switch (this._point) {
      case 0: this._point = 1; this._line ? this._context.lineTo(x, y) : this._context.moveTo(x, y); break;
      case 1: this._point = 2; // falls through
      default: {
        if (this._t <= 0) {
          this._context.lineTo(this._x, y);
          this._context.lineTo(x, y);
        } else {
          var x1 = this._x * (1 - this._t) + x * this._t;
          this._context.lineTo(x1, this._y);
          this._context.lineTo(x1, y);
        }
        break;
      }
    }
    this._x = x, this._y = y;
  }
};

function step(context) {
  return new Step(context, 0.5);
}

function stepBefore(context) {
  return new Step(context, 0);
}

function stepAfter(context) {
  return new Step(context, 1);
}

function none$1(series, order) {
  if (!((n = series.length) > 1)) return;
  for (var i = 1, j, s0, s1 = series[order[0]], n, m = s1.length; i < n; ++i) {
    s0 = s1, s1 = series[order[i]];
    for (j = 0; j < m; ++j) {
      s1[j][1] += s1[j][0] = isNaN(s0[j][1]) ? s0[j][0] : s0[j][1];
    }
  }
}

function none(series) {
  var n = series.length, o = new Array(n);
  while (--n >= 0) o[n] = n;
  return o;
}

function stackValue(d, key) {
  return d[key];
}

function stackSeries(key) {
  const series = [];
  series.key = key;
  return series;
}

function stack() {
  var keys = constant$1([]),
      order = none,
      offset = none$1,
      value = stackValue;

  function stack(data) {
    var sz = Array.from(keys.apply(this, arguments), stackSeries),
        i, n = sz.length, j = -1,
        oz;

    for (const d of data) {
      for (i = 0, ++j; i < n; ++i) {
        (sz[i][j] = [0, +value(d, sz[i].key, j, data)]).data = d;
      }
    }

    for (i = 0, oz = array(order(sz)); i < n; ++i) {
      sz[oz[i]].index = i;
    }

    offset(sz, oz);
    return sz;
  }

  stack.keys = function(_) {
    return arguments.length ? (keys = typeof _ === "function" ? _ : constant$1(Array.from(_)), stack) : keys;
  };

  stack.value = function(_) {
    return arguments.length ? (value = typeof _ === "function" ? _ : constant$1(+_), stack) : value;
  };

  stack.order = function(_) {
    return arguments.length ? (order = _ == null ? none : typeof _ === "function" ? _ : constant$1(Array.from(_)), stack) : order;
  };

  stack.offset = function(_) {
    return arguments.length ? (offset = _ == null ? none$1 : _, stack) : offset;
  };

  return stack;
}

function expand(series, order) {
  if (!((n = series.length) > 0)) return;
  for (var i, n, j = 0, m = series[0].length, y; j < m; ++j) {
    for (y = i = 0; i < n; ++i) y += series[i][j][1] || 0;
    if (y) for (i = 0; i < n; ++i) series[i][j][1] /= y;
  }
  none$1(series, order);
}

function diverging(series, order) {
  if (!((n = series.length) > 0)) return;
  for (var i, j = 0, d, dy, yp, yn, n, m = series[order[0]].length; j < m; ++j) {
    for (yp = yn = 0, i = 0; i < n; ++i) {
      if ((dy = (d = series[order[i]][j])[1] - d[0]) > 0) {
        d[0] = yp, d[1] = yp += dy;
      } else if (dy < 0) {
        d[1] = yn, d[0] = yn += dy;
      } else {
        d[0] = 0, d[1] = dy;
      }
    }
  }
}

function silhouette(series, order) {
  if (!((n = series.length) > 0)) return;
  for (var j = 0, s0 = series[order[0]], n, m = s0.length; j < m; ++j) {
    for (var i = 0, y = 0; i < n; ++i) y += series[i][j][1] || 0;
    s0[j][1] += s0[j][0] = -y / 2;
  }
  none$1(series, order);
}

function wiggle(series, order) {
  if (!((n = series.length) > 0) || !((m = (s0 = series[order[0]]).length) > 0)) return;
  for (var y = 0, j = 1, s0, m, n; j < m; ++j) {
    for (var i = 0, s1 = 0, s2 = 0; i < n; ++i) {
      var si = series[order[i]],
          sij0 = si[j][1] || 0,
          sij1 = si[j - 1][1] || 0,
          s3 = (sij0 - sij1) / 2;
      for (var k = 0; k < i; ++k) {
        var sk = series[order[k]],
            skj0 = sk[j][1] || 0,
            skj1 = sk[j - 1][1] || 0;
        s3 += skj0 - skj1;
      }
      s1 += sij0, s2 += s3 * sij0;
    }
    s0[j - 1][1] += s0[j - 1][0] = y;
    if (s1) y -= s2 / s1;
  }
  s0[j - 1][1] += s0[j - 1][0] = y;
  none$1(series, order);
}

function appearance(series) {
  var peaks = series.map(peak);
  return none(series).sort(function(a, b) { return peaks[a] - peaks[b]; });
}

function peak(series) {
  var i = -1, j = 0, n = series.length, vi, vj = -Infinity;
  while (++i < n) if ((vi = +series[i][1]) > vj) vj = vi, j = i;
  return j;
}

function ascending(series) {
  var sums = series.map(sum);
  return none(series).sort(function(a, b) { return sums[a] - sums[b]; });
}

function sum(series) {
  var s = 0, i = -1, n = series.length, v;
  while (++i < n) if (v = +series[i][1]) s += v;
  return s;
}

function descending(series) {
  return ascending(series).reverse();
}

function insideOut(series) {
  var n = series.length,
      i,
      j,
      sums = series.map(sum),
      order = appearance(series),
      top = 0,
      bottom = 0,
      tops = [],
      bottoms = [];

  for (i = 0; i < n; ++i) {
    j = order[i];
    if (top < bottom) {
      top += sums[j];
      tops.push(j);
    } else {
      bottom += sums[j];
      bottoms.push(j);
    }
  }

  return bottoms.reverse().concat(tops);
}

function reverse(series) {
  return none(series).reverse();
}

const constant = x => () => x;

function ZoomEvent(type, {
  sourceEvent,
  target,
  transform,
  dispatch
}) {
  Object.defineProperties(this, {
    type: {value: type, enumerable: true, configurable: true},
    sourceEvent: {value: sourceEvent, enumerable: true, configurable: true},
    target: {value: target, enumerable: true, configurable: true},
    transform: {value: transform, enumerable: true, configurable: true},
    _: {value: dispatch}
  });
}

function Transform(k, x, y) {
  this.k = k;
  this.x = x;
  this.y = y;
}

Transform.prototype = {
  constructor: Transform,
  scale: function(k) {
    return k === 1 ? this : new Transform(this.k * k, this.x, this.y);
  },
  translate: function(x, y) {
    return x === 0 & y === 0 ? this : new Transform(this.k, this.x + this.k * x, this.y + this.k * y);
  },
  apply: function(point) {
    return [point[0] * this.k + this.x, point[1] * this.k + this.y];
  },
  applyX: function(x) {
    return x * this.k + this.x;
  },
  applyY: function(y) {
    return y * this.k + this.y;
  },
  invert: function(location) {
    return [(location[0] - this.x) / this.k, (location[1] - this.y) / this.k];
  },
  invertX: function(x) {
    return (x - this.x) / this.k;
  },
  invertY: function(y) {
    return (y - this.y) / this.k;
  },
  rescaleX: function(x) {
    return x.copy().domain(x.range().map(this.invertX, this).map(x.invert, x));
  },
  rescaleY: function(y) {
    return y.copy().domain(y.range().map(this.invertY, this).map(y.invert, y));
  },
  toString: function() {
    return "translate(" + this.x + "," + this.y + ") scale(" + this.k + ")";
  }
};

var identity = new Transform(1, 0, 0);

transform.prototype = Transform.prototype;

function transform(node) {
  while (!node.__zoom) if (!(node = node.parentNode)) return identity;
  return node.__zoom;
}

function nopropagation(event) {
  event.stopImmediatePropagation();
}

function noevent(event) {
  event.preventDefault();
  event.stopImmediatePropagation();
}

// Ignore right-click, since that should open the context menu.
// except for pinch-to-zoom, which is sent as a wheel+ctrlKey event
function defaultFilter(event) {
  return (!event.ctrlKey || event.type === 'wheel') && !event.button;
}

function defaultExtent() {
  var e = this;
  if (e instanceof SVGElement) {
    e = e.ownerSVGElement || e;
    if (e.hasAttribute("viewBox")) {
      e = e.viewBox.baseVal;
      return [[e.x, e.y], [e.x + e.width, e.y + e.height]];
    }
    return [[0, 0], [e.width.baseVal.value, e.height.baseVal.value]];
  }
  return [[0, 0], [e.clientWidth, e.clientHeight]];
}

function defaultTransform() {
  return this.__zoom || identity;
}

function defaultWheelDelta(event) {
  return -event.deltaY * (event.deltaMode === 1 ? 0.05 : event.deltaMode ? 1 : 0.002) * (event.ctrlKey ? 10 : 1);
}

function defaultTouchable() {
  return navigator.maxTouchPoints || ("ontouchstart" in this);
}

function defaultConstrain(transform, extent, translateExtent) {
  var dx0 = transform.invertX(extent[0][0]) - translateExtent[0][0],
      dx1 = transform.invertX(extent[1][0]) - translateExtent[1][0],
      dy0 = transform.invertY(extent[0][1]) - translateExtent[0][1],
      dy1 = transform.invertY(extent[1][1]) - translateExtent[1][1];
  return transform.translate(
    dx1 > dx0 ? (dx0 + dx1) / 2 : Math.min(0, dx0) || Math.max(0, dx1),
    dy1 > dy0 ? (dy0 + dy1) / 2 : Math.min(0, dy0) || Math.max(0, dy1)
  );
}

function zoom() {
  var filter = defaultFilter,
      extent = defaultExtent,
      constrain = defaultConstrain,
      wheelDelta = defaultWheelDelta,
      touchable = defaultTouchable,
      scaleExtent = [0, Infinity],
      translateExtent = [[-Infinity, -Infinity], [Infinity, Infinity]],
      duration = 250,
      interpolate = interpolateZoom,
      listeners = dispatch("start", "zoom", "end"),
      touchstarting,
      touchfirst,
      touchending,
      touchDelay = 500,
      wheelDelay = 150,
      clickDistance2 = 0,
      tapDistance = 10;

  function zoom(selection) {
    selection
        .property("__zoom", defaultTransform)
        .on("wheel.zoom", wheeled, {passive: false})
        .on("mousedown.zoom", mousedowned)
        .on("dblclick.zoom", dblclicked)
      .filter(touchable)
        .on("touchstart.zoom", touchstarted)
        .on("touchmove.zoom", touchmoved)
        .on("touchend.zoom touchcancel.zoom", touchended)
        .style("-webkit-tap-highlight-color", "rgba(0,0,0,0)");
  }

  zoom.transform = function(collection, transform, point, event) {
    var selection = collection.selection ? collection.selection() : collection;
    selection.property("__zoom", defaultTransform);
    if (collection !== selection) {
      schedule(collection, transform, point, event);
    } else {
      selection.interrupt().each(function() {
        gesture(this, arguments)
          .event(event)
          .start()
          .zoom(null, typeof transform === "function" ? transform.apply(this, arguments) : transform)
          .end();
      });
    }
  };

  zoom.scaleBy = function(selection, k, p, event) {
    zoom.scaleTo(selection, function() {
      var k0 = this.__zoom.k,
          k1 = typeof k === "function" ? k.apply(this, arguments) : k;
      return k0 * k1;
    }, p, event);
  };

  zoom.scaleTo = function(selection, k, p, event) {
    zoom.transform(selection, function() {
      var e = extent.apply(this, arguments),
          t0 = this.__zoom,
          p0 = p == null ? centroid(e) : typeof p === "function" ? p.apply(this, arguments) : p,
          p1 = t0.invert(p0),
          k1 = typeof k === "function" ? k.apply(this, arguments) : k;
      return constrain(translate(scale(t0, k1), p0, p1), e, translateExtent);
    }, p, event);
  };

  zoom.translateBy = function(selection, x, y, event) {
    zoom.transform(selection, function() {
      return constrain(this.__zoom.translate(
        typeof x === "function" ? x.apply(this, arguments) : x,
        typeof y === "function" ? y.apply(this, arguments) : y
      ), extent.apply(this, arguments), translateExtent);
    }, null, event);
  };

  zoom.translateTo = function(selection, x, y, p, event) {
    zoom.transform(selection, function() {
      var e = extent.apply(this, arguments),
          t = this.__zoom,
          p0 = p == null ? centroid(e) : typeof p === "function" ? p.apply(this, arguments) : p;
      return constrain(identity.translate(p0[0], p0[1]).scale(t.k).translate(
        typeof x === "function" ? -x.apply(this, arguments) : -x,
        typeof y === "function" ? -y.apply(this, arguments) : -y
      ), e, translateExtent);
    }, p, event);
  };

  function scale(transform, k) {
    k = Math.max(scaleExtent[0], Math.min(scaleExtent[1], k));
    return k === transform.k ? transform : new Transform(k, transform.x, transform.y);
  }

  function translate(transform, p0, p1) {
    var x = p0[0] - p1[0] * transform.k, y = p0[1] - p1[1] * transform.k;
    return x === transform.x && y === transform.y ? transform : new Transform(transform.k, x, y);
  }

  function centroid(extent) {
    return [(+extent[0][0] + +extent[1][0]) / 2, (+extent[0][1] + +extent[1][1]) / 2];
  }

  function schedule(transition, transform, point, event) {
    transition
        .on("start.zoom", function() { gesture(this, arguments).event(event).start(); })
        .on("interrupt.zoom end.zoom", function() { gesture(this, arguments).event(event).end(); })
        .tween("zoom", function() {
          var that = this,
              args = arguments,
              g = gesture(that, args).event(event),
              e = extent.apply(that, args),
              p = point == null ? centroid(e) : typeof point === "function" ? point.apply(that, args) : point,
              w = Math.max(e[1][0] - e[0][0], e[1][1] - e[0][1]),
              a = that.__zoom,
              b = typeof transform === "function" ? transform.apply(that, args) : transform,
              i = interpolate(a.invert(p).concat(w / a.k), b.invert(p).concat(w / b.k));
          return function(t) {
            if (t === 1) t = b; // Avoid rounding error on end.
            else { var l = i(t), k = w / l[2]; t = new Transform(k, p[0] - l[0] * k, p[1] - l[1] * k); }
            g.zoom(null, t);
          };
        });
  }

  function gesture(that, args, clean) {
    return (!clean && that.__zooming) || new Gesture(that, args);
  }

  function Gesture(that, args) {
    this.that = that;
    this.args = args;
    this.active = 0;
    this.sourceEvent = null;
    this.extent = extent.apply(that, args);
    this.taps = 0;
  }

  Gesture.prototype = {
    event: function(event) {
      if (event) this.sourceEvent = event;
      return this;
    },
    start: function() {
      if (++this.active === 1) {
        this.that.__zooming = this;
        this.emit("start");
      }
      return this;
    },
    zoom: function(key, transform) {
      if (this.mouse && key !== "mouse") this.mouse[1] = transform.invert(this.mouse[0]);
      if (this.touch0 && key !== "touch") this.touch0[1] = transform.invert(this.touch0[0]);
      if (this.touch1 && key !== "touch") this.touch1[1] = transform.invert(this.touch1[0]);
      this.that.__zoom = transform;
      this.emit("zoom");
      return this;
    },
    end: function() {
      if (--this.active === 0) {
        delete this.that.__zooming;
        this.emit("end");
      }
      return this;
    },
    emit: function(type) {
      var d = select(this.that).datum();
      listeners.call(
        type,
        this.that,
        new ZoomEvent(type, {
          sourceEvent: this.sourceEvent,
          target: zoom,
          type,
          transform: this.that.__zoom,
          dispatch: listeners
        }),
        d
      );
    }
  };

  function wheeled(event, ...args) {
    if (!filter.apply(this, arguments)) return;
    var g = gesture(this, args).event(event),
        t = this.__zoom,
        k = Math.max(scaleExtent[0], Math.min(scaleExtent[1], t.k * Math.pow(2, wheelDelta.apply(this, arguments)))),
        p = pointer(event);

    // If the mouse is in the same location as before, reuse it.
    // If there were recent wheel events, reset the wheel idle timeout.
    if (g.wheel) {
      if (g.mouse[0][0] !== p[0] || g.mouse[0][1] !== p[1]) {
        g.mouse[1] = t.invert(g.mouse[0] = p);
      }
      clearTimeout(g.wheel);
    }

    // If this wheel event won’t trigger a transform change, ignore it.
    else if (t.k === k) return;

    // Otherwise, capture the mouse point and location at the start.
    else {
      g.mouse = [p, t.invert(p)];
      interrupt(this);
      g.start();
    }

    noevent(event);
    g.wheel = setTimeout(wheelidled, wheelDelay);
    g.zoom("mouse", constrain(translate(scale(t, k), g.mouse[0], g.mouse[1]), g.extent, translateExtent));

    function wheelidled() {
      g.wheel = null;
      g.end();
    }
  }

  function mousedowned(event, ...args) {
    if (touchending || !filter.apply(this, arguments)) return;
    var currentTarget = event.currentTarget,
        g = gesture(this, args, true).event(event),
        v = select(event.view).on("mousemove.zoom", mousemoved, true).on("mouseup.zoom", mouseupped, true),
        p = pointer(event, currentTarget),
        x0 = event.clientX,
        y0 = event.clientY;

    dragDisable(event.view);
    nopropagation(event);
    g.mouse = [p, this.__zoom.invert(p)];
    interrupt(this);
    g.start();

    function mousemoved(event) {
      noevent(event);
      if (!g.moved) {
        var dx = event.clientX - x0, dy = event.clientY - y0;
        g.moved = dx * dx + dy * dy > clickDistance2;
      }
      g.event(event)
       .zoom("mouse", constrain(translate(g.that.__zoom, g.mouse[0] = pointer(event, currentTarget), g.mouse[1]), g.extent, translateExtent));
    }

    function mouseupped(event) {
      v.on("mousemove.zoom mouseup.zoom", null);
      yesdrag(event.view, g.moved);
      noevent(event);
      g.event(event).end();
    }
  }

  function dblclicked(event, ...args) {
    if (!filter.apply(this, arguments)) return;
    var t0 = this.__zoom,
        p0 = pointer(event.changedTouches ? event.changedTouches[0] : event, this),
        p1 = t0.invert(p0),
        k1 = t0.k * (event.shiftKey ? 0.5 : 2),
        t1 = constrain(translate(scale(t0, k1), p0, p1), extent.apply(this, args), translateExtent);

    noevent(event);
    if (duration > 0) select(this).transition().duration(duration).call(schedule, t1, p0, event);
    else select(this).call(zoom.transform, t1, p0, event);
  }

  function touchstarted(event, ...args) {
    if (!filter.apply(this, arguments)) return;
    var touches = event.touches,
        n = touches.length,
        g = gesture(this, args, event.changedTouches.length === n).event(event),
        started, i, t, p;

    nopropagation(event);
    for (i = 0; i < n; ++i) {
      t = touches[i], p = pointer(t, this);
      p = [p, this.__zoom.invert(p), t.identifier];
      if (!g.touch0) g.touch0 = p, started = true, g.taps = 1 + !!touchstarting;
      else if (!g.touch1 && g.touch0[2] !== p[2]) g.touch1 = p, g.taps = 0;
    }

    if (touchstarting) touchstarting = clearTimeout(touchstarting);

    if (started) {
      if (g.taps < 2) touchfirst = p[0], touchstarting = setTimeout(function() { touchstarting = null; }, touchDelay);
      interrupt(this);
      g.start();
    }
  }

  function touchmoved(event, ...args) {
    if (!this.__zooming) return;
    var g = gesture(this, args).event(event),
        touches = event.changedTouches,
        n = touches.length, i, t, p, l;

    noevent(event);
    for (i = 0; i < n; ++i) {
      t = touches[i], p = pointer(t, this);
      if (g.touch0 && g.touch0[2] === t.identifier) g.touch0[0] = p;
      else if (g.touch1 && g.touch1[2] === t.identifier) g.touch1[0] = p;
    }
    t = g.that.__zoom;
    if (g.touch1) {
      var p0 = g.touch0[0], l0 = g.touch0[1],
          p1 = g.touch1[0], l1 = g.touch1[1],
          dp = (dp = p1[0] - p0[0]) * dp + (dp = p1[1] - p0[1]) * dp,
          dl = (dl = l1[0] - l0[0]) * dl + (dl = l1[1] - l0[1]) * dl;
      t = scale(t, Math.sqrt(dp / dl));
      p = [(p0[0] + p1[0]) / 2, (p0[1] + p1[1]) / 2];
      l = [(l0[0] + l1[0]) / 2, (l0[1] + l1[1]) / 2];
    }
    else if (g.touch0) p = g.touch0[0], l = g.touch0[1];
    else return;

    g.zoom("touch", constrain(translate(t, p, l), g.extent, translateExtent));
  }

  function touchended(event, ...args) {
    if (!this.__zooming) return;
    var g = gesture(this, args).event(event),
        touches = event.changedTouches,
        n = touches.length, i, t;

    nopropagation(event);
    if (touchending) clearTimeout(touchending);
    touchending = setTimeout(function() { touchending = null; }, touchDelay);
    for (i = 0; i < n; ++i) {
      t = touches[i];
      if (g.touch0 && g.touch0[2] === t.identifier) delete g.touch0;
      else if (g.touch1 && g.touch1[2] === t.identifier) delete g.touch1;
    }
    if (g.touch1 && !g.touch0) g.touch0 = g.touch1, delete g.touch1;
    if (g.touch0) g.touch0[1] = this.__zoom.invert(g.touch0[0]);
    else {
      g.end();
      // If this was a dbltap, reroute to the (optional) dblclick.zoom handler.
      if (g.taps === 2) {
        t = pointer(t, this);
        if (Math.hypot(touchfirst[0] - t[0], touchfirst[1] - t[1]) < tapDistance) {
          var p = select(this).on("dblclick.zoom");
          if (p) p.apply(this, arguments);
        }
      }
    }
  }

  zoom.wheelDelta = function(_) {
    return arguments.length ? (wheelDelta = typeof _ === "function" ? _ : constant(+_), zoom) : wheelDelta;
  };

  zoom.filter = function(_) {
    return arguments.length ? (filter = typeof _ === "function" ? _ : constant(!!_), zoom) : filter;
  };

  zoom.touchable = function(_) {
    return arguments.length ? (touchable = typeof _ === "function" ? _ : constant(!!_), zoom) : touchable;
  };

  zoom.extent = function(_) {
    return arguments.length ? (extent = typeof _ === "function" ? _ : constant([[+_[0][0], +_[0][1]], [+_[1][0], +_[1][1]]]), zoom) : extent;
  };

  zoom.scaleExtent = function(_) {
    return arguments.length ? (scaleExtent[0] = +_[0], scaleExtent[1] = +_[1], zoom) : [scaleExtent[0], scaleExtent[1]];
  };

  zoom.translateExtent = function(_) {
    return arguments.length ? (translateExtent[0][0] = +_[0][0], translateExtent[1][0] = +_[1][0], translateExtent[0][1] = +_[0][1], translateExtent[1][1] = +_[1][1], zoom) : [[translateExtent[0][0], translateExtent[0][1]], [translateExtent[1][0], translateExtent[1][1]]];
  };

  zoom.constrain = function(_) {
    return arguments.length ? (constrain = _, zoom) : constrain;
  };

  zoom.duration = function(_) {
    return arguments.length ? (duration = +_, zoom) : duration;
  };

  zoom.interpolate = function(_) {
    return arguments.length ? (interpolate = _, zoom) : interpolate;
  };

  zoom.on = function() {
    var value = listeners.on.apply(listeners, arguments);
    return value === listeners ? zoom : value;
  };

  zoom.clickDistance = function(_) {
    return arguments.length ? (clickDistance2 = (_ = +_) * _, zoom) : Math.sqrt(clickDistance2);
  };

  zoom.tapDistance = function(_) {
    return arguments.length ? (tapDistance = +_, zoom) : tapDistance;
  };

  return zoom;
}

var ResourceStatus = /* @__PURE__ */ ((ResourceStatus2) => {
  ResourceStatus2["Unknown"] = "Unknown";
  ResourceStatus2["Ready"] = "Ready";
  ResourceStatus2["Warning"] = "Warning";
  ResourceStatus2["Error"] = "Error";
  ResourceStatus2["Terminating"] = "Terminating";
  return ResourceStatus2;
})(ResourceStatus || {});

const STATUS_COLORS = {
  [ResourceStatus.Ready]: "#10b981",
  // emerald-500
  [ResourceStatus.Warning]: "#f59e0b",
  // amber-500
  [ResourceStatus.Error]: "#ef4444",
  // red-500
  [ResourceStatus.Terminating]: "#6b7280",
  // gray-500
  [ResourceStatus.Unknown]: "#374151"
  // gray-700
};
const MOCK_NAMESPACES = ["default", "kube-system", "production", "staging", "monitoring"];
const MOCK_KINDS = ["Deployment", "Pod", "Service", "ConfigMap", "Ingress"];

const MARGIN = { top: 40, right: 30, bottom: 20, left: 240 };
const ROW_HEIGHT_DEFAULT = 48;
const ROW_HEIGHT_COMPACT = 24;
const Timeline = ({
  resources,
  onSegmentClick,
  selectedPoint,
  highlightedEventIds = [],
  sidebarWidth = 0,
  timeRange,
  onVisibleTimeRangeChange,
  onZoomDetected,
  hasMore = false,
  loadingMore = false,
  onLoadMore
}) => {
  const svgRef = reactExports.useRef(null);
  const stickyAxisRef = reactExports.useRef(null);
  const containerRef = reactExports.useRef(null);
  const [containerSize, setContainerSize] = reactExports.useState({ width: 0, height: 0 });
  const [tooltip, setTooltip] = reactExports.useState(null);
  const { compactMode, formatTime, theme } = useSettings();
  const debounceTimeoutRef = reactExports.useRef(null);
  const rowHeight = compactMode ? ROW_HEIGHT_COMPACT : ROW_HEIGHT_DEFAULT;
  const bandPadding = compactMode ? 0.15 : 0.4;
  const themeColors = reactExports.useMemo(() => {
    if (typeof window === "undefined") {
      return {
        sidebar: "#0d1117",
        border: "#374151",
        textPrimary: "#f8fafc",
        textMuted: "#94a3b8",
        grid: "#334155",
        appBg: "#0f172a",
        surfaceMuted: "#1f2937"
      };
    }
    const styles = getComputedStyle(document.documentElement);
    return {
      sidebar: styles.getPropertyValue("--color-sidebar-bg").trim() || "#0d1117",
      border: styles.getPropertyValue("--color-border-soft").trim() || "#374151",
      textPrimary: styles.getPropertyValue("--color-text-primary").trim() || "#f8fafc",
      textMuted: styles.getPropertyValue("--color-text-muted").trim() || "#94a3b8",
      grid: styles.getPropertyValue("--color-grid-line").trim() || "#334155",
      appBg: styles.getPropertyValue("--color-app-bg").trim() || "#0f172a",
      surfaceMuted: styles.getPropertyValue("--color-surface-muted").trim() || "#1f2937"
    };
  }, [theme]);
  const resizeTimeoutRef = reactExports.useRef(null);
  const lastSizeRef = reactExports.useRef({ width: 0, height: 0 });
  const isInitializedRef = reactExports.useRef(false);
  reactExports.useEffect(() => {
    if (!containerRef.current) return;
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width: width2, height: height2 } = entry.contentRect;
        if (!isInitializedRef.current) {
          isInitializedRef.current = true;
          lastSizeRef.current = { width: width2, height: height2 };
          setContainerSize({ width: width2, height: height2 });
          return;
        }
        const heightDiff = Math.abs(height2 - lastSizeRef.current.height);
        if (heightDiff < 1) return;
        if (resizeTimeoutRef.current) {
          clearTimeout(resizeTimeoutRef.current);
        }
        resizeTimeoutRef.current = setTimeout(() => {
          lastSizeRef.current = { width: lastSizeRef.current.width, height: height2 };
          setContainerSize((prev) => ({ ...prev, height: height2 }));
          resizeTimeoutRef.current = null;
        }, 50);
      }
    });
    resizeObserver.observe(containerRef.current);
    return () => {
      resizeObserver.disconnect();
      if (resizeTimeoutRef.current) {
        clearTimeout(resizeTimeoutRef.current);
      }
    };
  }, []);
  reactExports.useEffect(() => {
    if (!containerRef.current || !hasMore || !onLoadMore) return;
    const container = containerRef.current;
    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container;
      const scrolledToBottom = scrollHeight - scrollTop - clientHeight < 200;
      if (scrolledToBottom && !loadingMore) {
        onLoadMore();
      }
    };
    container.addEventListener("scroll", handleScroll);
    return () => container.removeEventListener("scroll", handleScroll);
  }, [hasMore, loadingMore, onLoadMore]);
  const width = containerSize.width || 800;
  const height = containerSize.height || 600;
  const timeDomain = reactExports.useMemo(() => {
    const duration = timeRange.end.getTime() - timeRange.start.getTime();
    const padding = duration * 0.02;
    return [
      new Date(timeRange.start.getTime() - padding),
      new Date(timeRange.end.getTime() + padding)
    ];
  }, [timeRange]);
  const prevDomain = reactExports.useRef(timeDomain);
  const sortedResources = resources;
  const innerWidth = width - MARGIN.left - MARGIN.right;
  const contentHeight = sortedResources.length * rowHeight;
  const minInnerHeight = height - MARGIN.top - MARGIN.bottom;
  const innerHeight = Math.max(minInnerHeight, contentHeight);
  const xScale = reactExports.useMemo(() => time().domain(timeDomain).range([0, innerWidth]), [timeDomain, innerWidth]);
  const yScale = reactExports.useMemo(() => band().domain(sortedResources.map((r) => r.id)).range([0, contentHeight]).padding(bandPadding), [sortedResources, contentHeight, bandPadding]);
  const zoom$1 = reactExports.useMemo(() => zoom().scaleExtent([1, 3e4]).translateExtent([[0, 0], [innerWidth, height]]).extent([[0, 0], [innerWidth, height]]), [innerWidth, height]);
  const notifyVisibleTimeRange = (transform) => {
    if (!onVisibleTimeRangeChange) return;
    if (debounceTimeoutRef.current) {
      clearTimeout(debounceTimeoutRef.current);
    }
    debounceTimeoutRef.current = setTimeout(() => {
      const rescaledXScale = transform.rescaleX(xScale);
      const visibleStart = rescaledXScale.invert(0);
      const visibleEnd = rescaledXScale.invert(innerWidth);
      onVisibleTimeRangeChange({ start: visibleStart, end: visibleEnd });
      debounceTimeoutRef.current = null;
    }, 300);
  };
  reactExports.useEffect(() => {
    return () => {
      if (debounceTimeoutRef.current) {
        clearTimeout(debounceTimeoutRef.current);
      }
    };
  }, []);
  reactExports.useEffect(() => {
    if (!svgRef.current || sortedResources.length === 0) return;
    const svg = select(svgRef.current);
    const domainChanged = prevDomain.current !== timeDomain;
    prevDomain.current = timeDomain;
    let currentTransform = identity;
    if (!domainChanged) {
      currentTransform = transform(svg.node());
    }
    svg.selectAll("*").remove();
    const defs = svg.append("defs");
    defs.append("clipPath").attr("id", "chart-clip").append("rect").attr("x", 0).attr("y", 0).attr("width", innerWidth).attr("height", innerHeight);
    const mainGroup = svg.append("g").attr("transform", `translate(${MARGIN.left},${MARGIN.top})`);
    const gridGroup = mainGroup.append("g").attr("class", "grid-group").attr("clip-path", "url(#chart-clip)");
    const contentGroup = mainGroup.append("g").attr("class", "content-group").attr("clip-path", "url(#chart-clip)");
    const brushGroup = mainGroup.append("g").attr("class", "brush");
    const labelGroup = svg.append("g").attr("transform", `translate(0, ${MARGIN.top})`);
    const styles = getComputedStyle(document.documentElement);
    const sidebarBg = styles.getPropertyValue("--color-sidebar-bg").trim() || themeColors.sidebar;
    const borderColor = styles.getPropertyValue("--color-border-soft").trim() || themeColors.border;
    labelGroup.append("rect").attr("data-element", "sidebar-bg").attr("width", MARGIN.left - 2).attr("height", Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom)).attr("y", -MARGIN.top).attr("fill", sidebarBg).attr("stroke", "none");
    labelGroup.append("line").attr("data-element", "sidebar-border").attr("x1", MARGIN.left - 1).attr("x2", MARGIN.left - 1).attr("y1", -MARGIN.top).attr("y2", Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom)).attr("stroke", borderColor).attr("stroke-width", 1);
    const labels = labelGroup.selectAll(".label").data(sortedResources).enter().append("g").attr("class", "label cursor-pointer group").attr("transform", (d) => `translate(20, ${yScale(d.id)})`).on("click", (event, d) => {
      if (d.statusSegments.length > 0) {
        onSegmentClick(d, d.statusSegments.length - 1);
      }
    });
    labels.append("rect").attr("x", -20).attr("y", 0).attr("width", MARGIN.left - 2).attr("height", yScale.bandwidth()).attr("fill", "transparent");
    const maxTextWidth = MARGIN.left - 40;
    const textPrimary = getComputedStyle(document.documentElement).getPropertyValue("--color-text-primary").trim() || themeColors.textPrimary;
    const textMuted = getComputedStyle(document.documentElement).getPropertyValue("--color-text-muted").trim() || themeColors.textMuted;
    const nameFontSize = compactMode ? "10px" : "13px";
    const kindFontSize = compactMode ? "8px" : "11px";
    const nameY = compactMode ? yScale.bandwidth() / 2 - 2 : yScale.bandwidth() / 2 - 6;
    const kindY = compactMode ? yScale.bandwidth() / 2 + 6 : yScale.bandwidth() / 2 + 10;
    const nameText = labels.append("text").attr("data-element", "label-name").attr("x", 0).attr("y", nameY).attr("fill", textPrimary).style("font-size", nameFontSize).style("font-weight", "600").style("dominant-baseline", "middle").each(function(d) {
      const text = select(this);
      const textNode = this;
      const fullName = d.name;
      text.text(fullName);
      const bbox = textNode.getBBox();
      if (bbox.width > maxTextWidth) {
        let truncated = fullName;
        while (truncated.length > 0) {
          text.text(truncated + "...");
          const newBbox = textNode.getBBox();
          if (newBbox.width <= maxTextWidth || truncated.length <= 1) {
            break;
          }
          truncated = truncated.slice(0, -1);
        }
      }
    });
    nameText.append("title").text((d) => d.name);
    labels.append("text").attr("data-element", "label-kind-ns").text((d) => `${d.kind} • ${d.namespace}${d.preExisting ? " • Pre-existing" : ""}`).attr("x", 0).attr("y", kindY).attr("fill", textMuted).style("font-size", kindFontSize).style("dominant-baseline", "middle");
    labels.append("line").attr("x1", 0).attr("x2", MARGIN.left - 40).attr("y1", yScale.bandwidth() + yScale.padding() * yScale.step() / 2).attr("y2", yScale.bandwidth() + yScale.padding() * yScale.step() / 2).attr("stroke", borderColor).attr("stroke-width", 1);
    const rows = contentGroup.selectAll(".resource-row").data(sortedResources).enter().append("g").attr("class", "resource-row").attr("transform", (d) => `translate(0, ${yScale(d.id)})`);
    rows.append("line").attr("class", "row-guide").attr("x1", -1e4).attr("x2", 1e4).attr("y1", yScale.bandwidth() / 2).attr("y2", yScale.bandwidth() / 2).attr("stroke", themeColors.grid).attr("stroke-dasharray", "4,4").attr("opacity", 0.5);
    rows.selectAll(".segment").data((d) => d.statusSegments.map((s, i) => ({ ...s, resourceId: d.id, index: i, preExisting: d.preExisting }))).enter().append("rect").attr("class", "segment").style("transition", "stroke 0.2s, stroke-width 0.2s").attr("y", 0).attr("height", yScale.bandwidth()).attr("rx", 4).attr("fill", (s) => STATUS_COLORS[s.status]).attr("opacity", (s) => s.preExisting ? 0.5 : 1).attr("stroke", "none").attr("stroke-width", 0);
    const eventDots = rows.selectAll(".event-dot").data((d) => d.events.map((e) => ({ ...e, resourceId: d.id }))).enter().append("circle").attr("class", "event-dot").attr("cy", yScale.bandwidth() / 2).attr("r", 5).attr("fill", "#f8fafc").attr("stroke", "#1e293b").attr("stroke-width", 2).style("cursor", "pointer").style("pointer-events", "all").on("mouseenter", function(event, d) {
      const time = formatTime(d.timestamp);
      const typeLabel = d.type === "Warning" ? "⚠️ Warning" : d.type === "Normal" ? "✓ Normal" : d.type;
      const content = `${time}
${typeLabel}: ${d.reason}
${d.message}`;
      const [x, y] = pointer(event, containerRef.current);
      setTooltip({ x, y, content });
    }).on("mousemove", function(event) {
      const [x, y] = pointer(event, containerRef.current);
      setTooltip((prev) => prev ? { ...prev, x, y } : null);
    }).on("mouseleave", () => {
      setTooltip(null);
    });
    const updateChart = (transform) => {
      const newXScale = transform.rescaleX(xScale);
      const axis = axisTop(newXScale).ticks(Math.max(width / 120, 2)).tickSizeOuter(0).tickFormat((date) => formatTime(date));
      if (stickyAxisRef.current) {
        const stickyAxis = select(stickyAxisRef.current);
        const stickyAxisGroup = stickyAxis.select("g#sticky-axis");
        stickyAxisGroup.call(axis).call((g) => g.select(".domain").remove()).call((g) => g.selectAll(".tick line").attr("stroke", themeColors.grid).attr("stroke-width", 2).attr("y2", -5)).call(
          (g) => g.selectAll(".tick text").attr("fill", themeColors.textPrimary).attr("font-weight", "600").attr("font-size", "12px").attr("dy", "-8px")
        );
      }
      const gridAxis = axisBottom(newXScale).ticks(Math.max(width / 120, 2)).tickSize(innerHeight).tickFormat(() => "");
      gridGroup.call(gridAxis).call((g) => g.select(".domain").remove()).call(
        (g) => g.selectAll(".tick line").attr("stroke", themeColors.grid).attr("stroke-opacity", 0.5).attr("stroke-dasharray", "2,2")
      );
      contentGroup.selectAll(".segment").attr("x", (d) => newXScale(d.start)).attr("width", (d) => {
        const calculatedWidth = newXScale(d.end) - newXScale(d.start);
        if (d.status === "Terminating") {
          return 2;
        }
        return Math.max(4, calculatedWidth);
      });
      contentGroup.selectAll(".event-dot").attr("cx", (d) => newXScale(d.timestamp));
    };
    zoom$1.on("zoom", (event) => {
      updateChart(event.transform);
      notifyVisibleTimeRange(event.transform);
      onZoomDetected?.();
    });
    svg.call(zoom$1).on("wheel.zoom", null).on("mousedown.zoom", null).on("dblclick.zoom", () => {
      svg.transition().duration(750).call(zoom$1.transform, identity);
    });
    svg.on("wheel", (event) => {
      const isHorizontal = Math.abs(event.deltaX) > Math.abs(event.deltaY);
      if (isHorizontal) {
        event.preventDefault();
        const t = transform(svg.node());
        const dampingFactor = Math.max(1e-5, 1 / t.k);
        const dampedDelta = event.deltaX * dampingFactor;
        const maxDelta = 50;
        const finalDelta = Math.max(-maxDelta, Math.min(maxDelta, dampedDelta));
        zoom$1.translateBy(svg, -finalDelta, 0);
      }
    }, { passive: false });
    const brush = brushX().extent([[0, 0], [innerWidth, innerHeight]]).on("end", (event) => {
      if (!event.selection) return;
      const [x0, x1] = event.selection;
      brushGroup.call(brush.move, null);
      const t = transform(svg.node());
      const currentXScale = t.rescaleX(xScale);
      const x0_orig = currentXScale.invert(x0);
      const x1_orig = currentXScale.invert(x1);
      const dx = xScale(x1_orig) - xScale(x0_orig);
      const k = innerWidth / dx;
      const tx = -xScale(x0_orig) * k;
      const newTransform = identity.translate(tx, 0).scale(k);
      svg.transition().duration(750).call(zoom$1.transform, newTransform).on("end", () => {
        notifyVisibleTimeRange(newTransform);
      });
    });
    brushGroup.call(brush);
    brushGroup.select(".overlay").on("click", (event) => {
      const [x, y] = pointer(event);
      const eachBand = yScale.step();
      const index = Math.floor(y / eachBand);
      const resource = sortedResources[index];
      if (!resource) return;
      const bandTop = yScale(resource.id) || 0;
      const bandHeight = yScale.bandwidth();
      if (y < bandTop || y > bandTop + bandHeight) return;
      const t = transform(svg.node());
      const currentXScale = t.rescaleX(xScale);
      const clickedTime = currentXScale.invert(x);
      const segmentIndex = resource.statusSegments.findIndex(
        (s) => clickedTime >= s.start && clickedTime <= s.end
      );
      if (segmentIndex !== -1) {
        onSegmentClick(resource, segmentIndex);
      }
    }).on("mousemove", function(event) {
      const [x, y] = pointer(event);
      const eachBand = yScale.step();
      const index = Math.floor(y / eachBand);
      const resource = sortedResources[index];
      let isOverSegment = false;
      let isOverEvent = false;
      if (resource) {
        const bandTop = yScale(resource.id) || 0;
        const bandHeight = yScale.bandwidth();
        if (y >= bandTop && y <= bandTop + bandHeight) {
          const t = transform(svg.node());
          const currentXScale = t.rescaleX(xScale);
          const hoverTime = currentXScale.invert(x);
          isOverSegment = resource.statusSegments.some(
            (s) => hoverTime >= s.start && hoverTime <= s.end
          );
          const eventRadius = 8;
          const eventDot = resource.events.find((e) => {
            const eventX = currentXScale(e.timestamp);
            const eventY = bandTop + bandHeight / 2;
            const dx = x - eventX;
            const dy = y - eventY;
            return Math.sqrt(dx * dx + dy * dy) <= eventRadius;
          });
          if (eventDot) {
            isOverEvent = true;
            const time = formatTime(eventDot.timestamp);
            const typeLabel = eventDot.type === "Warning" ? "⚠️ Warning" : eventDot.type === "Normal" ? "✓ Normal" : eventDot.type;
            const content = `${time}
${typeLabel}: ${eventDot.reason}
${eventDot.message}`;
            const [absX, absY] = pointer(event, containerRef.current);
            setTooltip({ x: absX, y: absY, content });
          } else {
            setTooltip(null);
          }
        } else {
          setTooltip(null);
        }
      } else {
        setTooltip(null);
      }
      select(this).style("cursor", isOverSegment || isOverEvent ? "pointer" : "crosshair");
    }).on("mouseleave", () => {
      setTooltip(null);
    });
    svg.call(zoom$1.transform, currentTransform);
  }, [sortedResources, width, height, timeDomain, themeColors, theme, rowHeight, bandPadding]);
  reactExports.useEffect(() => {
    if (!svgRef.current) return;
    const frame = requestAnimationFrame(() => {
      const svg = select(svgRef.current);
      const styles = getComputedStyle(document.documentElement);
      const sidebarBg = styles.getPropertyValue("--color-sidebar-bg").trim();
      const borderColor = styles.getPropertyValue("--color-border-soft").trim();
      const textPrimary = styles.getPropertyValue("--color-text-primary").trim();
      const textMuted = styles.getPropertyValue("--color-text-muted").trim();
      const gridLine = styles.getPropertyValue("--color-grid-line").trim();
      const appBg = styles.getPropertyValue("--color-app-bg").trim();
      svg.selectAll('[data-element="sidebar-bg"]').attr("fill", sidebarBg || themeColors.sidebar);
      svg.selectAll('[data-element="sidebar-border"]').attr("stroke", borderColor || themeColors.border);
      svg.selectAll('[data-element="label-name"]').attr("fill", textPrimary || themeColors.textPrimary);
      svg.selectAll('[data-element="label-kind-ns"]').attr("fill", textMuted || themeColors.textMuted);
      svg.selectAll(".tick line").attr("stroke", gridLine || themeColors.grid);
      svg.selectAll(".tick text").attr("fill", textPrimary || themeColors.textPrimary);
      if (stickyAxisRef.current) {
        select(stickyAxisRef.current).selectAll(".tick text").attr("fill", textPrimary || themeColors.textPrimary);
      }
      svg.selectAll(".row-guide").attr("stroke", borderColor || themeColors.border);
      svg.selectAll(".event-dot").attr("stroke", appBg || themeColors.appBg);
    });
    return () => cancelAnimationFrame(frame);
  }, [theme]);
  reactExports.useEffect(() => {
    if (!svgRef.current) return;
    const svg = select(svgRef.current);
    const outlineColor = theme === "light" ? "#1e293b" : "#ffffff";
    svg.selectAll(".segment").attr("stroke", (d) => {
      const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
      return isSel ? outlineColor : "none";
    }).attr("stroke-width", (d) => {
      const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
      return isSel ? 3 : 0;
    });
    svg.selectAll(".event-dot").attr("fill", (d) => highlightedEventIds.includes(d.id) ? "#fbbf24" : "#f8fafc").attr("r", (d) => highlightedEventIds.includes(d.id) ? 7 : 5).attr("stroke", (d) => highlightedEventIds.includes(d.id) ? "#ffffff" : "#1e293b").attr("stroke-width", (d) => highlightedEventIds.includes(d.id) ? 2 : 2).filter((d) => highlightedEventIds.includes(d.id)).raise();
  }, [selectedPoint, highlightedEventIds, themeColors, theme, sortedResources]);
  const prevSidebarWidth = reactExports.useRef(0);
  reactExports.useEffect(() => {
    if (!svgRef.current || width === 0) return;
    const wasPanelOpen = prevSidebarWidth.current > 0;
    const isPanelClosing = wasPanelOpen && sidebarWidth === 0;
    if (isPanelClosing) {
      const svg = select(svgRef.current);
      const currentTransform = transform(svg.node());
      const currentXScale = currentTransform.rescaleX(xScale);
      const oldInnerWidth = innerWidth;
      const visibleRightX = oldInnerWidth - MARGIN.right;
      const rightEdgeTime = currentXScale.invert(visibleRightX);
      setTimeout(() => {
        if (!svgRef.current || !containerRef.current) return;
        const svg2 = select(svgRef.current);
        const updatedTransform = transform(svg2.node());
        const newWidth = containerRef.current.clientWidth;
        const newInnerWidth = newWidth - MARGIN.left - MARGIN.right;
        const newBaseXScale = time().domain(timeDomain).range([0, newInnerWidth]);
        const rightTimeX = newBaseXScale(rightEdgeTime);
        const k = updatedTransform.k;
        const targetX = newInnerWidth - MARGIN.right;
        const newTx = targetX - rightTimeX * k;
        const maxTx = 0;
        const minTx = newInnerWidth - newInnerWidth * k;
        const finalTx = Math.min(maxTx, Math.max(minTx, newTx));
        svg2.transition().duration(400).ease(cubicOut).call(zoom$1.transform, identity.translate(finalTx, 0).scale(k));
      }, 50);
    }
    prevSidebarWidth.current = sidebarWidth;
  }, [sidebarWidth, width, zoom$1, timeDomain, xScale, innerWidth]);
  reactExports.useEffect(() => {
    if (!svgRef.current) return;
    const svg = select(svgRef.current);
    const currentTransform = transform(svg.node());
    if (selectedPoint) {
      const resource = sortedResources.find((r) => r.id === selectedPoint.resourceId);
      if (resource) {
        const segment = resource.statusSegments[selectedPoint.index];
        if (segment) {
          const startTime = segment.start;
          const endTime = segment.end;
          const midTime = new Date((startTime.getTime() + endTime.getTime()) / 2);
          const visibleLeft = MARGIN.left;
          const visibleRight = width - sidebarWidth;
          const availableWidth = Math.max(0, visibleRight - visibleLeft);
          const padding = availableWidth * 0.1;
          const maxSegmentWidth = availableWidth - padding;
          const x0 = xScale(startTime);
          const x1 = xScale(endTime);
          const segmentWidthUnzoomed = x1 - x0;
          let k = currentTransform.k;
          const projectedWidth = segmentWidthUnzoomed * k;
          if (projectedWidth > maxSegmentWidth && segmentWidthUnzoomed > 0) {
            k = maxSegmentWidth / segmentWidthUnzoomed;
          }
          const targetCenterX = visibleLeft + availableWidth / 2;
          const pointOriginalX = xScale(midTime);
          const newTx = targetCenterX - MARGIN.left - pointOriginalX * k;
          svg.transition().duration(500).ease(cubicOut).call(zoom$1.transform, identity.translate(newTx, 0).scale(k));
        }
      }
    }
    prevSidebarWidth.current = sidebarWidth;
  }, [selectedPoint, sidebarWidth, sortedResources, width, xScale, zoom$1, timeRange]);
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "div",
    {
      ref: containerRef,
      className: "w-full h-full bg-[var(--color-surface-secondary)] rounded-lg shadow-inner border border-[var(--color-border-soft)] custom-scrollbar relative overflow-y-auto overflow-x-hidden transition-colors duration-300",
      children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "svg",
          {
            ref: stickyAxisRef,
            width,
            height: MARGIN.top,
            className: "sticky top-0 left-0 right-0 block bg-[var(--color-surface-secondary)] border-b border-[var(--color-border-soft)] z-10",
            style: { pointerEvents: "none" },
            children: /* @__PURE__ */ jsxRuntimeExports.jsx(
              "g",
              {
                id: "sticky-axis",
                transform: `translate(${MARGIN.left},${MARGIN.top})`
              }
            )
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "fixed flex gap-2 z-[10]", style: { top: `120px`, right: "10px" }, children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "button",
            {
              onClick: () => {
                if (!svgRef.current) return;
                const svg = select(svgRef.current);
                const currentTransform = transform(svg.node());
                const newScale = Math.max(1, currentTransform.k * 0.8);
                const centerX = innerWidth / 2;
                const newTx = centerX - (centerX - currentTransform.x) * (newScale / currentTransform.k);
                const newTransform = identity.translate(newTx, 0).scale(newScale);
                svg.transition().duration(300).ease(cubicOut).call(zoom$1.transform, newTransform).on("end", () => {
                  notifyVisibleTimeRange(newTransform);
                  onZoomDetected?.();
                });
              },
              title: "Zoom Out",
              className: "p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM13 10H7" }) })
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "button",
            {
              onClick: () => {
                if (!svgRef.current) return;
                const svg = select(svgRef.current);
                const currentTransform = transform(svg.node());
                const newScale = Math.min(3e4, currentTransform.k * 1.25);
                const centerX = innerWidth / 2;
                const newTx = centerX - (centerX - currentTransform.x) * (newScale / currentTransform.k);
                const newTransform = identity.translate(newTx, 0).scale(newScale);
                svg.transition().duration(300).ease(cubicOut).call(zoom$1.transform, newTransform).on("end", () => {
                  notifyVisibleTimeRange(newTransform);
                  onZoomDetected?.();
                });
              },
              title: "Zoom In",
              className: "p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v6m3-3H7" }) })
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "button",
            {
              onClick: () => {
                if (!svgRef.current) return;
                const svg = select(svgRef.current);
                svg.transition().duration(300).ease(cubicOut).call(zoom$1.transform, identity).on("end", () => {
                  notifyVisibleTimeRange(identity);
                  onZoomDetected?.();
                });
              },
              title: "Reset Zoom",
              className: "p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M3 8V6a2 2 0 012-2h2M3 16v2a2 2 0 002 2h2m10-16h2a2 2 0 012 2v2m0 8v2a2 2 0 01-2 2h-2" }) })
            }
          )
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "svg",
          {
            ref: svgRef,
            width,
            height: Math.max(height, sortedResources.length * rowHeight + MARGIN.top + MARGIN.bottom),
            className: "block",
            style: { overflow: "visible" }
          }
        ),
        tooltip && /* @__PURE__ */ jsxRuntimeExports.jsx(
          "div",
          {
            className: "fixed z-50 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl p-3 pointer-events-none max-w-sm",
            style: {
              left: `${tooltip.x + 10}px`,
              top: `${tooltip.y + 10}px`,
              transform: "translate(0, 0)"
            },
            children: /* @__PURE__ */ jsxRuntimeExports.jsx("pre", { className: "text-xs text-[var(--color-text-primary)] whitespace-pre-wrap font-sans", children: tooltip.content })
          }
        ),
        loadingMore && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "sticky bottom-0 left-0 right-0 flex items-center justify-center py-4 bg-[var(--color-surface-secondary)] border-t border-[var(--color-border-soft)] z-10", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2 text-[var(--color-text-muted)]", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1.5", d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-sm", children: "Loading more resources..." })
        ] }) })
      ]
    }
  );
};

const CONTEXT_LINES_DEFAULT = 3;
const toLines = (value) => {
  if (!value || Object.keys(value).length === 0) {
    return [];
  }
  return JSON.stringify(value, null, 2).split("\n");
};
const buildRawDiff = (a, b) => {
  const m = a.length;
  const n = b.length;
  const dp = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0));
  for (let i2 = m - 1; i2 >= 0; i2--) {
    for (let j2 = n - 1; j2 >= 0; j2--) {
      if (a[i2] === b[j2]) {
        dp[i2][j2] = dp[i2 + 1][j2 + 1] + 1;
      } else {
        dp[i2][j2] = Math.max(dp[i2 + 1][j2], dp[i2][j2 + 1]);
      }
    }
  }
  const diff = [];
  let i = 0;
  let j = 0;
  while (i < m && j < n) {
    if (a[i] === b[j]) {
      diff.push({ type: "equal", line: a[i] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      diff.push({ type: "remove", line: a[i] });
      i++;
    } else {
      diff.push({ type: "add", line: b[j] });
      j++;
    }
  }
  while (i < m) {
    diff.push({ type: "remove", line: a[i++] });
  }
  while (j < n) {
    diff.push({ type: "add", line: b[j++] });
  }
  return diff;
};
const diffJsonWithContext = (previous, current, contextLines = CONTEXT_LINES_DEFAULT) => {
  const prevLines = toLines(previous);
  const currLines = toLines(current);
  if (prevLines.length === 0 && currLines.length === 0) {
    return [];
  }
  if (prevLines.length === 0) {
    return currLines.map((line) => ({ type: "add", content: line }));
  }
  if (currLines.length === 0) {
    return prevLines.map((line) => ({ type: "remove", content: line }));
  }
  const rawDiff = buildRawDiff(prevLines, currLines);
  const includedIndices = /* @__PURE__ */ new Set();
  rawDiff.forEach((entry, index) => {
    if (entry.type !== "equal") {
      for (let offset = -contextLines; offset <= contextLines; offset++) {
        const target = index + offset;
        if (target >= 0 && target < rawDiff.length) {
          includedIndices.add(target);
        }
      }
    }
  });
  const result = [];
  let lastIncluded = -1;
  rawDiff.forEach((entry, index) => {
    if (!includedIndices.has(index)) {
      return;
    }
    if (lastIncluded !== -1 && index - lastIncluded > 1) {
      result.push({ type: "gap", content: "…" });
    }
    const lineType = entry.type === "equal" ? "context" : entry.type === "add" ? "add" : "remove";
    result.push({
      type: lineType,
      content: entry.line
    });
    lastIncluded = index;
  });
  return result;
};
const detectChangeCategories = (previous, current) => {
  const categories = {
    spec: false,
    status: false,
    metadata: false,
    other: false
  };
  if (!previous || !current) {
    return categories;
  }
  if (JSON.stringify(previous.spec) !== JSON.stringify(current.spec)) {
    categories.spec = true;
  }
  if (JSON.stringify(previous.status) !== JSON.stringify(current.status)) {
    categories.status = true;
  }
  const prevMeta = { ...previous.metadata };
  const currMeta = { ...current.metadata };
  delete prevMeta.managedFields;
  delete currMeta.managedFields;
  delete prevMeta.resourceVersion;
  delete currMeta.resourceVersion;
  delete prevMeta.generation;
  delete currMeta.generation;
  if (JSON.stringify(prevMeta) !== JSON.stringify(currMeta)) {
    categories.metadata = true;
  }
  const prevKeys = new Set(Object.keys(previous));
  const currKeys = new Set(Object.keys(current));
  const allKeys = /* @__PURE__ */ new Set([...prevKeys, ...currKeys]);
  for (const key of allKeys) {
    if (key !== "spec" && key !== "status" && key !== "metadata") {
      if (JSON.stringify(previous[key]) !== JSON.stringify(current[key])) {
        categories.other = true;
        break;
      }
    }
  }
  return categories;
};

const DiffLineView$1 = ({ line }) => {
  const styleMap = {
    add: "text-green-500 bg-green-100/10",
    remove: "text-red-400 bg-red-100/10",
    context: "text-[var(--color-text-primary)]",
    gap: "text-[var(--color-text-muted)] italic"
  };
  const prefixMap = {
    add: "+",
    remove: "-",
    context: " ",
    gap: "…"
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: `flex gap-2 px-2 rounded font-mono text-xs ${styleMap[line.type]}`, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "select-none w-3", children: prefixMap[line.type] }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "whitespace-pre-wrap", children: line.content })
  ] });
};
const FullDocumentDiff = ({
  current,
  previous
}) => {
  const fullJsonDiff = reactExports.useMemo(() => {
    const prev = previous ? JSON.stringify(previous, null, 2).split("\n") : [];
    const curr = current ? JSON.stringify(current, null, 2).split("\n") : [];
    const result = [];
    const maxLines = Math.max(prev.length, curr.length);
    for (let i = 0; i < maxLines; i++) {
      const prevLine = prev[i];
      const currLine = curr[i];
      if (prevLine === void 0) {
        result.push({ type: "add", content: currLine });
      } else if (currLine === void 0) {
        result.push({ type: "remove", content: prevLine });
      } else if (prevLine === currLine) {
        result.push({ type: "context", content: prevLine });
      } else {
        result.push({ type: "remove", content: prevLine });
        result.push({ type: "add", content: currLine });
      }
    }
    return result;
  }, [previous, current]);
  if (!current && !previous) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic", children: "Configuration details unavailable for this transition." });
  }
  return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-[var(--color-surface-secondary)] p-2 rounded border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto", children: fullJsonDiff.map((line, index) => /* @__PURE__ */ jsxRuntimeExports.jsx(DiffLineView$1, { line }, `${line.type}-${index}`)) });
};
const ConfigDiff = ({
  current,
  previous,
  showFullDiff = false
}) => {
  const diff = reactExports.useMemo(() => diffJsonWithContext(previous, current), [previous, current]);
  if (!current && !previous) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic", children: "Configuration details unavailable for this transition." });
  }
  if (showFullDiff) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx(FullDocumentDiff, { current, previous });
  }
  if (diff.length === 0) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic", children: "No changes detected in this transition." });
  }
  return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-[var(--color-surface-secondary)] p-2 rounded border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto", children: diff.map((line, index) => /* @__PURE__ */ jsxRuntimeExports.jsx(DiffLineView$1, { line }, `${line.type}-${index}`)) });
};
const DetailPanel = ({ resource, selectedIndex = 0, onClose, onAnalyzeRootCause }) => {
  const { formatTime } = useSettings();
  const [showFullDiff, setShowFullDiff] = reactExports.useState(false);
  const [width, setWidth] = reactExports.useState(384);
  const [isResizing, setIsResizing] = reactExports.useState(false);
  const MIN_WIDTH = 300;
  const MAX_WIDTH = 800;
  reactExports.useEffect(() => {
    if (!isResizing) return;
    const handleMouseMove = (e) => {
      const newWidth = window.innerWidth - e.clientX;
      if (newWidth >= MIN_WIDTH && newWidth <= MAX_WIDTH) {
        setWidth(newWidth);
      }
    };
    const handleMouseUp = () => {
      setIsResizing(false);
    };
    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing]);
  reactExports.useEffect(() => {
    if (!resource) return;
    const handleKeyDown = (e) => {
      if (e.key === "Escape") {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [resource, onClose]);
  if (!resource) return null;
  const currentSegment = resource.statusSegments[selectedIndex] || resource.statusSegments[0];
  const previousSegment = selectedIndex > 0 ? resource.statusSegments[selectedIndex - 1] : null;
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "div",
    {
      className: "fixed right-0 bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)] shadow-2xl transform transition-transform duration-300 ease-in-out z-20 overflow-y-auto flex flex-col text-[var(--color-text-primary)] DetailPanel",
      style: { width: `${width}px`, top: "73px", bottom: 0 },
      children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "div",
          {
            onMouseDown: () => setIsResizing(true),
            className: `absolute left-0 top-0 bottom-0 w-1 hover:w-1.5 hover:bg-blue-500/50 transition-all cursor-col-resize ${isResizing ? "bg-blue-500 w-1.5" : "bg-transparent"}`
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-6 flex-1", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex justify-between items-start mb-6", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("h2", { className: "text-xl font-bold break-all", children: resource.name }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2 mt-1", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs font-mono bg-[var(--color-surface-muted)] px-2 py-0.5 rounded text-[var(--color-text-muted)] border border-[var(--color-border-soft)]", children: resource.kind }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: resource.namespace })
              ] })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("button", { onClick: onClose, className: "text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors p-1 hover:bg-[var(--color-surface-muted)] rounded", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-6 h-6", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M6 18L18 6M6 6l12 12" }) }) })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("hr", { className: "my-4 border-[var(--color-border-soft)]" }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-6", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between mb-2", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold", children: [
                "Version ",
                selectedIndex + 1,
                " of ",
                resource.statusSegments.length
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: `w-2 h-2 rounded-full flex-shrink-0 ${currentSegment.status === "Ready" ? "bg-emerald-500" : currentSegment.status === "Error" ? "bg-red-500" : "bg-amber-500"}` }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: currentSegment.status })
              ] })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "bg-[var(--color-surface-muted)] rounded p-3 border border-[var(--color-border-soft)] space-y-2 mb-4", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)] flex justify-between", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "Start:" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-primary)]", children: formatTime(currentSegment.start) })
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)] flex justify-between", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "End:" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-primary)]", children: formatTime(currentSegment.end) })
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-primary)] italic border-t border-[var(--color-border-soft)] pt-2 mt-2", children: currentSegment.message })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-4", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between mb-2", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold", children: "Configuration Changes" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx(
                  "button",
                  {
                    onClick: () => setShowFullDiff(!showFullDiff),
                    className: `text-xs px-2 py-1 rounded transition-colors ${showFullDiff ? "bg-blue-500/20 text-blue-300 border border-blue-500/40" : "bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] border border-[var(--color-border-soft)] hover:bg-[var(--color-surface-active)]"}`,
                    children: showFullDiff ? "Full Document" : "Diff View"
                  }
                )
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx(
                ConfigDiff,
                {
                  current: currentSegment.resourceData,
                  previous: previousSegment?.resourceData,
                  showFullDiff
                }
              )
            ] })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold mb-2", children: "Relevant Events" }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "space-y-3", children: resource.events.filter((e) => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end).length === 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[var(--color-text-muted)] text-sm italic", children: "No events in this time window." }) : resource.events.filter((e) => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end).sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime()).map((event) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "bg-[var(--color-surface-muted)] p-3 rounded border-l-4 border-[var(--color-border-soft)] hover:bg-[var(--color-surface-active)] transition-colors space-y-2", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex justify-between items-start", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs font-mono text-[var(--color-text-muted)]", children: formatTime(event.timestamp) }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: `text-xs px-2 py-0.5 rounded font-semibold tracking-wide ${event.type === "Warning" ? "bg-amber-100/20 text-amber-300 border border-amber-500/40" : "bg-blue-100/20 text-blue-200 border border-blue-500/30"}`, children: event.reason || "Event" })
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-primary)] leading-snug", children: event.message }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)] flex gap-4", children: [
                event.source && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
                  "Source: ",
                  event.source
                ] }),
                /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
                  "Count: ",
                  event.count ?? 0
                ] })
              ] })
            ] }, event.id)) })
          ] })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-3 bg-[var(--color-surface-secondary)] border-t border-[var(--color-border-soft)] flex justify-between items-center", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: "Use Arrow Left/Right to navigate history" }),
          onAnalyzeRootCause && /* @__PURE__ */ jsxRuntimeExports.jsx(
            "button",
            {
              onClick: onAnalyzeRootCause,
              className: "px-3 py-1.5 text-xs font-medium bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors",
              children: "Analyze Root Cause"
            }
          )
        ] })
      ]
    }
  );
};

const CARD_WIDTH = 280;
const CARD_HEIGHT = 180;
const HEADER_HEIGHT = 60;
const PORT_INSET = 0;
function buildLayoutModel(analysis) {
  if (!analysis?.incident?.graph || analysis.incident.graph.nodes.length === 0) {
    console.warn("[buildLayoutModel] No graph data in analysis");
    return { nodes: [], edges: [] };
  }
  const graph = analysis.incident.graph;
  const nodes = [];
  const edges = [];
  console.log("[buildLayoutModel] Processing graph:", {
    nodeCount: graph.nodes.length,
    edgeCount: graph.edges.length,
    nodeTypes: graph.nodes.map((n) => n.nodeType)
  });
  graph.nodes.forEach((node) => {
    const hasEvents = node.allEvents && node.allEvents.length > 0;
    const eventCount = node.allEvents?.length || 0;
    let calculatedHeight = HEADER_HEIGHT + 10;
    if (hasEvents) {
      calculatedHeight += 65;
    }
    const finalHeight = Math.max(calculatedHeight, 80);
    const layoutNode = {
      id: node.id,
      kind: node.nodeType === "SPINE" ? "spine" : "side",
      nodeRef: node,
      width: CARD_WIDTH,
      height: finalHeight
    };
    nodes.push(layoutNode);
    console.log("[buildLayoutModel] Created node:", {
      id: layoutNode.id,
      kind: layoutNode.kind,
      nodeType: node.nodeType,
      resourceKind: node.resource.kind,
      hasEvents,
      eventCount,
      calculatedHeight: finalHeight
    });
  });
  graph.edges.forEach((edge) => {
    edges.push({
      id: edge.id,
      from: edge.from,
      to: edge.to,
      kind: edge.edgeType === "SPINE" ? "spine" : "attachment",
      relationshipType: edge.relationshipType
    });
  });
  console.log("[buildLayoutModel] Created:", {
    nodes: nodes.length,
    spineNodes: nodes.filter((n) => n.kind === "spine").length,
    sideNodes: nodes.filter((n) => n.kind === "side").length,
    edges: edges.length,
    spineEdges: edges.filter((e) => e.kind === "spine").length,
    attachmentEdges: edges.filter((e) => e.kind === "attachment").length
  });
  return { nodes, edges };
}
function calculatePorts(node) {
  const halfWidth = node.width / 2;
  const headerCenterY = HEADER_HEIGHT / 2;
  return {
    top: {
      side: "top",
      x: node.x + halfWidth,
      y: node.y + headerCenterY
      // Connect at header center
    },
    bottom: {
      side: "bottom",
      x: node.x + halfWidth,
      y: node.y + node.height
    },
    left: {
      side: "left",
      x: node.x,
      y: node.y + headerCenterY
      // Connect at header center
    },
    right: {
      side: "right",
      x: node.x + node.width,
      y: node.y + headerCenterY
      // Connect at header center
    }
  };
}
function selectPorts(fromNode, toNode) {
  const dx = toNode.x - fromNode.x;
  const dy = toNode.y - fromNode.y;
  if (Math.abs(dy) > Math.abs(dx)) {
    if (dy > 0) {
      return {
        fromPort: fromNode.ports.bottom,
        toPort: toNode.ports.top
      };
    } else {
      return {
        fromPort: fromNode.ports.top,
        toPort: toNode.ports.bottom
      };
    }
  } else {
    if (dx > 0) {
      return {
        fromPort: fromNode.ports.right,
        toPort: toNode.ports.left
      };
    } else {
      return {
        fromPort: fromNode.ports.left,
        toPort: toNode.ports.right
      };
    }
  }
}

const SPINE_HORIZONTAL_SPACING = 200;
const SPINE_VERTICAL_SPACING = 150;
const SIDE_NODE_SPACING = 80;
const SIDE_REGION_OFFSET = 180;
function buildGraph(spineNodes, edges) {
  const outgoing = /* @__PURE__ */ new Map();
  const incoming = /* @__PURE__ */ new Map();
  spineNodes.forEach((node) => {
    outgoing.set(node.id, []);
    incoming.set(node.id, []);
  });
  edges.filter((e) => e.kind === "spine").forEach((edge) => {
    outgoing.get(edge.from)?.push(edge.to);
    incoming.get(edge.to)?.push(edge.from);
  });
  return { outgoing, incoming };
}
function assignLayers(spineNodes, edges) {
  const { outgoing, incoming } = buildGraph(spineNodes, edges);
  const layers = /* @__PURE__ */ new Map();
  const roots = spineNodes.filter(
    (node) => (incoming.get(node.id)?.length || 0) === 0
  );
  if (roots.length === 0) {
    const firstNode = spineNodes.sort((a, b) => (a.nodeRef.stepNumber || 0) - (b.nodeRef.stepNumber || 0))[0];
    roots.push(firstNode);
  }
  const visited = /* @__PURE__ */ new Set();
  const queue = [];
  roots.forEach((root) => {
    layers.set(root.id, 0);
    queue.push({ nodeId: root.id, layer: 0 });
  });
  while (queue.length > 0) {
    const { nodeId, layer } = queue.shift();
    if (visited.has(nodeId)) continue;
    visited.add(nodeId);
    const children = outgoing.get(nodeId) || [];
    children.forEach((childId) => {
      const currentLayer = layers.get(childId) ?? -1;
      const newLayer = layer + 1;
      if (newLayer > currentLayer) {
        layers.set(childId, newLayer);
      }
      if (!visited.has(childId)) {
        queue.push({ nodeId: childId, layer: newLayer });
      }
    });
  }
  spineNodes.forEach((node) => {
    if (!layers.has(node.id)) {
      layers.set(node.id, 0);
    }
  });
  return layers;
}
function assignRows(nodesByLayer, edges) {
  const rows = /* @__PURE__ */ new Map();
  const { outgoing, incoming } = buildGraph(
    Array.from(nodesByLayer.values()).flat(),
    edges
  );
  const sortedLayers = Array.from(nodesByLayer.keys()).sort((a, b) => a - b);
  sortedLayers.forEach((layer) => {
    const nodesInLayer = nodesByLayer.get(layer) || [];
    if (layer === 0) {
      nodesInLayer.sort((a, b) => (a.nodeRef.stepNumber || 0) - (b.nodeRef.stepNumber || 0));
      nodesInLayer.forEach((node, idx) => {
        rows.set(node.id, idx);
      });
    } else {
      const nodeScores = nodesInLayer.map((node) => {
        const parents = incoming.get(node.id) || [];
        if (parents.length === 0) {
          return { node, score: 0 };
        }
        const parentRows = parents.map((p) => rows.get(p)).filter((r) => r !== void 0);
        if (parentRows.length === 0) {
          return { node, score: 0 };
        }
        parentRows.sort((a, b) => a - b);
        const median = parentRows[Math.floor(parentRows.length / 2)];
        return { node, score: median };
      });
      nodeScores.sort((a, b) => a.score - b.score);
      nodeScores.forEach((item, idx) => {
        rows.set(item.node.id, idx);
      });
    }
  });
  optimizeRowsToAvoidEdgeOverlaps(rows, nodesByLayer, edges, incoming, outgoing);
  return rows;
}
function optimizeForSideNodeEdges(rows, nodesByLayer, edges, sideNodes) {
  const sideNodeIds = new Set(sideNodes.map((n) => n.id));
  const attachmentEdges = edges.filter(
    (e) => e.kind === "attachment" && sideNodeIds.has(e.to)
  );
  if (attachmentEdges.length === 0) return;
  const spineNodesWithAttachments = new Set(attachmentEdges.map((e) => e.from));
  Array.from(rows.keys()).forEach((nodeId) => {
    if (spineNodesWithAttachments.has(nodeId)) {
      const currentRow = rows.get(nodeId) || 0;
      rows.set(nodeId, currentRow + 1);
    }
  });
}
function optimizeRowsToAvoidEdgeOverlaps(rows, nodesByLayer, edges, incoming, outgoing) {
  const sortedLayers = Array.from(nodesByLayer.keys()).sort((a, b) => a - b);
  const nodeToLayer = /* @__PURE__ */ new Map();
  sortedLayers.forEach((layer) => {
    const nodesInLayer = nodesByLayer.get(layer) || [];
    nodesInLayer.forEach((node) => nodeToLayer.set(node.id, layer));
  });
  for (let iteration = 0; iteration < 3; iteration++) {
    for (let layer = 1; layer < sortedLayers.length; layer++) {
      const currentLayer = sortedLayers[layer];
      const intermediateNodes = nodesByLayer.get(currentLayer) || [];
      intermediateNodes.forEach((node) => {
        const nodeRow = rows.get(node.id);
        if (nodeRow === void 0) return;
        edges.filter((e) => e.kind === "spine").forEach((edge) => {
          const fromLayer = nodeToLayer.get(edge.from);
          const toLayer = nodeToLayer.get(edge.to);
          const fromRow = rows.get(edge.from);
          const toRow = rows.get(edge.to);
          if (fromLayer === void 0 || toLayer === void 0 || fromRow === void 0 || toRow === void 0) {
            return;
          }
          if (fromLayer < currentLayer && toLayer > currentLayer) {
            const minRow = Math.min(fromRow, toRow);
            const maxRow = Math.max(fromRow, toRow);
            if (nodeRow >= minRow - 0.5 && nodeRow <= maxRow + 0.5) {
              const nodesInCurrentLayer = intermediateNodes.length;
              const nodeConnections = [
                ...incoming.get(node.id) || [],
                ...outgoing.get(node.id) || []
              ];
              if (nodeConnections.length > 0) {
                const connectedRows = nodeConnections.map((id) => rows.get(id)).filter((r) => r !== void 0);
                if (connectedRows.length > 0) {
                  connectedRows.sort((a, b) => a - b);
                  const medianRow = connectedRows[Math.floor(connectedRows.length / 2)];
                  if (medianRow < minRow - 0.5 || medianRow > maxRow + 0.5) {
                    rows.set(node.id, medianRow);
                  } else {
                    if (minRow > 0) {
                      rows.set(node.id, minRow - 1);
                    } else if (maxRow < nodesInCurrentLayer - 1) {
                      rows.set(node.id, maxRow + 1);
                    }
                  }
                }
              }
            }
          }
        });
      });
    }
  }
  console.log("[optimizeForSideNodeEdges] Final rows after optimization:", Array.from(rows.entries()));
}
function placeNodes(nodes, edges) {
  if (nodes.length === 0) {
    console.warn("[placeNodes] No nodes to place");
    return [];
  }
  const spineNodes = nodes.filter((n) => n.kind === "spine");
  const sideNodes = nodes.filter((n) => n.kind === "side");
  console.log("[placeNodes] Processing:", {
    total: nodes.length,
    spine: spineNodes.length,
    side: sideNodes.length
  });
  const layers = assignLayers(spineNodes, edges);
  const nodesByLayer = /* @__PURE__ */ new Map();
  spineNodes.forEach((node) => {
    const layer = layers.get(node.id) ?? 0;
    if (!nodesByLayer.has(layer)) {
      nodesByLayer.set(layer, []);
    }
    nodesByLayer.get(layer).push(node);
  });
  const rows = assignRows(nodesByLayer, edges);
  optimizeForSideNodeEdges(rows, nodesByLayer, edges, sideNodes);
  const positionedSpine = [];
  const startX = 50;
  const startY = 200;
  spineNodes.forEach((node) => {
    const layer = layers.get(node.id) ?? 0;
    const row = rows.get(node.id) ?? 0;
    const x = startX + layer * (CARD_WIDTH + SPINE_HORIZONTAL_SPACING);
    const y = startY + row * SPINE_VERTICAL_SPACING;
    const positioned = {
      ...node,
      x,
      y,
      ports: calculatePorts({ ...node, x, y })
    };
    positionedSpine.push(positioned);
  });
  const sideNodesByParent = /* @__PURE__ */ new Map();
  const attachmentEdges = edges.filter((e) => e.kind === "attachment");
  console.log("[placeNodes] Found", attachmentEdges.length, "attachment edges");
  attachmentEdges.forEach((edge) => {
    const sideNode = sideNodes.find((n) => n.id === edge.to);
    if (sideNode) {
      if (!sideNodesByParent.has(edge.from)) {
        sideNodesByParent.set(edge.from, []);
      }
      sideNodesByParent.get(edge.from).push(sideNode);
      console.log("[placeNodes] Mapped side node", sideNode.id, "to parent", edge.from);
    }
  });
  const positionedSide = [];
  const spineMap = /* @__PURE__ */ new Map();
  positionedSpine.forEach((node) => {
    spineMap.set(node.id, node);
  });
  positionedSpine.forEach((spineNode) => {
    const relatedNodes = sideNodesByParent.get(spineNode.id) || [];
    if (relatedNodes.length === 0) return;
    const stepNum = spineNode.nodeRef.stepNumber || 0;
    const useTop = stepNum % 2 === 1;
    const maxSideHeight = Math.max(...relatedNodes.map((n) => n.height));
    const regionY = useTop ? spineNode.y - SIDE_REGION_OFFSET - maxSideHeight : spineNode.y + spineNode.height + SIDE_REGION_OFFSET;
    const orderedNodes = [...relatedNodes].sort((a, b) => {
      const aTargets = edges.filter((e) => e.from === a.id && e.kind === "spine").map((e) => spineMap.get(e.to)).filter((n) => n !== void 0);
      const bTargets = edges.filter((e) => e.from === b.id && e.kind === "spine").map((e) => spineMap.get(e.to)).filter((n) => n !== void 0);
      const aAvgX = aTargets.length > 0 ? aTargets.reduce((sum, n) => sum + n.x, 0) / aTargets.length : spineNode.x;
      const bAvgX = bTargets.length > 0 ? bTargets.reduce((sum, n) => sum + n.x, 0) / bTargets.length : spineNode.x;
      return aAvgX - bAvgX;
    });
    const spineCenterX = spineNode.x + spineNode.width / 2;
    const totalSideWidth = orderedNodes.length * (CARD_WIDTH + SIDE_NODE_SPACING) - SIDE_NODE_SPACING;
    let startX2 = spineCenterX - totalSideWidth / 2;
    orderedNodes.forEach((sideNode) => {
      const positioned = {
        ...sideNode,
        x: startX2,
        y: regionY,
        ports: calculatePorts({ ...sideNode, x: startX2, y: regionY })
      };
      positionedSide.push(positioned);
      startX2 += CARD_WIDTH + SIDE_NODE_SPACING;
    });
  });
  const allPositioned = [...positionedSpine, ...positionedSide];
  console.log("[placeNodes] Result:", {
    totalPositioned: allPositioned.length,
    spine: positionedSpine.length,
    side: positionedSide.length
  });
  return allPositioned.map((node) => {
    return {
      ...node,
      ports: calculatePorts(node)
    };
  });
}
function calculateBounds(nodes) {
  if (nodes.length === 0) {
    return { x: 0, y: 0, width: 0, height: 0 };
  }
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  nodes.forEach((node) => {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x + node.width);
    maxY = Math.max(maxY, node.y + node.height);
  });
  return {
    x: minX,
    y: minY,
    width: maxX - minX,
    height: maxY - minY
  };
}

const GRID_CELL_SIZE = 25;
const COST_STRAIGHT = 1;
const COST_BEND = 5;
const COST_NEW_LANE = 10;
const COST_SPINE_TRUNK = 0.5;
const COST_ATTACHMENT_TRUNK = 0.8;
const COST_BLOCKED = 1e4;
function buildGrid(nodes, bounds) {
  const cols = Math.ceil(bounds.width / GRID_CELL_SIZE) + 2;
  const rows = Math.ceil(bounds.height / GRID_CELL_SIZE) + 2;
  const grid = [];
  for (let y = 0; y < rows; y++) {
    grid[y] = [];
    for (let x = 0; x < cols; x++) {
      const worldX = bounds.x + (x - 1) * GRID_CELL_SIZE;
      const worldY = bounds.y + (y - 1) * GRID_CELL_SIZE;
      grid[y][x] = {
        x: worldX,
        y: worldY,
        blocked: false,
        isSpineTrunk: false,
        isAttachmentTrunk: false
      };
    }
  }
  const OBSTACLE_PADDING = 0;
  nodes.forEach((node) => {
    const startCol = Math.floor((node.x - OBSTACLE_PADDING - bounds.x) / GRID_CELL_SIZE);
    const endCol = Math.ceil((node.x + node.width + OBSTACLE_PADDING - bounds.x) / GRID_CELL_SIZE);
    const startRow = Math.floor((node.y - OBSTACLE_PADDING - bounds.y) / GRID_CELL_SIZE);
    const endRow = Math.ceil((node.y + node.height + OBSTACLE_PADDING - bounds.y) / GRID_CELL_SIZE);
    for (let row = startRow; row <= endRow; row++) {
      for (let col = startCol; col <= endCol; col++) {
        if (row >= 0 && row < rows && col >= 0 && col < cols) {
          grid[row][col].blocked = true;
        }
      }
    }
  });
  const spineNodes = nodes.filter((n) => n.kind === "spine");
  if (spineNodes.length > 0) {
    const spineCenterY = spineNodes.reduce((sum, n) => sum + n.y + n.height / 2, 0) / spineNodes.length;
    const spineRow = Math.floor((spineCenterY - bounds.y) / GRID_CELL_SIZE);
    for (let col = 0; col < cols; col++) {
      if (spineRow >= 0 && spineRow < rows && !grid[spineRow][col].blocked) {
        grid[spineRow][col].isSpineTrunk = true;
      }
    }
  }
  spineNodes.forEach((spineNode) => {
    const sideNodes = nodes.filter((n) => n.kind === "side" && n.parentId === spineNode.id);
    if (sideNodes.length > 0) {
      const spineCenterX = spineNode.x + spineNode.width / 2;
      const spineCol = Math.floor((spineCenterX - bounds.x) / GRID_CELL_SIZE);
      sideNodes.forEach((sideNode) => {
        const sideCenterY = sideNode.y + sideNode.height / 2;
        const sideRow = Math.floor((sideCenterY - bounds.y) / GRID_CELL_SIZE);
        const spineRow = Math.floor((spineNode.y + spineNode.height / 2 - bounds.y) / GRID_CELL_SIZE);
        const startRow = Math.min(spineRow, sideRow);
        const endRow = Math.max(spineRow, sideRow);
        for (let row = startRow; row <= endRow; row++) {
          if (row >= 0 && row < rows && spineCol >= 0 && spineCol < cols && !grid[row][spineCol].blocked) {
            grid[row][spineCol].isAttachmentTrunk = true;
          }
        }
      });
    }
  });
  return grid;
}
function worldToGrid(x, y, bounds) {
  return {
    col: Math.floor((x - bounds.x) / GRID_CELL_SIZE),
    row: Math.floor((y - bounds.y) / GRID_CELL_SIZE)
  };
}
function gridToWorld(col, row, bounds) {
  return {
    x: bounds.x + col * GRID_CELL_SIZE + GRID_CELL_SIZE / 2,
    y: bounds.y + row * GRID_CELL_SIZE + GRID_CELL_SIZE / 2
  };
}
function heuristic(a, b) {
  return Math.abs(a.x - b.x) + Math.abs(a.y - b.y);
}
function getNeighbors(node, grid, bounds) {
  const neighbors = [];
  const { col, row } = worldToGrid(node.x, node.y, bounds);
  const directions = [
    { dx: 0, dy: -1 },
    // Up
    { dx: 1, dy: 0 },
    // Right
    { dx: 0, dy: 1 },
    // Down
    { dx: -1, dy: 0 }
    // Left
  ];
  directions.forEach((dir) => {
    const newRow = row + dir.dy;
    const newCol = col + dir.dx;
    if (newRow >= 0 && newRow < grid.length && newCol >= 0 && newCol < grid[0].length) {
      const cell = grid[newRow][newCol];
      if (!cell.blocked) {
        const worldPos = gridToWorld(newCol, newRow, bounds);
        neighbors.push({
          x: worldPos.x,
          y: worldPos.y,
          g: 0,
          h: 0,
          f: 0
        });
      }
    }
  });
  return neighbors;
}
function getMoveCost(from, to, grid, bounds) {
  const { col: fromCol, row: fromRow } = worldToGrid(from.x, from.y, bounds);
  const { col: toCol, row: toRow } = worldToGrid(to.x, to.y, bounds);
  if (fromRow < 0 || fromRow >= grid.length || fromCol < 0 || fromCol >= grid[0].length) {
    return COST_BLOCKED;
  }
  const cell = grid[toRow][toCol];
  if (cell.blocked) {
    return COST_BLOCKED;
  }
  let cost = COST_STRAIGHT;
  if (from.parent) {
    const { col: parentCol, row: parentRow } = worldToGrid(from.parent.x, from.parent.y, bounds);
    const isBend = parentCol !== toCol && parentRow !== toRow || fromCol !== toCol && fromRow !== toRow;
    if (isBend) {
      cost += COST_BEND;
    }
  }
  if (cell.isSpineTrunk) {
    cost *= COST_SPINE_TRUNK;
  } else if (cell.isAttachmentTrunk) {
    cost *= COST_ATTACHMENT_TRUNK;
  } else {
    cost += COST_NEW_LANE;
  }
  return cost;
}
function findPath(start, end, grid, bounds) {
  const startNode = { x: start.x, y: start.y, g: 0, h: heuristic({ x: start.x, y: start.y, g: 0, h: 0, f: 0 }, { x: end.x, y: end.y, g: 0, h: 0, f: 0 }), f: 0 };
  const endNode = { x: end.x, y: end.y, g: 0, h: 0, f: 0 };
  const openSet = [startNode];
  const closedSet = /* @__PURE__ */ new Set();
  const getKey = (node) => `${node.x},${node.y}`;
  while (openSet.length > 0) {
    openSet.sort((a, b) => a.f - b.f);
    const current = openSet.shift();
    if (getKey(current) === getKey(endNode)) {
      const path = [];
      let node = current;
      while (node) {
        path.unshift({ x: node.x, y: node.y });
        node = node.parent;
      }
      return path;
    }
    closedSet.add(getKey(current));
    const neighbors = getNeighbors(current, grid, bounds);
    for (const neighbor of neighbors) {
      if (closedSet.has(getKey(neighbor))) continue;
      const tentativeG = current.g + getMoveCost(current, neighbor, grid, bounds);
      neighbor.parent = current;
      neighbor.g = tentativeG;
      neighbor.h = heuristic(neighbor, endNode);
      neighbor.f = neighbor.g + neighbor.h;
      const existing = openSet.find((n) => getKey(n) === getKey(neighbor));
      if (!existing) {
        openSet.push(neighbor);
      } else if (tentativeG < existing.g) {
        existing.g = tentativeG;
        existing.f = existing.g + existing.h;
        existing.parent = current;
      }
    }
  }
  return [{ x: start.x, y: start.y }, { x: end.x, y: end.y }];
}
function simplifyPath(points) {
  if (points.length <= 2) return points;
  const simplified = [points[0]];
  for (let i = 1; i < points.length - 1; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const next = points[i + 1];
    const dx1 = curr.x - prev.x;
    const dy1 = curr.y - prev.y;
    const dx2 = next.x - curr.x;
    const dy2 = next.y - curr.y;
    const crossProduct = dx1 * dy2 - dy1 * dx2;
    if (Math.abs(crossProduct) > 0.1) {
      simplified.push(curr);
    }
  }
  simplified.push(points[points.length - 1]);
  return simplified;
}
function routeEdges(edges, nodes, bounds) {
  if (edges.length === 0) {
    return [];
  }
  const grid = buildGrid(nodes, bounds);
  const nodeMap = /* @__PURE__ */ new Map();
  nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });
  const routedEdges = [];
  edges.forEach((edge) => {
    const fromNode = nodeMap.get(edge.from);
    const toNode = nodeMap.get(edge.to);
    if (!fromNode || !toNode) {
      console.warn(`[routeEdges] Missing node for edge ${edge.from} -> ${edge.to}`);
      return;
    }
    let fromPort, toPort;
    if (edge.kind === "attachment") {
      const dy = toNode.y - fromNode.y;
      if (dy < 0) {
        fromPort = {
          side: "top",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y
          // Top edge of card
        };
        toPort = {
          side: "bottom",
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height
          // Bottom edge of card
        };
      } else {
        fromPort = {
          side: "bottom",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height
          // Bottom edge of card
        };
        toPort = {
          side: "top",
          x: toNode.x + toNode.width / 2,
          y: toNode.y
          // Top edge of card
        };
      }
    } else {
      const ports = selectPorts(fromNode, toNode);
      fromPort = ports.fromPort;
      toPort = ports.toPort;
      if (fromPort.side === "top") {
        fromPort = {
          side: "top",
          x: fromNode.x + fromNode.width / 2,
          // Center horizontally
          y: fromNode.y
          // Top edge
        };
      } else if (fromPort.side === "bottom") {
        fromPort = {
          side: "bottom",
          x: fromNode.x + fromNode.width / 2,
          // Center horizontally
          y: fromNode.y + fromNode.height
          // Bottom edge
        };
      } else if (fromPort.side === "left") {
        fromPort = {
          side: "left",
          x: fromNode.x,
          // Left edge
          y: fromNode.y + HEADER_HEIGHT / 2
          // Header center (30px from top)
        };
      } else if (fromPort.side === "right") {
        fromPort = {
          side: "right",
          x: fromNode.x + fromNode.width,
          // Right edge
          y: fromNode.y + HEADER_HEIGHT / 2
          // Header center (30px from top)
        };
      }
      if (toPort.side === "top") {
        toPort = {
          side: "top",
          x: toNode.x + toNode.width / 2,
          // Center horizontally
          y: toNode.y
          // Top edge
        };
      } else if (toPort.side === "bottom") {
        toPort = {
          side: "bottom",
          x: toNode.x + toNode.width / 2,
          // Center horizontally
          y: toNode.y + toNode.height
          // Bottom edge
        };
      } else if (toPort.side === "left") {
        toPort = {
          side: "left",
          x: toNode.x,
          // Left edge
          y: toNode.y + HEADER_HEIGHT / 2
          // Header center (30px from top)
        };
      } else if (toPort.side === "right") {
        toPort = {
          side: "right",
          x: toNode.x + toNode.width,
          // Right edge
          y: toNode.y + HEADER_HEIGHT / 2
          // Header center (30px from top)
        };
      }
    }
    const exactFromPort = { x: fromPort.x, y: fromPort.y };
    const exactToPort = { x: toPort.x, y: toPort.y };
    let path = findPath(
      exactFromPort,
      exactToPort,
      grid,
      bounds
    );
    if (path.length < 2) {
      path = [exactFromPort, exactToPort];
    }
    path[0] = exactFromPort;
    path[path.length - 1] = exactToPort;
    const simplifiedPath = simplifyPath(path);
    simplifiedPath[0] = exactFromPort;
    simplifiedPath[simplifiedPath.length - 1] = exactToPort;
    routedEdges.push({
      ...edge,
      points: simplifiedPath,
      fromPort: { ...fromPort, x: exactFromPort.x, y: exactFromPort.y },
      toPort: { ...toPort, x: exactToPort.x, y: exactToPort.y }
    });
  });
  return routedEdges;
}

const EDGE_OFFSET_SPACING = 8;
function areParallel(x1, y1, x2, y2, x3, y3, x4, y4, tolerance = 5) {
  const dx1 = x2 - x1;
  const dy1 = y2 - y1;
  const dx2 = x4 - x3;
  const dy2 = y4 - y3;
  const len1 = Math.sqrt(dx1 * dx1 + dy1 * dy1);
  const len2 = Math.sqrt(dx2 * dx2 + dy2 * dy2);
  if (len1 < 0.1 || len2 < 0.1) return false;
  const nx1 = dx1 / len1;
  const ny1 = dy1 / len1;
  const nx2 = dx2 / len2;
  const ny2 = dy2 / len2;
  const dot = nx1 * nx2 + ny1 * ny2;
  return Math.abs(Math.abs(dot) - 1) < tolerance / 100;
}
function offsetPoint(x1, y1, x2, y2, offset) {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy);
  if (len < 0.1) {
    return { x: x1, y: y1 };
  }
  const perpX = -dy / len;
  const perpY = dx / len;
  return {
    x: x1 + perpX * offset,
    y: y1 + perpY * offset
  };
}
function bundleEdges(edges) {
  if (edges.length === 0) {
    return edges;
  }
  const segments = [];
  edges.forEach((edge, edgeIdx) => {
    for (let i = 0; i < edge.points.length - 1; i++) {
      const p1 = edge.points[i];
      const p2 = edge.points[i + 1];
      let foundLane = false;
      for (const lane of segments) {
        if (areParallel(
          lane.x1,
          lane.y1,
          lane.x2,
          lane.y2,
          p1.x,
          p1.y,
          p2.x,
          p2.y
        )) {
          const dist1 = Math.abs((p2.x - p1.x) * (lane.y1 - p1.y) - (p2.y - p1.y) * (lane.x1 - p1.x)) / Math.sqrt((p2.x - p1.x) ** 2 + (p2.y - p1.y) ** 2);
          const dist2 = Math.abs((p2.x - p1.x) * (lane.y2 - p1.y) - (p2.y - p1.y) * (lane.x2 - p1.x)) / Math.sqrt((p2.x - p1.x) ** 2 + (p2.y - p1.y) ** 2);
          if (dist1 < EDGE_OFFSET_SPACING * 2 || dist2 < EDGE_OFFSET_SPACING * 2) {
            lane.edges.push({ edge, index: i });
            foundLane = true;
            break;
          }
        }
      }
      if (!foundLane) {
        segments.push({
          x1: p1.x,
          y1: p1.y,
          x2: p2.x,
          y2: p2.y,
          edges: [{ edge, index: i }]
        });
      }
    }
  });
  const bundledEdges = edges.map((edge) => ({ ...edge, points: [...edge.points] }));
  segments.forEach((lane) => {
    if (lane.edges.length <= 1) return;
    lane.edges.sort((a, b) => {
      const aMidX = (a.edge.points[a.index].x + a.edge.points[a.index + 1].x) / 2;
      const aMidY = (a.edge.points[a.index].y + a.edge.points[a.index + 1].y) / 2;
      const bMidX = (b.edge.points[b.index].x + b.edge.points[b.index + 1].x) / 2;
      const bMidY = (b.edge.points[b.index].y + b.edge.points[b.index + 1].y) / 2;
      const dx = lane.x2 - lane.x1;
      const dy = lane.y2 - lane.y1;
      const len = Math.sqrt(dx * dx + dy * dy);
      if (len < 0.1) return 0;
      const projA = ((aMidX - lane.x1) * dx + (aMidY - lane.y1) * dy) / len;
      const projB = ((bMidX - lane.x1) * dx + (bMidY - lane.y1) * dy) / len;
      return projA - projB;
    });
    const centerOffset = -(lane.edges.length - 1) * EDGE_OFFSET_SPACING / 2;
    lane.edges.forEach((item, idx) => {
      const offset = centerOffset + idx * EDGE_OFFSET_SPACING;
      const p1 = item.edge.points[item.index];
      const p2 = item.edge.points[item.index + 1];
      const offset1 = offsetPoint(lane.x1, lane.y1, lane.x2, lane.y2, offset);
      const offset2 = offsetPoint(lane.x1, lane.y1, lane.x2, lane.y2, offset);
      const dx = p2.x - p1.x;
      const dy = p2.y - p1.y;
      const len = Math.sqrt(dx * dx + dy * dy);
      if (len > 0.1) {
        const perpX = -dy / len;
        const perpY = dx / len;
        const offsetP1 = {
          x: p1.x + perpX * offset,
          y: p1.y + perpY * offset
        };
        const offsetP2 = {
          x: p2.x + perpX * offset,
          y: p2.y + perpY * offset
        };
        const bundledEdge = bundledEdges.find((e) => e.id === item.edge.id);
        if (bundledEdge) {
          bundledEdge.points[item.index] = offsetP1;
          bundledEdge.points[item.index + 1] = offsetP2;
        }
      }
    });
  });
  return bundledEdges;
}

const SIMULATION_ITERATIONS = 400;
const COLLISION_PADDING = 60;
const SPINE_LINK_DISTANCE = 200;
const ATTACHMENT_LINK_DISTANCE = 150;
const CHARGE_STRENGTH = -1800;
const SPINE_CENTER_Y = 400;
const SPINE_SPACING_X = CARD_WIDTH + 200;
const ATTACHMENT_RADIUS_BASE = 350;
const ATTACHMENT_RADIUS_VARIANCE = 120;
function hashString(str) {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
}
function getInitialPosition(node, allNodes) {
  const hash = hashString(node.id);
  if (node.kind === "spine") {
    const spineNodes = allNodes.filter((n) => n.kind === "spine");
    const spineIndex = spineNodes.findIndex((n) => n.id === node.id);
    const stepNumber = node.nodeRef.stepNumber || spineIndex;
    const baseX = 500 + stepNumber * SPINE_SPACING_X;
    const jitterX = hash % 80 - 40;
    return {
      x: baseX + jitterX,
      y: SPINE_CENTER_Y + (hash % 150 - 75)
      // Reduced vertical jitter
    };
  } else {
    const angle = hash % 360 * (Math.PI / 180);
    const radius = ATTACHMENT_RADIUS_BASE + hash % ATTACHMENT_RADIUS_VARIANCE;
    return {
      x: 800 + Math.cos(angle) * radius,
      y: SPINE_CENTER_Y + Math.sin(angle) * radius
    };
  }
}
function getRelationshipClusterParams(relationshipType, parentSpineId) {
  if (!relationshipType) {
    relationshipType = "UNKNOWN";
  }
  const clusterKey = `${relationshipType}_${parentSpineId}`;
  const hash = hashString(clusterKey);
  const angle = hash % 360 * (Math.PI / 180);
  const radiusOffset = hash % ATTACHMENT_RADIUS_VARIANCE - ATTACHMENT_RADIUS_VARIANCE / 2;
  return {
    angle,
    radius: ATTACHMENT_RADIUS_BASE + radiusOffset
  };
}
function buildParentMap(edges) {
  const parentMap = /* @__PURE__ */ new Map();
  edges.forEach((edge) => {
    if (edge.kind === "attachment") {
      parentMap.set(edge.to, edge.from);
    }
  });
  return parentMap;
}
function placeNodesForce(nodes, edges) {
  if (nodes.length === 0) {
    console.warn("[placeNodesForce] No nodes to place");
    return [];
  }
  console.log("[placeNodesForce] Processing:", {
    total: nodes.length,
    spine: nodes.filter((n) => n.kind === "spine").length,
    side: nodes.filter((n) => n.kind === "side").length,
    edges: edges.length,
    nodeDetails: nodes.map((n) => ({ id: n.id, kind: n.kind, resource: `${n.nodeRef.resource.kind}/${n.nodeRef.resource.name}` })),
    edgeDetails: edges.map((e) => ({ from: e.from, to: e.to, kind: e.kind, relationshipType: e.relationshipType }))
  });
  const parentMap = buildParentMap(edges);
  const simulationNodes = nodes.map((node) => {
    const initial = getInitialPosition(node, nodes);
    return {
      id: node.id,
      node,
      x: initial.x,
      y: initial.y
    };
  });
  const nodeMap = /* @__PURE__ */ new Map();
  simulationNodes.forEach((n) => nodeMap.set(n.id, n));
  const linkData = edges.map((edge) => ({
    source: nodeMap.get(edge.from),
    target: nodeMap.get(edge.to),
    kind: edge.kind
  }));
  const spineCount = nodes.filter((n) => n.kind === "spine").length;
  const spineTargetXById = /* @__PURE__ */ new Map();
  nodes.forEach((n) => {
    if (n.kind !== "spine") return;
    const step = n.nodeRef.stepNumber ?? 0;
    spineTargetXById.set(n.id, 500 + step * SPINE_SPACING_X);
  });
  const simulation$1 = simulation(simulationNodes).alphaDecay(0.02).velocityDecay(0.4).force(
    "link",
    link$2(linkData).id((d) => d.id).distance((d) => {
      const s = d.source;
      const t = d.target;
      const sR = Math.sqrt(s.node.width * s.node.width + s.node.height * s.node.height) / 2;
      const tR = Math.sqrt(t.node.width * t.node.width + t.node.height * t.node.height) / 2;
      const base = d.kind === "spine" ? SPINE_LINK_DISTANCE : ATTACHMENT_LINK_DISTANCE;
      return base + sR + tR + COLLISION_PADDING;
    }).strength(0.8)
  ).force(
    "charge",
    manyBody().strength(CHARGE_STRENGTH).distanceMax(2e3)
    // Increased repulsion range for better spacing
  ).force(
    "collision",
    collide().radius((d) => {
      const node = d.node;
      const radius = Math.sqrt(node.width * node.width + node.height * node.height) / 2 + COLLISION_PADDING;
      return radius;
    }).strength(1)
    // Increased strength to ensure no overlaps
  ).force("center", center(800, SPINE_CENTER_Y).strength(0.02)).force("spineX", x$1((d) => {
    if (d.node.kind !== "spine") return d.x ?? 0;
    return spineTargetXById.get(d.id) ?? (d.x ?? 0);
  }).strength((d) => d.node?.kind === "spine" ? 0.5 : 0)).force("spineY", y$1((d) => {
    if (d.node.kind !== "spine") return d.y ?? 0;
    return SPINE_CENTER_Y;
  }).strength((d) => d.node?.kind === "spine" ? 0.6 : 0)).force("sideX", x$1((d) => {
    if (d.node.kind !== "side") return d.x ?? 0;
    const parentId = parentMap.get(d.id);
    const parentTargetX = parentId ? spineTargetXById.get(parentId) ?? 800 : 800;
    const attachmentEdge = edges.find((e) => e.kind === "attachment" && e.to === d.id);
    const rel = attachmentEdge?.relationshipType;
    const cluster = getRelationshipClusterParams(rel, parentId ?? "NO_PARENT");
    return parentTargetX + Math.cos(cluster.angle) * cluster.radius;
  }).strength((d) => d.node?.kind === "side" ? 0.3 : 0)).force("sideY", y$1((d) => {
    if (d.node.kind !== "side") return d.y ?? 0;
    const parentId = parentMap.get(d.id);
    const parentTargetY = SPINE_CENTER_Y;
    const attachmentEdge = edges.find((e) => e.kind === "attachment" && e.to === d.id);
    const rel = attachmentEdge?.relationshipType;
    const cluster = getRelationshipClusterParams(rel, parentId ?? "NO_PARENT");
    return parentTargetY + Math.sin(cluster.angle) * cluster.radius;
  }).strength((d) => d.node?.kind === "side" ? 0.3 : 0));
  for (let i = 0; i < SIMULATION_ITERATIONS; i++) {
    simulation$1.tick();
  }
  simulation$1.stop();
  simulationNodes.forEach((simNode) => {
    if (!isFinite(simNode.x) || !isFinite(simNode.y)) {
      console.warn(`[placeNodesForce] Invalid position for node ${simNode.id}, resetting`);
      const initial = getInitialPosition(simNode.node, nodes);
      simNode.x = initial.x;
      simNode.y = initial.y;
    }
  });
  const positionedNodes = simulationNodes.map((simNode) => {
    const positioned = {
      ...simNode.node,
      x: simNode.x,
      y: simNode.y,
      ports: calculatePorts({
        ...simNode.node,
        x: simNode.x,
        y: simNode.y
      })
    };
    return positioned;
  });
  if (positionedNodes.length > 0) {
    const minX = Math.min(...positionedNodes.map((n) => n.x));
    const minY = Math.min(...positionedNodes.map((n) => n.y));
    const maxX = Math.max(...positionedNodes.map((n) => n.x + n.width));
    const maxY = Math.max(...positionedNodes.map((n) => n.y + n.height));
    console.log("[placeNodesForce] Result:", {
      totalPositioned: positionedNodes.length,
      spine: positionedNodes.filter((n) => n.kind === "spine").length,
      side: positionedNodes.filter((n) => n.kind === "side").length,
      bounds: { x: minX, y: minY, width: maxX - minX, height: maxY - minY }
    });
  }
  return positionedNodes;
}

function routeEdgesDirect(edges, nodes) {
  if (edges.length === 0) {
    return [];
  }
  const nodeMap = /* @__PURE__ */ new Map();
  nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });
  const routedEdges = [];
  edges.forEach((edge) => {
    const fromNode = nodeMap.get(edge.from);
    const toNode = nodeMap.get(edge.to);
    if (!fromNode || !toNode) {
      console.warn(`[routeEdgesDirect] Missing node for edge ${edge.from} -> ${edge.to}`, {
        edge,
        fromNodeExists: !!fromNode,
        toNodeExists: !!toNode,
        availableNodeIds: Array.from(nodeMap.keys())
      });
      return;
    }
    let fromPort, toPort;
    if (edge.kind === "attachment") {
      const dy = toNode.y - fromNode.y;
      if (dy < 0) {
        fromPort = {
          side: "top",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y
          // Top edge of card
        };
        toPort = {
          side: "bottom",
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height
          // Bottom edge of card
        };
      } else {
        fromPort = {
          side: "bottom",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height
          // Bottom edge of card
        };
        toPort = {
          side: "top",
          x: toNode.x + toNode.width / 2,
          y: toNode.y
          // Top edge of card
        };
      }
    } else {
      const ports = selectPorts(fromNode, toNode);
      fromPort = ports.fromPort;
      toPort = ports.toPort;
      if (fromPort.side === "top") {
        fromPort = {
          side: "top",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y
        };
      } else if (fromPort.side === "bottom") {
        fromPort = {
          side: "bottom",
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height
        };
      } else if (fromPort.side === "left") {
        fromPort = {
          side: "left",
          x: fromNode.x,
          y: fromNode.y + HEADER_HEIGHT / 2
        };
      } else if (fromPort.side === "right") {
        fromPort = {
          side: "right",
          x: fromNode.x + fromNode.width,
          y: fromNode.y + HEADER_HEIGHT / 2
        };
      }
      if (toPort.side === "top") {
        toPort = {
          side: "top",
          x: toNode.x + toNode.width / 2,
          y: toNode.y
        };
      } else if (toPort.side === "bottom") {
        toPort = {
          side: "bottom",
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height
        };
      } else if (toPort.side === "left") {
        toPort = {
          side: "left",
          x: toNode.x,
          y: toNode.y + HEADER_HEIGHT / 2
        };
      } else if (toPort.side === "right") {
        toPort = {
          side: "right",
          x: toNode.x + toNode.width,
          y: toNode.y + HEADER_HEIGHT / 2
        };
      }
    }
    routedEdges.push({
      ...edge,
      points: [
        { x: fromPort.x, y: fromPort.y },
        { x: toPort.x, y: toPort.y }
      ],
      fromPort,
      toPort
    });
  });
  return routedEdges;
}

function computeRootCauseLayout(analysis, options = {}) {
  const { engine = "force", enableBundling = true } = options;
  const { nodes, edges } = buildLayoutModel(analysis);
  if (nodes.length === 0) {
    return {
      nodes: [],
      edges: [],
      bounds: { x: 0, y: 0, width: 0, height: 0 }
    };
  }
  let positionedNodes;
  if (engine === "force") {
    positionedNodes = placeNodesForce(nodes, edges);
  } else {
    positionedNodes = placeNodes(nodes, edges);
  }
  const rawBounds = calculateBounds(positionedNodes);
  const boundsPadding = engine === "force" ? 240 : 80;
  const bounds = {
    x: rawBounds.x - boundsPadding,
    y: rawBounds.y - boundsPadding,
    width: rawBounds.width + boundsPadding * 2,
    height: rawBounds.height + boundsPadding * 2
  };
  let routedEdges;
  if (engine === "force") {
    routedEdges = routeEdgesDirect(edges, positionedNodes);
  } else {
    routedEdges = routeEdges(edges, positionedNodes, bounds);
  }
  if (enableBundling) {
    routedEdges = bundleEdges(routedEdges);
  }
  return {
    nodes: positionedNodes,
    edges: routedEdges,
    bounds
  };
}

const DiffLineView = ({ line }) => {
  const styleMap = {
    add: "text-emerald-400 bg-emerald-500/10",
    remove: "text-red-400 bg-red-500/10",
    context: "text-[var(--color-text-primary)]",
    gap: "text-[var(--color-text-muted)] italic"
  };
  const prefixMap = {
    add: "+",
    remove: "-",
    context: " ",
    gap: "…"
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: `flex gap-2 px-2 rounded ${styleMap[line.type]}`, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "select-none w-3 text-[var(--color-text-muted)]", children: prefixMap[line.type] }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "whitespace-pre-wrap break-all", children: line.content })
  ] });
};
const SignificanceBadge = ({ significance }) => {
  if (!significance) return null;
  const score = significance.score;
  const percentage = Math.round(score * 100);
  const getColorClasses = () => {
    if (score >= 0.7) return "bg-red-500/20 text-red-400 border-red-500/30";
    if (score >= 0.4) return "bg-amber-500/20 text-amber-400 border-amber-500/30";
    return "bg-gray-500/20 text-gray-400 border-gray-500/30";
  };
  const getLabel = () => {
    if (score >= 0.7) return "High";
    if (score >= 0.4) return "Med";
    return "Low";
  };
  const tooltip = significance.reasons.length > 0 ? significance.reasons.join(", ") : `Significance: ${percentage}%`;
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "div",
    {
      className: `text-[8px] px-1.5 py-0.5 rounded border ${getColorClasses()} cursor-help`,
      title: tooltip,
      children: [
        getLabel(),
        " ",
        percentage,
        "%"
      ]
    }
  );
};
const RootCauseView = ({
  analysis,
  initialResourceUID,
  initialTimestamp,
  initialLookbackMs,
  onClose,
  onRefresh,
  error
}) => {
  console.log("[RootCauseView] Component rendered with props:", {
    hasAnalysis: !!analysis,
    hasIncident: !!analysis?.incident,
    hasGraph: !!analysis?.incident?.graph,
    nodeCount: analysis?.incident?.graph?.nodes?.length || 0,
    error
  });
  const svgRef = reactExports.useRef(null);
  const containerRef = reactExports.useRef(null);
  const graphContainerRef = reactExports.useRef(null);
  const [selectedNodeId, setSelectedNodeId] = reactExports.useState(null);
  const [selectedEventIndex, setSelectedEventIndex] = reactExports.useState(0);
  const [containerSize, setContainerSize] = reactExports.useState({ width: 0, height: 0 });
  const [bottomPanelHeight, setBottomPanelHeight] = reactExports.useState(208);
  const [isCanvasReady, setIsCanvasReady] = reactExports.useState(false);
  const [isResizing, setIsResizing] = reactExports.useState(false);
  const resizeStartY = reactExports.useRef(0);
  const resizeStartHeight = reactExports.useRef(0);
  const [showFullDiff, setShowFullDiff] = reactExports.useState(false);
  const [changeTypeFilter, setChangeTypeFilter] = reactExports.useState(/* @__PURE__ */ new Set(["spec", "status", "metadata", "other"]));
  const { formatTime } = useSettings();
  const [transform, setTransform] = reactExports.useState(identity);
  const zoomRef = reactExports.useRef(null);
  const selectedNode = reactExports.useMemo(() => {
    if (!selectedNodeId) return null;
    return analysis.incident.graph.nodes.find((n) => n.id === selectedNodeId) || null;
  }, [selectedNodeId, analysis]);
  const [editableUID, setEditableUID] = reactExports.useState(initialResourceUID);
  const [editableTimestamp, setEditableTimestamp] = reactExports.useState(initialTimestamp);
  const [editableLookback, setEditableLookback] = reactExports.useState(initialLookbackMs);
  const [isLoading, setIsLoading] = reactExports.useState(false);
  const parseEventData = (data) => {
    if (!data) return null;
    try {
      if (data.startsWith("eyJ")) {
        const decoded = atob(data);
        return JSON.parse(decoded);
      } else {
        return JSON.parse(data);
      }
    } catch (e) {
      console.warn("Failed to parse event data:", e, "Data preview:", data.substring(0, 50));
      return null;
    }
  };
  if (!analysis || !analysis.incident || !analysis.incident.graph || analysis.incident.graph.nodes.length === 0) {
    console.error("[RootCauseView] Invalid analysis data - showing error screen:", {
      hasAnalysis: !!analysis,
      hasIncident: !!analysis?.incident,
      hasGraph: !!analysis?.incident?.graph,
      nodeCount: analysis?.incident?.graph?.nodes?.length || 0
    });
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "fixed inset-0 z-[100] bg-[var(--color-app-bg)] flex items-center justify-center", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-red-400 mb-4", children: "Invalid root cause analysis data" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: onClose,
          className: "px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white",
          children: "Close"
        }
      )
    ] }) });
  }
  reactExports.useEffect(() => {
    if (!containerRef.current) return;
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setContainerSize((prev) => {
          if (prev.width === width && prev.height === height) {
            return prev;
          }
          return { width, height };
        });
      }
    });
    resizeObserver.observe(containerRef.current);
    return () => resizeObserver.disconnect();
  }, []);
  reactExports.useEffect(() => {
    if (!graphContainerRef.current) return;
    const zoomBehavior = zoom().scaleExtent([0.1, 3]).on("zoom", (event) => {
      setTransform(event.transform);
    });
    zoomRef.current = zoomBehavior;
    const selection = select(graphContainerRef.current);
    selection.call(zoomBehavior);
    return () => {
      selection.on(".zoom", null);
    };
  }, []);
  reactExports.useEffect(() => {
    if (!selectedNodeId && analysis?.incident?.graph && analysis.incident.graph.nodes.length > 0) {
      const spineNodes = analysis.incident.graph.nodes.filter((n) => n.nodeType === "SPINE").sort((a, b) => (a.stepNumber || 0) - (b.stepNumber || 0));
      if (spineNodes.length > 0) {
        setSelectedNodeId(spineNodes[0].id);
      }
    }
  }, [analysis, selectedNodeId]);
  reactExports.useEffect(() => {
    setIsLoading(false);
    setIsCanvasReady(false);
    hasInitialFit.current = false;
  }, [analysis]);
  reactExports.useEffect(() => {
    if (error) {
      setIsLoading(false);
    }
  }, [error]);
  const unifiedTimeline = reactExports.useMemo(() => {
    if (!selectedNode) return [];
    console.log("[unifiedTimeline] Processing selectedNode:", {
      id: selectedNode.id,
      kind: selectedNode.resource.kind,
      hasAllEvents: !!selectedNode.allEvents,
      allEventsCount: selectedNode.allEvents?.length || 0,
      hasK8sEvents: !!selectedNode.k8sEvents,
      k8sEventsCount: selectedNode.k8sEvents?.length || 0
    });
    const timeline = [];
    if (selectedNode.allEvents) {
      selectedNode.allEvents.forEach((evt, idx) => {
        const currentData = parseEventData(evt.data);
        let previousData = null;
        if (idx > 0) {
          previousData = parseEventData(selectedNode.allEvents[idx - 1].data);
        }
        let changeCategories = detectChangeCategories(previousData, currentData);
        if (!changeCategories.spec && !changeCategories.status && !changeCategories.metadata && !changeCategories.other) {
          changeCategories = { spec: false, status: false, metadata: false, other: true };
        }
        console.log("[unifiedTimeline] Event", idx, "categories:", changeCategories);
        timeline.push({
          id: evt.eventId,
          timestamp: new Date(evt.timestamp),
          type: "change",
          changeEvent: evt,
          changeCategories
        });
      });
    }
    if (selectedNode.k8sEvents) {
      selectedNode.k8sEvents.forEach((evt) => {
        if (!evt.eventId || evt.eventId === "" || new Date(evt.timestamp).getTime() <= 0) {
          return;
        }
        timeline.push({
          id: evt.eventId,
          timestamp: new Date(evt.timestamp),
          type: "k8s",
          k8sEvent: evt
        });
      });
    }
    timeline.sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());
    console.log("[unifiedTimeline] Created timeline with", timeline.length, "events");
    return timeline;
  }, [selectedNode]);
  const filteredTimeline = reactExports.useMemo(() => {
    const filtered = unifiedTimeline.filter((event) => {
      if (event.type === "k8s") return true;
      if (event.changeCategories) {
        const hasMatchingCategory = changeTypeFilter.has("spec") && event.changeCategories.spec || changeTypeFilter.has("status") && event.changeCategories.status || changeTypeFilter.has("metadata") && event.changeCategories.metadata || changeTypeFilter.has("other") && event.changeCategories.other;
        return hasMatchingCategory;
      }
      return true;
    });
    console.log("[filteredTimeline] Filter result:", {
      total: unifiedTimeline.length,
      filtered: filtered.length,
      filters: Array.from(changeTypeFilter)
    });
    return filtered;
  }, [unifiedTimeline, changeTypeFilter]);
  reactExports.useEffect(() => {
    const handleKeyDown = (e) => {
      if (!filteredTimeline || filteredTimeline.length === 0) return;
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setSelectedEventIndex((prev) => Math.max(0, prev - 1));
      } else if (e.key === "ArrowDown") {
        e.preventDefault();
        setSelectedEventIndex((prev) => Math.min(filteredTimeline.length - 1, prev + 1));
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [filteredTimeline]);
  reactExports.useEffect(() => {
    setSelectedEventIndex(0);
  }, [selectedNode]);
  reactExports.useEffect(() => {
    if (selectedEventIndex >= 0 && filteredTimeline && filteredTimeline.length > 0) {
      const eventElement = document.querySelector(`[data-event-index="${selectedEventIndex}"]`);
      if (eventElement) {
        eventElement.scrollIntoView({
          behavior: "smooth",
          block: "nearest",
          inline: "nearest"
        });
      }
    }
  }, [selectedEventIndex, filteredTimeline]);
  reactExports.useEffect(() => {
    const handleMouseMove = (e) => {
      if (!containerRef.current) return;
      const deltaY = resizeStartY.current - e.clientY;
      const newHeight = resizeStartHeight.current + deltaY;
      const containerRect = containerRef.current.getBoundingClientRect();
      const minHeight = 200;
      const maxHeight = containerRect.height * 0.8;
      const clampedHeight = Math.max(minHeight, Math.min(maxHeight, newHeight));
      setBottomPanelHeight(clampedHeight);
    };
    const handleMouseUp = () => {
      setIsResizing(false);
      document.body.style.userSelect = "";
      document.body.style.cursor = "";
    };
    if (isResizing) {
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
      document.body.style.userSelect = "none";
      document.body.style.cursor = "ns-resize";
    }
    return () => {
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isResizing]);
  const convertEventDiffToDiffLines = (diffs) => {
    const lines = [];
    for (const d of diffs) {
      if (d.op === "remove" || d.op === "replace") {
        lines.push({
          type: "remove",
          content: `${d.path}: ${JSON.stringify(d.old)}`
        });
      }
      if (d.op === "add" || d.op === "replace") {
        lines.push({
          type: "add",
          content: `${d.path}: ${JSON.stringify(d.new)}`
        });
      }
    }
    return lines;
  };
  const convertObjectToContextLines = (obj) => {
    const lines = [];
    const jsonStr = JSON.stringify(obj, null, 2);
    const jsonLines = jsonStr.split("\n");
    for (const line of jsonLines) {
      lines.push({
        type: "context",
        content: line
      });
    }
    return lines;
  };
  const convertSnapshotToDiffLines = (snapshot) => {
    const lines = [];
    const jsonStr = JSON.stringify(snapshot, null, 2);
    const jsonLines = jsonStr.split("\n");
    for (const line of jsonLines) {
      lines.push({
        type: "add",
        content: line
      });
    }
    return lines;
  };
  const applyDiffsToSnapshot = (snapshot, diffs) => {
    const result = JSON.parse(JSON.stringify(snapshot));
    for (const diff of diffs) {
      const pathParts = diff.path.split(".");
      let current = result;
      for (let i = 0; i < pathParts.length - 1; i++) {
        const part = pathParts[i];
        if (current[part] === void 0) {
          current[part] = {};
        }
        current = current[part];
      }
      const lastPart = pathParts[pathParts.length - 1];
      switch (diff.op) {
        case "add":
        case "replace":
          current[lastPart] = diff.new;
          break;
        case "remove":
          delete current[lastPart];
          break;
      }
    }
    return result;
  };
  const reconstructFullResource = (eventIndex) => {
    if (!filteredTimeline) return null;
    let snapshot = null;
    let snapshotIndex = -1;
    for (let i = 0; i <= eventIndex; i++) {
      const item = filteredTimeline[i];
      if (item.type === "change" && item.changeEvent?.fullSnapshot) {
        snapshot = JSON.parse(JSON.stringify(item.changeEvent.fullSnapshot));
        snapshotIndex = i;
        break;
      }
    }
    if (!snapshot) return null;
    for (let i = snapshotIndex + 1; i <= eventIndex; i++) {
      const item = filteredTimeline[i];
      if (item.type === "change" && item.changeEvent?.diff) {
        snapshot = applyDiffsToSnapshot(snapshot, item.changeEvent.diff);
      }
    }
    return snapshot;
  };
  const eventDiff = reactExports.useMemo(() => {
    if (!filteredTimeline || filteredTimeline.length === 0) {
      return null;
    }
    const selectedItem = filteredTimeline[selectedEventIndex];
    if (!selectedItem || selectedItem.type !== "change" || !selectedItem.changeEvent) {
      return null;
    }
    const changeEvent = selectedItem.changeEvent;
    const hasNewFormat = changeEvent.diff !== void 0 || changeEvent.fullSnapshot !== void 0;
    if (hasNewFormat) {
      if (showFullDiff) {
        const fullResource = reconstructFullResource(selectedEventIndex);
        if (fullResource) {
          return convertObjectToContextLines(fullResource);
        }
      }
      if (changeEvent.diff && changeEvent.diff.length > 0) {
        return convertEventDiffToDiffLines(changeEvent.diff);
      }
      if (changeEvent.fullSnapshot) {
        return convertSnapshotToDiffLines(changeEvent.fullSnapshot);
      }
      return null;
    }
    if (!changeEvent.data) {
      return null;
    }
    let previousChangeEvent = null;
    for (let i = selectedEventIndex - 1; i >= 0; i--) {
      if (filteredTimeline[i].type === "change" && filteredTimeline[i].changeEvent?.data) {
        previousChangeEvent = filteredTimeline[i].changeEvent;
        break;
      }
    }
    const currentData = parseEventData(changeEvent.data);
    const previousData = parseEventData(previousChangeEvent?.data);
    if (!currentData) {
      return null;
    }
    const contextLines = showFullDiff ? 1e3 : 3;
    return diffJsonWithContext(previousData, currentData, contextLines);
  }, [filteredTimeline, selectedEventIndex, showFullDiff]);
  const layoutResult = reactExports.useMemo(() => {
    console.log("[RootCauseView] layoutResult useMemo executing");
    try {
      if (!analysis?.incident?.graph || analysis.incident.graph.nodes.length === 0) {
        console.warn("[RootCauseView] No graph found in analysis");
        return null;
      }
      console.log("[RootCauseView] Computing layout for graph with", analysis.incident.graph.nodes.length, "nodes");
      const result = computeRootCauseLayout(analysis);
      console.log("[RootCauseView] Layout result:", result ? `${result.nodes.length} nodes positioned, bounds:` : "null", result?.bounds);
      return result;
    } catch (error2) {
      console.error("[RootCauseView] Error computing layout:", error2);
      return null;
    }
  }, [analysis]);
  const nodePositions = reactExports.useMemo(() => {
    console.log("[RootCauseView] nodePositions useMemo executing, layoutResult:", !!layoutResult);
    if (!layoutResult) {
      console.log("[RootCauseView] No layoutResult, returning empty positions");
      return [];
    }
    const positions = [];
    layoutResult.nodes.forEach((layoutNode, idx) => {
      positions.push({
        x: layoutNode.x,
        y: layoutNode.y,
        width: layoutNode.width,
        height: layoutNode.height,
        node: layoutNode.nodeRef,
        index: idx,
        layer: layoutNode.nodeRef.stepNumber || 0
      });
    });
    console.log("[RootCauseView] Created", positions.length, "node positions");
    return positions;
  }, [layoutResult]);
  const edgeRoutes = reactExports.useMemo(() => {
    if (!layoutResult) return [];
    return layoutResult.edges.map((edge) => ({
      from: edge.from,
      to: edge.to,
      kind: edge.kind,
      relationshipType: edge.relationshipType,
      points: edge.points
    }));
  }, [layoutResult]);
  reactExports.useEffect(() => {
    if (!svgRef.current || edgeRoutes.length === 0 || nodePositions.length === 0) {
      return;
    }
    const svg = select(svgRef.current);
    svg.selectAll("*").remove();
    const defs = svg.append("defs");
    defs.append("marker").attr("id", "arrowhead-normal").attr("markerWidth", 4).attr("markerHeight", 4).attr("refX", 3.5).attr("refY", 2).attr("orient", "auto").append("path").attr("d", "M 0 0 L 4 2 L 0 4 z").attr("fill", "#64748b");
    defs.append("marker").attr("id", "arrowhead-highlight").attr("markerWidth", 5).attr("markerHeight", 5).attr("refX", 4.5).attr("refY", 2.5).attr("orient", "auto").append("path").attr("d", "M 0 0 L 5 2.5 L 0 5 z").attr("fill", "#60a5fa");
    defs.append("marker").attr("id", "arrowhead-related").attr("markerWidth", 4).attr("markerHeight", 4).attr("refX", 3.5).attr("refY", 2).attr("orient", "auto").append("path").attr("d", "M 0 0 L 4 2 L 0 4 z").attr("fill", "#9ca3af");
    defs.append("marker").attr("id", "arrowhead-related-highlight").attr("markerWidth", 5).attr("markerHeight", 5).attr("refX", 4.5).attr("refY", 2.5).attr("orient", "auto").append("path").attr("d", "M 0 0 L 5 2.5 L 0 5 z").attr("fill", "#a78bfa");
    const arrowGroup = svg.append("g").attr("class", "arrows");
    const findNodeById = (id) => {
      return nodePositions.find((n) => n.node.id === id);
    };
    edgeRoutes.forEach((edge) => {
      const fromNode = findNodeById(edge.from);
      const toNode = findNodeById(edge.to);
      if (!fromNode || !toNode || edge.points.length < 2) {
        return;
      }
      const isConnected = selectedNodeId && (fromNode.node.id === selectedNodeId || toNode.node.id === selectedNodeId);
      const offset = 1e4;
      let pathData = `M ${edge.points[0].x + offset} ${edge.points[0].y + offset}`;
      for (let i = 1; i < edge.points.length; i++) {
        pathData += ` L ${edge.points[i].x + offset} ${edge.points[i].y + offset}`;
      }
      const isAttachment = edge.kind === "attachment";
      const strokeColor = isAttachment ? isConnected ? "#a78bfa" : "#8b5cf6" : isConnected ? "#60a5fa" : "#64748b";
      const strokeWidth = isConnected ? isAttachment ? 2.5 : 2.5 : isAttachment ? 2 : 1.5;
      const opacity = isAttachment ? isConnected ? 1 : 0.6 : isConnected ? 1 : 0.4;
      const markerId = isAttachment ? isConnected ? "arrowhead-related-highlight" : "arrowhead-related" : isConnected ? "arrowhead-highlight" : "arrowhead-normal";
      arrowGroup.append("path").attr("d", pathData).attr("stroke", strokeColor).attr("stroke-width", strokeWidth).attr("fill", "none").attr("opacity", opacity).attr("stroke-dasharray", isAttachment ? "6,4" : "none").attr("marker-end", `url(#${markerId})`);
      const startPoint = edge.points[0];
      const endPoint = edge.points[edge.points.length - 1];
      const connectorRadius = isConnected ? isAttachment ? 4 : 5 : isAttachment ? 3 : 4;
      const connectorStrokeWidth = isConnected ? 2 : 1.5;
      if (!isConnected) {
        arrowGroup.append("circle").attr("cx", startPoint.x + offset).attr("cy", startPoint.y + offset).attr("r", connectorRadius).attr("fill", "var(--color-surface)").attr("stroke", strokeColor).attr("stroke-width", connectorStrokeWidth);
        arrowGroup.append("circle").attr("cx", endPoint.x + offset).attr("cy", endPoint.y + offset).attr("r", connectorRadius).attr("fill", "var(--color-surface)").attr("stroke", strokeColor).attr("stroke-width", connectorStrokeWidth);
      } else {
        arrowGroup.append("circle").attr("cx", startPoint.x + offset).attr("cy", startPoint.y + offset).attr("r", connectorRadius).attr("fill", strokeColor).attr("stroke", isAttachment ? "#8b5cf6" : "#3b82f6").attr("stroke-width", connectorStrokeWidth);
        arrowGroup.append("circle").attr("cx", endPoint.x + offset).attr("cy", endPoint.y + offset).attr("r", connectorRadius).attr("fill", strokeColor).attr("stroke", isAttachment ? "#8b5cf6" : "#3b82f6").attr("stroke-width", connectorStrokeWidth);
      }
      if (edge.relationshipType && edge.relationshipType !== "SYMPTOM") {
        const midIdx = Math.floor(edge.points.length / 2);
        const midPoint = edge.points[midIdx];
        let labelX = midPoint.x + offset;
        let labelY = midPoint.y + offset;
        if (edge.points.length === 2) {
          labelX = (edge.points[0].x + edge.points[1].x) / 2 + offset;
          labelY = (edge.points[0].y + edge.points[1].y) / 2 + offset;
        }
        const labelText = edge.relationshipType.replace(/_/g, " ");
        const estimatedWidth = labelText.length * 7 + 16;
        const offsetY = edge.kind === "attachment" ? -18 : -18;
        arrowGroup.append("rect").attr("x", labelX - estimatedWidth / 2).attr("y", labelY + offsetY - 10).attr("width", estimatedWidth).attr("height", 18).attr("fill", isConnected ? edge.kind === "attachment" ? "rgba(139, 92, 246, 0.95)" : "rgba(59, 130, 246, 0.95)" : "var(--color-app-bg)").attr("stroke", isConnected ? edge.kind === "attachment" ? "#a78bfa" : "#60a5fa" : "#64748b").attr("stroke-width", 1).attr("rx", 3);
        arrowGroup.append("text").attr("x", labelX).attr("y", labelY + offsetY + 3).attr("fill", isConnected ? "#ffffff" : "#94a3b8").attr("font-size", "10px").attr("font-weight", "700").attr("text-anchor", "middle").attr("letter-spacing", "0.05em").text(labelText);
      }
    });
  }, [edgeRoutes, nodePositions, selectedNodeId]);
  const getStatusColor = (index, total) => {
    if (index === 0) return "bg-blue-500";
    if (index === total - 1) return "bg-red-500";
    return "bg-amber-500";
  };
  const getStatusColorFromStatus = (status) => {
    if (!status) return "bg-gray-400 dark:bg-gray-600";
    switch (status) {
      case "Ready":
        return "bg-green-500";
      case "Warning":
        return "bg-yellow-500";
      case "Error":
        return "bg-red-500";
      case "Terminating":
        return "bg-orange-500";
      case "Unknown":
      default:
        return "bg-gray-400 dark:bg-gray-600";
    }
  };
  const getStatusTextColor = (status) => {
    if (!status) return "text-gray-400 dark:text-gray-500";
    switch (status) {
      case "Ready":
        return "text-green-400";
      case "Warning":
        return "text-yellow-400";
      case "Error":
        return "text-red-400";
      case "Terminating":
        return "text-orange-400";
      case "Unknown":
      default:
        return "text-gray-400";
    }
  };
  const formatDuration = (ms) => {
    if (ms < 6e4) return `${ms / 1e3}s`;
    if (ms < 36e5) return `${ms / 6e4}m`;
    return `${ms / 36e5}h`;
  };
  const formatDateTimeLocal = (date) => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    const hours = String(date.getHours()).padStart(2, "0");
    const minutes = String(date.getMinutes()).padStart(2, "0");
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  };
  const hasInitialFit = reactExports.useRef(false);
  const fitToView = React.useCallback(() => {
    if (!layoutResult || !graphContainerRef.current || !containerRef.current) {
      console.log("[fitToView] Early return - missing layoutResult or container");
      return;
    }
    const containerRect = containerRef.current.getBoundingClientRect();
    const viewportWidth = containerRect.width;
    const viewportHeight = containerRect.height;
    if (viewportWidth <= 0 || viewportHeight <= 0) {
      console.log("[fitToView] Invalid viewport size:", { viewportWidth, viewportHeight });
      return;
    }
    const nodes = layoutResult.nodes;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const node of nodes) {
      minX = Math.min(minX, node.x);
      minY = Math.min(minY, node.y);
      maxX = Math.max(maxX, node.x + node.width);
      maxY = Math.max(maxY, node.y + node.height);
    }
    const tightBounds = {
      x: minX,
      y: minY,
      width: maxX - minX,
      height: maxY - minY
    };
    const padding = 40;
    console.log("[fitToView] Viewport (graph area):", { viewportWidth, viewportHeight });
    console.log("[fitToView] Tight bounds:", tightBounds);
    const scaleX = (viewportWidth - padding * 2) / tightBounds.width;
    const scaleY = (viewportHeight - padding * 2) / tightBounds.height;
    const scale = Math.max(0.1, Math.min(scaleX, scaleY, 2));
    console.log("[fitToView] Scale:", { scaleX, scaleY, finalScale: scale });
    const targetCenterX = tightBounds.x + tightBounds.width / 2;
    const targetCenterY = tightBounds.y + tightBounds.height / 2;
    console.log("[fitToView] Centering on tight bounds center");
    const scaledTargetCenterX = targetCenterX * scale;
    const scaledTargetCenterY = targetCenterY * scale;
    const viewportCenterX = viewportWidth / 2;
    const viewportCenterY = viewportHeight / 2;
    const translateX = viewportCenterX - scaledTargetCenterX;
    const translateY = viewportCenterY - scaledTargetCenterY;
    console.log("[fitToView] TargetCenter:", { targetCenterX, targetCenterY });
    console.log("[fitToView] Transform:", { translateX, translateY, scale });
    const newTransform = identity.translate(translateX, translateY).scale(scale);
    if (zoomRef.current && graphContainerRef.current) {
      select(graphContainerRef.current).transition().duration(300).call(zoomRef.current.transform, newTransform).on("end", () => {
        setIsCanvasReady(true);
      });
    } else {
      setIsCanvasReady(true);
    }
  }, [layoutResult]);
  reactExports.useEffect(() => {
    if (layoutResult && !hasInitialFit.current) {
      const timer = setTimeout(() => {
        hasInitialFit.current = true;
        fitToView();
      }, 200);
      return () => clearTimeout(timer);
    }
  }, [layoutResult, fitToView]);
  if (nodePositions.length === 0) {
    console.warn("[RootCauseView] No node positions calculated, showing fallback:", {
      hasLayoutResult: !!layoutResult,
      layoutNodes: layoutResult?.nodes?.length || 0,
      graphNodes: analysis?.incident?.graph?.nodes?.length || 0
    });
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "fixed inset-0 z-[100] bg-[var(--color-app-bg)] flex items-center justify-center", "data-testid": "root-cause-view", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "bg-red-500 text-white p-4 mb-4 rounded", children: "ROOT CAUSE VIEW IS RENDERING (but no positions)" }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("p", { className: "text-[var(--color-text-muted)] mb-4", children: [
        "Graph nodes: ",
        analysis.incident.graph?.nodes.length || 0
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: onClose,
          className: "px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white",
          children: "Close"
        }
      )
    ] }) });
  }
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "fixed inset-0 z-[100] bg-[var(--color-app-bg)] overflow-hidden flex flex-col", "data-testid": "root-cause-view", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "h-14 border-b border-[var(--color-border-soft)] px-6 flex items-center justify-between bg-[var(--color-surface)]", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-4", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("h1", { className: "text-lg font-semibold text-[var(--color-text-primary)]", children: "Root Cause Analysis" }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-sm text-[var(--color-text-muted)]", children: [
          analysis.incident.observedSymptom.resource.kind,
          "/",
          analysis.incident.observedSymptom.resource.name
        ] })
      ] }),
      onRefresh && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-3 text-sm", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-muted)] text-xs", children: "UID:" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "input",
            {
              type: "text",
              value: editableUID,
              onChange: (e) => {
                const newUID = e.target.value;
                setEditableUID(newUID);
                if (newUID.length > 0) {
                  setIsLoading(true);
                  onRefresh(newUID, editableTimestamp, editableLookback);
                }
              },
              className: "px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] font-mono w-64",
              placeholder: "Resource UID",
              disabled: isLoading
            }
          )
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "input",
          {
            type: "datetime-local",
            value: formatDateTimeLocal(editableTimestamp),
            onChange: (e) => {
              const newDate = new Date(e.target.value);
              if (!isNaN(newDate.getTime())) {
                setEditableTimestamp(newDate);
                setIsLoading(true);
                onRefresh(editableUID, newDate, editableLookback);
              }
            },
            className: "px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)]",
            disabled: isLoading
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsxs(
          "select",
          {
            value: editableLookback,
            onChange: (e) => {
              const newLookback = Number(e.target.value);
              setEditableLookback(newLookback);
              setIsLoading(true);
              onRefresh(editableUID, editableTimestamp, newLookback);
            },
            className: "px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)]",
            disabled: isLoading,
            children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 3e5, children: "5 minutes" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 6e5, children: "10 minutes" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 18e5, children: "30 minutes" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 36e5, children: "1 hour" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 216e5, children: "6 hours" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 864e5, children: "24 hours" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("option", { value: 1728e5, children: "48 hours" })
            ]
          }
        )
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: onClose,
          className: "px-4 py-2 text-sm bg-[var(--color-surface-muted)] hover:bg-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] transition-colors",
          children: "Close"
        }
      )
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 overflow-hidden bg-[#0a0e1a] relative", ref: containerRef, children: [
      layoutResult && isCanvasReady && /* @__PURE__ */ jsxRuntimeExports.jsxs(
        "button",
        {
          onClick: fitToView,
          className: "absolute top-4 right-4 z-20 px-3 py-1.5 text-xs bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] transition-colors shadow-lg flex items-center gap-1.5",
          title: "Fit all nodes to view",
          children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-3.5 h-3.5", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" }) }),
            "Reset Zoom"
          ]
        }
      ),
      layoutResult && !isCanvasReady && !isLoading && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 flex items-center justify-center z-10", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-3", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-muted)] text-sm", children: "Positioning graph..." })
      ] }) }),
      isLoading && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 bg-black/50 flex items-center justify-center z-[200]", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-3", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-white text-sm font-medium", children: "Loading analysis..." })
      ] }) }),
      error && !isLoading && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute top-4 left-1/2 transform -translate-x-1/2 z-[200] animate-in slide-in-from-top duration-300", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "bg-red-500/90 backdrop-blur-sm text-white px-4 py-3 rounded-lg shadow-lg max-w-md flex items-start gap-3", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5 flex-shrink-0 mt-0.5", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" }) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1", children: /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "font-medium text-sm", children: error }) })
      ] }) }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "div",
        {
          ref: graphContainerRef,
          className: "relative w-full h-full transition-opacity duration-300",
          style: {
            minHeight: layoutResult ? Math.max(400, layoutResult.bounds.height + 100) : 400,
            minWidth: layoutResult ? Math.max(400, layoutResult.bounds.width + 100) : 400,
            padding: "40px",
            opacity: isCanvasReady ? 1 : 0
          },
          children: /* @__PURE__ */ jsxRuntimeExports.jsxs(
            "div",
            {
              style: {
                position: "absolute",
                top: "-10000px",
                left: "-10000px",
                width: "20000px",
                height: "20000px",
                transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.k})`,
                transformOrigin: "10040px 10040px",
                backgroundImage: `
                linear-gradient(rgba(30, 41, 59, 0.5) 1px, transparent 1px),
                linear-gradient(90deg, rgba(30, 41, 59, 0.5) 1px, transparent 1px),
                radial-gradient(circle at 0px 0px, rgba(51, 65, 85, 0.6) 1px, transparent 1px)
              `,
                backgroundSize: "20px 20px, 20px 20px, 20px 20px",
                backgroundPosition: "0 0, 0 0, 0 0"
              },
              children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx(
                  "svg",
                  {
                    ref: svgRef,
                    className: "absolute pointer-events-none",
                    style: {
                      top: 0,
                      left: 0,
                      zIndex: 15,
                      width: "20000px",
                      height: "20000px"
                    }
                  }
                ),
                nodePositions.length === 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 flex items-center justify-center", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[var(--color-text-muted)] mb-4", children: "No graph data available" }),
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("p", { className: "text-sm text-[var(--color-text-muted)]", children: [
                    "Graph nodes: ",
                    analysis.incident.graph?.nodes.length || 0
                  ] })
                ] }) }) : /* @__PURE__ */ jsxRuntimeExports.jsx("div", { children: nodePositions.map((nodePos) => {
                  const node = nodePos.node;
                  const resource = node.resource;
                  const events = node.allEvents;
                  const isSpine = node.nodeType === "SPINE";
                  return /* @__PURE__ */ jsxRuntimeExports.jsx(
                    "div",
                    {
                      className: "absolute cursor-pointer transition-all duration-200",
                      style: {
                        left: nodePos.x + 1e4,
                        top: nodePos.y + 1e4,
                        width: 280,
                        height: nodePos.height,
                        zIndex: 10
                      },
                      onClick: () => setSelectedNodeId(node.id),
                      children: /* @__PURE__ */ jsxRuntimeExports.jsxs(
                        "div",
                        {
                          className: `
                  bg-white dark:bg-[#1e293b] border-2 rounded-lg overflow-hidden
                  ${!isSpine ? "border-dashed" : ""}
                  ${selectedNodeId === node.id ? "border-blue-500 shadow-[0_0_12px_rgba(59,130,246,0.3)]" : "border-gray-200 dark:border-slate-600 hover:border-blue-300 shadow-[0_2px_8px_rgba(0,0,0,0.2)]"}
                `,
                          style: { height: "100%", display: "flex", flexDirection: "column" },
                          children: [
                            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-3 border-b border-gray-100 dark:border-slate-700 bg-white dark:bg-slate-800/50", children: [
                              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-start justify-between mb-1.5", children: [
                                /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
                                  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `w-2.5 h-2.5 rounded-full ${!isSpine ? events && events.length > 0 ? "bg-orange-500" : "bg-gray-400" : getStatusColor(nodePos.index, nodePositions.filter((n) => n.node.nodeType === "SPINE").length)}` }),
                                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[10px] text-gray-500 dark:text-[var(--color-text-muted)] uppercase font-bold tracking-wider", children: resource.kind })
                                ] }),
                                node.changeEvent && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[10px] text-gray-400 dark:text-[var(--color-text-muted)]", children: new Date(node.changeEvent.timestamp).toLocaleTimeString() })
                              ] }),
                              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm font-bold text-gray-900 dark:text-[var(--color-text-primary)] truncate", title: resource.name, children: resource.name })
                            ] }),
                            events && events.length > 0 && (() => {
                              const counts = {
                                CREATE: 0,
                                DELETE: 0,
                                UPDATE: {
                                  total: 0,
                                  spec: 0,
                                  status: 0,
                                  metadata: 0
                                }
                              };
                              events.forEach((evt) => {
                                if (evt.eventType === "CREATE") {
                                  counts.CREATE++;
                                } else if (evt.eventType === "DELETE") {
                                  counts.DELETE++;
                                } else if (evt.eventType === "UPDATE") {
                                  counts.UPDATE.total++;
                                  if (evt.data) {
                                    try {
                                      const prevData = JSON.parse(evt.data);
                                      const categories = detectChangeCategories(prevData, prevData);
                                      if (categories.spec) counts.UPDATE.spec++;
                                      if (categories.status) counts.UPDATE.status++;
                                      if (categories.metadata) counts.UPDATE.metadata++;
                                    } catch (e) {
                                    }
                                  }
                                }
                              });
                              return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "px-3 py-2 bg-gray-50 dark:bg-slate-900/30", children: [
                                /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-wrap gap-1.5", children: [
                                  counts.CREATE > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400", children: [
                                    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "CREATE" }),
                                    /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "opacity-70", children: [
                                      "×",
                                      counts.CREATE
                                    ] })
                                  ] }),
                                  counts.UPDATE.total > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400", children: [
                                    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "UPDATE" }),
                                    /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "opacity-70", children: [
                                      "×",
                                      counts.UPDATE.total
                                    ] })
                                  ] }),
                                  counts.DELETE > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400", children: [
                                    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "DELETE" }),
                                    /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "opacity-70", children: [
                                      "×",
                                      counts.DELETE
                                    ] })
                                  ] })
                                ] }),
                                counts.UPDATE.total > 0 && (counts.UPDATE.spec > 0 || counts.UPDATE.status > 0 || counts.UPDATE.metadata > 0) && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-wrap gap-1.5 mt-1.5 pl-2 border-l-2 border-blue-300 dark:border-blue-700", children: [
                                  counts.UPDATE.spec > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[9px] font-semibold px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-950/50 text-blue-600 dark:text-blue-300", children: [
                                    ".spec ×",
                                    counts.UPDATE.spec
                                  ] }),
                                  counts.UPDATE.status > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[9px] font-semibold px-1.5 py-0.5 rounded bg-amber-50 dark:bg-amber-950/50 text-amber-600 dark:text-amber-300", children: [
                                    ".status ×",
                                    counts.UPDATE.status
                                  ] }),
                                  counts.UPDATE.metadata > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[9px] font-semibold px-1.5 py-0.5 rounded bg-purple-50 dark:bg-purple-950/50 text-purple-600 dark:text-purple-300", children: [
                                    ".metadata ×",
                                    counts.UPDATE.metadata
                                  ] })
                                ] })
                              ] });
                            })()
                          ]
                        }
                      )
                    },
                    node.id
                  );
                }) })
              ]
            }
          )
        }
      )
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs(
      "div",
      {
        className: "border-t border-[var(--color-border-soft)] bg-[var(--color-surface)] flex flex-col relative",
        style: { height: `${bottomPanelHeight}px` },
        children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "div",
            {
              className: "absolute top-0 left-0 right-0 h-2 -mt-1 cursor-ns-resize hover:bg-blue-500/30 transition-colors z-10 group",
              onMouseDown: (e) => {
                e.preventDefault();
                resizeStartY.current = e.clientY;
                resizeStartHeight.current = bottomPanelHeight;
                setIsResizing(true);
              },
              title: "Drag to resize",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-12 h-1 bg-[var(--color-border-soft)] group-hover:bg-blue-500/70 rounded-full transition-colors" })
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between px-4 py-2 border-b border-[var(--color-border-soft)]", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wider", children: "Event Timeline & Manifest Comparison" }),
            selectedNode && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)]", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono text-[var(--color-text-primary)]", children: selectedNode.resource.kind }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-2", children: "/" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono text-[var(--color-text-primary)]", children: selectedNode.resource.name })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex gap-2 text-[10px]", children: ["spec", "status", "metadata"].map((category) => {
              const isActive = changeTypeFilter.has(category);
              const colors = {
                spec: { bg: "bg-blue-500/20", text: "text-blue-400", activeBg: "bg-blue-500", activeText: "text-white" },
                status: { bg: "bg-amber-500/20", text: "text-amber-400", activeBg: "bg-amber-500", activeText: "text-white" },
                metadata: { bg: "bg-purple-500/20", text: "text-purple-400", activeBg: "bg-purple-500", activeText: "text-white" }
              }[category];
              return /* @__PURE__ */ jsxRuntimeExports.jsxs(
                "button",
                {
                  onClick: () => {
                    const newFilter = new Set(changeTypeFilter);
                    if (isActive) {
                      newFilter.delete(category);
                    } else {
                      newFilter.add(category);
                    }
                    setChangeTypeFilter(newFilter);
                    setSelectedEventIndex(0);
                  },
                  className: `px-2 py-1 rounded cursor-pointer transition-all ${isActive ? `${colors.activeBg} ${colors.activeText} font-semibold` : `${colors.bg} ${colors.text} hover:opacity-80`}`,
                  title: `${isActive ? "Hide" : "Show"} ${category} changes`,
                  children: [
                    ".",
                    category.toUpperCase()
                  ]
                },
                category
              );
            }) })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex overflow-hidden", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 min-w-0 border-r border-[var(--color-border-soft)] flex flex-col", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "px-4 py-2 border-b border-[var(--color-border-soft)] flex items-center justify-between", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[10px] font-semibold text-[var(--color-text-muted)] uppercase", children: "Manifest Diff" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx(
                  "button",
                  {
                    onClick: () => setShowFullDiff(!showFullDiff),
                    className: "text-[10px] px-2 py-1 rounded bg-[var(--color-surface-active)] hover:bg-[var(--color-surface-hover)] text-[var(--color-text-primary)] transition-colors",
                    children: showFullDiff ? "Show Changes Only" : "Show Full Resource"
                  }
                )
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1 overflow-y-auto overflow-x-auto p-4", children: filteredTimeline && filteredTimeline[selectedEventIndex] ? /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
                filteredTimeline[selectedEventIndex].type === "change" && eventDiff && eventDiff.length > 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "font-mono text-xs space-y-0", children: eventDiff.map((line, i) => /* @__PURE__ */ jsxRuntimeExports.jsx(DiffLineView, { line }, i)) }) : filteredTimeline[selectedEventIndex].type === "change" ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex items-center justify-center h-full text-[var(--color-text-muted)]", children: (() => {
                  const evt = filteredTimeline[selectedEventIndex].changeEvent;
                  if (evt?.diff && evt.diff.length === 0) {
                    return "No changes detected in this event";
                  }
                  if (selectedEventIndex === 0) {
                    return "First event - no previous state to compare";
                  }
                  return "No data available for diff";
                })() }) : null,
                filteredTimeline[selectedEventIndex].type === "k8s" && filteredTimeline[selectedEventIndex].k8sEvent && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "space-y-4", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "border border-[var(--color-border-soft)] rounded-lg p-4 bg-[var(--color-surface-muted)]", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-start gap-3 mb-3", children: [
                    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `px-2 py-1 rounded text-xs font-semibold ${filteredTimeline[selectedEventIndex].k8sEvent.type === "Warning" ? "bg-amber-500/20 text-amber-400" : filteredTimeline[selectedEventIndex].k8sEvent.type === "Error" ? "bg-red-500/20 text-red-400" : "bg-blue-500/20 text-blue-400"}`, children: filteredTimeline[selectedEventIndex].k8sEvent.type }),
                    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1", children: [
                      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm font-semibold text-[var(--color-text-primary)] mb-1", children: filteredTimeline[selectedEventIndex].k8sEvent.reason }),
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)]", children: [
                        "Count: ",
                        filteredTimeline[selectedEventIndex].k8sEvent.count,
                        " •",
                        " ",
                        "Source: ",
                        filteredTimeline[selectedEventIndex].k8sEvent.source || "Unknown"
                      ] })
                    ] })
                  ] }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-primary)] whitespace-pre-wrap", children: filteredTimeline[selectedEventIndex].k8sEvent.message })
                ] }) })
              ] }) : /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex items-center justify-center h-full text-[var(--color-text-muted)]", children: "Click a resource card to view events" }) })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "w-[35%] shrink-0 flex flex-col overflow-hidden", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "px-3 py-2 border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[10px] font-semibold text-[var(--color-text-muted)] uppercase tracking-wider", children: "Event Timeline" }),
                filteredTimeline && filteredTimeline.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[9px] text-[var(--color-text-muted)] mt-0.5", children: [
                  filteredTimeline.length,
                  " event",
                  filteredTimeline.length !== 1 ? "s" : "",
                  " • Use ↑↓ to navigate"
                ] })
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1 overflow-y-auto", children: filteredTimeline && filteredTimeline.length > 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "divide-y divide-[var(--color-border-soft)]", children: filteredTimeline.map((item, index) => {
                let borderColor = "!border-l-gray-500";
                if (item.type === "change" && item.changeEvent?.status) {
                  switch (item.changeEvent.status) {
                    case "Ready":
                      borderColor = "!border-l-green-500";
                      break;
                    case "Warning":
                      borderColor = "!border-l-yellow-500";
                      break;
                    case "Error":
                      borderColor = "!border-l-red-500";
                      break;
                    case "Terminating":
                      borderColor = "!border-l-orange-500";
                      break;
                    default:
                      borderColor = "!border-l-gray-500";
                  }
                } else if (item.type === "k8s") {
                  borderColor = "!border-l-blue-400";
                }
                return /* @__PURE__ */ jsxRuntimeExports.jsx(
                  "div",
                  {
                    "data-event-index": index,
                    onClick: () => setSelectedEventIndex(index),
                    className: `px-3 py-2 cursor-pointer transition-colors border-l-4 ${borderColor} ${index === selectedEventIndex ? "bg-blue-500/20" : "hover:bg-[var(--color-surface-active)]"}`,
                    children: item.type === "change" && item.changeEvent ? /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between mb-1", children: [
                        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
                          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `text-[10px] font-semibold ${item.changeEvent.eventType === "CREATE" ? "text-emerald-400" : item.changeEvent.eventType === "DELETE" ? "text-red-400" : "text-blue-400"}`, children: item.changeEvent.eventType }),
                          /* @__PURE__ */ jsxRuntimeExports.jsx(SignificanceBadge, { significance: item.changeEvent.significance })
                        ] }),
                        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)]", children: formatTime(item.timestamp) })
                      ] }),
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex gap-1.5 flex-wrap", children: [
                        item.changeCategories?.spec && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[8px] px-1.5 py-0.5 bg-blue-500/20 text-blue-400 rounded", children: ".spec" }),
                        item.changeCategories?.status && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[8px] px-1.5 py-0.5 bg-amber-500/20 text-amber-400 rounded", children: ".status" }),
                        item.changeCategories?.metadata && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[8px] px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded", children: ".metadata" })
                      ] }),
                      item.changeEvent.description && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)] mt-1 truncate", children: item.changeEvent.description })
                    ] }) : item.type === "k8s" && item.k8sEvent ? /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between mb-1", children: [
                        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
                          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[10px] font-semibold text-[var(--color-text-primary)]", children: "K8s Event" }),
                          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `text-[8px] px-1.5 py-0.5 rounded ${item.k8sEvent.type === "Warning" ? "bg-amber-500/20 text-amber-400" : item.k8sEvent.type === "Error" ? "bg-red-500/20 text-red-400" : "bg-blue-500/20 text-blue-400"}`, children: item.k8sEvent.type }),
                          /* @__PURE__ */ jsxRuntimeExports.jsx(SignificanceBadge, { significance: item.k8sEvent.significance })
                        ] }),
                        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)]", children: formatTime(item.timestamp) })
                      ] }),
                      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] font-semibold text-[var(--color-text-primary)] mb-0.5", children: item.k8sEvent.reason }),
                      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)] truncate", children: item.k8sEvent.message })
                    ] }) : null
                  },
                  item.id
                );
              }) }) : /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex items-center justify-center h-full px-3 text-center text-[10px] text-[var(--color-text-muted)]", children: selectedNode ? "No events in time window" : "Select a resource to view events" }) })
            ] })
          ] })
        ]
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "h-6 border-t border-[var(--color-border-soft)] bg-[var(--color-app-bg)] px-6 flex items-center gap-4 text-[10px] text-[var(--color-text-muted)]", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-1.5 h-1.5 rounded-full bg-green-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "READY" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-1.5 h-1.5 rounded-full bg-yellow-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "WARNING" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-1.5 h-1.5 rounded-full bg-red-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "ERROR" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-1.5 h-1.5 rounded-full bg-orange-500" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "TERMINATING" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "border-l border-[var(--color-border-soft)] h-4 mx-2" }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-emerald-400", children: "+" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "ADDITIONS" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-red-400", children: "-" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "DELETIONS" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "border-l border-[var(--color-border-soft)] h-4 mx-2" }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-6 h-0.5 bg-blue-400" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "CAUSAL CHAIN" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-6 h-0.5 border-t-2 border-dashed border-purple-400" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "RELATED RESOURCES" })
      ] })
    ] })
  ] });
};

const useTimeline = (options) => {
  const [resources, setResources] = reactExports.useState([]);
  const [loading, setLoading] = reactExports.useState(true);
  const [loadingMore, setLoadingMore] = reactExports.useState(false);
  const [error, setError] = reactExports.useState(null);
  const [totalCount, setTotalCount] = reactExports.useState(0);
  const [loadedCount, setLoadedCount] = reactExports.useState(0);
  const [hasMore, setHasMore] = reactExports.useState(false);
  const [nextCursor, setNextCursor] = reactExports.useState("");
  const abortControllerRef = reactExports.useRef(null);
  const accumulatedResourcesRef = reactExports.useRef([]);
  const batchTimeoutRef = reactExports.useRef(null);
  const filtersKey = reactExports.useMemo(() => {
    if (!options?.filters) return "";
    return JSON.stringify({
      namespace: options.filters.namespace || null,
      kind: options.filters.kind || null,
      namespaces: options.filters.namespaces || null,
      kinds: options.filters.kinds || null,
      group: options.filters.group || null,
      version: options.filters.version || null
    });
  }, [options?.filters?.namespace, options?.filters?.kind, options?.filters?.namespaces, options?.filters?.kinds, options?.filters?.group, options?.filters?.version]);
  const startTimeMs = options?.startTime?.getTime() ?? null;
  const endTimeMs = options?.endTime?.getTime() ?? null;
  const prevFiltersRef = reactExports.useRef("");
  const prevStartTimeRef = reactExports.useRef(null);
  const prevEndTimeRef = reactExports.useRef(null);
  const refreshToken = options?.refreshToken ?? 0;
  const prevRefreshTokenRef = reactExports.useRef(refreshToken);
  const fetchData = reactExports.useCallback(async (force = false, cursor) => {
    if (!options?.startTime || !options?.endTime) {
      setLoading(false);
      return;
    }
    const isLoadingMore = !!cursor;
    if (!isLoadingMore) {
      const timeRangeChanged = prevStartTimeRef.current !== startTimeMs || prevEndTimeRef.current !== endTimeMs;
      const filtersChanged = prevFiltersRef.current !== filtersKey;
      const refreshChanged = prevRefreshTokenRef.current !== refreshToken;
      if (!force && !timeRangeChanged && !filtersChanged && !refreshChanged) {
        return;
      }
      prevStartTimeRef.current = startTimeMs;
      prevEndTimeRef.current = endTimeMs;
      prevFiltersRef.current = filtersKey;
      prevRefreshTokenRef.current = refreshToken;
    }
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }
    if (batchTimeoutRef.current) {
      clearTimeout(batchTimeoutRef.current);
      batchTimeoutRef.current = null;
    }
    abortControllerRef.current = new AbortController();
    accumulatedResourcesRef.current = [];
    let isFirstBatch = true;
    const flushBatch = () => {
      if (accumulatedResourcesRef.current.length > 0) {
        const batchedResources = [...accumulatedResourcesRef.current];
        const batchSize = batchedResources.length;
        if (isFirstBatch) {
          setResources((prev) => [...prev, ...batchedResources]);
          setLoadedCount((prev) => prev + batchSize);
          setLoading(false);
          isFirstBatch = false;
        } else {
          reactExports.startTransition(() => {
            setResources((prev) => [...prev, ...batchedResources]);
            setLoadedCount((prev) => prev + batchSize);
          });
        }
        accumulatedResourcesRef.current = [];
      }
    };
    try {
      if (isLoadingMore) {
        setLoadingMore(true);
      } else {
        setLoading(true);
        setError(null);
        setResources([]);
        setTotalCount(0);
        setLoadedCount(0);
        setHasMore(false);
        setNextCursor("");
      }
      const startParam = options?.rawStart || startTimeMs;
      const endParam = options?.rawEnd || endTimeMs;
      if (!startParam || !endParam) {
        setLoading(false);
        return;
      }
      const BATCH_SIZE = 50;
      const BATCH_DELAY_MS = 150;
      const apiFilters = {
        ...options?.filters,
        pageSize: options?.pageSize || 100,
        cursor: cursor || ""
      };
      const result = await apiClient.getTimelineGrpc(
        startParam,
        endParam,
        apiFilters,
        (chunk) => {
          console.log("getTimeline Grpc: chunk.resources", chunk.resources);
          if (chunk.metadata?.totalCount !== void 0) {
            setTotalCount(chunk.metadata.totalCount);
          }
          if (chunk.metadata.hasMore !== void 0) {
            setHasMore(chunk.metadata.hasMore);
          }
          if (chunk.metadata.nextCursor) {
            setNextCursor(chunk.metadata.nextCursor);
          }
          accumulatedResourcesRef.current.push(...chunk.resources);
          if (accumulatedResourcesRef.current.length >= BATCH_SIZE || chunk.isComplete) {
            if (batchTimeoutRef.current) {
              clearTimeout(batchTimeoutRef.current);
              batchTimeoutRef.current = null;
            }
            flushBatch();
          } else {
            if (batchTimeoutRef.current) {
              clearTimeout(batchTimeoutRef.current);
            }
            batchTimeoutRef.current = setTimeout(() => {
              flushBatch();
              batchTimeoutRef.current = null;
            }, BATCH_DELAY_MS);
          }
        }
      );
      if (batchTimeoutRef.current) {
        clearTimeout(batchTimeoutRef.current);
        batchTimeoutRef.current = null;
      }
      flushBatch();
      if (isLoadingMore) {
        setLoadingMore(false);
      } else {
        setLoading(false);
      }
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") {
        return;
      }
      const errorMessage = err instanceof Error ? err.message : "Failed to fetch timeline data";
      setError(new Error(errorMessage));
      if (isLoadingMore) {
        setLoadingMore(false);
      } else {
        setLoading(false);
      }
    }
  }, [startTimeMs, endTimeMs, filtersKey, options?.rawStart, options?.rawEnd, options?.filters, options?.pageSize, refreshToken]);
  reactExports.useEffect(() => {
    fetchData();
    return () => {
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
      if (batchTimeoutRef.current) {
        clearTimeout(batchTimeoutRef.current);
        batchTimeoutRef.current = null;
      }
    };
  }, [fetchData]);
  const loadMore = reactExports.useCallback(() => {
    console.log("loading more", hasMore, loadingMore, nextCursor);
    if (!hasMore || loadingMore || !nextCursor) return;
    fetchData(false, nextCursor);
  }, [hasMore, loadingMore, nextCursor, fetchData]);
  return {
    resources,
    loading,
    loadingMore,
    error,
    refresh: () => fetchData(true),
    totalCount,
    loadedCount,
    hasMore,
    loadMore
  };
};

const useMetadata = (timeRange) => {
  const [namespaces, setNamespaces] = reactExports.useState([]);
  const [kinds, setKinds] = reactExports.useState([]);
  const [loading, setLoading] = reactExports.useState(true);
  const [error, setError] = reactExports.useState(null);
  reactExports.useEffect(() => {
    if (!timeRange) {
      setLoading(false);
      return;
    }
    const fetchMetadata = async () => {
      try {
        setLoading(true);
        setError(null);
        let metadata = await apiClient.getMetadata(
          timeRange.start.getTime(),
          timeRange.end.getTime()
        );
        setNamespaces(metadata.namespaces || []);
        setKinds(metadata.kinds || []);
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : "Failed to fetch metadata";
        setError(new Error(errorMessage));
        console.error("Metadata fetch error:", err);
      } finally {
        setLoading(false);
      }
    };
    fetchMetadata();
  }, [timeRange?.start.getTime(), timeRange?.end.getTime()]);
  return {
    namespaces,
    kinds,
    loading,
    error
  };
};

const STORAGE_KEY_KINDS = "spectre-filters-kinds";
const STORAGE_KEY_NAMESPACES = "spectre-filters-namespaces";
const STORAGE_KEY_DEFAULT_KINDS_HASH = "spectre-filters-default-kinds-hash";
const hashArray = (arr) => {
  return arr.slice().sort().join(",");
};
const loadFilters = (defaultKinds) => {
  if (typeof window === "undefined") {
    return { kinds: defaultKinds, namespaces: [] };
  }
  try {
    const kindsStr = window.localStorage.getItem(STORAGE_KEY_KINDS);
    const namespacesStr = window.localStorage.getItem(STORAGE_KEY_NAMESPACES);
    let kinds = defaultKinds;
    if (kindsStr) {
      const parsed = JSON.parse(kindsStr);
      if (Array.isArray(parsed) && parsed.length > 0) {
        kinds = parsed;
      }
    }
    const namespaces = namespacesStr ? JSON.parse(namespacesStr) : [];
    return {
      kinds,
      namespaces: Array.isArray(namespaces) ? namespaces : []
    };
  } catch {
    return { kinds: defaultKinds, namespaces: [] };
  }
};
const saveFilters = (filters) => {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY_KINDS, JSON.stringify(filters.kinds));
    window.localStorage.setItem(STORAGE_KEY_NAMESPACES, JSON.stringify(filters.namespaces));
  } catch (error) {
    console.error("Failed to save filters to localStorage:", error);
  }
};
const usePersistedFilters = (availableKinds, availableNamespaces, defaultKinds) => {
  const [filters, setFilters] = reactExports.useState(() => loadFilters(defaultKinds));
  const previousDefaultKindsHashRef = reactExports.useRef(null);
  const initializedRef = reactExports.useRef(false);
  reactExports.useEffect(() => {
    if (!initializedRef.current) {
      initializedRef.current = true;
      if (filters.kinds.length === 0 && defaultKinds.length > 0) {
        console.log("[usePersistedFilters] Empty kinds on init, applying defaults:", defaultKinds);
        setFilters((prev) => ({ ...prev, kinds: defaultKinds }));
      }
    }
  }, [defaultKinds, filters.kinds.length]);
  reactExports.useEffect(() => {
    const currentHash = hashArray(defaultKinds);
    const storedHash = window.localStorage.getItem(STORAGE_KEY_DEFAULT_KINDS_HASH);
    if (previousDefaultKindsHashRef.current === null) {
      previousDefaultKindsHashRef.current = storedHash || currentHash;
      if (!storedHash) {
        window.localStorage.setItem(STORAGE_KEY_DEFAULT_KINDS_HASH, currentHash);
      }
      return;
    }
    if (storedHash !== currentHash) {
      console.log("[usePersistedFilters] Default kinds changed, resetting filters");
      setFilters((prev) => ({ ...prev, kinds: defaultKinds }));
      window.localStorage.setItem(STORAGE_KEY_DEFAULT_KINDS_HASH, currentHash);
      previousDefaultKindsHashRef.current = currentHash;
    }
  }, [defaultKinds]);
  reactExports.useEffect(() => {
    if (availableKinds.length === 0 && availableNamespaces.length === 0) {
      return;
    }
    setFilters((prev) => {
      const validatedKinds = availableKinds.length > 0 ? prev.kinds.filter((kind) => availableKinds.includes(kind) || defaultKinds.includes(kind)) : prev.kinds;
      const validatedNamespaces = availableNamespaces.length > 0 ? prev.namespaces.filter((ns) => availableNamespaces.includes(ns)) : prev.namespaces;
      if (validatedKinds.length !== prev.kinds.length || validatedNamespaces.length !== prev.namespaces.length) {
        const validated = {
          kinds: validatedKinds,
          namespaces: validatedNamespaces
        };
        return validated;
      }
      return prev;
    });
  }, [availableKinds, availableNamespaces, defaultKinds]);
  reactExports.useEffect(() => {
    saveFilters(filters);
  }, [filters]);
  const setKinds = reactExports.useCallback((kinds) => {
    setFilters((prev) => ({ ...prev, kinds }));
  }, []);
  const setNamespaces = reactExports.useCallback((namespaces) => {
    setFilters((prev) => ({ ...prev, namespaces }));
  }, []);
  return {
    kinds: filters.kinds,
    namespaces: filters.namespaces,
    setKinds,
    setNamespaces
  };
};

const DEFAULT_OPTIONS = {
  maxAttempts: 3,
  initialDelay: 1e3,
  maxDelay: 1e4,
  backoffMultiplier: 2,
  shouldRetry: (error) => {
    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      return message.includes("network") || message.includes("timeout") || message.includes("failed to fetch") || message.includes("connection");
    }
    return false;
  },
  onRetry: () => {
  }
};
async function withRetry(fn, options = {}) {
  const opts = { ...DEFAULT_OPTIONS, ...options };
  let lastError;
  let delay = opts.initialDelay;
  for (let attempt = 1; attempt <= opts.maxAttempts; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
      if (attempt >= opts.maxAttempts) {
        break;
      }
      if (!opts.shouldRetry(error)) {
        break;
      }
      opts.onRetry(attempt, error);
      await sleep(delay);
      delay = Math.min(delay * opts.backoffMultiplier, opts.maxDelay);
    }
  }
  throw lastError;
}
function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
function createRetryable(fn, options = {}) {
  return (...args) => {
    return withRetry(() => fn(...args), options);
  };
}
function isRetryableError(error) {
  if (error instanceof Error) {
    const message = error.message.toLowerCase();
    return message.includes("network") || message.includes("timeout") || message.includes("failed to fetch") || message.includes("connection") || message.includes("abort") || // HTTP status codes that are typically retryable
    message.includes("502") || message.includes("503") || message.includes("504") || message.includes("429");
  }
  return false;
}

const CACHE_TTL_MS = 5 * 60 * 1e3;
function stripResourceUIDPrefix(uid) {
  const parts = uid.split("/");
  if (parts.length >= 3 && parts[0] === "" && parts[1] === "v1") {
    return parts[parts.length - 1];
  }
  return uid;
}
const cache = /* @__PURE__ */ new Map();
function getCacheKey(resourceUID, timestamp, lookbackMs, format) {
  const timestampSec = Math.floor(timestamp.getTime() / 1e3);
  const lookbackKey = lookbackMs ? `:${lookbackMs}` : "";
  const formatKey = format ? `:${format}` : ":diff";
  return `${resourceUID}:${timestampSec}${lookbackKey}${formatKey}`;
}
function isCacheValid(entry) {
  return Date.now() - entry.timestamp < CACHE_TTL_MS;
}
async function fetchRootCauseAnalysis(resourceUID, failureTimestamp, options) {
  const cacheKey = getCacheKey(resourceUID, failureTimestamp, options?.lookbackMs, options?.format);
  const cached = cache.get(cacheKey);
  if (cached && isCacheValid(cached)) {
    console.log("[RootCause] Cache hit:", cacheKey);
    return cached.data;
  }
  const cleanUID = stripResourceUIDPrefix(resourceUID);
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    failureTimestamp: failureTimestamp.getTime().toString()
  });
  if (options?.maxDepth !== void 0) {
    params.set("maxDepth", options.maxDepth.toString());
  }
  if (options?.minConfidence !== void 0) {
    params.set("minConfidence", options.minConfidence.toString());
  }
  if (options?.lookbackMs !== void 0) {
    params.set("lookbackMs", options.lookbackMs.toString());
  }
  params.set("format", options?.format || "diff");
  const url = `/v1/causal-graph?${params.toString()}`;
  console.log("[RootCause] Fetching:", url);
  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: "GET",
        headers: {
          "Content-Type": "application/json"
        }
      });
      console.log("[RootCause] Response status:", response.status, response.statusText);
      if (!response.ok) {
        if (response.status === 404) {
          throw new Error("No root cause analysis available for this resource");
        }
        const errorText = await response.text();
        console.error("[RootCause] Error response:", errorText);
        throw new Error(`Root cause analysis failed: ${response.status} ${errorText}`);
      }
      const data2 = await response.json();
      console.log("[RootCause] Received data:", data2);
      return data2;
    },
    {
      maxAttempts: 3,
      initialDelay: 1e3,
      onRetry: (attempt, error) => {
        console.log(`[RootCause] Retry attempt ${attempt} after error:`, error);
      }
    }
  );
  cache.set(cacheKey, {
    data,
    timestamp: Date.now()
  });
  return data;
}
function clearRootCauseCache() {
  cache.clear();
  console.log("[RootCause] Cache cleared");
}
function pruneRootCauseCache() {
  const now = Date.now();
  for (const [key, entry] of cache.entries()) {
    if (now - entry.timestamp >= CACHE_TTL_MS) {
      cache.delete(key);
    }
  }
}
setInterval(pruneRootCauseCache, 6e4);
async function fetchAnomalies(resourceUID, startUnixSeconds, endUnixSeconds) {
  const cleanUID = stripResourceUIDPrefix(resourceUID);
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    start: startUnixSeconds.toString(),
    end: endUnixSeconds.toString()
  });
  const url = `/v1/anomalies?${params.toString()}`;
  console.log("[Anomalies] Fetching:", url);
  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: "GET",
        headers: {
          "Content-Type": "application/json"
        }
      });
      console.log("[Anomalies] Response status:", response.status, response.statusText);
      if (!response.ok) {
        if (response.status === 404) {
          throw new Error("No anomalies available for this resource");
        }
        const errorText = await response.text();
        console.error("[Anomalies] Error response:", errorText);
        throw new Error(`Anomaly detection failed: ${response.status} ${errorText}`);
      }
      const data2 = await response.json();
      console.log("[Anomalies] Received data:", data2);
      return data2;
    },
    {
      maxAttempts: 3,
      initialDelay: 1e3,
      onRetry: (attempt, error) => {
        console.log(`[Anomalies] Retry attempt ${attempt} after error:`, error);
      }
    }
  );
  return data;
}
async function fetchCausalPaths(resourceUID, failureTimestamp, options) {
  const cleanUID = stripResourceUIDPrefix(resourceUID);
  const params = new URLSearchParams({
    resourceUID: cleanUID,
    failureTimestamp: failureTimestamp.toString()
  });
  if (options?.maxDepth !== void 0) {
    params.set("maxDepth", options.maxDepth.toString());
  }
  if (options?.maxPaths !== void 0) {
    params.set("maxPaths", options.maxPaths.toString());
  }
  if (options?.lookbackMs !== void 0) {
    params.set("lookbackMs", options.lookbackMs.toString());
  }
  const url = `/v1/causal-paths?${params.toString()}`;
  console.log("[CausalPaths] Fetching:", url);
  const data = await withRetry(
    async () => {
      const response = await fetch(url, {
        method: "GET",
        headers: {
          "Content-Type": "application/json"
        }
      });
      console.log("[CausalPaths] Response status:", response.status, response.statusText);
      if (!response.ok) {
        if (response.status === 404) {
          return {
            paths: [],
            metadata: {
              queryExecutionMs: 0,
              algorithmVersion: "unknown",
              executedAt: (/* @__PURE__ */ new Date()).toISOString(),
              nodesExplored: 0,
              pathsDiscovered: 0,
              pathsReturned: 0
            }
          };
        }
        const errorText = await response.text();
        console.error("[CausalPaths] Error response:", errorText);
        throw new Error(`Causal paths fetch failed: ${response.status} ${errorText}`);
      }
      const data2 = await response.json();
      console.log("[CausalPaths] Received data:", data2.paths.length, "paths");
      return data2;
    },
    {
      maxAttempts: 3,
      initialDelay: 1e3,
      onRetry: (attempt, error) => {
        console.log(`[CausalPaths] Retry attempt ${attempt} after error:`, error);
      }
    }
  );
  return data;
}

const AUTO_REFRESH_INTERVALS = {
  off: 0,
  "30s": 3e4,
  "60s": 6e4,
  "300s": 3e5
};
function TimelinePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const initialTimeRangeRef = reactExports.useRef(null);
  const [timeRange, setTimeRange] = reactExports.useState(null);
  const [rawTimeExpressions, setRawTimeExpressions] = reactExports.useState({});
  const originalUrlParamsRef = reactExports.useRef(null);
  const isUpdatingFromZoom = reactExports.useRef(false);
  reactExports.useEffect(() => {
    const startParam = searchParams.get("start");
    const endParam = searchParams.get("end");
    if (!startParam && !endParam) {
      console.log("[TimelinePage] Auto-setting time range to now-30m -> now");
      const endExpr = "now";
      const startExpr = "now-30m";
      setSearchParams({
        start: startExpr,
        end: endExpr
      }, { replace: true });
    }
  }, [searchParams, setSearchParams]);
  const displayTimeRange = reactExports.useMemo(() => {
    const startParam = searchParams.get("start");
    const endParam = searchParams.get("end");
    if (!startParam || !endParam) return timeRange;
    const start = new Date(startParam);
    const end = new Date(endParam);
    if (isNaN(start.getTime()) || isNaN(end.getTime())) return timeRange;
    return { start, end };
  }, [searchParams, timeRange]);
  reactExports.useEffect(() => {
    if (isUpdatingFromZoom.current) {
      isUpdatingFromZoom.current = false;
      return;
    }
    const startParam = searchParams.get("start");
    const endParam = searchParams.get("end");
    if (!startParam || !endParam) {
      setTimeRange(null);
      return;
    }
    originalUrlParamsRef.current = { start: startParam, end: endParam };
    let start = parseTimeExpression(startParam);
    let end = parseTimeExpression(endParam);
    if (!start) {
      const isoStart = new Date(startParam);
      if (!isNaN(isoStart.getTime())) {
        start = isoStart;
      }
    }
    if (!end) {
      const isoEnd = new Date(endParam);
      if (!isNaN(isoEnd.getTime())) {
        end = isoEnd;
      }
    }
    if (!start || !end || isNaN(start.getTime()) || isNaN(end.getTime()) || start >= end) {
      setTimeRange(null);
      setRawTimeExpressions({});
      originalUrlParamsRef.current = null;
      return;
    }
    const range = { start, end };
    initialTimeRangeRef.current = range;
    setTimeRange(range);
    const parsedStart = parseTimeExpression(startParam);
    const parsedEnd = parseTimeExpression(endParam);
    setRawTimeExpressions({
      start: parsedStart !== null ? startParam : void 0,
      end: parsedEnd !== null ? endParam : void 0
    });
  }, [searchParams]);
  const handleTimeRangeChange = (range, rawStart, rawEnd) => {
    const startParam = rawStart || range.start.toISOString();
    const endParam = rawEnd || range.end.toISOString();
    setSearchParams({
      start: startParam,
      end: endParam
    });
    initialTimeRangeRef.current = range;
    setTimeRange(range);
    setRawTimeExpressions({ start: rawStart, end: rawEnd });
  };
  const handleVisibleTimeRangeChange = (range) => {
    if (timeRange) {
      const currentDuration = timeRange.end.getTime() - timeRange.start.getTime();
      const expectedPadding = currentDuration * 0.02;
      const expectedDomain = {
        start: new Date(timeRange.start.getTime() - expectedPadding),
        end: new Date(timeRange.end.getTime() + expectedPadding)
      };
      const startDiff = Math.abs(range.start.getTime() - expectedDomain.start.getTime());
      const endDiff = Math.abs(range.end.getTime() - expectedDomain.end.getTime());
      const tolerance = 2e3;
      if (startDiff < tolerance && endDiff < tolerance) {
        return;
      }
    }
    const duration = range.end.getTime() - range.start.getTime();
    const paddingMs = duration * 0.02;
    const unpaddedRange = {
      start: new Date(range.start.getTime() + paddingMs),
      end: new Date(range.end.getTime() - paddingMs)
    };
    isUpdatingFromZoom.current = true;
    setSearchParams({
      start: unpaddedRange.start.toISOString(),
      end: unpaddedRange.end.toISOString()
    });
    setRawTimeExpressions({});
  };
  const { clearPreset } = usePersistedQuickPreset();
  const handleZoomDetected = reactExports.useCallback(() => {
    clearPreset();
  }, [clearPreset]);
  const { namespaces: availableNamespaces, kinds: availableKinds } = useMetadata(timeRange);
  const { defaultKinds } = useSettings();
  const { kinds: persistedKinds, namespaces: persistedNamespaces, setKinds, setNamespaces } = usePersistedFilters(availableKinds, availableNamespaces, defaultKinds);
  const [selectedPoint, setSelectedPoint] = reactExports.useState(null);
  const [showRootCause, setShowRootCause] = reactExports.useState(false);
  const [rootCauseAnalysis, setRootCauseAnalysis] = reactExports.useState(null);
  const [rootCauseLoading, setRootCauseLoading] = reactExports.useState(false);
  const [rootCauseError, setRootCauseError] = reactExports.useState(null);
  const [rootCauseParams, setRootCauseParams] = reactExports.useState(null);
  reactExports.useEffect(() => {
    console.log("[RootCause] State changed:", {
      showRootCause,
      hasAnalysis: !!rootCauseAnalysis,
      analysisChainLength: rootCauseAnalysis?.incident?.causalChain?.length
    });
  }, [showRootCause, rootCauseAnalysis]);
  const deepLinkLoadedRef = reactExports.useRef(false);
  reactExports.useEffect(() => {
    const rcResourceUID = searchParams.get("rcResourceUID");
    const rcTimestamp = searchParams.get("rcTimestamp");
    if (deepLinkLoadedRef.current || showRootCause || !rcResourceUID || !rcTimestamp) {
      return;
    }
    console.log("[RootCause] Deep link detected:", { rcResourceUID, rcTimestamp });
    deepLinkLoadedRef.current = true;
    const fetchFromDeepLink = async () => {
      setRootCauseLoading(true);
      try {
        const timestamp = new Date(parseInt(rcTimestamp));
        const defaultLookbackMs = 6e5;
        const analysis = await fetchRootCauseAnalysis(rcResourceUID, timestamp, { lookbackMs: defaultLookbackMs });
        setRootCauseAnalysis(analysis);
        setRootCauseParams({
          uid: rcResourceUID,
          timestamp,
          lookbackMs: defaultLookbackMs
        });
        setShowRootCause(true);
      } catch (err) {
        console.error("[RootCause] Failed to fetch from deep link:", err);
        deepLinkLoadedRef.current = false;
      } finally {
        setRootCauseLoading(false);
      }
    };
    fetchFromDeepLink();
  }, [searchParams, showRootCause]);
  const [search, setSearch] = reactExports.useState("");
  const [hasProblematicStatus, setHasProblematicStatus] = reactExports.useState(false);
  const filters = reactExports.useMemo(() => ({
    kinds: persistedKinds,
    namespaces: persistedNamespaces,
    search,
    hasProblematicStatus
  }), [persistedKinds, persistedNamespaces, search, hasProblematicStatus]);
  const setFilters = reactExports.useCallback((updater) => {
    if (typeof updater === "function") {
      const currentFilters = {
        kinds: persistedKinds,
        namespaces: persistedNamespaces,
        search,
        hasProblematicStatus
      };
      const newFilters = updater(currentFilters);
      setSearch(newFilters.search);
      setKinds(newFilters.kinds);
      setNamespaces(newFilters.namespaces);
      if (newFilters.hasProblematicStatus !== void 0) {
        setHasProblematicStatus(newFilters.hasProblematicStatus);
      }
    } else {
      setSearch(updater.search);
      setKinds(updater.kinds);
      setNamespaces(updater.namespaces);
      if (updater.hasProblematicStatus !== void 0) {
        setHasProblematicStatus(updater.hasProblematicStatus);
      }
    }
  }, [persistedKinds, persistedNamespaces, search, hasProblematicStatus, setKinds, setNamespaces]);
  const apiFilters = reactExports.useMemo(() => ({
    namespaces: filters.namespaces,
    kinds: filters.kinds
  }), [filters.namespaces, filters.kinds]);
  const [refreshToken, setRefreshToken] = reactExports.useState(0);
  const { autoRefresh } = useSettings();
  reactExports.useEffect(() => {
    if (!timeRange) return;
    const intervalMs = AUTO_REFRESH_INTERVALS[autoRefresh] || 0;
    if (intervalMs === 0) return;
    const id = window.setInterval(() => {
      setRefreshToken((prev) => prev + 1);
    }, intervalMs);
    return () => window.clearInterval(id);
  }, [autoRefresh, timeRange]);
  const { resources, loading, loadingMore, error, totalCount, loadedCount, hasMore, loadMore } = useTimeline({
    startTime: timeRange?.start,
    endTime: timeRange?.end,
    rawStart: rawTimeExpressions.start,
    rawEnd: rawTimeExpressions.end,
    filters: apiFilters,
    pageSize: 100,
    // Load 100 resources per page
    refreshToken
  });
  const [loadingAllForProblematic, setLoadingAllForProblematic] = reactExports.useState(false);
  reactExports.useEffect(() => {
    if (hasProblematicStatus && timeRange) {
      if (loading || loadingMore || hasMore) {
        setLoadingAllForProblematic(true);
      } else {
        setLoadingAllForProblematic(false);
      }
    } else if (!hasProblematicStatus) {
      setLoadingAllForProblematic(false);
    }
  }, [hasProblematicStatus, timeRange, loading, loadingMore, hasMore]);
  reactExports.useEffect(() => {
    if (hasProblematicStatus && hasMore && !loading && !loadingMore && timeRange) {
      loadMore();
    }
  }, [hasProblematicStatus, hasMore, loading, loadingMore, loadMore, timeRange]);
  const filteredResources = reactExports.useMemo(() => {
    return resources.filter((r) => {
      const matchesSearch = r.name.toLowerCase().includes(filters.search.toLowerCase());
      const matchesStatus = !filters.hasProblematicStatus || r.statusSegments.some((s) => s.status !== ResourceStatus.Ready);
      if (timeRange && r.deletedAt) {
        const deletedBeforeWindow = r.deletedAt < timeRange.start;
        if (deletedBeforeWindow) {
          return false;
        }
      }
      return matchesSearch && matchesStatus;
    });
  }, [resources, filters.search, filters.hasProblematicStatus, timeRange]);
  const selectedResource = reactExports.useMemo(() => {
    if (!selectedPoint) return null;
    return resources.find((r) => r.id === selectedPoint.resourceId) || null;
  }, [selectedPoint, resources]);
  const relevantEventIds = reactExports.useMemo(() => {
    if (!selectedResource || !selectedPoint) return [];
    const segment = selectedResource.statusSegments[selectedPoint.index];
    if (!segment) return [];
    return selectedResource.events.filter((e) => e.timestamp >= segment.start && e.timestamp <= segment.end).map((e) => e.id);
  }, [selectedResource, selectedPoint]);
  const handleSegmentClick = async (resource, index) => {
    setSelectedPoint({ resourceId: resource.id, index });
  };
  const handleClosePanel = () => {
    setSelectedPoint(null);
  };
  const handleAnalyzeRootCause = async () => {
    if (!selectedResource || selectedPoint === null) return;
    const segment = selectedResource.statusSegments[selectedPoint.index];
    if (!segment) return;
    console.log("[RootCause] Starting analysis for:", {
      resourceId: selectedResource.id,
      resourceName: selectedResource.name,
      segmentStart: segment.start,
      segmentEnd: segment.end
    });
    setRootCauseLoading(true);
    deepLinkLoadedRef.current = true;
    const defaultLookbackMs = 6e5;
    try {
      console.log("[RootCause] Calling fetchRootCauseAnalysis...");
      const analysis = await fetchRootCauseAnalysis(selectedResource.id, segment.start, { lookbackMs: defaultLookbackMs });
      console.log("[RootCause] Analysis received:", analysis);
      setRootCauseAnalysis(analysis);
      setRootCauseParams({
        uid: selectedResource.id,
        timestamp: segment.start,
        lookbackMs: defaultLookbackMs
      });
      setShowRootCause(true);
      setSelectedPoint(null);
      console.log("[RootCause] State updated, adding URL params...");
      const currentParams = Object.fromEntries(searchParams.entries());
      setSearchParams({
        ...currentParams,
        rcResourceUID: selectedResource.id,
        rcTimestamp: segment.start.getTime().toString()
      }, { replace: true });
      console.log("[RootCause] URL params added");
    } catch (err) {
      console.error("[RootCause] Failed to fetch:", err);
      deepLinkLoadedRef.current = false;
      toast.apiError(err, "Failed to load root cause analysis");
    } finally {
      setRootCauseLoading(false);
      console.log("[RootCause] Loading state cleared");
    }
  };
  const handleRootCauseRefresh = async (uid, timestamp, lookbackMs) => {
    setRootCauseLoading(true);
    setRootCauseError(null);
    try {
      const analysis = await fetchRootCauseAnalysis(uid, timestamp, { lookbackMs });
      setRootCauseAnalysis(analysis);
      setRootCauseParams({ uid, timestamp, lookbackMs });
    } catch (err) {
      console.error("[RootCause] Failed to refresh:", err);
      const errorMessage = err instanceof Error ? err.message : "Failed to fetch root cause analysis";
      setRootCauseError(errorMessage);
      toast.apiError(err, "Failed to refresh root cause analysis");
    } finally {
      setRootCauseLoading(false);
    }
  };
  const handleCloseRootCause = () => {
    setShowRootCause(false);
    setRootCauseAnalysis(null);
    setRootCauseParams(null);
    setRootCauseError(null);
    deepLinkLoadedRef.current = false;
    const currentParams = Object.fromEntries(searchParams.entries());
    const { rcResourceUID, rcTimestamp, ...remainingParams } = currentParams;
    setSearchParams(remainingParams);
  };
  reactExports.useEffect(() => {
    const handleKeyDown = (e) => {
      if (!selectedPoint || !selectedResource) return;
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        if (selectedPoint.index > 0) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index - 1 });
        }
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        if (selectedPoint.index < selectedResource.statusSegments.length - 1) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index + 1 });
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedPoint, selectedResource]);
  if (!timeRange) {
    return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col h-screen bg-[var(--color-app-bg)] text-[var(--color-text-primary)] overflow-hidden font-sans transition-colors duration-300 items-center justify-center", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin mb-4", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-12 h-12 text-brand-500", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1.5", d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" }) }) }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[var(--color-text-muted)]", children: "Loading timeline..." })
    ] });
  }
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col h-screen bg-[var(--color-app-bg)] text-[var(--color-text-primary)] overflow-hidden font-sans transition-colors duration-300", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      FilterBar,
      {
        filters,
        setFilters,
        timeRange: displayTimeRange,
        onTimeRangeChange: handleTimeRangeChange,
        availableNamespaces,
        availableKinds,
        rawStart: rawTimeExpressions.start,
        rawEnd: rawTimeExpressions.end
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex overflow-hidden relative", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 overflow-hidden flex flex-col relative", children: [
        loadingAllForProblematic && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 bg-[var(--color-surface-elevated)]/80 backdrop-blur-sm flex items-center justify-center z-50", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-4 text-[var(--color-text-primary)]", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-12 h-12 text-brand-500", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1.5", d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-lg font-medium", children: "Loading all resources..." }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)]", children: "Fetching remaining resources for problematic filter" }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex flex-col items-center gap-2 mt-2", children: hasMore ? /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-64 h-2 bg-gray-700 rounded-full overflow-hidden", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
                "div",
                {
                  className: "h-full bg-brand-500 transition-all duration-300 animate-pulse",
                  style: { width: "90%" }
                }
              ) }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("p", { className: "text-sm text-[var(--color-text-muted)]", children: [
                loadedCount,
                " resources loaded"
              ] })
            ] }) : /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-64 h-2 bg-gray-700 rounded-full overflow-hidden", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
                "div",
                {
                  className: "h-full bg-brand-500 transition-all duration-300",
                  style: { width: "100%" }
                }
              ) }),
              /* @__PURE__ */ jsxRuntimeExports.jsxs("p", { className: "text-sm text-[var(--color-text-muted)]", children: [
                loadedCount,
                " resources loaded"
              ] })
            ] }) })
          ] })
        ] }) }),
        error ? /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex items-center justify-center text-red-400 flex-col gap-4", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-16 h-16 opacity-50", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1", d: "M12 9v2m0 4v2m0 0v2m0-6V9m0 0V7m0 2V5m0 0h-2m2 0h2m-6 0h2m-2 0h-2m0 6h2m-2 0h-2m0 0v2m0-2v-2m0 0h2m-2 0h-2m6 0h2m-2 0h-2" }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-lg font-semibold mb-2", children: "Failed to load resources" }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-gray-400 mb-4", children: error.message }),
            /* @__PURE__ */ jsxRuntimeExports.jsx(
              "button",
              {
                onClick: () => window.location.reload(),
                className: "px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm transition-colors",
                children: "Retry"
              }
            )
          ] })
        ] }) : loading ? /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex items-center justify-center text-gray-400 flex-col gap-4", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-12 h-12", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1.5", d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("p", { children: "Loading resources..." }),
          totalCount > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-64 h-2 bg-gray-700 rounded-full overflow-hidden", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
              "div",
              {
                className: "h-full bg-blue-500 transition-all duration-300",
                style: { width: `${loadedCount / totalCount * 100}%` }
              }
            ) }),
            /* @__PURE__ */ jsxRuntimeExports.jsxs("p", { className: "text-sm text-gray-500", children: [
              loadedCount,
              " / ",
              totalCount,
              " resources"
            ] })
          ] })
        ] }) : filteredResources.length > 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx(jsxRuntimeExports.Fragment, { children: /* @__PURE__ */ jsxRuntimeExports.jsx(
          Timeline,
          {
            resources: filteredResources,
            onSegmentClick: handleSegmentClick,
            selectedPoint,
            highlightedEventIds: relevantEventIds,
            sidebarWidth: selectedPoint ? 384 : 0,
            timeRange,
            onVisibleTimeRangeChange: handleVisibleTimeRangeChange,
            onZoomDetected: handleZoomDetected,
            hasMore,
            loadingMore,
            onLoadMore: loadMore
          }
        ) }) : /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex items-center justify-center text-gray-500 flex-col", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-16 h-16 mb-4 opacity-50", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "1", d: "M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z" }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("p", { children: "No resources match your filters." })
        ] })
      ] }),
      !showRootCause && /* @__PURE__ */ jsxRuntimeExports.jsx(
        DetailPanel,
        {
          resource: selectedResource,
          selectedIndex: selectedPoint?.index,
          onClose: handleClosePanel,
          onAnalyzeRootCause: handleAnalyzeRootCause
        }
      )
    ] }),
    showRootCause && rootCauseAnalysis && rootCauseParams ? /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
      console.log("[TimelinePage] Rendering RootCauseView - condition met"),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        RootCauseView,
        {
          analysis: rootCauseAnalysis,
          initialResourceUID: rootCauseParams.uid,
          initialTimestamp: rootCauseParams.timestamp,
          initialLookbackMs: rootCauseParams.lookbackMs,
          onClose: handleCloseRootCause,
          onRefresh: handleRootCauseRefresh,
          error: rootCauseError
        }
      )
    ] }) : showRootCause && console.warn("[TimelinePage] RootCauseView NOT rendering - missing analysis or params", { showRootCause, hasAnalysis: !!rootCauseAnalysis, hasParams: !!rootCauseParams })
  ] });
}

const NODE_STATUS_COLORS = {
  Ready: "#10b981",
  // emerald-500
  Warning: "#f59e0b",
  // amber-500
  Error: "#ef4444",
  // red-500
  Terminating: "#6b7280",
  // gray-500
  Unknown: "#374151"
  // gray-700
};
const RELATIONSHIP_LABELS = {
  OWNS: "Owns",
  SELECTS: "Selects",
  REFERENCES: "References",
  MANAGES: "Manages",
  SCHEDULED_ON: "Scheduled On",
  MEMBER_OF: "Member Of"
};
function toD3Node(node, anomalyCount = 0) {
  return {
    ...node,
    anomalyCount,
    isClusterScoped: node.namespace === ""
  };
}
function toD3Link(edge) {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    relationshipType: edge.relationshipType
  };
}
function transformToD3Graph(response) {
  const anomalyCounts = /* @__PURE__ */ new Map();
  if (response.anomalies) {
    for (const anomaly of response.anomalies) {
      const uid = anomaly.node.uid;
      anomalyCounts.set(uid, (anomalyCounts.get(uid) || 0) + 1);
    }
  }
  const nodes = response.graph.nodes.map(
    (node) => toD3Node(node, anomalyCounts.get(node.uid) || 0)
  );
  const links = response.graph.edges.map(toD3Link);
  return { nodes, links };
}
function mergeNamespaceGraphResponses(existing, incoming) {
  const existingNodeUids = new Set(existing.graph.nodes.map((n) => n.uid));
  const existingEdgeIds = new Set(existing.graph.edges.map((e) => e.id));
  const newNodes = incoming.graph.nodes.filter((n) => !existingNodeUids.has(n.uid));
  const newEdges = incoming.graph.edges.filter((e) => !existingEdgeIds.has(e.id));
  const existingAnomalyKeys = new Set(
    (existing.anomalies || []).map((a) => `${a.node.uid}:${a.category}:${a.type}`)
  );
  const newAnomalies = (incoming.anomalies || []).filter(
    (a) => !existingAnomalyKeys.has(`${a.node.uid}:${a.category}:${a.type}`)
  );
  const existingPathIds = new Set((existing.causalPaths || []).map((p) => p.id));
  const newPaths = (incoming.causalPaths || []).filter((p) => !existingPathIds.has(p.id));
  return {
    graph: {
      nodes: [...existing.graph.nodes, ...newNodes],
      edges: [...existing.graph.edges, ...newEdges]
    },
    anomalies: [...existing.anomalies || [], ...newAnomalies],
    causalPaths: [...existing.causalPaths || [], ...newPaths],
    metadata: {
      ...incoming.metadata,
      nodeCount: existing.graph.nodes.length + newNodes.length,
      edgeCount: existing.graph.edges.length + newEdges.length,
      // Keep query time as cumulative
      queryExecutionMs: existing.metadata.queryExecutionMs + incoming.metadata.queryExecutionMs
    }
  };
}

const DEFAULT_PAGE_SIZE = 50;
const DEFAULT_AUTO_LOAD_DELAY = 100;
function useNamespaceGraph(options) {
  const {
    namespace,
    timestamp,
    includeAnomalies = true,
    includeCausalPaths = false,
    lookback,
    maxDepth,
    enabled = true,
    pageSize = DEFAULT_PAGE_SIZE,
    autoLoad = true,
    autoLoadDelay = DEFAULT_AUTO_LOAD_DELAY
  } = options;
  const [data, setData] = reactExports.useState(null);
  const [isLoading, setIsLoading] = reactExports.useState(false);
  const [isLoadingMore, setIsLoadingMore] = reactExports.useState(false);
  const [error, setError] = reactExports.useState(null);
  const [pagesLoaded, setPagesLoaded] = reactExports.useState(0);
  const [hasMore, setHasMore] = reactExports.useState(false);
  const [nextCursor, setNextCursor] = reactExports.useState(void 0);
  const fetchSessionRef = reactExports.useRef(0);
  const mountedRef = reactExports.useRef(true);
  const autoLoadTimerRef = reactExports.useRef(null);
  const isAutoLoadingRef = reactExports.useRef(false);
  const timestampNanos = timestamp.getTime() * 1e6;
  const optionsRef = reactExports.useRef({
    namespace,
    timestampNanos,
    includeAnomalies,
    includeCausalPaths,
    lookback,
    maxDepth,
    pageSize,
    autoLoadDelay
  });
  optionsRef.current = {
    namespace,
    timestampNanos,
    includeAnomalies,
    includeCausalPaths,
    lookback,
    maxDepth,
    pageSize,
    autoLoadDelay
  };
  const fetchPage = reactExports.useCallback(
    async (cursor, sessionId, isFirstPage) => {
      const opts = optionsRef.current;
      const response = await apiClient.getNamespaceGraph({
        namespace: opts.namespace,
        timestamp: opts.timestampNanos,
        includeAnomalies: opts.includeAnomalies,
        includeCausalPaths: opts.includeCausalPaths,
        lookback: opts.lookback,
        maxDepth: opts.maxDepth,
        limit: opts.pageSize,
        cursor
      });
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        return false;
      }
      if (isFirstPage) {
        setData(response);
      } else {
        setData((prev) => prev ? mergeNamespaceGraphResponses(prev, response) : response);
      }
      setHasMore(response.metadata.hasMore);
      setNextCursor(response.metadata.nextCursor);
      setPagesLoaded((prev) => prev + 1);
      return response.metadata.hasMore;
    },
    []
    // No dependencies - uses refs
  );
  const clearAutoLoadTimer = reactExports.useCallback(() => {
    if (autoLoadTimerRef.current) {
      clearTimeout(autoLoadTimerRef.current);
      autoLoadTimerRef.current = null;
    }
  }, []);
  const reset = reactExports.useCallback(() => {
    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(void 0);
    setPagesLoaded(0);
    setIsLoading(false);
    setIsLoadingMore(false);
  }, [clearAutoLoadTimer]);
  reactExports.useEffect(() => {
    mountedRef.current = true;
    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;
    if (!namespace || !enabled) {
      reset();
      return;
    }
    const sessionId = ++fetchSessionRef.current;
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(void 0);
    setPagesLoaded(0);
    setIsLoading(true);
    setIsLoadingMore(false);
    const runInitialFetch = async () => {
      try {
        await fetchPage(void 0, sessionId, true);
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setIsLoading(false);
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setIsLoading(false);
        }
      }
    };
    runInitialFetch();
    return () => {
      mountedRef.current = false;
      clearAutoLoadTimer();
    };
  }, [namespace, timestampNanos, lookback, enabled]);
  reactExports.useEffect(() => {
    if (!autoLoad || !hasMore || !nextCursor || isLoading) {
      return;
    }
    if (isAutoLoadingRef.current) {
      return;
    }
    const sessionId = fetchSessionRef.current;
    const cursor = nextCursor;
    const delay = optionsRef.current.autoLoadDelay;
    isAutoLoadingRef.current = true;
    clearAutoLoadTimer();
    autoLoadTimerRef.current = setTimeout(async () => {
      if (!mountedRef.current || sessionId !== fetchSessionRef.current) {
        isAutoLoadingRef.current = false;
        return;
      }
      setIsLoadingMore(true);
      try {
        const moreAvailable = await fetchPage(cursor, sessionId, false);
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          isAutoLoadingRef.current = false;
          if (!moreAvailable) {
            setIsLoadingMore(false);
          }
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          console.error("[useNamespaceGraph] Auto-load error:", err);
          isAutoLoadingRef.current = false;
          setIsLoadingMore(false);
        }
      }
    }, delay);
    return () => {
      clearAutoLoadTimer();
    };
  }, [autoLoad, hasMore, nextCursor, isLoading, fetchPage, clearAutoLoadTimer]);
  const loadMore = reactExports.useCallback(() => {
    if (!hasMore || isLoadingMore || isLoading || !nextCursor) return;
    const sessionId = fetchSessionRef.current;
    setIsLoadingMore(true);
    fetchPage(nextCursor, sessionId, false).then((moreAvailable) => {
      if (mountedRef.current && sessionId === fetchSessionRef.current && !moreAvailable) {
        setIsLoadingMore(false);
      }
    }).catch((err) => {
      if (mountedRef.current && sessionId === fetchSessionRef.current) {
        setError(err instanceof Error ? err : new Error(String(err)));
        setIsLoadingMore(false);
      }
    });
  }, [hasMore, isLoadingMore, isLoading, nextCursor, fetchPage]);
  const refetch = reactExports.useCallback(() => {
    if (!namespace || !enabled) return;
    clearAutoLoadTimer();
    isAutoLoadingRef.current = false;
    const sessionId = ++fetchSessionRef.current;
    setData(null);
    setError(null);
    setHasMore(false);
    setNextCursor(void 0);
    setPagesLoaded(0);
    setIsLoading(true);
    setIsLoadingMore(false);
    const runFetch = async () => {
      try {
        await fetchPage(void 0, sessionId, true);
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setIsLoading(false);
        }
      } catch (err) {
        if (mountedRef.current && sessionId === fetchSessionRef.current) {
          setError(err instanceof Error ? err : new Error(String(err)));
          setIsLoading(false);
        }
      }
    };
    runFetch();
  }, [namespace, enabled, fetchPage, clearAutoLoadTimer]);
  return {
    data,
    isLoading,
    isLoadingMore,
    hasMore,
    pagesLoaded,
    error,
    loadMore,
    refetch
  };
}

const STORAGE_KEY$2 = "spectre-graph-namespace";
const usePersistedGraphNamespace = (availableNamespaces) => {
  const [namespace, setNamespaceState] = reactExports.useState(() => {
    if (typeof window === "undefined") return null;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY$2);
      return stored || null;
    } catch {
      return null;
    }
  });
  reactExports.useEffect(() => {
    if (availableNamespaces.length === 0) return;
    if (namespace && !availableNamespaces.includes(namespace)) {
      console.log("[usePersistedGraphNamespace] Stored namespace not available, clearing");
      setNamespaceState(null);
      try {
        window.localStorage.removeItem(STORAGE_KEY$2);
      } catch {
      }
    }
  }, [namespace, availableNamespaces]);
  const setNamespace = reactExports.useCallback((ns) => {
    setNamespaceState(ns);
    if (typeof window === "undefined") return;
    try {
      if (ns) {
        window.localStorage.setItem(STORAGE_KEY$2, ns);
      } else {
        window.localStorage.removeItem(STORAGE_KEY$2);
      }
    } catch (error) {
      console.error("Failed to save namespace to localStorage:", error);
    }
  }, []);
  return { namespace, setNamespace };
};

const STORAGE_KEY$1 = "spectre-graph-timestamp";
const DEFAULT_EXPRESSION = "now";
const usePersistedGraphTimestamp = () => {
  const [expression, setExpressionState] = reactExports.useState(() => {
    if (typeof window === "undefined") return DEFAULT_EXPRESSION;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY$1);
      return stored || DEFAULT_EXPRESSION;
    } catch {
      return DEFAULT_EXPRESSION;
    }
  });
  const setExpression = reactExports.useCallback((expr) => {
    setExpressionState(expr);
    if (typeof window === "undefined") return;
    try {
      if (expr && expr !== DEFAULT_EXPRESSION) {
        window.localStorage.setItem(STORAGE_KEY$1, expr);
      } else {
        window.localStorage.removeItem(STORAGE_KEY$1);
      }
    } catch (error) {
      console.error("Failed to save timestamp to localStorage:", error);
    }
  }, []);
  return { expression, setExpression };
};

const STORAGE_KEY = "spectre-graph-lookback";
const DEFAULT_LOOKBACK = "10m";
const LOOKBACK_OPTIONS = [
  { value: "5m", label: "5 minutes" },
  { value: "10m", label: "10 minutes" },
  { value: "30m", label: "30 minutes" },
  { value: "1h", label: "1 hour" },
  { value: "2h", label: "2 hours" },
  { value: "6h", label: "6 hours" },
  { value: "12h", label: "12 hours" },
  { value: "24h", label: "24 hours" }
];
const usePersistedGraphLookback = () => {
  const [lookback, setLookbackState] = reactExports.useState(() => {
    if (typeof window === "undefined") return DEFAULT_LOOKBACK;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      return stored || DEFAULT_LOOKBACK;
    } catch {
      return DEFAULT_LOOKBACK;
    }
  });
  const setLookback = reactExports.useCallback((value) => {
    setLookbackState(value);
    if (typeof window === "undefined") return;
    try {
      if (value && value !== DEFAULT_LOOKBACK) {
        window.localStorage.setItem(STORAGE_KEY, value);
      } else {
        window.localStorage.removeItem(STORAGE_KEY);
      }
    } catch (error) {
      console.error("Failed to save lookback to localStorage:", error);
    }
  }, []);
  return { lookback, setLookback };
};

const NODE_RADIUS = 24;
const COLLISION_RADIUS = 60;
const ZOOM_SCALE_FACTOR = 1.3;
const NamespaceGraph = reactExports.forwardRef(
  ({ data, onNodeClick, selectedNodeId, highlightedNodeUids, width: propWidth, height: propHeight }, ref) => {
    const containerRef = reactExports.useRef(null);
    const svgRef = reactExports.useRef(null);
    const simulationRef = reactExports.useRef(null);
    const zoomRef = reactExports.useRef(null);
    const isInitializedRef = reactExports.useRef(false);
    const prevNodeUidsRef = reactExports.useRef(/* @__PURE__ */ new Set());
    const selectedNodeIdRef = reactExports.useRef(selectedNodeId);
    selectedNodeIdRef.current = selectedNodeId;
    const onNodeClickRef = reactExports.useRef(onNodeClick);
    onNodeClickRef.current = onNodeClick;
    const { nodes, links } = reactExports.useMemo(() => transformToD3Graph(data), [data]);
    const [containerSize, setContainerSize] = React.useState({ width: 800, height: 600 });
    const lastSizeRef = reactExports.useRef({ width: 0, height: 0 });
    const sizeInitializedRef = reactExports.useRef(false);
    reactExports.useEffect(() => {
      if (!containerRef.current) return;
      const resizeObserver = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const { width: width2, height: height2 } = entry.contentRect;
          if (width2 <= 0 || height2 <= 0) return;
          if (!sizeInitializedRef.current) {
            sizeInitializedRef.current = true;
            lastSizeRef.current = { width: width2, height: height2 };
            setContainerSize({ width: width2, height: height2 });
            return;
          }
          const heightDiff = Math.abs(height2 - lastSizeRef.current.height);
          if (heightDiff < 1) return;
          lastSizeRef.current = { width: lastSizeRef.current.width, height: height2 };
          setContainerSize((prev) => ({ ...prev, height: height2 }));
        }
      });
      resizeObserver.observe(containerRef.current);
      return () => resizeObserver.disconnect();
    }, []);
    const width = propWidth ?? containerSize.width;
    const height = propHeight ?? containerSize.height;
    const getNodeColor = reactExports.useCallback((node) => {
      return NODE_STATUS_COLORS[node.status] || NODE_STATUS_COLORS.Unknown;
    }, []);
    const truncateName = reactExports.useCallback((name, maxLen = 20) => {
      if (name.length <= maxLen) return name;
      return name.slice(0, maxLen - 3) + "...";
    }, []);
    const createDragBehavior = reactExports.useCallback(() => {
      const simulation = simulationRef.current;
      if (!simulation) return null;
      return drag().on("start", (event, d) => {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
      }).on("drag", (event, d) => {
        d.fx = event.x;
        d.fy = event.y;
      }).on("end", (event, d) => {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
      });
    }, []);
    const renderNodeGroup = reactExports.useCallback(
      (nodeEnter) => {
        const g = nodeEnter.append("g").attr("class", "node").attr("cursor", "pointer").on("click", (event, d) => {
          event.stopPropagation();
          onNodeClickRef.current?.(d);
        });
        g.append("circle").attr("r", NODE_RADIUS).attr("fill", (d) => getNodeColor(d)).attr("stroke", (d) => d.isClusterScoped ? "#9ca3af" : "none").attr("stroke-width", (d) => d.isClusterScoped ? 2 : 0).attr("stroke-dasharray", (d) => d.isClusterScoped ? "4,2" : "none").attr("filter", (d) => d.status === "Error" ? "url(#glow-error)" : "none");
        g.append("circle").attr("r", NODE_RADIUS + 4).attr("fill", "none").attr("stroke", "#3b82f6").attr("stroke-width", 2).attr("opacity", (d) => d.uid === selectedNodeIdRef.current ? 1 : 0).attr("class", "selection-ring");
        g.append("text").attr("y", -NODE_RADIUS - 8).attr("text-anchor", "middle").attr("fill", "#9ca3af").attr("font-size", "10px").attr("font-weight", "bold").text((d) => d.kind);
        g.append("text").attr("y", NODE_RADIUS + 16).attr("text-anchor", "middle").attr("fill", "#f8fafc").attr("font-size", "11px").text((d) => truncateName(d.name));
        const badgeGroup = g.filter((d) => d.anomalyCount > 0).append("g").attr("class", "anomaly-badge").attr("transform", `translate(${NODE_RADIUS - 6}, ${-NODE_RADIUS + 6})`);
        badgeGroup.append("circle").attr("r", 10).attr("fill", "#ef4444").attr("stroke", "#1f2937").attr("stroke-width", 2);
        badgeGroup.append("text").attr("text-anchor", "middle").attr("dominant-baseline", "central").attr("fill", "white").attr("font-size", "9px").attr("font-weight", "bold").text((d) => d.anomalyCount > 9 ? "9+" : d.anomalyCount.toString());
        return g;
      },
      [getNodeColor, truncateName]
    );
    reactExports.useImperativeHandle(
      ref,
      () => ({
        zoomIn: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = select(svgRef.current);
          svg.transition().duration(300).call(zoomRef.current.scaleBy, ZOOM_SCALE_FACTOR);
        },
        zoomOut: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = select(svgRef.current);
          svg.transition().duration(300).call(zoomRef.current.scaleBy, 1 / ZOOM_SCALE_FACTOR);
        },
        fitToView: () => {
          if (!svgRef.current || !zoomRef.current || !simulationRef.current) return;
          const svg = select(svgRef.current);
          const simNodes = simulationRef.current.nodes();
          if (simNodes.length === 0) return;
          let minX = Infinity, maxX = -Infinity;
          let minY = Infinity, maxY = -Infinity;
          simNodes.forEach((node) => {
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });
          const padding = NODE_RADIUS * 3;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;
          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;
          const scale = Math.min(
            width / graphWidth,
            height / graphHeight,
            1.5
            // Max zoom for fit
          ) * 0.9;
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;
          const transform = identity.translate(translateX, translateY).scale(scale);
          svg.transition().duration(500).call(zoomRef.current.transform, transform);
        },
        resetZoom: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = select(svgRef.current);
          const initialScale = 0.8;
          const initialTransform = identity.translate(width * (1 - initialScale) / 2, height * (1 - initialScale) / 2).scale(initialScale);
          svg.transition().duration(500).call(zoomRef.current.transform, initialTransform);
        },
        focusOnNodes: (nodeUids) => {
          if (!svgRef.current || !zoomRef.current || !simulationRef.current) return;
          const svg = select(svgRef.current);
          const simNodes = simulationRef.current.nodes();
          const targetNodes = simNodes.filter((n) => nodeUids.has(n.uid));
          if (targetNodes.length === 0) return;
          let minX = Infinity, maxX = -Infinity;
          let minY = Infinity, maxY = -Infinity;
          targetNodes.forEach((node) => {
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });
          const padding = NODE_RADIUS * 4;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;
          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;
          const scale = Math.min(
            width / graphWidth,
            height / graphHeight,
            1.5
            // Max zoom for fit
          ) * 0.85;
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;
          const transform = identity.translate(translateX, translateY).scale(scale);
          svg.transition().duration(500).call(zoomRef.current.transform, transform);
        }
      }),
      [width, height]
    );
    reactExports.useEffect(() => {
      if (!svgRef.current || width === 0 || height === 0) return;
      const svg = select(svgRef.current);
      const currentNodeUids = new Set(nodes.map((n) => n.uid));
      const prevNodeUids = prevNodeUidsRef.current;
      const isReset = !isInitializedRef.current || prevNodeUids.size === 0;
      const hasNewNodes = !isReset && nodes.some((n) => !prevNodeUids.has(n.uid));
      prevNodeUidsRef.current = currentNodeUids;
      if (nodes.length === 0) {
        svg.selectAll(".graph-container").remove();
        isInitializedRef.current = false;
        simulationRef.current = null;
        return;
      }
      if (isReset) {
        svg.selectAll("*").remove();
        const defs = svg.append("defs");
        const glowFilter = defs.append("filter").attr("id", "glow-error").attr("x", "-50%").attr("y", "-50%").attr("width", "200%").attr("height", "200%");
        glowFilter.append("feGaussianBlur").attr("stdDeviation", "4").attr("result", "blur");
        glowFilter.append("feFlood").attr("flood-color", "#ef4444").attr("flood-opacity", "0.6").attr("result", "color");
        glowFilter.append("feComposite").attr("in", "color").attr("in2", "blur").attr("operator", "in").attr("result", "coloredBlur");
        const glowMerge = glowFilter.append("feMerge");
        glowMerge.append("feMergeNode").attr("in", "coloredBlur");
        glowMerge.append("feMergeNode").attr("in", "SourceGraphic");
        defs.append("marker").attr("id", "arrow").attr("viewBox", "0 -5 10 10").attr("refX", NODE_RADIUS + 8).attr("refY", 0).attr("markerWidth", 6).attr("markerHeight", 6).attr("orient", "auto").append("path").attr("d", "M0,-5L10,0L0,5").attr("fill", "#6b7280");
        const container = svg.append("g").attr("class", "graph-container");
        const zoom$1 = zoom().scaleExtent([0.1, 4]).on("zoom", (event) => {
          container.attr("transform", event.transform);
        });
        zoomRef.current = zoom$1;
        svg.call(zoom$1);
        const simulationNodes = nodes.map((n) => ({ ...n }));
        const simulationLinks = links.map((l) => ({
          ...l,
          source: l.source,
          target: l.target
        }));
        const simulation$1 = simulation(simulationNodes).force("charge", manyBody().strength(-800)).force("center", center(width / 2, height / 2)).force("collision", collide().radius(COLLISION_RADIUS)).force(
          "link",
          link$2(simulationLinks).id((d) => d.uid).distance(150).strength(0.3)
        );
        simulation$1.stop();
        for (let i = 0; i < 300; i++) {
          simulation$1.tick();
        }
        simulationRef.current = simulation$1;
        const linkGroup = container.append("g").attr("class", "links");
        const link = linkGroup.selectAll("line").data(simulationLinks, (d) => d.id).join("line").attr("stroke", "#6b7280").attr("stroke-width", 1.5).attr("stroke-opacity", 0.6).attr("marker-end", "url(#arrow)");
        const nodeGroup = container.append("g").attr("class", "nodes");
        const nodeSelection = nodeGroup.selectAll("g").data(simulationNodes, (d) => d.uid).join((enter) => renderNodeGroup(enter));
        const drag = createDragBehavior();
        if (drag) {
          nodeSelection.call(drag);
        }
        simulation$1.on("tick", () => {
          link.attr("x1", (d) => d.source.x ?? 0).attr("y1", (d) => d.source.y ?? 0).attr("x2", (d) => d.target.x ?? 0).attr("y2", (d) => d.target.y ?? 0);
          nodeSelection.attr("transform", (d) => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });
        link.attr("x1", (d) => d.source.x ?? 0).attr("y1", (d) => d.source.y ?? 0).attr("x2", (d) => d.target.x ?? 0).attr("y2", (d) => d.target.y ?? 0);
        nodeSelection.attr("transform", (d) => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        if (simulationNodes.length > 0) {
          let minX = Infinity, maxX = -Infinity;
          let minY = Infinity, maxY = -Infinity;
          simulationNodes.forEach((n) => {
            const x = n.x ?? 0;
            const y = n.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });
          const padding = NODE_RADIUS * 3;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;
          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;
          const scale = Math.min(
            width / graphWidth,
            height / graphHeight,
            1.5
            // Max zoom for fit
          ) * 0.9;
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;
          const initialTransform = identity.translate(translateX, translateY).scale(scale);
          svg.call(zoom$1.transform, initialTransform);
        }
        isInitializedRef.current = true;
        return () => {
          simulation$1.stop();
        };
      } else if (hasNewNodes) {
        const simulation = simulationRef.current;
        if (!simulation) return;
        const container = svg.select(".graph-container");
        const linkGroup = container.select(".links");
        const nodeGroup = container.select(".nodes");
        const existingSimNodes = simulation.nodes();
        const existingNodeMap = /* @__PURE__ */ new Map();
        for (const node of existingSimNodes) {
          existingNodeMap.set(node.uid, node);
        }
        const newSimulationNodes = nodes.map((n) => {
          const existing = existingNodeMap.get(n.uid);
          if (existing) {
            return {
              ...n,
              x: existing.x,
              y: existing.y,
              vx: existing.vx,
              vy: existing.vy,
              fx: existing.fx,
              fy: existing.fy
            };
          } else {
            return {
              ...n,
              x: width / 2 + (Math.random() - 0.5) * 200,
              y: height / 2 + (Math.random() - 0.5) * 200
            };
          }
        });
        const newSimulationLinks = links.map((l) => ({
          ...l,
          source: l.source,
          target: l.target
        }));
        simulation.nodes(newSimulationNodes);
        const linkForce = simulation.force("link");
        if (linkForce) {
          linkForce.links(newSimulationLinks);
        }
        linkGroup.selectAll("line").data(newSimulationLinks, (d) => d.id).join(
          (enter) => enter.append("line").attr("stroke", "#6b7280").attr("stroke-width", 1.5).attr("stroke-opacity", 0).attr("marker-end", "url(#arrow)").call((enter2) => enter2.transition().duration(300).attr("stroke-opacity", 0.6)),
          (update) => update,
          (exit) => exit.transition().duration(200).attr("stroke-opacity", 0).remove()
        );
        const nodeSelection = nodeGroup.selectAll("g.node").data(newSimulationNodes, (d) => d.uid).join(
          (enter) => {
            const g = renderNodeGroup(enter);
            g.attr("opacity", 0).transition().duration(300).attr("opacity", 1);
            return g;
          },
          (update) => update,
          (exit) => exit.transition().duration(200).attr("opacity", 0).remove()
        );
        const drag = createDragBehavior();
        if (drag) {
          nodeSelection.call(drag);
        }
        simulation.on("tick", () => {
          linkGroup.selectAll("line").attr("x1", (d) => d.source.x ?? 0).attr("y1", (d) => d.source.y ?? 0).attr("x2", (d) => d.target.x ?? 0).attr("y2", (d) => d.target.y ?? 0);
          nodeGroup.selectAll("g.node").attr("transform", (d) => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });
        simulation.alpha(0.3).restart();
        linkGroup.selectAll("line").attr("x1", (d) => d.source.x ?? 0).attr("y1", (d) => d.source.y ?? 0).attr("x2", (d) => d.target.x ?? 0).attr("y2", (d) => d.target.y ?? 0);
        nodeGroup.selectAll("g.node").attr("transform", (d) => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
      }
    }, [nodes, links, width, height, renderNodeGroup, createDragBehavior]);
    reactExports.useEffect(() => {
      if (!svgRef.current) return;
      select(svgRef.current).selectAll(".selection-ring").attr("opacity", (d) => d.uid === selectedNodeId ? 1 : 0);
    }, [selectedNodeId]);
    reactExports.useEffect(() => {
      if (!svgRef.current) return;
      const svg = select(svgRef.current);
      const highlighted = highlightedNodeUids;
      svg.selectAll(".node").transition().duration(300).attr("opacity", (d) => {
        if (!highlighted) return 1;
        return highlighted.has(d.uid) ? 1 : 0.15;
      });
      svg.selectAll("line").transition().duration(300).attr("stroke-opacity", (d) => {
        if (!highlighted) return 0.6;
        const sourceUid = typeof d.source === "string" ? d.source : d.source.uid;
        const targetUid = typeof d.target === "string" ? d.target : d.target.uid;
        return highlighted.has(sourceUid) && highlighted.has(targetUid) ? 0.9 : 0.05;
      }).attr("stroke-width", (d) => {
        if (!highlighted) return 1.5;
        const sourceUid = typeof d.source === "string" ? d.source : d.source.uid;
        const targetUid = typeof d.target === "string" ? d.target : d.target.uid;
        return highlighted.has(sourceUid) && highlighted.has(targetUid) ? 2.5 : 1;
      });
    }, [highlightedNodeUids]);
    return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { ref: containerRef, className: "w-full h-full bg-[var(--color-app-bg)] relative overflow-hidden", style: { minHeight: 400 }, children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { ref: svgRef, className: "block w-full h-full", style: { position: "absolute", top: 0, left: 0 } }),
      nodes.length === 0 && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 flex items-center justify-center text-[var(--color-text-muted)]", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-16 h-16 mx-auto mb-4 opacity-50", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
          "path",
          {
            strokeLinecap: "round",
            strokeLinejoin: "round",
            strokeWidth: 1.5,
            d: "M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"
          }
        ) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("p", { children: "No resources found in this namespace" })
      ] }) })
    ] });
  }
);
NamespaceGraph.displayName = "NamespaceGraph";

const TIMESTAMP_PRESETS = [
  { label: "Now", expression: "now", isLive: true },
  { label: "1h ago", expression: "now-1h" },
  { label: "6h ago", expression: "now-6h" },
  { label: "24h ago", expression: "now-24h" }
];

const TimestampPicker = ({
  expression,
  onChange,
  resolvedTimestamp
}) => {
  const [isOpen, setIsOpen] = reactExports.useState(false);
  const [customInput, setCustomInput] = reactExports.useState("");
  const [validationError, setValidationError] = reactExports.useState(null);
  const dropdownRef = reactExports.useRef(null);
  reactExports.useEffect(() => {
    if (isOpen) {
      setCustomInput(expression);
      setValidationError(null);
    }
  }, [isOpen, expression]);
  reactExports.useEffect(() => {
    const handleClickOutside = (event) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setIsOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);
  const selectedPreset = reactExports.useMemo(() => {
    return TIMESTAMP_PRESETS.find((p) => p.expression === expression.toLowerCase().trim());
  }, [expression]);
  const isLive = selectedPreset?.isLive ?? false;
  const displayLabel = reactExports.useMemo(() => {
    if (selectedPreset) {
      return selectedPreset.label;
    }
    if (isHumanFriendlyExpression(expression)) {
      return expression;
    }
    if (resolvedTimestamp) {
      return formatDateTimeForDisplay(resolvedTimestamp);
    }
    return expression;
  }, [selectedPreset, expression, resolvedTimestamp]);
  const handlePresetClick = (preset) => {
    onChange(preset.expression);
    setIsOpen(false);
  };
  const handleCustomApply = () => {
    const parsed = parseTimeExpression(customInput);
    if (!parsed) {
      setValidationError(`Invalid time: "${customInput}"`);
      return;
    }
    onChange(customInput);
    setIsOpen(false);
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative ml-auto", ref: dropdownRef, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs(
      "button",
      {
        onClick: () => setIsOpen(!isOpen),
        className: "flex items-center gap-2 px-4 py-2 text-sm font-medium rounded-md border\n                   border-[var(--color-border-soft)] bg-[var(--color-input-bg)]\n                   text-[var(--color-text-primary)] hover:border-brand-500\n                   hover:text-brand-200 transition-all min-w-[140px] justify-between",
        children: [
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4", fill: "none", stroke: "currentColor", viewBox: "0 0 24 24", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
              "path",
              {
                strokeLinecap: "round",
                strokeLinejoin: "round",
                strokeWidth: "2",
                d: "M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
              }
            ) }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "truncate text-xs", children: displayLabel }),
            isLive && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 rounded-full bg-green-400 animate-pulse", title: "Live" })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "svg",
            {
              className: `w-4 h-4 text-[var(--color-text-muted)] transition-transform duration-200
                        ${isOpen ? "rotate-180" : ""}`,
              fill: "none",
              stroke: "currentColor",
              viewBox: "0 0 24 24",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: "2", d: "M19 9l-7 7-7-7" })
            }
          )
        ]
      }
    ),
    isOpen && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute top-full right-0 mt-2 w-72 bg-[var(--color-surface-elevated)]\n                        border border-[var(--color-border-soft)] rounded-lg shadow-xl z-[60] p-4\n                        animate-in fade-in zoom-in-95 duration-100 ring-1 ring-black/10", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-4", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("label", { className: "block text-xs font-medium text-[var(--color-text-muted)] mb-2", children: "Quick Select" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "grid grid-cols-2 gap-2", children: TIMESTAMP_PRESETS.map((preset) => {
          const isSelected = selectedPreset?.expression === preset.expression;
          return /* @__PURE__ */ jsxRuntimeExports.jsxs(
            "button",
            {
              onClick: () => handlePresetClick(preset),
              className: `px-3 py-1.5 text-xs font-medium rounded-md border transition-all ${isSelected ? "bg-brand-600 border-brand-500 text-white ring-1 ring-brand-500/20" : "border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:border-brand-500 hover:text-brand-200"}`,
              children: [
                preset.label,
                preset.isLive && isSelected && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "ml-1 w-1.5 h-1.5 inline-block rounded-full bg-green-300" })
              ]
            },
            preset.expression
          );
        }) })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-4", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("label", { className: "block text-xs font-medium text-[var(--color-text-muted)] mb-2", children: "Custom Time" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          TimeInputWithCalendar,
          {
            value: customInput,
            onChange: (value) => {
              setCustomInput(value);
              setValidationError(null);
            },
            onEnter: handleCustomApply,
            placeholder: "e.g., 2h ago, 2025-01-14 10:00"
          }
        ),
        validationError && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "mt-2 text-xs text-red-400 bg-red-900/20 border border-red-500/40\n                              rounded px-2 py-1", children: validationError })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex justify-end", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: handleCustomApply,
          className: "px-4 py-1.5 text-sm bg-brand-600 hover:bg-brand-700 text-white\n                         font-medium rounded-md transition-colors",
          children: "Apply"
        }
      ) })
    ] })
  ] });
};

const LOOKBACK_LABELS = Object.fromEntries(
  LOOKBACK_OPTIONS.map((opt) => [opt.value, opt.label])
);
const NamespaceGraphControls = ({
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
  onLookbackChange
}) => {
  const handleNamespaceChange = (value) => {
    if (value && typeof value === "string") {
      onNamespaceChange(value);
    }
  };
  const handleKindsChange = (value) => {
    onKindsChange(value || []);
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "w-full bg-[var(--color-surface-elevated)]/95 backdrop-blur border-b border-[var(--color-border-soft)] p-4 flex flex-row gap-6 items-center shadow-lg z-30 text-[var(--color-text-primary)] transition-colors duration-300", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      SelectDropdown,
      {
        label: "Select Namespace",
        options: namespaces,
        selected: namespace,
        onChange: handleNamespaceChange,
        multiple: false,
        minWidth: "200px"
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      SelectDropdown,
      {
        label: "All Kinds",
        options: availableKinds,
        selected: kinds,
        onChange: handleKindsChange,
        multiple: true,
        minWidth: "200px"
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      TimestampPicker,
      {
        expression: timestampExpression,
        onChange: onTimestampChange,
        resolvedTimestamp
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      SelectDropdown,
      {
        label: "Lookback",
        options: LOOKBACK_OPTIONS.map((opt) => opt.value),
        selected: lookback,
        onChange: (value) => {
          if (value && typeof value === "string") {
            onLookbackChange(value);
          }
        },
        multiple: false,
        searchable: false,
        sortOptions: false,
        formatOption: (value) => LOOKBACK_LABELS[value] || value,
        minWidth: "140px"
      }
    )
  ] });
};

const NamespaceGraphLegend = ({
  showRelationships = true
}) => {
  const statusItems = [
    { status: "Ready", color: NODE_STATUS_COLORS.Ready, label: "Ready" },
    { status: "Warning", color: NODE_STATUS_COLORS.Warning, label: "Warning" },
    { status: "Error", color: NODE_STATUS_COLORS.Error, label: "Error" },
    { status: "Terminating", color: NODE_STATUS_COLORS.Terminating, label: "Terminating" },
    { status: "Unknown", color: NODE_STATUS_COLORS.Unknown, label: "Unknown" }
  ];
  const relationshipItems = Object.entries(RELATIONSHIP_LABELS).slice(0, 4);
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute bottom-4 left-4 p-3 rounded-lg bg-[var(--color-surface-muted)] \n                    border border-[var(--color-border-soft)] shadow-lg", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs font-semibold text-[var(--color-text-muted)] mb-2 uppercase tracking-wide", children: "Legend" }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex flex-wrap gap-3 mb-3", children: statusItems.map(({ status, color, label }) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "div",
        {
          className: "w-3 h-3 rounded-full",
          style: {
            backgroundColor: color,
            boxShadow: status === "Error" ? `0 0 6px ${color}` : "none"
          }
        }
      ),
      /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: label })
    ] }, status)) }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-wrap gap-3 mb-3 pt-2 border-t border-[var(--color-border-soft)]", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "div",
          {
            className: "w-3 h-3 rounded-full border-2 border-dashed",
            style: { borderColor: "#9ca3af" }
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: "Cluster-scoped" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-3 h-3 rounded-full bg-gray-500" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "div",
            {
              className: "absolute -top-1 -right-1 w-2 h-2 rounded-full flex items-center justify-center text-white",
              style: { backgroundColor: "#ef4444", fontSize: "6px" },
              children: "!"
            }
          )
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: "Has Anomalies" })
      ] })
    ] }),
    showRelationships && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "pt-2 border-t border-[var(--color-border-soft)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex flex-wrap gap-3", children: relationshipItems.map(([type, label]) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("svg", { width: "20", height: "8", viewBox: "0 0 20 8", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "line",
          {
            x1: "0",
            y1: "4",
            x2: "14",
            y2: "4",
            stroke: "#6b7280",
            strokeWidth: "1.5"
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "polygon",
          {
            points: "14,1 20,4 14,7",
            fill: "#6b7280"
          }
        )
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: label })
    ] }, type)) }) }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mt-3 pt-2 border-t border-[var(--color-border-soft)] text-[10px] text-[var(--color-text-muted)]", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-1", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "opacity-70", children: "Drag nodes" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-2 opacity-40", children: "|" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "opacity-70", children: "Scroll to zoom" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-2 opacity-40", children: "|" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "opacity-70", children: "Click node for details" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "opacity-60", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono bg-[var(--color-surface-elevated)] px-1 rounded", children: "Esc" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1", children: "back" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1 opacity-40", children: "|" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono bg-[var(--color-surface-elevated)] px-1 rounded", children: "R" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1", children: "refresh" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1 opacity-40", children: "|" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono bg-[var(--color-surface-elevated)] px-1 rounded", children: "F" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1", children: "fit" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1 opacity-40", children: "|" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono bg-[var(--color-surface-elevated)] px-1 rounded", children: "+/-" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "mx-1", children: "zoom" })
      ] })
    ] })
  ] });
};

const SEVERITY_COLORS = {
  critical: { bg: "bg-red-600", text: "text-white" },
  high: { bg: "bg-red-500", text: "text-white" },
  medium: { bg: "bg-amber-500", text: "text-white" },
  low: { bg: "bg-blue-500", text: "text-white" }
};
const NodeDetailPanel = ({
  node,
  anomalies = [],
  onClose
}) => {
  const statusColor = NODE_STATUS_COLORS[node.status] || NODE_STATUS_COLORS.Unknown;
  const formatTimestamp = (nanos) => {
    const date = new Date(nanos / 1e6);
    return date.toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit"
    });
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "w-80 h-full border-l border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] flex flex-col", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between px-4 py-3 border-b border-[var(--color-border-soft)]", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-sm font-semibold text-[var(--color-text-primary)]", children: "Resource Details" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          onClick: onClose,
          className: "p-1 rounded hover:bg-[var(--color-surface-elevated)] text-[var(--color-text-muted)]\n                     hover:text-[var(--color-text-primary)] transition-colors",
          children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M6 18L18 6M6 6l12 12" }) })
        }
      )
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 overflow-y-auto p-4 space-y-4", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "space-y-3", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "px-2 py-0.5 rounded bg-[var(--color-surface-elevated)] text-xs font-medium text-[var(--color-text-muted)]", children: node.kind }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1.5", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx(
              "div",
              {
                className: "w-2.5 h-2.5 rounded-full",
                style: { backgroundColor: statusColor }
              }
            ),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: node.status })
          ] })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-1", children: "Name" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm font-mono text-[var(--color-text-primary)] break-all", children: node.name })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-1", children: "Namespace" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-primary)]", children: node.isClusterScoped ? /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "italic text-[var(--color-text-muted)]", children: "Cluster-scoped" }) : node.namespace })
        ] }),
        node.apiGroup && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-1", children: "API Group" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm font-mono text-[var(--color-text-primary)]", children: node.apiGroup })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-1", children: "UID" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs font-mono text-[var(--color-text-muted)] break-all", children: node.uid })
        ] })
      ] }),
      node.labels && Object.keys(node.labels).length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "pt-3 border-t border-[var(--color-border-soft)]", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-2", children: "Labels" }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "space-y-1", children: [
          Object.entries(node.labels).slice(0, 8).map(([key, value]) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-start gap-1 text-xs", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[var(--color-text-muted)] flex-shrink-0", children: [
              key,
              ":"
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono text-[var(--color-text-primary)] break-all", children: value })
          ] }, key)),
          Object.keys(node.labels).length > 8 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs text-[var(--color-text-muted)] italic", children: [
            "+",
            Object.keys(node.labels).length - 8,
            " more"
          ] })
        ] })
      ] }),
      node.latestEvent && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "pt-3 border-t border-[var(--color-border-soft)]", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-2", children: "Latest Event" }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-2 rounded bg-[var(--color-surface-elevated)] space-y-1", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs font-medium text-[var(--color-text-primary)]", children: node.latestEvent.eventType }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: formatTimestamp(node.latestEvent.timestamp) })
          ] }),
          node.latestEvent.description && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)]", children: node.latestEvent.description }),
          node.latestEvent.errorMessage && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "group relative", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-red-500 cursor-help", children: node.latestEvent.errorMessage.length > 100 ? `${node.latestEvent.errorMessage.slice(-100)}...` : node.latestEvent.errorMessage }),
            node.latestEvent.errorMessage.length > 100 && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute z-50 hidden group-hover:block bottom-full left-0 mb-1 p-2\n                                    bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)]\n                                    rounded shadow-lg max-w-md max-h-48 overflow-auto", children: /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-red-500 whitespace-pre-wrap break-words", children: node.latestEvent.errorMessage }) })
          ] }),
          node.latestEvent.containerIssues && node.latestEvent.containerIssues.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex flex-wrap gap-1 mt-1", children: node.latestEvent.containerIssues.map((issue, idx) => /* @__PURE__ */ jsxRuntimeExports.jsx(
            "span",
            {
              className: "px-1.5 py-0.5 rounded text-xs bg-amber-500/20 text-amber-500",
              children: issue
            },
            idx
          )) })
        ] })
      ] }),
      node.latestEvent?.specChanges && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "pt-3 border-t border-[var(--color-border-soft)]", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-2", children: "Spec Changes (within lookback)" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "p-2 rounded bg-[var(--color-surface-elevated)] overflow-x-auto", children: /* @__PURE__ */ jsxRuntimeExports.jsx("pre", { className: "text-xs font-mono whitespace-pre-wrap break-words", children: node.latestEvent.specChanges.split("\n").map((line, idx) => {
          let lineClass = "text-[var(--color-text-muted)]";
          if (line.startsWith("+")) lineClass = "text-green-500";
          else if (line.startsWith("-")) lineClass = "text-red-500";
          else if (line.startsWith("@@")) lineClass = "text-blue-400 font-semibold";
          return /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: lineClass, children: [
            line,
            idx < node.latestEvent.specChanges.split("\n").length - 1 ? "\n" : ""
          ] }, idx);
        }) }) })
      ] }),
      anomalies.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "pt-3 border-t border-[var(--color-border-soft)]", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2 mb-2", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)]", children: "Anomalies" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "px-1.5 py-0.5 rounded-full bg-red-500 text-white text-xs font-medium", children: anomalies.length })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "space-y-2", children: anomalies.map((anomaly, idx) => {
          const severityStyle = SEVERITY_COLORS[anomaly.severity] || SEVERITY_COLORS.low;
          const hasDetails = anomaly.details && Object.keys(anomaly.details).length > 0;
          return /* @__PURE__ */ jsxRuntimeExports.jsxs(
            "div",
            {
              className: "p-2 rounded bg-[var(--color-surface-elevated)] space-y-2",
              children: [
                /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: `px-1.5 py-0.5 rounded text-xs font-medium ${severityStyle.bg} ${severityStyle.text}`, children: anomaly.severity }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)] capitalize", children: anomaly.category })
                ] }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs font-medium text-[var(--color-text-primary)]", children: anomaly.type }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)]", children: anomaly.summary }),
                hasDetails && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "pt-2 mt-2 border-t border-[var(--color-border-soft)] space-y-1", children: Object.entries(anomaly.details).map(([key, value]) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-start gap-1 text-xs", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[var(--color-text-muted)] flex-shrink-0", children: [
                    key,
                    ":"
                  ] }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "font-mono text-[var(--color-text-primary)] break-all", children: typeof value === "object" ? JSON.stringify(value) : String(value) })
                ] }, key)) })
              ]
            },
            idx
          );
        }) })
      ] })
    ] })
  ] });
};

const ZoomControls = ({
  onZoomIn,
  onZoomOut,
  onFitToView,
  onResetZoom
}) => {
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute bottom-4 right-4 flex flex-col gap-1 bg-[var(--color-surface-elevated)] \n                    border border-[var(--color-border-soft)] rounded-lg shadow-lg p-1 z-10", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      "button",
      {
        onClick: onZoomIn,
        title: "Zoom In (+)",
        className: "p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors\n                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]",
        children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M12 6v12m6-6H6" }) })
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      "button",
      {
        onClick: onZoomOut,
        title: "Zoom Out (-)",
        className: "p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors\n                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]",
        children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M18 12H6" }) })
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "h-px bg-[var(--color-border-soft)] my-1" }),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      "button",
      {
        onClick: onFitToView,
        title: "Fit to View (F)",
        className: "p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors\n                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]",
        children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
          "path",
          {
            strokeLinecap: "round",
            strokeLinejoin: "round",
            strokeWidth: 2,
            d: "M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4"
          }
        ) })
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      "button",
      {
        onClick: onResetZoom,
        title: "Reset Zoom (0)",
        className: "p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors\n                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]",
        children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-5 h-5", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
          "path",
          {
            strokeLinecap: "round",
            strokeLinejoin: "round",
            strokeWidth: 2,
            d: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
          }
        ) })
      }
    )
  ] });
};

const CausalPathsSidebar = ({
  causalPaths,
  selectedPathId,
  onPathSelect,
  isLoading = false
}) => {
  const sortedPaths = React.useMemo(
    () => [...causalPaths].sort((a, b) => b.confidenceScore - a.confidenceScore),
    [causalPaths]
  );
  const getConfidenceColor = (score) => {
    if (score >= 0.7) return "text-green-400 bg-green-500/20";
    if (score >= 0.4) return "text-amber-400 bg-amber-500/20";
    return "text-red-400 bg-red-500/20";
  };
  const getSeverityColor = (severity) => {
    switch (severity) {
      case "critical":
        return "bg-red-500";
      case "high":
        return "bg-orange-500";
      case "medium":
        return "bg-yellow-500";
      case "low":
        return "bg-blue-400";
      default:
        return "bg-gray-500";
    }
  };
  const handlePathClick = (pathId) => {
    onPathSelect(selectedPathId === pathId ? null : pathId);
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "div",
    {
      className: "h-full flex flex-col bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)]",
      style: { width: "220px", minWidth: "220px" },
      children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "px-4 py-3 border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wider", children: "Causal Paths" }),
            causalPaths.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[10px] text-[var(--color-text-muted)] bg-[var(--color-surface-active)] px-2 py-0.5 rounded-full", children: [
              causalPaths.length,
              " path",
              causalPaths.length !== 1 ? "s" : ""
            ] })
          ] }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[10px] text-[var(--color-text-muted)] mt-1", children: "Ranked candidate root causes" })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1 overflow-y-auto", children: isLoading ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex items-center justify-center h-full", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-2", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[10px] text-[var(--color-text-muted)]", children: "Loading paths..." })
        ] }) }) : sortedPaths.length > 0 ? /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "divide-y divide-[var(--color-border-soft)]", children: sortedPaths.map((path, index) => {
          const isSelected = selectedPathId === path.id;
          const confidencePercent = Math.round(path.confidenceScore * 100);
          return /* @__PURE__ */ jsxRuntimeExports.jsxs(
            "div",
            {
              onClick: () => handlePathClick(path.id),
              className: `p-3 cursor-pointer transition-colors ${isSelected ? "bg-blue-500/20 border-l-2 border-l-blue-500" : "hover:bg-[var(--color-surface-hover)] border-l-2 border-l-transparent"}`,
              children: [
                /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center justify-between mb-2", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2", children: [
                    /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[10px] font-bold text-[var(--color-text-muted)] uppercase", children: [
                      "#",
                      index + 1
                    ] }),
                    /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: `text-[9px] font-semibold px-1.5 py-0.5 rounded ${getConfidenceColor(path.confidenceScore)}`, children: [
                      confidencePercent,
                      "%"
                    ] })
                  ] }),
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-1", children: [
                    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `w-1.5 h-1.5 rounded-full ${getSeverityColor(path.ranking.maxAnomalySeverity)}` }),
                    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[9px] text-[var(--color-text-muted)]", children: path.ranking.maxAnomalySeverity })
                  ] })
                ] }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "mb-2", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-xs font-semibold text-[var(--color-text-primary)]", children: [
                  path.candidateRoot.resource.kind,
                  "/",
                  path.candidateRoot.resource.name
                ] }) }),
                path.candidateRoot.anomalies && path.candidateRoot.anomalies.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "mb-2", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-wrap gap-1", children: [
                  path.candidateRoot.anomalies.slice(0, 2).map((anomaly, i) => /* @__PURE__ */ jsxRuntimeExports.jsx(
                    "span",
                    {
                      className: "text-[8px] px-1.5 py-0.5 bg-red-500/20 text-red-400 rounded truncate max-w-full",
                      title: anomaly.summary,
                      children: anomaly.type
                    },
                    i
                  )),
                  path.candidateRoot.anomalies.length > 2 && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[8px] px-1.5 py-0.5 bg-gray-500/20 text-gray-400 rounded", children: [
                    "+",
                    path.candidateRoot.anomalies.length - 2
                  ] })
                ] }) }),
                /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-3 text-[9px] text-[var(--color-text-muted)]", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { title: "Effective causal distance (cause-introducing edges)", children: [
                    path.ranking.effectiveCausalDistance,
                    " hop",
                    path.ranking.effectiveCausalDistance !== 1 ? "s" : ""
                  ] }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "-" }),
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { title: "Number of steps in path", children: [
                    path.steps.length,
                    " step",
                    path.steps.length !== 1 ? "s" : ""
                  ] })
                ] }),
                isSelected && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mt-3 pt-3 border-t border-[var(--color-border-soft)]", children: [
                  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[10px] text-[var(--color-text-primary)] mb-3 leading-relaxed", children: path.explanation }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)] uppercase mb-2", children: "Path Steps" }),
                  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "space-y-1", children: path.steps.map((step, stepIndex) => {
                    const hasSpecChanges = step.node.primaryEvent?.diff && step.node.primaryEvent.diff.length > 0;
                    return /* @__PURE__ */ jsxRuntimeExports.jsxs(
                      "div",
                      {
                        className: "group relative flex items-center gap-2",
                        children: [
                          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: `w-4 h-4 rounded-full flex items-center justify-center text-[8px] font-bold ${stepIndex === 0 ? "bg-red-500 text-white" : stepIndex === path.steps.length - 1 ? "bg-blue-500 text-white" : "bg-[var(--color-surface-muted)] text-[var(--color-text-muted)]"}`, children: stepIndex + 1 }),
                          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 min-w-0", children: [
                            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[10px] text-[var(--color-text-primary)] truncate flex items-center gap-1", children: [
                              step.node.resource.kind,
                              "/",
                              step.node.resource.name,
                              hasSpecChanges && /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[8px] px-1 py-0.5 bg-blue-500/20 text-blue-400 rounded", title: "Has spec changes", children: "Δ" })
                            ] }),
                            step.edge && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-[8px] text-[var(--color-text-muted)]", children: [
                              "via ",
                              step.edge.relationshipType,
                              /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: `ml-1 ${step.edge.edgeCategory === "CAUSE_INTRODUCING" ? "text-orange-400" : "text-gray-500"}`, children: [
                                "(",
                                step.edge.edgeCategory === "CAUSE_INTRODUCING" ? "causal" : "ownership",
                                ")"
                              ] })
                            ] })
                          ] }),
                          step.node.anomalies && step.node.anomalies.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsx(
                            "div",
                            {
                              className: "w-2 h-2 rounded-full bg-red-500",
                              title: `${step.node.anomalies.length} anomal${step.node.anomalies.length > 1 ? "ies" : "y"}`
                            }
                          ),
                          hasSpecChanges && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "absolute z-50 hidden group-hover:block left-0 top-full mt-1 p-2\n                                                bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)]\n                                                rounded shadow-lg max-w-xs max-h-48 overflow-auto", children: [
                            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)] uppercase mb-1", children: "Spec Changes" }),
                            /* @__PURE__ */ jsxRuntimeExports.jsx("pre", { className: "text-[9px] font-mono whitespace-pre-wrap break-words", children: step.node.primaryEvent.diff.map((d, i) => {
                              const opColor = d.op === "add" ? "text-green-500" : d.op === "remove" ? "text-red-500" : "text-amber-500";
                              return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: opColor, children: [
                                d.op === "add" ? "+" : d.op === "remove" ? "-" : "~",
                                " ",
                                d.path,
                                ": ",
                                d.op === "remove" ? JSON.stringify(d.old) : d.op === "add" ? JSON.stringify(d.new) : `${JSON.stringify(d.old)} → ${JSON.stringify(d.new)}`
                              ] }, i);
                            }) })
                          ] })
                        ]
                      },
                      stepIndex
                    );
                  }) }),
                  /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mt-3 pt-2 border-t border-[var(--color-border-soft)]", children: [
                    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-[9px] text-[var(--color-text-muted)] uppercase mb-1", children: "Ranking Factors" }),
                    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "grid grid-cols-2 gap-x-3 gap-y-1 text-[9px]", children: [
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex justify-between", children: [
                        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-muted)]", children: "Temporal" }),
                        /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[var(--color-text-primary)]", children: [
                          Math.round(path.ranking.temporalScore * 100),
                          "%"
                        ] })
                      ] }),
                      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex justify-between", children: [
                        /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-muted)]", children: "Severity" }),
                        /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-[var(--color-text-primary)]", children: [
                          Math.round(path.ranking.severityScore * 100),
                          "%"
                        ] })
                      ] })
                    ] })
                  ] })
                ] })
              ]
            },
            path.id
          );
        }) }) : /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex items-center justify-center h-full p-6", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "text-center", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            "svg",
            {
              className: "w-12 h-12 mx-auto mb-3 text-[var(--color-text-muted)] opacity-40",
              fill: "none",
              viewBox: "0 0 24 24",
              stroke: "currentColor",
              children: /* @__PURE__ */ jsxRuntimeExports.jsx(
                "path",
                {
                  strokeLinecap: "round",
                  strokeLinejoin: "round",
                  strokeWidth: 1.5,
                  d: "M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                }
              )
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)]", children: "No causal paths found" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[10px] text-[var(--color-text-muted)] mt-1", children: "No anomalies detected in this namespace" })
        ] }) }) }),
        selectedPathId && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "px-3 py-2 border-t border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[9px] text-[var(--color-text-muted)] text-center", children: "Click path again to deselect" }) })
      ]
    }
  );
};

const LoadingSpinner = ({ message = "Loading..." }) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center justify-center h-full", children: [
  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-10 h-10 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4" }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-[var(--color-text-muted)]", children: message })
] });
const ErrorMessage = ({ error, onRetry }) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center justify-center h-full p-8", children: [
  /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-16 h-16 text-red-500 mb-4", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
    "path",
    {
      strokeLinecap: "round",
      strokeLinejoin: "round",
      strokeWidth: 1.5,
      d: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
    }
  ) }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-lg font-medium text-[var(--color-text-primary)] mb-2", children: "Failed to load graph" }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)] mb-4 text-center max-w-md", children: error.message }),
  onRetry && /* @__PURE__ */ jsxRuntimeExports.jsx(
    "button",
    {
      onClick: onRetry,
      className: "px-4 py-2 rounded-md bg-blue-600 hover:bg-blue-700 text-white text-sm transition-colors",
      children: "Try Again"
    }
  )
] });
const EmptyState = () => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center justify-center h-full p-8", children: [
  /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-16 h-16 text-[var(--color-text-muted)] opacity-50 mb-4", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
    "path",
    {
      strokeLinecap: "round",
      strokeLinejoin: "round",
      strokeWidth: 1.5,
      d: "M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
    }
  ) }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-lg font-medium text-[var(--color-text-primary)] mb-2", children: "No Namespaces Found" }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)] text-center max-w-md", children: "No namespaces are available in the cluster. Make sure resources are being monitored." })
] });
function NamespaceGraphPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const graphRef = reactExports.useRef(null);
  const [selectedNode, setSelectedNode] = reactExports.useState(null);
  const [selectedCausalPathId, setSelectedCausalPathId] = reactExports.useState(null);
  const metadataTimeRange = reactExports.useMemo(() => ({
    start: new Date(Date.now() - 24 * 60 * 60 * 1e3),
    end: /* @__PURE__ */ new Date()
  }), []);
  const { namespaces, kinds: availableKinds, loading: namespacesLoading, error: namespacesError } = useMetadata(metadataTimeRange);
  const sortedNamespaces = reactExports.useMemo(() => [...namespaces].sort(), [namespaces]);
  const { namespace: persistedNamespace, setNamespace: setPersistedNamespace } = usePersistedGraphNamespace(sortedNamespaces);
  const urlNamespace = searchParams.get("namespace");
  const selectedNamespace = urlNamespace || persistedNamespace;
  const setSelectedNamespace = reactExports.useCallback((ns) => {
    if (ns) {
      setSearchParams({ namespace: ns }, { replace: true });
    } else {
      setSearchParams({}, { replace: true });
    }
    setPersistedNamespace(ns);
    setSelectedNode(null);
    setSelectedCausalPathId(null);
  }, [setSearchParams, setPersistedNamespace]);
  reactExports.useEffect(() => {
    if (!urlNamespace && persistedNamespace && sortedNamespaces.includes(persistedNamespace)) {
      setSearchParams({ namespace: persistedNamespace }, { replace: true });
    }
  }, [urlNamespace, persistedNamespace, sortedNamespaces, setSearchParams]);
  const { defaultKinds } = useSettings();
  const { kinds: selectedKinds, setKinds } = usePersistedFilters(availableKinds, [], defaultKinds);
  const { expression: timestampExpression, setExpression: setTimestampExpression } = usePersistedGraphTimestamp();
  const { lookback, setLookback } = usePersistedGraphLookback();
  const graphTimestamp = reactExports.useMemo(() => {
    const parsed = parseTimeExpression(timestampExpression);
    if (parsed) {
      return parsed;
    }
    return /* @__PURE__ */ new Date();
  }, [timestampExpression, selectedNamespace]);
  const isLiveMode = timestampExpression.toLowerCase().trim() === "now";
  reactExports.useEffect(() => {
    if (!selectedNamespace && !namespacesLoading && sortedNamespaces.length > 0) {
      setSelectedNamespace(sortedNamespaces[0]);
    }
  }, [selectedNamespace, namespacesLoading, sortedNamespaces, setSelectedNamespace]);
  const { data, isLoading, isLoadingMore, hasMore, pagesLoaded, error, refetch } = useNamespaceGraph({
    namespace: selectedNamespace,
    timestamp: graphTimestamp,
    includeAnomalies: true,
    includeCausalPaths: true,
    lookback,
    enabled: !!selectedNamespace,
    pageSize: 50,
    autoLoad: true
  });
  const filteredData = reactExports.useMemo(() => {
    if (!data) return null;
    let causalPathNodeUids = null;
    if (selectedCausalPathId && data.causalPaths) {
      const path = data.causalPaths.find((p) => p.id === selectedCausalPathId);
      if (path) {
        causalPathNodeUids = new Set(path.steps.map((step) => step.node.resource.uid));
      }
    }
    const filteredNodes = selectedKinds.length > 0 ? data.graph.nodes.filter(
      (node) => selectedKinds.includes(node.kind) || causalPathNodeUids?.has(node.uid)
    ) : [];
    const nodesToShow = filteredNodes.length > 0 ? filteredNodes : data.graph.nodes;
    const nodeUids = new Set(nodesToShow.map((n) => n.uid));
    const filteredEdges = data.graph.edges.filter(
      (edge) => nodeUids.has(edge.source) && nodeUids.has(edge.target)
    );
    return {
      ...data,
      graph: { nodes: nodesToShow, edges: filteredEdges },
      metadata: { ...data.metadata, nodeCount: nodesToShow.length, edgeCount: filteredEdges.length }
    };
  }, [data, selectedKinds, selectedCausalPathId]);
  const selectedNodeAnomalies = reactExports.useMemo(() => {
    if (!selectedNode || !data?.anomalies) return [];
    return data.anomalies.filter((a) => a.node.uid === selectedNode.uid);
  }, [selectedNode, data?.anomalies]);
  const highlightedNodeUids = reactExports.useMemo(() => {
    if (!selectedCausalPathId || !data?.causalPaths) return null;
    const path = data.causalPaths.find((p) => p.id === selectedCausalPathId);
    if (!path) return null;
    return new Set(path.steps.map((step) => step.node.resource.uid));
  }, [selectedCausalPathId, data?.causalPaths]);
  reactExports.useEffect(() => {
    if (highlightedNodeUids && highlightedNodeUids.size > 0) {
      const timeoutId = setTimeout(() => {
        graphRef.current?.focusOnNodes(highlightedNodeUids);
      }, 100);
      return () => clearTimeout(timeoutId);
    }
  }, [highlightedNodeUids]);
  const handlePathSelect = reactExports.useCallback((pathId) => {
    setSelectedCausalPathId((prev) => prev === pathId ? null : pathId);
  }, []);
  reactExports.useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }
      switch (e.key) {
        case "Escape":
          if (selectedNode) {
            setSelectedNode(null);
          }
          break;
        case "+":
        case "=":
          if (selectedNamespace) {
            graphRef.current?.zoomIn();
          }
          break;
        case "-":
        case "_":
          if (selectedNamespace) {
            graphRef.current?.zoomOut();
          }
          break;
        case "f":
        case "F":
          if (selectedNamespace && !e.ctrlKey && !e.metaKey) {
            graphRef.current?.fitToView();
          }
          break;
        case "0":
          if (selectedNamespace && !e.ctrlKey && !e.metaKey) {
            graphRef.current?.resetZoom();
          }
          break;
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedNode, selectedNamespace]);
  if (namespacesLoading) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "h-full bg-[var(--color-app-bg)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx(LoadingSpinner, { message: "Loading namespaces..." }) });
  }
  if (namespacesError) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "h-full bg-[var(--color-app-bg)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx(ErrorMessage, { error: namespacesError }) });
  }
  if (sortedNamespaces.length === 0) {
    return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "h-full bg-[var(--color-app-bg)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx(EmptyState, {}) });
  }
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "h-full flex flex-col bg-[var(--color-app-bg)]", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx(
      NamespaceGraphControls,
      {
        namespace: selectedNamespace || sortedNamespaces[0],
        namespaces: sortedNamespaces,
        onNamespaceChange: setSelectedNamespace,
        kinds: selectedKinds,
        availableKinds,
        onKindsChange: setKinds,
        timestampExpression,
        onTimestampChange: setTimestampExpression,
        resolvedTimestamp: graphTimestamp,
        lookback,
        onLookbackChange: setLookback
      }
    ),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex overflow-hidden min-h-0", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 relative overflow-hidden min-w-0", children: [
        isLoading && !data && /* @__PURE__ */ jsxRuntimeExports.jsx(LoadingSpinner, { message: "Loading graph..." }),
        error && !data && /* @__PURE__ */ jsxRuntimeExports.jsx(ErrorMessage, { error, onRetry: refetch }),
        filteredData && /* @__PURE__ */ jsxRuntimeExports.jsxs(jsxRuntimeExports.Fragment, { children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            NamespaceGraph,
            {
              ref: graphRef,
              data: filteredData,
              onNodeClick: setSelectedNode,
              selectedNodeId: selectedNode?.uid,
              highlightedNodeUids
            }
          ),
          /* @__PURE__ */ jsxRuntimeExports.jsx(
            ZoomControls,
            {
              onZoomIn: () => graphRef.current?.zoomIn(),
              onZoomOut: () => graphRef.current?.zoomOut(),
              onFitToView: () => graphRef.current?.fitToView(),
              onResetZoom: () => graphRef.current?.resetZoom()
            }
          ),
          isLoading && data && /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "absolute inset-0 bg-[var(--color-app-bg)]/60 backdrop-blur-sm flex items-center justify-center z-20", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col items-center gap-3 p-4 rounded-lg bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] shadow-xl", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-8 h-8 border-3 border-blue-500 border-t-transparent rounded-full animate-spin" }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-sm text-[var(--color-text-muted)]", children: "Updating graph..." })
          ] }) })
        ] })
      ] }),
      selectedNode && /* @__PURE__ */ jsxRuntimeExports.jsx(
        NodeDetailPanel,
        {
          node: selectedNode,
          anomalies: selectedNodeAnomalies,
          onClose: () => setSelectedNode(null)
        }
      ),
      data && /* @__PURE__ */ jsxRuntimeExports.jsx(
        CausalPathsSidebar,
        {
          causalPaths: data.causalPaths || [],
          selectedPathId: selectedCausalPathId,
          onPathSelect: handlePathSelect,
          isLoading
        }
      )
    ] }),
    filteredData && /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "px-4 py-2 border-t border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]\n                        text-xs text-[var(--color-text-muted)] flex items-center gap-4", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
        filteredData.metadata.nodeCount,
        " resources",
        hasMore && " (loading more...)"
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
        filteredData.metadata.edgeCount,
        " relationships"
      ] }),
      data && data.anomalies && data.anomalies.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-red-400", children: [
        data.anomalies.length,
        " anomalies"
      ] }),
      data && data.causalPaths && data.causalPaths.length > 0 && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-blue-400", children: [
        data.causalPaths.length,
        " causal paths"
      ] }),
      isLoadingMore && /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "text-blue-400 flex items-center gap-1", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("svg", { className: "w-3 h-3 animate-spin", viewBox: "0 0 24 24", fill: "none", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("circle", { className: "opacity-25", cx: "12", cy: "12", r: "10", stroke: "currentColor", strokeWidth: "4" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("path", { className: "opacity-75", fill: "currentColor", d: "M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" })
        ] }),
        "Loading page ",
        pagesLoaded + 1,
        "..."
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "ml-auto flex items-center gap-3", children: [
        isLiveMode ? /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { className: "flex items-center gap-1", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 rounded-full bg-green-400 animate-pulse" }),
          "Live"
        ] }) : /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
          "Snapshot: ",
          formatDateTimeForDisplay(graphTimestamp)
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("span", { children: [
          "Query: ",
          filteredData.metadata.queryExecutionMs,
          "ms"
        ] })
      ] })
    ] })
  ] });
}

const PREVIOUS_AGENTS = [
  { id: 1, time: "1h ago", title: "Certificate expiry investigation" },
  { id: 2, time: "3h ago", title: "High latency in payment service" },
  { id: 3, time: "8h ago", title: "Memory leak in auth-service" },
  { id: 4, time: "32h ago", title: "Failed deployment rollback analysis" },
  { id: 5, time: "2d ago", title: "Network policy misconfiguration" }
];
const ACTIVE_TOOL_CALLS = [
  { name: "cluster_health", status: "running" },
  { name: "resource_timeline", status: "running" },
  { name: "resource_timeline_changes", status: "running" },
  { name: "detect_anomalies", status: "running" }
];
const ToolSpinner = () => /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" });
const ToolCallCard = ({ name }) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-3 px-4 py-3 bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)]", children: [
  /* @__PURE__ */ jsxRuntimeExports.jsx(ToolSpinner, {}),
  /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex flex-col", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-sm font-mono text-blue-400", children: name }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: "Executing..." })
  ] })
] });
const AgentHistoryCard = ({ time, title }) => /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-4 bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)] hover:border-[var(--color-border-default)] transition-colors cursor-pointer", children: [
  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-1", children: time }),
  /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-primary)] line-clamp-2", children: title })
] });
function AgentsPage() {
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "h-full flex bg-[var(--color-app-bg)]", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 flex flex-col min-w-0", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1 overflow-y-auto p-6", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "max-w-3xl mx-auto space-y-6", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex gap-3", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4 text-white", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 bg-[var(--color-surface-elevated)] rounded-lg p-4 border border-[var(--color-border-soft)]", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-muted)] mb-1", children: "You" }),
            /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[var(--color-text-primary)]", children: "cart-api starting to throw errors 30 minutes ago. Please give me all spec changes in the cluster 15 minutes before it started failing." })
          ] })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex gap-3", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-shrink-0 w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center", children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4 text-white", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" }) }) }),
          /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex-1 space-y-4", children: [
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "bg-[var(--color-surface-elevated)] rounded-lg p-4 border border-[var(--color-border-soft)]", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-sm text-[var(--color-text-muted)] mb-2", children: "Spectre Agent" }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[var(--color-text-primary)] mb-4", children: "I'll help you investigate the errors. Let me check the cluster health and gather relevant information about spec changes in the time window you mentioned." }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "text-xs text-[var(--color-text-muted)] mb-3", children: "Executing tools..." }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "grid grid-cols-2 gap-3", children: ACTIVE_TOOL_CALLS.map((tool) => /* @__PURE__ */ jsxRuntimeExports.jsx(ToolCallCard, { name: tool.name }, tool.name)) })
            ] }),
            /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex items-center gap-2 text-[var(--color-text-muted)] text-sm", children: [
              /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "flex gap-1", children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 bg-purple-500 rounded-full animate-bounce", style: { animationDelay: "0ms" } }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 bg-purple-500 rounded-full animate-bounce", style: { animationDelay: "150ms" } }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 bg-purple-500 rounded-full animate-bounce", style: { animationDelay: "300ms" } })
              ] }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { children: "Agent is analyzing..." })
            ] })
          ] })
        ] })
      ] }) }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "border-t border-[var(--color-border-soft)] p-4 bg-[var(--color-surface-muted)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "max-w-3xl mx-auto", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "relative", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "textarea",
          {
            disabled: true,
            placeholder: "Ask the agent to investigate issues, analyze changes, or explain anomalies...",
            className: "w-full px-4 py-3 pr-12 rounded-lg bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)]\n                           text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] resize-none\n                           opacity-50 cursor-not-allowed",
            rows: 1
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "button",
          {
            disabled: true,
            className: "absolute right-3 bottom-5 p-1 rounded-lg bg-blue-600 opacity-50 cursor-not-allowed",
            children: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { className: "w-4 h-4 text-white", fill: "none", viewBox: "0 0 24 24", stroke: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { strokeLinecap: "round", strokeLinejoin: "round", strokeWidth: 2, d: "M12 19l9 2-9-18-9 18 9-2zm0 0v-8" }) })
          }
        )
      ] }) }) })
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "w-72 border-l border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] flex flex-col", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "p-4 border-b border-[var(--color-border-soft)]", children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("h2", { className: "text-sm font-semibold text-[var(--color-text-primary)]", children: "Previous Investigations" }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-xs text-[var(--color-text-muted)] mt-1", children: "Your agent conversation history" })
      ] }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "flex-1 overflow-y-auto p-4 space-y-3", children: PREVIOUS_AGENTS.map((agent) => /* @__PURE__ */ jsxRuntimeExports.jsx(AgentHistoryCard, { time: agent.time, title: agent.title }, agent.id)) }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "p-4 border-t border-[var(--color-border-soft)]", children: /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          disabled: true,
          className: "w-full px-4 py-2 rounded-lg bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)]\n                       text-[var(--color-text-muted)] text-sm opacity-50 cursor-not-allowed",
          children: "+ New Investigation"
        }
      ) })
    ] })
  ] });
}

const INTEGRATIONS = [
  {
    id: "slack",
    name: "Slack",
    description: "Get real-time alerts and investigate incidents directly from Slack channels.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z" }) }),
    color: "#4A154B"
  },
  {
    id: "pagerduty",
    name: "PagerDuty",
    description: "Automatically create and enrich incidents with root cause analysis.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M16.965 1.18C15.085.164 13.769 0 10.683 0H3.73v14.55h6.926c2.743 0 4.8-.164 6.61-1.37 1.975-1.303 3.004-3.484 3.004-6.007 0-2.716-1.262-4.896-3.305-5.994zm-5.5 10.326h-4.21V3.113l3.977-.027c3.62-.028 5.43 1.316 5.43 4.128 0 3.073-2.075 4.292-5.196 4.292zM3.73 17.61h3.525V24H3.73z" }) }),
    color: "#06AC38"
  },
  {
    id: "grafana",
    name: "Grafana",
    description: "Visualize Spectre data in custom Grafana dashboards and panels.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M22.228 10.371c-.037-.466-.186-.858-.39-1.25-.167-.316-.39-.596-.65-.838a3.24 3.24 0 0 0-.817-.597c-.279-.149-.576-.261-.892-.336a4.09 4.09 0 0 0-.929-.112c-.149 0-.298.019-.446.037a7.537 7.537 0 0 0-.15-1.082 5.83 5.83 0 0 0-.334-1.082 5.14 5.14 0 0 0-.521-.985 5.27 5.27 0 0 0-.706-.856 5.615 5.615 0 0 0-.892-.707 5.835 5.835 0 0 0-1.007-.521 5.63 5.63 0 0 0-1.1-.317 6.17 6.17 0 0 0-1.155-.112c-.186 0-.372.019-.558.037-.186.019-.372.056-.558.093a8.757 8.757 0 0 0-.521-1.155 6.81 6.81 0 0 0-.706-.985 5.67 5.67 0 0 0-.892-.818 4.94 4.94 0 0 0-1.044-.596 4.72 4.72 0 0 0-1.155-.336C6.657.019 6.378 0 6.099 0c-.353 0-.706.037-1.044.112-.335.075-.651.186-.967.336-.316.149-.613.335-.873.558-.261.223-.502.465-.706.744-.204.279-.372.576-.502.892-.131.316-.224.651-.28.985-.055.335-.074.67-.055 1.007.019.335.075.67.167.985-.037.019-.074.019-.112.037a3.88 3.88 0 0 0-.91.391c-.28.167-.54.372-.763.615-.223.242-.409.521-.558.818-.149.298-.261.615-.316.948-.056.335-.075.67-.037 1.007.037.335.13.651.26.967.13.316.298.613.503.873.204.261.446.484.724.67a4.86 4.86 0 0 0-.26.948c-.056.335-.075.67-.056 1.007.019.335.074.67.167.985.093.316.223.614.39.892.167.279.372.54.596.763.223.223.484.428.762.596.279.167.576.316.892.409.316.093.65.167.985.186.335.019.67.019 1.007-.037.335-.056.65-.149.967-.28.056.056.112.112.167.167.316.279.651.521 1.007.726.354.204.744.371 1.155.502.409.13.836.223 1.28.279.446.056.892.074 1.34.056.446-.019.892-.075 1.32-.186.428-.112.837-.26 1.227-.446.39-.186.762-.409 1.1-.67.335-.261.632-.558.892-.892.186.019.372.019.558.019.447 0 .892-.056 1.32-.167.428-.112.836-.26 1.227-.447.39-.186.762-.409 1.1-.67.335-.261.632-.558.892-.892.26-.335.484-.706.65-1.1.167-.391.298-.8.372-1.229.074-.428.093-.873.056-1.302-.037-.428-.13-.855-.279-1.266a5.32 5.32 0 0 0-.502-1.155 5.61 5.61 0 0 0-.763-1.007z" }) }),
    color: "#F46800"
  },
  {
    id: "prometheus",
    name: "Prometheus",
    description: "Export metrics and correlate with Kubernetes events and changes.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12 0C5.373 0 0 5.372 0 12c0 6.627 5.373 12 12 12s12-5.373 12-12c0-6.628-5.373-12-12-12zm0 22.46c-1.885 0-3.414-1.455-3.414-3.25h6.828c0 1.795-1.529 3.25-3.414 3.25zm5.64-4.357H6.36v-2.097h11.28v2.097zm-.04-3.127H6.4c-.03-.031-.06-.063-.089-.094-1.083-1.13-1.266-1.925-1.332-2.576-.068-.665-.034-1.048-.034-1.048h10.11s.034.383-.034 1.048c-.066.651-.249 1.446-1.332 2.576-.03.031-.059.063-.089.094zm-1.4-4.548c-.22 0-.398-.357-.398-.796 0-.44.178-.796.398-.796s.398.357.398.796c0 .44-.178.796-.398.796zm-8.4 0c-.22 0-.398-.357-.398-.796 0-.44.178-.796.398-.796s.398.357.398.796c0 .44-.178.796-.398.796z" }) }),
    color: "#E6522C"
  },
  {
    id: "logzio",
    name: "Logz.io",
    description: "Correlate log patterns with infrastructure changes and anomalies.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12 0L1.5 6v12L12 24l10.5-6V6L12 0zm0 2.25l8.25 4.75v9.5L12 21.25 3.75 16.5V7L12 2.25zM12 6a6 6 0 100 12 6 6 0 000-12zm0 2a4 4 0 110 8 4 4 0 010-8z" }) }),
    color: "#3A5EE9"
  },
  {
    id: "betterstack",
    name: "BetterStack",
    description: "Unified incident management with uptime monitoring and on-call scheduling.",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { viewBox: "0 0 24 24", className: "w-8 h-8", fill: "currentColor", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5", strokeWidth: "2", stroke: "currentColor", fill: "none", strokeLinecap: "round", strokeLinejoin: "round" }) }),
    color: "#2DD4BF"
  }
];
const IntegrationCard = ({ integration }) => {
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "div",
    {
      className: "relative p-6 bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)]\n                 hover:border-[var(--color-border-default)] transition-all cursor-pointer group",
      children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(
          "div",
          {
            className: "w-14 h-14 rounded-xl flex items-center justify-center mb-4 transition-transform group-hover:scale-105",
            style: { backgroundColor: `${integration.color}20`, color: integration.color },
            children: integration.icon
          }
        ),
        /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-lg font-semibold text-[var(--color-text-primary)] mb-2", children: integration.name }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)] leading-relaxed", children: integration.description }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mt-4 inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)]", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "w-2 h-2 rounded-full bg-amber-500" }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "text-xs text-[var(--color-text-muted)]", children: "Coming Soon" })
        ] })
      ]
    }
  );
};
function IntegrationsPage() {
  return /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "h-full overflow-y-auto bg-[var(--color-app-bg)]", children: /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "max-w-6xl mx-auto p-8", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mb-8", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("h1", { className: "text-2xl font-bold text-[var(--color-text-primary)] mb-2", children: "Integrations" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-[var(--color-text-muted)] max-w-2xl", children: "Connect Spectre with your existing tools to streamline incident response and enable seamless collaboration across your team." })
    ] }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6", children: INTEGRATIONS.map((integration) => /* @__PURE__ */ jsxRuntimeExports.jsx(IntegrationCard, { integration }, integration.id)) }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "mt-12 p-6 bg-[var(--color-surface-muted)] rounded-xl border border-[var(--color-border-soft)] text-center", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("h3", { className: "text-lg font-semibold text-[var(--color-text-primary)] mb-2", children: "Missing an integration?" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("p", { className: "text-sm text-[var(--color-text-muted)] mb-4", children: "Let us know which tools you'd like to see integrated with Spectre." }),
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        "button",
        {
          disabled: true,
          className: "px-6 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium\n                       opacity-50 cursor-not-allowed",
          children: "Request Integration"
        }
      )
    ] })
  ] }) });
}

const navItems = [
  {
    path: "/",
    label: "Timeline",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { width: "20", height: "20", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M2 12h3l2-4 3 8 4-12 5 6 3-2h3" }) })
  },
  {
    path: "/graph",
    label: "Graph",
    icon: /* @__PURE__ */ jsxRuntimeExports.jsxs("svg", { xmlns: "http://www.w3.org/2000/svg", width: "20", height: "20", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", "aria-hidden": "true", children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx("circle", { cx: "5", cy: "6", r: "3" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M5 9v6" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("circle", { cx: "5", cy: "18", r: "3" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12 3v18" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("circle", { cx: "19", cy: "6", r: "3" }),
      /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M16 15.7A9 9 0 0 0 19 9" })
    ] })
  },
  {
    path: "/integrations",
    label: "Integrations",
    icon: (
      // Puzzle piece / plug icon for integrations
      /* @__PURE__ */ jsxRuntimeExports.jsx("svg", { width: "20", height: "20", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M19.439 7.85c-.049.322.059.648.289.878l1.568 1.568c.47.47.706 1.087.706 1.704s-.235 1.233-.706 1.704l-1.611 1.611a.98.98 0 0 1-.837.276c-.47-.07-.802-.48-.968-.925a2.501 2.501 0 1 0-3.214 3.214c.446.166.855.497.925.968a.979.979 0 0 1-.276.837l-1.61 1.61a2.404 2.404 0 0 1-1.705.707 2.402 2.402 0 0 1-1.704-.706l-1.568-1.568a1.026 1.026 0 0 0-.877-.29c-.493.074-.84.504-1.02.968a2.5 2.5 0 1 1-3.237-3.237c.464-.18.894-.527.967-1.02a1.026 1.026 0 0 0-.289-.877l-1.568-1.568A2.402 2.402 0 0 1 1.998 12c0-.617.236-1.234.706-1.704L4.23 8.77c.24-.24.581-.353.917-.303.515.077.877.528 1.073 1.01a2.5 2.5 0 1 0 3.259-3.259c-.482-.196-.933-.558-1.01-1.073-.05-.336.062-.676.303-.917l1.525-1.525A2.402 2.402 0 0 1 12 1.998c.617 0 1.234.236 1.704.706l1.568 1.568c.23.23.556.338.877.29.493-.074.84-.504 1.02-.968a2.5 2.5 0 1 1 3.237 3.237c-.464.18-.894.527-.967 1.02Z" }) })
    )
  }
];
const settingsItem = {
  path: "/settings",
  label: "Settings",
  icon: /* @__PURE__ */ jsxRuntimeExports.jsxs("svg", { width: "20", height: "20", viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round", children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" }),
    /* @__PURE__ */ jsxRuntimeExports.jsx("circle", { cx: "12", cy: "12", r: "3" })
  ] })
};
const sidebarCSS = `
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

  .sidebar-container {
    width: 64px;
    min-width: 64px;
    height: 100vh;
    background-color: #0d0d0d;
    border-right: 1px solid #2a2a2a;
    display: flex;
    flex-direction: column;
    padding: 0 0 16px 0;
    transition: width 0.25s cubic-bezier(0.4, 0, 0.2, 1), min-width 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    overflow: hidden;
    position: fixed;
    top: 0;
    left: 0;
    z-index: 50;
  }

  .sidebar-container:hover {
    width: 220px;
    min-width: 220px;
  }

  .sidebar-logo-container {
    padding: 16px 12px 14px;
    border-bottom: 1px solid #2a2a2a;
    margin-bottom: 16px;
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 73px;
  }

  .sidebar-logo-icon {
    width: 40px;
    height: 40px;
    min-width: 40px;
    flex-shrink: 0;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    box-shadow: 0 10px 15px -3px rgba(99, 102, 241, 0.2);
    background: linear-gradient(135deg, #6366f1 0%, #8b5cf6 25%, #a855f7 50%, #7c3aed 75%, #6366f1 100%);
    background-size: 200% 200%;
    animation: gradient-shift 15s ease-in-out infinite;
    transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .sidebar-logo-text {
    color: #ffffff;
    font-size: 20px;
    font-weight: 700;
    letter-spacing: 0.5px;
    margin: 0;
    white-space: nowrap;
    opacity: 0;
    transform: translateX(-10px);
    transition: opacity 0.2s ease 0.05s, transform 0.2s ease 0.05s;
  }

  .sidebar-container:hover .sidebar-logo-text {
    opacity: 1;
    transform: translateX(0);
  }

  .sidebar-nav {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0;
    flex: 1;
  }

  .sidebar-nav-settings {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 0;
    margin-top: auto;
  }

  .sidebar-link {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 22px;
    border-radius: 0;
    text-decoration: none;
    color: #a0a0a0;
    font-size: 14px;
    font-weight: 500;
    transition: background-color 0.15s ease, color 0.15s ease;
    position: relative;
    width: 100%;
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
  }

  .sidebar-link:hover {
    background-color: #161616;
    color: #ffffff;
  }

  .sidebar-link.active {
    background-color: #1a1a1a;
    color: #a855f7;
    border-right: 3px solid #a855f7;
  }

  .sidebar-link.active:hover {
    color: #a855f7;
  }

  .sidebar-link-icon {
    display: flex;
    align-items: center;
    min-width: 20px;
    transition: color 0.15s ease;
  }

  .sidebar-link.active .sidebar-link-icon {
    color: #a855f7;
  }

  .sidebar-link-label {
    opacity: 0;
    transform: translateX(-10px);
    transition: opacity 0.2s ease 0.05s, transform 0.2s ease 0.05s;
  }

  .sidebar-container:hover .sidebar-link-label {
    opacity: 1;
    transform: translateX(0);
  }
`;
function Sidebar({ onHoverChange }) {
  return /* @__PURE__ */ jsxRuntimeExports.jsxs(
    "aside",
    {
      className: "sidebar-container",
      onMouseEnter: () => onHoverChange?.(true),
      onMouseLeave: () => onHoverChange?.(false),
      children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx("style", { children: sidebarCSS }),
        /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { className: "sidebar-logo-container", children: [
          /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "sidebar-logo-icon", children: /* @__PURE__ */ jsxRuntimeExports.jsxs(
            "svg",
            {
              width: "24",
              height: "24",
              viewBox: "0 0 24 24",
              fill: "none",
              xmlns: "http://www.w3.org/2000/svg",
              style: { animation: "levitate 10s ease-in-out infinite" },
              children: [
                /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M12 2C7.58172 2 4 5.58172 4 10V19C4 20.6569 5.34315 22 7 22C7.63228 22 8.21952 21.7909 8.70773 21.4312L10.5858 19.5531C10.9609 19.1781 11.4696 18.9674 12 18.9674C12.5304 18.9674 13.0391 19.1781 13.4142 19.5531L15.2923 21.4312C15.7805 21.7909 16.3677 22 17 22C18.6569 22 20 20.6569 20 19V10C20 5.58172 16.4183 2 12 2Z", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M9 10H9.01", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round" }),
                /* @__PURE__ */ jsxRuntimeExports.jsx("path", { d: "M15 10H15.01", stroke: "currentColor", strokeWidth: "2", strokeLinecap: "round", strokeLinejoin: "round" })
              ]
            }
          ) }),
          /* @__PURE__ */ jsxRuntimeExports.jsx("h1", { className: "sidebar-logo-text", children: "Spectre" })
        ] }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("nav", { className: "sidebar-nav", children: navItems.map((item) => /* @__PURE__ */ jsxRuntimeExports.jsxs(
          NavLink,
          {
            to: item.path,
            className: ({ isActive }) => `sidebar-link ${isActive ? "active" : ""}`,
            children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "sidebar-link-icon", children: item.icon }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "sidebar-link-label", children: item.label })
            ]
          },
          item.path
        )) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx("nav", { className: "sidebar-nav-settings", children: /* @__PURE__ */ jsxRuntimeExports.jsxs(
          NavLink,
          {
            to: settingsItem.path,
            className: ({ isActive }) => `sidebar-link ${isActive ? "active" : ""}`,
            children: [
              /* @__PURE__ */ jsxRuntimeExports.jsx("div", { className: "sidebar-link-icon", children: settingsItem.icon }),
              /* @__PURE__ */ jsxRuntimeExports.jsx("span", { className: "sidebar-link-label", children: settingsItem.label })
            ]
          }
        ) })
      ]
    }
  );
}

const appContainerStyles = {
  display: "flex",
  height: "100vh",
  backgroundColor: "#111111",
  overflow: "hidden"
};
function App() {
  const [sidebarExpanded, setSidebarExpanded] = reactExports.useState(false);
  const mainContentStyles = {
    flex: 1,
    height: "100vh",
    overflow: "hidden",
    marginLeft: sidebarExpanded ? "220px" : "64px",
    transition: "margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1)"
  };
  return /* @__PURE__ */ jsxRuntimeExports.jsxs("div", { style: appContainerStyles, children: [
    /* @__PURE__ */ jsxRuntimeExports.jsx(Sidebar, { onHoverChange: setSidebarExpanded }),
    /* @__PURE__ */ jsxRuntimeExports.jsxs("main", { style: mainContentStyles, children: [
      /* @__PURE__ */ jsxRuntimeExports.jsx(
        Toaster,
        {
          position: "top-right",
          theme: "dark",
          richColors: true,
          closeButton: true,
          expand: false,
          duration: 5e3
        }
      ),
      /* @__PURE__ */ jsxRuntimeExports.jsxs(Routes, { children: [
        /* @__PURE__ */ jsxRuntimeExports.jsx(Route, { path: "/", element: /* @__PURE__ */ jsxRuntimeExports.jsx(TimelinePage, {}) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(Route, { path: "/graph", element: /* @__PURE__ */ jsxRuntimeExports.jsx(NamespaceGraphPage, {}) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(Route, { path: "/agents", element: /* @__PURE__ */ jsxRuntimeExports.jsx(AgentsPage, {}) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(Route, { path: "/integrations", element: /* @__PURE__ */ jsxRuntimeExports.jsx(IntegrationsPage, {}) }),
        /* @__PURE__ */ jsxRuntimeExports.jsx(Route, { path: "/settings", element: /* @__PURE__ */ jsxRuntimeExports.jsx(SettingsPage, {}) })
      ] })
    ] })
  ] });
}

export { App as default };
//# sourceMappingURL=App-CrLVUZD2.js.map
