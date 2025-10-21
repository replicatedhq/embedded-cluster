import React, { useState, useEffect, useCallback, useRef } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useSettings } from "../../../../contexts/SettingsContext";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";
import { NextButtonConfig } from "../types";
import { State } from "../../../../types";
import ErrorMessage from "../shared/ErrorMessage";
import { useStartAppInstallation } from '../../../../mutations/useMutations';
import { useAppInstallStatus } from '../../../../queries/useQueries';

interface AppInstallationPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
  ignoreAppPreflights: boolean;
}

const AppInstallationPhase: React.FC<AppInstallationPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange, ignoreAppPreflights }) => {
  const { text } = useWizard();
  const { settings } = useSettings();
  const [isPolling, setIsPolling] = useState(true);
  const [installationComplete, setInstallationComplete] = useState(false);
  const [installationSuccess, setInstallationSuccess] = useState(false);
  const themeColor = settings.themeColor;
  const startAppInstallation = useStartAppInstallation();
  const mutationStarted = useRef(false);

  // Query to poll app installation status
  const { data: appInstallStatus, error: appStatusError } = useAppInstallStatus({
    enabled: isPolling,
    refetchInterval: 2000,
  });

  // Handle mutation callbacks
  useEffect(() => {
    if (startAppInstallation.isSuccess) {
      setIsPolling(true);
    }
    if (startAppInstallation.isError) {
      setIsPolling(false);
      onStateChange('Failed');
    }
  }, [startAppInstallation.isSuccess, startAppInstallation.isError]);

  // Auto-trigger mutation when status is Pending
  useEffect(() => {
    if (appInstallStatus?.status?.state === "Pending" && !mutationStarted.current) {
      mutationStarted.current = true;
      startAppInstallation.mutate({ ignoreAppPreflights });
    }
  }, [appInstallStatus?.status?.state]);

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

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installationComplete || !installationSuccess,
      onClick: onNext,
    });
  }, [installationComplete, installationSuccess, onNext]);

  const renderInstallationStatus = () => {
    // Loading state
    if (isPolling) {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="app-installation-loading">
          <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
          <p className="text-lg font-medium text-gray-900">{text.appInstallationLoadingTitle}</p>
          <p className="text-sm text-gray-500 mt-2" data-testid="app-installation-loading-description">
            {appInstallStatus?.status?.description || "Please wait while we install your application."}
          </p>
        </div>
      );
    }

    // Success state
    if (appInstallStatus?.status?.state === "Succeeded") {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="app-installation-success">
          <div
            className="w-12 h-12 rounded-full flex items-center justify-center mb-4"
            style={{ backgroundColor: `${themeColor}1A` }}
          >
            <CheckCircle className="w-6 h-6" style={{ color: themeColor }} />
          </div>
          <p className="text-lg font-medium text-gray-900">{text.appInstallationSuccessTitle}</p>
          <p className="text-sm text-gray-500 mt-2">Your application is now ready to use.</p>
        </div>
      );
    }

    // Error state
    if (appInstallStatus?.status?.state === "Failed") {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="app-installation-error">
          <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
            <XCircle className="w-6 h-6 text-red-600" />
          </div>
          <p className="text-lg font-medium text-gray-900">{text.appInstallationFailureTitle}</p>
          <p className="text-sm text-gray-500 mt-2" data-testid="app-installation-error-message">
            {appInstallStatus?.status?.description || "An error occurred during installation."}
          </p>
        </div>
      );
    }

    // Default loading state
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Preparing installation...</p>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.appInstallationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.appInstallationDescription}</p>
      </div>

      {renderInstallationStatus()}

      {startAppInstallation.error && <ErrorMessage error={startAppInstallation.error.message} />}
      {appStatusError && <ErrorMessage error={appStatusError?.message} />}
    </div>
  );
};

export default AppInstallationPhase;
