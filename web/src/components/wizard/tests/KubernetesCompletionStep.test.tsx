import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesCompletionStep from "../completion/KubernetesCompletionStep.tsx";

describe("KubernetesCompletionStep", () => {
  beforeEach(() => {
    // Mock window.location.hostname
    Object.defineProperty(window, 'location', {
      value: { hostname: 'localhost' },
      writable: true,
    });
  });

  it("renders completion message and copy command button", () => {
    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    expect(screen.getByTestId("completion-message")).toBeInTheDocument();
    expect(screen.getByText("Copy Command")).toBeInTheDocument();
  });

  it("copies install command when button is clicked", async () => {
    // Mock navigator.clipboard.writeText
    const mockWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: mockWriteText },
      writable: true,
    });

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    const button = screen.getByText("Copy Command");
    fireEvent.click(button);

    expect(mockWriteText).toHaveBeenCalled();
  });
});
