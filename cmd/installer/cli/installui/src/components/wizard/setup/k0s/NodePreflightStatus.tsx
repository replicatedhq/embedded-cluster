import React from 'react';
import { HostPreflightStatus } from '../../../../types';
import { XCircle } from 'lucide-react';

interface NodePreflightStatusProps {
  nodeId: string;
  status: HostPreflightStatus;
  progress: number;
  message: string;
  error?: string;
  themeColor: string;
}

const NodePreflightStatus: React.FC<NodePreflightStatusProps> = ({
  status,
  progress,
  message,
  error,
  themeColor,
}) => {
  const getFailedChecks = (status: HostPreflightStatus) => {
    return Object.entries(status)
      .filter(([_, result]) => result && !result.success)
      .map(([key, result]) => ({
        key,
        label: key.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase()),
        message: result?.message || ''
      }));
  };

  const failedChecks = getFailedChecks(status);
  const hasFailures = failedChecks.length > 0;

  return (
    <div className="space-y-2 mt-4 border-t border-gray-200 pt-4">
      <div className="mb-4">
        <div className="w-full bg-gray-200 rounded-full h-2">
          <div
            className="h-2 rounded-full transition-all duration-300"
            style={{
              width: `${progress}%`,
              backgroundColor: error ? 'rgb(239 68 68)' : themeColor,
            }}
          />
        </div>
        <p className="text-sm text-gray-500 mt-2">{message}</p>
      </div>

      {hasFailures && (
        <div className="space-y-3">
          {failedChecks.map(({ key, label, message }) => (
            <div key={key} className="flex items-start">
              <XCircle className="w-4 h-4 text-red-500 mt-0.5 mr-2 flex-shrink-0" />
              <div>
                <h5 className="text-sm font-medium text-red-800">{label}</h5>
                <p className="mt-1 text-sm text-red-700">{message}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

export default NodePreflightStatus;