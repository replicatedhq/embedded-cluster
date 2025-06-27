import React from "react";
import LinuxSetup from "./setup/LinuxSetup";
import KubernetesSetup from "./setup/KubernetesSetup";
import { useWizard } from "../../contexts/WizardModeContext";

interface SetupStepProps {
  onNext: () => void;
}

const SetupStep: React.FC<SetupStepProps> = ({ onNext }) => {
  const { target } = useWizard();
  return target === "kubernetes" ? (
    <KubernetesSetup onNext={onNext} />
  ) : (
    <LinuxSetup onNext={onNext} />
  );
};

export default SetupStep;
