import React, { useState } from "react";
import StepNavigation from "./StepNavigation";
import WelcomeStep from "./WelcomeStep";
import SetupStep from "./SetupStep";
import ValidationStep from "./ValidationStep";
import InstallationStep from "./InstallationStep";
import { WizardStep } from "../../types";
import { AppIcon } from "../common/Logo";
import { useWizardMode } from "../../contexts/WizardModeContext";
import CompletionStep from "./CompletionStep";

const InstallWizard: React.FC = () => {
  const [currentStep, setCurrentStep] = useState<WizardStep>("welcome");
  const { text } = useWizardMode();

  const goToNextStep = () => {
    const steps: WizardStep[] = ["welcome", "setup", "validation", "installation", "completion"];
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex < steps.length - 1) {
      setCurrentStep(steps[currentIndex + 1]);
    }
  };

  const goToPreviousStep = () => {
    const steps: WizardStep[] = ["welcome", "setup", "validation", "installation", "completion"];
    const currentIndex = steps.indexOf(currentStep);
    if (currentIndex > 0) {
      setCurrentStep(steps[currentIndex - 1]);
    }
  };

  const renderStep = () => {
    switch (currentStep) {
      case "welcome":
        return <WelcomeStep onNext={goToNextStep} />;
      case "setup":
        return <SetupStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "validation":
        return <ValidationStep onNext={goToNextStep} onBack={goToPreviousStep} />;
      case "installation":
        return <InstallationStep onNext={goToNextStep} />;
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
