import React from "react";
import { render, RenderOptions } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, RouteObject } from "react-router-dom";
import { JSX } from "react/jsx-runtime";

import { InitialState } from "../types";
import { InstallationTarget } from "../types/installation-target";
import { createQueryClient } from "../query-client";
import { LinuxConfig } from "../contexts/definitions/LinuxConfigContext";
import { KubernetesConfig } from "../contexts/definitions/KubernetesConfigContext";
import { Settings } from "../contexts/definitions/SettingsContext";
import { WizardMode } from "../contexts/definitions/WizardModeContext";
import { MockProvider } from "./setup";

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

interface RenderWithProvidersOptions extends RenderOptions {
  wrapperProps?: {
    initialEntries?: string[];
    preloadedState?: Record<string, unknown>;
    contextValues?: Partial<MockProviderProps["contexts"]>;
    routePath?: string;
    authenticated?: boolean;
    authToken?: string;
    target?: InstallationTarget;
  };
  wrapper?: React.ComponentType<{ children: React.ReactNode }>;
}

export const renderWithProviders = (
  ui: JSX.Element,
  options: RenderWithProvidersOptions = {},
) => {
  const defaultContextValues: MockProviderProps["contexts"] = {
    initialStateContext: { title: "Test App", installTarget: options.wrapperProps?.target || "linux" },
    linuxConfigContext: {
      config: { dataDirectory: "/tmp", useProxy: false },
      updateConfig: () => {},
      resetConfig: () => {},
    },
    kubernetesConfigContext: {
      config: { useProxy: false },
      updateConfig: () => {},
      resetConfig: () => {},
    },
    settingsContext: {
      settings: { themeColor: "#316DE6" },
      updateSettings: () => {},
    },
    wizardModeContext: {
      target: options.wrapperProps?.target || "linux",
      mode: "install",
      text: {
        title: "Test App",
        subtitle: "Installation Wizard",
        welcomeTitle: "Welcome to Test App",
        welcomeDescription: "Test description",
        configurationTitle: "Configuration",
        configurationDescription: "Configure settings",
        linuxSetupTitle: "System Configuration",
        linuxSetupDescription: "Configure Linux",
        kubernetesSetupTitle: "Kubernetes Configuration",
        kubernetesSetupDescription: "Configure Kubernetes",
        validationTitle: "Pre-Installation Checks",
        validationDescription: "Validating",
        installationTitle: "Installation",
        installationDescription: "Installing",
        welcomeButtonText: "Get Started",
        nextButtonText: "Next",
      },
    },
    authContext: {
      token: options.wrapperProps?.authToken || null,
      setToken: () => {},
      isAuthenticated: options.wrapperProps?.authenticated || false,
    },
  };

  // If using react-router, setup memory router
  const routes: RouteObject[] = [
    {
      path: options.wrapperProps?.routePath || "/",
      element: ui,
    },
  ];

  const router = createMemoryRouter(routes, {
    initialEntries: options.wrapperProps?.initialEntries || ["/"],
  });

  const { wrapperProps, ...renderOptions } = options;

  const AllTheProviders = ({ children }: { children: React.ReactNode }) => {
    return (
      <MockProvider
        queryClient={createQueryClient()}
        contexts={{
          ...defaultContextValues,
          ...wrapperProps?.contextValues,
        }}
      >
        {children}
      </MockProvider>
    );
  };

  if (options.wrapperProps?.routePath) {
    return render(<RouterProvider router={router} />, {
      wrapper: AllTheProviders,
      ...renderOptions,
    });
  }

  return render(ui, { wrapper: AllTheProviders, ...renderOptions });
};

export default renderWithProviders;
