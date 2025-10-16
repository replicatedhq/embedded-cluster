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
import { formatErrorMessage } from "../../../utils/errorMessage";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import { LinuxConfig, State } from "../../../types";
import { getApiBase } from '../../../utils/api-base';
import { ApiError } from '../../../utils/api-error';

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

interface LinuxConfigResponse {
  values: LinuxConfig;
  defaults: LinuxConfig;
  resolved: LinuxConfig;
}

interface InstallationStatusResponse {
  description: string;
  lastUpdated: string;
  state: State;
}

interface NetworkInterfacesResponse {
  networkInterfaces: string[]
}

const LinuxSetupStep: React.FC<LinuxSetupStepProps> = ({ onNext, onBack }) => {
  const { updateConfig } = useLinuxConfig(); // We need to make sure to update the global config
  const { text, target, mode } = useWizard();
  const { title } = useInitialState();
  const [isInstallationStatusPolling, setIsInstallationStatusPolling] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [defaults, setDefaults] = useState<LinuxConfig>({ dataDirectory: "" });
  const [configValues, setConfigValues] = useState<LinuxConfig>({ dataDirectory: "" });
  const { token } = useAuth();
  const apiBase = getApiBase(target, mode);

  // Query for fetching install configuration
  const { isLoading: isConfigLoading } = useQuery<LinuxConfigResponse, Error>({
    queryKey: ["installConfig"],
    queryFn: async () => {
      const response = await fetch(`${apiBase}/installation/config`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to fetch install configuration")
      }
      const configResponse = await response.json();
      // Update the global config with resolved config which includes user values and defaults.
      updateConfig(configResponse.resolved);
      // Store defaults for display in help text
      setDefaults(configResponse.defaults);
      // Store the config values for display in the form inputs
      setConfigValues(configResponse.values)
      return configResponse;
    },
  });

  // Query for fetching network interfaces
  const { data: networkInterfacesData, isLoading: isInterfacesLoading } = useQuery<NetworkInterfacesResponse, Error>({
    queryKey: ["networkInterfaces"],
    queryFn: async () => {
      const response = await fetch("/api/console/available-network-interfaces", {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to fetch network interfaces")
      }
      return response.json();
    },
  });

  // Query to poll installation status
  const { data: installationStatus } = useQuery<InstallationStatusResponse, Error>({
    queryKey: ["installationStatus"],
    queryFn: async () => {
      const response = await fetch(`${apiBase}/installation/status`, {
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });
      if (!response.ok) {
        throw await ApiError.fromResponse(response, "Failed to get installation status")
      }
      return response.json() as Promise<InstallationStatusResponse>;
    },
    enabled: isInstallationStatusPolling,
    refetchInterval: 1000,
    gcTime: 0,
  });

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ApiError, LinuxConfig>({
    mutationFn: async (configData) => {
      const response = await fetch(`${apiBase}/installation/configure`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(configData),
      });

      if (!response.ok) {
        throw await ApiError.fromResponse(response, 'Failed to submit configuration')
      }
      return response.json();
    },
    onSuccess: () => {
      // Update the global (context) config we use accross the project
      updateConfig(configValues);
      // Clear any previous errors
      setError(null);
      // Start polling installation status
      setIsInstallationStatusPolling(true);
    },
    onError: (err: ApiError) => {
      // share the error message from the API
      setError(err.details || err.message);
    },
  });

  // Expand advanced settings if there is an error in an advanced field
  useEffect(() => {
    if (submitError?.fieldErrors) {
      if (submitError.fieldErrors.some(e => e.field === "networkInterface" || e.field === "globalCidr")) {
        setShowAdvanced(true);
      }
    }
  }, [submitError]);


  // Trigger next step when installation status polling finishes
  useEffect(() => {
    if (installationStatus?.state === "Failed") {
      setIsInstallationStatusPolling(false);
      setError(`Installation configuration failed with: ${installationStatus.description}`)
      return;
    }
    if (installationStatus?.state === "Succeeded") {
      setIsInstallationStatusPolling(false);
      onNext();
    }
  }, [installationStatus, onNext]);


  // Handle input changes for text and number inputs
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    if (id === "adminConsolePort" || id === "localArtifactMirrorPort") {
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

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    setConfigValues({ ...configValues, [id]: value });
  };

  const isLoading = isConfigLoading || isInterfacesLoading || isInstallationStatusPolling;
  const availableNetworkInterfaces = networkInterfacesData?.networkInterfaces || [];

  const getFieldError = (fieldName: string) => {
    const fieldError = submitError?.fieldErrors?.find((err) => err.field === fieldName);
    return fieldError ? formatErrorMessage(fieldError.message, fieldNames) : undefined;
  };

  const getLoadingText = () => {
    if (isInstallationStatusPolling) {
      return "Preparing the host."
    }
    return "Loading configuration..."
  }

  return (
    <div className="space-y-6" data-testid="linux-setup">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">{text.linuxSetupTitle}</h2>
          <p className="text-gray-600 mt-1">Configure the installation settings.</p>
        </div>

        {isLoading ? (
          <div className="py-4 text-center" data-testid="linux-setup-loading">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 mx-auto"></div>
            <p className="mt-2 text-gray-600" data-testid="linux-setup-loading-text">{getLoadingText()}</p>
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
                    value={configValues.dataDirectory || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.dataDirectory}
                    helpText={`Directory where ${title} will store its data`}
                    error={getFieldError("dataDirectory")}
                    className="w-96"
                    dataTestId="data-directory-input"
                  />

                  <Input
                    id="adminConsolePort"
                    label={fieldNames.adminConsolePort}
                    value={configValues.adminConsolePort && configValues.adminConsolePort.toString() || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.adminConsolePort?.toString()}
                    helpText="Port for the Admin Console"
                    error={getFieldError("adminConsolePort")}
                    className="w-96"
                    dataTestId="admin-console-port-input"
                  />

                  <Input
                    id="localArtifactMirrorPort"
                    label={fieldNames.localArtifactMirrorPort}
                    value={configValues.localArtifactMirrorPort && configValues.localArtifactMirrorPort.toString() || ""}
                    onChange={handleInputChange}
                    defaultValue={defaults.localArtifactMirrorPort?.toString()}
                    helpText="Port for the local artifact mirror"
                    error={getFieldError("localArtifactMirrorPort")}
                    className="w-96"
                    dataTestId="local-artifact-mirror-port-input"
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
                    dataTestId="http-proxy-input"
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
                    dataTestId="https-proxy-input"
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
                    dataTestId="no-proxy-input"
                  />
                </div>
              </div>

              <div className="pt-2">
                <button
                  type="button"
                  className="flex items-center text-lg font-semibold text-gray-900 mb-6"
                  onClick={() => setShowAdvanced(!showAdvanced)}
                  data-testid="advanced-settings-toggle"
                >
                  {showAdvanced ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronRight className="w-4 h-4 mr-1" />}
                  Advanced Settings
                </button>

                {showAdvanced && (
                  <div className="space-y-4">
                    <Select
                      id="networkInterface"
                      label={fieldNames.networkInterface}
                      value={configValues.networkInterface || defaults.networkInterface || ""}
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
                      className="w-96"
                      dataTestId="network-interface-select"
                    />

                    <Input
                      id="globalCidr"
                      label={fieldNames.globalCidr}
                      value={configValues.globalCidr || ""}
                      onChange={handleInputChange}
                      defaultValue={defaults.globalCidr}
                      helpText="CIDR notation for the reserved network range (must be /16 or larger)"
                      error={getFieldError("globalCidr")}
                      className="w-96"
                      dataTestId="global-cidr-input"
                    />
                  </div>
                )}
              </div>
            </div>

            {error && (
              <div className="mt-6 p-3 bg-red-50 text-red-500 rounded-md" data-testid="linux-setup-error">
                {submitError?.fieldErrors && submitError.fieldErrors.length > 0
                  ? "Please fix the errors in the form above before proceeding."
                  : error
                }
              </div>
            )}
          </>
        )}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} dataTestId="linux-setup-button-back" icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={() => submitConfig(configValues)} icon={<ChevronRight className="w-5 h-5" />} dataTestId="linux-setup-submit-button">
          Next: Validate Host
        </Button>
      </div>
    </div>
  );
};

export default LinuxSetupStep;
