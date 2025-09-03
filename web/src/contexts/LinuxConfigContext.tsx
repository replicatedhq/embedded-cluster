import { createContext, useContext } from 'react';
import { LinuxConfig } from '../types';


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
