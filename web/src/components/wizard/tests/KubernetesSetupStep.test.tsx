import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesSetupStep from "../setup/KubernetesSetupStep.tsx";
import { MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE, MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_WITH_ZEROS, MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_EMPTY } from "../../../test/testData.ts";

const createServer = (mode: 'install' | 'upgrade' = 'install') => setupServer(
  // Mock config endpoint for both install and upgrade
  http.get(`*/api/kubernetes/${mode}/installation/config`, () => {
    return HttpResponse.json(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE);
  }),

  // Mock config submission endpoint for both modes
  http.post(`*/api/kubernetes/${mode}/installation/configure`, () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock infrastructure setup endpoint for both modes
  http.post(`*/api/kubernetes/${mode}/infra/setup`, () => {
    return HttpResponse.json({ success: true });
  })
);

describe.each([
  { mode: "install" as const, modeDisplayName: "Install Mode" },
  { mode: "upgrade" as const, modeDisplayName: "Upgrade Mode" }
])("KubernetesSetupStep - $modeDisplayName", ({ mode }) => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();
  const mockUpdateConfig = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(mode);
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
    it("renders the kubernetes setup form with card, title, and next button", async () => {
      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Check if the component is rendered
      expect(screen.getByTestId("kubernetes-setup")).toBeInTheDocument();

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");

      // Check for title and description
      await screen.findByText("Set up the Kubernetes cluster for this installation.");

      // Check all input fields are present
      screen.getByTestId("admin-console-port-input");
      screen.getByTestId("http-proxy-input");
      screen.getByTestId("https-proxy-input");
      screen.getByTestId("no-proxy-input");

      // Check next button
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
      expect(nextButton).toBeInTheDocument();
    });
  });

  describe("Error Handling", () => {
    it("handles form errors gracefully", async () => {
      server.use(
        // Mock config submission endpoint to return an error
        http.post(`*/api/kubernetes/${mode}/installation/configure`, () => {
          return new HttpResponse(JSON.stringify({ message: "Invalid configuration" }), { status: 400 });
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Set up the Kubernetes cluster for this installation.");

      // Fill in required form values
      const adminPortInput = screen.getByTestId("admin-console-port-input");

      // Use fireEvent to simulate user input
      fireEvent.change(adminPortInput, { target: { value: "30000" } });

      // Submit form
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
      fireEvent.click(nextButton);

      // Verify error message is displayed (using partial match since component shows additional error details)
      await screen.findByText("Invalid configuration");

      // Verify onNext was not called
      expect(mockOnNext).not.toHaveBeenCalled();
      // Verify updateConfig was called only during initial load (once)
      expect(mockUpdateConfig).toHaveBeenCalledTimes(1);
    });

    it("handles field-specific errors gracefully", async () => {
      server.use(
        // Mock config submission endpoint to return field-specific errors
        http.post(`*/api/kubernetes/${mode}/installation/configure`, () => {
          return new HttpResponse(JSON.stringify({
            message: "Validation failed",
            errors: [
              { field: "adminConsolePort", message: "Admin Console Port must be between 1024 and 65535" },
              { field: "httpProxy", message: "HTTP Proxy must be a valid URL" }
            ]
          }), { status: 400 });
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Set up the Kubernetes cluster for this installation.");

      // Fill in required form values
      const adminPortInput = screen.getByTestId("admin-console-port-input");

      // Use fireEvent to simulate user input
      fireEvent.change(adminPortInput, { target: { value: "30000" } });

      // Submit form
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
      fireEvent.click(nextButton);

      // Verify generic error message is displayed for field errors
      await screen.findByText("Please fix the errors in the form above before proceeding.");

      // Verify field-specific error messages are displayed
      await screen.findByText("Admin Console Port must be between 1024 and 65535");
      await screen.findByText("HTTP Proxy must be a valid URL");

      // Verify onNext was not called
      expect(mockOnNext).not.toHaveBeenCalled();
      // Verify updateConfig was called only during initial load (once)
      expect(mockUpdateConfig).toHaveBeenCalledTimes(1);
    });

    it("clears errors when re-submitting after previous failure", async () => {
      // First, set up server to return an error
      server.use(
        http.post(`*/api/kubernetes/${mode}/installation/configure`, () => {
          return new HttpResponse(JSON.stringify({ message: "Initial error" }), { status: 400 });
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await screen.findByText("Set up the Kubernetes cluster for this installation.");

      // Fill in form values
      const adminPortInput = screen.getByTestId("admin-console-port-input");
      fireEvent.change(adminPortInput, { target: { value: "30000" } });

      // Submit form and verify error appears
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
      fireEvent.click(nextButton);
      await screen.findByText("Initial error");

      // Now change server to return success
      server.use(
        http.post(`*/api/kubernetes/${mode}/installation/configure`, () => {
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
        http.get(`*/api/kubernetes/${mode}/installation/config`, () => {
          return HttpResponse.json(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_WITH_ZEROS);
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Check that port input is empty (not displaying "0")
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
      
      expect(adminPortInput.value).toBe("");
    });

    it("handles empty config values correctly", async () => {
      server.use(
        http.get(`*/api/kubernetes/${mode}/installation/config`, () => {
          return HttpResponse.json(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE_EMPTY);
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Check that inputs show empty values appropriately
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
      const httpProxyInput = screen.getByTestId("http-proxy-input") as HTMLInputElement;
      
      expect(adminPortInput.value).toBe("");
      expect(httpProxyInput.value).toBe("");
    });

    it("only accepts integer values for port fields", async () => {
      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
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
      fireEvent.change(adminPortInput, { target: { value: "30000.5" } });
      expect(adminPortInput.value).toBe("");
      
      // Test that non-numeric values are rejected
      fireEvent.change(adminPortInput, { target: { value: "abc" } });
      expect(adminPortInput.value).toBe("");
      
      // Test that valid integer is accepted
      fireEvent.change(adminPortInput, { target: { value: "30000" } });
      expect(adminPortInput.value).toBe("30000");
    });

    it("accepts valid proxy URLs", async () => {
      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      const httpProxyInput = screen.getByTestId("http-proxy-input") as HTMLInputElement;
      const httpsProxyInput = screen.getByTestId("https-proxy-input") as HTMLInputElement;
      
      // Test that valid HTTP URLs are accepted
      fireEvent.change(httpProxyInput, { target: { value: "http://proxy.example.com:3128" } });
      expect(httpProxyInput.value).toBe("http://proxy.example.com:3128");
      
      // Test that valid HTTPS URLs are accepted
      fireEvent.change(httpsProxyInput, { target: { value: "https://proxy.example.com:3128" } });
      expect(httpsProxyInput.value).toBe("https://proxy.example.com:3128");
    });
  });

  describe("API Response Structure", () => {
    it("handles the new values/defaults API response structure correctly", async () => {
      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Verify that updateConfig was called with the resolved from the API response
      expect(mockUpdateConfig).toHaveBeenCalledWith(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE.resolved);

      // Check that form shows the correct values
      const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
      expect(adminPortInput.value).toBe("8800");
    });
  });

  describe("Form Submission", () => {
    it("submits the form successfully and calls updateConfig", async () => {
      // Mock all required API endpoints
      server.use(
        // Mock install config endpoint
        http.get("*/api/kubernetes/install/installation/config", ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          return HttpResponse.json(MOCK_KUBERNETES_INSTALL_CONFIG_RESPONSE);
        }),
        // Mock config submission endpoint
        http.post("*/api/kubernetes/install/installation/configure", async ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          const body = await request.json();
          // Verify the request body has required fields
          expect(body).toMatchObject({
            adminConsolePort: 30000,
          });
          return new HttpResponse(JSON.stringify({ success: true }), {
            status: 200,
            headers: {
              "Content-Type": "application/json",
            },
          });
        }),
        // Mock infrastructure setup endpoint
        http.post("*/api/kubernetes/install/infra/setup", ({ request }) => {
          // Verify auth header
          expect(request.headers.get("Authorization")).toBe("Bearer test-token");
          return HttpResponse.json({ success: true });
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Fill in all required form values
      const adminPortInput = screen.getByTestId("admin-console-port-input");

      // Use fireEvent to simulate user input
      fireEvent.change(adminPortInput, { target: { value: "30000" } });

      // Get the next button and ensure it's not disabled
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
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

      // Verify updateConfig was called with the correct values
      expect(mockUpdateConfig).toHaveBeenCalledWith({
        adminConsolePort: 30000,
      });
    });

    it("submits form with proxy configuration", async () => {
      // Mock all required API endpoints
      server.use(
        // Mock config submission endpoint
        http.post("*/api/kubernetes/install/installation/configure", async ({ request }) => {
          const body = await request.json();
          // Verify the request body includes proxy configuration
          expect(body).toMatchObject({
            adminConsolePort: 30000,
            httpProxy: "http://proxy.example.com:3128",
            httpsProxy: "https://proxy.example.com:3128",
            noProxy: "localhost,127.0.0.1,.internal",
          });
          return new HttpResponse(JSON.stringify({ success: true }), {
            status: 200,
            headers: {
              "Content-Type": "application/json",
            },
          });
        }),
        // Mock infrastructure setup endpoint
        http.post(`*/api/kubernetes/${mode}/infra/setup`, () => {
          return HttpResponse.json({ success: true });
        })
      );

      renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
        wrapperProps: {
          authenticated: true,
          target: "kubernetes",
          mode,
          contextValues: {
            kubernetesConfigContext: {
              config: {},
              updateConfig: mockUpdateConfig,
              resetConfig: vi.fn(),
            },
          },
        },
      });

      // Wait for loading to complete
      await screen.findByText("Loading configuration...");
      await waitFor(() => {
        expect(screen.queryByText("Loading configuration...")).not.toBeInTheDocument();
      });

      // Fill in all form values including proxy settings
      const adminPortInput = screen.getByTestId("admin-console-port-input");
      const httpProxyInput = screen.getByTestId("http-proxy-input");
      const httpsProxyInput = screen.getByTestId("https-proxy-input");
      const noProxyInput = screen.getByTestId("no-proxy-input");

      fireEvent.change(adminPortInput, { target: { value: "30000" } });
      fireEvent.change(httpProxyInput, { target: { value: "http://proxy.example.com:3128" } });
      fireEvent.change(httpsProxyInput, { target: { value: "https://proxy.example.com:3128" } });
      fireEvent.change(noProxyInput, { target: { value: "localhost,127.0.0.1,.internal" } });

      // Submit form
      const nextButton = screen.getByTestId("kubernetes-setup-submit-button");
      fireEvent.click(nextButton);

      // Wait for success
      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalled();
      });

      // Verify updateConfig was called with all proxy values
      expect(mockUpdateConfig).toHaveBeenCalledWith({
        adminConsolePort: 30000,
        httpProxy: "http://proxy.example.com:3128",
        httpsProxy: "https://proxy.example.com:3128",
        noProxy: "localhost,127.0.0.1,.internal",
      });
    });
  });
}); // end of mode describe