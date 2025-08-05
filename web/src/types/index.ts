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




// todo (screspod): clean this up
export interface ValidationResult {
  success: boolean;
  message: string;
  details?: string;
}

export interface ValidationStatus {
  kubernetes: ValidationResult | null;
  helm: ValidationResult | null;
  storage: ValidationResult | null;
  networking: ValidationResult | null;
  permissions: ValidationResult | null;
}

// export interface HostPreflightStatus {
//   kernelVersion: ValidationResult | null;
//   kernelParameters: ValidationResult | null;
//   dataDirectory: ValidationResult | null;
//   systemMemory: ValidationResult | null;
//   systemCPU: ValidationResult | null;
//   diskSpace: ValidationResult | null;
//   selinux: ValidationResult | null;
//   networkEndpoints: ValidationResult | null;
// }

// export interface NodeJoinStatus {
//   id: string;
//   phase: 'preflight' | 'joining';
//   preflightStatus: HostPreflightStatus | null;
//   progress: number;
//   currentMessage: string;
//   logs: string[];
//   error?: string;
// }

// export interface K0sInstallStatus {
//   phase: 'installing' | 'ready';
//   progress: number;
//   currentMessage: string;
//   logs: string[];
//   error?: string;
//   joinToken?: string;
//   nodes: NodeJoinStatus[];
// }

export interface InstallationStatus {
  openebs: 'pending' | 'in-progress' | 'completed' | 'failed';
  registry: 'pending' | 'in-progress' | 'completed' | 'failed';
  velero: 'pending' | 'in-progress' | 'completed' | 'failed';
  components: 'pending' | 'in-progress' | 'completed' | 'failed';
  database: 'pending' | 'in-progress' | 'completed' | 'failed';
  core: 'pending' | 'in-progress' | 'completed' | 'failed';
  plugins: 'pending' | 'in-progress' | 'completed' | 'failed';
  overall: 'pending' | 'in-progress' | 'completed' | 'failed';
  currentMessage: string;
  error?: string;
  logs: string[];
  progress: number;
}

// export type WizardStep = 'welcome' | 'configuration' | 'setup' | 'installation' | 'completion';

// export interface ImagePushStatus {
//   image: string;
//   status: 'pending' | 'pushing' | 'complete' | 'failed';
//   progress: number;
// }

// export type SetupPhase = 'configuration' | 'k0s-installation';