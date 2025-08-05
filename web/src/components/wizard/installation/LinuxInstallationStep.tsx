import React, { useState, useEffect } from 'react';
import Card from '../../common/Card';
import Button from '../../common/Button';
import { Modal } from '../../common/Modal';
import { useSettings } from '../../../contexts/SettingsContext';
// import { useWizardMode } from '../../../contexts/WizardModeContext';
import { ValidationStatus, InstallationStatus } from '../../../types';
import { ChevronRight, ChevronLeft, AlertTriangle } from 'lucide-react';
// import { setupInfrastructure } from '../../utils/infrastructure';
// import { validateEnvironment } from '../../utils/validation';
// import { installWordPress } from '../../utils/wordpress';
import InstallationTimeline, { InstallationStep, StepStatus } from './shared/InstallationTimeline';
import StepDetailPanel from './shared/StepDetailPanel';
import LogViewer from './shared/LogViewer';
// import K0sInstallation from './setup/K0sInstallation';

interface LinuxInstallationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const LinuxInstallationStep: React.FC<LinuxInstallationStepProps> = ({ onNext, onBack }) => {
  // const { config, prototypeSettings } = useConfig();
  // const { text } = useWizardMode();
  // const isLinuxMode = prototypeSettings.clusterMode === 'embedded';
  const { settings } = useSettings();
  const themeColor = settings.themeColor;

  const [currentStep, setCurrentStep] = useState<InstallationStep>(
    // isLinuxMode ? 'hosts' : 'preflights'
    'hosts'
  );

  const [selectedStep, setSelectedStep] = useState<InstallationStep>(
    // isLinuxMode ? 'hosts' : 'preflights'
    'hosts'
  );

  const [steps, setSteps] = useState<Record<InstallationStep, StepStatus>>({
    hosts: {
      status: 'running',
      title: 'Host Setup',
      description: 'Installing runtime and setting up hosts',
      progress: 0
    },
    infrastructure: {
      status: 'pending',
      title: 'Infrastructure Setup',
      description: 'Installing storage, registry, and disaster recovery components',
      progress: 0
    }
    // preflights: {
    //   status: 'pending',
    //   title: 'Preflight Checks',
    //   description: 'Validating environment requirements',
    //   progress: 0
    // },
    // application: {
    //   status: 'pending',
    //   title: 'Application Installation',
    //   description: 'Installing database, core, and plugins',
    //   progress: 0
    // }
  });

  const [infrastructureStatus, setInfrastructureStatus] = useState<InstallationStatus>({
    openebs: 'pending',
    registry: 'pending',
    velero: 'pending',
    components: 'pending',
    database: 'pending',
    core: 'pending',
    plugins: 'pending',
    overall: 'pending',
    currentMessage: '',
    logs: [],
    progress: 0,
  });

  const [applicationStatus, setApplicationStatus] = useState<InstallationStatus>({
    openebs: 'pending',
    registry: 'pending',
    velero: 'pending',
    components: 'pending',
    database: 'pending',
    core: 'pending',
    plugins: 'pending',
    overall: 'pending',
    currentMessage: '',
    logs: [],
    progress: 0,
  });

  const [validationResults, setValidationResults] = useState<ValidationStatus>({
    kubernetes: null,
    helm: null,
    storage: null,
    networking: null,
    permissions: null,
  });

  const [showLogs, setShowLogs] = useState(true); //screspod: set this to false after testing
  const [showPreflightModal, setShowPreflightModal] = useState(false);
  const [allLogs, setAllLogs] = useState<string[]>([]);
  const [installationComplete, setInstallationComplete] = useState(false);
  const [hostsComplete, setHostsComplete] = useState(false);

  // Start the appropriate first step
  useEffect(() => {
      startHostSetup();
  }, []);

  const updateStepStatus = (step: InstallationStep, updates: Partial<StepStatus>) => {
    setSteps(prev => ({
      ...prev,
      [step]: { ...prev[step], ...updates }
    }));
  };

  const handleStepClick = (step: InstallationStep) => {
    // Only allow clicking on steps that have started (not pending)
    if (steps[step].status !== 'pending') {
      setSelectedStep(step);
    }
  };

  // Auto-update selected step when current step changes
  useEffect(() => {
    setSelectedStep(currentStep);
  }, [currentStep]);

  const addToAllLogs = (newLogs: string[]) => {
    setAllLogs(prev => [...prev, ...newLogs]);
  };

  const startHostSetup = async () => {
    updateStepStatus('hosts', { status: 'running' });
    setCurrentStep('hosts');

    // screspod: remove this
    await new Promise(resolve => setTimeout(resolve, 4000));
    handleHostsComplete(false)
    // Host setup is handled by the K0sInstallation component
  };

  const handleHostsComplete = (hasFailures: boolean = false) => {
    setHostsComplete(true);
    updateStepStatus('hosts', { status: hasFailures ? 'failed' : 'completed' });
    // Don't auto-proceed - let user manually click Next when ready
  };

