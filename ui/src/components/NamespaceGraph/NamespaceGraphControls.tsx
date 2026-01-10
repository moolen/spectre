import React from 'react';
import { SelectDropdown } from '../SelectDropdown';
import { getDemoMode } from '../../services/api';

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
      {/* Demo Mode Indicator */}
      {getDemoMode() && (
        <div className="px-3 py-1 rounded-full bg-amber-500/20 border border-amber-500/40 flex items-center gap-2 min-w-max">
          <div className="w-2 h-2 rounded-full bg-amber-400 animate-pulse"></div>
          <span className="text-xs font-semibold text-amber-300">Demo Mode</span>
        </div>
      )}

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
    </div>
  );
};

export default NamespaceGraphControls;
