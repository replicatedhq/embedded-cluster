import React, { useState, useEffect } from "react";
import Input from "../../common/Input";
import Select from "../../common/Select";
import Button from "../../common/Button";
import Card from "../../common/Card";
import { useInitialState } from "../../../contexts/InitialStateContext";
import { useLinuxConfig } from "../../../contexts/LinuxConfigContext";
import { useWizard } from "../../../contexts/WizardModeContext";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { handleUnauthorized } from "../../../utils/auth";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";

/**
 * Maps internal field names to user-friendly display names.
 * Used for:
 * - Input IDs: <Input id="adminConsolePort" />
 * - Labels: <Input label={fieldNames.adminConsolePort} />
 * - Error formatting: formatErrorMessage("adminConsolePort invalid") -> "Admin Console Port invalid"
 */
const fieldNames = {
  adminConsolePort: "Admin Console Port",
  dataDirectory: "Data Directory",
  localArtifactMirrorPort: "Local Artifact Mirror Port",
  httpProxy: "HTTP Proxy",
  httpsProxy: "HTTPS Proxy",
  noProxy: "Proxy Bypass List",
  networkInterface: "Network Interface",
  podCidr: "Pod CIDR",
  serviceCidr: "Service CIDR",
  globalCidr: "Reserved Network Range (CIDR)",
  cidr: "CIDR",
}

interface LinuxSetupStepProps {
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

const LinuxSetupStep: React.FC<LinuxSetupStepProps> = ({ onNext, onBack }) => {
  const { config, updateConfig } = useLinuxConfig();
  const { text } = useWizard();
  const { title } = useInitialState();
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
      setError(err.message || "Failed to configure installation");
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

  const isLoading = isConfigLoading || isInterfacesLoading;
  const availableNetworkInterfaces = networkInterfacesData?.networkInterfaces || [];

  const getFieldError = (fieldName: string) => {
    const fieldError = submitError?.errors?.find((err) => err.field === fieldName);
    return fieldError ? formatErrorMessage(fieldError.message) : undefined;
  };

  return (
    <div className="space-y-6" data-testid="linux-setup">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.linuxSetupTitle}</h2>
          <p className="text-gray-600 mt-1">Configure the installation settings.</p>
        </div>

        {isLoading ? (
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
                    id="dataDirectory"
                    label={fieldNames.dataDirectory}
                    value={config.dataDirectory || ""}
                    onChange={handleInputChange}
                    placeholder="/var/lib/embedded-cluster"
                    helpText={`Directory where ${title} will store its data`}
                    error={getFieldError("dataDirectory")}
                    required
                  />

                  <Input
                    id="adminConsolePort"
                    label={fieldNames.adminConsolePort}
                    value={config.adminConsolePort?.toString() || ""}
                    onChange={handleInputChange}
                    placeholder="30000"
                    helpText="Port for the Admin Console"
                    error={getFieldError("adminConsolePort")}
                    required
                  />

                  <Input
                    id="localArtifactMirrorPort"
                    label={fieldNames.localArtifactMirrorPort}
                    value={config.localArtifactMirrorPort?.toString() || ""}
                    onChange={handleInputChange}
                    placeholder="50000"
                    helpText="Port for the local artifact mirror"
                    error={getFieldError("localArtifactMirrorPort")}
                    required
                  />
                </div>
              </div>

              <div className="space-y-4">
                <h2 className="text-lg font-semibold text-gray-900 mb-4">Proxy Configuration</h2>
                <div className="space-y-4">
                  <Input
                    id="httpProxy"
                    label={fieldNames.httpProxy}
                    value={config.httpProxy || ""}
                    onChange={handleInputChange}
                    placeholder="http://proxy.example.com:3128"
                    helpText="HTTP proxy server URL"
                    error={getFieldError("httpProxy")}
                  />

                  <Input
                    id="httpsProxy"
                    label={fieldNames.httpsProxy}
                    value={config.httpsProxy || ""}
                    onChange={handleInputChange}
                    placeholder="https://proxy.example.com:3128"
                    helpText="HTTPS proxy server URL"
                    error={getFieldError("httpsProxy")}
                  />

                  <Input
                    id="noProxy"
                    label={fieldNames.noProxy}
                    value={config.noProxy || ""}
                    onChange={handleInputChange}
                    placeholder="localhost,127.0.0.1,.example.com"
                    helpText="Comma-separated list of hosts to bypass the proxy"
                    error={getFieldError("noProxy")}
                  />
                </div>
              </div>

              <div className="pt-2">
                <button
                  type="button"
                  className="flex items-center text-lg font-semibold text-gray-900 mb-6"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                >
                  {showAdvanced ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronRight className="w-4 h-4 mr-1" />}
                  Advanced Settings
                </button>

                {showAdvanced && (
                  <div className="space-y-4">
                    <Select
                      id="networkInterface"
                      label={fieldNames.networkInterface}
                      value={config.networkInterface || ""}
                      onChange={handleSelectChange}
                      options={[
                        ...(availableNetworkInterfaces.length > 0
                          ? availableNetworkInterfaces.map((iface: string) => ({
                            value: iface,
                            label: iface,
                          }))
                          : []),
                      ]}
                      helpText={`Network interface to use for ${title}`}
                      error={getFieldError("networkInterface")}
                      required
                      placeholder="Select a network interface"
                    />

                    <Input
                      id="globalCidr"
                      label={fieldNames.globalCidr}
                      value={config.globalCidr || ""}
                      onChange={handleInputChange}
                      placeholder="10.244.0.0/16"
                      helpText="CIDR notation for the reserved network range (must be /16 or larger)"
                      error={getFieldError("globalCidr")}
                      required
                    />
                  </div>
                )}
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
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={() => submitConfig(config)} icon={<ChevronRight className="w-5 h-5" />}>
          Next: Validate Host
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

export default LinuxSetupStep;
