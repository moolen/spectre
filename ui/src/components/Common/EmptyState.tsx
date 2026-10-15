import React from 'react';

interface EmptyStateProps {
  title: string;
  message: string;
  icon?: React.ReactNode;
  action?: {
    label: string;
    onClick: () => void;
  };
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  title,
  message,
  icon,
  action,
}) => {
  return (
    <div className="flex flex-col items-center justify-center p-8 text-center">
      {icon && <div className="mb-4 text-4xl text-[var(--color-text-faint)]">{icon}</div>}
      <h2 className="text-[13.5px] font-medium text-[var(--color-text-primary)] mb-2">{title}</h2>
      <p className="text-xs text-[var(--color-text-faint)] mb-4 max-w-sm">{message}</p>
      {action && (
        <button
          onClick={action.onClick}
          className="ui-btn-primary"
        >
          {action.label}
        </button>
      )}
    </div>
  );
};
