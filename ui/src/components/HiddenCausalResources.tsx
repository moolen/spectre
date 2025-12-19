/**
 * Hidden Causal Resources Component
 * Displays resources that are part of the causal chain but not visible in the timeline
 */

import React from 'react';
import { SymptomResource, CausalStep } from '../types/rootCause';

interface HiddenCausalResourcesProps {
  resources: Array<{
    resource: SymptomResource;
    step: CausalStep;
  }>;
  onResourceClick?: (resource: SymptomResource) => void;
}

export const HiddenCausalResources: React.FC<HiddenCausalResourcesProps> = ({
  resources,
  onResourceClick
}) => {
  if (!resources || resources.length === 0) {
    return null;
  }

  return (
    <div className="border-t border-[var(--color-border-soft)] bg-[var(--color-surface-secondary)] p-4">
      <div className="mb-3">
        <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">
          Hidden Causal Resources
        </h3>
        <p className="text-xs text-gray-500">
          These resources are part of the causal chain but are currently filtered out or not visible in the timeline.
        </p>
      </div>

      <div className="space-y-2">
        {resources.map(({ resource, step }, index) => (
          <div
            key={`${resource.uid}-${index}`}
            className="flex items-center gap-3 p-2 rounded bg-[var(--color-app-bg)] border border-[var(--color-border-soft)] hover:border-blue-500/50 transition-colors cursor-pointer"
            onClick={() => onResourceClick?.(resource)}
          >
            {/* Step indicator */}
            <div className="flex-shrink-0 w-6 h-6 rounded-full bg-blue-500/20 flex items-center justify-center text-xs text-blue-400 font-mono">
              {step.stepNumber}
            </div>

            {/* Resource info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-xs font-mono text-[var(--color-text-primary)] truncate">
                  {resource.kind}
                </span>
                <span className="text-xs text-gray-500">/</span>
                <span className="text-xs text-[var(--color-text-primary)] truncate">
                  {resource.name}
                </span>
              </div>
              {resource.namespace && (
                <div className="text-xs text-gray-500">
                  namespace: {resource.namespace}
                </div>
              )}
            </div>

            {/* Relationship type badge */}
            <div className="flex-shrink-0">
              <span className="text-xs px-2 py-1 rounded bg-gray-700 text-gray-300 font-mono">
                {step.relationshipType}
              </span>
            </div>
          </div>
        ))}
      </div>

      <div className="mt-3 text-xs text-gray-500 italic">
        💡 Clear filters to see these resources in the timeline
      </div>
    </div>
  );
};
