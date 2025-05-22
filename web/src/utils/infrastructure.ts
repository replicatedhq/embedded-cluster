import { ClusterConfig } from '../contexts/ConfigContext';
import { InstallationStatus } from '../types';

export const setupInfrastructure = async (
  config: ClusterConfig,
  onStatusUpdate: (status: Partial<InstallationStatus>) => void
): Promise<void> => {
  const status: InstallationStatus = {
    openebs: 'pending',
    registry: 'pending',
    velero: 'pending',
    components: 'pending',
    database: 'pending',
    core: 'pending',
    plugins: 'pending',
    overall: 'pending',
    currentMessage: '',
    logs: [],
    progress: 0
  };

  const addLogs = (newLogs: string[]) => {
    status.logs = [...status.logs, ...newLogs];
    onStatusUpdate({ logs: status.logs });
  };

  // Create values files for each chart
  addLogs(['Creating directory for Helm values files...']);
  await new Promise(resolve => setTimeout(resolve, 1000));

  // OpenEBS values
  const openebsValues = `
storageClass:
  name: openebs-local
  isDefaultClass: true
`;

  addLogs([
    'Creating OpenEBS values file...',
    openebsValues
  ]);

  // Install OpenEBS
  onStatusUpdate({ 
    openebs: 'in-progress', 
    currentMessage: 'Installing storage...',
    progress: 20
  });

  addLogs([
    'Installing OpenEBS...',
    'Creating namespace...',
    'Waiting for OpenEBS deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['OpenEBS installation complete']);

  onStatusUpdate({ 
    openebs: 'completed', 
    currentMessage: 'OpenEBS installation completed',
    progress: 35
  });

  // Registry values
  const registryValues = `
persistence:
  enabled: true
  storageClass: openebs-local
  size: 10Gi
service:
  type: ClusterIP
`;

  addLogs([
    'Creating Registry values file...',
    registryValues
  ]);

  // Install Registry
  onStatusUpdate({ 
    registry: 'in-progress', 
    currentMessage: 'Installing registry...',
    progress: 50
  });

  addLogs([
    'Installing Docker Registry...',
    'Creating namespace...',
    'Waiting for Registry deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['Registry installation complete']);

  onStatusUpdate({ 
    registry: 'completed', 
    currentMessage: 'Registry installation completed',
    progress: 65
  });

  // Velero values
  const veleroValues = `
configuration:
  provider: aws
  backupStorageLocation:
    name: default
    bucket: gitea-backups
    config:
      region: us-east-1
persistence:
  enabled: true
  storageClass: openebs-local
`;

  addLogs([
    'Creating Velero values file...',
    veleroValues
  ]);

  // Install Velero
  onStatusUpdate({ 
    velero: 'in-progress', 
    currentMessage: 'Preparing disaster recovery...',
    progress: 75
  });

  addLogs([
    'Installing Velero...',
    'Creating namespace...',
    'Waiting for Velero deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['Velero installation complete']);

  onStatusUpdate({ 
    velero: 'completed', 
    currentMessage: 'Velero installation completed',
    progress: 85
  });

  // Install additional components
  onStatusUpdate({ 
    components: 'in-progress', 
    currentMessage: 'Installing additional components...',
    progress: 90
  });

  addLogs([
    'Installing Ingress Controller...',
    'Creating ingress-nginx namespace...',
    'Waiting for Ingress Controller deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 2000));
  addLogs(['Ingress Controller installation complete']);

  addLogs([
    'Installing Metrics Server...',
    'Creating metrics-server namespace...',
    'Waiting for Metrics Server deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 2000));
  addLogs(['Metrics Server installation complete']);

  addLogs([
    'Installing Cert Manager...',
    'Creating cert-manager namespace...',
    'Waiting for Cert Manager deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 2000));
  addLogs(['Cert Manager installation complete']);

  onStatusUpdate({ 
    components: 'completed', 
    currentMessage: 'Components installation completed',
    progress: 100,
    overall: 'completed'
  });
};