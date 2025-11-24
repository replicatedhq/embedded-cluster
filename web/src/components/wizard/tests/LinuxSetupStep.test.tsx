import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import { MOCK_LINUX_INSTALL_CONFIG_RESPONSE, MOCK_NETWORK_INTERFACES } from "../../../test/testData.ts";
import { mockHandlers } from "../../../test/mockHandlers.ts";
import LinuxSetupStep, {
  processInputValue,
  extractFieldError,
  determineLoadingText,
  shouldExpandAdvancedSettings,
  evaluateInstallationStatus,
  determineLoadingState,
  fieldNames,
} from "../setup/LinuxSetupStep.tsx";

const server = setupServer(
  mockHandlers.installation.getConfig(MOCK_LINUX_INSTALL_CONFIG_RESPONSE),
  mockHandlers.console.getNetworkInterfaces(MOCK_NETWORK_INTERFACES.networkInterfaces.map(name => ({ name, addresses: [] }))),
  mockHandlers.installation.configure(true),
  mockHandlers.installation.getStatus({ state: 'Succeeded', description: 'Installation configured successfully' }),
  mockHandlers.preflights.host.run(true)
);

describe("LinuxSetupStep - Unit", () => {
  describe("processInputValue", () => {
    it("converts valid integer string to number for localArtifactMirrorPort", () => {
      const result = processInputValue(
        "localArtifactMirrorPort",
        "8080",
        { dataDirectory: "/var" }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        localArtifactMirrorPort: 8080,
      });
    });

    it("converts empty string to undefined for port fields", () => {
      const result = processInputValue(
        "localArtifactMirrorPort",
        "",
        { dataDirectory: "/var", localArtifactMirrorPort: 8080 }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        localArtifactMirrorPort: undefined,
      });
    });

    it("rejects decimal values for port fields", () => {
      const currentValues = { dataDirectory: "/var", localArtifactMirrorPort: 8080 };
      const result = processInputValue(
        "localArtifactMirrorPort",
        "8080.5",
        currentValues
      );

      expect(result).toBe(currentValues); // Reference equality - unchanged
    });

    it("rejects non-numeric values for port fields", () => {
      const currentValues = { localArtifactMirrorPort: 8080 };
      const result = processInputValue(
        "localArtifactMirrorPort",
        "abc",
        currentValues
      );

      expect(result).toBe(currentValues); // Unchanged
    });

    it("handles localArtifactMirrorPort as a port field", () => {
      const result = processInputValue(
        "localArtifactMirrorPort",
        "9090",
        {}
      );

      expect(result).toEqual({
        localArtifactMirrorPort: 9090,
      });
    });

    it("stores string values for non-port fields", () => {
      const result = processInputValue(
        "dataDirectory",
        "/opt/data",
        {}
      );

      expect(result).toEqual({
        dataDirectory: "/opt/data",
      });
    });

    it("handles proxy field as string", () => {
      const result = processInputValue(
        "httpProxy",
        "http://proxy:8080",
        { dataDirectory: "/var" }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        httpProxy: "http://proxy:8080",
      });
    });
  });

  describe("extractFieldError", () => {
    it("returns formatted error for matching field", () => {
      const errors = [
        { field: "localArtifactMirrorPort", message: "localArtifactMirrorPort must be between 1024 and 65535" },
      ];

      const result = extractFieldError("localArtifactMirrorPort", errors, fieldNames);

      expect(result).toBe("Admin Console Port must be between 1024 and 65535");
    });

    it("returns undefined when field has no error", () => {
      const errors = [
        { field: "localArtifactMirrorPort", message: "some error" },
      ];

      const result = extractFieldError("dataDirectory", errors, fieldNames);

      expect(result).toBeUndefined();
    });

    it("returns undefined when fieldErrors is undefined", () => {
      const result = extractFieldError("dataDirectory", undefined, fieldNames);

      expect(result).toBeUndefined();
    });

    it("handles empty error array", () => {
      const result = extractFieldError("dataDirectory", [], fieldNames);

      expect(result).toBeUndefined();
    });
  });

  describe("determineLoadingText", () => {
    it("shows installation message when polling", () => {
      const result = determineLoadingText(true);
      expect(result).toBe("Preparing the host.");
    });

    it("shows configuration message when not polling", () => {
      const result = determineLoadingText(false);
      expect(result).toBe("Loading configuration...");
    });
  });

  describe("shouldExpandAdvancedSettings", () => {
    it("expands when networkInterface has error", () => {
      const errors = [{ field: "networkInterface", message: "required" }];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(true);
    });

    it("expands when globalCidr has error", () => {
      const errors = [{ field: "globalCidr", message: "invalid CIDR" }];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(true);
    });

    it("does not expand for non-advanced field errors", () => {
      const errors = [
        { field: "dataDirectory", message: "required" },
        { field: "localArtifactMirrorPort", message: "invalid" },
      ];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(false);
    });

    it("handles undefined errors", () => {
      const result = shouldExpandAdvancedSettings(undefined);
      expect(result).toBe(false);
    });

    it("handles empty error array", () => {
      const result = shouldExpandAdvancedSettings([]);
      expect(result).toBe(false);
    });
  });

  describe("evaluateInstallationStatus", () => {
    it("continues polling when status is undefined", () => {
      const result = evaluateInstallationStatus(undefined);

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });

    it("succeeds and proceeds to next step", () => {
      const result = evaluateInstallationStatus({
        state: "Succeeded",
        description: "Installation configured successfully",
        lastUpdated: "",
      });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: true,
        errorMessage: null,
      });
    });

    it("fails with description", () => {
      const result = evaluateInstallationStatus({
        state: "Failed",
        description: "Network configuration error",
        lastUpdated: "",
      });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: false,
        errorMessage: "Installation configuration failed with: Network configuration error",
      });
    });

    it("fails without description", () => {
      const result = evaluateInstallationStatus({
        state: "Failed",
        description: "",
        lastUpdated: "",
      });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: false,
        errorMessage: "Installation configuration failed",
      });
    });

    it("continues polling for Running state", () => {
      const result = evaluateInstallationStatus({
        state: "Running",
        description: "Processing",
        lastUpdated: "",
      });

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });

    it("continues polling for unknown states", () => {
      const result = evaluateInstallationStatus({
        state: "Pending",
        description: "",
        lastUpdated: "",
      });

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });
  });

  describe("determineLoadingState", () => {
    it("returns true when config is loading", () => {
      const result = determineLoadingState(true, false, false);
      expect(result).toBe(true);
    });

    it("returns true when interfaces are loading", () => {
      const result = determineLoadingState(false, true, false);
      expect(result).toBe(true);
    });

    it("returns true when polling installation", () => {
      const result = determineLoadingState(false, false, true);
      expect(result).toBe(true);
    });

    it("returns true when multiple sources loading", () => {
      const result = determineLoadingState(true, true, true);
      expect(result).toBe(true);
    });

    it("returns false when nothing is loading", () => {
      const result = determineLoadingState(false, false, false);
      expect(result).toBe(false);
    });
  });
});


