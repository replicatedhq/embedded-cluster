import React, { useState, useEffect, useRef } from 'react';
import { useSettings } from '../../../../contexts/SettingsContext';
import { useWizard } from '../../../../contexts/WizardModeContext';
import InstallationProgress from '../shared/InstallationProgress';
import LogViewer from '../shared/LogViewer';
import StatusIndicator from '../shared/StatusIndicator';
import ErrorMessage from '../shared/ErrorMessage';
import { NextButtonConfig, BackButtonConfig } from '../types';
import { useStartInfraSetup, useUpgradeInfra } from '../../../../mutations/useMutations';
import { useLinuxInfraStatus } from '../../../../queries/useQueries';
import type { components } from "../../../../types/api";

type State = components["schemas"]["types.State"];

interface LinuxInstallationPhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
  ignoreHostPreflights: boolean;
}

const LinuxInstallationPhase: React.FC<LinuxInstallationPhaseProps> = ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange, ignoreHostPreflights }) => {
  const { settings } = useSettings();
  const { mode, text } = useWizard();
  const [isInfraPolling, setIsInfraPolling] = useState(true);
  const [installComplete, setInstallComplete] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;
  const startInfraSetup = useStartInfraSetup({ ignoreHostPreflights });
  const upgradeInfra = useUpgradeInfra();
  const mutationStarted = useRef(false);

  // Use the appropriate mutation based on mode
  const infraMutation = mode === 'upgrade' ? upgradeInfra : startInfraSetup;

  // Query to poll infra status
  const { data: infraStatusResponse, error: infraStatusError } = useLinuxInfraStatus({
    enabled: isInfraPolling,
    refetchInterval: 2000,
  });

  // Handle mutation callbacks
  useEffect(() => {
    if (infraMutation.isSuccess) {
      setIsInfraPolling(true);
    }
    if (infraMutation.isError) {
      setIsInfraPolling(false);
      onStateChange('Failed');
    }
  }, [infraMutation.isSuccess, infraMutation.isError]);

  // Auto-trigger mutation when status is Pending
  useEffect(() => {
    if (infraStatusResponse?.status?.state === "Pending" && !mutationStarted.current) {
      mutationStarted.current = true;
      infraMutation.mutate();
    }
  }, [infraStatusResponse?.status?.state]);

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

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installComplete,
      onClick: onNext,
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

      {infraMutation.error && <ErrorMessage error={infraMutation.error.message} />}
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
