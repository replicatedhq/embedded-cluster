import React, { useState, useEffect } from 'react';
import { SettingsContext } from '../contexts/SettingsContext';

export interface Settings {
  themeColor: string;
}

const defaultSettings: Settings = {
  themeColor: '#316DE6',
};

const SETTINGS_KEY = 'app-settings';

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettings] = useState<Settings>(() => {
    const saved = localStorage.getItem(SETTINGS_KEY);
    const config = saved ? JSON.parse(saved) : defaultSettings;
    if (!config.themeColor) {
      config.themeColor = defaultSettings.themeColor;
    }
    return config;
  });

  useEffect(() => {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
  }, [settings]);

  const updateSettings = (newSettings: Partial<Settings>) => {
    setSettings((prev) => {
      const updated = { ...prev, ...newSettings };
      if (!updated.themeColor) {
        updated.themeColor = defaultSettings.themeColor;
      }
      return updated;
    });
  };

  return (
    <SettingsContext.Provider value={{ settings, updateSettings }}>
      {children}
    </SettingsContext.Provider>
  );
};
