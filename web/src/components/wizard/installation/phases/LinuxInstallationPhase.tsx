import React, { useState, useEffect } from 'react';
import { useQuery, useMutation } from "@tanstack/react-query";
import { useSettings } from '../../../../contexts/SettingsContext';
import { useAuth } from "../../../../contexts/AuthContext";
import { useWizard } from '../../../../contexts/WizardModeContext';
import { InfraStatusResponse, State } from '../../../../types';
import InstallationProgress from '../shared/InstallationProgress';
import LogViewer from '../shared/LogViewer';
import StatusIndicator from '../shared/StatusIndicator';
import ErrorMessage from '../shared/ErrorMessage';
import { NextButtonConfig, BackButtonConfig } from '../types';
import { getApiBase } from '../../../../utils/api-base';
import { ApiError } from '../../../../utils/api-error';

interface LinuxInstallationPhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const LinuxInstallationPhase: React.FC<LinuxInstallationPhaseProps> = ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange }) => {
  const { token } = useAuth();
  const { settings } = useSettings();
  const { mode, text } = useWizard();
  const [isInfraPolling, setIsInfraPolling] = useState(true);
  const [installComplete, setInstallComplete] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;

  // Query to poll infra status
  const { data: infraStatusResponse, error: infraStatusError } = useQuery<InfraStatusResponse, Error>({
    queryKey: ["infraStatus", mode],
    queryFn: async () => {
      const apiBase = getApiBase("linux", mode);
      const response = await fetch(`${apiBase}/infra/status`, {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to get infra status")
      }
      return response.json() as Promise<InfraStatusResponse>;
    },
    enabled: isInfraPolling,
    refetchInterval: 2000,
  });

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Handle infra status changes
  useEffect(() => {
    if (infraStatusResponse?.status?.state === "Failed") {
      setIsInfraPolling(false);
      onStateChange('Failed');
      return;
    }
    if (infraStatusResponse?.status?.state === "Succeeded") {
      setIsInfraPolling(false);
      setInstallComplete(true);
      onStateChange('Succeeded');
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

  // Mutation for starting app preflights
  const { mutate: startAppPreflights, error: appPreflightError } = useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase("linux", mode);
      const response = await fetch(`${apiBase}/app-preflights/run`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isUi: true }),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start app preflight checks")
      }
      return response.json();
    },
    onSuccess: () => {
      onNext();
    },
  });

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installComplete,
      onClick: () => startAppPreflights(),
    });
  }, [installComplete]);

  // Update back button configuration
  useEffect(() => {
    // Back button is hidden for linux-installation phase as the changes made in this phase are currently irreversible
    setBackButtonConfig({
      hidden: true,
      onClick: onBack,
    });
  }, [setBackButtonConfig, onBack]);

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

      {appPreflightError && <ErrorMessage error={appPreflightError?.message} />}
      {infraStatusError && <ErrorMessage error={infraStatusError?.message} />}
      {infraStatusResponse?.status?.state === 'Failed' && <ErrorMessage error={infraStatusResponse?.status?.description} />}
    </div>
  );


  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.linuxInstallationHeader}</h2>
        <p className="text-gray-600 mt-1">{text.linuxInstallationDescription}</p>
      </div>

      {renderInfrastructurePhase()}
    </div>
  );
};

export default LinuxInstallationPhase;
