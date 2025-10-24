import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../../test/setup.tsx";
import LinuxPreflightCheck from "../phases/LinuxPreflightCheck";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { setupServer } from "msw/node";
import { mockHandlers, preflightPresets } from "../../../../test/mockHandlers.ts";

const TEST_TOKEN = "test-auth-token";

const server = setupServer(
  mockHandlers.preflights.host.getStatus({
    status: { state: "Failed" },
    output: {
      pass: [{ title: "CPU Check", message: "CPU requirements met" }],
      warn: [{ title: "Memory Warning", message: "Memory is below recommended" }],
      fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
    },
    allowIgnoreHostPreflights: false,
  }),
  mockHandlers.preflights.host.run(true)
);

describe("LinuxPreflightCheck", () => {
  const mockOnComplete = vi.fn();
  const mockOnRun = vi.fn();

  beforeAll(() => server.listen());
  afterEach(() => {
    server.resetHandlers();
    mockOnComplete.mockClear();
    mockOnRun.mockClear();
    vi.clearAllMocks();
  });
  afterAll(() => server.close());

  it("shows validating state when preflights are polling", async () => {
    server.use(
      mockHandlers.preflights.host.getStatus({
        status: { state: "Running" },
        output: { fail: [], warn: [], pass: [] },
      })
    );

    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
      expect(screen.getByText("Please wait while we check your system.")).toBeInTheDocument();
    });
  });

  it("displays preflight results correctly", async () => {
    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      // passed preflights are not displayed
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Memory Warning")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });
  });

  it("shows success state when all preflights pass", async () => {
    server.use(
      mockHandlers.preflights.host.getStatus({
        status: { state: "Succeeded" },
        output: preflightPresets.success(),
      })
    );

    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host validation successful!")).toBeInTheDocument();
    });
    expect(mockOnComplete).toHaveBeenCalledWith(true, false); // success: true, allowIgnore: false (default)
  });

  it("allows re-running validation when there are failures", async () => {
    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // Clear the mock to isolate the button click action
    mockOnRun.mockClear();

    const runValidationButton = screen.getByRole("button", { name: "Run Validation Again" });
    expect(runValidationButton).toBeInTheDocument();
    fireEvent.click(runValidationButton);

    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
    });

    // Verify that onRun is called exactly once when preflight run succeeds
    await waitFor(() => {
      expect(mockOnRun).toHaveBeenCalledTimes(1);
    });
  });

  it("receives allowIgnoreHostPreflights field in preflight response", async () => {
    // Mock preflight status endpoint with allowIgnoreHostPreflights: true
    server.use(
      mockHandlers.preflights.host.getStatus({
        status: { state: "Failed" },
        output: preflightPresets.failed("Insufficient disk space"),
        allowIgnoreHostPreflights: true,
      })
    );

    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // The component should call onComplete with BOTH success status AND allowIgnoreHostPreflights flag
    expect(mockOnComplete).toHaveBeenCalledWith(false, true); // success: false, allowIgnore: true
  });

  it("passes allowIgnoreHostPreflights false to onComplete callback", async () => {
    // Mock preflight status endpoint with allowIgnoreHostPreflights: false
    server.use(
      mockHandlers.preflights.host.getStatus({
        status: { state: "Failed" },
        output: preflightPresets.failed("Insufficient disk space"),
        allowIgnoreHostPreflights: false,
      })
    );

    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // The component should call onComplete with success: false, allowIgnore: false
    expect(mockOnComplete).toHaveBeenCalledWith(false, false);
  });

  it("handles preflight run error gracefully", async () => {
    server.use(
      // Mock status endpoint to return Pending state (which triggers the mutation)
      mockHandlers.preflights.host.getStatus({
        status: { state: "Pending" },
        output: { fail: [], warn: [], pass: [] },
      }),
      // Mock run endpoint to return an error
      mockHandlers.preflights.host.run(false)
    );

    renderWithProviders(<LinuxPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        authToken: TEST_TOKEN,
      },
    });

    // Should display error message when preflight run fails
    await waitFor(() => {
      expect(screen.getByText("Failed to run preflights")).toBeInTheDocument();
    });

    // onRun should not be called when mutation fails
    expect(mockOnRun).not.toHaveBeenCalled();
  });
});
