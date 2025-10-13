import React, { useState, useEffect, useCallback } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useAuth } from "../../../../contexts/AuthContext";
import { useQuery, useMutation } from "@tanstack/react-query";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";
import { NextButtonConfig, BackButtonConfig } from "../types";
import { State, InfraStatusResponse } from "../../../../types";
import { getApiBase } from '../../../../utils/api-base';
import ErrorMessage from "../shared/ErrorMessage";
import { ApiError } from '../../../../utils/api-error';
import LogViewer from "../shared/LogViewer";

interface InfraUpgradePhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const InfraUpgradePhase: React.FC<InfraUpgradePhaseProps> = ({
  onNext,
  onBack,
  setNextButtonConfig,
  setBackButtonConfig,
  onStateChange
}) => {
  const { target, mode } = useWizard();
  const { settings } = useSettings();
  const { token } = useAuth();
  const [isPolling, setIsPolling] = useState(false);
  const [upgradeStarted, setUpgradeStarted] = useState(false);
  const [upgradeComplete, setUpgradeComplete] = useState(false);
  const [upgradeSuccess, setUpgradeSuccess] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const themeColor = settings.themeColor;

  // Mutation for starting infrastructure upgrade
  const { mutate: startInfraUpgrade, error: startUpgradeError } = useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/infra/upgrade`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start infrastructure upgrade")
      }
      return response.json();
    },
    onSuccess: () => {
      setUpgradeStarted(true);
      setIsPolling(true);
      onStateChange('Running');
    },
  });

  // Query to poll infrastructure upgrade status
  const { data: infraStatus, error: statusError } = useQuery<InfraStatusResponse, Error>({
    queryKey: ["infraUpgradeStatus"],
    queryFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/infra/status`, {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to get infrastructure upgrade status")
      }
      return response.json() as Promise<InfraStatusResponse>;
    },
    enabled: isPolling,
    refetchInterval: 2000,
  });

  const handleUpgradeComplete = useCallback((success: boolean) => {
    setUpgradeComplete(true);
    setUpgradeSuccess(success);
    setIsPolling(false);
    onStateChange(success ? 'Succeeded' : 'Failed');
  }, [onStateChange]);

  // Report that step is pending when component mounts
  useEffect(() => {
    onStateChange('Pending');
  }, [onStateChange]);

  // Handle status changes
  useEffect(() => {
    if (infraStatus?.status?.state === "Succeeded") {
      handleUpgradeComplete(true);
    } else if (infraStatus?.status?.state === "Failed") {
      handleUpgradeComplete(false);
    }
  }, [infraStatus, handleUpgradeComplete]);

  // Update next button configuration
  useEffect(() => {
    if (!upgradeStarted) {
      setNextButtonConfig({
        disabled: false,
        onClick: () => startInfraUpgrade(),
      });
    } else {
      setNextButtonConfig({
        disabled: !upgradeComplete || !upgradeSuccess,
        onClick: onNext,
      });
    }
  }, [upgradeStarted, upgradeComplete, upgradeSuccess, onNext, setNextButtonConfig, startInfraUpgrade]);

  // Update back button configuration
  useEffect(() => {
    // Back button is hidden for infrastructure upgrade phase because there are no previous phases
    setBackButtonConfig({
      hidden: true,
      onClick: onBack,
    });
  }, [setBackButtonConfig, onBack]);

  const renderUpgradeStatus = () => {
    // Not started yet
    if (!upgradeStarted) {
      return (
        <div className="space-y-6" data-testid="infra-upgrade-ready">
          <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <p className="text-sm text-blue-800">
              Your infrastructure needs to be upgraded before proceeding with the application upgrade.
              This will upgrade Kubernetes, system addons, and extensions.
            </p>
          </div>
          <div className="flex items-center justify-center py-8">
            <p className="text-gray-600">
              Click "Next" to start the infrastructure upgrade.
            </p>
          </div>
        </div>
      );
    }

    // Loading state
    if (isPolling && infraStatus?.status?.state === "Running") {
      return (
        <div className="space-y-6" data-testid="infra-upgrade-running">
          <div className="flex flex-col items-center justify-center py-8">
            <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
            <p className="text-lg font-medium text-gray-900">Upgrading Infrastructure</p>
            <p className="text-sm text-gray-500 mt-2" data-testid="infra-upgrade-description">
              {infraStatus?.status?.description || "Please wait while we upgrade your infrastructure."}
            </p>
          </div>

          {/* Show component status if available */}
          {infraStatus?.components && infraStatus.components.length > 0 && (
            <div className="space-y-2">
              <h3 className="text-sm font-medium text-gray-700">Components:</h3>
              {infraStatus.components.map((component) => (
                <div
                  key={component.name}
                  className="flex items-center justify-between p-3 bg-gray-50 rounded-md"
                >
                  <span className="text-sm text-gray-700">{component.name}</span>
                  <span className={`text-xs font-medium ${component.status.state === 'Succeeded' ? 'text-green-600' :
                    component.status.state === 'Running' ? 'text-blue-600' :
                      component.status.state === 'Failed' ? 'text-red-600' :
                        'text-gray-500'
                    }`}>
                    {component.status.state}
                  </span>
                </div>
              ))}
            </div>
          )}

          {/* Show logs if available */}
          <LogViewer
            title="Upgrade Logs"
            logs={infraStatus?.logs ? [infraStatus.logs] : []}
            isExpanded={showLogs}
            onToggle={() => setShowLogs(!showLogs)}
          />
        </div>
      );
    }

    // Success state
    if (infraStatus?.status?.state === "Succeeded") {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="infra-upgrade-success">
          <div
            className="w-12 h-12 rounded-full flex items-center justify-center mb-4"
            style={{ backgroundColor: `${themeColor}1A` }}
          >
            <CheckCircle className="w-6 h-6" style={{ color: themeColor }} />
          </div>
          <p className="text-lg font-medium text-gray-900">Infrastructure Upgraded Successfully</p>
          <p className="text-sm text-gray-500 mt-2">Your infrastructure is now ready for the application upgrade.</p>
        </div>
      );
    }

    // Error state
    if (infraStatus?.status?.state === "Failed") {
      return (
        <div className="space-y-6" data-testid="infra-upgrade-error">
          <div className="flex flex-col items-center justify-center py-8">
            <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
              <XCircle className="w-6 h-6 text-red-600" />
            </div>
            <p className="text-lg font-medium text-gray-900">Infrastructure Upgrade Failed</p>
            <p className="text-sm text-gray-500 mt-2" data-testid="infra-upgrade-error-message">
              {infraStatus?.status?.description || "An error occurred during infrastructure upgrade."}
            </p>
          </div>

          {/* Show logs for debugging */}
          <LogViewer
            title="Upgrade Logs"
            logs={infraStatus?.logs ? [infraStatus.logs] : []}
            isExpanded={showLogs}
            onToggle={() => setShowLogs(!showLogs)}
          />
        </div>
      );
    }

    // Default loading state
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Preparing infrastructure upgrade...</p>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Infrastructure Upgrade</h2>
        <p className="text-gray-600 mt-1">Upgrade Kubernetes, system addons, and extensions</p>
      </div>

      {renderUpgradeStatus()}

      {(startUpgradeError || statusError) && (
        <ErrorMessage error={startUpgradeError?.message || statusError?.message || ''} />
      )}
    </div>
  );
};

export default InfraUpgradePhase;
