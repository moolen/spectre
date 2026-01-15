import React, { useState } from 'react';

/**
 * Integration configuration
 */
interface Integration {
  id: string;
  name: string;
  description: string;
  icon: React.ReactNode;
  color: string;
}

/**
 * Available integrations
 */
const INTEGRATIONS: Integration[] = [
  {
    id: 'slack',
    name: 'Slack',
    description: 'Get real-time alerts and investigate incidents directly from Slack channels.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zM6.313 15.165a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zM8.834 6.313a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zM18.956 8.834a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zM17.688 8.834a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zM15.165 18.956a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zM15.165 17.688a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z" />
      </svg>
    ),
    color: '#4A154B',
  },
  {
    id: 'pagerduty',
    name: 'PagerDuty',
    description: 'Automatically create and enrich incidents with root cause analysis.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M16.965 1.18C15.085.164 13.769 0 10.683 0H3.73v14.55h6.926c2.743 0 4.8-.164 6.61-1.37 1.975-1.303 3.004-3.484 3.004-6.007 0-2.716-1.262-4.896-3.305-5.994zm-5.5 10.326h-4.21V3.113l3.977-.027c3.62-.028 5.43 1.316 5.43 4.128 0 3.073-2.075 4.292-5.196 4.292zM3.73 17.61h3.525V24H3.73z" />
      </svg>
    ),
    color: '#06AC38',
  },
  {
    id: 'grafana',
    name: 'Grafana',
    description: 'Visualize Spectre data in custom Grafana dashboards and panels.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M22.228 10.371c-.037-.466-.186-.858-.39-1.25-.167-.316-.39-.596-.65-.838a3.24 3.24 0 0 0-.817-.597c-.279-.149-.576-.261-.892-.336a4.09 4.09 0 0 0-.929-.112c-.149 0-.298.019-.446.037a7.537 7.537 0 0 0-.15-1.082 5.83 5.83 0 0 0-.334-1.082 5.14 5.14 0 0 0-.521-.985 5.27 5.27 0 0 0-.706-.856 5.615 5.615 0 0 0-.892-.707 5.835 5.835 0 0 0-1.007-.521 5.63 5.63 0 0 0-1.1-.317 6.17 6.17 0 0 0-1.155-.112c-.186 0-.372.019-.558.037-.186.019-.372.056-.558.093a8.757 8.757 0 0 0-.521-1.155 6.81 6.81 0 0 0-.706-.985 5.67 5.67 0 0 0-.892-.818 4.94 4.94 0 0 0-1.044-.596 4.72 4.72 0 0 0-1.155-.336C6.657.019 6.378 0 6.099 0c-.353 0-.706.037-1.044.112-.335.075-.651.186-.967.336-.316.149-.613.335-.873.558-.261.223-.502.465-.706.744-.204.279-.372.576-.502.892-.131.316-.224.651-.28.985-.055.335-.074.67-.055 1.007.019.335.075.67.167.985-.037.019-.074.019-.112.037a3.88 3.88 0 0 0-.91.391c-.28.167-.54.372-.763.615-.223.242-.409.521-.558.818-.149.298-.261.615-.316.948-.056.335-.075.67-.037 1.007.037.335.13.651.26.967.13.316.298.613.503.873.204.261.446.484.724.67a4.86 4.86 0 0 0-.26.948c-.056.335-.075.67-.056 1.007.019.335.074.67.167.985.093.316.223.614.39.892.167.279.372.54.596.763.223.223.484.428.762.596.279.167.576.316.892.409.316.093.65.167.985.186.335.019.67.019 1.007-.037.335-.056.65-.149.967-.28.056.056.112.112.167.167.316.279.651.521 1.007.726.354.204.744.371 1.155.502.409.13.836.223 1.28.279.446.056.892.074 1.34.056.446-.019.892-.075 1.32-.186.428-.112.837-.26 1.227-.446.39-.186.762-.409 1.1-.67.335-.261.632-.558.892-.892.186.019.372.019.558.019.447 0 .892-.056 1.32-.167.428-.112.836-.26 1.227-.447.39-.186.762-.409 1.1-.67.335-.261.632-.558.892-.892.26-.335.484-.706.65-1.1.167-.391.298-.8.372-1.229.074-.428.093-.873.056-1.302-.037-.428-.13-.855-.279-1.266a5.32 5.32 0 0 0-.502-1.155 5.61 5.61 0 0 0-.763-1.007z" />
      </svg>
    ),
    color: '#F46800',
  },
  {
    id: 'prometheus',
    name: 'Prometheus',
    description: 'Export metrics and correlate with Kubernetes events and changes.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M12 0C5.373 0 0 5.372 0 12c0 6.627 5.373 12 12 12s12-5.373 12-12c0-6.628-5.373-12-12-12zm0 22.46c-1.885 0-3.414-1.455-3.414-3.25h6.828c0 1.795-1.529 3.25-3.414 3.25zm5.64-4.357H6.36v-2.097h11.28v2.097zm-.04-3.127H6.4c-.03-.031-.06-.063-.089-.094-1.083-1.13-1.266-1.925-1.332-2.576-.068-.665-.034-1.048-.034-1.048h10.11s.034.383-.034 1.048c-.066.651-.249 1.446-1.332 2.576-.03.031-.059.063-.089.094zm-1.4-4.548c-.22 0-.398-.357-.398-.796 0-.44.178-.796.398-.796s.398.357.398.796c0 .44-.178.796-.398.796zm-8.4 0c-.22 0-.398-.357-.398-.796 0-.44.178-.796.398-.796s.398.357.398.796c0 .44-.178.796-.398.796z" />
      </svg>
    ),
    color: '#E6522C',
  },
  {
    id: 'logzio',
    name: 'Logz.io',
    description: 'Correlate log patterns with infrastructure changes and anomalies.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M12 0L1.5 6v12L12 24l10.5-6V6L12 0zm0 2.25l8.25 4.75v9.5L12 21.25 3.75 16.5V7L12 2.25zM12 6a6 6 0 100 12 6 6 0 000-12zm0 2a4 4 0 110 8 4 4 0 010-8z" />
      </svg>
    ),
    color: '#3A5EE9',
  },
  {
    id: 'betterstack',
    name: 'BetterStack',
    description: 'Unified incident management with uptime monitoring and on-call scheduling.',
    icon: (
      <svg viewBox="0 0 24 24" className="w-8 h-8" fill="currentColor">
        <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" strokeWidth="2" stroke="currentColor" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    ),
    color: '#2DD4BF',
  },
];

