import React, { createContext, useContext, useState } from 'react';

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

const defaultLinuxConfig: LinuxConfig = {
  dataDirectory: '/var/lib/embedded-cluster',
  useProxy: false,
};

export const LinuxConfigContext = createContext<LinuxConfigContextType | undefined>(undefined);

export const LinuxConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<LinuxConfig>(defaultLinuxConfig);

  const updateConfig = (newConfig: Partial<LinuxConfig>) => {
    setConfig((prev) => ({ ...prev, ...newConfig }));
  };

  const resetConfig = () => {
    setConfig(defaultLinuxConfig);
  };

  return (
    <LinuxConfigContext.Provider value={{ config, updateConfig, resetConfig }}>
      {children}
    </LinuxConfigContext.Provider>
  );
};

export const useLinuxConfig = (): LinuxConfigContextType => {
  const context = useContext(LinuxConfigContext);
  if (context === undefined) {
    throw new Error('useLinuxConfig must be used within a LinuxConfigProvider');
  }
  return context;
};