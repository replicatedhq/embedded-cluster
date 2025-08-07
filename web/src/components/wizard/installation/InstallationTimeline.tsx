import React from 'react';
import { CheckCircle, XCircle, Loader2, Clock } from 'lucide-react';
import { State } from '../../../types';

export type InstallationStep = 'linux-validation' | 'linux-installation' | 'kubernetes-installation' | 'app-validation' | 'app-installation';

export interface StepStatus {
  status: State;
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
  stepOrder: InstallationStep[];
  themeColor: string;
}

const InstallationTimeline: React.FC<InstallationTimelineProps> = ({
  steps,
  currentStep,
  selectedStep,
  onStepClick,
  stepOrder
}) => {
  const getStatusIcon = (status: State) => {
    switch (status) {
      case 'Succeeded':
        return <CheckCircle className="w-6 h-6 text-green-500" />;
      case 'Failed':
        return <XCircle className="w-6 h-6 text-red-500" />;
      case 'Running':
        return <Loader2 className="w-6 h-6 text-blue-500 animate-spin" />;
      case 'Pending':
      default:
        return <Clock className="w-6 h-6 text-gray-400" />;
    }
  };

  return (
    <div className="w-80 bg-gray-50 border-r border-gray-200 p-6">
      <h3 className="text-lg font-medium text-gray-900 mb-6">Installation Progress</h3>
      
      <div className="space-y-6">
        {stepOrder.map((stepKey) => {
          const step = steps[stepKey];
          const isActive = currentStep === stepKey;
          const isSelected = selectedStep === stepKey;
          const isFailed = step.status === 'Failed';
          const isClickable = step.status !== 'Pending';
          
          return (
            <div key={stepKey} className="relative">
              <button
                className={`flex items-start space-x-3 text-left w-full p-2 rounded-md transition-colors ${
                  isClickable ? 'hover:bg-gray-100 cursor-pointer' : 'cursor-default'
                } ${isSelected ? 'bg-blue-50 border border-blue-200' : ''}`}
                onClick={() => isClickable && onStepClick(stepKey)}
                disabled={!isClickable}
              >
                <div className="flex-shrink-0 mt-0.5">
                  {getStatusIcon(step.status)}
                </div>
                
                <div className="flex-grow min-w-0">
                  <h4 className={`text-sm font-medium ${
                    isSelected ? 'text-gray-900' : isActive ? 'text-gray-900' : step.status === 'Succeeded' ? 'text-gray-700' : 'text-gray-600'
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
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default InstallationTimeline;