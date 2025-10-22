import React, { useState, useEffect } from "react";
import Input from "../../common/Input";
import Button from "../../common/Button";
import Card from "../../common/Card";
import { useWizard } from "../../../contexts/WizardModeContext";
import { useMutation } from "@tanstack/react-query";
import { useAuth } from "../../../contexts/AuthContext";
import { formatErrorMessage } from "../../../utils/errorMessage";
import { ChevronRight, ChevronLeft } from "lucide-react";
import type { components } from "../../../types/api";
import { createAuthedClient, getWizardBasePath } from '../../../api/client';
import { ApiError } from '../../../api/error';
import { useKubernetesInstallConfig } from '../../../queries/useQueries';

type KubernetesInstallationConfig = components["schemas"]["types.KubernetesInstallationConfig"];

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

const KubernetesSetupStep: React.FC<KubernetesSetupStepProps> = ({ onNext, onBack }) => {
  const { text } = useWizard();
  const [error, setError] = useState<string | null>(null);
  const [defaults, setDefaults] = useState<Partial<KubernetesInstallationConfig>>({});
  const [configValues, setConfigValues] = useState<Partial<KubernetesInstallationConfig>>({});
  const { token } = useAuth();

  // Query for fetching install configuration
  const { data: configResponse, isLoading: isConfigLoading } = useKubernetesInstallConfig();

  // Store defaults and config values when config loads
  useEffect(() => {
    if (configResponse) {
      setDefaults(configResponse.defaults);
      setConfigValues(configResponse.values);
    }
  }, [configResponse]);

  // Mutation for submitting the configuration
  const { mutate: submitConfig, error: submitError } = useMutation<Status, ApiError, Partial<KubernetesInstallationConfig>>({
    mutationFn: async (configData) => {
      const client = createAuthedClient(token);
      const path = getWizardBasePath("kubernetes", "install");

      const { data, error, response } = await client.POST(`${path}/installation/configure`, {
        body: configData,
      });

      if (error || !response.ok) {
        throw await ApiError.fromResponse(response, "Failed to submit configuration");
      }
      return data;
    },
    onSuccess: () => {
      // Clear any previous errors
      setError(null);
      startInstallation();
    },
    onError: (err: ApiError) => {
      setError(err.details || err.message);
    },
  });

  // Mutation for starting the installation
  const { mutate: startInstallation } = useMutation({
    mutationFn: async () => {
      const client = createAuthedClient(token);
      const path = getWizardBasePath("kubernetes", "install");

      const { data, error, response } = await client.POST(`${path}/infra/setup`, {});

      if (error || !response.ok) {
        throw await ApiError.fromResponse(response, "Failed to start installation");
      }
      return data;
    },
    onSuccess: () => {
      setError(null); // Clear any previous errors
      onNext();
    },
    onError: (err: ApiError) => {
      setError(err.details || err.message);
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
    const fieldError = submitError?.fieldErrors?.find((err) => err.field === fieldName);
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
                    dataTestId="admin-console-port-input"
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
            </div>

            {error && (
              <div className="mt-6 p-3 bg-red-50 text-red-500 rounded-md">
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
        <Button variant="outline" onClick={onBack} dataTestId="kubernetes-setup-button-back" icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button onClick={() => submitConfig(configValues)} icon={<ChevronRight className="w-5 h-5" />} dataTestId="kubernetes-setup-submit-button">
          Next: Start Installation
        </Button>
      </div>
    </div>
  );
};

export default KubernetesSetupStep;
