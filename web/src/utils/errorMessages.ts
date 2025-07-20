/**
 * Field name mappings for different installation targets
 */
export const kubernetesFieldNames = {
  adminConsolePort: "Admin Console Port",
  httpProxy: "HTTP Proxy",
  httpsProxy: "HTTPS Proxy",
  noProxy: "Proxy Bypass List",
};

export const linuxFieldNames = {
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
};

/**
 * Formats error messages by replacing internal field names with user-friendly display names.
 * Example: "adminConsolePort" becomes "Admin Console Port".
 *
 * @param message - The error message to format
 * @param fieldNames - The field name mapping object to use
 * @returns The formatted error message with replaced field names
 */
export function formatErrorMessage(message: string, fieldNames: Record<string, string>) {
  let finalMsg = message;
  for (const [field, fieldName] of Object.entries(fieldNames)) {
    // Case-insensitive regex that matches whole words only
    // Example: "podCidr", "PodCidr", "PODCIDR" all become "Pod CIDR"
    finalMsg = finalMsg.replace(new RegExp(`\\b${field}\\b`, 'gi'), fieldName);
  }
  return finalMsg;
}