describe("LinuxSetupStep - Integration", () => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();

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

  it("SUCCESS PATH: Complete flow from form submission to successful installation", async () => {
    // Setup successful submission and polling handlers with auth verification
    const statusCounter = { callCount: 0 };
    server.use(
      mockHandlers.installation.configure({
        captureRequest: (body: Record<string, unknown>, headers: Headers) => {
          // Verify authentication header is present
          expect(headers.get("Authorization")).toBe("Bearer test-token");
          // Verify request body structure
          expect(body).toHaveProperty("dataDirectory");
          expect(body).toHaveProperty("localArtifactMirrorPort");
        }
      }),
      mockHandlers.installation.getStatus({
        state: "Running",
        sequence: [
          { state: "Running", description: "Configuring installation" },
          { state: "Running", description: "Setting up network" },
          { state: "Succeeded", description: "Installation configured successfully" }
        ],
        counter: statusCounter
      })
    );

    // Render component
    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
      wrapperProps: { authenticated: true },
    });

    // Wait for initial load
    await waitFor(() => {
      expect(screen.queryByTestId("linux-setup-loading-text")).not.toBeInTheDocument();
    });

    // Verify all form fields are rendered (component smoke test)
    expect(screen.getByTestId("linux-setup")).toBeInTheDocument();

    // Check all basic input fields are present
    const dataDirectoryInput = screen.getByTestId("data-directory-input") as HTMLInputElement;
    const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
    const localArtifactMirrorPortInput = screen.getByTestId("local-artifact-mirror-port-input") as HTMLInputElement;

    // Check proxy configuration inputs are present
    expect(screen.getByTestId("http-proxy-input")).toBeInTheDocument();
    expect(screen.getByTestId("https-proxy-input")).toBeInTheDocument();
    expect(screen.getByTestId("no-proxy-input")).toBeInTheDocument();

    // Check buttons are present
    expect(screen.getByTestId("linux-setup-submit-button")).toBeInTheDocument();
    expect(screen.getByTestId("linux-setup-button-back")).toBeInTheDocument();

    // Verify API configuration loaded correctly
    expect(dataDirectoryInput.value).toBe("/custom/data/dir");
    expect(adminPortInput.value).toBe("8800");
    expect(localArtifactMirrorPortInput.value).toBe("8801");

    // Test advanced settings toggle
    expect(screen.queryByTestId("network-interface-select")).not.toBeInTheDocument();
    expect(screen.queryByTestId("global-cidr-input")).not.toBeInTheDocument();

    const advancedButton = screen.getByTestId("advanced-settings-toggle");
    fireEvent.click(advancedButton);

    // Advanced settings should now be visible
    expect(screen.getByTestId("network-interface-select")).toBeInTheDocument();
    expect(screen.getByTestId("global-cidr-input")).toBeInTheDocument();

    // Fill in the form
    fireEvent.change(dataDirectoryInput, { target: { value: "/opt/my-app" } });
    fireEvent.change(adminPortInput, { target: { value: "8080" } });

    // Test hop: processInputValue - verify port conversion
    expect(adminPortInput.value).toBe("8080");

    // Try invalid port (decimal) - should be rejected by hop
    fireEvent.change(adminPortInput, { target: { value: "8080.5" } });
    expect(adminPortInput.value).toBe("8080"); // Should remain unchanged

    // Submit the form
    const submitButton = screen.getByTestId("linux-setup-submit-button");
    fireEvent.click(submitButton);

    // Verify loading state changes (hop: determineLoadingText)
    await screen.findByText("Preparing the host.");

    // Wait for polling to complete and succeed (hop: evaluateInstallationStatus)
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    }, { timeout: 5000 });

    // Verify no errors displayed
    expect(screen.queryByTestId("linux-setup-error")).not.toBeInTheDocument();
  });

  it("FAILURE PATH: Form validation errors followed by installation failure", async () => {
    // First setup handler for validation errors
    server.use(
      mockHandlers.installation.configure({
        error: {
          message: "Validation failed",
          fields: [
            { field: "networkInterface", message: "networkInterface is required" },
            { field: "localArtifactMirrorPort", message: "localArtifactMirrorPort must be between 1024 and 65535" }
          ]
        }
      })
    );

    // Render component
    renderWithProviders(<LinuxSetupStep onNext={mockOnNext} onBack={mockOnBack} />, {
      wrapperProps: { authenticated: true },
    });

    // Wait for initial load
    await waitFor(() => {
      expect(screen.queryByTestId("linux-setup-loading-text")).not.toBeInTheDocument();
    });

    // Verify advanced settings are initially collapsed
    expect(screen.queryByTestId("network-interface-select")).not.toBeInTheDocument();

    // Fill form with invalid values
    const adminPortInput = screen.getByTestId("admin-console-port-input") as HTMLInputElement;
    fireEvent.change(adminPortInput, { target: { value: "80" } });

    // Submit and get validation errors
    const submitButton = screen.getByTestId("linux-setup-submit-button");
    fireEvent.click(submitButton);

    // Wait for field errors to appear
    await screen.findByText("Please fix the errors in the form above before proceeding.");

    // Verify hop: extractFieldError - field error messages formatted correctly
    await screen.findByText("Admin Console Port must be between 1024 and 65535");

    // Verify hop: shouldExpandAdvancedSettings - auto-expanded due to networkInterface error
    await waitFor(() => {
      expect(screen.getByTestId("network-interface-select")).toBeInTheDocument();
    });

    // Fix the errors
    fireEvent.change(adminPortInput, { target: { value: "8080" } });

    const networkSelect = screen.getByTestId("network-interface-select") as HTMLSelectElement;
    fireEvent.change(networkSelect, { target: { value: "eth0" } });

    // Now setup successful submission but with installation failure
    server.use(
      mockHandlers.installation.configure(true),
      mockHandlers.installation.getStatus({
        state: "Failed",
        description: "Network configuration failed: unable to bind to port"
      })
    );

    // Submit again
    fireEvent.click(submitButton);

    // Verify loading state (hop: determineLoadingText during polling)
    await screen.findByText("Preparing the host.");

    // Wait for installation to fail (hop: evaluateInstallationStatus with Failed state)
    await screen.findByText("Installation configuration failed with: Network configuration failed: unable to bind to port");

    // Verify we didn't proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();

    // Verify advanced settings remain expanded after error
    expect(screen.getByTestId("network-interface-select")).toBeInTheDocument();
  });
});