import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ValidationStep from "../ValidationStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

const server = setupServer(
  // Mock installation start endpoint
  http.post("*/api/install/infra/setup", () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock installation status endpoint (for LinuxPreflightCheck)
  http.get("*/api/install/installation/status", () => {
    return HttpResponse.json({ state: "Succeeded" });
  }),

  // Mock preflight run endpoint
  http.post("*/api/install/host-preflights/run", () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock preflight status endpoint  
  http.get("*/api/install/host-preflights/status", () => {
    return HttpResponse.json({
      output: {
        pass: [{ title: "CPU Check", message: "CPU requirements met" }],
      },
      status: { state: "Succeeded" },
    });
  })
);

describe("ValidationStep", () => {
  const mockOnComplete = vi.fn();
  const mockOnBack = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("renders validation step with correct content", async () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Check if validation step is rendered
    expect(screen.getByTestId("validation-step")).toBeInTheDocument();

    // Check title and description
    expect(screen.getByText("Setup")).toBeInTheDocument();
    expect(screen.getByText("Validate the installation settings.")).toBeInTheDocument();

    // Check buttons
    expect(screen.getByText("Back")).toBeInTheDocument();
    expect(screen.getByText("Next: Start Installation")).toBeInTheDocument();
  });

  it("calls onBack when Back button is clicked", () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    const backButton = screen.getByText("Back");
    fireEvent.click(backButton);

    expect(mockOnBack).toHaveBeenCalledOnce();
  });

  it("disables Start Installation button initially", () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).toBeDisabled();
  });

  it("enables Start Installation button when preflights succeed", async () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Start Installation button should now be enabled
    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).not.toBeDisabled();
  });

  it("calls installation start API and onComplete when Start Installation clicked", async () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Click Start Installation button
    const startInstallButton = screen.getByText("Next: Start Installation");
    fireEvent.click(startInstallButton);

    // Should call onComplete with true
    await waitFor(() => {
      expect(mockOnComplete).toHaveBeenCalledWith(true);
    });
  });

  it("handles installation start API errors", async () => {
    // Mock installation start to fail
    server.use(
      http.post("*/api/install/infra/setup", () => {
        return new HttpResponse(JSON.stringify({ message: "Installation failed" }), { status: 500 });
      })
    );

    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Click Start Installation button
    const startInstallButton = screen.getByText("Next: Start Installation");
    fireEvent.click(startInstallButton);

    // Should show error message
    await waitFor(() => {
      expect(screen.getByText("Installation failed")).toBeInTheDocument();
    });

    // Should not call onComplete
    expect(mockOnComplete).not.toHaveBeenCalled();
  });

  it("handles preflight failures correctly", async () => {
    // Mock preflights to fail
    server.use(
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            fail: [{ title: "Disk Space", message: "Not enough disk space" }],
          },
          status: { state: "Failed" },
        });
      })
    );

    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to fail
    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
    });

    // Start Installation button should remain disabled
    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).toBeDisabled();
  });

  it("shows loading states during validation", () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Should show initializing state initially
    expect(screen.getByText("Initializing...")).toBeInTheDocument();
  });

  it("handles authentication errors correctly", async () => {
    // Mock installation start to return 401
    server.use(
      http.post("*/api/install/infra/setup", () => {
        return new HttpResponse(JSON.stringify({ message: "Unauthorized" }), { status: 401 });
      })
    );

    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Click Start Installation button
    const startInstallButton = screen.getByText("Next: Start Installation");
    fireEvent.click(startInstallButton);

    // Should show session expired error message
    await waitFor(() => {
      expect(screen.getByText("Session expired. Please log in again.")).toBeInTheDocument();
    });

    // Should not call onComplete
    expect(mockOnComplete).not.toHaveBeenCalled();
  });

  it("handles network errors gracefully", async () => {
    // Mock installation start to fail with network error
    server.use(
      http.post("*/api/install/infra/setup", () => {
        return new HttpResponse(null, { status: 0 }); // Network error
      })
    );

    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Click Start Installation button
    const startInstallButton = screen.getByText("Next: Start Installation");
    fireEvent.click(startInstallButton);

    // Should show error message (this is what actually gets displayed with status 0)
    await waitFor(() => {
      expect(screen.getByText("Unexpected end of JSON input")).toBeInTheDocument();
    });

    // Should not call onComplete
    expect(mockOnComplete).not.toHaveBeenCalled();
  });

  it("does not call onComplete when validation completes unsuccessfully", async () => {
    // Mock preflights to fail
    server.use(
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            fail: [{ title: "Memory Check", message: "Insufficient memory" }],
          },
          status: { state: "Failed" },
        });
      })
    );

    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to fail
    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
    });

    // onComplete should not be called automatically for failed validation
    expect(mockOnComplete).not.toHaveBeenCalled();

    // Button should remain disabled
    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).toBeDisabled();
  });

  it("properly handles LinuxPreflightCheck onComplete callback", async () => {
    renderWithProviders(
      <ValidationStep onComplete={mockOnComplete} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true,
          preloadedState: {
            prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          },
        },
      }
    );

    // Wait for preflights to complete successfully
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Verify that internal state is updated correctly
    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).not.toBeDisabled();

    // Verify no external onComplete call until installation is started
    expect(mockOnComplete).not.toHaveBeenCalled();
  });
}); 