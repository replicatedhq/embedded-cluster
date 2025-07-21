import React, { useState } from 'react';
import { KubernetesConfigContext, defaultKubernetesConfig, KubernetesConfig, KubernetesConfigContextType } from '../definitions/KubernetesConfigContext';

export const KubernetesConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<KubernetesConfig>(defaultKubernetesConfig);

  const updateConfig = (newConfig: Partial<KubernetesConfig>) => {
    setConfig(prev => ({ ...prev, ...newConfig }));
  };

  const resetConfig = () => {
    setConfig(defaultKubernetesConfig);
  };

  const value: KubernetesConfigContextType = {
    config,
    updateConfig,
    resetConfig,
  };

  return (
    <KubernetesConfigContext.Provider value={value}>
      {children}
    </KubernetesConfigContext.Provider>
  );
};
