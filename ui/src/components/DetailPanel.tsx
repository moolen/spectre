import React, { useMemo, useEffect, useState } from 'react';
import { K8sResource } from '../types';
import { diffJsonWithContext, DiffLine } from '../utils/jsonDiff';
import { useSettings } from '../hooks/useSettings';
import { STATUS_COLORS } from '../constants';

export const DEFAULT_DETAIL_PANEL_WIDTH = 400;

interface DetailPanelProps {
  resource: K8sResource | null;
  selectedIndex?: number;
  onClose: () => void;
}

const DiffLineView = ({ line }: { line: DiffLine }) => {
  const styleMap: Record<DiffLine['type'], string> = {
    add: 'text-[var(--color-status-ok-text)] bg-[var(--color-status-ok-soft)]',
    remove: 'text-[var(--color-status-err-text)] bg-[var(--color-status-err-soft)]',
    context: 'text-[var(--color-text-secondary)]',
    gap: 'text-[var(--color-text-faint)] italic',
  };

  const prefixMap: Record<DiffLine['type'], string> = {
    add: '+',
    remove: '-',
    context: ' ',
    gap: '…',
  };

  return (
    <div className={`flex gap-2 px-3 font-mono text-[11.5px] leading-[1.65] ${styleMap[line.type]}`}>
      <span className="select-none w-3 opacity-70">{prefixMap[line.type]}</span>
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
      <div className="bg-[var(--color-app-bg)] px-3 py-2.5 rounded-lg border border-[var(--color-border-soft)] text-xs text-[var(--color-text-faint)]">
        Configuration details unavailable for this transition.
      </div>
    );
  }

  return (
    <div className="bg-[var(--color-app-bg)] py-2 rounded-lg border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto">
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
      <div className="bg-[var(--color-app-bg)] px-3 py-2.5 rounded-lg border border-[var(--color-border-soft)] text-xs text-[var(--color-text-faint)]">
        Configuration details unavailable for this transition.
      </div>
    );
  }

  if (showFullDiff) {
    return <FullDocumentDiff current={current} previous={previous} />;
  }

  if (diff.length === 0) {
    return (
      <div className="bg-[var(--color-app-bg)] px-3 py-2.5 rounded-lg border border-[var(--color-border-soft)] text-xs text-[var(--color-text-faint)]">
        No changes detected in this transition.
      </div>
    );
  }

  return (
    <div className="bg-[var(--color-app-bg)] py-2 rounded-lg border border-[var(--color-border-soft)] font-mono text-xs overflow-x-auto">
      {diff.map((line, index) => (
        <DiffLineView key={`${line.type}-${index}`} line={line} />
      ))}
    </div>
  );
};

