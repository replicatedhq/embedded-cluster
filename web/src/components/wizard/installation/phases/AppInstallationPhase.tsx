import React, { useState, useEffect, useCallback } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useAuth } from "../../../../contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";
import { NextButtonConfig, BackButtonConfig } from "../types";
import { State, AppInstallStatus } from "../../../../types";
import ErrorMessage from "../shared/ErrorMessage";

interface AppInstallationPhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const AppInstallationPhase: React.FC<AppInstallationPhaseProps> = ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange }) => {
  const { text, target } = useWizard();
  const { settings } = useSettings();
  const { token } = useAuth();
  const [isPolling, setIsPolling] = useState(true);
  const [installationComplete, setInstallationComplete] = useState(false);
  const [installationSuccess, setInstallationSuccess] = useState(false);
  const themeColor = settings.themeColor;

  // Query to poll app installation status
  const { data: appInstallStatus, error: appStatusError } = useQuery<AppInstallStatus, Error>({
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
      return response.json() as Promise<AppInstallStatus>;
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

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installationComplete || !installationSuccess,
      onClick: onNext,
    });
  }, [installationComplete, installationSuccess]);

  // Update back button configuration
  useEffect(() => {
    // Back button is hidden for app-installation phase since linux-preflight has already succeeded
    setBackButtonConfig({
      hidden: true,
      onClick: onBack,
    });
  }, [setBackButtonConfig, onBack]);

  const renderInstallationStatus = () => {
    // Loading state
    if (isPolling) {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="app-installation-loading">
          <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
          <p className="text-lg font-medium text-gray-900">Installing application...</p>
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
          <p className="text-lg font-medium text-gray-900">Application installed successfully!</p>
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
          <p className="text-lg font-medium text-gray-900">Application installation failed</p>
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

      {appStatusError && <ErrorMessage error={appStatusError?.message} />}
    </div>
  );
};

export default AppInstallationPhase;
