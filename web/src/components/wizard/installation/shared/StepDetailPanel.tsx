import React from 'react';
import { InstallationStep, StepStatus } from './InstallationTimeline';
import HostsDetail from '../steps/HostsDetail';
import InfrastructureDetail from '../steps/InfrastructureDetail';

interface StepDetailPanelProps {
  selectedStep: InstallationStep;
  stepData: StepStatus;
  infrastructureStatus?: any;
//   preflightResults?: any;
//   applicationStatus?: any;
  onHostsComplete: (success: boolean, allowIgnoreHostPreflights: boolean) => void;
  onInfrastructureComplete?: () => void;
}

const StepDetailPanel: React.FC<StepDetailPanelProps> = ({
  selectedStep,
  stepData,
  infrastructureStatus,
//   preflightResults,
//   applicationStatus,
  onHostsComplete,
}) => {

  const renderStepContent = () => {
      switch (selectedStep) {
         case 'hosts':
         return (
            <div>
               hosts component
            </div>
            // <HostsDetail
            //    onComplete={onHostsComplete}
            // />
         );
         case 'infrastructure':
         return (
            <div>
               infrastructure component
            </div>
            // <InfrastructureDetail
            //    status={infrastructureStatus}
            // />
         );
         // case 'preflights':
         // return (
         //    <div>
         //       preflights
         //    </div>
         //    // <PreflightDetail
         //    //    results={preflightResults}
         //    //    status={stepData.status}
         //    // />
         // );
         // case 'application':
         // return (
         //    <div>
         //       application
         //    </div>
         //    // <ApplicationDetail
         //    //    status={applicationStatus}
         //    // />
         // );
      default:
         return null;
    }
  };

  return (
    <div className="p-8">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">{stepData.title}</h2>
        <p className="text-gray-600 mt-1">{stepData.description}</p>
      </div>

      {renderStepContent()}
    </div>
  );
};

export default StepDetailPanel;