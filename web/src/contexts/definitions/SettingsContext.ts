import { createContext } from 'react';

export interface Settings {
  themeColor: string;
}

export interface SettingsContextType {
  settings: Settings;
  updateSettings: (newSettings: Partial<Settings>) => void;
}

export const defaultSettings: Settings = {
  themeColor: '#316DE6',
};

export const SETTINGS_KEY = 'app-settings';

export const SettingsContext = createContext<SettingsContextType | undefined>(undefined);
