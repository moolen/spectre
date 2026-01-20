import React from 'react';
import { D3GraphNode, NODE_STATUS_COLORS } from '../../types/namespaceGraph';
import { Anomaly } from '../../types/rootCause';

interface NodeDetailPanelProps {
  /** Selected node to display */
  node: D3GraphNode;
  /** Anomalies for this node */
  anomalies?: Anomaly[];
  /** Callback to close the panel */
  onClose: () => void;
}

/**
 * Severity badge colors
 */
const SEVERITY_COLORS: Record<string, { bg: string; text: string }> = {
  critical: { bg: 'bg-red-600', text: 'text-white' },
  high: { bg: 'bg-red-500', text: 'text-white' },
  medium: { bg: 'bg-amber-500', text: 'text-white' },
  low: { bg: 'bg-blue-500', text: 'text-white' },
};

/**
 * Detail panel showing information about a selected node
 */
export const NodeDetailPanel: React.FC<NodeDetailPanelProps> = ({
  node,
  anomalies = [],
  onClose,
}) => {
  const statusColor = NODE_STATUS_COLORS[node.status] || NODE_STATUS_COLORS.Unknown;

  // Format timestamp
  const formatTimestamp = (nanos: number): string => {
    const date = new Date(nanos / 1_000_000);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  };

  return (
    <div className="w-80 h-full border-l border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border-soft)]">
        <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">Resource Details</h3>
        <button
          onClick={onClose}
          className="p-1 rounded hover:bg-[var(--color-surface-elevated)] text-[var(--color-text-muted)]
                     hover:text-[var(--color-text-primary)] transition-colors"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {/* Resource info */}
        <div className="space-y-3">
          {/* Kind and Status */}
          <div className="flex items-center gap-2">
            <span className="px-2 py-0.5 rounded bg-[var(--color-surface-elevated)] text-xs font-medium text-[var(--color-text-muted)]">
              {node.kind}
            </span>
            <div className="flex items-center gap-1.5">
              <div 
                className="w-2.5 h-2.5 rounded-full" 
                style={{ backgroundColor: statusColor }}
              />
              <span className="text-xs text-[var(--color-text-muted)]">{node.status}</span>
            </div>
          </div>

          {/* Name */}
          <div>
            <div className="text-xs text-[var(--color-text-muted)] mb-1">Name</div>
            <div className="text-sm font-mono text-[var(--color-text-primary)] break-all">
              {node.name}
            </div>
          </div>

          {/* Namespace */}
          <div>
            <div className="text-xs text-[var(--color-text-muted)] mb-1">Namespace</div>
            <div className="text-sm text-[var(--color-text-primary)]">
              {node.isClusterScoped ? (
                <span className="italic text-[var(--color-text-muted)]">Cluster-scoped</span>
              ) : (
                node.namespace
              )}
            </div>
          </div>

          {/* API Group */}
          {node.apiGroup && (
            <div>
              <div className="text-xs text-[var(--color-text-muted)] mb-1">API Group</div>
              <div className="text-sm font-mono text-[var(--color-text-primary)]">
                {node.apiGroup}
              </div>
            </div>
          )}

          {/* UID */}
          <div>
            <div className="text-xs text-[var(--color-text-muted)] mb-1">UID</div>
            <div className="text-xs font-mono text-[var(--color-text-muted)] break-all">
              {node.uid}
            </div>
          </div>
        </div>

        {/* Labels */}
        {node.labels && Object.keys(node.labels).length > 0 && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="text-xs text-[var(--color-text-muted)] mb-2">Labels</div>
            <div className="space-y-1">
              {Object.entries(node.labels).slice(0, 8).map(([key, value]) => (
                <div key={key} className="flex items-start gap-1 text-xs">
                  <span className="text-[var(--color-text-muted)] flex-shrink-0">{key}:</span>
                  <span className="font-mono text-[var(--color-text-primary)] break-all">{value}</span>
                </div>
              ))}
              {Object.keys(node.labels).length > 8 && (
                <div className="text-xs text-[var(--color-text-muted)] italic">
                  +{Object.keys(node.labels).length - 8} more
                </div>
              )}
            </div>
          </div>
        )}

        {/* Latest Event */}
        {node.latestEvent && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="text-xs text-[var(--color-text-muted)] mb-2">Latest Event</div>
            <div className="p-2 rounded bg-[var(--color-surface-elevated)] space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-[var(--color-text-primary)]">
                  {node.latestEvent.eventType}
                </span>
                <span className="text-xs text-[var(--color-text-muted)]">
                  {formatTimestamp(node.latestEvent.timestamp)}
                </span>
              </div>
              {node.latestEvent.description && (
                <div className="text-xs text-[var(--color-text-muted)]">
                  {node.latestEvent.description}
                </div>
              )}
              {/* Error Message (truncated with hover) */}
              {node.latestEvent.errorMessage && (
                <div className="group relative">
                  <div className="text-xs text-red-500 cursor-help">
                    {node.latestEvent.errorMessage.length > 100
                      ? `${node.latestEvent.errorMessage.slice(-100)}...`
                      : node.latestEvent.errorMessage}
                  </div>
                  {node.latestEvent.errorMessage.length > 100 && (
                    <div className="absolute z-50 hidden group-hover:block bottom-full left-0 mb-1 p-2
                                    bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)]
                                    rounded shadow-lg max-w-md max-h-48 overflow-auto">
                      <div className="text-xs text-red-500 whitespace-pre-wrap break-words">
                        {node.latestEvent.errorMessage}
                      </div>
                    </div>
                  )}
                </div>
              )}
              {/* Container Issues */}
              {node.latestEvent.containerIssues && node.latestEvent.containerIssues.length > 0 && (
                <div className="flex flex-wrap gap-1 mt-1">
                  {node.latestEvent.containerIssues.map((issue, idx) => (
                    <span
                      key={idx}
                      className="px-1.5 py-0.5 rounded text-xs bg-amber-500/20 text-amber-500"
                    >
                      {issue}
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}

        {/* Spec Changes */}
        {node.latestEvent?.specChanges && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="text-xs text-[var(--color-text-muted)] mb-2">Spec Changes (within lookback)</div>
            <div className="p-2 rounded bg-[var(--color-surface-elevated)] overflow-x-auto">
              <pre className="text-xs font-mono whitespace-pre-wrap break-words">
                {node.latestEvent.specChanges.split('\n').map((line, idx) => {
                  let lineClass = 'text-[var(--color-text-muted)]';
                  if (line.startsWith('+')) lineClass = 'text-green-500';
                  else if (line.startsWith('-')) lineClass = 'text-red-500';
                  else if (line.startsWith('@@')) lineClass = 'text-blue-400 font-semibold';
                  return (
                    <span key={idx} className={lineClass}>
                      {line}
                      {idx < node.latestEvent!.specChanges!.split('\n').length - 1 ? '\n' : ''}
                    </span>
                  );
                })}
              </pre>
            </div>
          </div>
        )}

        {/* Anomalies */}
        {anomalies.length > 0 && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="flex items-center gap-2 mb-2">
              <div className="text-xs text-[var(--color-text-muted)]">Anomalies</div>
              <div className="px-1.5 py-0.5 rounded-full bg-red-500 text-white text-xs font-medium">
                {anomalies.length}
              </div>
            </div>
            <div className="space-y-2">
              {anomalies.map((anomaly, idx) => {
                const severityStyle = SEVERITY_COLORS[anomaly.severity] || SEVERITY_COLORS.low;
                const hasDetails = anomaly.details && Object.keys(anomaly.details).length > 0;
                return (
                  <div
                    key={idx}
                    className="p-2 rounded bg-[var(--color-surface-elevated)] space-y-2"
                  >
                    <div className="flex items-center gap-2">
                      <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${severityStyle.bg} ${severityStyle.text}`}>
                        {anomaly.severity}
                      </span>
                      <span className="text-xs text-[var(--color-text-muted)] capitalize">
                        {anomaly.category}
                      </span>
                    </div>
                    <div className="text-xs font-medium text-[var(--color-text-primary)]">
                      {anomaly.type}
                    </div>
                    <div className="text-xs text-[var(--color-text-muted)]">
                      {anomaly.summary}
                    </div>
                    {/* Anomaly Details */}
                    {hasDetails && (
                      <div className="pt-2 mt-2 border-t border-[var(--color-border-soft)] space-y-1">
                        {Object.entries(anomaly.details).map(([key, value]) => (
                          <div key={key} className="flex items-start gap-1 text-xs">
                            <span className="text-[var(--color-text-muted)] flex-shrink-0">{key}:</span>
                            <span className="font-mono text-[var(--color-text-primary)] break-all">
                              {typeof value === 'object' ? JSON.stringify(value) : String(value)}
                            </span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default NodeDetailPanel;
