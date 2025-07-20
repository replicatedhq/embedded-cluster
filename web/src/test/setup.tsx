import React from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { vi } from "vitest";

import { InitialState } from "../types";
import { InstallationTarget } from "../types/installation-target";
import { createQueryClient } from "../query-client";
import { LinuxConfigContext } from "../contexts/definitions/LinuxConfigContext";
import { LinuxConfig } from "../contexts/definitions/LinuxConfigContext";
import { KubernetesConfigContext } from "../contexts/definitions/KubernetesConfigContext";
import { KubernetesConfig } from "../contexts/definitions/KubernetesConfigContext";
import { SettingsContext } from "../contexts/definitions/SettingsContext";
import { Settings } from "../contexts/definitions/SettingsContext";
import { WizardContext } from "../contexts/definitions/WizardModeContext";
import { WizardMode } from "../contexts/definitions/WizardModeContext";
import { InitialStateContext } from "../contexts/definitions/InitialStateContext";
import { AuthContext } from "../contexts/definitions/AuthContext";

// Mock localStorage for tests
const mockLocalStorage = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, "localStorage", { value: mockLocalStorage });

// Mock scrollIntoView for all tests (JSDOM does not implement it)
if (!window.HTMLElement.prototype.scrollIntoView) {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
}

interface MockProviderProps {
  children: React.ReactNode;
  queryClient: ReturnType<typeof createQueryClient>;
  contexts: {
    initialStateContext: InitialState
    linuxConfigContext: {
      config: LinuxConfig;
      updateConfig: (newConfig: Partial<LinuxConfig>) => void;
      resetConfig: () => void;
    };
    kubernetesConfigContext: {
      config: KubernetesConfig;
      updateConfig: (newConfig: Partial<KubernetesConfig>) => void;
      resetConfig: () => void;
    };
    settingsContext: {
      settings: Settings;
      updateSettings: (newSettings: Partial<Settings>) => void;
    };
    wizardModeContext: {
      target: InstallationTarget;
      mode: WizardMode;
      text: {
        title: string;
        subtitle: string;
        welcomeTitle: string;
        welcomeDescription: string;
        configurationTitle: string;
        configurationDescription: string;
        linuxSetupTitle: string;
        linuxSetupDescription: string;
        kubernetesSetupTitle: string;
        kubernetesSetupDescription: string;
        validationTitle: string;
        validationDescription: string;
        installationTitle: string;
        installationDescription: string;
        welcomeButtonText: string;
        nextButtonText: string;
      };
    };
    authContext: {
      token: string | null;
      setToken: (token: string | null) => void;
      isAuthenticated: boolean;
    };
  };
}

const MockProvider = ({ children, queryClient, contexts }: MockProviderProps) => {
  // Set up localStorage with the auth token if provided
  React.useEffect(() => {
    if (contexts.authContext.token) {
      mockLocalStorage.getItem.mockReturnValue(contexts.authContext.token);
    } else {
      mockLocalStorage.getItem.mockReturnValue(null);
    }
  }, [contexts.authContext.token]);

  return (
    <InitialStateContext.Provider value={contexts.initialStateContext}>
      <QueryClientProvider client={queryClient}>
        <AuthContext.Provider value={{ ...contexts.authContext, isLoading: false }}>
          <LinuxConfigContext.Provider value={contexts.linuxConfigContext}>
            <KubernetesConfigContext.Provider value={contexts.kubernetesConfigContext}>
              <SettingsContext.Provider value={contexts.settingsContext}>
                <WizardContext.Provider value={contexts.wizardModeContext}>{children}</WizardContext.Provider>
              </SettingsContext.Provider>
            </KubernetesConfigContext.Provider>
          </LinuxConfigContext.Provider>
        </AuthContext.Provider>
      </QueryClientProvider>
    </InitialStateContext.Provider>
  );
};


export default MockProvider;
