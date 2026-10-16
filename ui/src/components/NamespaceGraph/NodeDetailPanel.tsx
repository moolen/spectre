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
  critical: { bg: 'bg-[var(--color-status-err-soft)]', text: 'text-[var(--color-status-err-text)]' },
  high: { bg: 'bg-[var(--color-status-err-soft)]', text: 'text-[var(--color-status-err-text)]' },
  medium: { bg: 'bg-[var(--color-status-warn-soft)]', text: 'text-[var(--color-status-warn-text)]' },
  low: { bg: 'bg-[var(--color-surface-active)]', text: 'text-[var(--color-text-secondary)]' },
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
    <aside className="w-[340px] min-w-[340px] h-full border-l border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] flex flex-col text-[var(--color-text-primary)]">
      {/* Header */}
      <div className="flex items-center justify-between px-5 h-12 border-b border-[var(--color-border-soft)]">
        <h3 className="text-[13.5px] font-semibold">Resource details</h3>
        <button
          onClick={onClose}
          aria-label="Close details"
          className="w-[26px] h-[26px] grid place-items-center rounded-md text-[var(--color-text-faint)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors"
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.75} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-5 py-4 space-y-4">
        {/* Resource info */}
        <div className="space-y-3">
          {/* Kind and Status */}
          <div className="flex items-center gap-2">
            <span className="px-2 py-0.5 rounded-md bg-[var(--color-surface-active)] text-xs text-[var(--color-text-secondary)]">
              {node.kind}
            </span>
            <div className="flex items-center gap-1.5">
              <div
                className="w-1.5 h-1.5 rounded-full" 
                style={{ backgroundColor: statusColor }}
              />
              <span className="text-xs text-[var(--color-text-faint)]">{node.status}</span>
            </div>
          </div>

          {/* Name */}
          <div>
            <div className="text-xs text-[var(--color-text-faint)] mb-1">Name</div>
            <div className="text-sm font-mono text-[var(--color-text-primary)] break-all">
              {node.name}
            </div>
          </div>

          {/* Namespace */}
          <div>
            <div className="text-xs text-[var(--color-text-faint)] mb-1">Namespace</div>
            <div className="text-sm text-[var(--color-text-primary)]">
              {node.isClusterScoped ? (
                <span className="italic text-[var(--color-text-faint)]">Cluster-scoped</span>
              ) : (
                node.namespace
              )}
            </div>
          </div>

          {/* API Group */}
          {node.apiGroup && (
            <div>
              <div className="text-xs text-[var(--color-text-faint)] mb-1">API Group</div>
              <div className="text-sm font-mono text-[var(--color-text-primary)]">
                {node.apiGroup}
              </div>
            </div>
          )}

          {/* UID */}
          <div>
            <div className="text-xs text-[var(--color-text-faint)] mb-1">UID</div>
            <div className="text-xs font-mono text-[var(--color-text-faint)] break-all">
              {node.uid}
            </div>
          </div>
        </div>

        {/* Labels */}
        {node.labels && Object.keys(node.labels).length > 0 && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="text-xs text-[var(--color-text-faint)] mb-2">Labels</div>
            <div className="space-y-1">
              {Object.entries(node.labels).slice(0, 8).map(([key, value]) => (
                <div key={key} className="flex items-start gap-1 text-xs">
                  <span className="text-[var(--color-text-faint)] flex-shrink-0">{key}:</span>
                  <span className="font-mono text-[var(--color-text-primary)] break-all">{value}</span>
                </div>
              ))}
              {Object.keys(node.labels).length > 8 && (
                <div className="text-xs text-[var(--color-text-faint)] italic">
                  +{Object.keys(node.labels).length - 8} more
                </div>
              )}
            </div>
          </div>
        )}

        {/* Latest Event */}
        {node.latestEvent && (
          <div className="pt-3 border-t border-[var(--color-border-soft)]">
            <div className="text-xs text-[var(--color-text-faint)] mb-2">Latest Event</div>
            <div className="p-2 rounded-lg bg-[var(--color-app-bg)] border border-[var(--color-border-soft)] space-y-1">
              <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-[var(--color-text-primary)]">
                  {node.latestEvent.eventType}
                </span>
                <span className="text-xs text-[var(--color-text-faint)]">
                  {formatTimestamp(node.latestEvent.timestamp)}
                </span>
              </div>
              {node.latestEvent.description && (
                <div className="text-xs text-[var(--color-text-faint)]">
                  {node.latestEvent.description}
                </div>
              )}
              {/* Error Message (truncated with hover) */}
              {node.latestEvent.errorMessage && (
                <div className="group relative">
                  <div className="text-xs text-[var(--color-status-err-text)] cursor-help">
                    {node.latestEvent.errorMessage.length > 100
                      ? `${node.latestEvent.errorMessage.slice(-100)}...`
                      : node.latestEvent.errorMessage}
                  </div>
                  {node.latestEvent.errorMessage.length > 100 && (
                    <div className="ui-popover absolute z-50 hidden group-hover:block bottom-full left-0 mb-1 p-2.5 max-w-md max-h-48 overflow-auto">
                      <div className="text-xs text-[var(--color-status-err-text)] whitespace-pre-wrap break-words">
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
                      className="px-1.5 py-0.5 rounded text-xs bg-[var(--color-status-warn-soft)] text-[var(--color-status-warn-text)]"
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
            <div className="text-xs text-[var(--color-text-faint)] mb-2">Spec Changes (within lookback)</div>
            <div className="p-2 rounded-lg bg-[var(--color-app-bg)] border border-[var(--color-border-soft)] overflow-x-auto">
              <pre className="text-xs font-mono whitespace-pre-wrap break-words">
                {node.latestEvent.specChanges.split('\n').map((line, idx) => {
                  let lineClass = 'text-[var(--color-text-faint)]';
                  if (line.startsWith('+')) lineClass = 'text-[var(--color-status-ok-text)]';
                  else if (line.startsWith('-')) lineClass = 'text-[var(--color-status-err-text)]';
                  else if (line.startsWith('@@')) lineClass = 'text-[var(--color-accent-strong)]';
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
              <div className="text-xs text-[var(--color-text-faint)]">Anomalies</div>
              <div className="px-1.5 rounded-md bg-[var(--color-status-err-soft)] text-[var(--color-status-err-text)] text-[11px] font-medium tabular-nums">
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
                    className="p-2 rounded-lg bg-[var(--color-app-bg)] border border-[var(--color-border-soft)] space-y-2"
                  >
                    <div className="flex items-center gap-2">
                      <span className={`px-1.5 py-0.5 rounded text-xs font-medium ${severityStyle.bg} ${severityStyle.text}`}>
                        {anomaly.severity}
                      </span>
                      <span className="text-xs text-[var(--color-text-faint)] capitalize">
                        {anomaly.category}
                      </span>
                    </div>
                    <div className="text-xs font-medium text-[var(--color-text-primary)]">
                      {anomaly.type}
                    </div>
                    <div className="text-xs text-[var(--color-text-faint)]">
                      {anomaly.summary}
                    </div>
                    {/* Anomaly Details */}
                    {hasDetails && (
                      <div className="pt-2 mt-2 border-t border-[var(--color-border-soft)] space-y-1">
                        {Object.entries(anomaly.details).map(([key, value]) => (
                          <div key={key} className="flex items-start gap-1 text-xs">
                            <span className="text-[var(--color-text-faint)] flex-shrink-0">{key}:</span>
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
    </aside>
  );
};

export default NodeDetailPanel;
