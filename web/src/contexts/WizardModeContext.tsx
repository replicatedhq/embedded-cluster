import { createContext, useContext } from "react";
import { InstallationTarget } from "../types/installation-target";
import { WizardText } from "../types";
import { WizardMode } from "../types/wizard-mode";

interface WizardModeContextType {
  target: InstallationTarget;
  mode: WizardMode;
  text: WizardText;
  isAirgap: boolean;
  requiresInfraUpgrade: boolean;
}

export const WizardContext = createContext<WizardModeContextType | undefined>(undefined);

export const useWizard = (): WizardModeContextType => {
  const context = useContext(WizardContext);
  if (context === undefined) {
    throw new Error("useWizardMode must be used within a WizardProvider");
  }
  return context;
};
