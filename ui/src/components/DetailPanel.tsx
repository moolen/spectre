import React, { useMemo, useEffect, useState } from 'react';
import { K8sResource } from '../types';
import { diffJsonWithContext, DiffLine } from '../utils/jsonDiff';
import { useSettings } from '../hooks/useSettings';

interface DetailPanelProps {
  resource: K8sResource | null;
  selectedIndex?: number;
  onClose: () => void;
  onAnalyzeRootCause?: () => void; // New prop
}

const DiffLineView = ({ line }: { line: DiffLine }) => {
  const styleMap: Record<DiffLine['type'], string> = {
    add: 'text-green-500 bg-green-100/10',
    remove: 'text-red-400 bg-red-100/10',
    context: 'text-[var(--color-text-primary)]',
    gap: 'text-[var(--color-text-muted)] italic',
  };

  const prefixMap: Record<DiffLine['type'], string> = {
    add: '+',
    remove: '-',
    context: ' ',
    gap: '…',
  };

  return (
    <div className={`flex gap-2 px-2 rounded font-mono text-xs ${styleMap[line.type]}`}>
      <span className="select-none w-3">{prefixMap[line.type]}</span>
      <span className="whitespace-pre-wrap">{line.content}</span>
    </div>
  );
};

const FullDocumentDiff = ({
  current,
  previous,
}: {
  current?: Record<string, any>;
  previous?: Record<string, any>;
}) => {
  // Get the full diff - we'll use a large context to show everything
  const fullJsonDiff = useMemo(() => {
    // Create a full diff by comparing the entire documents
    // This will include all lines with their diff types
    const prev = previous ? JSON.stringify(previous, null, 2).split('\n') : [];
    const curr = current ? JSON.stringify(current, null, 2).split('\n') : [];

    // Simple line-by-line comparison
    const result: DiffLine[] = [];
    const maxLines = Math.max(prev.length, curr.length);

    for (let i = 0; i < maxLines; i++) {
      const prevLine = prev[i];
      const currLine = curr[i];

      if (prevLine === undefined) {
        // Only in current
        result.push({ type: 'add', content: currLine });
      } else if (currLine === undefined) {
        // Only in previous
        result.push({ type: 'remove', content: prevLine });
      } else if (prevLine === currLine) {
        // Same in both
        result.push({ type: 'context', content: prevLine });
      } else {
        // Different
        result.push({ type: 'remove', content: prevLine });
        result.push({ type: 'add', content: currLine });
      }
    }

    return result;
  }, [previous, current]);

  if (!current && !previous) {
    return (
      <div className="bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic">
        Configuration details unavailable for this transition.
      </div>
    );
  }

  return (
    <div className="bg-[var(--color-surface-secondary)] p-2 rounded border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto">
      {fullJsonDiff.map((line, index) => (
        <DiffLineView key={`${line.type}-${index}`} line={line} />
      ))}
    </div>
  );
};

const ConfigDiff = ({
  current,
  previous,
  showFullDiff = false,
}: {
  current?: Record<string, any>;
  previous?: Record<string, any>;
  showFullDiff?: boolean;
}) => {
  const diff = useMemo(() => diffJsonWithContext(previous, current), [previous, current]);

  if (!current && !previous) {
    return (
      <div className="bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic">
        Configuration details unavailable for this transition.
      </div>
    );
  }

  if (showFullDiff) {
    return <FullDocumentDiff current={current} previous={previous} />;
  }

  if (diff.length === 0) {
    return (
      <div className="bg-[var(--color-surface-secondary)] p-3 rounded border border-[var(--color-border-soft)] text-xs text-[var(--color-text-muted)] italic">
        No changes detected in this transition.
      </div>
    );
  }

  return (
    <div className="bg-[var(--color-surface-secondary)] p-2 rounded border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto">
      {diff.map((line, index) => (
        <DiffLineView key={`${line.type}-${index}`} line={line} />
      ))}
    </div>
  );
};

