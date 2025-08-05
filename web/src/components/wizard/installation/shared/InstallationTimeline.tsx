import React from 'react';
import { CheckCircle, XCircle, AlertTriangle, Loader2, Clock } from 'lucide-react';
import Button from '../../../common/Button';

export type InstallationStep = 'hosts' | 'infrastructure' // | 'preflights' | 'application';

export interface StepStatus {
  status: 'pending' | 'running' | 'completed' | 'failed';
  title: string;
  description: string;
  progress?: number;
  error?: string;
}

interface InstallationTimelineProps {
  steps: Record<InstallationStep, StepStatus>;
  currentStep: InstallationStep;
  selectedStep: InstallationStep;
  onStepClick: (step: InstallationStep) => void;
}

const InstallationTimeline: React.FC<InstallationTimelineProps> = ({
  steps,
  currentStep,
  selectedStep,
  onStepClick,
}) => {

  const getStatusIcon = (status: 'pending' | 'running' | 'completed' | 'failed') => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="w-6 h-6 text-green-500" />;
      case 'failed':
        return <XCircle className="w-6 h-6 text-red-500" />;
      case 'running':
        return <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />;
      default:
        return <Clock className="w-6 h-6 text-gray-400" />;
    }
  };

  const stepOrder: InstallationStep[] = ['hosts', 'infrastructure']//, 'preflights', 'application'];

  return (
    <div className="w-80 bg-gray-50 border-r border-gray-200 p-6">
      <h3 className="text-lg font-medium text-gray-900 mb-6">Installation Progress</h3>

      <div className="space-y-6">
        {stepOrder.map((stepKey, index) => {
          const step = steps[stepKey];
          const isActive = currentStep === stepKey;
          const isSelected = selectedStep === stepKey;
          const isCompleted = step.status === 'completed';
          const isFailed = step.status === 'failed';
          const isClickable = step.status !== 'pending';

          return (
            <div key={stepKey} className="relative">
               <Button
                 variant="outline"
                 className={`flex items-start space-x-3 text-left w-full p-2 rounded-md transition-colors bg-transparent border-none shadow-none ${
                   isClickable ? 'hover:bg-gray-100 cursor-pointer' : 'cursor-default opacity-50'
                 } ${isSelected ? 'bg-blue-50 border border-blue-200' : ''}`}
                 onClick={() => isClickable && onStepClick(stepKey)}
                 disabled={!isClickable}
               >
                <div className="flex-shrink-0 mt-0.5">
                  {getStatusIcon(step.status)}
                </div>

                <div className="flex-grow min-w-0">
                  <h4 className={`text-sm font-medium ${
                    isSelected ? 'text-gray-900' : isActive ? 'text-gray-900' : step.status === 'completed' ? 'text-gray-700' : 'text-gray-600'
                  }`}>
                    {step.title}
                  </h4>
                  <p className={`text-xs mt-1 ${
                    isSelected ? 'text-gray-600' : isActive ? 'text-gray-600' : 'text-gray-500'
                  }`}>
                    {step.description}
                  </p>

                  {isFailed && step.error && (
                    <p className="text-xs text-red-600 mt-1">{step.error}</p>
                  )}
                </div>
              </Button>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default InstallationTimeline;