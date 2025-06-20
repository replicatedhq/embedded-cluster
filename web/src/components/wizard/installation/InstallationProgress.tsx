import React from 'react';

interface InstallationProgressProps {
  progress: number;
  currentMessage: string;
  themeColor: string;
  status?: 'Pending' | 'Running' | 'Succeeded' | 'Failed';
}

const InstallationProgress: React.FC<InstallationProgressProps> = ({
  progress,
  currentMessage,
  themeColor,
  status
}) => {
  const displayMessage = () => {
    if (!currentMessage) return 'Preparing installation…';
    return status === 'Running' ? `${currentMessage}…` : currentMessage;
  };

  return (
    <div className="mb-6">
      <div className="w-full bg-gray-200 rounded-full h-2.5">
        <div
          className="h-2.5 rounded-full transition-all duration-300"
          style={{
            backgroundColor: status === 'Failed' ? 'rgb(239 68 68)' : themeColor,
            width: `${progress}%`,
          }}
        />
      </div>
      <p className="text-sm text-gray-500 mt-2">
        {displayMessage()}
      </p>
    </div>
  );
};

export default InstallationProgress;
