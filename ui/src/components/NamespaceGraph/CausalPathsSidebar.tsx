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
  // Get confidence color based on score
  const getConfidenceColor = (score: number): string => {
    if (score >= 0.7) return 'text-green-400 bg-green-500/20';
    if (score >= 0.4) return 'text-amber-400 bg-amber-500/20';
    return 'text-red-400 bg-red-500/20';
  };

  // Get severity color
  const getSeverityColor = (severity: string): string => {
    switch (severity) {
      case 'critical': return 'bg-red-500';
      case 'high': return 'bg-orange-500';
      case 'medium': return 'bg-yellow-500';
      case 'low': return 'bg-blue-400';
      default: return 'bg-gray-500';
    }
  };

  // Handle path click - toggle selection
  const handlePathClick = (pathId: string) => {
    onPathSelect(selectedPathId === pathId ? null : pathId);
  };

  return (
    <div
      className="h-full flex flex-col bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)]"
      style={{ width: '220px', minWidth: '220px' }}
    >
      {/* Sidebar Header */}
      <div className="px-4 py-3 border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wider">
            Causal Paths
          </h3>
          {causalPaths.length > 0 && (
            <span className="text-[10px] text-[var(--color-text-muted)] bg-[var(--color-surface-active)] px-2 py-0.5 rounded-full">
              {causalPaths.length} path{causalPaths.length !== 1 ? 's' : ''}
            </span>
          )}
        </div>
        <p className="text-[10px] text-[var(--color-text-muted)] mt-1">
          Ranked candidate root causes
        </p>
      </div>

      {/* Paths List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex items-center justify-center h-full">
            <div className="flex flex-col items-center gap-2">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
              <span className="text-[10px] text-[var(--color-text-muted)]">Loading paths...</span>
            </div>
          </div>
        ) : causalPaths.length > 0 ? (
          <div className="divide-y divide-[var(--color-border-soft)]">
            {causalPaths.map((path, index) => {
              const isSelected = selectedPathId === path.id;
              const confidencePercent = Math.round(path.confidenceScore * 100);

              return (
                <div
                  key={path.id}
                  onClick={() => handlePathClick(path.id)}
                  className={`p-3 cursor-pointer transition-colors ${
                    isSelected
                      ? 'bg-blue-500/20 border-l-2 border-l-blue-500'
                      : 'hover:bg-[var(--color-surface-hover)] border-l-2 border-l-transparent'
                  }`}
                >
                  {/* Path header */}
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <span className="text-[10px] font-bold text-[var(--color-text-muted)] uppercase">
                        #{index + 1}
                      </span>
                      <span className={`text-[9px] font-semibold px-1.5 py-0.5 rounded ${getConfidenceColor(path.confidenceScore)}`}>
                        {confidencePercent}%
                      </span>
                    </div>
                    <div className="flex items-center gap-1">
                      <div className={`w-1.5 h-1.5 rounded-full ${getSeverityColor(path.ranking.maxAnomalySeverity)}`} />
                      <span className="text-[9px] text-[var(--color-text-muted)]">
                        {path.ranking.maxAnomalySeverity}
                      </span>
                    </div>
                  </div>

                  {/* Root cause resource */}
                  <div className="mb-2">
                    <div className="text-[9px] text-[var(--color-text-muted)] uppercase mb-0.5">
                      Root Cause
                    </div>
                    <div className="text-xs font-semibold text-[var(--color-text-primary)]">
                      {path.candidateRoot.resource.kind}/{path.candidateRoot.resource.name}
                    </div>
                  </div>

                  {/* Anomalies at root */}
                  {path.candidateRoot.anomalies && path.candidateRoot.anomalies.length > 0 && (
                    <div className="mb-2">
                      <div className="flex flex-wrap gap-1">
                        {path.candidateRoot.anomalies.slice(0, 2).map((anomaly, i) => (
                          <span
                            key={i}
                            className="text-[8px] px-1.5 py-0.5 bg-red-500/20 text-red-400 rounded truncate max-w-full"
                            title={anomaly.summary}
                          >
                            {anomaly.type}
                          </span>
                        ))}
                        {path.candidateRoot.anomalies.length > 2 && (
                          <span className="text-[8px] px-1.5 py-0.5 bg-gray-500/20 text-gray-400 rounded">
                            +{path.candidateRoot.anomalies.length - 2}
                          </span>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Path metrics */}
                  <div className="flex items-center gap-3 text-[9px] text-[var(--color-text-muted)]">
                    <span title="Effective causal distance (cause-introducing edges)">
                      {path.ranking.effectiveCausalDistance} hop{path.ranking.effectiveCausalDistance !== 1 ? 's' : ''}
                    </span>
                    <span>-</span>
                    <span title="Number of steps in path">
                      {path.steps.length} step{path.steps.length !== 1 ? 's' : ''}
                    </span>
                  </div>

                  {/* Expanded details when selected */}
                  {isSelected && (
                    <div className="mt-3 pt-3 border-t border-[var(--color-border-soft)]">
                      {/* Explanation */}
                      <div className="text-[10px] text-[var(--color-text-primary)] mb-3 leading-relaxed">
                        {path.explanation}
                      </div>

                      {/* Path steps */}
                      <div className="text-[9px] text-[var(--color-text-muted)] uppercase mb-2">
                        Path Steps
                      </div>
                      <div className="space-y-1">
                        {path.steps.map((step, stepIndex) => (
                          <div
                            key={stepIndex}
                            className="flex items-center gap-2"
                          >
                            {/* Step indicator */}
                            <div className={`w-4 h-4 rounded-full flex items-center justify-center text-[8px] font-bold ${
                              stepIndex === 0
                                ? 'bg-red-500 text-white'
                                : stepIndex === path.steps.length - 1
                                  ? 'bg-blue-500 text-white'
                                  : 'bg-[var(--color-surface-muted)] text-[var(--color-text-muted)]'
                            }`}>
                              {stepIndex + 1}
                            </div>

                            {/* Resource info */}
                            <div className="flex-1 min-w-0">
                              <div className="text-[10px] text-[var(--color-text-primary)] truncate">
                                {step.node.resource.kind}/{step.node.resource.name}
                              </div>
                              {step.edge && (
                                <div className="text-[8px] text-[var(--color-text-muted)]">
                                  via {step.edge.relationshipType}
                                  <span className={`ml-1 ${
                                    step.edge.edgeCategory === 'CAUSE_INTRODUCING'
                                      ? 'text-orange-400'
                                      : 'text-gray-500'
                                  }`}>
                                    ({step.edge.edgeCategory === 'CAUSE_INTRODUCING' ? 'causal' : 'ownership'})
                                  </span>
                                </div>
                              )}
                            </div>

                            {/* Anomaly indicator */}
                            {step.node.anomalies && step.node.anomalies.length > 0 && (
                              <div
                                className="w-2 h-2 rounded-full bg-red-500"
                                title={`${step.node.anomalies.length} anomal${step.node.anomalies.length > 1 ? 'ies' : 'y'}`}
                              />
                            )}
                          </div>
                        ))}
                      </div>

                      {/* Ranking breakdown */}
                      <div className="mt-3 pt-2 border-t border-[var(--color-border-soft)]">
                        <div className="text-[9px] text-[var(--color-text-muted)] uppercase mb-1">
                          Ranking Factors
                        </div>
                        <div className="grid grid-cols-2 gap-x-3 gap-y-1 text-[9px]">
                          <div className="flex justify-between">
                            <span className="text-[var(--color-text-muted)]">Temporal</span>
                            <span className="text-[var(--color-text-primary)]">{Math.round(path.ranking.temporalScore * 100)}%</span>
                          </div>
                          <div className="flex justify-between">
                            <span className="text-[var(--color-text-muted)]">Severity</span>
                            <span className="text-[var(--color-text-primary)]">{Math.round(path.ranking.severityScore * 100)}%</span>
                          </div>
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
              <svg
                className="w-12 h-12 mx-auto mb-3 text-[var(--color-text-muted)] opacity-40"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                />
              </svg>
              <p className="text-sm text-[var(--color-text-muted)]">No causal paths found</p>
              <p className="text-[10px] text-[var(--color-text-muted)] mt-1">
                No anomalies detected in this namespace
              </p>
            </div>
          </div>
        )}
      </div>

      {/* Keyboard hint */}
      {selectedPathId && (
        <div className="px-3 py-2 border-t border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]">
          <p className="text-[9px] text-[var(--color-text-muted)] text-center">
            Click path again to deselect
          </p>
        </div>
      )}
    </div>
  );
};

export default CausalPathsSidebar;
