import React, { useState } from "react";
import Card from "../../common/Card";
import Button from "../../common/Button";
import { useWizard } from "../../../contexts/WizardModeContext";
import { ChevronRight } from "lucide-react";
import AppInstallationStatus from "./AppInstallationStatus";

interface AppInstallationStepProps {
  onNext: () => void;
}

const AppInstallationStep: React.FC<AppInstallationStepProps> = ({ onNext }) => {
  const { text } = useWizard();
  const [installationComplete, setInstallationComplete] = useState(false);
  const [installationSuccess, setInstallationSuccess] = useState(false);

  const handleInstallationComplete = (success: boolean) => {
    setInstallationComplete(true);
    setInstallationSuccess(success);
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.appInstallationTitle}</h2>
          <p className="text-gray-600 mt-1">{text.appInstallationDescription}</p>
        </div>

        <AppInstallationStatus onComplete={handleInstallationComplete} />
      </Card>

      <div className="flex justify-end">
        <Button
          onClick={onNext}
          disabled={!installationComplete || !installationSuccess}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Finish
        </Button>
      </div>
    </div>
  );
};

export default AppInstallationStep;