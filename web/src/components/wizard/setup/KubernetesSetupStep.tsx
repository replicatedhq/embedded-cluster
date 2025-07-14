import React, { useState } from "react";
import Input from "../../common/Input";
import Button from "../../common/Button";
import Card from "../../common/Card";
import { useKubernetesConfig } from "../../../contexts/KubernetesConfigContext";
import { useWizard } from "../../../contexts/WizardModeContext";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { handleUnauthorized } from "../../../utils/auth";
import { ChevronRight, ChevronLeft } from "lucide-react";

/**
 * Maps internal field names to user-friendly display names.
 * Used for:
 * - Input IDs: <Input id="adminConsolePort" />
 * - Labels: <Input label={fieldNames.adminConsolePort} />
 * - Error formatting: formatErrorMessage("adminConsolePort invalid") -> "Admin Console Port invalid"
 */
const fieldNames = {
   adminConsolePort: "Admin Console Port",
   httpProxy: "HTTP Proxy",
   httpsProxy: "HTTPS Proxy",
   noProxy: "Proxy Bypass List",
}

interface KubernetesSetupStepProps {
  onNext: () => void;
  onBack: () => void;
}

interface Status {
  state: string;
  description?: string;
}

interface ConfigError extends Error {
  errors?: { field: string; message: string }[];
}

// TODO NOW: add tests for this component
const KubernetesSetupStep: React.FC<KubernetesSetupStepProps> = ({ onNext, onBack }) => {
  const { config, updateConfig } = useKubernetesConfig();
  const { text } = useWizard();
  const [error, setError] = useState<string | null>(null);
  const { token } = useAuth();

  // Query for fetching install configuration
  const { isLoading: isConfigLoading } = useQuery({
    queryKey: ["installConfig"],
    queryFn: async () => {
      const response = await fetch("/api/kubernetes/install/installation/config", {
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

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ConfigError, typeof config>({
    mutationFn: async (configData: typeof config) => {
      const response = await fetch("/api/kubernetes/install/installation/configure", {
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
      setError(null); // Clear any previous errors
      startInstallation();
    },
    onError: (err: ConfigError) => {
      setError(err.message || "Failed to submit config");
      return err;
    },
  });

  // Mutation for starting the installation
  const { mutate: startInstallation } = useMutation({
    mutationFn: async () => {
      const response = await fetch("/api/kubernetes/install/infra/setup", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.message || "Failed to start installation");
      }
      return response.json();
    },
    onSuccess: () => {
      setError(null); // Clear any previous errors
      onNext();
    },
    onError: (err: Error) => {
      setError(err.message || "Failed to start installation");
    },
  });

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    if (id === "adminConsolePort") {
      // Only update if the value is empty or a valid number
      if (value === "" || !isNaN(Number(value))) {
        updateConfig({ [id]: value === "" ? undefined : Number(value) });
      }
    } else {
      updateConfig({ [id]: value });
    }
  };

  const getFieldError = (fieldName: string) => {
    const fieldError = submitError?.errors?.find((err) => err.field === fieldName);
    return fieldError ? formatErrorMessage(fieldError.message) : undefined;
  };

  return (
    <div className="space-y-6" data-testid="kubernetes-setup">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.linuxSetupTitle}</h2>
          <p className="text-gray-600 mt-1">Configure the installation settings.</p>
        </div>

        {isConfigLoading ? (
          <div className="py-4 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600">Loading configuration...</p>
          </div>
        ) : (
          <>
            <div className="space-y-4">
              <Input
                id="adminConsolePort"
                label={fieldNames.adminConsolePort}
                value={config.adminConsolePort?.toString() || ""}
                onChange={handleInputChange}
                placeholder="30000"
                helpText="Port for the Admin Console"
                error={getFieldError("adminConsolePort")}
                required
                className="w-96"
              />

              <Input
                id="httpProxy"
                label={fieldNames.httpProxy}
                value={config.httpProxy || ""}
                onChange={handleInputChange}
                placeholder="http://proxy.example.com:3128"
                helpText="HTTP proxy server URL"
                error={getFieldError("httpProxy")}
                className="w-96"
              />

              <Input
                id="httpsProxy"
                label={fieldNames.httpsProxy}
                value={config.httpsProxy || ""}
                onChange={handleInputChange}
                placeholder="https://proxy.example.com:3128"
                helpText="HTTPS proxy server URL"
                error={getFieldError("httpsProxy")}
                className="w-96"
              />

              <Input
                id="noProxy"
                label={fieldNames.noProxy}
                value={config.noProxy || ""}
                onChange={handleInputChange}
                placeholder="localhost,127.0.0.1,.example.com"
                helpText="Comma-separated list of hosts to bypass the proxy"
                error={getFieldError("noProxy")}
                className="w-96"
              />
            </div>

            {error && (
              <div className="mt-4 p-3 bg-red-50 text-red-500 rounded-md">
                {submitError?.errors && submitError.errors.length > 0 
                  ? "Please fix the errors in the form above before proceeding."
                  : error
                }
              </div>
            )}
          </>
        )}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={() => submitConfig(config)} icon={<ChevronRight className="w-5 h-5" />}>
          Next: Start Installation
        </Button>
      </div>
    </div>
  );
};

/**
 * Formats error messages by replacing technical field names with more user-friendly display names.
 * Example: "adminConsolePort" becomes "Admin Console Port".
 *
 * @param message - The error message to format
 * @returns The formatted error message with replaced field names
 */
export function formatErrorMessage(message: string) {
   let finalMsg = message
   for (const [field, fieldName] of Object.entries(fieldNames)) {
      // Case-insensitive regex that matches whole words only
      // Example: "podCidr", "PodCidr", "PODCIDR" all become "Pod CIDR"
      finalMsg = finalMsg.replace(new RegExp(`\\b${field}\\b`, 'gi'), fieldName)
   }
   return finalMsg
}

export default KubernetesSetupStep;
