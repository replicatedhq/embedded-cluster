import React, { useState, useEffect } from "react";
import Card from "../common/Card";
import { useConfig } from "../../contexts/ConfigContext";
import { ExternalLink } from "lucide-react";

const ValidationInstallStep: React.FC = () => {
  const { config } = useConfig();
  const [showAdminLink, setShowAdminLink] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let pollInterval: NodeJS.Timeout;

    const checkInstallStatus = async () => {
      try {
        const response = await fetch("/api/install/status", {
          method: "GET",
          headers: {
            "Content-Type": "application/json",
            // Include auth credentials if available from localStorage or another source
            ...(localStorage.getItem("auth") && {
              Authorization: `Bearer ${localStorage.getItem("auth")}`,
            }),
          },
        });

        if (!response.ok) {
          throw new Error(`Installation failed: ${response.statusText}`);
        }

        const data = await response.json();

        if (data.state === "Succeeded") {
          setShowAdminLink(true);
          setIsLoading(false);
          if (pollInterval) {
            clearInterval(pollInterval);
          }
        } else if (data.state === "Failed") {
          throw new Error("Installation failed");
        }
        // If state is neither Succeeded nor Failed, continue polling
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Failed to install cluster"
        );
        setIsLoading(false);
        if (pollInterval) {
          clearInterval(pollInterval);
        }
      }
    };

    // Initial check
    checkInstallStatus();

    // Set up polling every 5 seconds
    pollInterval = setInterval(checkInstallStatus, 2000);

    // Cleanup on unmount
    return () => {
      if (pollInterval) {
        clearInterval(pollInterval);
      }
    };
  }, [config.adminPassword]);

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <h2 className="text-2xl font-bold text-gray-900 mb-4">
            Installing Embedded Cluster
          </h2>

          {isLoading && (
            <p className="text-xl text-gray-600 mb-8">
              Please wait while we complete the installation...
              {/* TODO: Add a loader for now */}
            </p>
          )}

          {error && (
            <div className="text-red-600 mb-8">
              <p className="text-xl">Installation Error</p>
              <p>{error}</p>
            </div>
          )}

          {showAdminLink && (
            <a
              href={`http://${window.location.hostname}:${config.adminConsolePort}`}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center px-4 py-2 border border-transparent text-base font-medium rounded-md shadow-sm text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            >
              Visit Admin Console
              <ExternalLink className="ml-2 -mr-1 h-5 w-5" />
            </a>
          )}
        </div>
      </Card>
    </div>
  );
};

export default ValidationInstallStep;
