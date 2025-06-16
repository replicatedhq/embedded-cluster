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
   Cidr: "CIDR",
   CIDR: "CIDR"
}

/**
 * Formats error messages by replacing technical field names with more user-friendly display names.
 * For example, "adminConsolePort" becomes "Admin Console Port".
 *
 * @param message - The error message to format
 * @returns The formatted error message with replaced field names
 */
export function formatErrorMessage(message: string) {
   let finalMsg = message
   for (const [field, fieldName] of Object.entries(fieldNames)) {
      finalMsg = finalMsg.replaceAll(new RegExp(`\\b${field}\\b`, 'g'), fieldName)
   }
   return finalMsg
}