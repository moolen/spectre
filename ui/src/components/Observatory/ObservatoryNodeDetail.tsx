import React from 'react';
import { D3ObservatoryNode, NODE_TYPE_COLORS, NODE_TYPE_ICONS } from '../../types/observatoryGraph';

interface ObservatoryNodeDetailProps {
  node: D3ObservatoryNode;
  onClose: () => void;
}

/**
 * Detail panel showing properties of a selected node
 */
export function ObservatoryNodeDetail({ node, onClose }: ObservatoryNodeDetailProps) {
  const color = NODE_TYPE_COLORS[node.type] || '#6b7280';
  const icon = NODE_TYPE_ICONS[node.type] || '📦';

  return (
    <div className="w-80 bg-[#1a1a1a] border-l border-[#2a2a2a] h-full overflow-hidden flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-[#2a2a2a] flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div
            className="w-10 h-10 rounded-lg flex items-center justify-center text-lg"
            style={{ backgroundColor: color + '20', color }}
          >
            {icon}
          </div>
          <div>
            <div className="text-xs font-medium text-gray-400 uppercase">{node.type}</div>
            <div className="text-sm font-semibold text-white truncate max-w-[180px]" title={node.label}>
              {node.label}
            </div>
          </div>
        </div>
        <button
          onClick={onClose}
          className="text-gray-500 hover:text-white transition-colors"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <line x1="18" y1="6" x2="6" y2="18" />
            <line x1="6" y1="6" x2="18" y2="18" />
          </svg>
        </button>
      </div>

      {/* Properties */}
      <div className="flex-1 overflow-y-auto p-4">
        <div className="space-y-3">
          <PropertySection title="Identity">
            <PropertyRow label="ID" value={node.id} />
            <PropertyRow label="Type" value={node.type} />
            <PropertyRow label="Label" value={node.label} />
          </PropertySection>

          {node.properties && Object.keys(node.properties).length > 0 && (
            <PropertySection title="Properties">
              {Object.entries(node.properties).map(([key, value]) => (
                <PropertyRow
                  key={key}
                  label={formatPropertyLabel(key)}
                  value={formatPropertyValue(value)}
                />
              ))}
            </PropertySection>
          )}
        </div>
      </div>
    </div>
  );
}

interface PropertySectionProps {
  title: string;
  children: React.ReactNode;
}

function PropertySection({ title, children }: PropertySectionProps) {
  return (
    <div className="space-y-2">
      <h3 className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{title}</h3>
      <div className="space-y-1.5">{children}</div>
    </div>
  );
}

interface PropertyRowProps {
  label: string;
  value: string | number | undefined;
}

function PropertyRow({ label, value }: PropertyRowProps) {
  if (value === undefined || value === null || value === '') return null;

  return (
    <div className="flex justify-between items-start text-sm">
      <span className="text-gray-500 shrink-0">{label}</span>
      <span className="text-gray-300 text-right ml-2 break-all max-w-[180px]" title={String(value)}>
        {String(value).length > 50 ? String(value).slice(0, 50) + '...' : String(value)}
      </span>
    </div>
  );
}

function formatPropertyLabel(key: string): string {
  // Convert camelCase to Title Case
  return key
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, str => str.toUpperCase())
    .trim();
}

function formatPropertyValue(value: any): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  if (typeof value === 'number') {
    if (Number.isInteger(value)) return value.toString();
    return value.toFixed(3);
  }
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}

export default ObservatoryNodeDetail;
