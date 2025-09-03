import { createContext, useContext } from "react";
import { InstallationTarget } from "../types/installation-target";
import { WizardMode, WizardText } from "../types";

interface WizardModeContextType {
  target: InstallationTarget;
  mode: WizardMode;
  text: WizardText;
}

export const WizardContext = createContext<WizardModeContextType | undefined>(undefined);

export const useWizard = (): WizardModeContextType => {
  const context = useContext(WizardContext);
  if (context === undefined) {
    throw new Error("useWizardMode must be used within a WizardProvider");
  }
  return context;
};
