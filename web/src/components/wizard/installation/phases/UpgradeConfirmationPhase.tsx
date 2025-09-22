import React, { useEffect } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../../contexts/AuthContext";
import { NextButtonConfig } from "../types";
import { State } from "../../../../types";
import { getApiBase } from '../../../../utils/api-base';

interface AppPreflightPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
}

// TODO Upgrade this component sole purpose is to trigger the installation. Once we add more phases it can be removed.
const UpgradeConfirmationPhase: React.FC<AppPreflightPhaseProps> = ({ onNext, setNextButtonConfig }) => {
  const { text, target, mode } = useWizard();
  const [error, setError] = React.useState<string | null>(null);
  const { token } = useAuth();

  const { mutate: startAppInstallation } = useMutation({
    mutationFn: async ({ ignoreAppPreflights }: { ignoreAppPreflights: boolean }) => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/app/${mode}`, {
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
    startAppInstallation({ ignoreAppPreflights: false }); // No need to ignore preflights
  };



  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: false,
      onClick: handleNextClick,
    });
  }, []);

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{text.appValidationTitle}</h2>
        <p className="text-gray-600 mt-1">{text.appValidationDescription}</p>
      </div>
      {error && <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">{error}</div>}
    </div>
  );
};

export default UpgradeConfirmationPhase;
