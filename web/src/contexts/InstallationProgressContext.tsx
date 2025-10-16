import { createContext, useContext } from "react";
import { WizardStep, InstallationPhaseId } from "../types";

export interface StoredInstallState {
  wizardStep: WizardStep;
  installationPhase?: InstallationPhaseId;
}

interface InstallationProgressContextType {
  wizardStep: WizardStep;
  setWizardStep: (step: WizardStep) => void;
  installationPhase: InstallationPhaseId | undefined;
  setInstallationPhase: (phase: InstallationPhaseId | undefined) => void;
  clearProgress: () => void;
}

export const InstallationProgressContext = createContext<InstallationProgressContextType | undefined>(undefined);

export const useInstallationProgress = () => {
  const context = useContext(InstallationProgressContext);
  if (context === undefined) {
    throw new Error("useInstallationProgress must be used within an InstallationProgressProvider");
  }
  return context;
};
