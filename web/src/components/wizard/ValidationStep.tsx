import React from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { useMutation } from "@tanstack/react-query";
import LinuxPreflightCheck from "./preflight/LinuxPreflightCheck";
import { useAuth } from "../../contexts/AuthContext";

interface ValidationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const ValidationStep: React.FC<ValidationStepProps> = ({ onNext, onBack }) => {
  const { text } = useWizardMode();
  const { token } = useAuth();
  const [preflightComplete, setPreflightComplete] = React.useState(false);
  const [preflightSuccess, setPreflightSuccess] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

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
        const error = new Error("Failed to start installation") as Error & { status: number };
        error.status = response.status;
        throw error;
      }
      return response.json();
    },
    onSuccess: () => {
      onNext();
    },
    onError: (err: unknown) => {
      const errorMessage = err instanceof Error ? err.message : "Failed to start installation";
      setError(errorMessage);
    },
  });

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <h2 className="text-3xl font-bold text-gray-900">{text.validationTitle}</h2>
          <p className="text-xl text-gray-600 max-w-2xl mb-8">{text.validationDescription}</p>

          <LinuxPreflightCheck onComplete={handlePreflightComplete} />

          {error && (
            <div className="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg">
              <p className="text-red-700">{error}</p>
            </div>
          )}

          <div className="flex justify-between w-full mt-8">
            <Button onClick={onBack} variant="secondary" icon={<ChevronLeft className="w-5 h-5" />}>
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
      </Card>
    </div>
  );
};

export default ValidationStep;
