import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesInstallationStep from "../installation/KubernetesInstallationStep.tsx";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

const server = setupServer(
  // Mock installation status endpoint
  http.get("*/api/kubernetes/install/infra/status", () => {
    return HttpResponse.json({
      status: { state: "Running", description: "Installing..." },
      components: [
        { name: "Admin Console", status: { state: "Pending" } },
        { name: "Disaster Recovery", status: { state: "Pending" } }
      ]
    });
  })
);

describe("KubernetesInstallationStep", () => {
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
    renderWithProviders(<KubernetesInstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
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
    
    // Verify Admin Console component
    const adminConsoleContainer = screen.getByTestId("status-indicator-admin-console");
    expect(adminConsoleContainer).toBeInTheDocument();
    expect(within(adminConsoleContainer).getByTestId("status-title")).toHaveTextContent("Admin Console");
    expect(within(adminConsoleContainer).getByTestId("status-text")).toHaveTextContent("Pending");
    
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
      http.get("*/api/kubernetes/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "InProgress", description: "Installing components..." },
          components: [
            { name: "Admin Console", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Running" } }
          ]
        });
      })
    );

    renderWithProviders(<KubernetesInstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    // Wait for progress update
    await waitFor(() => {
      expect(screen.getByText("Installing components...")).toBeInTheDocument();
    });

    // Verify Admin Console component
    const adminConsoleContainer = screen.getByTestId("status-indicator-admin-console");
    expect(adminConsoleContainer).toBeInTheDocument();
    expect(within(adminConsoleContainer).getByTestId("status-title")).toHaveTextContent("Admin Console");
    expect(within(adminConsoleContainer).getByTestId("status-text")).toHaveTextContent("Complete");

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
      http.get("*/api/kubernetes/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { state: "Succeeded", description: "Installation complete" },
          components: [
            { name: "Admin Console", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Succeeded" } }
          ]
        });
      })
    );

    renderWithProviders(<KubernetesInstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText("Next: Finish")).not.toBeDisabled();
    });

    // Verify final state
    expect(screen.getByText("Installation complete")).toBeInTheDocument();
    
    // Verify Admin Console component
    const adminConsoleContainer = screen.getByTestId("status-indicator-admin-console");
    expect(adminConsoleContainer).toBeInTheDocument();
    expect(within(adminConsoleContainer).getByTestId("status-title")).toHaveTextContent("Admin Console");
    expect(within(adminConsoleContainer).getByTestId("status-text")).toHaveTextContent("Complete");

    // Verify Disaster Recovery component
    const drContainer = screen.getByTestId("status-indicator-disaster-recovery");
    expect(drContainer).toBeInTheDocument();
    expect(within(drContainer).getByTestId("status-title")).toHaveTextContent("Disaster Recovery");
    expect(within(drContainer).getByTestId("status-text")).toHaveTextContent("Complete");
  });

  it("shows error message when installation fails", async () => {
    const mockOnNext = vi.fn();
    server.use(
      http.get("*/api/kubernetes/install/infra/status", ({ request }) => {
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          status: { 
            state: "Failed", 
            description: "Installation failed: Disaster Recovery setup failed" 
          },
          components: [
            { name: "Admin Console", status: { state: "Succeeded" } },
            { name: "Disaster Recovery", status: { state: "Failed" } }
          ]
        });
      })
    );

    renderWithProviders(<KubernetesInstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    // Wait for error state
    await waitFor(() => {
      const errorMessage = screen.getByTestId("error-message");
      expect(errorMessage).toBeInTheDocument();
      expect(within(errorMessage).getByText("Installation failed: Disaster Recovery setup failed")).toBeInTheDocument();
    });

    // Verify Admin Console component
    const adminConsoleContainer = screen.getByTestId("status-indicator-admin-console");
    expect(adminConsoleContainer).toBeInTheDocument();
    expect(within(adminConsoleContainer).getByTestId("status-title")).toHaveTextContent("Admin Console");
    expect(within(adminConsoleContainer).getByTestId("status-text")).toHaveTextContent("Complete");

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
      http.get("*/api/kubernetes/install/infra/status", () => {
        return HttpResponse.json({
          status: { state: "Running", description: "Installing..." },
          components: [
            { name: "Admin Console", status: { state: "Pending" } },
            { name: "Disaster Recovery", status: { state: "Pending" } }
          ],
          logs: "[k0s] creating k0s configuration file\n[k0s] creating systemd unit files"
        });
      })
    );

    renderWithProviders(<KubernetesInstallationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
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
}); 