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
  http.get("*/api/linux/install/installation/config", () => {
    return HttpResponse.json({ config: MOCK_INSTALL_CONFIG });
  }),

  // Mock network interfaces endpoint
  http.get("*/api/console/available-network-interfaces", () => {
    return HttpResponse.json({ networkInterfaces: MOCK_NETWORK_INTERFACES });
  }),

  // Mock config submission endpoint
  http.post("*/api/linux/install/installation/configure", () => {
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

  it("renders the linux setup form when the install target is linux", async () => {
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
      http.post("*/api/linux/install/installation/configure", () => {
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
      http.get("*/api/linux/install/installation/config", ({ request }) => {
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
      http.post("*/api/linux/install/installation/configure", async ({ request }) => {
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

    // Wait for the mutation to complete and verify onNext was called
    await waitFor(
      () => {
        expect(mockOnNext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );
  });
});
