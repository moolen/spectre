import React from 'react';

interface ZoomControlsProps {
  /** Handler for zoom in action */
  onZoomIn: () => void;
  /** Handler for zoom out action */
  onZoomOut: () => void;
  /** Handler for fit-to-view action */
  onFitToView: () => void;
  /** Handler for reset zoom action */
  onResetZoom: () => void;
}

/**
 * Zoom controls for the namespace graph
 * Positioned in the bottom-left corner of the graph area
 */
export const ZoomControls: React.FC<ZoomControlsProps> = ({
  onZoomIn,
  onZoomOut,
  onFitToView,
  onResetZoom,
}) => {
  return (
    <div className="absolute bottom-4 left-4 flex flex-col rounded-lg border border-[var(--color-border-strong)] bg-[var(--color-surface-elevated)] overflow-hidden z-10 divide-y divide-[var(--color-border-soft)]">
      <button type="button" onClick={onZoomIn} title="Zoom In (+)" aria-label="Zoom In" className="w-[30px] h-[30px] grid place-items-center text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors">
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
          <path d="M12 5v14M5 12h14" />
        </svg>
      </button>
      <button type="button" onClick={onZoomOut} title="Zoom Out (-)" aria-label="Zoom Out" className="w-[30px] h-[30px] grid place-items-center text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors">
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
          <path d="M5 12h14" />
        </svg>
      </button>
      <button type="button" onClick={onFitToView} title="Fit to View (F)" aria-label="Fit to View" className="w-[30px] h-[30px] grid place-items-center text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors">
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
          <path d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3" />
        </svg>
      </button>
      <button type="button" onClick={onResetZoom} title="Reset Zoom (0)" aria-label="Reset Zoom" className="w-[30px] h-[30px] grid place-items-center text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors">
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
          <path d="M21 12a9 9 0 1 1-3-6.7L21 8" /><path d="M21 3v5h-5" />
        </svg>
      </button>
    </div>
  );
};

export default ZoomControls;
