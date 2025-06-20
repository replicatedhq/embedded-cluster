import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallationStep from "../InstallationStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

const server = setupServer(
  // Mock installation status endpoint
  http.get("*/api/install/infra/status", () => {
    return HttpResponse.json({
      status: { state: "Running", description: "Installing..." },
      components: [
        { name: "Runtime", status: { state: "Pending" } },
        { name: "Disaster Recovery", status: { state: "Pending" } }
      ]
    });
  })
);

describe("InstallationStep", () => {
  const mockConfig = {
    useHttps: false,
    domain: "localhost:8080",
    adminConsolePort: 30000,
  };

  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("shows initial installation UI", async () => {
    const mockOnNext = vi.fn();
    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Check header
    expect(screen.getByText("Installation")).toBeInTheDocument();
    expect(screen.getByText("Installing infrastructure components")).toBeInTheDocument();

    // Check progress and status indicators
    expect(screen.getByText("Preparing installation...")).toBeInTheDocument();

    // Wait for progress update
    await waitFor(() => {
      expect(screen.getByText("Installing...")).toBeInTheDocument();
    });
    
    // Verify Runtime component
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(runtimeContainer).toBeInTheDocument();
    expect(within(runtimeContainer).getByTestId("status-title")).toHaveTextContent("Runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Pending");
    
    // Verify Disaster Recovery component
    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(drContainer).toBeInTheDocument();
    expect(within(drContainer).getByTestId("status-title")).toHaveTextContent("Disaster Recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Pending");

    // Check next button is disabled
    const nextButton = screen.getByText("Next: Finish");
    expect(nextButton).toBeDisabled();
  });

  it("shows progress as components complete", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "InProgress", description: "Installing components..." },
          components: [
            { name: "Runtime", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Running" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for progress update
    await waitFor(() => {
      expect(screen.getByText("Installing components...")).toBeInTheDocument();
    });

    // Verify Runtime component
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(runtimeContainer).toBeInTheDocument();
    expect(within(runtimeContainer).getByTestId("status-title")).toHaveTextContent("Runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    // Verify Disaster Recovery component
    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(drContainer).toBeInTheDocument();
    expect(within(drContainer).getByTestId("status-title")).toHaveTextContent("Disaster Recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Installing...");

    // Next button should still be disabled
    expect(screen.getByText("Next: Finish")).toBeDisabled();
  });

  it("enables next button when installation succeeds", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Succeeded", description: "Installation complete" },
          components: [
            { name: "Runtime", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Succeeded" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText("Next: Finish")).not.toBeDisabled();
    });

    // Verify final state
    expect(screen.getByText("Installation complete")).toBeInTheDocument();
    
    // Verify Runtime component
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(runtimeContainer).toBeInTheDocument();
    expect(within(runtimeContainer).getByTestId("status-title")).toHaveTextContent("Runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    // Verify Disaster Recovery component
    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(drContainer).toBeInTheDocument();
    expect(within(drContainer).getByTestId("status-title")).toHaveTextContent("Disaster Recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Complete");
  });

  it("shows error message when installation fails", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { 
            state: "Failed", 
            description: "Installation failed: Disaster Recovery setup failed" 
          },
          components: [
            { name: "Runtime", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Failed" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for error state
    await waitFor(() => {
      const errorMessage = screen.getByTestId("error-message");
      expect(errorMessage).toBeInTheDocument();
      expect(within(errorMessage).getByText("Installation failed: Disaster Recovery setup failed")).toBeInTheDocument();
    });

    // Verify Runtime component
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(runtimeContainer).toBeInTheDocument();
    expect(within(runtimeContainer).getByTestId("status-title")).toHaveTextContent("Runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    // Verify Disaster Recovery component
    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(drContainer).toBeInTheDocument();
    expect(within(drContainer).getByTestId("status-title")).toHaveTextContent("Disaster Recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Failed");

    // Next button should be disabled
    expect(screen.getByText("Next: Finish")).toBeDisabled();
  });

  it("verify log viewer", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/install/infra/status", () => {
        return HttpResponse.json({
          status: { state: "Running", description: "Installing..." },
          components: [
            { name: "Runtime", status: { state: "Pending" } },
            { name: "Disaster Recovery", status: { state: "Pending" } }
          ],
          logs: "[k0s] creating k0s configuration file\n[k0s] creating systemd unit files"
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for log viewer to be available
    await waitFor(() => {
      expect(screen.getByTestId("log-viewer")).toBeInTheDocument();
    });

    // Initially logs should be collapsed and not visible
    expect(screen.queryByTestId("log-viewer-content")).not.toBeInTheDocument();

    // Expand and verify logs
    const toggleButton = screen.getByTestId("log-viewer-toggle");
    expect(toggleButton).toBeInTheDocument();
    fireEvent.click(toggleButton);
    await waitFor(() => {
      const logContent = screen.getByTestId("log-viewer-content");
      expect(logContent).toHaveTextContent("[k0s] creating k0s configuration file");
      expect(logContent).toHaveTextContent("[k0s] creating systemd unit files");
    });

    // Click to collapse logs
    expect(toggleButton).toBeInTheDocument();
    fireEvent.click(toggleButton);
    await waitFor(() => {
      expect(screen.queryByTestId("log-viewer-content")).not.toBeInTheDocument();
    });
  });

  it("shows correct progress for Runtime installation phases", async () => {
    const mockOnNext = vi.fn();
    
    // Test Runtime installing phase
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Running", description: "Installing Runtime" },
          components: [
            { name: "Runtime", status: { state: "Running" } },
            { name: "Disaster Recovery", status: { state: "Pending" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for progress update
    await waitFor(() => {
      expect(screen.getByText("Installing Runtime")).toBeInTheDocument();
    });

    // Verify Runtime component is installing
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(runtimeContainer).toBeInTheDocument();
    expect(within(runtimeContainer).getByTestId("status-title")).toHaveTextContent("Runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Installing...");

    // Test Runtime waiting phase
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Running", description: "Waiting for Runtime" },
          components: [
            { name: "Runtime", status: { state: "Running" } },
            { name: "Disaster Recovery", status: { state: "Pending" } }
          ]
        });
      })
    );

    // Wait for progress update with retry
    await waitFor(() => {
      expect(screen.getByText("Waiting for Runtime")).toBeInTheDocument();
    }, { timeout: 5000 }); // Increase timeout and add implicit retry

    // Verify Runtime component is still running but in waiting phase
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Installing...");
  });

  it("shows correct progress for mixed component states", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Running", description: "Installing components..." },
          components: [
            { name: "Runtime", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Running" } },
            { name: "Additional Components", status: { state: "Pending" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for progress update
    await waitFor(() => {
      expect(screen.getByText("Installing components...")).toBeInTheDocument();
    });

    // Verify component states
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Installing...");

    const additionalContainer = screen.getByTestId("status-indicator-additional-components");
    expect(within(additionalContainer).getByTestId("status-text")).toHaveTextContent("Pending");
  });

  it("shows completion messages when components succeed", async () => {
    const mockOnNext = vi.fn();
    
    // Test component completion message
    server.use(
      http.get("*/api/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Running", description: "Installing components..." },
          components: [
            { name: "Runtime", status: { state: "Succeeded" } },
            { name: "Storage", status: { state: "Succeeded" } },
            { name: "Runtime Operator", status: { state: "Running" } }
          ]
        });
      })
    );

    renderWithProviders(<InstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for completion message
    await waitFor(() => {
      expect(screen.getByText("Installing components...")).toBeInTheDocument();
    });

    // Verify component states
    const runtimeContainer = screen.getByTestId("status-indicator-runtime");
    expect(within(runtimeContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    const storageContainer = screen.getByTestId("status-indicator-storage");
    expect(within(storageContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    const operatorContainer = screen.getByTestId("status-indicator-runtime-operator");
    expect(within(operatorContainer).getByTestId("status-text")).toHaveTextContent("Installing...");
  });
});
