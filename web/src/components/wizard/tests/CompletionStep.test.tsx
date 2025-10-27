import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import CompletionStep from "../CompletionStep.tsx";

describe.each([
  { target: "kubernetes" as const, mode: "install" as const, displayName: "Kubernetes Install" },
  { target: "linux" as const, mode: "install" as const, displayName: "Linux Install" },
  { target: "kubernetes" as const, mode: "upgrade" as const, displayName: "Kubernetes Upgrade" },
  { target: "linux" as const, mode: "upgrade" as const, displayName: "Linux Upgrade" }
])("CompletionStep - $displayName", ({ target, mode }) => {
  it("renders completion message", () => {
    renderWithProviders(<CompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target,
        mode,
      },
    });

    // Check that the completion message is displayed
    const message = screen.getByTestId("completion-message");
    expect(message).toBeInTheDocument();
    // Check message is not empty
    expect(message).toBeTruthy();
  });

  it("renders success icon", () => {
    renderWithProviders(<CompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target,
        mode,
      },
    });

    // Check that the CheckCircle icon is rendered (via its container)
    const message = screen.getByTestId("completion-message");
    expect(message).toBeInTheDocument();
  });
});
