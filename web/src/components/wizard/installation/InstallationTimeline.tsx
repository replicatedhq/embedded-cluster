import React from 'react';
import { CheckCircle, XCircle, Loader2, Clock } from 'lucide-react';
import { useWizard } from "../../../contexts/WizardModeContext";

import type { InstallationPhaseId } from '../../../types';
import type { components } from "../../../types/api";

type State = components["schemas"]["types.State"];

export interface PhaseStatus {
  status: State;
  title: string;
  description: string;
  error?: string;
}

interface InstallationTimelineProps {
  phases: Record<InstallationPhaseId, PhaseStatus>;
  currentPhase: InstallationPhaseId;
  selectedPhase: InstallationPhaseId;
  onPhaseClick: (phase: InstallationPhaseId) => void;
  phaseOrder: InstallationPhaseId[];
  themeColor: string;
}

const InstallationTimeline: React.FC<InstallationTimelineProps> = ({
  phases,
  currentPhase,
  selectedPhase,
  onPhaseClick,
  phaseOrder
}) => {
  const { text } = useWizard();
  const getStatusIcon = (status: State) => {
    switch (status) {
      case 'Succeeded':
        return <CheckCircle className="w-6 h-6 text-green-500" data-testid="icon-succeeded" />;
      case 'Failed':
        return <XCircle className="w-6 h-6 text-red-500" data-testid="icon-failed" />;
      case 'Running':
        return <Loader2 className="w-6 h-6 text-blue-500 animate-spin" data-testid="icon-running" />;
      case 'Pending':
      default:
        return <Clock className="w-6 h-6 text-gray-400" data-testid="icon-pending" />;
    }
  };

  return (
    <div className="w-80 bg-gray-50 border-r border-gray-200 p-6">
      <h3 className="text-lg font-medium text-gray-900 mb-6" data-testid="timeline-title">{text.timelineTitle}</h3>

      <div className="space-y-6">
        {phaseOrder.map((phaseKey) => {
          const phase = phases[phaseKey];
          const isActive = currentPhase === phaseKey;
          const isSelected = selectedPhase === phaseKey;
          const isFailed = phase.status === 'Failed';
          const isClickable = phase.status !== 'Pending';

          return (
            <div key={phaseKey} className="relative">
              <button
                className={`flex items-start space-x-3 text-left w-full p-2 rounded-md transition-colors ${isClickable ? 'hover:bg-gray-100 cursor-pointer' : 'cursor-default'
                  } ${isSelected ? 'bg-blue-50 border border-blue-200' : ''}`}
                onClick={() => isClickable && onPhaseClick(phaseKey)}
                disabled={!isClickable}
                data-testid={`timeline-${phaseKey}`}
              >
                <div className="shrink-0 mt-0.5">
                  {getStatusIcon(phase.status)}
                </div>

                <div className="grow min-w-0">
                  <h4 className={`text-sm font-medium ${isSelected ? 'text-gray-900' : isActive ? 'text-gray-900' : phase.status === 'Succeeded' ? 'text-gray-700' : 'text-gray-600'
                    }`}>
                    {phase.title}
                  </h4>
                  <p className={`text-xs mt-1 ${isSelected ? 'text-gray-600' : isActive ? 'text-gray-600' : 'text-gray-500'
                    }`}>
                    {phase.description}
                  </p>

                  {isFailed && phase.error && (
                    <p className="text-xs text-red-600 mt-1">{phase.error}</p>
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
