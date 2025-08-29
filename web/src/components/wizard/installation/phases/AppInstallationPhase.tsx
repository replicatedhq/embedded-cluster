import React, { useState, useEffect, useCallback } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useAuth } from "../../../../contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { NextButtonConfig } from "../types";
import { State, AppInstallStatusResponse } from "../../../../types";
import InstallationProgress from '../shared/InstallationProgress';
import LogViewer from '../shared/LogViewer';
import StatusIndicator from '../shared/StatusIndicator';
import ErrorMessage from "../shared/ErrorMessage";

interface AppInstallationPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const AppInstallationPhase: React.FC<AppInstallationPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange }) => {
  const { text, target } = useWizard();
  const { settings } = useSettings();
  const { token } = useAuth();
  const [isPolling, setIsPolling] = useState(true);
  const [installationComplete, setInstallationComplete] = useState(false);
  const [installationSuccess, setInstallationSuccess] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;

  // Query to poll app installation status
  const { data: appInstallStatus, error: appStatusError } = useQuery<AppInstallStatusResponse, Error>({
    queryKey: ["appInstallationStatus"],
    queryFn: async () => {
      const response = await fetch(`/api/${target}/install/app/status`, {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || "Failed to get app installation status");
      }
      return response.json() as Promise<AppInstallStatusResponse>;
    },
    enabled: isPolling,
    refetchInterval: 2000,
  });

  const handleInstallationComplete = useCallback((success: boolean) => {
    setInstallationComplete(true);
    setInstallationSuccess(success);
    setIsPolling(false);
    onStateChange(success ? 'Succeeded' : 'Failed');
  }, []);

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Handle status changes
  useEffect(() => {
    if (appInstallStatus?.status?.state === "Succeeded") {
      handleInstallationComplete(true);
    } else if (appInstallStatus?.status?.state === "Failed") {
      handleInstallationComplete(false);
    }
  }, [appInstallStatus, handleInstallationComplete]);

  const getProgress = () => {
    const components = appInstallStatus?.components || [];
    if (components.length === 0) {
      return 0;
    }
    const completedComponents = components.filter(component => component.status?.state === 'Succeeded').length;
    return Math.round((completedComponents / components.length) * 100);
  }

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installationComplete || !installationSuccess,
      onClick: onNext,
    });
  }, [installationComplete, installationSuccess]);

  const renderApplicationPhase = () => (
    <div className="space-y-6">
      <InstallationProgress
        progress={getProgress()}
        currentMessage={appInstallStatus?.status?.description || ''}
        themeColor={themeColor}
        status={appInstallStatus?.status?.state}
      />

      <div className="space-y-2 divide-y divide-gray-200">
        {(appInstallStatus?.components || []).map((component, index) => (
          <StatusIndicator
            key={index}
            title={component.name}
            status={component.status?.state}
            themeColor={themeColor}
          />
        ))}
      </div>

      <LogViewer
        title="Application Installation Logs"
        logs={appInstallStatus?.logs ? [appInstallStatus.logs] : []}
        isExpanded={showLogs}
        onToggle={() => setShowLogs(!showLogs)}
      />

      {appStatusError && <ErrorMessage error={appStatusError?.message} />}
      {appInstallStatus?.status?.state === 'Failed' && <ErrorMessage error={appInstallStatus?.status?.description} />}
    </div>
  );

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.appInstallationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.appInstallationDescription}</p>
      </div>

      {renderApplicationPhase()}
    </div>
  );
};

export default AppInstallationPhase;
