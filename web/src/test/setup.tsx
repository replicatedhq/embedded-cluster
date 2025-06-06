import React from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { render, RenderOptions } from "@testing-library/react";
import { createMemoryRouter, RouterProvider, RouteObject } from "react-router-dom";
import { vi } from "vitest";

import { createQueryClient } from "../query-client";
import { ConfigContext, ClusterConfig } from "../contexts/ConfigContext";
import { WizardModeContext, WizardMode } from "../contexts/WizardModeContext";
import { BrandingContext } from "../contexts/BrandingContext";

interface PrototypeSettings {
  skipValidation: boolean;
  failPreflights: boolean;
  failInstallation: boolean;
  failHostPreflights: boolean;
  clusterMode: "existing" | "embedded";
  themeColor: string;
  skipNodeValidation: boolean;
  useSelfSignedCert: boolean;
  enableMultiNode: boolean;
  availableNetworkInterfaces: Array<{
    name: string;
  }>;
}

interface MockProviderProps {
  children: React.ReactNode;
  queryClient: ReturnType<typeof createQueryClient>;
  contexts: {
    brandingContext: {
      title: string;
      icon?: string;
    };
    configContext: {
      config: ClusterConfig;
      prototypeSettings: PrototypeSettings;
      updateConfig: (newConfig: Partial<ClusterConfig>) => void;
      updatePrototypeSettings: (newSettings: Partial<PrototypeSettings>) => void;
      resetConfig: () => void;
    };
    wizardModeContext: {
      mode: WizardMode;
      text: {
        title: string;
        subtitle: string;
        welcomeTitle: string;
        welcomeDescription: string;
        setupTitle: string;
        setupDescription: string;
        validationTitle: string;
        validationDescription: string;
        installationTitle: string;
        installationDescription: string;
        welcomeButtonText: string;
        nextButtonText: string;
      };
    };
  };
}

const MockProvider = ({ children, queryClient, contexts }: MockProviderProps) => {
  return (
    <QueryClientProvider client={queryClient}>
      <ConfigContext.Provider value={contexts.configContext}>
        <BrandingContext.Provider value={contexts.brandingContext}>
          <WizardModeContext.Provider value={contexts.wizardModeContext}>{children}</WizardModeContext.Provider>
        </BrandingContext.Provider>
      </ConfigContext.Provider>
    </QueryClientProvider>
  );
};

interface RenderWithProvidersOptions extends RenderOptions {
  wrapperProps?: {
    initialEntries?: string[];
    preloadedState?: Record<string, unknown>;
    routePath?: string;
    authenticated?: boolean;
  };
}

export const renderWithProviders = (
  ui: JSX.Element,
  options: RenderWithProvidersOptions = {},
  contextValues: MockProviderProps["contexts"] = {
    brandingContext: { title: "My App" },
    configContext: {
      config: {
        clusterName: "",
        namespace: "",
        storageClass: "standard",
        domain: "",
        useHttps: true,
        adminUsername: "admin",
        adminPassword: "",
        adminEmail: "",
        databaseType: "internal",
        dataDirectory: "/var/lib/embedded-cluster",
        useProxy: false,
      },
      prototypeSettings: {
        skipValidation: false,
        failPreflights: false,
        failInstallation: false,
        failHostPreflights: false,
        clusterMode: "embedded",
        themeColor: "#316DE6",
        skipNodeValidation: false,
        useSelfSignedCert: false,
        enableMultiNode: true,
        availableNetworkInterfaces: [],
      },
      updateConfig: () => {},
      updatePrototypeSettings: () => {},
      resetConfig: () => {},
    },
    wizardModeContext: {
      mode: "install",
      text: {
        title: "My App",
        subtitle: "Installation Wizard",
        welcomeTitle: "Welcome to My App",
        welcomeDescription: "This wizard will guide you through installing My App on your Linux machine.",
        setupTitle: "Setup",
        setupDescription: "Set up the hosts to use for this installation.",
        validationTitle: "Validation",
        validationDescription: "Validate the host requirements before proceeding with installation.",
        installationTitle: "Installing My App",
        installationDescription: "",
        welcomeButtonText: "Start",
        nextButtonText: "Next: Start Installation",
      },
    },
  }
) => {
  const { wrapperProps = {} } = options;
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

  // Create router with the test component wrapped in MockProvider
  const routes: RouteObject[] = [
    {
      path: routePath || "/*",
      element: (
        <MockProvider queryClient={queryClient} contexts={contextValues}>
          {ui}
        </MockProvider>
      ),
    },
  ];
  const router = createMemoryRouter(routes, {
    initialEntries,
  });

  const view = render(<RouterProvider router={router} />, options);

  return { ...view, router };
};
