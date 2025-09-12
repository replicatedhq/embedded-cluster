import React, { useEffect, useMemo, useCallback } from "react";
import Button from "../../../common/Button";
import { Modal } from "../../../common/Modal";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { AlertTriangle } from "lucide-react";
import AppPreflightCheck from "./AppPreflightCheck";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../../contexts/AuthContext";
import { NextButtonConfig } from "../types";
import { State } from "../../../../types";

interface AppPreflightPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const AppPreflightPhase: React.FC<AppPreflightPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange }) => {
  const { text, target } = useWizard();
  const [preflightComplete, setPreflightComplete] = React.useState(false);
  const [preflightSuccess, setPreflightSuccess] = React.useState(false);
  const [allowIgnoreAppPreflights, setAllowIgnoreAppPreflights] = React.useState(false);
  const [hasStrictFailures, setHasStrictFailures] = React.useState(false);
  const [showPreflightModal, setShowPreflightModal] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const { token } = useAuth();

  const onRun = useCallback(() => {
    setPreflightComplete(false);
    setPreflightSuccess(false);
    setAllowIgnoreAppPreflights(false);
    setHasStrictFailures(false);
    onStateChange('Running');
  }, []);

  const onComplete = useCallback((success: boolean, allowIgnore: boolean, hasStrictFailures: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
    setAllowIgnoreAppPreflights(allowIgnore);
    setHasStrictFailures(hasStrictFailures);
    onStateChange(success ? 'Succeeded' : 'Failed');
  }, []);

  const { mutate: startAppInstallation } = useMutation({
    mutationFn: async ({ ignoreAppPreflights }: { ignoreAppPreflights: boolean }) => {
      const response = await fetch(`/api/${target}/install/app/install`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          ignoreAppPreflights: ignoreAppPreflights
        }),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || "Failed to start application installation");
      }
      return response.json();
    },
    onSuccess: () => {
      setError(null); // Clear any previous errors
      onNext();
    },
    onError: (err: Error) => {
      setError(err.message || "Failed to start application installation");
    },
  });

  const handleNextClick = () => {
    // If preflights passed, proceed normally
    if (preflightSuccess) {
      startAppInstallation({ ignoreAppPreflights: false }); // No need to ignore preflights
      return;
    }


    // Show warning modal if app preflights failed, none are strict, and button is enabled (allowIgnoreAppPreflights is true)
    if (!hasStrictFailures && allowIgnoreAppPreflights) {
      setShowPreflightModal(true);
    }
    // Note: This button will be disabled if hasStrictFailures is true or allowIgnoreAppPreflights is false
  };

  const handleCancelProceed = () => {
    setShowPreflightModal(false);
  };

  const handleConfirmProceed = () => {
    setShowPreflightModal(false);
    startAppInstallation({ ignoreAppPreflights: true }); // User confirmed they want to ignore preflight failures
  };

  const canProceed = useMemo(() => {
    // If preflights haven't completed yet, disable button
    if (!preflightComplete) {
      return false;
    }

    // If preflights passed, always allow proceeding
    if (preflightSuccess) {
      return true;
    }

    // If strict failures exist, never allow proceeding
    if (hasStrictFailures) {
      return false;
    }

    // If preflights failed, only allow proceeding if CLI flag was used
    return allowIgnoreAppPreflights;
  }, [preflightComplete, preflightSuccess, hasStrictFailures, allowIgnoreAppPreflights]);

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !canProceed,
      onClick: handleNextClick,
    });
  }, [canProceed]);

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.appValidationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.appValidationDescription}</p>
      </div>

      <AppPreflightCheck onRun={onRun} onComplete={onComplete} />

      {error && <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">{error}</div>}

      {showPreflightModal && (
        <Modal
          onClose={handleCancelProceed}
          title="Proceed with Failed Checks?"
          footer={
            <div className="flex space-x-3">
              <Button
                variant="outline"
                onClick={handleCancelProceed}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleConfirmProceed}
              >
                Continue Anyway
              </Button>
            </div>
          }
        >
          <div className="flex items-start space-x-3">
            <div className="shrink-0">
              <AlertTriangle className="h-6 w-6 text-amber-500" />
            </div>
            <div>
              <p className="text-sm text-gray-700">
                Some application preflight checks have failed. Continuing with the installation is likely to cause errors. Are you sure you want to proceed?
              </p>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default AppPreflightPhase;