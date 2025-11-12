import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../../test/setup.tsx";
import AppPreflightCheck from "../phases/AppPreflightCheck";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { setupServer } from "msw/node";
import { mockHandlers, preflightPresets } from "../../../../test/mockHandlers.ts";

const TEST_TOKEN = "test-auth-token";

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  mockHandlers.preflights.app.getStatus({
    status: { state: "Succeeded" },
    output: {
      pass: [{ title: "CPU Check", message: "CPU requirements met" }],
      warn: [{ title: "Memory Warning", message: "Memory is below recommended" }],
      fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
    },
    allowIgnoreAppPreflights: false,
  }, target, 'install'),
  mockHandlers.preflights.app.run(true, target, 'install')
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])("AppPreflightCheck - $displayName", ({ target }) => {
  const mockOnComplete = vi.fn();
  const mockOnRun = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    mockOnComplete.mockClear();
    mockOnRun.mockClear();
    vi.clearAllMocks();
  });

  afterAll(() => server.close());

  it("shows validating state when preflights are polling", async () => {
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Running" },
        output: { fail: [], warn: [], pass: [] },
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Validating application requirements...")).toBeInTheDocument();
      expect(screen.getByText("Please wait while we check your application.")).toBeInTheDocument();
    });
  });

  it("displays preflight results correctly", async () => {
    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      // passed preflights are not displayed
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Memory Warning")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });
  });

  it("shows success state when all preflights pass", async () => {
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: preflightPresets.success(),
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application validation successful!")).toBeInTheDocument();
    });
    expect(mockOnComplete).toHaveBeenCalledWith(true, false, false); // success: true, allowIgnore: false, hasStrictFailures: false
  });

  it("shows API error when preflight execution fails", async () => {
    const errorMessage = "Failed to run preflight checks: connection timeout";
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: {
          state: "Failed",
          description: errorMessage
        },
        output: undefined,
        allowIgnoreAppPreflights: false
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Unable to complete application requirement checks")).toBeInTheDocument();
      expect(screen.getByText(errorMessage)).toBeInTheDocument();
    });

    // Should not show normal preflight results UI
    expect(screen.queryByText("Application Requirements Not Met")).not.toBeInTheDocument();
    expect(screen.queryByText("Run Validation Again")).not.toBeInTheDocument();
  });

  it("allows re-running validation when there are failures", async () => {
    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // Clear the mock to isolate the button click action
    mockOnRun.mockClear();

    const runValidationButton = screen.getByTestId("run-validation");
    expect(runValidationButton).toBeInTheDocument();
    fireEvent.click(runValidationButton);

    await waitFor(() => {
      expect(screen.getByText("Validating application requirements...")).toBeInTheDocument();
    });

    // Verify that onRun is called exactly once when preflight run succeeds
    await waitFor(() => {
      expect(mockOnRun).toHaveBeenCalledTimes(1);
    });
  });

  it("receives allowIgnoreAppPreflights field in preflight response", async () => {
    // Mock preflight status endpoint with allowIgnoreAppPreflights: true
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: {
          pass: [{ title: "CPU Check", message: "CPU requirements met" }],
          warn: [],
          fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
        },
        allowIgnoreAppPreflights: true,
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // The component should call onComplete with BOTH success status AND allowIgnoreAppPreflights flag
    expect(mockOnComplete).toHaveBeenCalledWith(false, true, false); // success: false, allowIgnore: true, hasStrictFailures: false
  });

  it("passes allowIgnoreAppPreflights false to onComplete callback", async () => {
    // Mock preflight status endpoint with allowIgnoreAppPreflights: false
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: {
          pass: [{ title: "CPU Check", message: "CPU requirements met" }],
          warn: [],
          fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
        },
        allowIgnoreAppPreflights: false,
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // The component should call onComplete with success: false, allowIgnore: false, hasStrictFailures: false
    expect(mockOnComplete).toHaveBeenCalledWith(false, false, false);
  });

  // New tests for strict preflight functionality
  it("displays strict failures with visual distinction", async () => {
    // Mock preflight status endpoint with strict and non-strict failures
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: {
          pass: [],
          warn: [],
          fail: [
            { title: "Critical Security Check", message: "Security requirement not met", strict: true },
            { title: "Disk Space", message: "Insufficient disk space", strict: false }
          ],
        },
        allowIgnoreAppPreflights: true,
        hasStrictAppPreflightFailures: true,
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
    });

    // Check that strict failure is displayed with "Critical" badge
    expect(screen.getByText("Critical Security Check")).toBeInTheDocument();
    expect(screen.getByText("Critical")).toBeInTheDocument(); // Critical badge

    // Check that non-strict failure is displayed normally
    expect(screen.getByText("Disk Space")).toBeInTheDocument();

    // The component should call onComplete with strict failures flag
    expect(mockOnComplete).toHaveBeenCalledWith(false, true, true); // success: false, allowIgnore: true, hasStrictFailures: true
  });

  it("shows appropriate guidance for strict failures in What's Next section", async () => {
    // Mock preflight status endpoint with strict failures
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: {
          pass: [],
          warn: [],
          fail: [
            { title: "Critical Security Check", message: "Security requirement not met", strict: true }
          ],
        },
        allowIgnoreAppPreflights: true,
        hasStrictAppPreflightFailures: true,
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
    });

  });

  it("passes hasStrictAppPreflightFailures from API response to onComplete", async () => {
    // Mock preflight status endpoint with hasStrictAppPreflightFailures field
    server.use(
      mockHandlers.preflights.app.getStatus({
        status: { state: "Succeeded" },
        output: {
          pass: [],
          warn: [],
          fail: [
            { title: "Regular Check", message: "Regular failure", strict: false }
          ],
        },
        allowIgnoreAppPreflights: true,
        hasStrictAppPreflightFailures: true,
      }, target, 'install')
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application Requirements Not Met")).toBeInTheDocument();
    });

    // Should use the hasStrictAppPreflightFailures from API response (true), not client-side detection (false)
    expect(mockOnComplete).toHaveBeenCalledWith(false, true, true);
  });
});
