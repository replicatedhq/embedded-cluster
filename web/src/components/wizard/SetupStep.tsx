import React, { useState, useEffect } from "react";
import Card from "../common/Card";
import Button from "../common/Button";
import { useConfig } from "../../contexts/ConfigContext";
import { useWizardMode } from "../../contexts/WizardModeContext";
import { ChevronRight } from "lucide-react";
import LinuxSetup from "./setup/LinuxSetup";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useAuth } from "../../contexts/AuthContext";
import { handleUnauthorized } from "../../utils/auth";

interface SetupStepProps {
  onNext: () => void;
}

interface Status {
  state: string;
  description?: string;
}

interface ConfigError extends Error {
  errors?: { field: string; message: string }[];
}

const SetupStep: React.FC<SetupStepProps> = ({ onNext }) => {
  const { config, updateConfig, prototypeSettings } = useConfig();
  const { text } = useWizardMode();
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const { token } = useAuth();

  // Query for fetching install configuration
  const { isLoading: isConfigLoading } = useQuery({
    queryKey: ["installConfig"],
    queryFn: async () => {
      const response = await fetch("/api/linux/install/installation/config", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error("Session expired. Please log in again.");
        }
        throw new Error(errorData.message || "Failed to fetch install configuration");
      }
      const config = await response.json();
      updateConfig(config);
      return config;
    },
  });

  // Query for fetching network interfaces
  const { data: networkInterfacesData, isLoading: isInterfacesLoading } = useQuery({
    queryKey: ["networkInterfaces"],
    queryFn: async () => {
      const response = await fetch("/api/console/available-network-interfaces", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error("Session expired. Please log in again.");
        }
        throw new Error(errorData.message || "Failed to fetch network interfaces");
      }
      return response.json();
    },
  });

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ConfigError, typeof config>({
    mutationFn: async (configData: typeof config) => {
      const response = await fetch("/api/linux/install/installation/configure", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(configData),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        if (response.status === 401) {
          handleUnauthorized(errorData);
          throw new Error("Session expired. Please log in again.");
        }
        throw errorData;
      }
      return response.json();
    },
    onSuccess: () => {
      onNext();
    },
    onError: (err: ConfigError) => {
      setError(err.message || "Failed to setup cluster");
      return err;
    },
  });

  // Expand advanced settings if there is an error in an advanced field
  useEffect(() => {
    if (submitError?.errors) {
      if (submitError.errors.some(e => e.field === "networkInterface" || e.field === "globalCidr")) {
        setShowAdvanced(true);
      }
    }
  }, [submitError]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    if (id === "adminConsolePort" || id === "localArtifactMirrorPort") {
      // Only update if the value is empty or a valid number
      if (value === "" || !isNaN(Number(value))) {
        updateConfig({ [id]: value === "" ? undefined : Number(value) });
      }
    } else {
      updateConfig({ [id]: value });
    }
  };

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    updateConfig({ [id]: value });
  };

  const handleNext = async () => {
    submitConfig(config);
  };

  const isLoading = isConfigLoading || isInterfacesLoading;
  const availableNetworkInterfaces = networkInterfacesData?.networkInterfaces || [];

  return (
    <div className="space-y-6" data-testid="setup-step">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.setupTitle}</h2>
          <p className="text-gray-600 mt-1">Configure the installation settings.</p>
        </div>

        {isLoading ? (
          <div className="py-4 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600">Loading configuration...</p>
          </div>
        ) : (
          <LinuxSetup
            config={config}
            prototypeSettings={prototypeSettings}
            showAdvanced={showAdvanced}
            onShowAdvancedChange={setShowAdvanced}
            onInputChange={handleInputChange}
            onSelectChange={handleSelectChange}
            availableNetworkInterfaces={availableNetworkInterfaces}
            fieldErrors={submitError?.errors || []}
          />
        )}

        {error && (
          <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">
            Please fix the errors in the form above before proceeding.
          </div>
        )}
      </Card>

      <div className="flex justify-end">
        <Button onClick={handleNext} icon={<ChevronRight className="w-5 h-5" />}>
          Next: Validate Host
        </Button>
      </div>
    </div>
  );
};

export default SetupStep;
