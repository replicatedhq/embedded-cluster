import React from "react";
import Input from "../../common/Input";
import Select from "../../common/Select";
import { useBranding } from "../../../contexts/BrandingContext";
import { ChevronDown, ChevronRight } from "lucide-react";

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

interface LinuxSetupProps {
  config: {
    dataDirectory?: string;
    adminConsolePort?: number;
    localArtifactMirrorPort?: number;
    httpProxy?: string;
    httpsProxy?: string;
    noProxy?: string;
    networkInterface?: string;
    globalCidr?: string;
  };
  prototypeSettings: {
    installTarget: string;
    availableNetworkInterfaces?: Array<{
      name: string;
    }>;
  };
  showAdvanced: boolean;
  onShowAdvancedChange: (show: boolean) => void;
  onInputChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onSelectChange: (e: React.ChangeEvent<HTMLSelectElement>) => void;
  availableNetworkInterfaces?: string[];
  fieldErrors?: Array<{ field: string; message: string }>;
}

const LinuxSetup: React.FC<LinuxSetupProps> = ({
  config,
  prototypeSettings,
  showAdvanced,
  onShowAdvancedChange,
  onInputChange,
  onSelectChange,
  availableNetworkInterfaces = [],
  fieldErrors = [],
}) => {
  const { title } = useBranding();

  const getFieldError = (fieldName: string) => {
    const fieldError = fieldErrors.find((err) => err.field === fieldName);
    return fieldError ? formatErrorMessage(fieldError.message) : undefined;
  };

  return (
    <div className="space-y-6" data-testid="linux-setup">
      <div className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">System Configuration</h2>
        <Input
          id="dataDirectory"
          label={fieldNames.dataDirectory}
          value={config.dataDirectory || ""}
          onChange={onInputChange}
          placeholder="/var/lib/embedded-cluster"
          helpText={`Directory where ${title} will store its data`}
          error={getFieldError("dataDirectory")}
          required
        />

        <Input
          id="adminConsolePort"
          label={fieldNames.adminConsolePort}
          value={config.adminConsolePort?.toString() || ""}
          onChange={onInputChange}
          placeholder="30000"
          helpText="Port for the Admin Console"
          error={getFieldError("adminConsolePort")}
          required
        />

        <Input
          id="localArtifactMirrorPort"
          label={fieldNames.localArtifactMirrorPort}
          value={config.localArtifactMirrorPort?.toString() || ""}
          onChange={onInputChange}
          placeholder="50000"
          helpText="Port for the local artifact mirror"
          error={getFieldError("localArtifactMirrorPort")}
          required
        />
      </div>

      <div className="space-y-4">
        <h2 className="text-lg font-medium text-gray-900">Proxy Configuration</h2>
        <div className="space-y-4">
          <Input
            id="httpProxy"
            label={fieldNames.httpProxy}
            value={config.httpProxy || ""}
            onChange={onInputChange}
            placeholder="http://proxy.example.com:3128"
            helpText="HTTP proxy server URL"
            error={getFieldError("httpProxy")}
          />

          <Input
            id="httpsProxy"
            label={fieldNames.httpsProxy}
            value={config.httpsProxy || ""}
            onChange={onInputChange}
            placeholder="https://proxy.example.com:3128"
            helpText="HTTPS proxy server URL"
            error={getFieldError("httpsProxy")}
          />

          <Input
            id="noProxy"
            label={fieldNames.noProxy}
            value={config.noProxy || ""}
            onChange={onInputChange}
            placeholder="localhost,127.0.0.1,.example.com"
            helpText="Comma-separated list of hosts to bypass the proxy"
            error={getFieldError("noProxy")}
          />
        </div>
      </div>

      <div className="pt-4">
        <button
          type="button"
          className="flex items-center text-lg font-medium text-gray-900 mb-4"
          onClick={() => onShowAdvancedChange(!showAdvanced)}
        >
          {showAdvanced ? <ChevronDown className="w-4 h-4 mr-1" /> : <ChevronRight className="w-4 h-4 mr-1" />}
          Advanced Settings
        </button>

        {showAdvanced && (
          <div className="space-y-6">
            <Select
              id="networkInterface"
              label={fieldNames.networkInterface}
              value={config.networkInterface || ""}
              onChange={onSelectChange}
              options={[
                ...(availableNetworkInterfaces.length > 0
                  ? availableNetworkInterfaces.map((iface) => ({
                      value: iface,
                      label: iface,
                    }))
                  : (prototypeSettings.availableNetworkInterfaces || []).map((iface) => ({
                      value: iface.name,
                      label: iface.name,
                    }))),
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
              onChange={onInputChange}
              placeholder="10.244.0.0/16"
              helpText="CIDR notation for the reserved network range (must be /16 or larger)"
              error={getFieldError("globalCidr")}
              required
            />
          </div>
        )}
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

export default LinuxSetup;
