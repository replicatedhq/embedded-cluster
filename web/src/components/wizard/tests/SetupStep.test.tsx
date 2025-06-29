import React from "react";
import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import SetupStep from "../SetupStep.tsx";

describe("SetupStep", () => {
  const mockOnNext = vi.fn();

  it("renders the LinuxSetup component when target is linux", () => {
    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        target: "linux",
      },
    });

    expect(screen.getByTestId("linux-setup")).toBeInTheDocument();
  });

  it("renders the KubernetesSetup component when target is kubernetes", () => {
    renderWithProviders(<SetupStep onNext={mockOnNext} />, {
      wrapperProps: {
        target: "kubernetes",
      },
    });

    expect(screen.getByTestId("kubernetes-setup")).toBeInTheDocument();
  });
});
