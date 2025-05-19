import React from 'react';

interface KubernetesSetupProps {
  config: {
    hostNetworkInterface?: string;
    clusterNetworkCIDR?: string;
  };
  onInputChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
}

const KubernetesSetup: React.FC<KubernetesSetupProps> = ({
  config,
  onInputChange,
}) => (
  <div className="space-y-6">
    <div className="space-y-4">
      <h3 className="text-lg font-medium text-gray-900">Network Settings</h3>
      <p className="text-sm text-gray-600">
        Configure network settings for your Kubernetes cluster.
      </p>
      <div className="space-y-4">
        <input
          id="hostNetworkInterface"
          type="text"
          value={config.hostNetworkInterface || ''}
          onChange={onInputChange}
          placeholder="Network Interface"
          className="w-full px-3 py-2 border border-gray-300 rounded-md"
        />
        <input
          id="clusterNetworkCIDR"
          type="text"
          value={config.clusterNetworkCIDR || ''}
          onChange={onInputChange}
          placeholder="Network CIDR (e.g., 10.244.0.0/16)"
          className="w-full px-3 py-2 border border-gray-300 rounded-md"
        />
      </div>
    </div>
  </div>
);

export default KubernetesSetup;