/**
 * Integration card
 */
const IntegrationCard: React.FC<{ integration: Integration }> = ({ integration }) => {

  return (
    <div
      className="relative p-6 bg-[var(--color-surface-elevated)] rounded-xl border border-[var(--color-border-soft)]
                 hover:border-[var(--color-border-default)] transition-all cursor-pointer group"
    >
      {/* Icon with colored background */}
      <div
        className="w-14 h-14 rounded-xl flex items-center justify-center mb-4 transition-transform group-hover:scale-105"
        style={{ backgroundColor: `${integration.color}20`, color: integration.color }}
      >
        {integration.icon}
      </div>

      {/* Content */}
      <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
        {integration.name}
      </h3>
      <p className="text-sm text-[var(--color-text-muted)] leading-relaxed">
        {integration.description}
      </p>

      {/* Status badge */}
      <div className="mt-4 inline-flex items-center gap-1.5 px-2 py-1 rounded-full bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)]">
        <span className="w-2 h-2 rounded-full bg-amber-500" />
        <span className="text-xs text-[var(--color-text-muted)]">Coming Soon</span>
      </div>
    </div>
  );
};

/**
 * IntegrationsPage - Mock integrations showcase
 */
export default function IntegrationsPage() {
  return (
    <div className="h-full overflow-y-auto bg-[var(--color-app-bg)]">
      <div className="max-w-6xl mx-auto p-8">
        {/* Header */}
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-[var(--color-text-primary)] mb-2">
            Integrations
          </h1>
          <p className="text-[var(--color-text-muted)] max-w-2xl">
            Connect Spectre with your existing tools to streamline incident response and enable seamless collaboration across your team.
          </p>
        </div>

        {/* Integration grid */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {INTEGRATIONS.map((integration) => (
            <IntegrationCard key={integration.id} integration={integration} />
          ))}
        </div>

        {/* Request integration section */}
        <div className="mt-12 p-6 bg-[var(--color-surface-muted)] rounded-xl border border-[var(--color-border-soft)] text-center">
          <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
            Missing an integration?
          </h3>
          <p className="text-sm text-[var(--color-text-muted)] mb-4">
            Let us know which tools you'd like to see integrated with Spectre.
          </p>
          <button
            disabled
            className="px-6 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium
                       opacity-50 cursor-not-allowed"
          >
            Request Integration
          </button>
        </div>
      </div>
    </div>
  );
}
