import React from 'react';
import { WizardStep } from '../../types';
import { WizardMode } from '../../types/wizard-mode.ts';
import { ClipboardList, Settings, Download, CheckCircle, Server } from 'lucide-react';
import { useWizard } from '../../contexts/WizardModeContext';
import { useSettings } from '../../contexts/SettingsContext';

interface StepNavigationProps {
  currentStep: WizardStep;
  enabledSteps: WizardStep[];
}

interface NavigationStep {
  id: WizardStep;
  name: string;
  icon: React.ElementType;
  hidden?: boolean;
  parentId?: WizardStep;
}

const getNavigationSteps = (mode: WizardMode): NavigationStep[] => {
  return [
    { id: 'welcome', name: 'Welcome', icon: ClipboardList },
    { id: 'configuration', name: 'Configuration', icon: Server },
    { id: 'linux-setup', name: 'Setup', icon: Settings },
    { id: 'kubernetes-setup', name: 'Setup', icon: Settings },
    { id: 'installation', name: mode === 'upgrade' ? 'Upgrade' : 'Installation', icon: Download },
    { id: 'kubernetes-completion', name: 'Completion', icon: CheckCircle },
    { id: 'linux-completion', name: 'Completion', icon: CheckCircle },
  ]
}

const StepNavigation: React.FC<StepNavigationProps> = ({ currentStep: currentStepId, enabledSteps }) => {
  const { mode } = useWizard();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  // Get the navigation steps for this wizard mode and then filter them, removing steps the wizard isn't
  // configured to use
  const steps = getNavigationSteps(mode)
    .filter(({ id: navId }) => enabledSteps.includes(navId));
  console.log(steps)
  console.log(enabledSteps)

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
                  className={`shrink-0 w-8 h-8 flex items-center justify-center mr-3 rounded-full`}
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
