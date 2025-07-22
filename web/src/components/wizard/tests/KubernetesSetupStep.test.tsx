import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesSetupStep from "../setup/KubernetesSetupStep.tsx";

const MOCK_KUBERNETES_CONFIG = {
  adminConsolePort: 30000,
  useProxy: false,
  httpProxy: "",
  httpsProxy: "",
  noProxy: "",
};

const server = setupServer(
  // Mock install config endpoint
  http.get("*/api/kubernetes/install/installation/config", () => {
    return HttpResponse.json({ config: MOCK_KUBERNETES_CONFIG });
  }),

  // Mock config submission endpoint
  http.post("*/api/kubernetes/install/installation/configure", () => {
    return HttpResponse.json({ success: true });
  }),

  // Mock infrastructure setup endpoint
  http.post("*/api/kubernetes/install/infra/setup", () => {
    return HttpResponse.json({ success: true });
  })
);

describe("KubernetesSetupStep", () => {
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

  it("renders the kubernetes setup form with card, title, and next button", async () => {
    renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        contextValues: {
          kubernetesConfigContext: {
            config: MOCK_KUBERNETES_CONFIG,
            updateConfig: vi.fn(),
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
    screen.getByLabelText(/Admin Console Port/, { selector: "input" });
    screen.getByLabelText(/HTTP Proxy/, { selector: "input" });
    screen.getByLabelText(/HTTPS Proxy/, { selector: "input" });
    screen.getByLabelText(/Proxy Bypass List/, { selector: "input" });

    // Check next button
    const nextButton = screen.getByText("Next: Start Installation");
    expect(nextButton).toBeInTheDocument();
  });

  it("handles form errors gracefully", async () => {
    server.use(
      // Mock config submission endpoint to return an error
      http.post("*/api/kubernetes/install/installation/configure", () => {
        return new HttpResponse(JSON.stringify({ message: "Invalid configuration" }), { status: 400 });
      })
    );

    renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        contextValues: {
          kubernetesConfigContext: {
            config: MOCK_KUBERNETES_CONFIG,
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Set up the Kubernetes cluster for this installation.");

    // Fill in required form values
    const adminPortInput = screen.getByLabelText(/Admin Console Port/);

    // Use fireEvent to simulate user input
    fireEvent.change(adminPortInput, { target: { value: "30000" } });

    // Submit form
    const nextButton = screen.getByText("Next: Start Installation");
    fireEvent.click(nextButton);

    // Verify error message is displayed (using partial match since component shows additional error details)
    await screen.findByText("Invalid configuration");

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("handles field-specific errors gracefully", async () => {
    server.use(
      // Mock config submission endpoint to return field-specific errors
      http.post("*/api/kubernetes/install/installation/configure", () => {
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
        contextValues: {
          kubernetesConfigContext: {
            config: MOCK_KUBERNETES_CONFIG,
            updateConfig: vi.fn(),
            resetConfig: vi.fn(),
          },
        },
      },
    });

    // Wait for loading to complete
    await screen.findByText("Loading configuration...");
    await screen.findByText("Set up the Kubernetes cluster for this installation.");

    // Fill in required form values
    const adminPortInput = screen.getByLabelText(/Admin Console Port/);

    // Use fireEvent to simulate user input
    fireEvent.change(adminPortInput, { target: { value: "30000" } });

    // Submit form
    const nextButton = screen.getByText("Next: Start Installation");
    fireEvent.click(nextButton);

    // Verify generic error message is displayed for field errors
    await screen.findByText("Please fix the errors in the form above before proceeding.");

    // Verify field-specific error messages are displayed
    await screen.findByText("Admin Console Port must be between 1024 and 65535");
    await screen.findByText("HTTP Proxy must be a valid URL");

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("submits the form successfully", async () => {    
    // Mock all required API endpoints
    server.use(
      // Mock install config endpoint
      http.get("*/api/kubernetes/install/installation/config", ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({
          config: MOCK_KUBERNETES_CONFIG,
        });
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
      })
    );

    renderWithProviders(<KubernetesSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        contextValues: {
          kubernetesConfigContext: {
            config: MOCK_KUBERNETES_CONFIG,
            updateConfig: vi.fn(),
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
    const adminPortInput = screen.getByLabelText(/Admin Console Port/);

    // Use fireEvent to simulate user input
    fireEvent.change(adminPortInput, { target: { value: "30000" } });

    // Get the next button and ensure it's not disabled
    const nextButton = screen.getByText("Next: Start Installation");
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
