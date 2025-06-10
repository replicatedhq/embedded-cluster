import React, { useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useConfig } from "../../../contexts/ConfigContext";
import { useAuth } from "../../../contexts/AuthContext";
import { XCircle, CheckCircle, Loader2, AlertTriangle, RefreshCw } from "lucide-react";
import Button from "../../common/Button";

interface LinuxPreflightCheckProps {
  onComplete: (success: boolean) => void;
}

interface PreflightRecord {
  title: string;
  message: string;
}

interface PreflightOutput {
  pass?: PreflightRecord[];
  warn?: PreflightRecord[];
  fail?: PreflightRecord[];
}

interface PreflightResponse {
  status: {
    state: string;
    description?: string;
  };
  output?: PreflightOutput;
  titles?: string[];
}

interface InstallationStatusResponse {
  state: string;
  description?: string;
}

const LinuxPreflightCheck: React.FC<LinuxPreflightCheckProps> = ({ onComplete }) => {
  const [isPreflightsPolling, setIsPreflightsPolling] = useState(false);
  const [isInstallationStatusPolling, setIsInstallationStatusPolling] = useState(true);
  const { prototypeSettings } = useConfig();
  const { token } = useAuth();
  const themeColor = prototypeSettings.themeColor;

  const hasFailures = (output?: PreflightOutput) => (output?.fail?.length ?? 0) > 0;
  const hasWarnings = (output?: PreflightOutput) => (output?.warn?.length ?? 0) > 0;
  const isSuccessful = (response?: PreflightResponse) => response?.status?.state === "Succeeded";

  const getErrorMessage = () => {
    if (installationStatus?.state === "Failed") {
      return installationStatus?.description;
    }
    if (preflightsRunError) {
      return preflightsRunError.message;
    }
    if (preflightResponse?.status?.state === "Failed") {
      return preflightResponse?.status?.description;
    }
    return "";
  };

  // Mutation to run preflight checks
  const { mutate: runPreflights, error: preflightsRunError } = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/install/host-preflights/run", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const error = new Error("Failed to run preflight checks") as Error & { status: number };
        error.status = response.status;
        throw error;
      }
      return response.json() as Promise<PreflightResponse>;
    },
    onSuccess: () => {
      setIsPreflightsPolling(true);
    },
    onError: () => {
      setIsPreflightsPolling(false);
    },
  });

  // Query to poll preflight status
  const { data: preflightResponse } = useQuery<PreflightResponse, Error>({
    queryKey: ["preflightStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/host-preflights/status", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const error = new Error("Failed to fetch preflight status") as Error & { status: number };
        error.status = response.status;
        throw error;
      }
      return response.json();
    },
    enabled: isPreflightsPolling,
    refetchInterval: 1000,
  });

  // Query to poll installation status
  const { data: installationStatus } = useQuery<InstallationStatusResponse, Error>({
    queryKey: ["installationStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/installation/status", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const error = new Error("Failed to fetch installation status") as Error & { status: number };
        error.status = response.status;
        throw error;
      }
      return response.json();
    },
    enabled: isInstallationStatusPolling,
    refetchInterval: 1000,
  });

  React.useEffect(() => {
    if (preflightResponse?.status?.state === "Succeeded" || preflightResponse?.status?.state === "Failed") {
      setIsPreflightsPolling(false);
      const success = isSuccessful(preflightResponse) && !hasFailures(preflightResponse.output);
      onComplete(success);
    }
  }, [preflightResponse, onComplete]);

  React.useEffect(() => {
    if (installationStatus?.state === "Succeeded" || installationStatus?.state === "Failed") {
      setIsInstallationStatusPolling(false);
    }
  }, [installationStatus]);

  React.useEffect(() => {
    runPreflights();
  }, [runPreflights]);

  if (isInstallationStatusPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Initializing...</p>
        <p className="text-sm text-gray-500 mt-2">Preparing the host.</p>
      </div>
    );
  }

  if (isPreflightsPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Validating host requirements...</p>
        <p className="text-sm text-gray-500 mt-2">Please wait while we check your system.</p>
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
        <p className="text-lg font-medium text-gray-900">Host validation successful!</p>
      </div>
    );
  }

  // If there are no failures and no warnings then we have an api error
  if (!hasFailures(preflightResponse?.output) && !hasWarnings(preflightResponse?.output)) {
    return (
      <div className="bg-white rounded-lg border border-red-200 p-4">
        <div className="flex items-start">
          <XCircle className="w-5 h-5 text-red-500 mt-0.5 flex-shrink-0" />
          <div className="ml-3">
            <h4 className="text-sm font-medium text-gray-900">Unable to complete system requirement checks</h4>
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
            <h3 className="text-lg font-medium text-gray-900">Host Requirements Not Met</h3>
            <p className="text-sm text-gray-600 mt-1 max-w-lg">
              We found some issues that need to be resolved before proceeding with the installation.
            </p>
          </>
        ) : (
          <>
            <div className="w-12 h-12 rounded-full bg-yellow-100 flex items-center justify-center mb-4">
              <AlertTriangle className="w-6 h-6 text-yellow-600" />
            </div>
            <h3 className="text-lg font-medium text-gray-900">Host Requirements Review</h3>
            <p className="text-sm text-gray-600 mt-1 max-w-lg">
              Please review the following warnings before proceeding with the installation.
            </p>
          </>
        )}
      </div>

      {/* Failures Section */}
      {hasFailures(preflightResponse?.output) && (
        <div className="bg-white rounded-lg border border-gray-200 divide-y divide-gray-200">
          {preflightResponse?.output?.fail?.map((result, index) => (
            <div key={`fail-${index}`} className="p-4">
              <div className="flex items-start">
                <XCircle className="w-5 h-5 text-red-500 mt-0.5 flex-shrink-0" />
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

      {/* Warnings Section */}
      {hasWarnings(preflightResponse?.output) && (
        <div className="bg-white rounded-lg border border-yellow-200 divide-y divide-yellow-100">
          {preflightResponse?.output?.warn?.map((result, index) => (
            <div key={`warn-${index}`} className="p-4">
              <div className="flex items-start">
                <AlertTriangle className="w-5 h-5 text-yellow-500 mt-0.5 flex-shrink-0" />
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

export default LinuxPreflightCheck;
