import { InstallationTarget } from "../types/installation-target";
import { WizardMode } from "../types/wizard-mode";

/**
 * Returns base API path for wizard operations
 * Dynamically routes to install or upgrade endpoints based on mode
 */
export const getApiBase = (target: InstallationTarget, mode: WizardMode): string => {
  return `/api/${target}/${mode}`;
};