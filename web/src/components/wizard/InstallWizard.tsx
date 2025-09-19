import React, { useState } from "react";
import StepNavigation from "./StepNavigation";
import WelcomeStep from "./WelcomeStep";
import ConfigurationStep from "./config/ConfigurationStep";
import LinuxSetupStep from "./setup/LinuxSetupStep";
import KubernetesSetupStep from "./setup/KubernetesSetupStep";
import InstallationStep from "./installation/InstallationStep";
import UpgradeStep from "./installation/UpgradeStep";
import LinuxCompletionStep from "./completion/LinuxCompletionStep";
import KubernetesCompletionStep from "./completion/KubernetesCompletionStep";
import { WizardStep } from "../../types";
import { AppIcon } from "../common/Logo";
import { useWizard } from "../../contexts/WizardModeContext";

const InstallWizard: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<WizardStep>("welcome");
  const { text, target, mode } = useWizard();
  let steps: WizardStep[] = []

  // TODO Upgrade
  // Iteration 1:
  // - Remove configuration step for upgrades for now
  // - There's no setup step for upgrades
  if (mode == "upgrade") {
    steps = ["welcome", "installation", `${target}-completion`]
  } else {
    // install steps
    steps = ["welcome", "configuration", `${target}-setup`, "installation", `${target}-completion`]
  }

  console.log(mode)
  console.log(steps)

  const goToNextStep = () => {
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex < steps.length - 1) {
      setCurrentStep(steps[currentIndex + 1]);
    }
  };

  const goToPreviousStep = () => {
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex > 0) {
      setCurrentStep(steps[currentIndex - 1]);
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case "welcome":
        return <WelcomeStep onNext={goToNextStep} />;
      case "configuration":
        return <ConfigurationStep onNext={goToNextStep} />;
      case "linux-setup":
        return <LinuxSetupStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "kubernetes-setup":
        return <KubernetesSetupStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "installation":
        // TODO Upgrade use a dedicated upgrade component while we work on making the upgrade flow similar to installation
        if (mode == "upgrade") {
          return <UpgradeStep onNext={goToNextStep} onBack={goToPreviousStep} />;
        }
        return <InstallationStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "linux-completion":
        return <LinuxCompletionStep />;
      case "kubernetes-completion":
        return <KubernetesCompletionStep />;
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

      <main className="grow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <StepNavigation currentStep={currentStep} enabledSteps={steps} />
          <div className="mt-8">{renderStep()}</div>
        </div>
      </main>
    </div>
  );
};

export default InstallWizard;
