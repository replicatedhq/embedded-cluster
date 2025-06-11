import React from 'react';
import { Server, CheckCircle, XCircle, AlertTriangle, Loader2 } from 'lucide-react';

interface StatusIndicatorProps {
  title: string;
  status: 'Pending' | 'Running' | 'Succeeded' | 'Failed';
  themeColor: string;
}

const StatusIndicator: React.FC<StatusIndicatorProps> = ({ title, status, themeColor }) => {
  let Icon;
  let statusColor;
  let statusText;

  switch (status) {
    case 'Succeeded':
      Icon = CheckCircle;
      statusColor = 'rgb(34, 197, 94)'; // green-500
      statusText = 'Complete';
      break;
    case 'Failed':
      Icon = XCircle;
      statusColor = 'rgb(239, 68, 68)'; // red-500
      statusText = 'Failed';
      break;
    case 'Running':
      Icon = Loader2;
      statusColor = themeColor;
      statusText = 'Installing...';
      break;
    default:
      Icon = AlertTriangle;
      statusColor = 'rgb(156, 163, 175)'; // gray-400
      statusText = 'Pending';
  }

  return (
    <div className="flex items-center space-x-4 py-3">
      <div className="flex-shrink-0 text-gray-400">
        <Server className="w-5 h-5" />
      </div>
      <div className="flex-grow">
        <h4 className="text-sm font-medium text-gray-900">{title}</h4>
      </div>
      <div className="text-sm font-medium">
        <div className="flex items-center">
          <Icon style={{ color: statusColor }} className={`w-5 h-5 ${status === 'Running' ? 'animate-spin' : ''}`} />
          <span style={{ color: statusColor }} className="ml-2">{statusText}</span>
        </div>
      </div>
    </div>
  );
};

export default StatusIndicator;