export const DetailPanel: React.FC<DetailPanelProps> = ({ resource, selectedIndex = 0, onClose }) => {
  const { formatTime } = useSettings();
  const [showFullDiff, setShowFullDiff] = useState(false);
  const [width, setWidth] = useState(DEFAULT_DETAIL_PANEL_WIDTH);
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
  const segmentEvents = resource.events
    .filter(e => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end)
    .sort((a, b) => b.timestamp.getTime() - a.timestamp.getTime());
  const statusTone = STATUS_TONE[currentSegment.status] ?? STATUS_TONE.Unknown;

  // Proportional status strip across all versions of this resource
  const firstStart = resource.statusSegments[0]?.start.getTime() ?? 0;
  const lastEnd = resource.statusSegments[resource.statusSegments.length - 1]?.end.getTime() ?? 0;
  const totalSpan = Math.max(1, lastEnd - firstStart);

  return (
    <aside
      className="absolute right-0 top-0 bottom-0 bg-[var(--color-surface-elevated)] border-l border-[var(--color-border-soft)] z-20 overflow-y-auto flex flex-col text-[var(--color-text-primary)] DetailPanel"
      style={{ width: `${width}px` }}
    >
      {/* Resize Handle */}
      <div
        onMouseDown={() => setIsResizing(true)}
        className={`absolute left-0 top-0 bottom-0 w-1 cursor-col-resize transition-colors ${
          isResizing ? 'bg-[var(--color-accent-line)]' : 'bg-transparent hover:bg-[var(--color-accent-soft)]'
        }`}
      />

      {/* Header */}
      <div className="px-5 pt-4 pb-4 border-b border-[var(--color-border-soft)]">
        <div className="flex items-center gap-2 text-[11.5px] text-[var(--color-text-faint)]">
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24" strokeLinecap="round" strokeLinejoin="round">
            <path d="m12 2 10 5-10 5L2 7z" /><path d="m2 17 10 5 10-5" /><path d="m2 12 10 5 10-5" />
          </svg>
          <span>{resource.kind}</span>
          <span className="text-[var(--color-border-strong)]">·</span>
          <span className="truncate">{resource.namespace}</span>
          <button
            onClick={onClose}
            aria-label="Close details"
            className="ml-auto w-[26px] h-[26px] grid place-items-center rounded-md text-[var(--color-text-faint)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12"></path></svg>
          </button>
        </div>
        <h2 className="text-base font-semibold tracking-[-0.01em] mt-2 mb-2 break-all">{resource.name}</h2>
        <span className="ui-pill" style={{ background: statusTone.soft, color: statusTone.text }}>
          <i style={{ background: statusTone.dot }} />
          {currentSegment.status}
        </span>
      </div>

      <div className="px-5 flex-1">
        {/* Selected version */}
        <section className="py-4 border-b border-[var(--color-border-soft)]">
          <div className="flex items-center mb-2.5 ui-section-label">
            <span>Version {selectedIndex + 1} of {resource.statusSegments.length}</span>
            <span className="ml-auto font-normal tabular-nums">{formatTime(currentSegment.start)} – {formatTime(currentSegment.end)}</span>
          </div>
          <div className="flex h-1.5 gap-[2px] rounded-sm overflow-hidden">
            {resource.statusSegments.map((segment, index) => (
              <div
                key={index}
                title={`${segment.status}: ${formatTime(segment.start)} – ${formatTime(segment.end)}`}
                style={{
                  flexGrow: Math.max(0.002, (segment.end.getTime() - segment.start.getTime()) / totalSpan),
                  background: STATUS_COLORS[segment.status],
                  opacity: index === selectedIndex ? 1 : 0.45,
                }}
              />
            ))}
          </div>
          {currentSegment.message && (
            <p className="mt-3 text-[12.5px] leading-relaxed text-[var(--color-text-secondary)]">
              {currentSegment.message}
            </p>
          )}
        </section>

        {/* Config Diff */}
        <section className="py-4 border-b border-[var(--color-border-soft)]">
          <div className="flex items-center justify-between mb-2.5">
            <div className="ui-section-label">Configuration changes</div>
            <div className="ui-segmented !h-[26px] !p-[2px]">
              <button type="button" aria-pressed={!showFullDiff} onClick={() => setShowFullDiff(false)} className="!text-[11px] !px-2">
                Diff View
              </button>
              <button type="button" aria-pressed={showFullDiff} onClick={() => setShowFullDiff(true)} className="!text-[11px] !px-2">
                Full Document
              </button>
            </div>
          </div>
          <ConfigDiff
            current={currentSegment.resourceData}
            previous={previousSegment?.resourceData}
            showFullDiff={showFullDiff}
          />
        </section>

        {/* Events */}
        <section className="py-4">
          <div className="flex items-center mb-1 ui-section-label">
            <span>Relevant events</span>
            <span className="ml-auto font-normal">{segmentEvents.length}</span>
          </div>
          {segmentEvents.length === 0 ? (
            <div className="text-[var(--color-text-faint)] text-[12.5px] py-2">No events in this time window.</div>
          ) : (
            segmentEvents.map((event) => (
              <div key={event.id} className="grid grid-cols-[62px_1fr] gap-2.5 py-2">
                <span className="font-mono text-[11px] leading-[18px] text-[var(--color-text-faint)] tabular-nums">{formatTime(event.timestamp)}</span>
                <div className="min-w-0">
                  <div className="flex items-center gap-1.5 text-[12.5px] font-medium">
                    <span
                      className="w-1.5 h-1.5 rounded-full flex-none"
                      style={{ background: event.type === 'Warning' ? 'var(--color-status-warn)' : 'var(--color-text-faint)' }}
                    />
                    <span className="truncate">{event.reason || 'Event'}</span>
                  </div>
                  <div className="text-xs text-[var(--color-text-secondary)] leading-snug mt-0.5 break-words">{event.message}</div>
                  <div className="text-[11px] text-[var(--color-text-faint)] mt-1 flex gap-3">
                    {event.source && <span>{event.source}</span>}
                    <span>×{event.count ?? 0}</span>
                  </div>
                </div>
              </div>
            ))
          )}
        </section>
      </div>

      {/* Footer */}
      <div className="px-5 h-8 flex-none border-t border-[var(--color-border-soft)] flex items-center text-[11px] text-[var(--color-text-faint)] gap-1.5">
        <span className="ui-kbd">←</span><span className="ui-kbd">→</span>
        <span className="ml-1">navigate history</span>
        <span className="ml-auto ui-kbd">esc</span>
      </div>
    </aside>
  );
};

const STATUS_TONE: Record<string, { dot: string; soft: string; text: string }> = {
  Ready: { dot: 'var(--color-status-ok)', soft: 'var(--color-status-ok-soft)', text: 'var(--color-status-ok-text)' },
  Warning: { dot: 'var(--color-status-warn)', soft: 'var(--color-status-warn-soft)', text: 'var(--color-status-warn-text)' },
  Error: { dot: 'var(--color-status-err)', soft: 'var(--color-status-err-soft)', text: 'var(--color-status-err-text)' },
  Terminating: { dot: 'var(--color-status-neutral)', soft: 'var(--color-surface-muted)', text: 'var(--color-text-secondary)' },
  Unknown: { dot: 'var(--color-status-neutral)', soft: 'var(--color-surface-muted)', text: 'var(--color-text-secondary)' },
};
