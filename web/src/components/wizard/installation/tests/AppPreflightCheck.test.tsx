import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../../test/setup.tsx";
import AppPreflightCheck from "../phases/AppPreflightCheck";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

const TEST_TOKEN = "test-auth-token";

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  // Mock preflight status endpoint
  http.get(`*/api/${target}/install/app-preflights/status`, ({ request }) => {
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
      allowIgnoreAppPreflights: false,
    });
  }),

  // Mock preflight run endpoint
  http.post(`*/api/${target}/install/app-preflights/run`, ({ request }) => {
    const authHeader = request.headers.get("Authorization");
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      return new HttpResponse(null, { status: 401 });
    }
    return HttpResponse.json({ success: true });
  })
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
      http.get(`*/api/${target}/install/app-preflights/status`, ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({ state: "Running" });
      })
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
      http.get(`*/api/${target}/install/app-preflights/status`, ({ request }) => {
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

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Application validation successful!")).toBeInTheDocument();
    });
    expect(mockOnComplete).toHaveBeenCalledWith(true, false); // success: true, allowIgnore: false (default)
  });

  it("handles preflight run error", async () => {
    server.use(
      http.post(`*/api/${target}/install/app-preflights/run`, ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (!authHeader || !authHeader.startsWith("Bearer ")) {
          return new HttpResponse(null, { status: 401 });
        }
        return HttpResponse.json({ message: "Failed to run application preflight checks" }, { status: 500 });
      })
    );

    renderWithProviders(<AppPreflightCheck onRun={mockOnRun} onComplete={mockOnComplete} />, {
      wrapperProps: {
        target,
        authToken: TEST_TOKEN,
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Unable to complete application requirement checks")).toBeInTheDocument();
      expect(screen.getByText("Failed to run application preflight checks")).toBeInTheDocument();
    });
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

    const runValidationButton = screen.getByRole("button", { name: "Run Validation Again" });
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
      http.get(`*/api/${target}/install/app-preflights/status`, ({ request }) => {
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
          allowIgnoreAppPreflights: true, // Test that this field is properly received
        });
      })
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
    expect(mockOnComplete).toHaveBeenCalledWith(false, true); // success: false, allowIgnore: true
  });

  it("passes allowIgnoreAppPreflights false to onComplete callback", async () => {
    // Mock preflight status endpoint with allowIgnoreAppPreflights: false
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, ({ request }) => {
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
          allowIgnoreAppPreflights: false, // Test that this field is properly received
        });
      })
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

    // The component should call onComplete with success: false, allowIgnore: false
    expect(mockOnComplete).toHaveBeenCalledWith(false, false);
  });
});