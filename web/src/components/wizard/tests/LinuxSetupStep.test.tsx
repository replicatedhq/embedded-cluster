import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import LinuxSetupStep from "../setup/LinuxSetupStep.tsx";
import { MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE, MOCK_LINUX_INSTALL_CONFIG_RESPONSE, MOCK_LINUX_INSTALL_CONFIG_RESPONSE_WITH_ZEROS, MOCK_LINUX_INSTALL_CONFIG_RESPONSE_EMPTY, MOCK_NETWORK_INTERFACES } from "../../../test/testData.ts";

const server = setupServer(
  // Mock install config endpoint
  http.get("*/api/linux/install/installation/config", () => {
    return HttpResponse.json(MOCK_LINUX_INSTALL_CONFIG_RESPONSE);
  }),

  // Mock network interfaces endpoint
  http.get("*/api/console/available-network-interfaces", () => {
    return HttpResponse.json(MOCK_NETWORK_INTERFACES);
  }),

  // Mock config submission endpoint
  http.post("*/api/linux/install/installation/configure", () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock installation status endpoint
  http.get("*/api/linux/install/installation/status", () => {
    return HttpResponse.json({
      state: "Succeeded",
      description: "Installation configured successfully",
      lastUpdated: new Date().toISOString()
    });
  }),

  // Mock preflight run endpoint
  http.post("*/api/linux/install/host-preflights/run", () => {
    return HttpResponse.json({ success: true });
  })
);

describe("LinuxSetupStep", () => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();

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

  describe("Component Rendering", () => {
    it("renders the linux setup form with card, title, and next button", async () => {
      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Check if the component is rendered
      expect(screen.getByTestId("linux-setup")).toBeInTheDocument();

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");

      // Check for title and description
      await screen.findByText("Configure the installation settings.");

      // Check all input fields are present
      await screen.findByTestId("data-directory-input");
      screen.getByTestId("admin-console-port-input");
      screen.getByTestId("local-artifact-mirror-port-input");

      // Check proxy configuration inputs
      screen.getByTestId("http-proxy-input");
      screen.getByTestId("https-proxy-input");
      screen.getByTestId("no-proxy-input");

      // Reveal advanced settings before checking advanced fields
      const advancedButton = screen.getByTestId("advanced-settings-toggle");
      fireEvent.click(advancedButton);

      // Check advanced settings
      screen.getByTestId("network-interface-select");
      screen.getByTestId("global-cidr-input");

      // Check next button
      const nextButton = screen.getByTestId("linux-setup-submit-button");
      expect(nextButton).toBeInTheDocument();
    });
  });

  describe("Error Handling", () => {
    it("handles form errors gracefully", async () => {
      server.use(
        http.get("*/api/console/available-network-interfaces", () => {
          return HttpResponse.json(MOCK_NETWORK_INTERFACES);
        }),
        // Mock config submission endpoint to return an error
        http.post("*/api/linux/install/installation/configure", () => {
          return new HttpResponse(JSON.stringify({ message: "Invalid configuration" }), { status: 400 });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Configure the installation settings.");

      // Fill in required form values
      const dataDirectoryInput = screen.getByTestId("data-directory-input");
      const adminPortInput = screen.getByTestId("admin-console-port-input");
      const mirrorPortInput = screen.getByTestId("local-artifact-mirror-port-input");

      // Use fireEvent to simulate user input
      fireEvent.change(dataDirectoryInput, {
        target: { value: "/var/lib/my-cluster" },
      });
      fireEvent.change(adminPortInput, { target: { value: "8080" } });
      fireEvent.change(mirrorPortInput, { target: { value: "8081" } });

      // Submit form
      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      // Verify error message is displayed
      await screen.findByText("Invalid configuration");

      // Verify onNext was not called
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("handles field-specific errors gracefully", async () => {
      server.use(
        http.get("*/api/console/available-network-interfaces", () => {
          return HttpResponse.json(MOCK_NETWORK_INTERFACES);
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

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Configure the installation settings.");

      // Fill in required form values
      const dataDirectoryInput = screen.getByTestId("data-directory-input");
      const adminPortInput = screen.getByTestId("admin-console-port-input");
      const mirrorPortInput = screen.getByTestId("local-artifact-mirror-port-input");

      // Use fireEvent to simulate user input
      fireEvent.change(dataDirectoryInput, {
        target: { value: "/var/lib/my-cluster" },
      });
      fireEvent.change(adminPortInput, { target: { value: "8080" } });
      fireEvent.change(mirrorPortInput, { target: { value: "8081" } });

      // Submit form
      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      // Verify generic error message is displayed for field errors
      await screen.findByText("Please fix the errors in the form above before proceeding.");

      // Verify field-specific error messages are displayed
      await screen.findByText("Data Directory is required");
      await screen.findByText("Admin Console Port must be between 1024 and 65535");

      // Verify onNext was not called
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("clears errors when re-submitting after previous failure", async () => {
      // First, set up server to return an error
      server.use(
        http.post("*/api/linux/install/installation/configure", () => {
          return new HttpResponse(JSON.stringify({ message: "Initial error" }), { status: 400 });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Configure the installation settings.");

      // Fill in form values
      const dataDirectoryInput = screen.getByTestId("data-directory-input");
      fireEvent.change(dataDirectoryInput, {
        target: { value: "/var/lib/my-cluster" },
      });

      // Submit form and verify error appears
      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);
      await screen.findByText("Initial error");

      // Now change server to return success
      server.use(
        http.post("*/api/linux/install/installation/configure", () => {
          return HttpResponse.json({ success: true });
        })
      );

      // Submit again
      fireEvent.click(nextButton);

      // Wait for success and verify error is cleared
      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalled();
      });

      // Error should no longer be displayed
      expect(screen.queryByText("Initial error")).not.toBeInTheDocument();
    });
  });

  describe("Input Validation and Edge Cases", () => {
    it("does not display zero values in port input fields", async () => {
      server.use(
        http.get("*/api/linux/install/installation/config", () => {
          return HttpResponse.json(MOCK_LINUX_INSTALL_CONFIG_RESPONSE_WITH_ZEROS);
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Check that port inputs are empty (not displaying "0")
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
      const mirrorPortInput = screen.getByTestId("local-artifact-mirror-port-input") as HTMLInputElement;

      expect(adminPortInput.value).toBe("");
      expect(mirrorPortInput.value).toBe("");
    });

    it("handles empty config values correctly", async () => {
      server.use(
        http.get("*/api/linux/install/installation/config", () => {
          return HttpResponse.json(MOCK_LINUX_INSTALL_CONFIG_RESPONSE_EMPTY);
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Check that inputs show empty values appropriately
      const dataDirectoryInput = screen.getByTestId("data-directory-input") as HTMLInputElement;
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;

      expect(dataDirectoryInput.value).toBe("");
      expect(adminPortInput.value).toBe("");
    });

    it("only accepts integer values for port fields", async () => {
      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;

      // Clear the existing value first
      fireEvent.change(adminPortInput, { target: { value: "" } });

      // Test that decimal values are rejected
      fireEvent.change(adminPortInput, { target: { value: "8080.5" } });
      expect(adminPortInput.value).toBe("");

      // Test that non-numeric values are rejected
      fireEvent.change(adminPortInput, { target: { value: "abc" } });
      expect(adminPortInput.value).toBe("");

      // Test that valid integer is accepted
      fireEvent.change(adminPortInput, { target: { value: "8080" } });
      expect(adminPortInput.value).toBe("8080");
    });

    it("handles advanced settings toggle", async () => {
      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Advanced settings should initially be hidden
      expect(screen.queryByTestId("network-interface-select")).not.toBeInTheDocument();
      expect(screen.queryByTestId("global-cidr-input")).not.toBeInTheDocument();

      // Click advanced settings button
      const advancedButton = screen.getByTestId("advanced-settings-toggle");
      fireEvent.click(advancedButton);

      // Advanced settings should now be visible
      expect(screen.getByTestId("network-interface-select")).toBeInTheDocument();
      expect(screen.getByTestId("global-cidr-input")).toBeInTheDocument();
    });
  });

  describe("API Response Structure", () => {
    it("handles the new values/defaults API response structure correctly", async () => {
      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Check that form shows the correct values
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
      expect(adminPortInput.value).toBe("8800");

      const dataDirectoryInput = screen.getByTestId("data-directory-input") as HTMLInputElement;
      expect(dataDirectoryInput.value).toBe("/custom/data/dir");
    });
  });

  describe("Form Submission", () => {
    it("submits the form successfully and calls updateConfig", async () => {
      // Mock all required API endpoints
      server.use(
        // Mock install config endpoint
        http.get("*/api/linux/install/installation/config", ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          return HttpResponse.json(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE);
        }),
        // Mock network interfaces endpoint
        http.get("*/api/console/available-network-interfaces", ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          return HttpResponse.json(MOCK_NETWORK_INTERFACES);
        }),
        // Mock config submission endpoint
        http.post("*/api/linux/install/installation/configure", async ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          const body = await request.json();
          // Verify the request body has all required fields
          expect(body).toMatchObject({
            adminConsolePort: 8080,
            localArtifactMirrorPort: 8081,
            dataDirectory: "/var/lib/embedded-cluster",
            networkInterface: "eth0",
            globalCidr: "10.244.0.0/16",
          });
          return new HttpResponse(JSON.stringify({ success: true }), {
            status: 200,
            headers: {
              "Content-Type": "application/json",
            },
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Configure the installation settings.");

      // Fill in all required form values
      const dataDirectoryInput = screen.getByTestId("data-directory-input");
      const adminPortInput = screen.getByTestId("admin-console-port-input");
      const mirrorPortInput = screen.getByTestId("local-artifact-mirror-port-input");

      // Reveal advanced settings before filling advanced fields
      const advancedButton = screen.getByTestId("advanced-settings-toggle");
      fireEvent.click(advancedButton);

      const networkInterfaceSelect = screen.getByTestId("network-interface-select");
      const globalCidrInput = screen.getByTestId("global-cidr-input");

      // Use fireEvent to simulate user input
      fireEvent.change(dataDirectoryInput, {
        target: { value: "/var/lib/embedded-cluster" },
      });
      fireEvent.change(adminPortInput, { target: { value: "8080" } });
      fireEvent.change(mirrorPortInput, { target: { value: "8081" } });
      fireEvent.change(networkInterfaceSelect, { target: { value: "eth0" } });
      fireEvent.change(globalCidrInput, { target: { value: "10.244.0.0/16" } });

      // Get the next button and ensure it's not disabled
      const nextButton = screen.getByTestId("linux-setup-submit-button");
      expect(nextButton).not.toBeDisabled();

      // Submit form
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );
    });
  });

  describe("Installation Status Polling", () => {
    it("shows 'Preparing the host.' loading state when installation status is polling", async () => {
      server.use(
        http.get("*/api/linux/install/installation/status", () => {
          return HttpResponse.json({
            state: "Running",
            description: "Configuring installation",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(screen.getByTestId("linux-setup-loading-text")).toHaveTextContent("Preparing the host.");
      });
    });

    it("triggers preflights after installation status succeeds", async () => {
      let statusCallCount = 0;
      server.use(
        http.get("*/api/linux/install/installation/status", () => {
          statusCallCount++;
          if (statusCallCount === 1) {
            return HttpResponse.json({
              state: "Running",
              description: "Configuring installation",
              lastUpdated: new Date().toISOString()
            });
          }
          return HttpResponse.json({
            state: "Succeeded",
            description: "Installation configured successfully",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalled();
      }, { timeout: 5000 });
    });

    it("handles installation status failure and shows error", async () => {
      server.use(
        http.get("*/api/linux/install/installation/status", () => {
          return HttpResponse.json({
            state: "Failed",
            description: "Network configuration failed",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        const errorElement = screen.getByTestId("linux-setup-error");
        expect(errorElement).toHaveTextContent("Installation configuration failed with: Network configuration failed");
      });

      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("stops polling installation status on failure", async () => {
      let statusCallCount = 0;
      server.use(
        http.get("*/api/linux/install/installation/status", () => {
          statusCallCount++;
          return HttpResponse.json({
            state: "Failed",
            description: "Configuration error",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(screen.getByTestId("linux-setup-error")).toBeInTheDocument();
      });

      const initialCallCount = statusCallCount;
      await new Promise(resolve => setTimeout(resolve, 2000));

      expect(statusCallCount).toBe(initialCallCount);
    });

    it("does not trigger preflights if installation status is still running", async () => {
      server.use(
        http.get("*/api/linux/install/installation/status", () => {
          return HttpResponse.json({
            state: "Running",
            description: "Still configuring",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(screen.getByTestId("linux-setup-loading-text")).toHaveTextContent("Preparing the host.");
      });

      await new Promise(resolve => setTimeout(resolve, 2000));

      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("allows retry after installation status failure", async () => {
      let submitCount = 0;
      server.use(
        http.post("*/api/linux/install/installation/configure", () => {
          submitCount++;
          return HttpResponse.json({ success: true });
        }),
        http.get("*/api/linux/install/installation/status", () => {
          if (submitCount === 1) {
            return HttpResponse.json({
              state: "Failed",
              description: "First attempt failed",
              lastUpdated: new Date().toISOString()
            });
          }
          return HttpResponse.json({
            state: "Succeeded",
            description: "Installation configured successfully",
            lastUpdated: new Date().toISOString()
          });
        })
      );

      renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
        },
      });

      await screen.findByText("Configure the installation settings.");

      const nextButton = screen.getByTestId("linux-setup-submit-button");

      fireEvent.click(nextButton);

      await waitFor(() => {
        const errorElement = screen.getByTestId("linux-setup-error");
        expect(errorElement).toHaveTextContent("Installation configuration failed with: First attempt failed");
      });

      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalled();
      }, { timeout: 5000 });
    });
  });
});
