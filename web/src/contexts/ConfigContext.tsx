import React, { createContext, useContext, useState, useEffect } from 'react';

export interface ClusterConfig {
  clusterName: string;
  namespace: string;
  storageClass: string;
  domain: string;
  useHttps: boolean;
  adminUsername: string;
  adminPassword: string;
  adminEmail: string;
  adminConsolePort?: number;
  localArtifactMirrorPort?: number;
  databaseType: 'internal' | 'external';
  dataDirectory: string;
  useProxy: boolean;
  httpProxy?: string;
  httpsProxy?: string;
  noProxy?: string;
  networkInterface?: string;
  globalCidr?: string;
  databaseConfig?: {
    host: string;
    port: number;
    username: string;
    password: string;
    database: string;
  };
}

interface PrototypeSettings {
  skipValidation: boolean;
  failPreflights: boolean;
  failInstallation: boolean;
  failHostPreflights: boolean;
  clusterMode: 'existing' | 'embedded';
  themeColor: string;
  skipNodeValidation: boolean;
  useSelfSignedCert: boolean;
  enableMultiNode: boolean;
  availableHostNetworkInterfaces: Array<{
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
  clusterName: '',
  namespace: 'gitea',
  storageClass: 'standard',
  domain: '',
  useHttps: true,
  adminUsername: 'giteaadmin',
  adminPassword: '',
  adminEmail: '',
  databaseType: 'internal',
  dataDirectory: '/var/lib/gitea',
  useProxy: false,
};

const defaultPrototypeSettings: PrototypeSettings = {
  skipValidation: false,
  failPreflights: false,
  failInstallation: false,
  failHostPreflights: false,
  clusterMode: 'embedded',
  themeColor: '#316DE6',
  skipNodeValidation: false,
  useSelfSignedCert: false,
  enableMultiNode: true,
  availableHostNetworkInterfaces: [
    { name: 'eth0' },
    { name: 'eth1' },
    { name: 'wlan0' },
    { name: 'docker0' }
  ]
};

const PROTOTYPE_SETTINGS_KEY = 'gitea-prototype-settings';

const ConfigContext = createContext<ConfigContextType | undefined>(undefined);

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