import React from "react";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import LinuxPreflightCheck from "../preflight/LinuxPreflightCheck";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

// Mock localStorage
const mockLocalStorage = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, "localStorage", { value: mockLocalStorage });

const server = setupServer(
  // Mock config status endpoint
  http.get("*/api/install/installation/status", () => {
    return HttpResponse.json({ state: "Succeeded" });
  }),

  // Mock preflight status endpoint
  http.get("*/api/install/host-preflights/status", () => {
    return HttpResponse.json({
      output: {
        pass: [{ title: "CPU Check", message: "CPU requirements met" }],
        warn: [{ title: "Memory Warning", message: "Memory is below recommended" }],
        fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
      },
      status: { state: "Succeeded" },
    });
  }),

  // Mock preflight run endpoint
  http.post("*/api/install/host-preflights/run", () => {
    return HttpResponse.json({ success: true });
  })
);

describe("LinuxPreflightCheck", () => {
  const mockOnComplete = vi.fn();
  const mockConfig: ClusterConfig = {
    clusterName: "test-cluster",
    namespace: "default",
    storageClass: "standard",
    domain: "example.com",
    useHttps: true,
    adminUsername: "admin",
    adminPassword: "test",
    adminEmail: "admin@example.com",
    databaseType: "internal",
    dataDirectory: "/var/lib/embedded-cluster",
    useProxy: false,
  };

  beforeAll(() => server.listen());
  afterEach(() => {
    server.resetHandlers();
    mockOnComplete.mockClear();
    mockLocalStorage.getItem.mockClear();
    vi.clearAllMocks();
  });
  afterAll(() => server.close());

  it("shows initializing state when config is polling", async () => {
    server.use(
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Running" });
      })
    );

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    expect(screen.getByText("Initializing...")).toBeInTheDocument();
    expect(screen.getByText("Preparing the host.")).toBeInTheDocument();
  });

  it("shows validating state when preflights are polling", async () => {
    server.use(
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({ state: "Running" });
      })
    );

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
      expect(screen.getByText("Please wait while we check your system.")).toBeInTheDocument();
    });
  });

  it("displays preflight results correctly", async () => {
    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("CPU Check")).toBeInTheDocument();
      expect(screen.getByText("Memory Warning")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });
  });

  it("calls onComplete with false when preflights fail", async () => {
    server.use(
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
          },
          status: { state: "Failed" },
        });
      })
    );

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(mockOnComplete).toHaveBeenCalledWith(false);
    });
  });

  it("calls onComplete with true when all preflights pass", async () => {
    server.use(
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          output: {
            pass: [{ title: "CPU Check", message: "CPU requirements met" }],
          },
          status: { state: "Succeeded" },
        });
      })
    );

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(mockOnComplete).toHaveBeenCalledWith(true);
    });
  });

  it("handles API errors gracefully", async () => {
    // First return success for config status to get past initializing
    server.use(
      http.get("*/api/install/installation/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      }),
      http.get("*/api/install/host-preflights/status", () => {
        return HttpResponse.json({
          titles: ["CPU", "Memory", "Disk Space"],
          output: {
            pass: [{ title: "CPU", message: "CPU requirements met" }],
            warn: null,
            fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
          },
          status: {
            state: "Failed",
            description: "Host preflights failed",
            lastUpdated: "2025-06-06T16:06:20.464621507Z",
          },
        });
      })
    );

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // First wait for the initializing state
    await waitFor(() => {
      expect(screen.getByText("Initializing...")).toBeInTheDocument();
    });

    // wait for initializing state to disappear
    await waitFor(() => {
      expect(screen.queryByText("Initializing...")).not.toBeInTheDocument();
    });

    // expect host requirements not met
    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
    });

    // expect disk space to be in the document
    await waitFor(() => {
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
      expect(screen.getByText("Insufficient disk space")).toBeInTheDocument();
    });

    // verify onComplete was called with false
    expect(mockOnComplete).toHaveBeenCalledWith(false);
  });

  it("allows re-running validation when there are failures", async () => {
    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    const runValidationButton = screen.getByRole("button", { name: "Run Validation Again" });
    expect(runValidationButton).toBeInTheDocument();
    fireEvent.click(runValidationButton);

    await waitFor(() => {
      expect(screen.getByText("Validating host requirements...")).toBeInTheDocument();
    });
  });
});
