import React, { useState, useEffect, useRef } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import { useConfig } from '../../contexts/ConfigContext';
import { InstallationStatus } from '../../types';
import { 
  ChevronRight, 
  CheckCircle, 
  XCircle, 
  AlertTriangle, 
  Loader2, 
  Database, 
  Server, 
  Package 
} from 'lucide-react';
import { installGitea } from '../../utils/installation';

interface InstallationStepProps {
  onNext: () => void;
}

const InstallationStep: React.FC<InstallationStepProps> = ({ onNext }) => {
  const { config } = useConfig();
  const [status, setStatus] = useState<InstallationStatus>({
    database: 'pending',
    core: 'pending',
    plugins: 'pending',
    overall: 'pending',
    currentMessage: '',
    logs: [],
    progress: 0,
  });
  const [isInstalling, setIsInstalling] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);

  const startInstallation = async () => {
    setIsInstalling(true);
    setStatus({
      database: 'pending',
      core: 'pending',
      plugins: 'pending',
      overall: 'in-progress',
      currentMessage: 'Preparing for installation...',
      logs: ['Starting Gitea Enterprise installation...'],
      progress: 0,
    });

    try {
      await installGitea(config, (newStatus) => {
        setStatus((prev) => ({
          ...prev,
          ...newStatus,
          logs: [...prev.logs, ...(newStatus.logs || [])],
        }));
      });
    } catch (error) {
      console.error('Installation error:', error);
      setStatus((prev) => ({
        ...prev,
        overall: 'failed',
        currentMessage: 'Installation failed',
        error: error instanceof Error ? error.message : 'Unknown error occurred',
      }));
    } finally {
      setIsInstalling(false);
    }
  };

  useEffect(() => {
    startInstallation();
  }, []);

  useEffect(() => {
    if (logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [status.logs]);

  const renderComponentStatus = (
    title: string,
    componentStatus: 'pending' | 'in-progress' | 'completed' | 'failed',
    icon: React.ReactNode
  ) => {
    let statusIcon;
    let statusColor;

    switch (componentStatus) {
      case 'completed':
        statusIcon = <CheckCircle className="w-5 h-5 text-green-500" />;
        statusColor = 'text-green-500';
        break;
      case 'failed':
        statusIcon = <XCircle className="w-5 h-5 text-red-500" />;
        statusColor = 'text-red-500';
        break;
      case 'in-progress':
        statusIcon = <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />;
        statusColor = 'text-blue-500';
        break;
      default:
        statusIcon = <AlertTriangle className="w-5 h-5 text-gray-400" />;
        statusColor = 'text-gray-400';
    }

    return (
      <div className="flex items-center space-x-4 py-3">
        <div className="flex-shrink-0 text-gray-500">{icon}</div>
        <div className="flex-grow">
          <h4 className="text-sm font-medium text-gray-900">{title}</h4>
        </div>
        <div className={`text-sm font-medium flex items-center ${statusColor}`}>
          {statusIcon}
          <span className="ml-2">
            {componentStatus === 'completed'
              ? 'Completed'
              : componentStatus === 'failed'
              ? 'Failed'
              : componentStatus === 'in-progress'
              ? 'Installing...'
              : 'Pending'}
          </span>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">Installing Gitea Enterprise</h2>
          <p className="text-gray-600 mt-1">
            Please wait while we install Gitea Enterprise in your Kubernetes cluster.
          </p>
        </div>

        <div className="mb-6">
          <div className="w-full bg-gray-200 rounded-full h-2.5">
            <div
              className={`h-2.5 rounded-full ${
                status.overall === 'failed' ? 'bg-red-500' : 'bg-[#2ECC71]'
              }`}
              style={{ width: `${status.progress}%` }}
            ></div>
          </div>
          <p className="text-sm text-gray-500 mt-2">
            {status.currentMessage || 'Preparing installation...'}
          </p>
        </div>

        <div className="space-y-2 divide-y divide-gray-200 mb-6">
          {renderComponentStatus('Database Installation', status.database, <Database className="w-5 h-5" />)}
          {renderComponentStatus('Gitea Core Installation', status.core, <Server className="w-5 h-5" />)}
          {renderComponentStatus('Plugins & Extensions', status.plugins, <Package className="w-5 h-5" />)}
        </div>

        <div className="mt-6">
          <h3 className="text-sm font-medium text-gray-900 mb-2">Installation Logs</h3>
          <div className="bg-gray-900 text-gray-200 rounded-md p-4 h-48 overflow-y-auto font-mono text-xs">
            {status.logs.map((log, index) => (
              <div key={index} className="whitespace-pre-wrap pb-1">
                {log}
              </div>
            ))}
            <div ref={logsEndRef} />
          </div>
        </div>

        {status.error && (
          <div className="mt-6 p-4 bg-red-50 text-red-800 rounded-md">
            <div className="flex">
              <div className="flex-shrink-0">
                <XCircle className="h-5 w-5 text-red-400" />
              </div>
              <div className="ml-3">
                <h3 className="text-sm font-medium">Installation Error</h3>
                <div className="mt-2 text-sm">
                  <p>{status.error}</p>
                </div>
              </div>
            </div>
          </div>
        )}
      </Card>

      <div className="flex justify-end">
        <Button
          onClick={onNext}
          disabled={status.overall !== 'completed'}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Finish
        </Button>
      </div>
    </div>
  );
};

export default InstallationStep;