import { createContext } from 'react';

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

export interface LinuxConfigContextType {
  config: LinuxConfig;
  updateConfig: (newConfig: Partial<LinuxConfig>) => void;
  resetConfig: () => void;
}

export const defaultLinuxConfig: LinuxConfig = {
  dataDirectory: '/var/lib/embedded-cluster',
  useProxy: false,
};

export const LinuxConfigContext = createContext<LinuxConfigContextType | undefined>(undefined);