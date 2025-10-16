import React, { useEffect } from 'react';
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../../contexts/AuthContext";
import { useWizard } from '../../../../contexts/WizardModeContext';
import { State } from '../../../../types';
import ErrorMessage from '../shared/ErrorMessage';
import { NextButtonConfig, BackButtonConfig } from '../types';
import { getApiBase } from '../../../../utils/api-base';
import { ApiError } from '../../../../utils/api-error';

interface KubernetesInstallationPhaseProps {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onStateChange: (status: State) => void;
}

// TODO this is just a placeholder component to trigger the app preflights for the upgrade flow while we're missing the other phases of the upgrade
const UpgradeInstallationPhase: React.FC<KubernetesInstallationPhaseProps> = ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange }) => {
  const { token } = useAuth();
  const { mode, target } = useWizard();

  // Mutation for starting app preflights
  const { mutate: startAppPreflights, error: startAppPreflightsError } = useMutation({
    mutationFn: async () => {
      const apiBase = getApiBase(target, mode);
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
      onStateChange('Succeeded');
      onNext();
    },
  });

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Report failing state if there is an error starting app preflights
  useEffect(() => {
    if (startAppPreflightsError) {
      onStateChange('Failed');
    }
  }, [startAppPreflightsError]);

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: false,
      onClick: () => startAppPreflights(),
    });
  }, [setNextButtonConfig]);

  // Update back button configuration
  useEffect(() => {
    setBackButtonConfig({
      hidden: false,
      onClick: onBack,
    });
  }, [setBackButtonConfig, onBack]);

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Installation</h2>
        <p className="text-gray-600 mt-1">Start App Upgrade</p>
      </div>

      <div className="space-y-6">
        {startAppPreflightsError && <ErrorMessage error={startAppPreflightsError?.message} />}
      </div>
    </div>
  );
};

export default UpgradeInstallationPhase;
