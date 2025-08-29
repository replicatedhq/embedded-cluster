import { InstallationTarget } from './installation-target';

export interface InitialState {
  title: string;
  icon?: string;
  installTarget: InstallationTarget;
}

export type State = 'Pending' | 'Running' | 'Succeeded' | 'Failed';

export interface Status {
  state: State;
  description: string;
  lastUpdated: string;
}

export interface InfraStatusResponse {
  components: InfraComponent[];
  status: Status;
  logs: string;
}

export interface InfraComponent {
  name: string;
  status: Status;
}

export type WizardStep = 'welcome' | 'configuration' | 'linux-setup' | 'kubernetes-setup' | 'installation' | 'linux-completion' | 'kubernetes-completion';

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
}

export interface PreflightOutput {
  pass: PreflightResult[];
  warn: PreflightResult[];
  fail: PreflightResult[];
}

export interface HostPreflightResponse {
  titles: string[];
  output?: PreflightOutput;
  status?: Status;
  allowIgnoreHostPreflights?: boolean;
}

export interface AppPreflightResponse {
  titles: string[];
  output?: PreflightOutput;
  status?: Status;
  allowIgnoreAppPreflights?: boolean;
}

export interface AppInstallStatusResponse {
  components: AppComponent[];
  status: Status;
  logs: string;
}

export interface AppComponent {
  name: string;
  status: Status;
}

export type InstallationStatusResponse = Status;
