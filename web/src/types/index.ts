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

export type WizardStep = 'welcome' | 'linux-setup' | 'kubernetes-setup' | 'linux-validation' | 'linux-installation' | 'kubernetes-installation' | 'linux-completion' | 'kubernetes-completion';
