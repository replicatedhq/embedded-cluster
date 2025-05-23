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

export type WizardStep = 'welcome' | 'setup' | 'validation' | 'installation' | 'completion';
