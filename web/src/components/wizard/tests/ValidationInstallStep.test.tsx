import React from "react";
import {
  describe,
  it,
  expect,
  vi,
  beforeAll,
  afterEach,
  afterAll,
} from "vitest";
import { screen, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ValidationInstallStep from "../ValidationInstallStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

const server = setupServer(
  // Mock initial installation status
  http.get("*/api/install/status", () => {
    return HttpResponse.json({
      state: "Succeeded",
    });
  })
);

describe("ValidationInstallStep", () => {
  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
  });

  afterAll(() => {
    server.close();
  });

  it("shows loading state initially", async () => {
    // Mock initial loading state
    server.use(
      http.get("*/api/install/status", () => {
        return HttpResponse.json({
          state: "InProgress",
        });
      })
    );

    renderWithProviders(<ValidationInstallStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Verify loading state is shown
    expect(
      screen.getByText(/Please wait while we complete the installation/)
    ).toBeInTheDocument();
  });

  it("shows error message when installation fails", async () => {
    // Mock failed installation status
    server.use(
      http.get("*/api/install/status", () => {
        return HttpResponse.json({
          state: "Failed",
        });
      })
    );

    renderWithProviders(<ValidationInstallStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for and verify error message
    await waitFor(() => {
      expect(screen.getByText(/Installation Error/)).toBeInTheDocument();
      expect(screen.getByText(/Installation failed/)).toBeInTheDocument();
    });
  });

  it("shows admin console link when installation succeeds", async () => {
    // Mock successful installation status
    server.use(
      http.get("*/api/install/status", () => {
        return HttpResponse.json({
          state: "Succeeded",
        });
      })
    );

    renderWithProviders(<ValidationInstallStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Wait for and verify admin console link
    await waitFor(() => {
      expect(screen.getByText(/Visit Admin Console/)).toBeInTheDocument();
    });
  });
});
