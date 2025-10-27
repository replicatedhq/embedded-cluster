import { InstallationTarget } from "./installation-target";
import { WizardMode } from "./wizard-mode";

// Window type with optional __INITIAL_STATE__ property
export type WindowWithInitialState = typeof window & {
  __INITIAL_STATE__?: unknown;
};

export interface InitialState {
  title: string;
  icon?: string;
  installTarget: InstallationTarget;
  mode: WizardMode;
  isAirgap: boolean;
  requiresInfraUpgrade: boolean;
}

// WizardText type holds the text fields for the multiple wizard step text fields
export interface WizardText {
  title: string;
  subtitle: string;
  welcomeTitle: string;
  welcomeDescription: string;
  welcomeButtonText: string;
  timelineTitle: string;
  configurationTitle: string;
  configurationDescription: string;
  linuxSetupTitle: string;
  linuxSetupDescription: string;
  kubernetesSetupTitle: string;
  kubernetesSetupDescription: string;
  kubernetesInstallationTitle: string;
  kubernetesInstallationDescription: string;
  linuxValidationTitle: string;
  linuxValidationDescription: string;
  linuxInstallationHeader: string;
  linuxInstallationTitle: string;
  linuxInstallationDescription: string;
  appValidationTitle: string;
  appValidationDescription: string;
  appInstallationLoadingTitle: string;
  appInstallationTitle: string;
  appInstallationDescription: string;
  appInstallationFailureTitle: string;
  appInstallationSuccessTitle: string;
  nextButtonText: string;
  completion: string;
}

// WizardStep type represents the different steps in the installation or upgrade wizard
export type WizardStep =
  | "welcome"
  | "configuration"
  | "linux-setup"
  | "kubernetes-setup"
  | "installation"
  | "linux-completion"
  | "kubernetes-completion";

// InstallationPhaseId type represents the different phases of the installation process
export type InstallationPhaseId =
  | "linux-preflight"
  | "linux-installation"
  | "airgap-processing"
  | "kubernetes-installation"
  | "app-preflight"
  | "app-installation";
