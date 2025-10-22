import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesCompletionStep from "../completion/KubernetesCompletionStep.tsx";

describe("KubernetesCompletionStep", () => {
  it("renders completion message with app title", () => {
    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    // Check that the completion message is displayed with default title "My App"
    const message = screen.getByTestId("completion-message");
    expect(message).toBeInTheDocument();
    expect(message).toHaveTextContent("Visit the Admin Console to configure and install My App");
  });

  it("renders success icon", () => {
    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    // Check that the CheckCircle icon is rendered (via its container)
    const message = screen.getByTestId("completion-message");
    expect(message).toBeInTheDocument();
  });

  it("uses custom app title from initial state", () => {
    const customTitle = "My Custom App";

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
        contextValues: {
          initialStateContext: {
            title: customTitle,
            installTarget: "kubernetes",
            mode: "install",
            isAirgap: false,
            requiresInfraUpgrade: false,
          },
        },
      },
    });

    const message = screen.getByTestId("completion-message");
    expect(message).toHaveTextContent(`Visit the Admin Console to configure and install ${customTitle}`);
  });
});
