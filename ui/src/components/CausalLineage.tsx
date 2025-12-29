/**
 * Causal Lineage Component
 * Displays the root cause analysis in the detail panel
 */

import React from 'react';
import { RootCauseAnalysisV2, CausalStep } from '../types/rootCause';

interface CausalLineageProps {
  analysis: RootCauseAnalysisV2 | null;
  isLoading?: boolean;
}

const formatTimestamp = (isoString: string): string => {
  const date = new Date(isoString);
  return date.toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  });
};

const formatTimeLag = (ms: number): string => {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ago`;
  } else if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s ago`;
  } else {
    return `${seconds}s ago`;
  }
};

const StepIndicator: React.FC<{ type: 'root' | 'propagation' | 'terminal' }> = ({ type }) => {
  const colors = {
    root: 'bg-orange-500',
    propagation: 'bg-gray-500',
    terminal: 'bg-blue-500'
  };

  return <div className={`w-3 h-3 rounded-full ${colors[type]} mt-1 flex-shrink-0`} />;
};

const CausalStepView: React.FC<{
  step: CausalStep;
  type: 'root' | 'propagation' | 'terminal';
  timestamp?: string;
  isLast?: boolean;
}> = ({ step, type, timestamp, isLast }) => {
  const labels = {
    root: 'ROOT CAUSE',
    propagation: 'PROPAGATION',
    terminal: 'TERMINAL EFFECT'
  };

  const labelColors = {
    root: 'text-orange-500',
    propagation: 'text-gray-400',
    terminal: 'text-blue-500'
  };

  return (
    <div className="relative flex gap-3">
      {/* Vertical line and dot */}
      <div className="flex flex-col items-center">
        <StepIndicator type={type} />
        {!isLast && (
          <div className="w-0.5 h-full bg-gray-700 mt-1"></div>
        )}
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0 pb-6">
        <div className="flex items-start justify-between gap-2 mb-1">
          <div className={`text-[10px] font-semibold tracking-wide ${labelColors[type]}`}>
            {labels[type]}
          </div>
          {timestamp && (
            <div className="text-[10px] text-gray-500 font-mono">
              {formatTimestamp(timestamp)}
            </div>
          )}
        </div>
        
        <div className="text-sm font-medium text-white mb-0.5">
          {step.resource.name}
        </div>
        
        <div className="text-xs text-gray-400 italic">
          {step.reasoning}
        </div>
      </div>
    </div>
  );
};

export const CausalLineage: React.FC<CausalLineageProps> = ({ analysis, isLoading }) => {
  if (isLoading) {
    return (
      <div className="mb-4 pb-4 border-b border-[var(--color-border-soft)]">
        <h3 className="text-sm font-semibold mb-3 text-[var(--color-text-primary)]">Causal Lineage</h3>
        <div className="flex items-center gap-2 text-sm text-gray-400">
          <div className="animate-spin">
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <circle cx="12" cy="12" r="10" strokeWidth="2" opacity="0.25" />
              <path d="M12 2 A10 10 0 0 1 22 12" strokeWidth="2" strokeLinecap="round" />
            </svg>
          </div>
          <span>Analyzing root cause...</span>
        </div>
      </div>
    );
  }

  if (!analysis) {
    return (
      <div className="mb-4 pb-4 border-b border-[var(--color-border-soft)]">
        <h3 className="text-sm font-semibold mb-2 text-[var(--color-text-primary)]">Causal Lineage</h3>
        <div className="text-xs text-gray-500 italic">
          No causal lineage detected for this segment.
        </div>
      </div>
    );
  }

  const { incident } = analysis;
  const { causalChain, rootCause, confidence, observedSymptom } = incident;

  // Reverse the chain to show root → symptom (backend gives symptom → root)
  const reversedChain = [...causalChain].reverse();

  // Root cause is the last step in reversed chain
  const rootStep = reversedChain[reversedChain.length - 1];
  // Propagation steps are everything in between
  const propagationSteps = reversedChain.slice(1, -1);
  // Terminal effect is the first step (symptom)
  const terminalStep = reversedChain[0];

  return (
    <div className="mb-4 pb-4 border-b border-[var(--color-border-soft)]">
      <h3 className="text-xs font-semibold text-gray-400 tracking-wide mb-4">CAUSAL LINEAGE</h3>

      {/* Root Cause */}
      <CausalStepView 
        step={rootStep} 
        type="root"
        timestamp={rootStep.changeEvent?.timestamp}
      />

      {/* Propagation Steps */}
      {propagationSteps.map((step, index) => (
        <CausalStepView
          key={`prop-${step.stepNumber}`}
          step={step}
          type="propagation"
          timestamp={step.changeEvent?.timestamp}
        />
      ))}

      {/* Terminal Effect */}
      <CausalStepView 
        step={terminalStep} 
        type="terminal"
        timestamp={terminalStep.changeEvent?.timestamp}
        isLast 
      />
    </div>
  );
};
