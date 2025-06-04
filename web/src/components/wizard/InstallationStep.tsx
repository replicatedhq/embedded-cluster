import React, { useState, useEffect } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useConfig } from "../../contexts/ConfigContext";
import { ExternalLink } from "lucide-react";
import { useQuery, Query } from "@tanstack/react-query";

interface InstallStatus {
  state: "Succeeded" | "Failed" | "InProgress";
}

const InstallationStep: React.FC = () => {
  const { config } = useConfig();
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
      return query.state.data?.state === "Succeeded" ||
        query.state.data?.state === "Failed"
        ? false
        : 5000;
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
            <Button
              onClick={() => window.open(`http://${window.location.hostname}:${config.adminConsolePort}`, '_blank')}
              icon={<ExternalLink className="ml-2 -mr-1 h-5 w-5" />}
            >
              Visit Admin Console
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
};

export default InstallationStep;
