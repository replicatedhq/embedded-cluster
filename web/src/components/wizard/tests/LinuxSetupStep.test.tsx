import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/renderWithProviders.tsx";
import LinuxSetupStep from "../setup/LinuxSetupStep.tsx";
import { formatErrorMessage } from "../setup/LinuxSetupStep.tsx";
import { MOCK_LINUX_INSTALL_CONFIG, MOCK_NETWORK_INTERFACES } from "../../../test/testData.ts";

const server = setupServer(
  // Mock install config endpoint
  http.get("*/api/linux/install/installation/config", () => {
    return HttpResponse.json({ config: MOCK_LINUX_INSTALL_CONFIG });
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

describe("LinuxSetupStep", () => {
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

  it("renders the linux setup form with card, title, and next button", async () => {
    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        contextValues: {
          linuxConfigContext: {
            config: {
              dataDirectory: "/var/lib/embedded-cluster",
              adminConsolePort: 8080,
              localArtifactMirrorPort: 8081,
              networkInterface: "eth0",
              globalCidr: "10.244.0.0/16",
              useProxy: false,
            },
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Check if the component is rendered
    expect(screen.getByTestId("linux-setup")).toBeInTheDocument();

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");

    // Check for title and description
    await screen.findByText("Configure the installation settings.");

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
    screen.getByLabelText(/Reserved Network Range/, { selector: "input" });

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

    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        contextValues: {
          linuxConfigContext: {
            config: {
              dataDirectory: "/var/lib/embedded-cluster",
              adminConsolePort: 8080,
              localArtifactMirrorPort: 8081,
              networkInterface: "eth0",
              globalCidr: "10.244.0.0/16",
              useProxy: false,
            },
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Configure the installation settings.");

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
    await screen.findByText("Invalid configuration");

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("handles field-specific errors gracefully", async () => {
    server.use(
      http.get("*/api/console/available-network-interfaces", () => {
        return HttpResponse.json({
          networkInterfaces: MOCK_NETWORK_INTERFACES,
        });
      }),
      // Mock config submission endpoint to return field-specific errors
      http.post("*/api/linux/install/installation/configure", () => {
        return new HttpResponse(JSON.stringify({ 
          message: "Validation failed",
          errors: [
            { field: "dataDirectory", message: "Data Directory is required" },
            { field: "adminConsolePort", message: "Admin Console Port must be between 1024 and 65535" }
          ]
        }), { status: 400 });
      })
    );

    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        contextValues: {
          linuxConfigContext: {
            config: {
              dataDirectory: "",
              adminConsolePort: 0,
              localArtifactMirrorPort: 8081,
              networkInterface: "eth0",
              globalCidr: "10.244.0.0/16",
              useProxy: false,
            },
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Configure the installation settings.");

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

    // Verify generic error message is displayed for field errors
    await screen.findByText("Please fix the errors in the form above before proceeding.");

    // Verify field-specific error messages are displayed
    await screen.findByText("Data Directory is required");
    await screen.findByText("Admin Console Port must be between 1024 and 65535");

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
            dataDirectory: "/var/lib/embedded-cluster",
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
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

    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        contextValues: {
          linuxConfigContext: {
            config: {
              dataDirectory: "/var/lib/embedded-cluster",
              adminConsolePort: 8080,
              localArtifactMirrorPort: 8081,
              networkInterface: "eth0",
              globalCidr: "10.244.0.0/16",
              useProxy: false,
            },
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Configure the installation settings.");

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

describe("formatErrorMessage", () => {
   it("handles empty string", () => {
      expect(formatErrorMessage("")).toBe("");
   });

   it("replaces field names with their proper format", () => {
      expect(formatErrorMessage("adminConsolePort")).toBe("Admin Console Port");
      expect(formatErrorMessage("dataDirectory")).toBe("Data Directory");
      expect(formatErrorMessage("localArtifactMirrorPort")).toBe("Local Artifact Mirror Port");
      expect(formatErrorMessage("httpProxy")).toBe("HTTP Proxy");
      expect(formatErrorMessage("httpsProxy")).toBe("HTTPS Proxy");
      expect(formatErrorMessage("noProxy")).toBe("Proxy Bypass List");
      expect(formatErrorMessage("networkInterface")).toBe("Network Interface");
      expect(formatErrorMessage("podCidr")).toBe("Pod CIDR");
      expect(formatErrorMessage("serviceCidr")).toBe("Service CIDR");
      expect(formatErrorMessage("globalCidr")).toBe("Reserved Network Range (CIDR)");
      expect(formatErrorMessage("cidr")).toBe("CIDR");
   });

   it("handles multiple field names in one message", () => {
      expect(formatErrorMessage("podCidr and serviceCidr are required")).toBe("Pod CIDR and Service CIDR are required");
      expect(formatErrorMessage("httpProxy and httpsProxy must be set")).toBe("HTTP Proxy and HTTPS Proxy must be set");
   });

   it("preserves non-field words", () => {
      expect(formatErrorMessage("The podCidr is invalid")).toBe("The Pod CIDR is invalid");
      expect(formatErrorMessage("Please set the httpProxy")).toBe("Please set the HTTP Proxy");
   });

   it("handles case insensitivity correctly", () => {
      expect(formatErrorMessage("PodCidr")).toBe("Pod CIDR");
      expect(formatErrorMessage("HTTPPROXY")).toBe("HTTP Proxy");
      expect(formatErrorMessage("cidr")).toBe("CIDR");
      expect(formatErrorMessage("Cidr")).toBe("CIDR");
      expect(formatErrorMessage("CIDR")).toBe("CIDR");
   });

   it("handles real-world error messages", () => {
      expect(formatErrorMessage("The podCidr 10.0.0.0/24 overlaps with serviceCidr 10.0.0.0/16")).toBe(
         "The Pod CIDR 10.0.0.0/24 overlaps with Service CIDR 10.0.0.0/16"
      );
      expect(formatErrorMessage("httpProxy and httpsProxy cannot be empty when noProxy is set")).toBe(
         "HTTP Proxy and HTTPS Proxy cannot be empty when Proxy Bypass List is set"
      );
      expect(formatErrorMessage("adminConsolePort must be between 1024 and 65535")).toBe(
         "Admin Console Port must be between 1024 and 65535"
      );
      expect(formatErrorMessage("dataDirectory /var/lib/k0s is not writable")).toBe(
         "Data Directory /var/lib/k0s is not writable"
      );
      expect(formatErrorMessage("globalCidr must be a valid CIDR block")).toBe(
         "Reserved Network Range (CIDR) must be a valid CIDR block"
      );
   });

   it("handles special characters and formatting", () => {
      expect(formatErrorMessage("admin_console_port and localArtifactMirrorPort cannot be equal.")).toBe(
         "admin_console_port and Local Artifact Mirror Port cannot be equal."
      );
      expect(formatErrorMessage("httpProxy: invalid URL format")).toBe("HTTP Proxy: invalid URL format");
      expect(formatErrorMessage("podCidr: 192.168.0.0/24 (invalid)")).toBe("Pod CIDR: 192.168.0.0/24 (invalid)");
   });
});
