import React, { useState } from "react";
import Input from "../../common/Input";
import Button from "../../common/Button";
import Card from "../../common/Card";
import { useKubernetesConfig } from "../../../contexts/KubernetesConfigContext";
import { useWizard } from "../../../contexts/WizardModeContext";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { handleUnauthorized } from "../../../utils/auth";
import { formatErrorMessage } from "../../../utils/errorMessage";
import { ChevronRight, ChevronLeft } from "lucide-react";
import { KubernetesConfig } from "../../../types";

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

interface KubernetesConfigResponse {
  values: KubernetesConfig;
  defaults: KubernetesConfig;
}

const KubernetesSetupStep: React.FC<KubernetesSetupStepProps> = ({ onNext, onBack }) => {
  const { updateConfig } = useKubernetesConfig(); // We need to make sure to update the global config
  const { text } = useWizard();
  const [error, setError] = useState<string | null>(null);
  const [defaults, setDefaults] = useState<KubernetesConfig>({});
  const [configValues, setConfigValues] = useState<KubernetesConfig>({});
  const { token } = useAuth();

  // Query for fetching install configuration
  const { isLoading: isConfigLoading } = useQuery<KubernetesConfigResponse, Error>({
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
      const configResponse = await response.json();
      // Update the global config with user values, TODO use the "resolved" config to update the context once the API supports it.
      updateConfig(configResponse.values);
      // Store defaults for display in help text
      setDefaults(configResponse.defaults);
      // Store the config values for display in the form inputs
      setConfigValues(configResponse.values)
      return configResponse;
    },
  });

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ConfigError, KubernetesConfig>({
    mutationFn: async (configData) => {
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
      // Update the global (context) config we use accross the project
      updateConfig(configValues);
      // Clear any previous errors
      setError(null);
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
      if (value === "") {
        setConfigValues({ ...configValues, [id]: undefined })
      }
      else if (Number.isInteger(Number(value))) {
        setConfigValues({ ...configValues, [id]: Number.parseInt(value) })
      }
    } else {
      setConfigValues({ ...configValues, [id]: value });
    }
  };

  const getFieldError = (fieldName: string) => {
    const fieldError = submitError?.errors?.find((err) => err.field === fieldName);
    return fieldError ? formatErrorMessage(fieldError.message, fieldNames) : undefined;
  };

  return (
    <div className="space-y-6" data-testid="kubernetes-setup">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.kubernetesSetupTitle}</h2>
          <p className="text-gray-600 mt-1">{text.kubernetesSetupDescription}</p>
        </div>

        {isConfigLoading ? (
          <div className="py-4 text-center">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600">Loading configuration...</p>
          </div>
        ) : (
          <>
            <div className="space-y-8">
              <div className="space-y-4">
                <h2 className="text-lg font-semibold text-gray-900 mb-4">System Configuration</h2>
                <div className="space-y-4">
                  <Input
                    id="adminConsolePort"
                    label={fieldNames.adminConsolePort}
                    value={configValues.adminConsolePort && configValues.adminConsolePort.toString() || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.adminConsolePort?.toString()}
                    helpText="Port for the Admin Console"
                    error={getFieldError("adminConsolePort")}
                    className="w-96"
                  />
                </div>
              </div>

              <div className="space-y-4">
                <h2 className="text-lg font-semibold text-gray-900 mb-4">Proxy Configuration</h2>
                <div className="space-y-4">
                  <Input
                    id="httpProxy"
                    label={fieldNames.httpProxy}
                    value={configValues.httpProxy || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.httpProxy}
                    helpText="HTTP proxy server URL"
                    error={getFieldError("httpProxy")}
                    className="w-96"
                  />

                  <Input
                    id="httpsProxy"
                    label={fieldNames.httpsProxy}
                    value={configValues.httpsProxy || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.httpsProxy}
                    helpText="HTTPS proxy server URL"
                    error={getFieldError("httpsProxy")}
                    className="w-96"
                  />

                  <Input
                    id="noProxy"
                    label={fieldNames.noProxy}
                    value={configValues.noProxy || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.noProxy}
                    helpText="Comma-separated list of hosts to bypass the proxy"
                    error={getFieldError("noProxy")}
                    className="w-96"
                  />
                </div>
              </div>
            </div>

            {error && (
              <div className="mt-6 p-3 bg-red-50 text-red-500 rounded-md">
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
        <Button variant="outline" onClick={onBack} dataTestId="kubernetes-setup-button-back" icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={() => submitConfig(configValues)} icon={<ChevronRight className="w-5 h-5" />}>
          Next: Start Installation
        </Button>
      </div>
    </div>
  );
};

export default KubernetesSetupStep;
