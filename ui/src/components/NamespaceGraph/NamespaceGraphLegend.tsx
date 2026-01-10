import React from 'react';
import { NODE_STATUS_COLORS, RELATIONSHIP_LABELS } from '../../types/namespaceGraph';

interface NamespaceGraphLegendProps {
  /** Whether to show relationship types */
  showRelationships?: boolean;
}

/**
 * Legend component for the namespace graph
 * Shows status colors, edge types, and special node styling
 */
export const NamespaceGraphLegend: React.FC<NamespaceGraphLegendProps> = ({
  showRelationships = true,
}) => {
  const statusItems = [
    { status: 'Ready', color: NODE_STATUS_COLORS.Ready, label: 'Ready' },
    { status: 'Warning', color: NODE_STATUS_COLORS.Warning, label: 'Warning' },
    { status: 'Error', color: NODE_STATUS_COLORS.Error, label: 'Error' },
    { status: 'Terminating', color: NODE_STATUS_COLORS.Terminating, label: 'Terminating' },
    { status: 'Unknown', color: NODE_STATUS_COLORS.Unknown, label: 'Unknown' },
  ];

  const relationshipItems = Object.entries(RELATIONSHIP_LABELS).slice(0, 4);

  return (
    <div className="absolute bottom-4 left-4 p-3 rounded-lg bg-[var(--color-surface-muted)] 
                    border border-[var(--color-border-soft)] shadow-lg">
      <div className="text-xs font-semibold text-[var(--color-text-muted)] mb-2 uppercase tracking-wide">
        Legend
      </div>
      
      {/* Status colors */}
      <div className="flex flex-wrap gap-3 mb-3">
        {statusItems.map(({ status, color, label }) => (
          <div key={status} className="flex items-center gap-1.5">
            <div 
              className="w-3 h-3 rounded-full" 
              style={{ 
                backgroundColor: color,
                boxShadow: status === 'Error' ? `0 0 6px ${color}` : 'none'
              }}
            />
            <span className="text-xs text-[var(--color-text-muted)]">{label}</span>
          </div>
        ))}
      </div>

      {/* Special node styles */}
      <div className="flex flex-wrap gap-3 mb-3 pt-2 border-t border-[var(--color-border-soft)]">
        <div className="flex items-center gap-1.5">
          <div 
            className="w-3 h-3 rounded-full border-2 border-dashed"
            style={{ borderColor: '#9ca3af' }}
          />
          <span className="text-xs text-[var(--color-text-muted)]">Cluster-scoped</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="relative">
            <div className="w-3 h-3 rounded-full bg-gray-500" />
            <div 
              className="absolute -top-1 -right-1 w-2 h-2 rounded-full flex items-center justify-center text-white"
              style={{ backgroundColor: '#ef4444', fontSize: '6px' }}
            >
              !
            </div>
          </div>
          <span className="text-xs text-[var(--color-text-muted)]">Has Anomalies</span>
        </div>
      </div>

      {/* Relationship types */}
      {showRelationships && (
        <div className="pt-2 border-t border-[var(--color-border-soft)]">
          <div className="flex flex-wrap gap-3">
            {relationshipItems.map(([type, label]) => (
              <div key={type} className="flex items-center gap-1.5">
                <svg width="20" height="8" viewBox="0 0 20 8">
                  <line 
                    x1="0" y1="4" x2="14" y2="4" 
                    stroke="#6b7280" 
                    strokeWidth="1.5"
                  />
                  <polygon 
                    points="14,1 20,4 14,7" 
                    fill="#6b7280"
                  />
                </svg>
                <span className="text-xs text-[var(--color-text-muted)]">{label}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Interaction hints */}
      <div className="mt-3 pt-2 border-t border-[var(--color-border-soft)] text-[10px] text-[var(--color-text-muted)]">
        <div className="mb-1">
          <span className="opacity-70">Drag nodes</span>
          <span className="mx-2 opacity-40">|</span>
          <span className="opacity-70">Scroll to zoom</span>
          <span className="mx-2 opacity-40">|</span>
          <span className="opacity-70">Click node for details</span>
        </div>
        <div className="opacity-60">
          <span className="font-mono bg-[var(--color-surface-elevated)] px-1 rounded">Esc</span>
          <span className="mx-1">back</span>
          <span className="mx-1 opacity-40">|</span>
          <span className="font-mono bg-[var(--color-surface-elevated)] px-1 rounded">R</span>
          <span className="mx-1">refresh</span>
          <span className="mx-1 opacity-40">|</span>
          <span className="font-mono bg-[var(--color-surface-elevated)] px-1 rounded">F</span>
          <span className="mx-1">fit</span>
          <span className="mx-1 opacity-40">|</span>
          <span className="font-mono bg-[var(--color-surface-elevated)] px-1 rounded">+/-</span>
          <span className="mx-1">zoom</span>
        </div>
      </div>
    </div>
  );
};

export default NamespaceGraphLegend;
