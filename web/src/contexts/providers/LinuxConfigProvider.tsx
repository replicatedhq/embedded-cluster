import React, { useState } from 'react';
import { LinuxConfigContext, defaultLinuxConfig, LinuxConfig, LinuxConfigContextType } from '../definitions/LinuxConfigContext';

export const LinuxConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<LinuxConfig>(defaultLinuxConfig);

  const updateConfig = (newConfig: Partial<LinuxConfig>) => {
    setConfig(prev => ({ ...prev, ...newConfig }));
  };

  const resetConfig = () => {
    setConfig(defaultLinuxConfig);
  };

  const value: LinuxConfigContextType = {
    config,
    updateConfig,
    resetConfig,
  };

  return (
    <LinuxConfigContext.Provider value={value}>
      {children}
    </LinuxConfigContext.Provider>
  );
};
