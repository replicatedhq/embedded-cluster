import React, { createContext, useContext, useState, useEffect } from 'react';

export interface ClusterConfig {
  storageClass: string;
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

interface PrototypeSettings {
  skipValidation: boolean;
  failPreflights: boolean;
  failInstallation: boolean;
  failHostPreflights: boolean;
  installTarget: 'linux' | 'kubernetes';
  themeColor: string;
  skipNodeValidation: boolean;
  useSelfSignedCert: boolean;
  enableMultiNode: boolean;
  availableNetworkInterfaces: Array<{
    name: string;
  }>;
}

interface ConfigContextType {
  config: ClusterConfig;
  prototypeSettings: PrototypeSettings;
  updateConfig: (newConfig: Partial<ClusterConfig>) => void;
  updatePrototypeSettings: (newSettings: Partial<PrototypeSettings>) => void;
  resetConfig: () => void;
}

const defaultConfig: ClusterConfig = {
  storageClass: 'standard',
  dataDirectory: '/var/lib/embedded-cluster',
  useProxy: false,
};

const defaultPrototypeSettings: PrototypeSettings = {
  skipValidation: false,
  failPreflights: false,
  failInstallation: false,
  failHostPreflights: false,
  installTarget: 'linux',
  themeColor: '#316DE6',
  skipNodeValidation: false,
  useSelfSignedCert: false,
  enableMultiNode: true,
  availableNetworkInterfaces: []
};

const PROTOTYPE_SETTINGS_KEY = 'app-prototype-settings';

export const ConfigContext = createContext<ConfigContextType | undefined>(undefined);

export const ConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [config, setConfig] = useState<ClusterConfig>(defaultConfig);
  const [prototypeSettings, setPrototypeSettings] = useState<PrototypeSettings>(() => {
    const saved = localStorage.getItem(PROTOTYPE_SETTINGS_KEY);
    const settings = saved ? JSON.parse(saved) : defaultPrototypeSettings;
    if (!settings.themeColor) {
      settings.themeColor = defaultPrototypeSettings.themeColor;
    }
    return settings;
  });

  useEffect(() => {
    localStorage.setItem(PROTOTYPE_SETTINGS_KEY, JSON.stringify(prototypeSettings));
  }, [prototypeSettings]);

  const updateConfig = (newConfig: Partial<ClusterConfig>) => {
    setConfig((prev) => ({ ...prev, ...newConfig }));
  };

  const updatePrototypeSettings = (newSettings: Partial<PrototypeSettings>) => {
    setPrototypeSettings((prev) => {
      const updated = { ...prev, ...newSettings };
      if (!updated.themeColor) {
        updated.themeColor = defaultPrototypeSettings.themeColor;
      }
      return updated;
    });
  };

  const resetConfig = () => {
    setConfig(defaultConfig);
  };

  return (
    <ConfigContext.Provider value={{ config, prototypeSettings, updateConfig, updatePrototypeSettings, resetConfig }}>
      {children}
    </ConfigContext.Provider>
  );
};

export const useConfig = (): ConfigContextType => {
  const context = useContext(ConfigContext);
  if (context === undefined) {
    throw new Error('useConfig must be used within a ConfigProvider');
  }
  return context;
};