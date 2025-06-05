import React, { useState, useEffect } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useConfig } from "../../contexts/ConfigContext";
import { CheckCircle, ExternalLink, Loader2 } from "lucide-react";
import { useQuery, Query } from "@tanstack/react-query";
import { useWizardMode } from "../../contexts/WizardModeContext";

interface InstallStatus {
  state: "Succeeded" | "Failed" | "InProgress";
}

const InstallationStep: React.FC = () => {
  const { config } = useConfig();
  const { text } = useWizardMode();
  const [showAdminLink, setShowAdminLink] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const { data: installStatus } = useQuery<InstallStatus, Error>({
    queryKey: ["installStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/status", {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
          ...(localStorage.getItem("auth") && {
            Authorization: `Bearer ${localStorage.getItem("auth")}`,
          }),
        },
      });

      if (!response.ok) {
        throw new Error(`Installation failed: ${response.statusText}`);
      }

      return response.json();
    },
    refetchInterval: (query: Query<InstallStatus, Error>) => {
      // Continue polling until we get a final state
      return query.state.data?.state === "Succeeded" || query.state.data?.state === "Failed" ? false : 5000;
    },
  });

  useEffect(() => {
    if (installStatus?.state === "Succeeded") {
      setShowAdminLink(true);
      setIsLoading(false);
    } else if (installStatus?.state === "Failed") {
      setError("Installation failed");
      setIsLoading(false);
    }
  }, [installStatus]);

  return (
    <div className="space-y-6">
      <Card>
        {installStatus?.state !== "Succeeded" && (
          <div className="my-6">
            <h2 className="text-2xl font-bold text-gray-900">{text.installationTitle}</h2>
            <p className="text-gray-600 mt-1">{text.installationDescription}</p>
          </div>
        )}

        <div className="flex flex-col items-center text-center py-6">
          {isLoading && (
            <div className="flex flex-col items-center pt-6">
              <Loader2 className="h-8 w-8 animate-spin text-gray-600 mb-4" />
              <p className="text-lg font-medium text-gray-900">Please wait while we complete the installation...</p>
              <p className="text-sm text-gray-500 mt-2">This may take a few minutes.</p>
            </div>
          )}

          {error && (
            <div className="text-red-600 mb-8">
              <p className="text-xl">Installation Error</p>
              <p>{error}</p>
            </div>
          )}

          {showAdminLink && (
            <div className="flex flex-col items-center justify-center mb-6">
              <div className="w-16 h-16 rounded-full flex items-center justify-center mb-6">
                <CheckCircle className="w-10 h-10" style={{ color: "blue" }} />
              </div>
              <p className="text-gray-600 mt-4">
                Visit the Admin Console to configure and install {text.installationTitle}
              </p>
              <Button
                className="mt-4"
                onClick={() => window.open(`http://${window.location.hostname}:${config.adminConsolePort}`, "_blank")}
                icon={<ExternalLink className="ml-2 -mr-1 h-5 w-5" />}
              >
                Visit Admin Console
              </Button>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};

export default InstallationStep;
