import { ClusterConfig } from '../contexts/ConfigContext';
import { ValidationStatus } from '../types';

export const validateEnvironment = async (config: ClusterConfig): Promise<ValidationStatus> => {
  const validationStatus: ValidationStatus = {
    kubernetes: null,
    helm: null,
    storage: null,
    networking: null,
    permissions: null,
  };

  // Kubernetes check completes after 1 second
  await new Promise(resolve => setTimeout(resolve, 1000));
  validationStatus.kubernetes = {
    success: true,
    message: 'Kubernetes cluster is accessible and running version 1.24.0',
  };

  // Other checks complete after 2 more seconds
  await new Promise(resolve => setTimeout(resolve, 2000));

  // Get prototype settings
  const prototypeSettings = JSON.parse(localStorage.getItem('gitea-prototype-settings') || '{}');
  const shouldFail = prototypeSettings.failPreflights;

  if (shouldFail) {
    validationStatus.helm = {
      success: true,
      message: 'Helm version 3.8.0 detected',
    };

    validationStatus.storage = {
      success: false,
      message: `Storage class "${config.storageClass}" not found. Please create the storage class or select a different one.`,
    };

    validationStatus.networking = {
      success: false,
      message: 'Ingress controller not detected. Install an ingress controller (e.g., nginx-ingress) to enable external access.',
    };

    validationStatus.permissions = {
      success: true,
      message: 'The current user has sufficient permissions in the namespace',
    };
  } else {
    validationStatus.helm = {
      success: true,
      message: 'Helm version 3.8.0 detected',
    };

    validationStatus.storage = {
      success: true,
      message: `Storage class "${config.storageClass}" is available with dynamic provisioning support`,
    };

    validationStatus.networking = {
      success: true,
      message: 'All networking prerequisites verified successfully',
    };

    validationStatus.permissions = {
      success: true,
      message: 'The current user has sufficient permissions in the namespace',
    };
  }

  return validationStatus;
};

// Add these interfaces to match the new structure
interface PreflightResult {
  title: string;
  message: string;
}

interface PreflightOutput {
  pass: PreflightResult[];
  warn: PreflightResult[];
  fail: PreflightResult[];
}

interface PreflightStatus {
  state: string;
  description: string;
  lastUpdated: string;
}

interface PreflightResponse {
  status: PreflightStatus;
  output?: PreflightOutput;
}

export const validateHostPreflights = async (config: ClusterConfig): Promise<PreflightResponse> => {
  // Simulate status
  const status: PreflightStatus = {
    state: 'Succeeded',
    description: 'Preflight checks completed',
    lastUpdated: new Date().toISOString(),
  };

  // Get prototype settings
  const prototypeSettings = JSON.parse(localStorage.getItem('gitea-prototype-settings') || '{}');
  const shouldFail = prototypeSettings.failHostPreflights;

  // Simulate preflight checks
  await new Promise(resolve => setTimeout(resolve, 2000));

  let output: PreflightOutput = {
    pass: [],
    warn: [],
    fail: [],
  };

  if (shouldFail) {
    output.fail.push(
      { title: 'Kernel Version', message: 'Kernel version 3.10.0 is not supported. Please upgrade to kernel version 4.15.0 or later.' },
      { title: 'Kernel Parameters', message: 'Required kernel parameter net.bridge.bridge-nf-call-iptables=1 is not set. Run: sysctl -w net.bridge.bridge-nf-call-iptables=1 and add to /etc/sysctl.conf' },
      { title: 'Data Directory', message: 'Data directory is a symbolic link. Please use a real directory path for data storage.' },
      { title: 'System Memory', message: 'Insufficient memory: 4GB available, minimum 8GB required. Add more memory to meet the requirements.' },
      { title: 'CPU Resources', message: 'Insufficient CPU cores: 2 cores available, minimum 4 cores required. Add more CPU resources to meet the requirements.' },
      { title: 'Disk Space', message: 'Insufficient disk space: 5GB available, minimum 20GB required. Free up space or add more storage.' },
      { title: 'SELinux Status', message: "SELinux must be disabled or run in permissive mode. To run SELinux in permissive mode, edit /etc/selinux/config, change the line 'SELINUX=enforcing' to 'SELINUX=permissive', save the file, and reboot. You can run getenforce to verify the change." },
      { title: 'Network Connectivity', message: 'Cannot reach required network endpoints. Check firewall rules and DNS resolution for registry.gitea.com.' },
    );
  } else {
    output.pass.push(
      { title: 'Kernel Version', message: 'Kernel version 5.15.0 meets the minimum requirement of 4.15.0' },
      { title: 'Kernel Parameters', message: 'All required kernel parameters are configured correctly' },
      { title: 'Data Directory', message: 'Data directory is a valid path with correct permissions' },
      { title: 'System Memory', message: '16GB RAM available, exceeds minimum requirement of 8GB' },
      { title: 'CPU Resources', message: '4 CPU cores available, meets minimum requirement' },
      { title: 'Disk Space', message: '50GB disk space available, exceeds minimum requirement of 20GB' },
      { title: 'SELinux Status', message: 'SELinux is in permissive mode as required' },
      { title: 'Network Connectivity', message: 'All required network endpoints are accessible' },
    );
  }

  return { status, output };
};

export type HostPreflightStatus = Record<string, { success: boolean; message: string } | null>;