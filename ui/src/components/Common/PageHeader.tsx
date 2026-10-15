import React from 'react';

interface PageHeaderProps {
  /** Page title shown on the left */
  title: string;
  /** Optional muted context shown after the title (e.g. namespace) */
  context?: React.ReactNode;
  /** Main controls (query bar, filters) */
  children?: React.ReactNode;
  /** Right-aligned controls (time range, actions) */
  actions?: React.ReactNode;
  'data-testid'?: string;
}

/**
 * 52px top bar shared by all views: title on the left, controls in the middle,
 * time and actions on the right. Separated from the content by a hairline.
 */
export const PageHeader: React.FC<PageHeaderProps> = ({ title, context, children, actions, ...rest }) => (
  <header
    className="h-[52px] flex-none flex items-center gap-4 pl-5 pr-4 border-b border-[var(--color-border-soft)] bg-[var(--color-app-bg)] text-[var(--color-text-primary)] relative z-30"
    data-testid={rest['data-testid']}
  >
    <div className="flex items-center gap-2 whitespace-nowrap min-w-0">
      <h1 className="text-[13.5px] font-semibold tracking-[-0.005em]">{title}</h1>
      {context && (
        <>
          <span className="text-[var(--color-text-faint)]">/</span>
          <span className="text-[var(--color-text-secondary)] truncate">{context}</span>
        </>
      )}
    </div>
    {children && <div className="flex items-center gap-2 flex-1 min-w-0">{children}</div>}
    {actions && <div className="ml-auto flex items-center gap-2 flex-none">{actions}</div>}
  </header>
);

export default PageHeader;
