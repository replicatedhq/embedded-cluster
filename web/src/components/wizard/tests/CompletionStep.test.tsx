import React from "react";
import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import CompletionStep from "../CompletionStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

describe("CompletionStep", () => {
  it("renders completion content correctly", () => {
    renderWithProviders(<CompletionStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Check if completion content is rendered
    expect(screen.getByText(/Installation Complete!/)).toBeInTheDocument();
    expect(screen.getByText(/is installed successfully/)).toBeInTheDocument();
  });

  it("shows admin console link", () => {
    renderWithProviders(<CompletionStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
          config: {
            useHttps: false,
            domain: "localhost:8080",
          },
        },
      },
    });

    // Check if admin console link is present
    const adminLink = screen.getByText(/Access Admin Dashboard/).closest("button");
    expect(adminLink).toBeInTheDocument();
  });

  it("shows next steps", () => {
    renderWithProviders(<CompletionStep />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Check if next steps are rendered
    expect(screen.getByText(/Next Steps/)).toBeInTheDocument();
    expect(screen.getByText(/Log in to your/)).toBeInTheDocument();
    expect(screen.getByText(/Configure additional settings/)).toBeInTheDocument();
    expect(screen.getByText(/Create your first organization/)).toBeInTheDocument();
  });
});