export const DetailPanel: React.FC<DetailPanelProps> = ({ resource, selectedIndex = 0, onClose, onAnalyzeRootCause }) => {
  const { formatTime } = useSettings();
  const [showFullDiff, setShowFullDiff] = useState(false);
  const [width, setWidth] = useState(384); // Default w-96 = 24rem = 384px
  const [isResizing, setIsResizing] = useState(false);

  const MIN_WIDTH = 300;
  const MAX_WIDTH = 800;

  // Handle resize on mouse move
  useEffect(() => {
    if (!isResizing) return;

    const handleMouseMove = (e: MouseEvent) => {
      // Calculate new width based on mouse position from right edge
      const newWidth = window.innerWidth - e.clientX;

      // Constrain width between min and max
      if (newWidth >= MIN_WIDTH && newWidth <= MAX_WIDTH) {
        setWidth(newWidth);
      }
    };

    const handleMouseUp = () => {
      setIsResizing(false);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing]);

  // Handle Escape key to close panel
  useEffect(() => {
    if (!resource) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [resource, onClose]);

  if (!resource) return null;

  const currentSegment = resource.statusSegments[selectedIndex] || resource.statusSegments[0];
  const previousSegment = selectedIndex > 0 ? resource.statusSegments[selectedIndex - 1] : null;

  return (
    <div
      className="fixed right-0 bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)] shadow-2xl transform transition-transform duration-300 ease-in-out z-20 overflow-y-auto flex flex-col text-[var(--color-text-primary)] DetailPanel"
      style={{ width: `${width}px`, top: '73px', bottom: 0 }}
    >
      {/* Resize Handle */}
      <div
        onMouseDown={() => setIsResizing(true)}
        className={`absolute left-0 top-0 bottom-0 w-1 hover:w-1.5 hover:bg-blue-500/50 transition-all cursor-col-resize ${
          isResizing ? 'bg-blue-500 w-1.5' : 'bg-transparent'
        }`}
      />
      <div className="p-6 flex-1">
        {/* Header */}
        <div className="flex justify-between items-start mb-6">
          <div>
            <h2 className="text-xl font-bold break-all">{resource.name}</h2>
            <div className="flex items-center gap-2 mt-1">
                <span className="text-xs font-mono bg-[var(--color-surface-muted)] px-2 py-0.5 rounded text-[var(--color-text-muted)] border border-[var(--color-border-soft)]">{resource.kind}</span>
                <span className="text-xs text-[var(--color-text-muted)]">{resource.namespace}</span>
            </div>
          </div>
          <button onClick={onClose} className="text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] transition-colors p-1 hover:bg-[var(--color-surface-muted)] rounded">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
          </button>
        </div>

        <hr className="my-4 border-[var(--color-border-soft)]" />

        {/* Analyze Root Cause Button */}
        {onAnalyzeRootCause && (
          <div className="mb-6">
            <button
              onClick={onAnalyzeRootCause}
              className="w-full px-4 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg font-semibold transition-colors flex items-center justify-center gap-2"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 20l-5.447-2.724A1 1 0 013 16.382V5.618a1 1 0 011.447-.894L9 7m0 13l6-3m-6 3V7m6 10l4.553 2.276A1 1 0 0021 18.382V7.618a1 1 0 00-.553-.894L15 4m0 13V4m0 0L9 7" />
              </svg>
              Analyze Root Cause
            </button>
          </div>
        )}

        {/* Selected Segment Details */}
        <div className="mb-6">
            <div className="flex items-center justify-between mb-2">
                <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold">
                    Version {selectedIndex + 1} of {resource.statusSegments.length}
                </div>
                <div className="flex items-center gap-1">
                     <span className={`w-2 h-2 rounded-full flex-shrink-0 ${
                        currentSegment.status === 'Ready' ? 'bg-emerald-500' :
                        currentSegment.status === 'Error' ? 'bg-red-500' : 'bg-amber-500'
                     }`}></span>
                     <span className="text-xs text-[var(--color-text-muted)]">{currentSegment.status}</span>
                </div>
            </div>

            <div className="bg-[var(--color-surface-muted)] rounded p-3 border border-[var(--color-border-soft)] space-y-2 mb-4">
                 <div className="text-xs text-[var(--color-text-muted)] flex justify-between">
                    <span>Start:</span>
                    <span className="text-[var(--color-text-primary)]">{formatTime(currentSegment.start)}</span>
                 </div>
                 <div className="text-xs text-[var(--color-text-muted)] flex justify-between">
                    <span>End:</span>
                    <span className="text-[var(--color-text-primary)]">{formatTime(currentSegment.end)}</span>
                 </div>
                 <div className="text-xs text-[var(--color-text-primary)] italic border-t border-[var(--color-border-soft)] pt-2 mt-2">
                    {currentSegment.message}
                 </div>
            </div>

            {/* Config Diff */}
            <div className="mb-4">
                <div className="flex items-center justify-between mb-2">
                    <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold">Configuration Changes</div>
                    <button
                        onClick={() => setShowFullDiff(!showFullDiff)}
                        className={`text-xs px-2 py-1 rounded transition-colors ${
                            showFullDiff
                                ? 'bg-blue-500/20 text-blue-300 border border-blue-500/40'
                                : 'bg-[var(--color-surface-muted)] text-[var(--color-text-muted)] border border-[var(--color-border-soft)] hover:bg-[var(--color-surface-active)]'
                        }`}
                    >
                        {showFullDiff ? 'Full Document' : 'Diff View'}
                    </button>
                </div>
                <ConfigDiff
                  current={currentSegment.resourceData}
                  previous={previousSegment?.resourceData}
                  showFullDiff={showFullDiff}
                />
            </div>
        </div>

        {/* Audit Logs */}
        <div>
            <div className="text-sm text-[var(--color-text-muted)] uppercase tracking-wider font-semibold mb-2">Relevant Events</div>
            <div className="space-y-3">
                {resource.events.filter(e => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end).length === 0 ? (
                    <div className="text-[var(--color-text-muted)] text-sm italic">No events in this time window.</div>
                ) : (
                    resource.events
                    .filter(e => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end)
                    .sort((a,b) => b.timestamp.getTime() - a.timestamp.getTime())
                    .map((event) => (
                        <div key={event.id} className="bg-[var(--color-surface-muted)] p-3 rounded border-l-4 border-[var(--color-border-soft)] hover:bg-[var(--color-surface-active)] transition-colors space-y-2">
                            <div className="flex justify-between items-start">
                                <span className="text-xs font-mono text-[var(--color-text-muted)]">{formatTime(event.timestamp)}</span>
                                <span className={`text-xs px-2 py-0.5 rounded font-semibold tracking-wide ${
                                    event.type === 'Warning'
                                        ? 'bg-amber-100/20 text-amber-300 border border-amber-500/40'
                                        : 'bg-blue-100/20 text-blue-200 border border-blue-500/30'
                                }`}>
                                    {event.reason || 'Event'}
                                </span>
                            </div>
                            <div className="text-sm text-[var(--color-text-primary)] leading-snug">{event.message}</div>
                            <div className="text-xs text-[var(--color-text-muted)] flex gap-4">
                                {event.source && (
                                    <span>Source: {event.source}</span>
                                )}
                                <span>Count: {event.count ?? 0}</span>
                            </div>
                        </div>
                    ))
                )}
            </div>
        </div>
      </div>

      {/* Footer Hint */}
      <div className="p-3 bg-[var(--color-surface-secondary)] border-t border-[var(--color-border-soft)] text-center text-xs text-[var(--color-text-muted)]">
        Use Arrow Left/Right to navigate history
      </div>
    </div>
  );
};