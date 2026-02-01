import React from 'react';

interface ObservatoryZoomControlsProps {
  onZoomIn: () => void;
  onZoomOut: () => void;
  onFitToView: () => void;
  onResetZoom: () => void;
}

/**
 * Zoom control buttons for the Observatory graph
 */
export function ObservatoryZoomControls({
  onZoomIn,
  onZoomOut,
  onFitToView,
  onResetZoom,
}: ObservatoryZoomControlsProps) {
  return (
    <div className="absolute bottom-4 right-4 flex flex-col gap-1 bg-[#1a1a1a] rounded-lg p-1 shadow-lg border border-[#2a2a2a]">
      <button
        onClick={onZoomIn}
        className="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-white hover:bg-[#2a2a2a] rounded transition-colors"
        title="Zoom In (+)"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="12" y1="5" x2="12" y2="19" />
          <line x1="5" y1="12" x2="19" y2="12" />
        </svg>
      </button>
      <button
        onClick={onZoomOut}
        className="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-white hover:bg-[#2a2a2a] rounded transition-colors"
        title="Zoom Out (-)"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="5" y1="12" x2="19" y2="12" />
        </svg>
      </button>
      <div className="h-px bg-[#2a2a2a] mx-1" />
      <button
        onClick={onFitToView}
        className="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-white hover:bg-[#2a2a2a] rounded transition-colors"
        title="Fit to View (F)"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3" />
        </svg>
      </button>
      <button
        onClick={onResetZoom}
        className="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-white hover:bg-[#2a2a2a] rounded transition-colors"
        title="Reset Zoom (0)"
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
        </svg>
      </button>
    </div>
  );
}

export default ObservatoryZoomControls;
