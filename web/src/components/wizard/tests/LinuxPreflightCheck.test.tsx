import React from "react";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import LinuxPreflightCheck from "../preflight/LinuxPreflightCheck";
import { ClusterConfig } from "../../../contexts/ConfigContext";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock localStorage
const mockLocalStorage = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  clear: vi.fn(),
};
Object.defineProperty(window, "localStorage", { value: mockLocalStorage });

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

  beforeEach(() => {
    mockFetch.mockClear();
    mockOnComplete.mockClear();
    mockLocalStorage.getItem.mockClear();
    vi.clearAllMocks();
  });

  it("shows initializing state when config is polling", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ state: "Running" }),
    });

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
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Succeeded" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Running" }),
      });

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
    const mockPreflightResponse = {
      output: {
        pass: [{ title: "CPU Check", message: "CPU requirements met" }],
        warn: [{ title: "Memory Warning", message: "Memory is below recommended" }],
        fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
      },
      status: { state: "Succeeded" },
    };

    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Succeeded" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreflightResponse),
      });

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
    const mockPreflightResponse = {
      output: {
        fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
      },
      status: { state: "Failed" },
    };

    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Succeeded" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreflightResponse),
      });

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
    const mockPreflightResponse = {
      output: {
        pass: [{ title: "CPU Check", message: "CPU requirements met" }],
      },
      status: { state: "Succeeded" },
    };

    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Succeeded" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreflightResponse),
      });

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
    mockFetch.mockRejectedValueOnce(new Error("API Error"));

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

  it("allows re-running validation when there are failures", async () => {
    const mockPreflightResponse = {
      output: {
        fail: [{ title: "Disk Space", message: "Insufficient disk space" }],
      },
      status: { state: "Succeeded" },
    };

    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ state: "Succeeded" }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockPreflightResponse),
      });

    renderWithProviders(<LinuxPreflightCheck config={mockConfig} onComplete={mockOnComplete} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    // Wait for the preflight response to be processed and the component to update
    await waitFor(() => {
      expect(screen.getByText("Host Requirements Not Met")).toBeInTheDocument();
      expect(screen.getByText("Disk Space")).toBeInTheDocument();
    });

    // Now check for the button and click it
    const runValidationButton = screen.getByRole("button", { name: "Run Validation Again" });
    expect(runValidationButton).toBeInTheDocument();
    fireEvent.click(runValidationButton);

    // Verify the API call was made
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/install/host-preflights/run",
        expect.objectContaining({
          method: "POST",
          headers: expect.any(Object),
        })
      );
    });
  });
});
