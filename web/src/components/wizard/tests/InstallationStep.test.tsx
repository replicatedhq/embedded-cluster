import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallationStep from "../InstallationStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";
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
  // Mock installation status endpoint
  http.get("*/api/install/status", () => {
    return HttpResponse.json({ state: "InProgress" });
  })
);

describe("InstallationStep", () => {
  const mockConfig = {
    useHttps: false,
    domain: "localhost:8080",
    adminConsolePort: 30000,
  };

  beforeAll(() => server.listen());
  afterEach(() => {
    server.resetHandlers();
    mockLocalStorage.getItem.mockClear();
    vi.clearAllMocks();
  });
  afterAll(() => server.close());

  it("shows loading state initially", () => {
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
    server.use(
      http.get("*/api/install/status", () => {
        return HttpResponse.json({ state: "Succeeded" });
      })
    );

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
    server.use(
      http.get("*/api/install/status", () => {
        return HttpResponse.json(
          {
            state: "Failed",
            description: "Installation failed",
            lastUpdated: "2024-03-21T00:00:00Z",
          },
          { status: 200 }
        );
      })
    );

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
    server.use(
      http.get("*/api/install/status", ({ request }) => {
        const authHeader = request.headers.get("Authorization");
        if (authHeader !== "Bearer test-token") {
          return HttpResponse.error();
        }
        return HttpResponse.json({ state: "Succeeded" });
      })
    );

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
  });
});
