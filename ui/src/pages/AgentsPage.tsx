import React from 'react';

/**
 * Mock data for previous agent conversations
 */
const PREVIOUS_AGENTS = [
  { id: 1, time: '1h ago', title: 'Certificate expiry investigation' },
  { id: 2, time: '3h ago', title: 'High latency in payment service' },
  { id: 3, time: '8h ago', title: 'Memory leak in auth-service' },
  { id: 4, time: '32h ago', title: 'Failed deployment rollback analysis' },
  { id: 5, time: '2d ago', title: 'Network policy misconfiguration' },
];

/**
 * Mock tool calls currently being executed
 */
const ACTIVE_TOOL_CALLS = [
  { name: 'cluster_health', status: 'running' },
  { name: 'resource_timeline', status: 'running' },
  { name: 'resource_timeline_changes', status: 'running' },
  { name: 'detect_anomalies', status: 'running' },
];

/**
 * Animated spinner component for tool calls
 */
const ToolSpinner: React.FC = () => (
  <div className="w-4 h-4 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
);

/**
 * Tool call card component
 */
const ToolCallCard: React.FC<{ name: string }> = ({ name }) => (
  <div className="flex items-center gap-3 px-4 py-3 bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)]">
    <ToolSpinner />
    <div className="flex flex-col">
      <span className="text-sm font-mono text-blue-400">{name}</span>
      <span className="text-xs text-[var(--color-text-muted)]">Executing...</span>
    </div>
  </div>
);

/**
 * Previous agent conversation card
 */
const AgentHistoryCard: React.FC<{ time: string; title: string }> = ({ time, title }) => (
  <div className="p-4 bg-[var(--color-surface-muted)] rounded-lg border border-[var(--color-border-soft)] hover:border-[var(--color-border-default)] transition-colors cursor-pointer">
    <div className="text-xs text-[var(--color-text-muted)] mb-1">{time}</div>
    <div className="text-sm text-[var(--color-text-primary)] line-clamp-2">{title}</div>
  </div>
);

/**
 * AgentsPage - Mock chat interface for AI agent
 *
 * This is a demonstration page showing what the agent interface will look like.
 * Features:
 * - Chat input at the bottom (disabled)
 * - User message with sample query
 * - Active tool calls with animations
 * - Right sidebar showing previous agent conversations
 */
export default function AgentsPage() {
  return (
    <div className="h-full flex bg-[var(--color-app-bg)]">
      {/* Main chat area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Chat content area */}
        <div className="flex-1 overflow-y-auto p-6">
          <div className="max-w-3xl mx-auto space-y-6">
            {/* User message */}
            <div className="flex gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center">
                <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </div>
              <div className="flex-1 bg-[var(--color-surface-elevated)] rounded-lg p-4 border border-[var(--color-border-soft)]">
                <div className="text-sm text-[var(--color-text-muted)] mb-1">You</div>
                <p className="text-[var(--color-text-primary)]">
                  cart-api starting to throw errors 30 minutes ago. Please give me all spec changes in the cluster 15 minutes before it started failing.
                </p>
              </div>
            </div>

            {/* Agent response with tool calls */}
            <div className="flex gap-3">
              <div className="flex-shrink-0 w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center">
                <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
              </div>
              <div className="flex-1 space-y-4">
                <div className="bg-[var(--color-surface-elevated)] rounded-lg p-4 border border-[var(--color-border-soft)]">
                  <div className="text-sm text-[var(--color-text-muted)] mb-2">Spectre Agent</div>
                  <p className="text-[var(--color-text-primary)] mb-4">
                    I'll help you investigate the errors. Let me check the cluster health and gather relevant information about spec changes in the time window you mentioned.
                  </p>
                  <div className="text-xs text-[var(--color-text-muted)] mb-3">Executing tools...</div>
                  <div className="grid grid-cols-2 gap-3">
                    {ACTIVE_TOOL_CALLS.map((tool) => (
                      <ToolCallCard key={tool.name} name={tool.name} />
                    ))}
                  </div>
                </div>

                {/* Thinking indicator */}
                <div className="flex items-center gap-2 text-[var(--color-text-muted)] text-sm">
                  <div className="flex gap-1">
                    <span className="w-2 h-2 bg-purple-500 rounded-full animate-bounce" style={{ animationDelay: '0ms' }} />
                    <span className="w-2 h-2 bg-purple-500 rounded-full animate-bounce" style={{ animationDelay: '150ms' }} />
                    <span className="w-2 h-2 bg-purple-500 rounded-full animate-bounce" style={{ animationDelay: '300ms' }} />
                  </div>
                  <span>Agent is analyzing...</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Chat input area (disabled) */}
        <div className="border-t border-[var(--color-border-soft)] p-4 bg-[var(--color-surface-muted)]">
          <div className="max-w-3xl mx-auto">
            <div className="relative">
              <textarea
                disabled
                placeholder="Ask the agent to investigate issues, analyze changes, or explain anomalies..."
                className="w-full px-4 py-3 pr-12 rounded-lg bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)]
                           text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] resize-none
                           opacity-50 cursor-not-allowed"
                rows={1}
              />
              <button
                disabled
                className="absolute right-3 bottom-5 p-1 rounded-lg bg-blue-600 opacity-50 cursor-not-allowed"
              >
                <svg className="w-4 h-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                </svg>
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* Right sidebar - Agent history */}
      <div className="w-72 border-l border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)] flex flex-col">
        <div className="p-4 border-b border-[var(--color-border-soft)]">
          <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">Previous Investigations</h2>
          <p className="text-xs text-[var(--color-text-muted)] mt-1">Your agent conversation history</p>
        </div>
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {PREVIOUS_AGENTS.map((agent) => (
            <AgentHistoryCard key={agent.id} time={agent.time} title={agent.title} />
          ))}
        </div>
        <div className="p-4 border-t border-[var(--color-border-soft)]">
          <button
            disabled
            className="w-full px-4 py-2 rounded-lg bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)]
                       text-[var(--color-text-muted)] text-sm opacity-50 cursor-not-allowed"
          >
            + New Investigation
          </button>
        </div>
      </div>
    </div>
  );
}
