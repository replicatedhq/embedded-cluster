import React from 'react';
import { WizardStep } from '../../types';
import { ClipboardList, Settings, Shield, Download, CheckCircle, Server } from 'lucide-react';
import { useWizard } from '../../contexts/WizardModeContext';
import { useSettings } from '../../contexts/SettingsContext';

interface StepNavigationProps {
  currentStep: WizardStep;
}

interface NavigationStep {
  id: WizardStep;
  name: string;
  icon: React.ElementType;
  hidden?: boolean;
  parentId?: WizardStep;
}

const StepNavigation: React.FC<StepNavigationProps> = ({ currentStep: currentStepId }) => {
  const { mode, target } = useWizard();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const getSteps = (): NavigationStep[] => {
    if (target === 'kubernetes') {
      return [
        { id: 'welcome', name: 'Welcome', icon: ClipboardList },
        { id: 'configuration', name: 'Configuration', icon: Server },
        { id: 'kubernetes-setup', name: 'Setup', icon: Settings },
        { id: 'kubernetes-installation', name: mode === 'upgrade' ? 'Upgrade' : 'Installation', icon: Download },
        { id: 'kubernetes-completion', name: 'Completion', icon: CheckCircle },
      ];
    } else {
      return [
        { id: 'welcome', name: 'Welcome', icon: ClipboardList },
        { id: 'configuration', name: 'Configuration', icon: Server },
        { id: 'linux-setup', name: 'Setup', icon: Settings },
        { id: 'linux-validation', name: 'Validation', icon: Shield, hidden: true, parentId: 'linux-setup' },
        { id: 'linux-installation', name: mode === 'upgrade' ? 'Upgrade' : 'Installation', icon: Download },
        { id: 'linux-completion', name: 'Completion', icon: CheckCircle },
      ];
    }
  }

  const steps = getSteps();
  const currentStep = steps.find(step => step.id === currentStepId);

  const getStepStatus = (step: NavigationStep) => {
    const stepIndex = steps.findIndex((s) => s.id === step.id);
    const currentIndex = steps.findIndex((s) => currentStep?.hidden ? s.id === currentStep.parentId : s.id === currentStepId);

    if (stepIndex < currentIndex) return 'complete';
    if (stepIndex === currentIndex) return 'current';
    return 'upcoming';
  };

  return (
    <nav aria-label="Progress">
      <ol className="space-y-4 md:flex md:space-y-0 md:space-x-8">
        {steps.filter(s => !s.hidden).map((step) => {
          const status = getStepStatus(step);
          const Icon = step.icon;

          return (
            <li key={step.id} className="md:flex-1">
              <div
                className={`flex items-center pl-4 py-2 text-sm font-medium rounded-md`}
                style={{
                  backgroundColor: status === 'complete' || status === 'current' ? `${themeColor}1A` : 'rgb(243 244 246)',
                  color: status === 'complete' || status === 'current' ? themeColor : 'rgb(107 114 128)',
                  border: status === 'current' ? `1px solid ${themeColor}` : undefined,
                }}
              >
                <span
                  className={`flex-shrink-0 w-8 h-8 flex items-center justify-center mr-3 rounded-full`}
                  style={{
                    backgroundColor: status === 'complete' || status === 'current' ? themeColor : 'rgb(229 231 235)',
                    color: status === 'complete' || status === 'current' ? 'white' : 'rgb(107 114 128)',
                  }}
                >
                  <Icon className="w-5 h-5" />
                </span>
                <span className="truncate">{step.name}</span>
              </div>
            </li>
          );
        })}
      </ol>
    </nav>
  );
};

export default StepNavigation;