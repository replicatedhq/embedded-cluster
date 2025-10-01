import React, { useState, useCallback, useRef } from 'react';
import Card from '../../common/Card';
import Button from '../../common/Button';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { State } from '../../../types';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import InstallationTimeline, { InstallationPhaseId as InstallationPhase, PhaseStatus } from './InstallationTimeline';
import AppPreflightPhase from './phases/AppPreflightPhase';
import AppInstallationPhase from './phases/AppInstallationPhase';
import { NextButtonConfig, BackButtonConfig } from './types';

interface InstallationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const UpgradeStep: React.FC<InstallationStepProps> = ({ onNext, onBack }) => {
  const { text } = useWizard();
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const getPhaseOrder = (): InstallationPhase[] => {
    // Iteration 3: Include app preflights before app installation
    return ["app-preflight", "app-installation"];
  };

  const phaseOrder = getPhaseOrder();
  const [currentPhase, setCurrentPhase] = useState<InstallationPhase>(phaseOrder[0]);
  const [selectedPhase, setSelectedPhase] = useState<InstallationPhase>(phaseOrder[0]);
  const [completedPhases, setCompletedPhases] = useState<Set<InstallationPhase>>(new Set());
  const [nextButtonConfig, setNextButtonConfig] = useState<NextButtonConfig | null>(null);
  const [backButtonConfig, setBackButtonConfig] = useState<BackButtonConfig | null>(null);
  const nextButtonRef = useRef<HTMLButtonElement>(null);

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

    // Auto-advance to next phase when current phase succeeds
    if (phase === currentPhase && status === 'Succeeded') {
      setTimeout(() => {
        nextButtonRef.current?.click();
      }, 500);
    }
  }, [currentPhase]);

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
      return `Next: ${phases[nextPhase]?.title}`;
    } else {
      return 'Next: Finish';
    }
  };

  const canSelectPhase = (phase: InstallationPhase) => {
    // Only allow clicking on completed phases or current phase
    return completedPhases.has(phase) || phase === currentPhase;
  };

  const handlePhaseClick = (phase: InstallationPhase) => {
    if (canSelectPhase(phase)) {
      setSelectedPhase(phase);
    }
  };

  // No-op function for non-current steps
  const noOp = useCallback(() => { }, []);

  const renderPhase = (phase: InstallationPhase) => {
    const commonProps = {
      onNext: goToNextPhase,
      onBack,
      // Only pass the real setNextButtonConfig to the current phase
      setNextButtonConfig: phase === currentPhase ? setNextButtonConfig : noOp,
      // Only pass the real setBackButtonConfig to the current phase
      setBackButtonConfig: phase === currentPhase ? setBackButtonConfig : noOp,
      onStateChange: handleStateChange(phase)
    };

    switch (phase) {
      case 'app-preflight':
        return <AppPreflightPhase {...commonProps} />;
      case 'app-installation':
        return <AppInstallationPhase {...commonProps} />;
      default:
        return (
          <div className="text-gray-600">
            Loading {phase} content...
          </div>
        );
    }
  };

  const renderPhases = () => {
    // Render all completed and current phases to preserve component state and polling logic.
    // This prevents unmounting when users switch between phases, which would stop React Query
    // polling intervals and lose component state. Non-selected phases are simply hidden.
    // Future phases are excluded to avoid triggering mutations on mount for phases that haven't started yet.
    return phaseOrder.map(phase => {
      if (!canSelectPhase(phase)) {
        return null;
      };

      return (
        <div
          key={phase}
          data-testid={`${phase}-container`}
          className={`h-full flex flex-col ${phase === selectedPhase ? 'block' : 'hidden'}`}
        >
          {renderPhase(phase)}
        </div>
      );
    });
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
            {renderPhases()}
          </div>
        </div>
      </Card>

      <div className="flex justify-between">
        {backButtonConfig && !backButtonConfig.hidden && (
          <Button
            onClick={backButtonConfig.onClick}
            variant="outline"
            disabled={backButtonConfig.disabled ?? false}
            icon={<ChevronLeft className="w-5 h-5" />}
            dataTestId="installation-back-button"
          >
            Back
          </Button>
        )}
        {nextButtonConfig && (
          <Button
            ref={nextButtonRef}
            onClick={nextButtonConfig.onClick}
            disabled={nextButtonConfig.disabled}
            icon={<ChevronRight className="w-5 h-5" />}
            dataTestId="installation-next-button"
            className={(!backButtonConfig || backButtonConfig.hidden) ? 'ml-auto' : ''}
          >
            {getNextButtonText()}
          </Button>
        )}
      </div>
    </div>
  );
};

export default UpgradeStep;
