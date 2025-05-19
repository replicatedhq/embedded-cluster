import { ClusterConfig } from '../contexts/ConfigContext';
import { K0sInstallStatus, NodeJoinStatus, HostPreflightStatus } from '../types';
import { validateHostPreflights } from './validation';

export const installK0s = async (
  config: ClusterConfig,
  onStatusUpdate: (status: Partial<K0sInstallStatus>) => void
): Promise<void> => {
  const status: K0sInstallStatus = {
    phase: 'installing',
    progress: 0,
    currentMessage: 'Starting k0s installation...',
    logs: [],
    nodes: []
  };

  const addLogs = (newLogs: string[]) => {
    status.logs = [...status.logs, ...newLogs];
    onStatusUpdate({ logs: status.logs });
  };

  // Start k0s installation
  onStatusUpdate({
    phase: 'installing',
    currentMessage: 'Installing k0s...',
    progress: 30
  });

  addLogs([
    'Downloading k0s...',
    'Installing k0s binary...',
    'Creating k0s configuration...'
  ]);

  await new Promise(resolve => setTimeout(resolve, 2000));
  addLogs(['k0s binary installed']);

  onStatusUpdate({
    currentMessage: 'Starting k0s services...',
    progress: 60
  });

  addLogs([
    'Creating systemd service...',
    'Starting k0s controller...',
    'Waiting for k0s to be ready...'
  ]);

  await new Promise(resolve => setTimeout(resolve, 3000));
  addLogs(['k0s services started']);

  onStatusUpdate({
    currentMessage: 'Configuring k0s...',
    progress: 80
  });

  addLogs([
    'Configuring network...',
    'Setting up storage...',
    'Initializing control plane...'
  ]);

  await new Promise(resolve => setTimeout(resolve, 2000));
  addLogs(['k0s configuration complete']);

  // Generate join token
  const joinToken = 'k0s-token.SAMPLE.TOKEN.HERE';

  onStatusUpdate({
    phase: 'ready',
    currentMessage: 'k0s installation completed',
    progress: 100,
    joinToken
  });

  addLogs([
    'k0s installation successful',
    'Join token generated successfully'
  ]);
};

export const joinNode = async (
  config: ClusterConfig,
  nodeId: string,
  onStatusUpdate: (nodeId: string, status: Partial<NodeJoinStatus>) => void
): Promise<void> => {
  const status: NodeJoinStatus = {
    id: nodeId,
    phase: 'preflight',
    preflightStatus: null,
    progress: 0,
    currentMessage: 'Starting node join process...',
    logs: []
  };

  const addLogs = (newLogs: string[]) => {
    status.logs = [...status.logs, ...newLogs];
    onStatusUpdate(nodeId, { logs: status.logs });
  };

  // Run host preflights
  onStatusUpdate(nodeId, {
    phase: 'preflight',
    currentMessage: 'Running preflight checks...',
    progress: 10
  });

  addLogs(['Starting preflight checks...']);

  try {
    const preflightResults = await validateHostPreflights(config);
    status.preflightStatus = preflightResults;

    const hasErrors = Object.values(preflightResults).some(
      (result) => result && !result.success
    );

    if (hasErrors) {
      onStatusUpdate(nodeId, {
        preflightStatus: preflightResults,
        error: 'Preflight checks failed. Please resolve the issues and try again.',
        progress: 20
      });
      return;
    }

    // Start node join process
    onStatusUpdate(nodeId, {
      phase: 'joining',
      currentMessage: 'Starting node join process...',
      progress: 30
    });

    addLogs([
      'Preflight checks passed',
      'Downloading k0s worker binary...',
      'Installing k0s worker...'
    ]);

    await new Promise(resolve => setTimeout(resolve, 2000));
    addLogs(['k0s worker binary installed']);

    onStatusUpdate(nodeId, {
      currentMessage: 'Joining cluster...',
      progress: 60
    });

    addLogs([
      'Creating worker configuration...',
      'Starting k0s worker service...',
      'Joining cluster...'
    ]);

    await new Promise(resolve => setTimeout(resolve, 3000));
    addLogs(['Node joined successfully']);

    onStatusUpdate(nodeId, {
      currentMessage: 'Node joined successfully',
      progress: 100
    });

  } catch (error) {
    console.error('Node join error:', error);
    onStatusUpdate(nodeId, {
      error: error instanceof Error ? error.message : 'Unknown error occurred',
      progress: status.progress
    });
  }
};