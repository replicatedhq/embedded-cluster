import React, { useState, useEffect } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import AppInstallationStatus from "./AppInstallationStatus";
import { NextButtonConfig } from "../types";
import { State } from "../../../../types";

interface AppInstallationPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const AppInstallationPhase: React.FC<AppInstallationPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange }) => {
  const { text } = useWizard();
  const [installationComplete, setInstallationComplete] = useState(false);
  const [installationSuccess, setInstallationSuccess] = useState(false);

  const handleInstallationComplete = (success: boolean) => {
    setInstallationComplete(true);
    setInstallationSuccess(success);
    onStateChange(success ? 'Succeeded' : 'Failed');
  };

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !installationComplete || !installationSuccess,
      onClick: onNext,
    });
  }, [installationComplete, installationSuccess]);

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.appInstallationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.appInstallationDescription}</p>
      </div>

      <AppInstallationStatus onComplete={handleInstallationComplete} />
    </div>
  );
};

export default AppInstallationPhase;