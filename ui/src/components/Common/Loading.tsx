import React from 'react';

interface LoadingProps {
  message?: string;
  fullScreen?: boolean;
}

export const Loading: React.FC<LoadingProps> = ({
  message = 'Loading...',
  fullScreen = false,
}) => {
  const content = (
    <div className="flex flex-col items-center justify-center gap-4">
      <div className="w-12 h-12 border-4 border-gray-700 border-t-brand-500 rounded-full animate-spin" />
      <p className="text-gray-400 text-sm">{message}</p>
    </div>
  );

  if (fullScreen) {
    return (
      <div className="fixed inset-0 flex items-center justify-center bg-gray-950/50 backdrop-blur-sm z-50">
        {content}
      </div>
    );
  }

  return (
    <div className="flex items-center justify-center p-8">
      {content}
    </div>
  );
};
