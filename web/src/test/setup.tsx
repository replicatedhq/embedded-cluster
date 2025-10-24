import React from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { render, RenderOptions } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, RouteObject } from "react-router-dom";
import { vi } from "vitest";
import { JSX } from "react/jsx-runtime";

import { InitialState, WizardText } from "../types";
import { InstallationTarget } from "../types/installation-target";
import { WizardMode } from "../types/wizard-mode";
import { createQueryClient } from "../query-client";
import { SettingsContext, Settings } from "../contexts/SettingsContext";
import { WizardContext } from "../contexts/WizardModeContext";
import { InitialStateContext } from "../contexts/InitialStateContext";
import { AuthContext } from "../contexts/AuthContext";
import { InstallationProgressProvider } from "../providers/InstallationProgressProvider";

// Mock localStorage for tests
const mockLocalStorage = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, "localStorage", { value: mockLocalStorage });

// Mock sessionStorage for tests with real storage functionality
const createMockStorage = () => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => {
      store[key] = value;
    }),
    removeItem: vi.fn((key: string) => {
      delete store[key];
    }),
    clear: vi.fn(() => {
      store = {};
    }),
  };
};
const mockSessionStorage = createMockStorage();
Object.defineProperty(window, "sessionStorage", { value: mockSessionStorage });

// Mock scrollIntoView for all tests (JSDOM does not implement it)
if (!window.HTMLElement.prototype.scrollIntoView) {
  window.HTMLElement.prototype.scrollIntoView = vi.fn();
}

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mock ResizeObserver
global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));

// Mock IntersectionObserver
global.IntersectionObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}));

interface MockProviderProps {
  children: React.ReactNode;
  queryClient: ReturnType<typeof createQueryClient>;
  contexts: {
    initialStateContext: InitialState
    settingsContext: {
      settings: Settings;
      updateSettings: (newSettings: Partial<Settings>) => void;
    };
    wizardModeContext: {
      target: InstallationTarget;
      mode: WizardMode;
      isAirgap: boolean;
      requiresInfraUpgrade: boolean;
      text: WizardText;
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
          <InstallationProgressProvider>
            <SettingsContext.Provider value={contexts.settingsContext}>
              <WizardContext.Provider value={contexts.wizardModeContext}>
                {children}
              </WizardContext.Provider>
            </SettingsContext.Provider>
          </InstallationProgressProvider>
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
    mode?: WizardMode;
  };
  wrapper?: React.ComponentType<{ children: React.ReactNode }>;
}

export const renderWithProviders = (
  ui: JSX.Element,
  options: RenderWithProvidersOptions = {},
) => {
  const defaultContextValues: MockProviderProps["contexts"] = {
    initialStateContext: {
      title: "My App",
      installTarget: options.wrapperProps?.target || "linux",
      mode: options.wrapperProps?.mode || "install",
      isAirgap: false,
      requiresInfraUpgrade: false
    },
    settingsContext: {
      settings: {
        themeColor: "#316DE6",
      },
      updateSettings: vi.fn(),
    },
    wizardModeContext: {
      target: options.wrapperProps?.target || "linux",
      mode: options.wrapperProps?.mode || "install",
      isAirgap: false,
      requiresInfraUpgrade: false,
      text: {
        title: "My App",
        subtitle: "Installation Wizard",
        welcomeTitle: "Welcome to My App",
        welcomeDescription: `This wizard will guide you through installing My App on your ${options.wrapperProps?.target === "kubernetes" ? "Kubernetes cluster" : "Linux machine"}.`,
        timelineTitle: `${options.wrapperProps?.mode === "install" ? "Installation" : "Upgrade"} Progress`,
        configurationTitle: "Configuration",
        configurationDescription: "Configure your My App installation by providing the information below.",
        linuxSetupTitle: "Setup",
        linuxSetupDescription: "Set up the hosts to use for this installation.",
        kubernetesSetupTitle: "Kubernetes Setup",
        kubernetesSetupDescription: "Set up the Kubernetes cluster for this installation.",
        kubernetesInstallationTitle: "Infrastructure Installation",
        kubernetesInstallationDescription: "Installing infrastructure components",
        linuxValidationTitle: "Validation",
        linuxValidationDescription: "Validate the host requirements before proceeding with installation.",
        linuxInstallationHeader: "Installation",
        linuxInstallationTitle: "Infrastructure Installation",
        linuxInstallationDescription: "Installing infrastructure components",
        appValidationTitle: "App Validation",
        appValidationDescription: "Validate the app requirements before proceeding with installation.",
        appInstallationTitle: "Installing My App",
        appInstallationDescription: "",
        appInstallationLoadingTitle: "Installing application...",
        appInstallationFailureTitle: "Application installation failed",
        appInstallationSuccessTitle: "Application installed successfully!",
        welcomeButtonText: "Start",
        nextButtonText: "Next: Start Installation",
        completion: "Installation complete"
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
