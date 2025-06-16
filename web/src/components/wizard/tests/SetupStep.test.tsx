import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import SetupStep from "../SetupStep.tsx";
import { MOCK_INSTALL_CONFIG, MOCK_NETWORK_INTERFACES, MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

const server = setupServer(
  // Mock install config endpoint
  http.get("*/api/install/installation/config", () => {
    return HttpResponse.json({ config: MOCK_INSTALL_CONFIG });
  }),

  // Mock network interfaces endpoint
  http.get("*/api/console/available-network-interfaces", () => {
    return HttpResponse.json({ networkInterfaces: MOCK_NETWORK_INTERFACES });
  }),

  // Mock config submission endpoint
  http.post("*/api/install/installation/configure", () => {
    return HttpResponse.json({ success: true });
  })
);

describe("SetupStep", () => {
  const mockOnNext = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  beforeEach(() => {
    // No need to set localStorage token anymore as it's handled by the test setup
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("renders the linux setup form when it's embedded", async () => {
    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            clusterName: "",
            namespace: "",
            storageClass: "standard",
            domain: "",
            useHttps: true,
            adminUsername: "admin",
            adminPassword: "",
            adminEmail: "",
            databaseType: "internal",
            useProxy: false,
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Check if form elements are rendered
    expect(screen.getByTestId("setup-step")).toBeInTheDocument();

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");

    await screen.findByText("Configure the installation settings.");

    // Wait for the linux-setup element to appear
    await screen.findByTestId("linux-setup");

    // Check all input fields are present
    await screen.findByLabelText(/Data Directory/);

    screen.getByLabelText(/Admin Console Port/, { selector: "input" });

    screen.getByLabelText(/Local Artifact Mirror Port/, { selector: "input" });

    // Check proxy configuration inputs
    screen.getByLabelText(/HTTP Proxy/, { selector: "input" });

    screen.getByLabelText(/HTTPS Proxy/, { selector: "input" });

    screen.getByLabelText(/Proxy Bypass List/, { selector: "input" });

    // Reveal advanced settings before checking advanced fields
    const advancedButton = screen.getByRole("button", { name: /Advanced Settings/i });
    fireEvent.click(advancedButton);

    // Check advanced settings
    screen.getByLabelText(/Network Interface/, { selector: "select" });

    screen.getByLabelText(/Reserved Network Range/, {
      selector: "input",
    });

    // Check next button
    const nextButton = screen.getByText("Next: Validate Host");
    expect(nextButton).toBeInTheDocument();
  });

  it("handles form errors gracefully", async () => {
    server.use(
      http.get("*/api/console/available-network-interfaces", () => {
        return HttpResponse.json({
          networkInterfaces: MOCK_NETWORK_INTERFACES,
        });
      }),
      // Mock config submission endpoint to return an error
      http.post("*/api/install/installation/configure", () => {
        return new HttpResponse(JSON.stringify({ message: "Invalid configuration" }), { status: 400 });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            clusterName: "",
            namespace: "",
            storageClass: "standard",
            domain: "",
            useHttps: true,
            adminUsername: "admin",
            adminPassword: "",
            adminEmail: "",
            databaseType: "internal",
            useProxy: false,
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Configure the installation settings.");

    // Wait for the linux-setup element to appear
    await screen.findByTestId("linux-setup");

    // Fill in required form values
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    const adminPortInput = screen.getByLabelText(/Admin Console Port/);
    const mirrorPortInput = screen.getByLabelText(/Local Artifact Mirror Port/);

    // Use fireEvent to simulate user input
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/my-cluster" },
    });
    fireEvent.change(adminPortInput, { target: { value: "8080" } });
    fireEvent.change(mirrorPortInput, { target: { value: "8081" } });

    // Submit form
    const nextButton = screen.getByText("Next: Validate Host");
    fireEvent.click(nextButton);

    // Verify error message is displayed
    await screen.findByText("Please fix the errors in the form above before proceeding.");

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("submits the form successfully", async () => {
    // Mock all required API endpoints
    server.use(
      // Mock install config endpoint
      http.get("*/api/install/installation/config", ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            storageClass: "standard",
          },
        });
      }),
      // Mock network interfaces endpoint
      http.get("*/api/console/available-network-interfaces", ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          networkInterfaces: MOCK_NETWORK_INTERFACES,
        });
      }),
      // Mock config submission endpoint
      http.post("*/api/install/installation/configure", async ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        const body = await request.json();
        // Verify the request body has all required fields
        expect(body).toMatchObject({
          storageClass: "standard",
          dataDirectory: "/var/lib/embedded-cluster",
        });
        return new HttpResponse(JSON.stringify({ success: true }), {
          status: 200,
          headers: {
            "Content-Type": "application/json",
          },
        });
      }),
      // Mock installation status for validation view
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            storageClass: "standard",
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Configure the installation settings.");

    // Wait for the linux-setup element to appear
    await screen.findByTestId("linux-setup");

    // Fill in all required form values
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    const adminPortInput = screen.getByLabelText(/Admin Console Port/);
    const mirrorPortInput = screen.getByLabelText(/Local Artifact Mirror Port/);

    // Reveal advanced settings before filling advanced fields
    const advancedButton = screen.getByRole("button", { name: /Advanced Settings/i });
    fireEvent.click(advancedButton);

    const networkInterfaceSelect = screen.getByLabelText(/Network Interface/);
    const globalCidrInput = screen.getByLabelText(/Reserved Network Range/);

    // Use fireEvent to simulate user input
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/embedded-cluster" },
    });
    fireEvent.change(adminPortInput, { target: { value: "8080" } });
    fireEvent.change(mirrorPortInput, { target: { value: "8081" } });
    fireEvent.change(networkInterfaceSelect, { target: { value: "eth0" } });
    fireEvent.change(globalCidrInput, { target: { value: "10.244.0.0/16" } });

    // Get the next button and ensure it's not disabled
    const nextButton = screen.getByText("Next: Validate Host");
    expect(nextButton).not.toBeDisabled();

    // Submit form
    fireEvent.click(nextButton);

    // Wait for transition to validation view (new behavior)
    await waitFor(() => {
      expect(screen.getByText("Validate the host requirements before proceeding with installation.")).toBeInTheDocument();
    });

    // Verify we're now in validation view with proper buttons
    expect(screen.getByText("Back to Configuration")).toBeInTheDocument();
  });

  // New tests for consolidated validation functionality
  it("shows validation view when Next: Validate Host is clicked", async () => {
    // Mock successful configuration submission and installation status
    server.use(
      http.post("*/api/install/installation/configure", () => {
        return HttpResponse.json({ success: true });
      }),
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      }),
      http.post("*/api/install/host-preflights/run", () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            storageClass: "standard",
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Configure the installation settings.");
    await screen.findByTestId("linux-setup");

    // Fill in form and click Next: Validate Host
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/embedded-cluster" },
    });

    const nextButton = screen.getByText("Next: Validate Host");
    fireEvent.click(nextButton);

    // Should show validation view instead of configuration
    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
    });

    // Should show Back to Configuration button
    expect(screen.getByText("Back to Configuration")).toBeInTheDocument();
  });

  it("enables Start Installation button when validation passes", async () => {
    // Mock successful configuration and validation
    server.use(
      http.post("*/api/install/installation/configure", () => {
        return HttpResponse.json({ success: true });
      }),
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      }),
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            pass: [{ title: "CPU Check", message: "CPU requirements met" }],
          },
          status: { state: "Succeeded" },
        });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            storageClass: "standard",
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for form to load
    await screen.findByText("Configure the installation settings.");
    await screen.findByTestId("linux-setup");

    // Fill form and navigate to validation view
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/embedded-cluster" },
    });

    const nextButton = screen.getByText("Next: Validate Host");
    fireEvent.click(nextButton);

    // Wait for validation success
    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });

    // Should show and enable Start Installation button
    const startInstallButton = screen.getByText("Next: Start Installation");
    expect(startInstallButton).toBeInTheDocument();
    expect(startInstallButton).not.toBeDisabled();

    // Should call onNext when clicked
    fireEvent.click(startInstallButton);
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalled();
    });
  });

  it("returns to configuration view when Back to Configuration is clicked", async () => {
    // Mock successful configuration submission
    server.use(
      http.post("*/api/install/installation/configure", () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for form to load
    await screen.findByText("Configure the installation settings.");
    await screen.findByTestId("linux-setup");

    // Fill form and navigate to validation view
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/embedded-cluster" },
    });

    const nextButton = screen.getByText("Next: Validate Host");
    fireEvent.click(nextButton);

    // Wait for validation view to appear
    await waitFor(() => {
      expect(screen.getByText("Validate the host requirements before proceeding with installation.")).toBeInTheDocument();
    });

    // Click Back to Configuration
    const backButton = screen.getByText("Back to Configuration");
    fireEvent.click(backButton);

    // Should return to configuration view
    await waitFor(() => {
      expect(screen.getByText("Configure the installation settings.")).toBeInTheDocument();
    });

    // Should preserve form data
    const updatedDataDirectoryInput = screen.getByLabelText(/Data Directory/);
    expect(updatedDataDirectoryInput).toHaveValue("/var/lib/embedded-cluster");

    // Should show Next: Validate Host button again
    expect(screen.getByText("Next: Validate Host")).toBeInTheDocument();
  });

  it("handles validation failures and allows retry", async () => {
    // Mock configuration success but validation failure
    server.use(
      http.post("*/api/install/installation/configure", () => {
        return HttpResponse.json({ success: true });
      }),
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      }),
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            fail: [
              { title: "Disk Space", message: "Not enough disk space available" },
            ],
          },
          status: { state: "Failed" },
        });
      }),
      http.post("*/api/install/host-preflights/run", () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: {
          config: {
            ...MOCK_INSTALL_CONFIG,
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
            storageClass: "standard",
          },
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for form to load
    await screen.findByText("Configure the installation settings.");
    await screen.findByTestId("linux-setup");

    // Fill form and navigate to validation view
    const dataDirectoryInput = screen.getByLabelText(/Data Directory/);
    fireEvent.change(dataDirectoryInput, {
      target: { value: "/var/lib/embedded-cluster" },
    });

    const nextButton = screen.getByText("Next: Validate Host");
    fireEvent.click(nextButton);

    // Wait for validation failure
    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // Should show retry button
    const retryButton = screen.getByRole("button", { name: "Run Validation Again" });
    expect(retryButton).toBeInTheDocument();

    // Should allow retry
    fireEvent.click(retryButton);
    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
    });

    // Next: Start Installation should be disabled during failures
    const startInstallButton = screen.queryByText("Next: Start Installation");
    if (startInstallButton) {
      expect(startInstallButton).toBeDisabled();
    }
  });
});
