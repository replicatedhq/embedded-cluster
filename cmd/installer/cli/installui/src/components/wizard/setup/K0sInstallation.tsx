import React, { useState, useEffect } from 'react';
import { useConfig } from '../../../contexts/ConfigContext';
import { K0sInstallStatus } from '../../../types';
import { installK0s } from '../../../utils/k0s';
import { validateHostPreflights } from '../../../utils/validation';
import NodeMetrics from './NodeMetrics';
import NodeJoinSection from './k0s/NodeJoinSection';

interface K0sInstallationProps {
  onComplete: () => void;
}

type NodeRole = 'application' | 'database';

interface NodeCount {
  application: number;
  database: number;
}

interface PendingNode {
  name: string;
  type: NodeRole;
  preflightStatus: HostPreflightStatus;
  progress: number;
  currentMessage: string;
  logs: string[];
  error?: string;
}

const REQUIRED_NODES: NodeCount = {
  application: 3,
  database: 3,
};

const SINGLE_NODE: NodeCount = {
  application: 1,
  database: 0,
};

const baseNodeMetrics = {
  cpu: 45,
  memory: 60,
  storage: { used: 800, total: 2000 },
  dataPath: '/data/gitea'
};

const K0sInstallation: React.FC<K0sInstallationProps> = ({ onComplete }) => {
  const { config, prototypeSettings } = useConfig();
  const themeColor = prototypeSettings.themeColor;
  const isMultiNode = prototypeSettings.enableMultiNode;
  const [phase, setPhase] = useState<'preflight' | 'installing'>('preflight');
  const [status, setStatus] = useState<K0sInstallStatus>({
    phase: 'installing',
    progress: 0,
    currentMessage: '',
    logs: [],
    nodes: []
  });
  const [copied, setCopied] = useState(false);
  const [selectedRole, setSelectedRole] = useState<NodeRole>('application');
  const [joinedNodes, setJoinedNodes] = useState<NodeCount>({
    application: 0,
    database: 0,
  });
  const [nodeMetrics, setNodeMetrics] = useState({
    application: {},
    database: {}
  });
  const [pendingNodes, setPendingNodes] = useState<PendingNode[]>([{
    name: 'gitea-app-1',
    type: 'application',
    preflightStatus: {
      kernelVersion: null,
      kernelParameters: null,
      dataDirectory: null,
      systemMemory: null,
      systemCPU: null,
      diskSpace: null,
      selinux: null,
      networkEndpoints: null,
    },
    progress: 0,
    currentMessage: 'Running preflight checks...',
    logs: ['Starting preflight checks...']
  }]);

  useEffect(() => {
    if (phase === 'preflight') {
      startPreflightChecks();
    } else if (phase === 'installing') {
      startInstallation();
    }
  }, [phase]);

  useEffect(() => {
    const requiredNodes = isMultiNode ? REQUIRED_NODES : SINGLE_NODE;
    if (status.phase === 'ready' && (
      prototypeSettings.skipNodeValidation || 
      (joinedNodes.application >= requiredNodes.application && 
       joinedNodes.database >= requiredNodes.database)
    )) {
      onComplete();
    }
  }, [status.phase, joinedNodes, prototypeSettings.skipNodeValidation, isMultiNode]);

  const startPreflightChecks = async () => {
    try {
      const results = await validateHostPreflights(config);
      
      const hasErrors = Object.values(results).some(
        (result) => result && !result.success
      );

      if (hasErrors) {
        setPendingNodes(prev => prev.map(node => ({
          ...node,
          preflightStatus: results,
          progress: 100,
          currentMessage: 'Preflight checks failed. Please resolve the issues and try again.',
          logs: [...node.logs, 'Preflight checks failed'],
          error: 'Preflight checks failed'
        })));
        return;
      }

      setPendingNodes(prev => prev.map(node => ({
        ...node,
        preflightStatus: results,
        progress: 30,
        currentMessage: 'Preflight checks complete, starting k0s installation...',
        logs: [...node.logs, 'Preflight checks completed']
      })));

      setPhase('installing');
    } catch (error) {
      console.error('Preflight check error:', error);
      setPendingNodes(prev => prev.map(node => ({
        ...node,
        progress: 100,
        currentMessage: 'Preflight checks failed due to an error',
        logs: [...node.logs, 'Error during preflight checks'],
        error: 'Preflight checks failed'
      })));
    }
  };

  const startInstallation = async () => {
    try {
      await installK0s(config, (newStatus) => {
        setStatus(prev => {
          const updatedStatus = { ...prev, ...newStatus };
          if (updatedStatus.overall === 'completed') {
            setTimeout(() => setPhase('validating'), 500);
          }
          return updatedStatus;
        });
        
        setPendingNodes(prev => prev.map(node => ({
          ...node,
          progress: newStatus.progress || node.progress,
          currentMessage: newStatus.currentMessage || node.currentMessage,
          logs: [...node.logs, ...(newStatus.logs || [])]
        })));
      });

      setPendingNodes([]);
      setJoinedNodes({ application: 1, database: 0 });
      setNodeMetrics(prev => ({
        ...prev,
        application: {
          'gitea-app-1': baseNodeMetrics
        }
      }));
    } catch (error) {
      console.error('k0s installation error:', error);
    }
  };

  const copyJoinCommand = () => {
    const joinCommand = `sudo ./gitea-mastodon join 10.128.0.45:30000 ${
      selectedRole === 'application' ? 'EaKuL6cNeIlzMci3JdDU9Oi4' : 'Xm9pK4vRtY2wQn8sLj5uH7fB'
    }`;
    navigator.clipboard.writeText(joinCommand).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  const startNodePreflight = async (nodeName: string, nodeType: NodeRole) => {
    const newPendingNode: PendingNode = {
      name: nodeName,
      type: nodeType,
      preflightStatus: {
        kernelVersion: null,
        kernelParameters: null,
        dataDirectory: null,
        systemMemory: null,
        systemCPU: null,
        diskSpace: null,
        selinux: null,
        networkEndpoints: null,
      },
      progress: 0,
      currentMessage: 'Running preflight checks...',
      logs: ['Starting preflight checks...']
    };
    
    setPendingNodes(prev => [...prev, newPendingNode]);
    
    try {
      const results = await validateHostPreflights(config);
      const hasErrors = Object.values(results).some(
        (result) => result && !result.success
      );

      if (hasErrors) {
        setPendingNodes(prev => 
          prev.map(node => 
            node.name === nodeName 
              ? {
                  ...node,
                  preflightStatus: results,
                  progress: 100,
                  currentMessage: 'Preflight checks failed. Please resolve the issues and try again.',
                  logs: [...node.logs, 'Preflight checks failed'],
                  error: 'Preflight checks failed'
                }
              : node
          )
        );
        return;
      }

      setPendingNodes(prev => 
        prev.map(node => 
          node.name === nodeName 
            ? {
                ...node,
                preflightStatus: results,
                progress: 50,
                currentMessage: 'Preflight checks complete, joining cluster...',
                logs: [...node.logs, 'Preflight checks completed', 'Starting cluster join process...']
              }
            : node
        )
      );

      await new Promise(resolve => setTimeout(resolve, 2000));
      
      setPendingNodes(prev => prev.filter(node => node.name !== nodeName));
      handleNodeJoined();
    } catch (error) {
      console.error('Node preflight check error:', error);
      setPendingNodes(prev =>
        prev.map(node =>
          node.name === nodeName
            ? {
                ...node,
                progress: 100,
                currentMessage: 'Preflight checks failed due to an error',
                logs: [...node.logs, 'Error during preflight checks'],
                error: 'Preflight checks failed'
              }
            : node
        )
      );
    }
  };

  const handleNodeJoined = () => {
    const newCount = {
      ...joinedNodes,
      [selectedRole]: joinedNodes[selectedRole] + 1
    };
    setJoinedNodes(newCount);

    const nodeNumber = newCount[selectedRole];
    const nodeName = selectedRole === 'application' ? 
      `gitea-app-${nodeNumber}` : 
      `gitea-db-${nodeNumber}`;

    const newMetrics = {
      cpu: Math.floor(Math.random() * 30) + 35,
      memory: Math.floor(Math.random() * 20) + 55,
      storage: {
        used: Math.floor(Math.random() * 300) + 700,
        total: selectedRole === 'application' ? 2000 : 4000
      },
      dataPath: '/data/gitea'
    };

    setNodeMetrics(prev => ({
      ...prev,
      [selectedRole]: {
        ...prev[selectedRole],
        [nodeName]: newMetrics
      }
    }));
  };

  const handleStartNodeJoin = () => {
    const nodeNumber = joinedNodes[selectedRole] + 1;
    const nodeName = selectedRole === 'application' ? 
      `gitea-app-${nodeNumber}` : 
      `gitea-db-${nodeNumber}`;
    
    startNodePreflight(nodeName, selectedRole);
  };

  return (
    <div className="space-y-6">
      {status.phase === 'ready' && isMultiNode && (
        <NodeJoinSection
          selectedRole={selectedRole}
          joinedNodes={joinedNodes}
          requiredNodes={REQUIRED_NODES}
          onRoleChange={setSelectedRole}
          onCopyCommand={copyJoinCommand}
          onStartNodeJoin={handleStartNodeJoin}
          copied={copied}
          themeColor={themeColor}
          skipNodeValidation={prototypeSettings.skipNodeValidation}
        />
      )}
      
      <NodeMetrics 
        nodes={nodeMetrics} 
        pendingNodes={pendingNodes}
        isMultiNode={isMultiNode}
      />
    </div>
  );
};

export default K0sInstallation;