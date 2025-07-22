import { createContext, useContext } from 'react';

export interface LinuxConfig {
  adminConsolePort?: number;
  localArtifactMirrorPort?: number;
  dataDirectory: string;
  useProxy: boolean;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  networkInterface?: string;
  globalCidr?: string;
}

interface LinuxConfigContextType {
  config: LinuxConfig;
  updateConfig: (newConfig: Partial<LinuxConfig>) => void;
  resetConfig: () => void;
}

export const LinuxConfigContext = createContext<LinuxConfigContextType | undefined>(undefined);

export const useLinuxConfig = (): LinuxConfigContextType => {
  const context = useContext(LinuxConfigContext);
  if (context === undefined) {
    throw new Error('useLinuxConfig must be used within a LinuxConfigProvider');
  }
  return context;
};
