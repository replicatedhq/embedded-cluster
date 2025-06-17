import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import SetupStep from "../SetupStep.tsx";
import { MOCK_INSTALL_CONFIG, MOCK_NETWORK_INTERFACES, MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

// Mock ValidationStep component for isolated SetupStep testing
vi.mock("../ValidationStep.tsx", () => ({
  default: ({ onComplete, onBack }: { onComplete: (success: boolean) => void; onBack: () => void }) => (
    <div data-testid="validation-step-mock">
      <h2>Validation Step</h2>
      <button onClick={() => onComplete(true)}>Complete Successfully</button>
      <button onClick={() => onComplete(false)}>Complete with Failure</button>
      <button onClick={onBack}>Back to Configuration</button>
    </div>
  ),
}));

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

    // Ensure ValidationStep is not shown initially
    expect(screen.queryByTestId("validation-step-mock")).not.toBeInTheDocument();
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

    // Verify ValidationStep is not shown on error
    expect(screen.queryByTestId("validation-step-mock")).not.toBeInTheDocument();
  });

  it("shows ValidationStep when Next: Validate Host is clicked", async () => {
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

    // Should show ValidationStep instead of configuration
    await waitFor(() => {
      expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    });

    // Should show ValidationStep content
    expect(screen.getByText("Validation Step")).toBeInTheDocument();
    expect(screen.getByText("Back to Configuration")).toBeInTheDocument();

    // Configuration form should not be visible
    expect(screen.queryByTestId("linux-setup")).not.toBeInTheDocument();
  });

  it("calls onNext when ValidationStep completes successfully", async () => {
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

    // Wait for ValidationStep to appear
    await waitFor(() => {
      expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    });

    // Click Complete Successfully button
    const completeButton = screen.getByText("Complete Successfully");
    fireEvent.click(completeButton);

    // Should call onNext when validation completes successfully
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalled();
    });
  });

  it("stays in ValidationStep when validation fails", async () => {
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

    // Wait for ValidationStep to appear
    await waitFor(() => {
      expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    });

    // Click Complete with Failure button
    const failButton = screen.getByText("Complete with Failure");
    fireEvent.click(failButton);

    // Should stay in ValidationStep and not call onNext
    expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("returns to configuration view when ValidationStep calls onBack", async () => {
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

    // Wait for ValidationStep to appear
    await waitFor(() => {
      expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    });

    // Click Back to Configuration
    const backButton = screen.getByText("Back to Configuration");
    fireEvent.click(backButton);

    // Should return to configuration view
    await waitFor(() => {
      expect(screen.getByText("Configure the installation settings.")).toBeInTheDocument();
      expect(screen.getByTestId("linux-setup")).toBeInTheDocument();
    });

    // Should preserve form data
    const updatedDataDirectoryInput = screen.getByLabelText(/Data Directory/);
    expect(updatedDataDirectoryInput).toHaveValue("/var/lib/embedded-cluster");

    // Should show Next: Validate Host button again
    expect(screen.getByText("Next: Validate Host")).toBeInTheDocument();

    // ValidationStep should not be visible
    expect(screen.queryByTestId("validation-step-mock")).not.toBeInTheDocument();
  });

  it("submits the form successfully and preserves configuration data", async () => {
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

    // Wait for transition to ValidationStep
    await waitFor(() => {
      expect(screen.getByTestId("validation-step-mock")).toBeInTheDocument();
    });

    // Verify we're now in validation view with proper content
    expect(screen.getByText("Validation Step")).toBeInTheDocument();
    expect(screen.getByText("Back to Configuration")).toBeInTheDocument();
  });
});
