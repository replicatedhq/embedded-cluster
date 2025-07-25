import React from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { render, RenderOptions } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, RouteObject } from "react-router-dom";
import { vi } from "vitest";
import { JSX } from "react/jsx-runtime";

import { InitialState } from "../types";
import { InstallationTarget } from "../types/installation-target";
import { createQueryClient } from "../query-client";
import { LinuxConfigContext, LinuxConfig } from "../contexts/LinuxConfigContext";
import { KubernetesConfigContext, KubernetesConfig } from "../contexts/KubernetesConfigContext";
import { SettingsContext, Settings } from "../contexts/SettingsContext";
import { WizardContext, WizardMode } from "../contexts/WizardModeContext";
import { InitialStateContext } from "../contexts/InitialStateContext";
import { AuthContext } from "../contexts/AuthContext";

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

// eslint-disable-next-line react-refresh/only-export-components -- this is a test component
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
    initialStateContext: { title: "My App", installTarget: options.wrapperProps?.target || "linux" },
    linuxConfigContext: {
      config: {
        adminConsolePort: 8800,
        dataDirectory: "/var/lib/embedded-cluster",
        useProxy: false,
      },
      updateConfig: vi.fn(),
      resetConfig: vi.fn(),
    },
    kubernetesConfigContext: {
      config: {
        adminConsolePort: 8080,
        useProxy: false,
        installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000',
      },
      updateConfig: vi.fn(),
      resetConfig: vi.fn(),
    },
    settingsContext: {
      settings: {
        themeColor: "#316DE6",
      },
      updateSettings: vi.fn(),
    },
    wizardModeContext: {
      target: options.wrapperProps?.target || "linux",
      mode: "install",
      text: {
        title: "My App",
        subtitle: "Installation Wizard",
        welcomeTitle: "Welcome to My App",
        welcomeDescription: `This wizard will guide you through installing My App on your ${options.wrapperProps?.target === "kubernetes" ? "Kubernetes cluster" : "Linux machine"}.`,
        configurationTitle: "Configuration",
        configurationDescription: "Configure your My App installation by providing the information below.",
        linuxSetupTitle: "Setup",
        linuxSetupDescription: "Set up the hosts to use for this installation.",
        kubernetesSetupTitle: "Kubernetes Setup",
        kubernetesSetupDescription: "Set up the Kubernetes cluster for this installation.",
        validationTitle: "Validation",
        validationDescription: "Validate the host requirements before proceeding with installation.",
        installationTitle: "Installing My App",
        installationDescription: "",
        welcomeButtonText: "Start",
        nextButtonText: "Next: Start Installation",
      },
    },
    authContext: {
      token: options.wrapperProps?.authToken || (options.wrapperProps?.authenticated ? "test-token" : null),
      setToken: vi.fn(),
      isAuthenticated: !!options.wrapperProps?.authenticated || !!options.wrapperProps?.authToken,
    },
  };

  const mergedContextValues: MockProviderProps["contexts"] = {
    initialStateContext: { ...defaultContextValues.initialStateContext, ...options.wrapperProps?.contextValues?.initialStateContext },
    linuxConfigContext: { ...defaultContextValues.linuxConfigContext, ...options.wrapperProps?.contextValues?.linuxConfigContext },
    kubernetesConfigContext: { ...defaultContextValues.kubernetesConfigContext, ...options.wrapperProps?.contextValues?.kubernetesConfigContext },
    settingsContext: { ...defaultContextValues.settingsContext, ...options.wrapperProps?.contextValues?.settingsContext },
    wizardModeContext: { ...defaultContextValues.wizardModeContext, ...options.wrapperProps?.contextValues?.wizardModeContext },
    authContext: { ...defaultContextValues.authContext, ...options.wrapperProps?.contextValues?.authContext },
  };
  const { wrapperProps = {}, wrapper: CustomWrapper } = options;
  const { routePath, initialEntries = ["/"] } = wrapperProps;

  const queryClient = createQueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: { retry: false },
    },
  });
  vi.mock("../query-client", async (importOriginal) => {
    const original = (await importOriginal()) as Record<string, unknown>;
    return {
      ...original,
      getQueryClient: () => queryClient,
    };
  });

  // Set up localStorage with the auth token if provided
  if (wrapperProps.authToken) {
    mockLocalStorage.getItem.mockReturnValue(wrapperProps.authToken);
  } else if (wrapperProps.authenticated) {
    mockLocalStorage.getItem.mockReturnValue("test-token");
  } else {
    mockLocalStorage.getItem.mockReturnValue(null);
  }

  // Create router with the test component wrapped in MockProvider
  const routes: RouteObject[] = [
    {
      path: routePath || "/*",
      element: (
        <MockProvider queryClient={queryClient} contexts={mergedContextValues}>
          {CustomWrapper ? <CustomWrapper>{ui}</CustomWrapper> : ui}
        </MockProvider>
      ),
    },
  ];
  const router = createMemoryRouter(routes, {
    initialEntries,
  });

  const view = render(<RouterProvider router={router} />, options);

  return { ...view, router, queryClient };
};
