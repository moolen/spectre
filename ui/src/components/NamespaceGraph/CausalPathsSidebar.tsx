import React from 'react';
import { CausalPath } from '../../types/namespaceGraph';

interface CausalPathsSidebarProps {
  /** List of causal paths to display */
  causalPaths: CausalPath[];
  /** Currently selected path ID */
  selectedPathId: string | null;
  /** Callback when a path is selected/deselected */
  onPathSelect: (pathId: string | null) => void;
  /** Loading state */
  isLoading?: boolean;
}

/**
 * Sidebar component displaying causal paths for the namespace graph.
 * When a path is selected, it highlights the corresponding nodes and edges on the graph.
 */
export const CausalPathsSidebar: React.FC<CausalPathsSidebarProps> = ({
  causalPaths,
  selectedPathId,
  onPathSelect,
  isLoading = false,
}) => {
  // Sort paths by confidence score (highest first)
  const sortedPaths = React.useMemo(
    () => [...causalPaths].sort((a, b) => b.confidenceScore - a.confidenceScore),
    [causalPaths]
  );

  // Severity dot color
  const getSeverityColor = (severity: string): string => {
    switch (severity) {
      case 'critical': return 'var(--color-status-err)';
      case 'high': return 'var(--color-status-err)';
      case 'medium': return 'var(--color-status-warn)';
      case 'low': return 'var(--color-text-faint)';
      default: return 'var(--color-text-faint)';
    }
  };

  // Handle path click - toggle selection
  const handlePathClick = (pathId: string) => {
    onPathSelect(selectedPathId === pathId ? null : pathId);
  };

  return (
    <aside
      className="h-full flex flex-col bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)] text-[var(--color-text-primary)]"
      style={{ width: '380px', minWidth: '380px' }}
    >
      {/* Header */}
      <div className="px-5 pt-4 pb-3.5 border-b border-[var(--color-border-soft)]">
        <h3 className="text-[13.5px] font-semibold flex items-center gap-2">
          Causal paths
          {causalPaths.length > 0 && (
            <span className="text-xs font-normal text-[var(--color-text-faint)]">
              {causalPaths.length} candidate{causalPaths.length !== 1 ? 's' : ''}
            </span>
          )}
        </h3>
        <p className="text-xs text-[var(--color-text-faint)] mt-0.5">
          Ranked root-cause candidates for this snapshot
        </p>
      </div>

      {/* Paths List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <div className="flex flex-col items-center gap-2.5">
              <div className="w-5 h-5 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
              <span className="text-xs text-[var(--color-text-faint)]">Loading paths...</span>
            </div>
          </div>
        ) : sortedPaths.length > 0 ? (
          <div>
            {sortedPaths.map((path, index) => {
              const isSelected = selectedPathId === path.id;
              const confidencePercent = Math.round(path.confidenceScore * 100);
              const rootAnomalies = path.candidateRoot.anomalies ?? [];

              return (
                <div
                  key={path.id}
                  data-causal-path={path.id}
                  onClick={() => handlePathClick(path.id)}
                  className={`px-5 py-4 cursor-pointer border-b border-[var(--color-border-soft)] transition-colors ${
                    isSelected
                      ? 'bg-[var(--color-surface-muted)] shadow-[inset_2px_0_0_var(--color-accent)]'
                      : 'hover:bg-[var(--color-surface-muted)]'
                  }`}
                >
                  {/* Rank, confidence and severity */}
                  <div className="flex items-center gap-2 text-[11.5px] text-[var(--color-text-faint)]">
                    <span className="font-mono font-medium text-[var(--color-text-secondary)]">#{index + 1}</span>
                    <span className="inline-flex items-center gap-1.5" title="Confidence">
                      <span className="w-11 h-1 rounded-full bg-[var(--color-border-strong)] overflow-hidden">
                        <span className="block h-full bg-[var(--color-accent)]" style={{ width: `${confidencePercent}%` }} />
                      </span>
                      <span className="tabular-nums">{confidencePercent}%</span>
                    </span>
                    <span className="ml-auto inline-flex items-center gap-1.5">
                      <span className="w-1.5 h-1.5 rounded-full" style={{ background: getSeverityColor(path.ranking.maxAnomalySeverity) }} />
                      {path.ranking.maxAnomalySeverity}
                    </span>
                  </div>

                  {/* Root cause resource */}
                  <div className="text-[13px] font-medium mt-2 mb-2 break-all">
                    {path.candidateRoot.resource.kind}/{path.candidateRoot.resource.name}
                  </div>

                  {/* Chain of kinds */}
                  <div className="flex items-center flex-wrap gap-1 text-[11.5px] text-[var(--color-text-secondary)]">
                    {path.steps.map((step, stepIndex) => (
                      <React.Fragment key={stepIndex}>
                        {stepIndex > 0 && (
                          <svg className="w-3 h-3 text-[var(--color-text-faint)]" fill="none" stroke="currentColor" strokeWidth={2} viewBox="0 0 24 24" strokeLinecap="round"><path d="m9 6 6 6-6 6" /></svg>
                        )}
                        <span className="px-1.5 py-0.5 rounded bg-[var(--color-surface-active)]">{step.node.resource.kind}</span>
                      </React.Fragment>
                    ))}
                  </div>

                  {/* Anomalies at root and path metrics */}
                  {!isSelected && (
                    <div className="mt-2.5 text-[11.5px] text-[var(--color-text-faint)]">
                      <span title="Effective causal distance (cause-introducing edges)">
                        {path.ranking.effectiveCausalDistance} hop{path.ranking.effectiveCausalDistance !== 1 ? 's' : ''}
                      </span>
                      <span className="mx-1.5">·</span>
                      <span title="Number of steps in path">
                        {path.steps.length} step{path.steps.length !== 1 ? 's' : ''}
                      </span>
                      {rootAnomalies.length > 0 && (
                        <>
                          <span className="mx-1.5">·</span>
                          <span>{rootAnomalies.length} anomal{rootAnomalies.length !== 1 ? 'ies' : 'y'} at root</span>
                        </>
                      )}
                    </div>
                  )}

                  {/* Expanded details when selected */}
                  {isSelected && (
                    <div className="mt-3">
                      {/* Explanation */}
                      <p className="text-xs text-[var(--color-text-secondary)] leading-relaxed">
                        {path.explanation}
                      </p>

                      {rootAnomalies.length > 0 && (
                        <div className="flex flex-wrap gap-1 mt-2.5">
                          {rootAnomalies.slice(0, 2).map((anomaly, i) => (
                            <span
                              key={i}
                              className="text-[11px] px-1.5 py-0.5 rounded border border-[var(--color-border-strong)] text-[var(--color-text-secondary)] truncate max-w-full"
                              title={anomaly.summary}
                            >
                              {anomaly.type}
                            </span>
                          ))}
                          {rootAnomalies.length > 2 && (
                            <span className="text-[11px] px-1.5 py-0.5 rounded border border-[var(--color-border-strong)] text-[var(--color-text-faint)]">
                              +{rootAnomalies.length - 2}
                            </span>
                          )}
                        </div>
                      )}

                      {/* Path steps */}
                      <div className="ui-section-label mt-4 mb-1.5">Path steps</div>
                      <div>
                        {path.steps.map((step, stepIndex) => {
                          // Check if step has spec changes to display
                          const hasSpecChanges = step.node.primaryEvent?.diff && step.node.primaryEvent.diff.length > 0;
                          const hasAnomalies = step.node.anomalies && step.node.anomalies.length > 0;

                          return (
                            <div
                              key={stepIndex}
                              className="group relative flex items-start gap-2.5 py-1"
                            >
                              {/* Step indicator */}
                              <div className="relative flex flex-col items-center pt-1">
                                <span
                                  className="w-2.5 h-2.5 rounded-full border-2 bg-[var(--color-surface-muted)]"
                                  style={{ borderColor: stepIndex === 0 ? 'var(--color-accent)' : hasAnomalies ? 'var(--color-status-err)' : 'var(--color-text-faint)' }}
                                />
                              </div>

                              {/* Resource info */}
                              <div className="flex-1 min-w-0">
                                <div className="text-xs truncate flex items-center gap-1.5">
                                  <span className="text-[var(--color-text-faint)]">{step.node.resource.kind}</span>
                                  <span className="truncate">{step.node.resource.name}</span>
                                  {hasSpecChanges && (
                                    <span className="text-[10px] px-1 rounded bg-[var(--color-accent-soft)] text-[var(--color-accent-strong)]" title="Has spec changes">
                                      Δ
                                    </span>
                                  )}
                                </div>
                                {step.edge && (
                                  <div className="text-[11px] text-[var(--color-text-faint)]">
                                    via {step.edge.relationshipType.toLowerCase()}
                                    {' · '}
                                    {step.edge.edgeCategory === 'CAUSE_INTRODUCING' ? 'causal' : 'ownership'}
                                  </div>
                                )}
                              </div>

                              {/* Anomaly indicator */}
                              {hasAnomalies && (
                                <span
                                  className="text-[10.5px] tabular-nums text-[var(--color-status-err-text)] mt-0.5"
                                  title={`${step.node.anomalies.length} anomal${step.node.anomalies.length > 1 ? 'ies' : 'y'}`}
                                >
                                  {step.node.anomalies.length}
                                </span>
                              )}

                              {/* Hover tooltip for spec changes */}
                              {hasSpecChanges && (
                                <div className="ui-popover absolute z-50 hidden group-hover:block left-0 top-full mt-1 p-2.5 max-w-xs max-h-48 overflow-auto">
                                  <div className="ui-section-label mb-1">
                                    Spec changes
                                  </div>
                                  <pre className="text-[11px] font-mono whitespace-pre-wrap break-words">
                                    {step.node.primaryEvent!.diff!.map((d, i) => {
                                      const opColor = d.op === 'add'
                                        ? 'text-[var(--color-status-ok-text)]'
                                        : d.op === 'remove'
                                          ? 'text-[var(--color-status-err-text)]'
                                          : 'text-[var(--color-status-warn-text)]';
                                      return (
                                        <div key={i} className={opColor}>
                                          {d.op === 'add' ? '+' : d.op === 'remove' ? '-' : '~'} {d.path}: {
                                            d.op === 'remove'
                                              ? JSON.stringify(d.old)
                                              : d.op === 'add'
                                                ? JSON.stringify(d.new)
                                                : `${JSON.stringify(d.old)} → ${JSON.stringify(d.new)}`
                                          }
                                        </div>
                                      );
                                    })}
                                  </pre>
                                </div>
                              )}
                            </div>
                          );
                        })}
                      </div>

                      {/* Ranking breakdown */}
                      <div className="mt-3 pt-3 border-t border-[var(--color-border-soft)] grid grid-cols-2 gap-x-4 text-[11.5px]">
                        <div className="flex justify-between">
                          <span className="text-[var(--color-text-faint)]">Temporal</span>
                          <span className="tabular-nums">{Math.round(path.ranking.temporalScore * 100)}%</span>
                        </div>
                        <div className="flex justify-between">
                          <span className="text-[var(--color-text-faint)]">Severity</span>
                          <span className="tabular-nums">{Math.round(path.ranking.severityScore * 100)}%</span>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        ) : (
          <div className="flex items-center justify-center h-full p-6">
            <div className="text-center">
              <div className="w-10 h-10 mx-auto mb-3 rounded-full grid place-items-center bg-[var(--color-surface-muted)] text-[var(--color-text-faint)]">
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
                  <path d="M20 6 9 17l-5-5" />
                </svg>
              </div>
              <p className="text-[12.5px] text-[var(--color-text-secondary)]">No causal paths found</p>
              <p className="text-xs text-[var(--color-text-faint)] mt-1">
                No anomalies detected in this namespace
              </p>
            </div>
          </div>
        )}
      </div>

      {/* Hint */}
      {selectedPathId && (
        <div className="px-5 h-8 flex items-center justify-center border-t border-[var(--color-border-soft)] text-[11px] text-[var(--color-text-faint)]">
          Click the path again to deselect
        </div>
      )}
    </aside>
  );
};

export default CausalPathsSidebar;
