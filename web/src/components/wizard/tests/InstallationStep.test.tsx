import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallationStep from "../InstallationStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";
import { setupServer } from "msw/node";
import { http, HttpResponse } from "msw";

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

  // Mock window.location.reload before all tests
  let reloadMock: ReturnType<typeof vi.fn>;
  beforeAll(() => {
    server.listen();
    // Setup window.location.reload mock
    reloadMock = vi.fn();
    Object.defineProperty(window, "location", {
      value: { reload: reloadMock },
      writable: true,
    });
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("shows loading state initially", () => {
    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        authenticated: true,
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
      http.get("*/api/install/status", ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        return HttpResponse.json({ state: "Succeeded" });
      })
    );

    renderWithProviders(<InstallationStep />, {
      wrapperProps: {
        authenticated: true,
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
      http.get("*/api/install/status", ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
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
        authenticated: true,
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
});
