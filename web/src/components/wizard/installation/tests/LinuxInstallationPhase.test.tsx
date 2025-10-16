import { describe, it, expect, vi, beforeEach, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor, within, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../../test/setup.tsx";
import LinuxInstallationPhase from "../phases/LinuxInstallationPhase.tsx";
import { withTestButton } from "./TestWrapper.tsx";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

const TestLinuxInstallationPhase = withTestButton(LinuxInstallationPhase);

const server = setupServer(
  // Mock installation status endpoint
  http.get("*/api/linux/install/infra/status", () => {
    return HttpResponse.json({
      status: { state: "Running", description: "Installing..." },
      components: [
        { name: "Runtime", status: { state: "Pending" } },
        { name: "Disaster Recovery", status: { state: "Pending" } }
      ]
    });
  })
);

describe("LinuxInstallationPhase", () => {
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
    const mockOnStateChange = vi.fn();
    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />, 
      {
        wrapperProps: {
          authenticated: true,
        },
      }
    );

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
    const nextButton = screen.getByTestId("next-button");
    expect(nextButton).toBeDisabled();
  });

  it("shows progress as components complete", async () => {
    const mockOnNext = vi.fn();
    const mockOnStateChange = vi.fn();
    server.use(
      http.get("*/api/linux/install/infra/status", ({ request }) => {
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

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />, 
      {
        wrapperProps: {
          authenticated: true,
        },
      }
    );

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
    expect(screen.getByTestId("next-button")).toBeDisabled();
  });

  it("enables next button when installation succeeds", async () => {
    const mockOnNext = vi.fn();
    const mockOnStateChange = vi.fn();
    server.use(
      http.get("*/api/linux/install/infra/status", ({ request }) => {
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

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />, 
      {
        wrapperProps: {
          authenticated: true,
        },
      }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByTestId("next-button")).not.toBeDisabled();
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
    const mockOnStateChange = vi.fn();
    server.use(
      http.get("*/api/linux/install/infra/status", ({ request }) => {
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

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />, 
      {
        wrapperProps: {
          authenticated: true,
        },
      }
    );

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
    expect(screen.getByTestId("next-button")).toBeDisabled();
  });

  it("verify log viewer", async () => {
    const mockOnNext = vi.fn();
    const mockOnStateChange = vi.fn();
    server.use(
      http.get("*/api/linux/install/infra/status", () => {
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

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />, 
      {
        wrapperProps: {
          authenticated: true,
        },
      }
    );

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

// Tests specifically for onStateChange callback
describe('LinuxInstallationPhase - onStateChange Tests', () => {
  beforeAll(() => server.listen());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls onStateChange with "Running" immediately when component mounts', async () => {
    // Mock infra status endpoint - returns running state
    server.use(
      http.get('*/api/linux/install/infra/status', () => {
        return HttpResponse.json({
          status: { state: 'Running', description: 'Installing...' },
          components: [
            { name: 'Runtime', status: { state: 'Running' } },
            { name: 'Disaster Recovery', status: { state: 'Pending' } }
          ]
        });
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');
    expect(mockOnStateChange).toHaveBeenCalledTimes(1);
  });

  it('calls onStateChange with "Succeeded" when installation completes successfully', async () => {
    // Mock infra status endpoint - returns success
    server.use(
      http.get('*/api/linux/install/infra/status', () => {
        return HttpResponse.json({
          status: { state: 'Succeeded', description: 'Installation completed successfully' },
          components: [
            { name: 'Runtime', status: { state: 'Succeeded' } },
            { name: 'Disaster Recovery', status: { state: 'Succeeded' } }
          ]
        });
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Wait for installation to complete
    await waitFor(() => {
      expect(screen.getByText('Installation completed successfully')).toBeInTheDocument();
    });

    // Should also call onStateChange with "Succeeded" when installation completes
    expect(mockOnStateChange).toHaveBeenCalledWith('Succeeded');
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });

  it('calls onStateChange with "Failed" when installation fails', async () => {
    // Mock infra status endpoint - returns failure
    server.use(
      http.get('*/api/linux/install/infra/status', () => {
        return HttpResponse.json({
          status: { state: 'Failed', description: 'Installation failed' },
          components: [
            { name: 'Runtime', status: { state: 'Failed' } },
            { name: 'Disaster Recovery', status: { state: 'Pending' } }
          ]
        });
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Wait for installation to fail
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Should also call onStateChange with "Failed" when installation fails
    expect(mockOnStateChange).toHaveBeenCalledWith('Failed');
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });
});

// Tests for mutation behavior (errors, success, validation, retry logic)
describe.each([
  { mode: "install" as const, modeDisplayName: "Install Mode" },
  { mode: "upgrade" as const, modeDisplayName: "Upgrade Mode" }
])('LinuxInstallationPhase - Mutation Behavior - $modeDisplayName', ({ mode }) => {
  beforeAll(() => server.listen());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  // Determine the correct API endpoints based on mode
  const statusEndpoint = `/api/linux/${mode}/infra/status`;
  const mutationEndpoint = mode === 'install' ? '/api/linux/install/infra/setup' : '/api/linux/upgrade/infra/upgrade';

  it('handles API error responses gracefully when starting installation', async () => {
    // Mock infra status endpoint to return Pending state to trigger mutation
    server.use(
      http.get(`*${statusEndpoint}`, () => {
        return HttpResponse.json({
          status: { state: 'Pending', description: 'Waiting to start...' },
          components: []
        });
      }),
      // Mock infra setup/upgrade endpoint to return API error
      http.post(`*${mutationEndpoint}`, () => {
        return HttpResponse.json(
          {
            statusCode: 400,
            message: 'Preflight checks failed. Cannot proceed with installation.'
          },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true, mode } }
    );

    // Should show error message when mutation fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
      const errorMessage = screen.getByTestId('error-message');
      expect(errorMessage).toHaveTextContent(/Installation Error/);
    });

    // Should call onStateChange with "Failed" when mutation fails
    expect(mockOnStateChange).toHaveBeenCalledWith('Failed');

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles network failure during installation start', async () => {
    // Mock infra status endpoint to return Pending state to trigger mutation
    server.use(
      http.get(`*${statusEndpoint}`, () => {
        return HttpResponse.json({
          status: { state: 'Pending', description: 'Waiting to start...' },
          components: []
        });
      }),
      // Mock infra setup/upgrade endpoint to return network error
      http.post(`*${mutationEndpoint}`, () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true, mode } }
    );

    // Should show error message when network fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
      const errorMessage = screen.getByTestId('error-message');
      expect(errorMessage).toHaveTextContent(/Installation Error/);
    });

    // Should call onStateChange with "Failed"
    expect(mockOnStateChange).toHaveBeenCalledWith('Failed');

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles successful mutation and proceeds correctly', async () => {
    // Mock infra status endpoint to transition from Pending to Running to Succeeded
    let statusCallCount = 0;
    server.use(
      http.get(`*${statusEndpoint}`, () => {
        statusCallCount++;
        if (statusCallCount === 1) {
          // First call - trigger mutation
          return HttpResponse.json({
            status: { state: 'Pending', description: 'Waiting to start...' },
            components: []
          });
        } else if (statusCallCount === 2) {
          // Second call - after mutation started
          return HttpResponse.json({
            status: { state: 'Running', description: 'Installing...' },
            components: [
              { name: 'Runtime', status: { state: 'Running' } },
              { name: 'Disaster Recovery', status: { state: 'Pending' } }
            ]
          });
        }
        // Subsequent calls - installation complete
        return HttpResponse.json({
          status: { state: 'Succeeded', description: 'Installation complete' },
          components: [
            { name: 'Runtime', status: { state: 'Succeeded' } },
            { name: 'Disaster Recovery', status: { state: 'Succeeded' } }
          ]
        });
      }),
      // Mock successful infra setup/upgrade
      http.post(`*${mutationEndpoint}`, () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true, mode } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Installation complete')).toBeInTheDocument();
    }, { timeout: 5000 });

    // Should call onStateChange with "Succeeded"
    expect(mockOnStateChange).toHaveBeenCalledWith('Succeeded');

    // Wait for next button to be enabled
    await waitFor(() => {
      expect(screen.getByTestId('next-button')).not.toBeDisabled();
    });
  });

  it('passes ignoreHostPreflights flag correctly to mutation', async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let mutationArgs: any = null;

    // Mock infra status endpoint to return Pending state to trigger mutation
    server.use(
      http.get(`*${statusEndpoint}`, () => {
        return HttpResponse.json({
          status: { state: 'Pending', description: 'Waiting to start...' },
          components: []
        });
      }),
      // Mock infra setup/upgrade endpoint to capture arguments
      http.post(`*${mutationEndpoint}`, async ({ request }) => {
        mutationArgs = await request.json();
        return HttpResponse.json(
          {
            statusCode: 400,
            message: 'preflight checks failed'
          },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={true}
      />,
      { wrapperProps: { authenticated: true, mode } }
    );

    // Wait for mutation to be called
    await waitFor(() => {
      expect(mutationArgs).not.toBeNull();
    });

    // Verify ignoreHostPreflights was passed correctly (upgrade mode also includes isUi)
    if (mode === 'upgrade') {
      expect(mutationArgs).toEqual({ isUi: true, ignoreHostPreflights: true });
    } else {
      expect(mutationArgs).toEqual({ ignoreHostPreflights: true });
    }

    // Should show error message
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
      const errorMessage = screen.getByTestId('error-message');
      expect(errorMessage).toHaveTextContent(/Installation Error/);
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('does not retry mutation infinitely on failure', async () => {
    let mutationCallCount = 0;

    // Mock infra status endpoint to return Pending state
    server.use(
      http.get(`*${statusEndpoint}`, () => {
        return HttpResponse.json({
          status: { state: 'Pending', description: 'Waiting to start...' },
          components: []
        });
      }),
      // Mock infra setup/upgrade endpoint to count calls and fail
      http.post(`*${mutationEndpoint}`, () => {
        mutationCallCount++;
        return HttpResponse.json(
          {
            statusCode: 500,
            message: 'Internal server error'
          },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <TestLinuxInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreHostPreflights={false}
      />,
      { wrapperProps: { authenticated: true, mode } }
    );

    // Wait for error to appear
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Wait a bit more to ensure no additional calls
    await new Promise(resolve => setTimeout(resolve, 1000));

    // Mutation should only be called once
    expect(mutationCallCount).toBe(1);

    // Should still show error
    expect(screen.getByTestId('error-message')).toBeInTheDocument();

    // Should NOT proceed
    expect(mockOnNext).not.toHaveBeenCalled();
  });
});
