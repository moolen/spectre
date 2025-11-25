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
      {icon && <div className="mb-4 text-4xl text-gray-500">{icon}</div>}
      <h2 className="text-lg font-semibold text-gray-200 mb-2">{title}</h2>
      <p className="text-sm text-gray-400 mb-4 max-w-sm">{message}</p>
      {action && (
        <button
          onClick={action.onClick}
          className="px-4 py-2 bg-brand-600 hover:bg-brand-500 text-white rounded transition-colors text-sm font-medium"
        >
          {action.label}
        </button>
      )}
    </div>
  );
};
