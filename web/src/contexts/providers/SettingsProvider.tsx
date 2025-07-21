import React, { useState, useEffect } from 'react';
import { SettingsContext, defaultSettings, SETTINGS_KEY, Settings, SettingsContextType } from '../definitions/SettingsContext';

export const SettingsProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [settings, setSettings] = useState<Settings>(() => {
    const saved = localStorage.getItem(SETTINGS_KEY);
    const config = saved ? JSON.parse(saved) : defaultSettings;
    if (!config.themeColor) {
      config.themeColor = defaultSettings.themeColor;
    }
    return config;
  });

  // Save settings to localStorage whenever they change
  useEffect(() => {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
  }, [settings]);

  const updateSettings = (newSettings: Partial<Settings>) => {
    setSettings(prev => {
      const updated = { ...prev, ...newSettings };
      return updated;
    });
  };

  const value: SettingsContextType = {
    settings,
    updateSettings,
  };

  return (
    <SettingsContext.Provider value={value}>
      {children}
    </SettingsContext.Provider>
  );
};
