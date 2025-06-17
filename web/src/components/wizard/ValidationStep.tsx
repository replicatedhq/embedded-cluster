import React, { useState } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { ChevronLeft, ChevronRight } from "lucide-react";
import LinuxPreflightCheck from "./preflight/LinuxPreflightCheck";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../contexts/AuthContext";
import { handleUnauthorized } from "../../utils/auth";

interface ValidationStepProps {
  onComplete: (success: boolean) => void;
  onBack: () => void;
}

const ValidationStep: React.FC<ValidationStepProps> = ({ onComplete, onBack }) => {
  const { text } = useWizardMode();
  const [preflightComplete, setPreflightComplete] = useState(false);
  const [preflightSuccess, setPreflightSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { token } = useAuth();

  // Mutation for starting the installation
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
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error("Session expired. Please log in again.");
        }
        throw errorData;
      }
      return response.json();
    },
    onSuccess: () => {
      onComplete(true);
    },
    onError: (err: Error) => {
      setError(err.message || "Failed to start installation");
      return err;
    },
  });

  const handlePreflightComplete = (success: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
  };

  const handleStartInstallation = () => {
    startInstallation();
  };

  return (
    <div className="space-y-6" data-testid="validation-step">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.setupTitle}</h2>
          <p className="text-gray-600 mt-1">
            Validate the host requirements before proceeding with installation.
          </p>
        </div>

        <LinuxPreflightCheck onComplete={handlePreflightComplete} />

        {error && (
          <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">
            {error}
          </div>
        )}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        
        <div className="flex justify-end flex-1">
          <Button
            onClick={handleStartInstallation}
            disabled={!preflightComplete || !preflightSuccess}
            icon={<ChevronRight className="w-5 h-5" />}
          >
            Next: Start Installation
          </Button>
        </div>
      </div>
    </div>
  );
};

export default ValidationStep; 