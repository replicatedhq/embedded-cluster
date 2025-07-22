import { createContext, useContext } from 'react';

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

export const KubernetesConfigContext = createContext<KubernetesConfigContextType | undefined>(undefined);

export const useKubernetesConfig = (): KubernetesConfigContextType => {
  const context = useContext(KubernetesConfigContext);
  if (context === undefined) {
    throw new Error('useKubernetesConfig must be used within a KubernetesConfigProvider');
  }
  return context;
};
