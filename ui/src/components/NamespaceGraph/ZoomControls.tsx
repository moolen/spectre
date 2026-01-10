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
 * Positioned in the bottom-right corner of the graph area
 */
export const ZoomControls: React.FC<ZoomControlsProps> = ({
  onZoomIn,
  onZoomOut,
  onFitToView,
  onResetZoom,
}) => {
  return (
    <div className="absolute bottom-4 right-4 flex flex-col gap-1 bg-[var(--color-surface-elevated)] 
                    border border-[var(--color-border-soft)] rounded-lg shadow-lg p-1 z-10">
      {/* Zoom In */}
      <button
        onClick={onZoomIn}
        title="Zoom In (+)"
        className="p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors
                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v12m6-6H6" />
        </svg>
      </button>

      {/* Zoom Out */}
      <button
        onClick={onZoomOut}
        title="Zoom Out (-)"
        className="p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors
                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M18 12H6" />
        </svg>
      </button>

      {/* Divider */}
      <div className="h-px bg-[var(--color-border-soft)] my-1" />

      {/* Fit to View */}
      <button
        onClick={onFitToView}
        title="Fit to View (F)"
        className="p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors
                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
        </svg>
      </button>

      {/* Reset Zoom */}
      <button
        onClick={onResetZoom}
        title="Reset Zoom (0)"
        className="p-2 rounded hover:bg-[var(--color-surface-muted)] transition-colors
                   text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]"
      >
        <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </button>
    </div>
  );
};

export default ZoomControls;
