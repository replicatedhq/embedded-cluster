import React, { useState, useEffect } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import { useQuery } from "@tanstack/react-query";
import { useConfig } from '../../contexts/ConfigContext';
import { useAuth } from "../../contexts/AuthContext";
import { InfraStatusResponse } from '../../types';
import { ChevronRight } from 'lucide-react';
import InstallationProgress from './installation/InstallationProgress';
import LogViewer from './installation/LogViewer';
import StatusIndicator from './installation/StatusIndicator';
import ErrorMessage from './installation/ErrorMessage';

interface InstallationStepProps {
  onNext: () => void;
}

const InstallationStep: React.FC<InstallationStepProps> = ({ onNext }) => {
  const { token } = useAuth();
  const { prototypeSettings } = useConfig();
  const [isInfraPolling, setIsInfraPolling] = useState(true);
  const [installComplete, setInstallComplete] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = prototypeSettings.themeColor;

  // Query to poll infra status
  const { data: infraStatusResponse, error: infraStatusError } = useQuery<InfraStatusResponse, Error>({
    queryKey: ["infraStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/infra/status", {
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

    let totalProgress = 0;
    const componentWeight = 100 / components.length;

    components.forEach(component => {
      let componentProgress = 0;

      switch (component.status?.state) {
        case 'Succeeded':
          componentProgress = 100;
          break;
        case 'Running':
          if (component.name === 'Runtime') {
            // Split Runtime's progress between installing and waiting phases
            const statusDescription = infraStatusResponse?.status?.description || '';
            if (statusDescription.includes('Installing Runtime')) {
              componentProgress = 25; // 25% of Runtime's weight = installing phase
            } else if (statusDescription.includes('Waiting for Runtime')) {
              componentProgress = 75; // 75% of Runtime's weight = waiting phase
            } else {
              componentProgress = 50; // Fallback for other Running states
            }
          } else if (component.name === 'Additional Components') {
            // Parse incremental progress for additional components
            const statusDescription = infraStatusResponse?.status?.description || '';
            const match = statusDescription.match(/Installing additional components \((\d+)\/(\d+)\)/);
            if (match) {
              const current = parseInt(match[1]);
              const total = parseInt(match[2]);
              if (total > 0) {
                // Calculate progress based on completed extensions + current extension progress
                const completedExtensions = current - 1; // Extensions that are done
                const currentExtensionProgress = 50; // Current extension gets 50% while running
                const totalProgress = (completedExtensions * 100) + currentExtensionProgress;
                componentProgress = totalProgress / total; // Average across all extensions
              } else {
                componentProgress = 50; // Fallback if parsing fails
              }
            } else {
              componentProgress = 50; // Fallback for other Running states
            }
          } else {
            componentProgress = 50; // Other components get 50% when Running
          }
          break;
        case 'Failed':
          componentProgress = 0; // No progress for failed components
          break;
        case 'Pending':
        default:
          componentProgress = 0; // No progress for pending components
          break;
      }

      totalProgress += (componentProgress / 100) * componentWeight;
    });

    return Math.round(totalProgress);
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

export default InstallationStep;
