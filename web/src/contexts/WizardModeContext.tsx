import React, { createContext, useContext } from "react";
import { useBranding } from "./BrandingContext";

export type WizardMode = "install" | "upgrade";

interface WizardText {
  title: string;
  subtitle: string;
  welcomeTitle: string;
  welcomeDescription: string;
  setupTitle: string;
  setupDescription: string;
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
    welcomeDescription: `This wizard will guide you through installing ${title} on your ${
      isLinux ? "Linux machine" : "Kubernetes cluster"
    }.`,
    setupTitle: "Setup",
    setupDescription: "Configure the host settings for this installation.",
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
    welcomeDescription: `This wizard will guide you through upgrading ${title} on your ${
      isLinux ? "Linux machine" : "Kubernetes cluster"
    }.`,
    setupTitle: "Setup",
    setupDescription: "Set up the hosts to use for this upgrade.",
    validationTitle: "Validation",
    validationDescription: "Validate the host requirements before proceeding with the upgrade.",
    installationTitle: `Upgrading ${title}`,
    installationDescription: "",
    welcomeButtonText: "Start Upgrade",
    nextButtonText: "Next: Start Upgrade",
  },
});

interface WizardModeContextType {
  mode: WizardMode;
  text: WizardText;
}

export const WizardModeContext = createContext<WizardModeContextType | undefined>(undefined);

export const WizardModeProvider: React.FC<{
  children: React.ReactNode;
  mode: WizardMode;
}> = ({ children, mode }) => {
  // __INITIAL_STATE__ is a global variable that can be set by the server-side rendering process
  // as a way to pass initial data to the client.
  const initialState = window.__INITIAL_STATE__ || {};
  const { title } = useBranding();
  const isLinux = initialState.installTarget === "linux";
  const text = getTextVariations(isLinux, title)[mode];

  return <WizardModeContext.Provider value={{ mode, text }}>{children}</WizardModeContext.Provider>;
};

export const useWizardMode = (): WizardModeContextType => {
  const context = useContext(WizardModeContext);
  if (context === undefined) {
    throw new Error("useWizardMode must be used within a WizardModeProvider");
  }
  return context;
};
