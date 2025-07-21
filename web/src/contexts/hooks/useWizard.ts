import { useContext } from "react";
import { WizardContext, WizardModeContextType } from "../definitions/WizardModeContext";

export const useWizard = (): WizardModeContextType => {
  const context = useContext(WizardContext);
  if (context === undefined) {
    throw new Error("useWizard must be used within a WizardProvider");
  }
  return context;
};
