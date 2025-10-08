import React, { useState, useEffect } from "react";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { XCircle, CheckCircle, Loader2, AlertTriangle, RefreshCw } from "lucide-react";
import { useQuery, useMutation } from "@tanstack/react-query";
import Button from "../../../common/Button";
import { useAuth } from "../../../../contexts/AuthContext";
import { PreflightOutput, AppPreflightResponse } from "../../../../types";
import { getApiBase } from '../../../../utils/api-base';
import { ApiError } from '../../../../utils/api-error';

interface AppPreflightCheckProps {
  onRun: () => void;
  onComplete: (success: boolean, allowIgnoreAppPreflights: boolean, hasStrictFailures: boolean) => void;
}

const AppPreflightCheck: React.FC<AppPreflightCheckProps> = ({ onRun, onComplete }) => {
  const [isPreflightsPolling, setIsPreflightsPolling] = useState(true);
  const { settings } = useSettings();
  const { target, mode } = useWizard();
  const themeColor = settings.themeColor;
  const { token } = useAuth();

  const hasFailures = (output?: PreflightOutput) => (output?.fail?.length ?? 0) > 0;
  const hasWarnings = (output?: PreflightOutput) => (output?.warn?.length ?? 0) > 0;
  const hasStrictFailures = (response?: AppPreflightResponse) => response?.hasStrictAppPreflightFailures ?? false;
  const isSuccessful = (response?: AppPreflightResponse) => response?.status?.state === "Succeeded";

  const getErrorMessage = () => {
    if (preflightsRunError) {
      return preflightsRunError.message;
    }
    if (preflightResponse?.status?.state === "Failed") {
      return preflightResponse?.status?.description;
    }
    return "";
  };

  const apiBase = getApiBase(target, mode);
  // Mutation to run preflight checks
  const { mutate: runPreflights, error: preflightsRunError } = useMutation({
    mutationFn: async () => {
      const response = await fetch(`${apiBase}/app-preflights/run`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ isUi: true }),
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to run application preflight checks")
      }
      return response.json() as Promise<AppPreflightResponse>;
    },
    onSuccess: () => {
      setIsPreflightsPolling(true);
      onRun();
    },
    onError: () => {
      setIsPreflightsPolling(false);
    },
  });

  // Query to poll preflight status
  const { data: preflightResponse } = useQuery<AppPreflightResponse, Error>({
    queryKey: ["appPreflightStatus"],
    queryFn: async () => {
      const response = await fetch(`${apiBase}/app-preflights/status`, {
        headers: {
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to get application preflight status")
      }
      return response.json() as Promise<AppPreflightResponse>;
    },
    enabled: isPreflightsPolling,
    refetchInterval: 1000,
  });

  // Handle preflight status changes
  useEffect(() => {
    if (preflightResponse?.status?.state === "Succeeded" || preflightResponse?.status?.state === "Failed") {
      setIsPreflightsPolling(false);
      onComplete(
        !hasFailures(preflightResponse.output),
        preflightResponse.allowIgnoreAppPreflights ?? false,
        hasStrictFailures(preflightResponse)
      );
    }
  }, [preflightResponse]);

  if (isPreflightsPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Validating application requirements...</p>
        <p className="text-sm text-gray-500 mt-2">Please wait while we check your application.</p>
      </div>
    );
  }

  // If there are no failures and no warnings and we have results, show success
  if (isSuccessful(preflightResponse)) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <div
          className="w-12 h-12 rounded-full flex items-center justify-center mb-4"
          style={{ backgroundColor: `${themeColor}1A` }}
        >
          <CheckCircle className="w-6 h-6" style={{ color: themeColor }} />
        </div>
        <p className="text-lg font-medium text-gray-900">Application validation successful!</p>
      </div>
    );
  }

  // If there are no failures and no warnings then we have an api error
  if (!hasFailures(preflightResponse?.output) && !hasWarnings(preflightResponse?.output)) {
    return (
      <div className="bg-white rounded-lg border border-red-200 p-4">
        <div className="flex items-start">
          <XCircle className="w-5 h-5 text-red-500 mt-0.5 shrink-0" />
          <div className="ml-3">
            <h4 className="text-sm font-medium text-gray-900">Unable to complete application requirement checks</h4>
            <p className="mt-1 text-sm text-red-600">{getErrorMessage()}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <div className="flex flex-col items-center text-center py-6">
        {/* Header Section */}
        {hasFailures(preflightResponse?.output) ? (
          <>
            <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
              <XCircle className="w-6 h-6 text-red-600" />
            </div>
            <h3 className="text-lg font-medium text-gray-900">Application Requirements Not Met</h3>
            <p className="text-sm text-gray-600 mt-1 max-w-lg">
              We found some issues that need to be resolved before proceeding with the application installation.
            </p>
          </>
        ) : (
          <>
            <div className="w-12 h-12 rounded-full bg-yellow-100 flex items-center justify-center mb-4">
              <AlertTriangle className="w-6 h-6 text-yellow-600" />
            </div>
            <h3 className="text-lg font-medium text-gray-900">Application Requirements Review</h3>
            <p className="text-sm text-gray-600 mt-1 max-w-lg">
              Please review the following warnings before proceeding with the application installation.
            </p>
          </>
        )}
      </div>

      {/* Failures Section */}
      {hasFailures(preflightResponse?.output) && (
        <div className="bg-white rounded-lg border border-gray-200 divide-y divide-gray-200">
          {preflightResponse?.output?.fail
            ?.slice()
            .sort((a, b) => (b.strict ? 1 : 0) - (a.strict ? 1 : 0))
            .map((result, index) => (
              <div
                key={`fail-${result.title}-${index}`}
                className={`p-4 ${index === 0 ? 'rounded-t-lg' : ''} ${index === (preflightResponse?.output?.fail?.length ?? 0) - 1 ? 'rounded-b-lg' : ''}`}
                style={result.strict ? {
                  borderLeft: '4px solid #dc2626',
                  backgroundColor: '#fef2f2'
                } : {}}
              >
                <div className="flex items-start" style={result.strict ? { marginLeft: '-4px' } : {}}>
                  <XCircle className="w-5 h-5 text-red-500 mt-0.5 shrink-0" />
                  <div className="ml-3 flex-grow">
                    <div className="flex items-center justify-between">
                      <h4 className="text-sm font-medium text-gray-900">{result.title}</h4>
                      {result.strict && (
                        <span className="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-red-100 text-red-800">
                          Critical
                        </span>
                      )}
                    </div>
                    <div className="mt-2 text-sm text-gray-600">
                      <p>{result.message}</p>
                    </div>
                  </div>
                </div>
              </div>
            ))}
        </div>
      )}

      {/* Warnings Section */}
      {hasWarnings(preflightResponse?.output) && (
        <div className="bg-white rounded-lg border border-yellow-200 divide-y divide-yellow-100">
          {preflightResponse?.output?.warn?.map((result, index) => (
            <div key={`warn-${index}`} className="p-4">
              <div className="flex items-start">
                <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 shrink-0" />
                <div className="ml-3">
                  <h4 className="text-sm font-medium text-gray-900">{result.title}</h4>
                  <div className="mt-2 text-sm text-gray-600">
                    <p>{result.message}</p>
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* What's Next Section */}
      <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
        <h4 className="text-sm font-medium text-gray-900">What's Next?</h4>
        <ul className="mt-2 text-sm text-gray-600 space-y-1">
          {hasFailures(preflightResponse?.output) && (
            <>
              <li>• Review and address each failed requirement</li>
              <li>• Click "Back" to modify your setup if needed</li>
            </>
          )}
          {hasWarnings(preflightResponse?.output) && <li>• Review the warnings above and take action if needed</li>}
          <li>• Re-run the validation once issues are addressed</li>
        </ul>
        <div className="mt-4">
          <Button onClick={() => runPreflights()} icon={<RefreshCw className="w-4 h-4" />}>
            Run Validation Again
          </Button>
        </div>
      </div>
    </div>
  );
};

export default AppPreflightCheck;
