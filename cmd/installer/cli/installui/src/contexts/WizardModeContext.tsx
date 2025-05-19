import React, { createContext, useContext } from 'react';
import { useConfig } from './ConfigContext';

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

const getTextVariations = (isEmbedded: boolean): Record<WizardMode, WizardText> => ({
  install: {
    title: 'Gitea Enterprise',
    subtitle: 'Installation Wizard',
    welcomeTitle: 'Welcome to Gitea Enterprise',
    welcomeDescription: `This wizard will guide you through installing Gitea Enterprise on your ${isEmbedded ? 'Linux machine' : 'Kubernetes cluster'}.`,
    setupTitle: 'Setup',
    setupDescription: 'Set up the hosts to use for this installation.',
    configurationTitle: 'Configuration',
    configurationDescription: 'Configure your Gitea Enterprise installation by providing the information below.',
    installationTitle: 'Installing Gitea Enterprise',
    installationDescription: '',
    completionTitle: 'Installation Complete!',
    completionDescription: 'Gitea Enterprise has been installed successfully.',
    welcomeButtonText: 'Start',
    nextButtonText: 'Next: Start Installation',
  },
  upgrade: {
    title: 'Gitea Enterprise',
    subtitle: 'Upgrade Wizard',
    welcomeTitle: 'Welcome to Gitea Enterprise',
    welcomeDescription: `This wizard will guide you through upgrading Gitea Enterprise on your ${isEmbedded ? 'Linux machine' : 'Kubernetes cluster'}.`,
    setupTitle: 'Setup',
    setupDescription: 'Set up the hosts to use for this installation.',
    configurationTitle: 'Upgrade Configuration',
    configurationDescription: 'Configure your Gitea Enterprise installation by providing the information below.',
    installationTitle: 'Upgrading Gitea Enterprise',
    installationDescription: '',
    completionTitle: 'Upgrade Complete!',
    completionDescription: 'Gitea Enterprise has been successfully upgraded.',
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
  const isEmbedded = prototypeSettings.clusterMode === 'embedded';
  const text = getTextVariations(isEmbedded)[mode];

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