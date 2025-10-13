import React, { useState, useEffect, useCallback, useRef } from "react";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useAuth } from "../../../../contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";
import { NextButtonConfig } from "../types";
import { State, Airgap } from "../../../../types";
import { getApiBase } from '../../../../utils/api-base';
import ErrorMessage from "../shared/ErrorMessage";
import { ApiError } from '../../../../utils/api-error';
import { useProcessAirgap } from '../../../../mutations/useMutations';

interface AirgapPhaseProps {
  onNext: () => void;
  setNextButtonConfig: (config: NextButtonConfig) => void;
  onStateChange: (status: State) => void;
}

const AirgapPhase: React.FC<AirgapPhaseProps> = ({ onNext, setNextButtonConfig, onStateChange }) => {
  const { target, mode } = useWizard();
  const { settings } = useSettings();
  const { token } = useAuth();
  const [isPolling, setIsPolling] = useState(true);
  const [processingComplete, setProcessingComplete] = useState(false);
  const [processingSuccess, setProcessingSuccess] = useState(false);
  const themeColor = settings.themeColor;
  const processAirgap = useProcessAirgap();
  const mutationStarted = useRef(false);

  // Query to poll airgap processing status
  const { data: airgapStatus, error: airgapStatusError } = useQuery<Airgap, Error>({
    queryKey: ["airgapStatus"],
    queryFn: async () => {
      const apiBase = getApiBase(target, mode);
      const response = await fetch(`${apiBase}/airgap/status`, {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to get airgap processing status")
      }
      return response.json() as Promise<Airgap>;
    },
    enabled: isPolling,
    refetchInterval: 2000,
  });

  // Handle mutation callbacks
  useEffect(() => {
    if (processAirgap.isSuccess) {
      setIsPolling(true);
    }
    if (processAirgap.isError) {
      setIsPolling(false);
      onStateChange('Failed');
    }
  }, [processAirgap.isSuccess, processAirgap.isError]);

  // Auto-trigger mutation when status is Pending
  useEffect(() => {
    if (airgapStatus?.status?.state === "Pending" && !mutationStarted.current) {
      mutationStarted.current = true;
      processAirgap.mutate();
    }
  }, [airgapStatus?.status?.state]);

  const handleProcessingComplete = useCallback((success: boolean) => {
    setProcessingComplete(true);
    setProcessingSuccess(success);
    setIsPolling(false);
    onStateChange(success ? 'Succeeded' : 'Failed');
  }, []);

  // Report that step is running when component mounts
  useEffect(() => {
    onStateChange('Running');
  }, []);

  // Handle status changes
  useEffect(() => {
    if (airgapStatus?.status?.state === "Succeeded") {
      handleProcessingComplete(true);
    } else if (airgapStatus?.status?.state === "Failed") {
      handleProcessingComplete(false);
    }
  }, [airgapStatus, handleProcessingComplete]);

  // Update next button configuration
  useEffect(() => {
    setNextButtonConfig({
      disabled: !processingComplete || !processingSuccess,
      onClick: onNext,
    });
  }, [processingComplete, processingSuccess, onNext]);

  const renderProcessingStatus = () => {
    // Loading state
    if (isPolling) {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="airgap-processing-loading">
          <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
          <p className="text-lg font-medium text-gray-900">Processing air gap bundle</p>
          <p className="text-sm text-gray-500 mt-2" data-testid="airgap-processing-loading-description">
            {airgapStatus?.status?.description || "Please wait while we process your air gap bundle."}
          </p>
        </div>
      );
    }

    // Success state
    if (airgapStatus?.status?.state === "Succeeded") {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="airgap-processing-success">
          <div
            className="w-12 h-12 rounded-full flex items-center justify-center mb-4"
            style={{ backgroundColor: `${themeColor}1A` }}
          >
            <CheckCircle className="w-6 h-6" style={{ color: themeColor }} />
          </div>
          <p className="text-lg font-medium text-gray-900">Air gap bundle processed successfully</p>
          <p className="text-sm text-gray-500 mt-2">Your air gap bundle has been processed and images are ready.</p>
        </div>
      );
    }

    // Error state
    if (airgapStatus?.status?.state === "Failed") {
      return (
        <div className="flex flex-col items-center justify-center py-12" data-testid="airgap-processing-error">
          <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
            <XCircle className="w-6 h-6 text-red-600" />
          </div>
          <p className="text-lg font-medium text-gray-900">Air gap processing failed</p>
          <p className="text-sm text-gray-500 mt-2" data-testid="airgap-processing-error-message">
            {airgapStatus?.status?.description || "An error occurred during air gap bundle processing."}
          </p>
        </div>
      );
    }

    // Default loading state
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Preparing air gap bundle...</p>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div className="mb-6">
        <h2 className="text-2xl font-bold text-gray-900">Air gap Bundle Processing</h2>
        <p className="text-gray-600 mt-1">Processing and pushing images from your air gap bundle to the registry.</p>
      </div>

      {renderProcessingStatus()}

      {processAirgap.error && <ErrorMessage error={processAirgap.error.message} />}
      {airgapStatusError && <ErrorMessage error={airgapStatusError?.message} />}
    </div>
  );
};

export default AirgapPhase;
