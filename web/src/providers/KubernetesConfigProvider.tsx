import React, { useState } from 'react';
import { KubernetesConfigContext } from '../contexts/KubernetesConfigContext';
import { KubernetesConfig } from '../types';

const defaultKubernetesConfig: KubernetesConfig = {
  installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000'
};

export const KubernetesConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<KubernetesConfig>(defaultKubernetesConfig);

  const updateConfig = (newConfig: Partial<KubernetesConfig>) => {
    setConfig((prev) => ({ ...prev, ...newConfig }));
  };

  const resetConfig = () => {
    setConfig(defaultKubernetesConfig);
  };

  return (
    <KubernetesConfigContext.Provider value={{ config, updateConfig, resetConfig }}>
      {children}
    </KubernetesConfigContext.Provider>
  );
};
