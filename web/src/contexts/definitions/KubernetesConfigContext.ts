import { createContext } from 'react';

export interface KubernetesConfig {
  adminConsolePort?: number;
  useProxy: boolean;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  installCommand?: string;
}

export interface KubernetesConfigContextType {
  config: KubernetesConfig;
  updateConfig: (newConfig: Partial<KubernetesConfig>) => void;
  resetConfig: () => void;
}

export const defaultKubernetesConfig: KubernetesConfig = {
  useProxy: false,
  installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000'
};

export const KubernetesConfigContext = createContext<KubernetesConfigContextType | undefined>(undefined);