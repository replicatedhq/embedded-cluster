import React, { useState, useEffect } from "react";
import Input from "../../common/Input";
import Select from "../../common/Select";
import Button from "../../common/Button";
import Card from "../../common/Card";
import { useInitialState } from "../../../contexts/InitialStateContext";
import { useWizard } from "../../../contexts/WizardModeContext";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import type { components } from "../../../types/api";
import { createAuthedClient, getWizardBasePath } from '../../../api/client';
import { ApiError } from '../../../api/error';
import { useLinuxInstallConfig, useInstallationStatus, useNetworkInterfaces } from '../../../queries/useQueries';
import { formatErrorMessage } from "../../../utils/errorMessage";

type Status = components["schemas"]["types.Status"];
type LinuxInstallationConfig = components["schemas"]["types.LinuxInstallationConfig"];

interface LinuxSetupStepProps {
  onNext: () => void;
  onBack: () => void;
}

const LinuxSetupStep: React.FC<LinuxSetupStepProps> = ({ onNext, onBack }) => {
  const { text } = useWizard();
  const { title } = useInitialState();
  const [isInstallationStatusPolling, setIsInstallationStatusPolling] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [defaults, setDefaults] = useState<LinuxInstallationConfig>({ dataDirectory: "" });
  const [configValues, setConfigValues] = useState<LinuxInstallationConfig>({ dataDirectory: "" });
  const { token } = useAuth();

  // Query for fetching install configuration
  const { data: configResponse, isLoading: isConfigLoading } = useLinuxInstallConfig();

  // Store defaults and config values when config loads
  useEffect(() => {
    if (configResponse) {
      setDefaults(configResponse.defaults);
      setConfigValues(configResponse.values);
    }
  }, [configResponse]);

  // Query for fetching network interfaces
  const { data: networkInterfacesData, isLoading: isInterfacesLoading } = useNetworkInterfaces();

  // Query to poll installation status
  const { data: installationStatus } = useInstallationStatus({
    enabled: isInstallationStatusPolling,
    refetchInterval: 1000,
    gcTime: 0,
  });

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ApiError, LinuxInstallationConfig>({
    mutationFn: async (configData) => {
      const client = createAuthedClient(token);
      const path = getWizardBasePath("linux", "install");

      const { data, error } = await client.POST(`${path}/installation/configure`, {
        body: configData,
      });

      if (error) {
        throw error;
      }
      return data;
    },
    onSuccess: () => {
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

  useEffect(() => {
    if (shouldExpandAdvancedSettings(submitError?.fieldErrors)) {
      setShowAdvanced(true);
    }
  }, [submitError]);

  useEffect(() => {
    const evaluation = evaluateInstallationStatus(installationStatus);

    if (evaluation.shouldStopPolling) {
      setIsInstallationStatusPolling(false);
    }

    if (evaluation.errorMessage) {
      setError(evaluation.errorMessage);
    }

    if (evaluation.shouldProceedToNext) {
      onNext();
    }
  }, [installationStatus, onNext]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { id, value } = e.target;
    const updatedConfig = processInputValue(id, value, configValues);
    setConfigValues(updatedConfig);
  };

  const handleSelectChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const { id, value } = e.target;
    setConfigValues({ ...configValues, [id]: value });
  };

  const isLoading = determineLoadingState(
    isConfigLoading,
    isInterfacesLoading,
    isInstallationStatusPolling
  );

  const availableNetworkInterfaces = networkInterfacesData?.networkInterfaces || [];

  const getFieldError = (fieldName: string) => {
    return extractFieldError(fieldName, submitError?.fieldErrors, fieldNames);
  };

  const loadingText = determineLoadingText(isInstallationStatusPolling);

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
            <p className="mt-2 text-gray-600" data-testid="linux-setup-loading-text">{loadingText}</p>
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

export interface FieldError {
  field: string;
  message: string;
}

export interface InstallationStatusEvaluation {
  shouldStopPolling: boolean;
  shouldProceedToNext: boolean;
  errorMessage: string | null;
}

/**
 * Maps internal field names to user-friendly display names.
 */
export const fieldNames = {
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
};

/**
 * Hop: processInputValue
 * Process and validate user input based on field type.
 * Port fields are converted to numbers or undefined, with validation.
 * Other fields are stored as strings.
 */
export function processInputValue(
  fieldId: string,
  value: string,
  currentValues: LinuxInstallationConfig
): LinuxInstallationConfig {
  if (fieldId === "localArtifactMirrorPort") {
    // Handle port fields - they need to be numbers
    if (value === "") {
      // Empty string becomes undefined
      return { ...currentValues, [fieldId]: undefined };
    }

    const numValue = Number(value);
    if (Number.isInteger(numValue)) {
      // Valid integer - update the value
      return { ...currentValues, [fieldId]: numValue };
    }

    // Invalid value (decimal or non-numeric) - don't update
    return currentValues;
  }

  // For all other fields, just store the string value
  return { ...currentValues, [fieldId]: value };
}

/**
 * Hop: extractFieldError
 * Extract and format error message for a specific field.
 * Uses the formatErrorMessage utility to replace field names with display names.
 */
export function extractFieldError(
  fieldName: string,
  fieldErrors: FieldError[] | undefined,
  fieldNameMap: Record<string, string>
): string | undefined {
  if (!fieldErrors) {
    return undefined;
  }

  const fieldError = fieldErrors.find(err => err.field === fieldName);
  if (!fieldError) {
    return undefined;
  }

  return formatErrorMessage(fieldError.message, fieldNameMap);
}

/**
 * Hop: determineLoadingText
 * Determine which loading message to display based on current state.
 */
export function determineLoadingText(isInstallationStatusPolling: boolean): string {
  if (isInstallationStatusPolling) {
    return "Preparing the host.";
  }
  return "Loading configuration...";
}

/**
 * Hop: shouldExpandAdvancedSettings
 * Determine if advanced settings should auto-expand due to errors.
 * Advanced fields include networkInterface and globalCidr.
 */
export function shouldExpandAdvancedSettings(
  fieldErrors: FieldError[] | undefined
): boolean {
  // Check if any advanced field has an error
  if (!fieldErrors) {
    return false;
  }

  return fieldErrors.some(err =>
    err.field === "networkInterface" ||
    err.field === "globalCidr"
  );
}

/**
 * Hop: evaluateInstallationStatus
 * Evaluate installation status and determine next action.
 * Returns instructions on whether to stop polling, proceed to next step, or show an error.
 */
export function evaluateInstallationStatus(
  status: Status | undefined
): InstallationStatusEvaluation {
  // No status yet - continue polling
  if (!status) {
    return {
      shouldStopPolling: false,
      shouldProceedToNext: false,
      errorMessage: null,
    };
  }

  // Installation failed
  if (status.state === "Failed") {
    const errorMsg = status.description
      ? `Installation configuration failed with: ${status.description}`
      : "Installation configuration failed";

    return {
      shouldStopPolling: true,
      shouldProceedToNext: false,
      errorMessage: errorMsg,
    };
  }

  // Installation succeeded
  if (status.state === "Succeeded") {
    return {
      shouldStopPolling: true,
      shouldProceedToNext: true,
      errorMessage: null,
    };
  }

  // Any other state (Running, Pending, etc.) - continue polling
  return {
    shouldStopPolling: false,
    shouldProceedToNext: false,
    errorMessage: null,
  };
}

/**
 * Hop: determineLoadingState
 * Aggregate loading states from multiple sources.
 * Returns true if any source is loading.
 */
export function determineLoadingState(
  isConfigLoading: boolean,
  isInterfacesLoading: boolean,
  isInstallationStatusPolling: boolean
): boolean {
  return isConfigLoading || isInterfacesLoading || isInstallationStatusPolling;
}
