import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallationStep from "../InstallationStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

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

describe("InstallationStep", () => {
  const mockConfig = {
    useHttps: false,
    domain: "localhost:8080",
    adminConsolePort: 30000,
  };

  beforeEach(() => {
    mockFetch.mockClear();
    mockLocalStorage.getItem.mockClear();
    vi.clearAllMocks();
  });

  it("shows loading state initially", () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ state: "InProgress" }),
    });

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    expect(screen.getByText("Please wait while we complete the installation...")).toBeInTheDocument();
    expect(screen.getByText("This may take a few minutes.")).toBeInTheDocument();
  });

  it("shows admin console link when installation succeeds", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ state: "Succeeded" }),
    });

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Visit Admin Console")).toBeInTheDocument();
    });

    const adminLink = screen.getByText("Visit Admin Console");
    expect(adminLink).toBeInTheDocument();
  });

  it("shows error message when installation fails", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ state: "Failed" }),
    });

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Installation Error")).toBeInTheDocument();
    });
  });

  it("handles API errors gracefully", async () => {
    mockFetch.mockRejectedValueOnce(new Error("API Error"));

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(screen.getByText("Installation Error")).toBeInTheDocument();
    });
  });

  it("includes auth token in API requests when available", async () => {
    mockLocalStorage.getItem.mockReturnValue("test-token");
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: () => Promise.resolve({ state: "Succeeded" }),
    });

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: mockConfig,
        },
      },
    });

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/install/status",
        expect.objectContaining({
          headers: expect.objectContaining({
            Authorization: "Bearer test-token",
          }),
        })
      );
    });
  });
});
