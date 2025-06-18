import React from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { Modal } from "../common/Modal";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { ChevronLeft, ChevronRight, AlertTriangle } from "lucide-react";
import LinuxPreflightCheck from "./preflight/LinuxPreflightCheck";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../contexts/AuthContext";

interface ValidationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const ValidationStep: React.FC<ValidationStepProps> = ({ onNext, onBack }) => {
  const { text } = useWizardMode();
  const [preflightComplete, setPreflightComplete] = React.useState(false);
  const [preflightSuccess, setPreflightSuccess] = React.useState(false);
  const [allowIgnoreHostPreflights, setAllowIgnoreHostPreflights] = React.useState(false);
  const [showPreflightModal, setShowPreflightModal] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const { token } = useAuth();

  const handlePreflightComplete = (success: boolean, allowIgnore: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
    setAllowIgnoreHostPreflights(allowIgnore);
  };

  const { mutate: startInstallation } = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/install/infra/setup", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw errorData;
      }
      return response.json();
    },
    onSuccess: () => {
      onNext();
    },
    onError: (err: Error) => {
      setError(err.message || "Failed to start installation");
      return err;
    },
  });

  const handleNextClick = () => {
    // If preflights passed, proceed normally
    if (preflightSuccess) {
      startInstallation();
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
    startInstallation();
  };

  const canProceed = () => {
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
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.validationTitle}</h2>
          <p className="text-gray-600 mt-1">{text.validationDescription}</p>
        </div>

        <LinuxPreflightCheck onComplete={handlePreflightComplete} />

        {error && <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">{error}</div>}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button
          onClick={handleNextClick}
          disabled={!canProceed()}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Start Installation
        </Button>
      </div>

      <Modal
        isOpen={showPreflightModal}
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
          <div className="flex-shrink-0">
            <AlertTriangle className="h-6 w-6 text-amber-500" />
          </div>
          <div>
            <p className="text-sm text-gray-700">
              Some preflight checks have failed. Continuing with the installation is likely to cause errors. Are you sure you want to proceed?
            </p>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default ValidationStep;
