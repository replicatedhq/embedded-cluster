import React from "react";
import { WizardText } from "../types";
import { useInitialState } from "../contexts/InitialStateContext";
import { WizardContext } from "../contexts/WizardModeContext";
import { WizardMode } from "../types/wizard-mode";

const getTextVariations = (isLinux: boolean, title: string): Record<WizardMode, WizardText> => ({
  install: {
    title: title || "",
    subtitle: "Installation Wizard",
    welcomeTitle: `Welcome to ${title}`,
    welcomeDescription: `This wizard will guide you through installing ${title} on your ${isLinux ? "Linux machine" : "Kubernetes cluster"
      }.`,
    welcomeButtonText: "Start",
    timelineTitle: "Upgrade Progress",
    configurationTitle: 'Configuration',
    configurationDescription: `Configure your ${title} installation by providing the information below.`,
    linuxSetupTitle: "Setup",
    linuxSetupDescription: "Configure the host settings for this installation.",
    kubernetesSetupTitle: "Setup",
    kubernetesSetupDescription: "Configure the settings for this installation.",
    kubernetesInstallationTitle: "Infrastructure Installation",
    kubernetesInstallationDescription: "Installing infrastructure components",
    linuxValidationTitle: "Host Preflight Checks",
    linuxValidationDescription: "Validating the host requirements",
    linuxInstallationTitle: "Infrastructure Installation",
    linuxInstallationDescription: "Installing infrastructure components",
    appValidationTitle: `${title} Preflight Checks`,
    appValidationDescription: "Validating the application requirements",
    appInstallaionLoadingTitle: "Installing application...",
    appInstallationTitle: `${title} Installation`,
    appInstallationDescription: `Installing ${title} components`,
    appInstallaionFailureTitle: "Application installation failed",
    appInstallaionSuccessTitle: "Application installed successfully!",
    nextButtonText: "Next: Start Installation",
  },
  upgrade: {
    title: title || "",
    subtitle: "Upgrade Wizard",
    welcomeTitle: `Welcome to ${title}`,
    welcomeDescription: `This wizard will guide you through upgrading ${title} on your ${isLinux ? "Linux machine" : "Kubernetes cluster"
      }.`,
    welcomeButtonText: "Start Upgrade",
    timelineTitle: "Upgrade Progress",
    configurationTitle: 'Upgrade Configuration',
    configurationDescription: `Configure your ${title} installation by providing the information below.`,
    linuxSetupTitle: "Setup",
    linuxSetupDescription: "Set up the hosts to use for this upgrade.",
    kubernetesSetupTitle: "Setup",
    kubernetesSetupDescription: "Set up the cluster to use for this upgrade.",
    kubernetesInstallationTitle: "Infrastructure Upgrade",
    kubernetesInstallationDescription: "Upgrading infrastructure components",
    linuxValidationTitle: "Host Preflight Checks",
    linuxValidationDescription: "Validating the host requirements",
    linuxInstallationTitle: "Infrastructure Upgrade",
    linuxInstallationDescription: "Upgrading infrastructure components",
    // TODO Upgrade we're using the app preflights phase to just confirm the upgrade for now. We need to change back the title and description once those are in place
    appValidationTitle: `${title} Upgrade Confirmation`,
    appValidationDescription: "Confirm the app upgrade",
    appInstallaionLoadingTitle: "Upgrading application...",
    appInstallationTitle: `${title} Upgrade`,
    appInstallationDescription: `Upgrading ${title} components`,
    appInstallaionFailureTitle: "Application upgrade failed",
    appInstallaionSuccessTitle: "Application upgraded successfully!",
    nextButtonText: "Next: Start Upgrade",
  },
});

export const WizardProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { title, installTarget, mode } = useInitialState();
  const isLinux = installTarget === "linux";
  const text = getTextVariations(isLinux, title)[mode];

  return <WizardContext.Provider value={{ mode, target: installTarget, text }}>{children}</WizardContext.Provider>;
};
