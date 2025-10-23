import React, { useEffect, useMemo, useCallback } from "react";
import Button from "../../../common/Button";
import { Modal } from "../../../common/Modal";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { AlertTriangle } from "lucide-react";
import LinuxPreflightCheck from "./LinuxPreflightCheck";
import { NextButtonConfig, BackButtonConfig } from "../types";
import type { components } from "../../../../types/api";

type State = components["schemas"]["types.State"];

interface LinuxPreflightPhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
  setIgnoreHostPreflights: (ignore: boolean) => void;
}

const LinuxPreflightPhase: React.FC<LinuxPreflightPhaseProps> = ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange, setIgnoreHostPreflights }) => {
  const { text } = useWizard();
  const [preflightComplete, setPreflightComplete] = React.useState(false);
  const [preflightSuccess, setPreflightSuccess] = React.useState(false);
  const [allowIgnoreHostPreflights, setAllowIgnoreHostPreflights] = React.useState(false);
  const [showPreflightModal, setShowPreflightModal] = React.useState(false);

  const onRun = useCallback(() => {
    setPreflightComplete(false);
    setPreflightSuccess(false);
    setAllowIgnoreHostPreflights(false);
    onStateChange('Running');
  }, []);

  const onComplete = useCallback((success: boolean, allowIgnore: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
    setAllowIgnoreHostPreflights(allowIgnore);
    onStateChange(success ? 'Succeeded' : 'Failed');
  }, []);

  const handleNextClick = () => {
    // If preflights passed, proceed normally
    if (preflightSuccess) {
      setIgnoreHostPreflights(false); // No need to ignore preflights
      onNext();
      return;
    }

    // If preflights failed and button is enabled (allowIgnoreHostPreflights is true), show warning modal
    if (allowIgnoreHostPreflights) {
      setShowPreflightModal(true);
    }
    // Note: If allowIgnoreHostPreflights is false, button should be disabled (handled in canProceed)
  };

  const handleCancelProceed = () => {
    setShowPreflightModal(false);
  };

  const handleConfirmProceed = () => {
    setShowPreflightModal(false);
    setIgnoreHostPreflights(true); // User confirmed they want to ignore preflight failures
    onNext();
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

    // If preflights failed, only allow proceeding if CLI flag was used
    return allowIgnoreHostPreflights;
  }, [preflightComplete, preflightSuccess, allowIgnoreHostPreflights]);

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

  useEffect(() => {
    setBackButtonConfig({
      // Back button is always visible in linux-preflight phase until preflights succeed
      hidden: preflightSuccess,
      // Back button is only enabled when preflights are done running and failed
      disabled: !preflightComplete || preflightSuccess,
      onClick: onBack,
    });
  }, [preflightComplete, preflightSuccess, setBackButtonConfig, onBack]);

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.linuxValidationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.linuxValidationDescription}</p>
      </div>

      <LinuxPreflightCheck onRun={onRun} onComplete={onComplete} />

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
                Some preflight checks have failed. Continuing with the installation is likely to cause errors. Are you sure you want to proceed?
              </p>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default LinuxPreflightPhase;
