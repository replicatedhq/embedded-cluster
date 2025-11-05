import type { components } from "../../../types/api";
import { formatErrorMessage } from "../../../utils/errorMessage";

type LinuxInstallationConfig = components["schemas"]["types.LinuxInstallationConfig"];

export interface FieldError {
  field: string;
  message: string;
}

export interface Status {
  state: string;
  description?: string;
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
  if (fieldId === "adminConsolePort" || fieldId === "localArtifactMirrorPort") {
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
  fieldErrors: FieldError[] | undefined,
  currentlyExpanded: boolean
): boolean {
  // If already expanded, keep it expanded
  if (currentlyExpanded) {
    return true;
  }

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