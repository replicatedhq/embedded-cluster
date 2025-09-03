import { createContext, useContext } from 'react';
import { KubernetesConfig } from '../types';

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
