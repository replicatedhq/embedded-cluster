import React, { useState, useEffect } from 'react';
import { useQuery } from "@tanstack/react-query";
import { useSettings } from '../../../../contexts/SettingsContext';
import { useAuth } from "../../../../contexts/AuthContext";
import { InfraStatusResponse } from '../../../../types';
import InstallationProgress from '../shared/InstallationProgress';
import LogViewer from '../shared/LogViewer';
import StatusIndicator from '../shared/StatusIndicator';
import ErrorMessage from '../shared/ErrorMessage';

interface LinuxInstallationStepProps {
  onInfrastructureComplete: () => void;
}

const LinuxInstallationStep: React.FC<LinuxInstallationStepProps> = ({ onInfrastructureComplete }) => {
  const { token } = useAuth();
  const { settings } = useSettings();
  const [isInfraPolling, setIsInfraPolling] = useState(true);
  // const [installComplete, setInstallComplete] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;

  // Query to poll infra status
  const { data: infraStatusResponse, error: infraStatusError } = useQuery<InfraStatusResponse, Error>({
    queryKey: ["infraStatus"],
    queryFn: async () => {
      const response = await fetch("/api/linux/install/infra/status", {
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
      // setInstallComplete(true);
      onInfrastructureComplete();
    }
  }, [infraStatusResponse]);

  const getProgress = () => {
    const components = infraStatusResponse?.components || [];
    if (components.length === 0) {
      return 0;
    }
    const completedComponents = components.filter((component: any) => component.status?.state === 'Succeeded').length;
    return Math.round((completedComponents / components.length) * 100);
  }

  return (
    <div className="space-y-6">
      <InstallationProgress
        progress={getProgress()}
        currentMessage={infraStatusResponse?.status?.description || ''}
        themeColor={themeColor}
        status={infraStatusResponse?.status?.state}
      />

      <div className="space-y-2 divide-y divide-gray-200">
        {(infraStatusResponse?.components || []).map((component: any, index: number) => (
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
};

export default LinuxInstallationStep;
