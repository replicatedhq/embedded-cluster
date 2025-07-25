import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { screen, fireEvent } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import LinuxCompletionStep from "../completion/LinuxCompletionStep.tsx";

// Mock window.open
const mockOpen = vi.fn();
Object.defineProperty(window, 'open', {
  value: mockOpen,
  writable: true,
});

describe("LinuxCompletionStep", () => {
  beforeEach(() => {
    mockOpen.mockClear();
    // Mock window.location.hostname
    Object.defineProperty(window, 'location', {
      value: { hostname: 'localhost' },
      writable: true,
    });
  });

  it("renders completion message and button", () => {
    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    expect(screen.getByTestId("completion-message")).toBeInTheDocument();
    expect(screen.getByTestId("admin-console-button")).toBeInTheDocument();
  });

  it("opens admin console when button is clicked", () => {
    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
      },
    });

    const button = screen.getByTestId("admin-console-button");
    fireEvent.click(button);

    expect(mockOpen).toHaveBeenCalledWith("http://localhost:8800", "_blank");
  });
}); 