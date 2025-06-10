import React, { useState, useEffect } from "react";
import Card from "../common/Card";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { useConfig } from "../../contexts/ConfigContext";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "../../contexts/AuthContext";

interface InstallStatus {
  state: string;
  description?: string;
}

const InstallationStep: React.FC = () => {
  const { config } = useConfig();
  const { text } = useWizardMode();
  const { token } = useAuth();
  const [showAdminLink, setShowAdminLink] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const { data: installStatus } = useQuery<InstallStatus, Error>({
    queryKey: ["installStatus"],
    queryFn: async () => {
      const response = await fetch("/api/install/status", {
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
    refetchInterval: (query: { state: { data?: InstallStatus } }) => {
      // Continue polling until we get a final state
      return query.state.data?.state === "Succeeded" || query.state.data?.state === "Failed" ? false : 5000;
    },
  });

  useEffect(() => {
    if (installStatus?.state === "Succeeded") {
      setShowAdminLink(true);
      setIsLoading(false);
    } else if (installStatus?.state === "Failed") {
      setError(installStatus.description || "Installation failed");
      setIsLoading(false);
    }
  }, [installStatus]);

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <h2 className="text-3xl font-bold text-gray-900">{text.installationTitle}</h2>
          <p className="text-xl text-gray-600 max-w-2xl mb-8">{text.installationDescription}</p>

          {isLoading && (
            <div className="animate-pulse">
              <p className="text-gray-600">Installing...</p>
            </div>
          )}

          {error && (
            <div className="mt-4 p-4 bg-red-50 border border-red-200 rounded-lg">
              <h3 className="text-lg font-medium text-red-800">Installation Error</h3>
              <p className="mt-2 text-red-700">{error}</p>
            </div>
          )}

          {showAdminLink && (
            <div className="mt-8">
              <a
                href={`https://localhost:${config.adminConsolePort}`}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                Visit Admin Console
              </a>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};

export default InstallationStep;
