import React, { createContext, useContext } from "react";
import { useInitialState } from "./InitialStateContext";
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
  validationTitle: string;
  validationDescription: string;
  installationTitle: string;
  installationDescription: string;
  welcomeButtonText: string;
  nextButtonText: string;
}

const getTextVariations = (isLinux: boolean, title: string): Record<WizardMode, WizardText> => ({
  install: {
    title: title || "",
    subtitle: "Installation Wizard",
    welcomeTitle: `Welcome to ${title}`,
    welcomeDescription: `This wizard will guide you through installing ${title} on your ${isLinux ? "Linux machine" : "Kubernetes cluster"
      }.`,
    configurationTitle: 'Configuration',
    configurationDescription: `Configure your ${title} installation by providing the information below.`,
    linuxSetupTitle: "Setup",
    linuxSetupDescription: "Configure the host settings for this installation.",
    validationTitle: "Validation",
    validationDescription: "Validate the host requirements before proceeding with installation.",
    installationTitle: `Installing ${title}`,
    installationDescription: "",
    welcomeButtonText: "Start",
    nextButtonText: "Next: Start Installation",
  },
  upgrade: {
    title: title || "",
    subtitle: "Upgrade Wizard",
    welcomeTitle: `Welcome to ${title}`,
    welcomeDescription: `This wizard will guide you through upgrading ${title} on your ${isLinux ? "Linux machine" : "Kubernetes cluster"
      }.`,
    configurationTitle: 'Upgrade Configuration',
    configurationDescription: `Configure your ${title} installation by providing the information below.`,
    linuxSetupTitle: "Setup",
    linuxSetupDescription: "Set up the hosts to use for this upgrade.",
    validationTitle: "Validation",
    validationDescription: "Validate the host requirements before proceeding with the upgrade.",
    installationTitle: `Upgrading ${title}`,
    installationDescription: "",
    welcomeButtonText: "Start Upgrade",
    nextButtonText: "Next: Start Upgrade",
  },
});

interface WizardModeContextType {
  target: InstallationTarget;
  mode: WizardMode;
  text: WizardText;
}

export const WizardContext = createContext<WizardModeContextType | undefined>(undefined);

export const WizardProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { title, installTarget } = useInitialState();
  const mode = "install"; // TODO: get mode from initial state
  const isLinux = installTarget === "linux";
  const text = getTextVariations(isLinux, title)[mode];

  return <WizardContext.Provider value={{ mode, target: installTarget, text }}>{children}</WizardContext.Provider>;
};

export const useWizard = (): WizardModeContextType => {
  const context = useContext(WizardContext);
  if (context === undefined) {
    throw new Error("useWizardMode must be used within a WizardProvider");
  }
  return context;
};
