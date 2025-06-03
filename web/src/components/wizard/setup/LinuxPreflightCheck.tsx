import React, { useState, useEffect } from "react";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { XCircle, CheckCircle, Loader2, AlertTriangle } from "lucide-react";
import { useQuery, useMutation } from "@tanstack/react-query";

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

const LinuxPreflightCheck: React.FC<LinuxPreflightCheckProps> = ({
  onComplete,
}) => {
  const [isChecking, setIsChecking] = useState(true);

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
      setIsChecking(true);
    },
    onError: () => {
      setIsChecking(false);
      onComplete(false);
    },
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
    enabled: isChecking,
    refetchInterval: 1000,
  });

  // Handle preflight status changes
  useEffect(() => {
    if (preflightResponse?.status?.state === "Succeeded" || preflightResponse?.status?.state === "Failed") {
      setIsChecking(false);
      // Consider it successful if there are no failures
      const hasFailures = (preflightResponse.output?.fail?.length ?? 0) > 0;
      onComplete(!hasFailures);
    }
  }, [preflightResponse, onComplete]);

  useEffect(() => {
    runPreflights();
  }, [runPreflights]);

  const renderCheckStatus = (
    result: PreflightResult,
    type: "pass" | "warn" | "fail"
  ) => {
    let Icon = Loader2;
    let statusColor = "text-blue-500";
    let iconClasses = "animate-spin";

    if (!isChecking) {
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
            <h4 className="text-sm font-medium text-gray-900">
              {result.title}
            </h4>
            <p
              className={`mt-1 text-sm ${
                type === "pass"
                  ? "text-gray-500"
                  : type === "warn"
                  ? "text-yellow-600"
                  : "text-red-600"
              }`}
            >
              {result.message}
            </p>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div>
      <h3 className="text-lg font-medium text-gray-900 mb-4">
        System Requirements Check
      </h3>
      <div className="space-y-2 divide-y divide-gray-200">
        {isChecking ? (
          <div className="py-3">
            <div className="flex items-start">
              <div className="flex-shrink-0 text-blue-500">
                <Loader2 className="w-5 h-5 animate-spin" />
              </div>
              <div className="ml-3">
                <h4 className="text-sm font-medium text-gray-900">
                  Running checks...
                </h4>
              </div>
            </div>
          </div>
        ) : preflightResponse?.output ? (
          <>
            {preflightResponse.output.pass?.map((result: PreflightResult, index: number) => (
              <div key={`pass-${index}`}>
                {renderCheckStatus(result, "pass")}
              </div>
            ))}
            {preflightResponse.output.warn?.map((result: PreflightResult, index: number) => (
              <div key={`warn-${index}`}>
                {renderCheckStatus(result, "warn")}
              </div>
            ))}
            {preflightResponse.output.fail?.map((result: PreflightResult, index: number) => (
              <div key={`fail-${index}`}>
                {renderCheckStatus(result, "fail")}
              </div>
            ))}
          </>
        ) : (
          <div className="py-3">
            <div className="flex items-start">
              <div className="flex-shrink-0 text-red-500">
                <XCircle className="w-5 h-5" />
              </div>
              <div className="ml-3">
                <h4 className="text-sm font-medium text-gray-900">
                  Failed to run checks
                </h4>
                <p className="mt-1 text-sm text-red-600">
                  Unable to complete system requirement checks
                </p>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default LinuxPreflightCheck;
