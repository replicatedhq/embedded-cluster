import React from 'react';
import { InstallationStatus } from '../../../types';

interface InstallationProgressProps {
  progress: number;
  currentMessage: string;
  themeColor: string;
  status?: 'failed' | 'success';
}

const InstallationProgress: React.FC<InstallationProgressProps> = ({
  progress,
  currentMessage,
  themeColor,
  status
}) => (
  <div className="mb-6">
    <div className="w-full bg-gray-200 rounded-full h-2.5">
      <div
        className="h-2.5 rounded-full transition-all duration-300"
        style={{
          backgroundColor: status === 'failed' ? 'rgb(239 68 68)' : themeColor,
          width: `${progress}%`,
        }}
      />
    </div>
    <p className="text-sm text-gray-500 mt-2">
      {currentMessage || 'Preparing installation...'}
    </p>
  </div>
);

export default InstallationProgress;