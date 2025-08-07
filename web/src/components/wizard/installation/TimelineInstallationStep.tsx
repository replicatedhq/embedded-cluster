import React, { useState, useMemo } from 'react';
import Card from '../../common/Card';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { State } from '../../../types';
import InstallationTimeline, { InstallationStep } from './InstallationTimeline';
import StepDetailPanel from './StepDetailPanel';
import LinuxValidationStep from '../validation/LinuxValidationStep';
import AppValidationStep from '../validation/AppValidationStep';
import LinuxInstallationStep from './LinuxInstallationStep';
import KubernetesInstallationStep from './KubernetesInstallationStep';
import AppInstallationStep from './AppInstallationStep';

interface TimelineInstallationStepProps {
  onNext: () => void;
}

const TimelineInstallationStep: React.FC<TimelineInstallationStepProps> = ({ onNext }) => {
  const { target, text } = useWizard();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const getStepOrder = (): InstallationStep[] => {
    if (target === 'kubernetes') {
      return ["kubernetes-installation", "app-validation", "app-installation"];
    }
    return ["linux-validation", "linux-installation", "app-validation", "app-installation"];
  };

  const stepOrder = getStepOrder();
  const [currentStep, setCurrentStep] = useState<InstallationStep>(stepOrder[0]);
  const [selectedStep, setSelectedStep] = useState<InstallationStep>(stepOrder[0]);
  const [completedSteps, setCompletedSteps] = useState<Set<InstallationStep>>(new Set());

  // Define step metadata
  const steps = useMemo(() => ({
    'linux-validation': {
      status: (completedSteps.has('linux-validation') ? 'Succeeded' :
        (currentStep === 'linux-validation' ? 'Running' : 'Pending')) as State,
      title: text.linuxValidationTitle,
      description: text.linuxValidationDescription,
    },
    'linux-installation': {
      status: (completedSteps.has('linux-installation') ? 'Succeeded' :
        (currentStep === 'linux-installation' ? 'Running' : 'Pending')) as State,
      title: text.linuxInstallationTitle,
      description: text.linuxInstallationDescription,
    },
    'kubernetes-installation': {
      status: (completedSteps.has('kubernetes-installation') ? 'Succeeded' :
        (currentStep === 'kubernetes-installation' ? 'Running' : 'Pending')) as State,
      title: text.kubernetesInstallationTitle,
      description: text.kubernetesInstallationDescription,
    },
    'app-validation': {
      status: (completedSteps.has('app-validation') ? 'Succeeded' :
        (currentStep === 'app-validation' ? 'Running' : 'Pending')) as State,
      title: text.appValidationTitle,
      description: text.appValidationDescription,
    },
    'app-installation': {
      status: (completedSteps.has('app-installation') ? 'Succeeded' :
        (currentStep === 'app-installation' ? 'Running' : 'Pending')) as State,
      title: text.appInstallationTitle,
      description: text.appInstallationDescription,
    },
  }), [currentStep, completedSteps, text]);

  const goToNextStep = () => {
    // Mark current step as completed
    setCompletedSteps(prev => new Set([...prev, currentStep]));

    // Move to next step
    const currentIndex = stepOrder.indexOf(currentStep);
    if (currentIndex < stepOrder.length - 1) {
      const nextStep = stepOrder[currentIndex + 1];
      setCurrentStep(nextStep);
      setSelectedStep(nextStep);
    } else {
      // All steps complete, go to next wizard step
      onNext();
    }
  };


  const handleStepClick = (step: InstallationStep) => {
    // Only allow clicking on completed steps or current step
    if (completedSteps.has(step) || step === currentStep) {
      setSelectedStep(step);
    }
  };

  const renderStepContent = () => {
    switch (selectedStep) {
      case 'linux-validation':
        return (
          <div className="h-full flex flex-col">
            <LinuxValidationStep onNext={goToNextStep} />
          </div>
        );
      case 'linux-installation':
        return (
          <div className="h-full flex flex-col">
            <LinuxInstallationStep onNext={goToNextStep} />
          </div>
        );
      case 'kubernetes-installation':
        return (
          <div className="h-full flex flex-col">
            <KubernetesInstallationStep onNext={goToNextStep} />
          </div>
        );
      case 'app-validation':
        return (
          <div className="h-full flex flex-col">
            <AppValidationStep onNext={goToNextStep} />
          </div>
        );
      case 'app-installation':
        return (
          <div className="h-full flex flex-col">
            <AppInstallationStep onNext={goToNextStep} />
          </div>
        );
      default:
        return (
          <div className="text-gray-600">
            Loading {selectedStep} content...
          </div>
        );
    }
  };

  return (
    <div className="space-y-6">
      <Card className="p-0 overflow-hidden">
        <div className="flex min-h-[600px]">
          <InstallationTimeline
            steps={steps}
            currentStep={currentStep}
            selectedStep={selectedStep}
            onStepClick={handleStepClick}
            stepOrder={stepOrder}
            themeColor={themeColor}
          />

          <div className="flex-1 p-8">
            {renderStepContent()}
          </div>
        </div>
      </Card>
    </div>
  );
};

export default TimelineInstallationStep;