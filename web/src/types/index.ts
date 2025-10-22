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

export type State = "Pending" | "Running" | "Succeeded" | "Failed";

// Linux Configuration Type used during the setup step
export interface LinuxConfig {
  adminConsolePort?: number;
  localArtifactMirrorPort?: number;
  dataDirectory: string;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  networkInterface?: string;
  globalCidr?: string;
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

// App Configuration Types
export interface AppConfig {
  groups: AppConfigGroup[];
}

export interface AppConfigGroup {
  name: string;
  title: string;
  description?: string;
  when?: string;
  items: AppConfigItem[];
}

export interface AppConfigItem {
  name: string;
  title: string;
  help_text?: string;
  error?: string;
  required?: boolean;
  readonly?: boolean;
  type: string;
  value?: string;
  default?: string;
  filename?: string;
  items?: AppConfigChildItem[];
}

export interface AppConfigChildItem {
  name: string;
  title: string;
  value?: string;
  default?: string;
}

export interface AppConfigValue {
  value: string;
  filename?: string;
}

export type AppConfigValues = Record<string, AppConfigValue>;
