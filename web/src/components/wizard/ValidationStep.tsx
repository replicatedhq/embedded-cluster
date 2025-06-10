import React from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { ChevronLeft, ChevronRight } from "lucide-react";
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
  const [error, setError] = React.useState<string | null>(null);
  const { token } = useAuth();

  const handlePreflightComplete = (success: boolean) => {
    setPreflightComplete(true);
    setPreflightSuccess(success);
  };

  const { mutate: startInstallation } = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/install/node/setup", {
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
          onClick={() => startInstallation()}
          disabled={!preflightComplete || !preflightSuccess}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Start Installation
        </Button>
      </div>
    </div>
  );
};

export default ValidationStep;
