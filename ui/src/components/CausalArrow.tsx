/**
 * Causal Arrow Component
 * Renders blue curved arrows showing causal relationships on the timeline
 */

import React from 'react';
import * as d3 from 'd3';

export interface CausalArrowProps {
  fromUID: string;
  toUID: string;
  timestamp: Date;
  xScale: d3.ScaleTime<number, number>;
  resourcePositions: Map<string, number>;
  label?: string;
}

export const CausalArrow: React.FC<CausalArrowProps> = ({
  fromUID,
  toUID,
  timestamp,
  xScale,
  resourcePositions,
  label
}) => {
  const y1 = resourcePositions.get(fromUID);
  const y2 = resourcePositions.get(toUID);
  const x = xScale(timestamp);

  // If either resource is not visible, don't render arrow
  if (y1 === undefined || y2 === undefined) {
    return null;
  }

  // Calculate bezier curve for smooth arc
  const controlPointOffset = Math.abs(y2 - y1) * 0.3;
  const midY = (y1 + y2) / 2;
  
  // Create curved path
  const path = `
    M ${x} ${y1}
    C ${x + controlPointOffset} ${y1},
      ${x + controlPointOffset} ${y2},
      ${x} ${y2}
  `;

  return (
    <g className="causal-arrow" data-from={fromUID} data-to={toUID}>
      {/* Arrow path - no arrowhead */}
      <path
        d={path}
        stroke="#3b82f6"
        strokeWidth={2}
        fill="none"
        strokeDasharray="5,5"
        opacity={0.8}
        className="transition-opacity duration-200 hover:opacity-100"
      />
      
      {/* Optional label */}
      {label && (
        <text
          x={x + controlPointOffset / 2}
          y={midY}
          fill="#3b82f6"
          fontSize="10"
          textAnchor="middle"
          className="pointer-events-none"
        >
          {label}
        </text>
      )}
    </g>
  );
};

export interface CausalArrowLayerProps {
  causalChain: Array<{
    resource: { uid: string };
    changeEvent?: { timestamp: string };
    relationshipTo?: { uid: string };
  }>;
  xScale: d3.ScaleTime<number, number>;
  resourcePositions: Map<string, number>;
}

/**
 * Renders all causal arrows as an SVG layer
 */
export const CausalArrowLayer: React.FC<CausalArrowLayerProps> = ({
  causalChain,
  xScale,
  resourcePositions
}) => {
  if (!causalChain || causalChain.length === 0) {
    return null;
  }

  const arrows: React.ReactElement[] = [];

  // Build arrows by connecting each step to the next
  for (let i = 0; i < causalChain.length - 1; i++) {
    const currentStep = causalChain[i];
    const nextStep = causalChain[i + 1];

    // Use relationshipTo if available, otherwise connect to next step
    const toUID = currentStep.relationshipTo?.uid || nextStep.resource.uid;
    
    if (!currentStep.changeEvent) {
      continue; // Skip if no timestamp available
    }

    const timestamp = new Date(currentStep.changeEvent.timestamp);

    arrows.push(
      <CausalArrow
        key={`arrow-${i}`}
        fromUID={currentStep.resource.uid}
        toUID={toUID}
        timestamp={timestamp}
        xScale={xScale}
        resourcePositions={resourcePositions}
      />
    );
  }

  return (
    <g className="causal-arrow-layer">
      {arrows}
    </g>
  );
};
