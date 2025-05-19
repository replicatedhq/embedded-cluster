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

export interface HostPreflightStatus {
  kernelVersion: ValidationResult | null;
  kernelParameters: ValidationResult | null;
  dataDirectory: ValidationResult | null;
  systemMemory: ValidationResult | null;
  systemCPU: ValidationResult | null;
  diskSpace: ValidationResult | null;
  selinux: ValidationResult | null;
  networkEndpoints: ValidationResult | null;
}

export interface NodeJoinStatus {
  id: string;
  phase: 'preflight' | 'joining';
  preflightStatus: HostPreflightStatus | null;
  progress: number;
  currentMessage: string;
  logs: string[];
  error?: string;
}

export interface K0sInstallStatus {
  phase: 'installing' | 'ready';
  progress: number;
  currentMessage: string;
  logs: string[];
  error?: string;
  joinToken?: string;
  nodes: NodeJoinStatus[];
}

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

export type WizardStep = 'welcome' | 'setup' | 'installation';

export interface ImagePushStatus {
  image: string;
  status: 'pending' | 'pushing' | 'complete' | 'failed';
  progress: number;
}

export type SetupPhase = 'configuration' | 'k0s-installation';