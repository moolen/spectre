import React, { useState } from 'react';
import { ObservatoryNodeType, NODE_TYPE_COLORS, NODE_TYPE_ICONS } from '../../types/observatoryGraph';

const NODE_TYPES: ObservatoryNodeType[] = [
  'SignalAnchor',
  'Alert',
  'Dashboard',
  'Panel',
  'Query',
  'Metric',
  'Service',
  'Workload',
  'SignalBaseline',
];

interface ObservatoryLegendProps {
  className?: string;
}

/**
 * Collapsible legend showing node type colors and icons
 */
export function ObservatoryLegend({ className }: ObservatoryLegendProps) {
  const [expanded, setExpanded] = useState(false);

  if (!expanded) {
    return (
      <button
        onClick={() => setExpanded(true)}
        className={`bg-[#1a1a1a] hover:bg-[#222222] rounded-lg p-2 border border-[#2a2a2a] transition-colors ${className || ''}`}
        title="Show legend"
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className="text-gray-400">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 16v-4" />
          <path d="M12 8h.01" />
        </svg>
      </button>
    );
  }

  return (
    <div className={`bg-[#1a1a1a] rounded-lg p-3 border border-[#2a2a2a] ${className || ''}`}>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Legend</h3>
        <button
          onClick={() => setExpanded(false)}
          className="text-gray-500 hover:text-gray-300 transition-colors"
          title="Hide legend"
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M18 6L6 18" />
            <path d="M6 6l12 12" />
          </svg>
        </button>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {NODE_TYPES.map(type => (
          <div key={type} className="flex items-center gap-2">
            <div
              className="w-4 h-4 rounded-full shrink-0"
              style={{ backgroundColor: NODE_TYPE_COLORS[type] }}
            />
            <span className="text-xs text-gray-400 truncate" title={type}>
              {NODE_TYPE_ICONS[type]} {type}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default ObservatoryLegend;
