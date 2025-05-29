import React, { useState, useEffect } from "react";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { validateHostPreflights } from "../../../utils/validation";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";

interface LinuxPreflightCheckProps {
  config: ClusterConfig;
  onComplete: (success: boolean) => void;
}

const LinuxPreflightCheck: React.FC<LinuxPreflightCheckProps> = ({
  config,
  onComplete,
}) => {
  const [isChecking, setIsChecking] = useState(true);
  const [preflightResults, setPreflightResults] = useState<
    Record<string, { success: boolean; message: string } | null>
  >({});

  useEffect(() => {
    runPreflightChecks();
  }, []);

  const runPreflightChecks = async () => {
    setIsChecking(true);
    try {
      const results = await validateHostPreflights(config);
      setPreflightResults(results);

      const hasErrors = Object.values(results).some(
        (result) => result && !result.success
      );

      onComplete(!hasErrors);
    } catch (error) {
      console.error("Preflight check error:", error);
      onComplete(false);
    } finally {
      setIsChecking(false);
    }
  };

  const renderCheckStatus = (
    title: string,
    result: { success: boolean; message: string } | null
  ) => {
    let Icon = Loader2;
    let statusColor = "text-blue-500";
    let iconClasses = "animate-spin";

    if (!isChecking && result) {
      if (result.success) {
        Icon = CheckCircle;
        statusColor = "text-green-500";
        iconClasses = "";
      } else {
        Icon = XCircle;
        statusColor = "text-red-500";
        iconClasses = "";
      }
    }

    return (
      <div className="py-3">
        <div className="flex items-start">
          <div className={`flex-shrink-0 ${statusColor}`}>
            <Icon className={`w-5 h-5 ${iconClasses}`} />
          </div>
          <div className="ml-3">
            <h4 className="text-sm font-medium text-gray-900">{title}</h4>
            {!isChecking && result && (
              <p
                className={`mt-1 text-sm ${
                  result.success ? "text-gray-500" : "text-red-600"
                }`}
              >
                {result.message}
              </p>
            )}
          </div>
        </div>
      </div>
    );
  };

  const checks = [
    { key: "kernelVersion", title: "Kernel Version" },
    { key: "kernelParameters", title: "Kernel Parameters" },
    { key: "dataDirectory", title: "Data Directory" },
    { key: "systemMemory", title: "System Memory" },
    { key: "systemCPU", title: "CPU Resources" },
    { key: "diskSpace", title: "Disk Space" },
    { key: "selinux", title: "SELinux Status" },
    { key: "networkEndpoints", title: "Network Connectivity" },
  ];

  return (
    <div>
      <h3 className="text-lg font-medium text-gray-900 mb-4">
        System Requirements Check
      </h3>
      <div className="space-y-2 divide-y divide-gray-200">
        {checks.map(({ key, title }) => (
          <div key={key}>{renderCheckStatus(title, preflightResults[key])}</div>
        ))}
      </div>
    </div>
  );
};

export default LinuxPreflightCheck;
