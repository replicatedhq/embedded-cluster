import { InstallationTarget } from "../types/installation-target";
import { WizardMode } from "../types/wizard-mode";

/**
 * Returns base API path for wizard operations
 * Currently returns install base for all modes until backend upgrade endpoints ready
 */
export const getApiBase = (target: InstallationTarget, mode: WizardMode): string => {
  // TODO: Switch to upgrade endpoints when available
  return `/api/${target}/${mode}`;
};