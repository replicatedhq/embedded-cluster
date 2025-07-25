
/**
 * Formats error messages by replacing technical field names with more user-friendly display names.
 * Example: "adminConsolePort" becomes "Admin Console Port".
 *
 * @param message - The error message to format
 * @returns The formatted error message with replaced field names
 */
export function formatErrorMessage(message: string, fieldNames: Record<string, string>) {
  let finalMsg = message
  for (const [field, fieldName] of Object.entries(fieldNames)) {
    // Case-insensitive regex that matches whole words only
    // Example: "podCidr", "PodCidr", "PODCIDR" all become "Pod CIDR"
    finalMsg = finalMsg.replace(new RegExp(`\\b${field}\\b`, 'gi'), fieldName)
  }
  return finalMsg
}
