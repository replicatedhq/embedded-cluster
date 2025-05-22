import { ClusterConfig } from '../contexts/ConfigContext';
import { InstallationStatus } from '../types';

export const installGitea = async (
  config: ClusterConfig,
  onStatusUpdate: (status: Partial<InstallationStatus>) => void
): Promise<void> => {
  const installStatus: InstallationStatus = {
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

  const prototypeSettings = JSON.parse(localStorage.getItem('gitea-prototype-settings') || '{}');
  const shouldFail = prototypeSettings.failInstallation;

  const addLogs = (newLogs: string[]) => {
    installStatus.logs = [...installStatus.logs, ...newLogs];
    onStatusUpdate({ logs: installStatus.logs });
  };

  // Create PostgreSQL values file
  const postgresValues = `
global:
  postgresql:
    auth:
      username: gitea
      password: gitea
      database: gitea
persistence:
  enabled: true
  storageClass: ${config.storageClass}
  size: 10Gi
`;

  addLogs([
    'Creating PostgreSQL configuration...',
    postgresValues
  ]);

  // Start with the database installation
  onStatusUpdate({ 
    database: 'in-progress', 
    currentMessage: 'Installing PostgreSQL database...',
    progress: 5
  });

  addLogs([
    'Installing PostgreSQL...',
    'Creating database namespace...',
    'Waiting for PostgreSQL deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['PostgreSQL installation complete']);

  onStatusUpdate({ 
    database: 'completed', 
    currentMessage: 'Database installation completed',
    progress: 30
  });

  // Start core installation
  onStatusUpdate({ 
    core: 'in-progress', 
    currentMessage: 'Installing Gitea Core...',
    progress: 35
  });

  if (shouldFail) {
    onStatusUpdate({
      core: 'failed',
      plugins: 'pending',
      overall: 'failed',
      currentMessage: 'Installation failed',
      error: 'Failed to create Gitea deployment: insufficient memory resources',
      progress: 45,
      logs: [
        ...installStatus.logs,
        'Error: Deployment failed',
        'Error: pods "gitea-0" failed to fit in node',
        'Error: 0/3 nodes are available: 3 Insufficient memory'
      ]
    });
    return;
  }

  // Create Gitea values file
  const giteaValues = `
gitea:
  admin:
    username: ${config.adminUsername}
    password: ${config.adminPassword}
    email: ${config.adminEmail}
  config:
    server:
      DOMAIN: ${config.domain}
      ROOT_URL: ${config.useHttps ? 'https' : 'http'}://${config.domain}
      PROTOCOL: ${config.useHttps ? 'https' : 'http'}
    database:
      DB_TYPE: postgres
      HOST: gitea-db-postgresql.${config.namespace}.svc.cluster.local:5432
      NAME: gitea
      USER: gitea
      PASSWD: gitea
persistence:
  enabled: true
  storageClass: ${config.storageClass}
  size: 10Gi
ingress:
  enabled: true
  annotations:
    kubernetes.io/ingress.class: nginx
  hosts:
    - host: ${config.domain}
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: gitea-tls
      hosts:
        - ${config.domain}
`;

  addLogs([
    'Creating Gitea configuration...',
    giteaValues
  ]);
  
  addLogs([
    'Installing Gitea Enterprise...',
    'Creating application namespace...',
    'Waiting for Gitea deployment...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 5000));
  addLogs(['Gitea Enterprise deployment complete']);

  onStatusUpdate({ 
    core: 'completed', 
    currentMessage: 'Gitea Core installation completed',
    progress: 75
  });

  // Create plugins values file
  const pluginsValues = `
gitea:
  plugins:
    enabled: true
    marketplace:
      enabled: true
    preinstalled:
      - name: gitea-actions
        version: latest
      - name: gitea-oauth2-proxy
        version: latest
`;

  addLogs([
    'Creating plugins configuration...',
    pluginsValues
  ]);

  // Install plugins via Helm values
  onStatusUpdate({ 
    plugins: 'in-progress', 
    currentMessage: 'Installing Gitea Enterprise plugins...',
    progress: 80
  });

  addLogs([
    'Installing Gitea Enterprise plugins...',
    'Configuring plugin marketplace...',
    'Installing preinstalled plugins...'
  ]);
  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['Plugins installation complete']);

  onStatusUpdate({ 
    plugins: 'completed', 
    currentMessage: 'Plugins installation completed',
    progress: 95
  });

  // Final verification
  onStatusUpdate({ 
    currentMessage: 'Performing final verification...',
    progress: 98
  });

  addLogs([
    'Verifying installation...',
    'NAME: gitea',
    'LAST DEPLOYED: ' + new Date().toISOString(),
    'NAMESPACE: ' + config.namespace,
    'STATUS: deployed',
    'REVISION: 1'
  ]);

  // Set all components as completed and overall status
  onStatusUpdate({ 
    database: 'completed',
    core: 'completed',
    plugins: 'completed',
    overall: 'completed',
    currentMessage: 'Gitea Enterprise installation completed successfully',
    logs: ['Gitea Enterprise is ready to use!'],
    progress: 100
  });
};