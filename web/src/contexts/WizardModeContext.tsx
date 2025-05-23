import React, { createContext, useContext } from 'react';
import { useConfig } from './ConfigContext';
import { useBranding } from './BrandingContext';

export type WizardMode = 'install' | 'upgrade';

interface WizardText {
  title: string;
  subtitle: string;
  welcomeTitle: string;
  welcomeDescription: string;
  setupTitle: string;
  setupDescription: string;
  configurationTitle: string;
  configurationDescription: string;
  installationTitle: string;
  installationDescription: string;
  completionTitle: string;
  completionDescription: string;
  welcomeButtonText: string;
  nextButtonText: string;
}

const getTextVariations = (isEmbedded: boolean, appTitle: string): Record<WizardMode, WizardText> => ({
  install: {
    title: appTitle || '',
    subtitle: 'Installation Wizard',
    welcomeTitle: `Welcome to ${appTitle}`,
    welcomeDescription: `This wizard will guide you through installing ${appTitle} on your ${isEmbedded ? 'Linux machine' : 'Kubernetes cluster'}.`,
    setupTitle: 'Setup',
    setupDescription: 'Set up the hosts to use for this installation.',
    configurationTitle: 'Configuration',
    configurationDescription: `Configure your ${appTitle} installation by providing the information below.`,
    installationTitle: `Installing ${appTitle}`,
    installationDescription: '',
    completionTitle: 'Installation Complete!',
    completionDescription: `${appTitle} has been installed successfully.`,
    welcomeButtonText: 'Start',
    nextButtonText: 'Next: Start Installation',
  },
  upgrade: {
    title: appTitle || '',
    subtitle: 'Upgrade Wizard',
    welcomeTitle: `Welcome to ${appTitle}`,
    welcomeDescription: `This wizard will guide you through upgrading ${appTitle} on your ${isEmbedded ? 'Linux machine' : 'Kubernetes cluster'}.`,
    setupTitle: 'Setup',
    setupDescription: 'Set up the hosts to use for this installation.',
    configurationTitle: 'Upgrade Configuration',
    configurationDescription: `Configure your ${appTitle} installation by providing the information below.`,
    installationTitle: `Upgrading ${appTitle}`,
    installationDescription: '',
    completionTitle: 'Upgrade Complete!',
    completionDescription: `${appTitle} has been successfully upgraded.`,
    welcomeButtonText: 'Start Upgrade',
    nextButtonText: 'Next: Start Upgrade',
  },
});

interface WizardModeContextType {
  mode: WizardMode;
  text: WizardText;
}

const WizardModeContext = createContext<WizardModeContextType | undefined>(undefined);

export const WizardModeProvider: React.FC<{
  children: React.ReactNode;
  mode: WizardMode;
}> = ({ children, mode }) => {
  const { prototypeSettings } = useConfig();
  const { branding } = useBranding();
  const isEmbedded = prototypeSettings.clusterMode === 'embedded';
  const text = getTextVariations(isEmbedded, branding?.appTitle || '')[mode];

  return (
    <WizardModeContext.Provider value={{ mode, text }}>
      {children}
    </WizardModeContext.Provider>
  );
};

export const useWizardMode = (): WizardModeContextType => {
  const context = useContext(WizardModeContext);
  if (context === undefined) {
    throw new Error('useWizardMode must be used within a WizardModeProvider');
  }
  return context;
};