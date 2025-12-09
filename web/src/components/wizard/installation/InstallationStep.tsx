import React, { useState, useCallback, useRef } from 'react';
import Card from '../../common/Card';
import Button from '../../common/Button';
import { useWizard } from '../../../contexts/WizardModeContext';
import { useSettings } from '../../../contexts/SettingsContext';
import { useInstallationProgress } from '../../../contexts/InstallationProgressContext';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import InstallationTimeline, { PhaseStatus } from './InstallationTimeline';
import LinuxPreflightPhase from './phases/LinuxPreflightPhase';
import AppPreflightPhase from './phases/AppPreflightPhase';
import LinuxInstallationPhase from './phases/LinuxInstallationPhase';
import KubernetesInstallationPhase from './phases/KubernetesInstallationPhase';
import AppInstallationPhase from './phases/AppInstallationPhase';
import AirgapPhase from './phases/AirgapPhase';
import { NextButtonConfig, BackButtonConfig } from './types';
import type { InstallationPhaseId as InstallationPhase } from '../../../types';
import type { components } from "../../../types/api";

type State = components["schemas"]["types.State"];

interface InstallationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const InstallationStep: React.FC<InstallationStepProps> = ({ onNext, onBack }) => {
  const { target, text, isAirgap, mode, requiresInfraUpgrade } = useWizard();
  const { settings } = useSettings();
  const { installationPhase: storedPhase, setInstallationPhase } = useInstallationProgress();
  const themeColor = settings.themeColor;

  const getPhaseOrder = (): InstallationPhase[] => {
    // Upgrade mode
    if (mode === 'upgrade') {
      const phases: InstallationPhase[] = [];

      // Add airgap processing if airgap
      if (isAirgap) {
        phases.push("airgap-processing");
      }

      // Add infrastructure upgrade if required
      if (requiresInfraUpgrade) {
        phases.push(`${target}-installation` as InstallationPhase);
      }

      // Always add app preflight and installation
      phases.push("app-preflight", "app-installation");

      return phases;
    }

    // Install mode
    if (target === 'kubernetes') {
      return ["kubernetes-installation", "app-preflight", "app-installation"];
    }

    const phases: InstallationPhase[] = ["linux-preflight", "linux-installation"];

    if (isAirgap) {
      phases.push("airgap-processing");
    }

    phases.push("app-preflight", "app-installation");

    return phases;
  };

  const phaseOrder = getPhaseOrder();
  const completedPhaseSet = new Set<InstallationPhase>();

  // If we have a stored phase then we need to set all the completed phases before too
  if (storedPhase) {
    const completedPhases = phaseOrder.slice(0, phaseOrder.indexOf(storedPhase))
    completedPhases.forEach(phase => completedPhaseSet.add(phase))
  }

  // If we have a stored phase use it
  const initialPhase = storedPhase || phaseOrder[0];

  // Initialize currentPhase from context or default to first phase
  const [currentPhase, setCurrentPhaseState] = useState<InstallationPhase>(initialPhase);

  // Selected phase for UI (can be current or any completed phase)
  const [selectedPhase, setSelectedPhase] = useState<InstallationPhase>(initialPhase);

  // Wrapper for setCurrentPhase that also updates context
  const setCurrentPhase = useCallback((phase: InstallationPhase) => {
    setCurrentPhaseState(phase);
    setInstallationPhase(phase);
  }, [setInstallationPhase]);
  const [completedPhases, setCompletedPhases] = useState<Set<InstallationPhase>>(completedPhaseSet);
  const [nextButtonConfig, setNextButtonConfig] = useState<NextButtonConfig | null>(null);
  const [backButtonConfig, setBackButtonConfig] = useState<BackButtonConfig | null>(null);
  const nextButtonRef = useRef<HTMLButtonElement>(null);
  const currentPhaseRef = useRef<InstallationPhase>(currentPhase);
  const [ignoreHostPreflights, setIgnoreHostPreflights] = useState(false);
  const [ignoreAppPreflights, setIgnoreAppPreflights] = useState(false);

  // Keep ref in sync with current phase
  currentPhaseRef.current = currentPhase;

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
    'airgap-processing': {
      status: 'Pending' as State,
      title: 'Air gap Bundle Processing',
      description: 'Processing air gap bundle',
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
    // But don't auto-advance on the final phase (user must click "Finish" button)
    if (phase === currentPhase && status === 'Succeeded') {
      const phaseIndex = phaseOrder.indexOf(phase);
      const isLastPhase = phaseIndex === phaseOrder.length - 1;

      if (!isLastPhase) {
        setTimeout(() => {
          // Check if we're still on the same phase when the timer fires
          // to prevent old timers from triggering advances on later phases
          if (currentPhaseRef.current === phase) {
            nextButtonRef.current?.click();
          }
        }, 500);
      }
    }
  }, [currentPhase, phaseOrder]);

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
      case 'linux-preflight':
        return <LinuxPreflightPhase {...commonProps} setIgnoreHostPreflights={setIgnoreHostPreflights} />;
      case 'linux-installation':
        return <LinuxInstallationPhase {...commonProps} ignoreHostPreflights={ignoreHostPreflights} />;
      case 'airgap-processing':
        return <AirgapPhase {...commonProps} />;
      case 'kubernetes-installation':
        return <KubernetesInstallationPhase {...commonProps} ignoreHostPreflights={ignoreHostPreflights} />;
      case 'app-preflight':
        return <AppPreflightPhase {...commonProps} setIgnoreAppPreflights={setIgnoreAppPreflights} />;
      case 'app-installation':
        return <AppInstallationPhase {...commonProps} ignoreAppPreflights={ignoreAppPreflights} />;
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

export default InstallationStep;
