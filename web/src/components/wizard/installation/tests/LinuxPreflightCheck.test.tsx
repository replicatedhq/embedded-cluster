import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../../test/setup.tsx";
import LinuxPreflightCheck from "../phases/LinuxPreflightCheck";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

const TEST_TOKEN = "test-auth-token";

const server = setupServer(
  // Mock preflight status endpoint
  http.get("*/api/linux/install/host-preflights/status", ({ request }) => {
    const authHeader = request.headers.get("Authorization");
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      return new HttpResponse(null, { status: 401 });
    }
    return HttpResponse.json({
      titles: ["Test"],
      output: {
        pass: [{ title: "CPU Check", message: "CPU requirements met" }],
        warn: [{ title: "Memory Warning", message: "Memory is below recommended" }],
        fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
      },
      status: { state: "Failed" },
      allowIgnoreHostPreflights: false,
    });
  }),

  // Mock preflight run endpoint
  http.post("*/api/linux/install/host-preflights/run", ({ request }) => {
    const authHeader = request.headers.get("Authorization");
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      return new HttpResponse(null, { status: 401 });
    }
    return HttpResponse.json({ success: true });
  })
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
      http.get("*/api/linux/install/host-preflights/status", ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({ state: "Running" });
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
      http.get("*/api/linux/install/host-preflights/status", ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({
          output: {
            pass: [{ title: "CPU Check", message: "CPU requirements met" }],
          },
          status: { state: "Succeeded" },
        });
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
      http.get("*/api/linux/install/host-preflights/status", ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({
          titles: ["Test"],
          output: {
            pass: [{ title: "CPU Check", message: "CPU requirements met" }],
            warn: [],
            fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
          },
          status: { state: "Failed" },
          allowIgnoreHostPreflights: true, // Test that this field is properly received
        });
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
      http.get("*/api/linux/install/host-preflights/status", ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({
          titles: ["Test"],
          output: {
            pass: [{ title: "CPU Check", message: "CPU requirements met" }],
            warn: [],
            fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
          },
          status: { state: "Failed" },
          allowIgnoreHostPreflights: false, // Test that this field is properly received
        });
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
});
