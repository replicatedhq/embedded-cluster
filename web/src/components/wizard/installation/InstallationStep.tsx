import React, { useState, useCallback } from 'react';
import Card from '../../common/Card';
import Button from '../../common/Button';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { State } from '../../../types';
import { ChevronRight } from 'lucide-react';
import InstallationTimeline, { InstallationPhaseId as InstallationPhase, PhaseStatus } from './InstallationTimeline';
import LinuxPreflightPhase from './phases/LinuxPreflightPhase';
import AppPreflightPhase from './phases/AppPreflightPhase';
import LinuxInstallationPhase from './phases/LinuxInstallationPhase';
import KubernetesInstallationPhase from './phases/KubernetesInstallationPhase';
import AppInstallationPhase from './phases/AppInstallationPhase';
import { NextButtonConfig } from './types';

interface InstallationStepProps {
  onNext: () => void;
}

const InstallationStep: React.FC<InstallationStepProps> = ({ onNext }) => {
  const { target, text } = useWizard();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const getPhaseOrder = (): InstallationPhase[] => {
    if (target === 'kubernetes') {
      return ["kubernetes-installation", "app-preflight", "app-installation"];
    }
    return ["linux-preflight", "linux-installation", "app-preflight", "app-installation"];
  };

  const phaseOrder = getPhaseOrder();
  const [currentPhase, setCurrentPhase] = useState<InstallationPhase>(phaseOrder[0]);
  const [selectedPhase, setSelectedPhase] = useState<InstallationPhase>(phaseOrder[0]);
  const [completedPhases, setCompletedPhases] = useState<Set<InstallationPhase>>(new Set());
  const [nextButtonConfig, setNextButtonConfig] = useState<NextButtonConfig | null>(null);

  const [phases, setPhases] = useState<Record<InstallationPhase, PhaseStatus>>(() => ({
    'linux-preflight': {
      status: 'Pending' as State,
      title: text.linuxValidationTitle,
      description: text.linuxValidationDescription,
    },
    'linux-installation': {
      status: 'Pending' as State,
      title: text.linuxInstallationTitle,
      description: text.linuxInstallationDescription,
    },
    'kubernetes-installation': {
      status: 'Pending' as State,
      title: text.kubernetesInstallationTitle,
      description: text.kubernetesInstallationDescription,
    },
    'app-preflight': {
      status: 'Pending' as State,
      title: text.appValidationTitle,
      description: text.appValidationDescription,
    },
    'app-installation': {
      status: 'Pending' as State,
      title: text.appInstallationTitle,
      description: text.appInstallationDescription,
    },
  }));

  const handleStateChange = useCallback((phase: InstallationPhase) => (status: State) => {
    setPhases(prev => ({
      ...prev,
      [phase]: { ...prev[phase], status }
    }));
  }, []);

  const goToNextPhase = () => {
    // Mark current phase as completed
    setCompletedPhases(prev => new Set([...prev, currentPhase]));

    // Move to next phase
    const currentIndex = phaseOrder.indexOf(currentPhase);
    if (currentIndex < phaseOrder.length - 1) {
      const nextPhase = phaseOrder[currentIndex + 1];
      setCurrentPhase(nextPhase);
      setSelectedPhase(nextPhase);
    } else {
      // All phases complete, go to next wizard step
      onNext();
    }
  };

  const getNextButtonText = () => {
    const currentIndex = phaseOrder.indexOf(currentPhase);
    if (currentIndex < phaseOrder.length - 1) {
      const nextPhase = phaseOrder[currentIndex + 1];
      return `Next: ${phases[nextPhase]?.title || 'Continue'}`;
    } else {
      return 'Next: Finish';
    }
  };

  const handlePhaseClick = (phase: InstallationPhase) => {
    // Only allow clicking on completed phases or current phase
    if (completedPhases.has(phase) || phase === currentPhase) {
      setSelectedPhase(phase);
    }
  };

  // No-op function for non-current steps
  const noOp = useCallback(() => {}, []);

  const renderPhaseContent = () => {
    // Only pass the real setNextButtonConfig to the currently selected phase that matches current phase
    const setBtnConfig = selectedPhase === currentPhase ? setNextButtonConfig : noOp;
    
    switch (selectedPhase) {
      case 'linux-preflight':
        return (
          <div className="h-full flex flex-col">
            <LinuxPreflightPhase 
              onNext={goToNextPhase} 
              setNextButtonConfig={setBtnConfig}
              onStateChange={handleStateChange('linux-preflight')}
            />
          </div>
        );
      case 'linux-installation':
        return (
          <div className="h-full flex flex-col">
            <LinuxInstallationPhase 
              onNext={goToNextPhase}
              setNextButtonConfig={setBtnConfig}
              onStateChange={handleStateChange('linux-installation')}
            />
          </div>
        );
      case 'kubernetes-installation':
        return (
          <div className="h-full flex flex-col">
            <KubernetesInstallationPhase 
              onNext={goToNextPhase}
              setNextButtonConfig={setBtnConfig}
              onStateChange={handleStateChange('kubernetes-installation')}
            />
          </div>
        );
      case 'app-preflight':
        return (
          <div className="h-full flex flex-col">
            <AppPreflightPhase 
              onNext={goToNextPhase}
              setNextButtonConfig={setBtnConfig}
              onStateChange={handleStateChange('app-preflight')}
            />
          </div>
        );
      case 'app-installation':
        return (
          <div className="h-full flex flex-col">
            <AppInstallationPhase 
              onNext={goToNextPhase}
              setNextButtonConfig={setBtnConfig}
              onStateChange={handleStateChange('app-installation')}
            />
          </div>
        );
      default:
        return (
          <div className="text-gray-600">
            Loading {selectedPhase} content...
          </div>
        );
    }
  };

  return (
    <div className="space-y-6">
      <Card className="p-0 overflow-hidden">
        <div className="flex min-h-[600px]">
          <InstallationTimeline
            phases={phases}
            currentPhase={currentPhase}
            selectedPhase={selectedPhase}
            onPhaseClick={handlePhaseClick}
            phaseOrder={phaseOrder}
            themeColor={themeColor}
          />

          <div className="flex-1 p-8">
            {renderPhaseContent()}
          </div>
        </div>
      </Card>

      {nextButtonConfig && (
        <div className="flex justify-end">
          <Button
            onClick={nextButtonConfig.onClick}
            disabled={nextButtonConfig.disabled}
            icon={<ChevronRight className="w-5 h-5" />}
          >
            {getNextButtonText()}
          </Button>
        </div>
      )}
    </div>
  );
};

export default InstallationStep;