import React, { useState, useEffect } from "react";
import { useSettings } from "../../../../contexts/SettingsContext";
import { useWizard } from "../../../../contexts/WizardModeContext";
import { XCircle, CheckCircle, Loader2 } from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "../../../../contexts/AuthContext";
import { AppInstallStatus } from "../../../../types";

interface AppInstallationStatusProps {
  onComplete: (success: boolean) => void;
}

const AppInstallationStatus: React.FC<AppInstallationStatusProps> = ({ onComplete }) => {
  const [isPolling, setIsPolling] = useState(true);
  const { settings } = useSettings();
  const { target } = useWizard();
  const themeColor = settings.themeColor;
  const { token } = useAuth();

  // Query to poll app installation status
  const { data: appInstallStatus, error: appInstallError } = useQuery<AppInstallStatus, Error>({
    queryKey: ["appInstallationStatus"],
    queryFn: async () => {
      const response = await fetch(`/api/${target}/install/app/status`, {
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || "Failed to get app installation status");
      }
      return response.json() as Promise<AppInstallStatus>;
    },
    enabled: isPolling,
    refetchInterval: 2000,
  });

  // Handle status changes
  useEffect(() => {
    if (appInstallStatus?.status?.state === "Succeeded") {
      setIsPolling(false);
      onComplete(true);
    } else if (appInstallStatus?.status?.state === "Failed") {
      setIsPolling(false);
      onComplete(false);
    }
  }, [appInstallStatus]);

  // Loading state
  if (isPolling) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
        <p className="text-lg font-medium text-gray-900">Installing application...</p>
        <p className="text-sm text-gray-500 mt-2">
          {appInstallStatus?.status?.description || "Please wait while we install your application."}
        </p>
      </div>
    );
  }

  // Success state
  if (appInstallStatus?.status?.state === "Succeeded") {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <div
          className="w-12 h-12 rounded-full flex items-center justify-center mb-4"
          style={{ backgroundColor: `${themeColor}1A` }}
        >
          <CheckCircle className="w-6 h-6" style={{ color: themeColor }} />
        </div>
        <p className="text-lg font-medium text-gray-900">Application installed successfully!</p>
        <p className="text-sm text-gray-500 mt-2">Your application is now ready to use.</p>
      </div>
    );
  }

  // Error state
  if (appInstallStatus?.status?.state === "Failed" || appInstallError) {
    return (
      <div className="flex flex-col items-center justify-center py-12">
        <div className="w-12 h-12 rounded-full bg-red-100 flex items-center justify-center mb-4">
          <XCircle className="w-6 h-6 text-red-600" />
        </div>
        <p className="text-lg font-medium text-gray-900">Application installation failed</p>
        <p className="text-sm text-gray-500 mt-2">
          {appInstallStatus?.status?.description || appInstallError?.message || "An error occurred during installation."}
        </p>
      </div>
    );
  }

  // Default loading state
  return (
    <div className="flex flex-col items-center justify-center py-12">
      <Loader2 className="w-8 h-8 animate-spin mb-4" style={{ color: themeColor }} />
      <p className="text-lg font-medium text-gray-900">Preparing installation...</p>
    </div>
  );
};

export default AppInstallationStatus;