  const startInfrastructureSetup = async () => {
    updateStepStatus('infrastructure', { status: 'running' });
    setCurrentStep('infrastructure');

    // screspod: remove this
    await new Promise(resolve => setTimeout(resolve, 4000));
    updateStepStatus('infrastructure', { status: 'completed' });
    addToAllLogs(["infrastructure setup logs"]);
    // Auto-proceed to preflights
    // setTimeout(() => {
    //   startPreflightChecks();
    // }, 500);

    // try {
    //   await setupInfrastructure(config, (newStatus) => {
    //     setInfrastructureStatus(prev => {
    //       const updated = { ...prev, ...newStatus };
    //       updateStepStatus('infrastructure', { progress: updated.progress });

    //       if (newStatus.logs) {
    //         addToAllLogs(newStatus.logs);
    //       }

    //       return updated;
    //     });
    //   });

    //   updateStepStatus('infrastructure', { status: 'completed' });

    //   // Auto-proceed to preflights
    //   setTimeout(() => {
    //     startPreflightChecks();
    //   }, 500);

    // } catch (error) {
    //   console.error('Infrastructure setup error:', error);
    //   updateStepStatus('infrastructure', {
    //     status: 'failed',
    //     error: 'Infrastructure setup failed'
    //   });
    // }
  };

  // const startPreflightChecks = async () => {
  //   updateStepStatus('preflights', { status: 'running' });
  //   setCurrentStep('preflights');

  //   // screspod: remove this after testing
  //   await new Promise(resolve => setTimeout(resolve, 4000));
  //   updateStepStatus('preflights', { status: 'completed' });
  //   addToAllLogs(["preflight checks complete"]);
  //   setTimeout(() => {
  //     startApplicationInstallation();
  //   }, 500);
  // };

  // const startApplicationInstallation = async () => {
  //   updateStepStatus('application', { status: 'running' });
  //   setCurrentStep('application');

  //   // screspod: remove this after testing
  //   await new Promise(resolve => setTimeout(resolve, 4000));
  //   setInstallationComplete(true);
  //   addToAllLogs(["application installation complete"]);
  //   updateStepStatus('application', { status: 'completed' });
  // };

  const handleConfirmProceed = () => {
    setShowPreflightModal(false);
    // startApplicationInstallation();
  };

  const handleCancelProceed = () => {
    setShowPreflightModal(false);
  };

  const canProceed = () => {
      if (currentStep === 'hosts') {
        return hostsComplete; // Can proceed once initial host setup is done
      } else if (currentStep === 'infrastructure') {
        return steps.infrastructure.status === 'completed';
      }
      // else if (currentStep === 'preflights') {
      //   return steps.preflights.status === 'completed'// todo (screspod): add: blockOnAppPreflights
      // } else if (currentStep === 'application') {
      //   return installationComplete;
      // }
      else {
        return false;
      }
  };

  const handleNextClick = () => {
    if (currentStep === 'hosts') {
      startInfrastructureSetup();
    } else if (currentStep === 'infrastructure') {
      onNext(); // startPreflightChecks();
    }
    // else if (currentStep === 'preflights') {
    //   startApplicationInstallation();
    // } else if (currentStep === 'application') {
    //   onNext(); // Go to completion step
    // }
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
          />

          <div className="flex-1">
            <StepDetailPanel
              selectedStep={selectedStep}
              stepData={steps[selectedStep]}
              infrastructureStatus={infrastructureStatus}
              // preflightResults={validationResults}
              // applicationStatus={applicationStatus}
              onHostsComplete={handleHostsComplete}
            />
          </div>
        </div>

        <div className="border-t border-gray-200 p-6">
          <LogViewer
            title="Installation Logs"
            logs={allLogs}
            isExpanded={showLogs}
            onToggle={() => setShowLogs(!showLogs)}
          />
        </div>
      </Card>

      <div className="flex justify-between">
        <Button
          variant="outline"
          onClick={onBack}
          icon={<ChevronLeft className="w-5 h-5" />}
        >
          Back
        </Button>
        <Button
          onClick={handleNextClick}
          disabled={!canProceed()}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          {currentStep === 'hosts' && 'Next: Infrastructure Setup'}
          {currentStep === 'infrastructure' && 'Next: Preflight Checks'}
          {/* {currentStep === 'preflights' && 'Next: Install Application'}
          {currentStep === 'application' && 'Next: Finish'} */}
        </Button>
      </div>

      {showPreflightModal && (
        <Modal
          onClose={handleCancelProceed}
          title="Proceed with Failed Checks?"
          footer={
            <div className="flex space-x-3">
              <Button
                variant="outline"
                onClick={handleCancelProceed}
              >
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={handleConfirmProceed}
              >
                Continue Anyway
              </Button>
            </div>
          }
        >
          <div className="flex items-start space-x-3">
            <div className="flex-shrink-0">
              <AlertTriangle className="h-6 w-6 text-amber-500" />
            </div>
            <div>
              <p className="text-sm text-gray-700">
                Some preflight checks have failed. Are you sure you want to continue with the installation? This may cause installation issues.
              </p>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default LinuxInstallationStep;