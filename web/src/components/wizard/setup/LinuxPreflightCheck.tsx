import React, { useState, useEffect } from "react";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { validateHostPreflights } from "../../../utils/validation";
import { XCircle, CheckCircle, Loader2, AlertTriangle } from "lucide-react";

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
  status: PreflightStatus;
  output?: PreflightOutput;
}

const LinuxPreflightCheck: React.FC<LinuxPreflightCheckProps> = ({
  config,
  onComplete,
}) => {
  const [isChecking, setIsChecking] = useState(true);
  const [preflightResponse, setPreflightResponse] =
    useState<PreflightResponse | null>(null);

  useEffect(() => {
    runPreflightChecks();
  }, []);

  const runPreflightChecks = async () => {
    setIsChecking(true);
    try {
      const response = await validateHostPreflights(config);
      setPreflightResponse(response);

      // Consider it successful if there are no failures
      const hasFailures = (response.output?.fail?.length ?? 0) > 0;
      onComplete(!hasFailures);
    } catch (error) {
      console.error("Preflight check error:", error);
      onComplete(false);
    } finally {
      setIsChecking(false);
    }
  };

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
            {preflightResponse.output.pass.map((result, index) => (
              <div key={`pass-${index}`}>
                {renderCheckStatus(result, "pass")}
              </div>
            ))}
            {preflightResponse.output.warn.map((result, index) => (
              <div key={`warn-${index}`}>
                {renderCheckStatus(result, "warn")}
              </div>
            ))}
            {preflightResponse.output.fail.map((result, index) => (
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
