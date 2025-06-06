import React, { useState, useEffect } from "react";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { XCircle, CheckCircle, Loader2, AlertTriangle } from "lucide-react";
import { useQuery, useMutation } from "@tanstack/react-query";
import Button from "../../common/Button";

interface LinuxPreflightCheckProps {
  config: ClusterConfig;
  onComplete: (success: boolean) => void;
}

interface PreflightResult {
  title: string;
  message: string;
}

interface PreflightOutput {
  pass: PreflightResult[];
  warn: PreflightResult[];
  fail: PreflightResult[];
}

interface PreflightStatus {
  state: string;
  description: string;
  lastUpdated: string;
}

interface PreflightResponse {
  titles: string[];
  output?: PreflightOutput;
  status?: PreflightStatus;
}

interface InstallationConfigResponse {
  description: string;
  lastUpdated: string;
  state: "Failed" | "Succeeded" | "Running";
}

const LinuxPreflightCheck: React.FC<LinuxPreflightCheckProps> = ({ onComplete }) => {
  const [isPreflightsPolling, setIsPreflightsPolling] = useState(false);
  const [isConfigPolling, setIsConfigPolling] = useState(true);

  // Mutation to run preflight checks
  const { mutate: runPreflights } = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/install/host-preflights/run", {
        method: "POST",
        headers: {
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });
      if (!response.ok) {
        throw new Error("Failed to run preflight checks");
      }
      return response.json() as Promise<PreflightResponse>;
    },
    onSuccess: () => {
      setIsPreflightsPolling(true);
    },
    onError: () => {
      setIsPreflightsPolling(false);
      onComplete(false);
    },
  });

  // Query to poll installation config status
  const { data: installationConfigStatus } = useQuery<
    InstallationConfigResponse,
    Error
  >({
    queryKey: ["installationConfigStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/installation/status", {
        headers: {
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });
      if (!response.ok) {
        throw new Error("Failed to get installation config status");
      }

      return response.json() as Promise<InstallationConfigResponse>;
    },
    enabled: isConfigPolling,
    refetchInterval: 1000,
  });
  // Query to poll preflight status
  const { data: preflightResponse } = useQuery<PreflightResponse, Error>({
    queryKey: ["preflightStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/host-preflights/status", {
        headers: {
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });
      if (!response.ok) {
        throw new Error("Failed to get preflight status");
      }
      return response.json() as Promise<PreflightResponse>;
    },
    enabled: isPreflightsPolling,
    refetchInterval: 1000,
  });

  // Handle preflight status changes
  useEffect(() => {
    if (preflightResponse?.status?.state === "Succeeded" || preflightResponse?.status?.state === "Failed") {
      setIsPreflightsPolling(false);
      // Consider it successful if there are no failures
      const hasFailures = (preflightResponse.output?.fail?.length ?? 0) > 0;
      onComplete(!hasFailures);
    }
  }, [preflightResponse, onComplete]);

  useEffect(() => {
    if (installationConfigStatus?.state === "Failed") {
      setIsConfigPolling(false);
      return; // Prevent running preflights if failed
    }
    if (installationConfigStatus?.state === "Succeeded") {
      setIsConfigPolling(false);
      runPreflights();
      setIsPreflightsPolling(true);
    }
  }, [installationConfigStatus, runPreflights]);

  const renderCheckStatus = (result: PreflightResult, type: "pass" | "warn" | "fail") => {
    let Icon = Loader2;
    let statusColor = "text-blue-500";
    let iconClasses = "animate-spin";

    if (!isPreflightsPolling) {
      switch (type) {
        case "pass":
          Icon = CheckCircle;
          statusColor = "text-green-500";
          break;
        case "warn":
          Icon = AlertTriangle;
          statusColor = "text-yellow-500";
          break;
        case "fail":
          Icon = XCircle;
          statusColor = "text-red-500";
          break;
      }
      iconClasses = "";
    }

    return (
      <div className="py-3">
        <div className="flex items-start">
          <div className={`flex-shrink-0 ${statusColor}`}>
            <Icon className={`w-5 h-5 ${iconClasses}`} />
          </div>
          <div className="ml-3">
            <h4 className="text-sm font-medium text-gray-900">{result.title}</h4>
            <p
              className={`mt-1 text-sm ${
                type === "pass" ? "text-gray-500" : type === "warn" ? "text-yellow-600" : "text-red-600"
              }`}
            >
              {result.message}
            </p>
          </div>
        </div>
      </div>
    );
  };

  if (isConfigPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 
          className="w-8 h-8 animate-spin mb-4"
        />
        <p className="text-lg font-medium text-gray-900">Initializing...</p>
        <p className="text-sm text-gray-500 mt-2">Preparing the host.</p>
      </div>
    );
  }

  if (isPreflightsPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 
          className="w-8 h-8 animate-spin mb-4"
        />
        <p className="text-lg font-medium text-gray-900">Validating host requirements...</p>
        <p className="text-sm text-gray-500 mt-2">Please wait while we check your system.</p>
      </div>
    );
  }

  return (
    <div>
      {/* Header for Host Requirements Not Met */}
      {(preflightResponse?.output?.fail?.length ?? 0) > 0 && (
        <div className="mb-6">
          <div className="flex items-center mb-2">
            <XCircle className="w-7 h-7 text-red-500 mr-2" />
            <span className="text-xl font-semibold text-gray-900">Host Requirements Not Met</span>
          </div>
          <div className="text-gray-600 text-sm mb-2">
            We found some issues that need to be resolved before proceeding with the installation.
          </div>
        </div>
      )}{" "}
      <div className="space-y-2 divide-y divide-gray-200">
        {preflightResponse?.output && (
          <>
            {/* Failures Box */}
            {preflightResponse.output.fail && preflightResponse.output.fail.length > 0 && (
              <div className="mb-6 border border-red-200 bg-red-50 rounded-lg p-4">
                {preflightResponse.output.fail.map((result: PreflightResult, index: number) => (
                  <div
                    key={`fail-${index}`}
                    className="py-3 border-b last:border-b-0 border-red-100 flex items-start"
                  >
                    <div className="flex-shrink-0 text-red-500 mt-1">
                      <XCircle className="w-5 h-5" />
                    </div>
                    <div className="ml-3">
                      <h4 className="text-sm font-semibold text-gray-900">{result.title}</h4>
                      <p className="mt-1 text-sm text-red-600">{result.message}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
            {/* Passes and Warnings */}
            {preflightResponse.output.pass?.map((result: PreflightResult, index: number) => (
              <div key={`pass-${index}`}>{renderCheckStatus(result, "pass")}</div>
            ))}
            {preflightResponse.output.warn?.map((result: PreflightResult, index: number) => (
              <div key={`warn-${index}`}>{renderCheckStatus(result, "warn")}</div>
            ))}
          </>
        )}
        {installationConfigStatus?.state === "Failed" && (
          <div className="py-3">
            <div className="flex items-start">
              <div className="flex-shrink-0 text-red-500">
                <XCircle className="w-5 h-5" />
              </div>
              <div className="ml-3">
                <h4 className="text-sm font-medium text-gray-900">Failed to run checks</h4>
                <p className="mt-1 text-sm text-red-600">Unable to complete system requirement checks</p>
                <p className="mt-1 text-sm text-red-600">{installationConfigStatus?.description}</p>
              </div>
            </div>
          </div>
        )}
      </div>
      {/* What's Next Section - always at the bottom if there are failures */}
      {preflightResponse?.output?.fail && preflightResponse.output.fail.length > 0 && (
        <div className="mt-8 bg-gray-50 border border-gray-200 rounded-lg p-4 w-full">
          <div className="font-semibold mb-2">What's Next?</div>
          <ul className="list-disc list-inside text-sm text-gray-700 space-y-1">
            <li>Review and address each failed requirement</li>
            <li>Click "Back" to modify your setup if needed</li>
            <li>Re-run the validation once issues are addressed</li>
          </ul>
          <Button
            className="mt-4 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 transition"
            onClick={() => runPreflights()}
          >
            Run Validation Again
          </Button>
        </div>
      )}
    </div>
  );
};

export default LinuxPreflightCheck;
