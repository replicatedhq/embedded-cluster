import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallWizard from "../InstallWizard";

// Mock all wizard step components - this test only validates navigation and state restoration
vi.mock("../config/ConfigurationStep", () => ({
  default: () => <div data-testid="configuration-step">Configuration Step</div>,
}));

vi.mock("../setup/LinuxSetupStep", () => ({
  default: () => <div data-testid="linux-setup">Linux Setup</div>,
}));

vi.mock("../setup/KubernetesSetupStep", () => ({
  default: () => <div data-testid="kubernetes-setup">Kubernetes Setup</div>,
}));

vi.mock("../installation/InstallationStep", () => ({
  default: () => <div data-testid="installation-step-mock">Installation Step</div>,
}));

vi.mock("../completion/LinuxCompletionStep", () => ({
  default: () => <div data-testid="linux-completion">Linux Completion</div>,
}));

vi.mock("../completion/KubernetesCompletionStep", () => ({
  default: () => <div data-testid="kubernetes-completion">Kubernetes Completion</div>,
}));

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])("InstallWizard - $displayName", ({ target }) => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
  });

  it("shows welcome step by default", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });
  });

  it("includes configuration step in install mode", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: "install"
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });

    // After welcome, configuration step should be accessible
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });
  });

  it("includes configuration step in upgrade mode", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: "upgrade"
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });
  });

  it("restores to configuration step from sessionStorage", async () => {
    const STORAGE_KEY = "embedded-cluster-install-progress";
    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        wizardStep: "configuration",
        installationPhase: undefined,
      })
    );

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });
  });

  it("restores to setup step from sessionStorage", async () => {
    const STORAGE_KEY = "embedded-cluster-install-progress";
    const setupStep = target === "linux" ? "linux-setup" : "kubernetes-setup";

    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        wizardStep: setupStep,
        installationPhase: undefined,
      })
    );

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId(`${target}-setup`)).toBeInTheDocument();
    });
  });

  it("restores to installation step from sessionStorage", async () => {
    const STORAGE_KEY = "embedded-cluster-install-progress";

    sessionStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        wizardStep: "installation",
        installationPhase: undefined,
      })
    );

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("installation-step-mock")).toBeInTheDocument();
    });
  });

  it("defaults to welcome step when sessionStorage has invalid data", async () => {
    const STORAGE_KEY = "embedded-cluster-install-progress";
    sessionStorage.setItem(STORAGE_KEY, "invalid-json{");

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });
  });

  it("defaults to welcome step when no sessionStorage data exists", async () => {
    sessionStorage.clear();

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });
  });
});
