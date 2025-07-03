import React, { createContext, useContext, useState } from 'react';

export interface KubernetesConfig {
  adminConsolePort?: number;
  useProxy: boolean;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  installCommand?: string;
}

interface KubernetesConfigContextType {
  config: KubernetesConfig;
  updateConfig: (newConfig: Partial<KubernetesConfig>) => void;
  resetConfig: () => void;
}

const defaultKubernetesConfig: KubernetesConfig = {
  useProxy: false,
  installCommand: 'todo: install command here'
};

export const KubernetesConfigContext = createContext<KubernetesConfigContextType | undefined>(undefined);

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

export const useKubernetesConfig = (): KubernetesConfigContextType => {
  const context = useContext(KubernetesConfigContext);
  if (context === undefined) {
    throw new Error('useKubernetesConfig must be used within a KubernetesConfigProvider');
  }
  return context;
};
