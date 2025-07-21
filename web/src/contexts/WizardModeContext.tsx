import { createContext, useContext } from "react";
import { InstallationTarget } from "../types/installation-target";

export type WizardMode = "install" | "upgrade";

interface WizardText {
  title: string;
  subtitle: string;
  welcomeTitle: string;
  welcomeDescription: string;
  configurationTitle: string;
  configurationDescription: string;
  linuxSetupTitle: string;
  linuxSetupDescription: string;
  kubernetesSetupTitle: string;
  kubernetesSetupDescription: string;
  validationTitle: string;
  validationDescription: string;
  installationTitle: string;
  installationDescription: string;
  welcomeButtonText: string;
  nextButtonText: string;
}

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
