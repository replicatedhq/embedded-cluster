import React, { useState } from 'react';
import { LinuxConfigContext } from '../contexts/LinuxConfigContext';
import { LinuxConfig } from '../types';

const defaultLinuxConfig: LinuxConfig = {
  dataDirectory: '/var/lib/embedded-cluster',
};

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
