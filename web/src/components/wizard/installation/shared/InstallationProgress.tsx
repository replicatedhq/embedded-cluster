import React from 'react';
import { State } from '../../../../types';

interface InstallationProgressProps {
  progress: number;
  currentMessage: string;
  themeColor: string;
  status?: State;
}

const InstallationProgress: React.FC<InstallationProgressProps> = ({
  progress,
  currentMessage,
  themeColor,
  status
}) => {
  const truncateMessage = (message: string, maxLength: number = 250) => {
    return message.length > maxLength ? message.substring(0, maxLength) + '...' : message;
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
        {currentMessage ? truncateMessage(currentMessage) : 'Preparing installation...'}
      </p>
    </div>
  );
};

export default InstallationProgress;
