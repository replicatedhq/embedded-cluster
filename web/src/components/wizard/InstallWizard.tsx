import React, { useState } from "react";
import StepNavigation from "./StepNavigation";
import WelcomeStep from "./WelcomeStep";
import LinuxSetupStep from "./setup/LinuxSetupStep";
import KubernetesSetupStep from "./setup/KubernetesSetupStep";
import LinuxValidationStep from "./validation/LinuxValidationStep";
import LinuxInstallationStep from "./installation/LinuxInstallationStep";
import { WizardStep } from "../../types";
import { AppIcon } from "../common/Logo";
import { useWizard } from "../../contexts/WizardModeContext";
import CompletionStep from "./CompletionStep";

const InstallWizard: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<WizardStep>("welcome");
  const { text, target } = useWizard();

  const getSteps = (): WizardStep[] => {
    if (target === "kubernetes") {
      return ["welcome", "kubernetes-setup", "kubernetes-installation", "completion"];
    } else {
      return ["welcome", "linux-setup", "linux-validation", "linux-installation", "completion"];
    }
  }

  const goToNextStep = () => {
    const steps = getSteps();
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex < steps.length - 1) {
      setCurrentStep(steps[currentIndex + 1]);
    }
  };

  const goToPreviousStep = () => {
    const steps = getSteps();
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex > 0) {
      setCurrentStep(steps[currentIndex - 1]);
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case "welcome":
        return <WelcomeStep onNext={goToNextStep} />;
      case "linux-setup":
        return <LinuxSetupStep onNext={goToNextStep} />;
      case "kubernetes-setup":
        return <KubernetesSetupStep onNext={goToNextStep} />;
      case "linux-validation":
        return <LinuxValidationStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "linux-installation":
        return <LinuxInstallationStep onNext={goToNextStep} />;
      // case "kubernetes-installation":
        // return <KubernetesInstallationStep onNext={goToNextStep} />;
      case "completion":
        return <CompletionStep />;
      default:
        return null;
    }
  };

  return (
    <div className="min-h-screen flex flex-col">
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between items-center py-4">
            <div className="flex items-center space-x-3">
              <AppIcon className="h-10 w-10" />
              <div>
                <h1 className="text-xl font-semibold text-gray-900">
                  {text.title}
                </h1>
                <p className="text-sm text-gray-500">{text.subtitle}</p>
              </div>
            </div>
          </div>
        </div>
      </header>

      <main className="flex-grow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <StepNavigation currentStep={currentStep} />
          <div className="mt-8">{renderStep()}</div>
        </div>
      </main>
    </div>
  );
};

export default InstallWizard;
