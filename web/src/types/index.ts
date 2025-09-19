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
}

export type State = "Pending" | "Running" | "Succeeded" | "Failed";

export interface InfraStatusResponse {
  components: InfraComponent[];
  status: InfraStatus;
  logs: string;
}

export interface InfraComponent {
  name: string;
  status: InfraStatus;
}

export interface InfraStatus {
  state: State;
  description: string;
  lastUpdated: string;
}

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

// Kubernetes Configuration Type used during the setup step
export interface KubernetesConfig {
  adminConsolePort?: number;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  installCommand?: string;
}

// WizardText type holds the text fields for the multiple wizard step text fields
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
  kubernetesInstallationTitle: string;
  kubernetesInstallationDescription: string;
  linuxValidationTitle: string;
  linuxValidationDescription: string;
  linuxInstallationTitle: string;
  linuxInstallationDescription: string;
  appValidationTitle: string;
  appValidationDescription: string;
  appInstallationTitle: string;
  appInstallationDescription: string;
  welcomeButtonText: string;
  nextButtonText: string;
}

export type WizardStep =
  | "welcome"
  | "configuration"
  | "linux-setup"
  | "kubernetes-setup"
  | "installation"
  | "linux-completion"
  | "kubernetes-completion";

// App Configuration Types
export interface AppConfig {
  groups: AppConfigGroup[];
}

export interface AppConfigGroup {
  name: string;
  title: string;
  description?: string;
  items: AppConfigItem[];
}

export interface AppConfigItem {
  name: string;
  title: string;
  help_text?: string;
  error?: string;
  required?: boolean;
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

// Preflight Types
export interface PreflightResult {
  title: string;
  message: string;
  strict?: boolean;
}

export interface PreflightOutput {
  pass: PreflightResult[];
  warn: PreflightResult[];
  fail: PreflightResult[];
}

export interface PreflightStatus {
  state: string;
  description: string;
  lastUpdated: string;
}

export interface HostPreflightResponse {
  titles: string[];
  output?: PreflightOutput;
  status?: PreflightStatus;
  allowIgnoreHostPreflights?: boolean;
}

export interface AppPreflightResponse {
  titles: string[];
  output?: PreflightOutput;
  status?: PreflightStatus;
  allowIgnoreAppPreflights?: boolean;
  hasStrictAppPreflightFailures?: boolean;
}

export interface AppInstallStatus {
  status: {
    state: State;
    description: string;
    lastUpdated: string;
  };
  logs: string;
}

export interface InstallationStatusResponse {
  description: string;
  lastUpdated: string;
  state: State;
}
