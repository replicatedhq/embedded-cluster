export interface InfraStatusResponse {
  components: { [key: string]: InfraComponent };
  status: InfraStatus;
  logs: string[];
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

export type WizardStep = 'welcome' | 'setup' | 'validation' | 'installation' | 'completion';
