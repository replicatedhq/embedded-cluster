import React from 'react';
import { Server, CheckCircle, XCircle, AlertTriangle, Loader2 } from 'lucide-react';

interface StatusIndicatorProps {
  title: string;
  status: 'pending' | 'in-progress' | 'completed' | 'failed';
}

const StatusIndicator: React.FC<StatusIndicatorProps> = ({ title, status }) => {
  let Icon;
  let statusColor;
  let statusText;

  switch (status) {
    case 'completed':
      Icon = CheckCircle;
      statusColor = 'text-green-500';
      statusText = 'Complete';
      break;
    case 'failed':
      Icon = XCircle;
      statusColor = 'text-red-500';
      statusText = 'Failed';
      break;
    case 'in-progress':
      Icon = Loader2;
      statusColor = 'text-blue-500';
      statusText = 'Installing...';
      break;
    default:
      Icon = AlertTriangle;
      statusColor = 'text-gray-400';
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
          <Icon className={`w-5 h-5 ${statusColor} ${status === 'in-progress' ? 'animate-spin' : ''}`} />
          <span className={`ml-2 ${statusColor}`}>{statusText}</span>
        </div>
      </div>
    </div>
  );
};

export default StatusIndicator;