import { createContext } from "react";
import { InstallationTarget } from "../../types/installation-target";

export type WizardMode = "install" | "upgrade";

export interface WizardText {
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

export interface WizardModeContextType {
  target: InstallationTarget;
  mode: WizardMode;
  text: WizardText;
}

export const getTextVariations = (isLinux: boolean, title: string): Record<WizardMode, WizardText> => ({
  install: {
    title: title || "",
    subtitle: "Installation Wizard",
    welcomeTitle: `Welcome to ${title}`,
    welcomeDescription: isLinux
      ? "Follow the guided installation to configure your environment and install on this Linux server."
      : "Follow the guided installation to configure your environment and install in your Kubernetes cluster.",
    configurationTitle: "Configuration",
    configurationDescription: "Configure the settings for your installation.",
    linuxSetupTitle: "System Configuration",
    linuxSetupDescription: "Configure your Linux server settings for the installation.",
    kubernetesSetupTitle: "Kubernetes Configuration",
    kubernetesSetupDescription: "Configure your Kubernetes cluster settings for the installation.",
    validationTitle: "Pre-Installation Checks",
    validationDescription: "Validating your configuration and system requirements.",
    installationTitle: "Installation",
    installationDescription: "Installing to your environment. This may take several minutes.",
    welcomeButtonText: "Get Started",
    nextButtonText: "Next",
  },
  upgrade: {
    title: title || "",
    subtitle: "Upgrade Wizard",
    welcomeTitle: `Upgrade ${title}`,
    welcomeDescription: isLinux
      ? "Follow the guided upgrade to update your installation on this Linux server."
      : "Follow the guided upgrade to update your installation in your Kubernetes cluster.",
    configurationTitle: "Configuration",
    configurationDescription: "Review and update the settings for your upgrade.",
    linuxSetupTitle: "System Configuration",
    linuxSetupDescription: "Review your Linux server settings for the upgrade.",
    kubernetesSetupTitle: "Kubernetes Configuration",
    kubernetesSetupDescription: "Review your Kubernetes cluster settings for the upgrade.",
    validationTitle: "Pre-Upgrade Checks",
    validationDescription: "Validating your configuration and system requirements.",
    installationTitle: "Upgrade",
    installationDescription: "Upgrading your installation. This may take several minutes.",
    welcomeButtonText: "Start Upgrade",
    nextButtonText: "Next",
  },
});

export const WizardContext = createContext<WizardModeContextType | undefined>(undefined);
