import React, { useState, useEffect } from 'react';
import Card from '../../common/Card';
import Button from '../../common/Button';
import { useQuery } from "@tanstack/react-query";
import { useSettings } from '../../../contexts/SettingsContext';
import { useAuth } from "../../../contexts/AuthContext";
import { InfraStatusResponse } from '../../../types';
import { ChevronRight } from 'lucide-react';
import InstallationProgress from './shared/InstallationProgress';
import LogViewer from './shared/LogViewer';
import StatusIndicator from './shared/StatusIndicator';
import ErrorMessage from './shared/ErrorMessage';

interface KubernetesInstallationStepProps {
  onNext: () => void;
}

const KubernetesInstallationStep: React.FC<KubernetesInstallationStepProps> = ({ onNext }) => {
  const { token } = useAuth();
  const { settings } = useSettings();
  const [isInfraPolling, setIsInfraPolling] = useState(true);
  const [installComplete, setInstallComplete] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;

  // Query to poll infra status
  const { data: infraStatusResponse, error: infraStatusError } = useQuery<InfraStatusResponse, Error>({
    queryKey: ["infraStatus"],
    queryFn: async () => {
      const response = await fetch("/api/kubernetes/install/infra/status", {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || "Failed to get infra status");
      }
      return response.json() as Promise<InfraStatusResponse>;
    },
    enabled: isInfraPolling,
    refetchInterval: 2000,
  });

  // Handle infra status changes
  useEffect(() => {
    if (infraStatusResponse?.status?.state === "Failed") {
      setIsInfraPolling(false);
      return;
    }
    if (infraStatusResponse?.status?.state === "Succeeded") {
      setIsInfraPolling(false);
      setInstallComplete(true);
    }
  }, [infraStatusResponse]);

  const getProgress = () => {
    const components = infraStatusResponse?.components || [];
    if (components.length === 0) {
      return 0;
    }
    const completedComponents = components.filter(component => component.status?.state === 'Succeeded').length;
    return Math.round((completedComponents / components.length) * 100);
  }

  const renderInfrastructurePhase = () => (
    <div className="space-y-6">
      <InstallationProgress
        progress={getProgress()}
        currentMessage={infraStatusResponse?.status?.description || ''}
        themeColor={themeColor}
        status={infraStatusResponse?.status?.state}
      />

      <div className="space-y-2 divide-y divide-gray-200">
        {(infraStatusResponse?.components || []).map((component, index) => (
          <StatusIndicator 
            key={index}
            title={component.name} 
            status={component.status?.state}
            themeColor={themeColor}
          />
        ))}
      </div>

      <LogViewer
        title="Installation Logs"
        logs={infraStatusResponse?.logs ? [infraStatusResponse.logs] : []}
        isExpanded={showLogs}
        onToggle={() => setShowLogs(!showLogs)}
      />
      
      {infraStatusError && <ErrorMessage error={infraStatusError?.message} />}
      {infraStatusResponse?.status?.state === 'Failed' && <ErrorMessage error={infraStatusResponse?.status?.description} />}
    </div>
  );

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">Installation</h2>
          <p className="text-gray-600 mt-1">Installing infrastructure components</p>
        </div>

        {renderInfrastructurePhase()}
      </Card>

      <div className="flex justify-end">
        <Button
          onClick={onNext}
          disabled={!installComplete}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Finish
        </Button>
      </div>
    </div>
  );
};

export default KubernetesInstallationStep;
