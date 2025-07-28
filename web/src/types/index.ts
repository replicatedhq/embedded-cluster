import { InstallationTarget } from './installation-target';

export interface InitialState {
  title: string;
  icon?: string;
  installTarget: InstallationTarget;
}

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
  state: 'Pending' | 'Running' | 'Succeeded' | 'Failed';
  description: string;
  lastUpdated: string;
}

export type WizardStep = 'welcome' | 'configuration' | 'linux-setup' | 'kubernetes-setup' | 'linux-validation' | 'linux-installation' | 'kubernetes-installation' | 'linux-completion' | 'kubernetes-completion';

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
