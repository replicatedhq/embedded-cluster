import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import KubernetesCompletionStep from "../completion/KubernetesCompletionStep.tsx";
import { KubernetesConfigResponse } from "../../../types";

const MOCK_CONFIG: KubernetesConfigResponse = {
  values: {
    installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000',
  },
  defaults: {
    installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000',
  },
  resolved: {
    installCommand: 'kubectl -n kotsadm port-forward svc/kotsadm 8800:3000',
  },
};

const createServer = () => setupServer(
  http.get(`*/api/kubernetes/install/installation/config`, () => {
    return HttpResponse.json(MOCK_CONFIG);
  })
);

describe("KubernetesCompletionStep", () => {
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer();
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("shows loading state initially", () => {
    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    expect(screen.getByTestId("kubernetes-completion-loading")).toBeInTheDocument();
    expect(screen.getByText("Loading installation configuration...")).toBeInTheDocument();
  });

  it("renders completion message and copy command button after loading", async () => {
    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("kubernetes-completion-loading")).not.toBeInTheDocument();
    });

    // Check success state
    expect(screen.getByTestId("completion-message")).toBeInTheDocument();
    expect(screen.getByText("Copy Command")).toBeInTheDocument();
    expect(screen.getByText(MOCK_CONFIG.resolved.installCommand!)).toBeInTheDocument();
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
        target: "kubernetes",
        mode: "install",
      },
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("kubernetes-completion-loading")).not.toBeInTheDocument();
    });

    const button = screen.getByText("Copy Command");
    fireEvent.click(button);

    expect(mockWriteText).toHaveBeenCalledWith(MOCK_CONFIG.resolved.installCommand);

    // Check that button text changes to "Copied!"
    await waitFor(() => {
      expect(screen.getByText("Copied!")).toBeInTheDocument();
    });
  });

  it("displays error state when API returns ApiError with details", async () => {
    // Override the server handler to return an ApiError with details
    server.use(
      http.get("*/api/kubernetes/install/installation/config", () => {
        return HttpResponse.json(
          { message: "Network error occurred" },
          { status: 500 }
        );
      })
    );

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    // Wait for error state to appear
    await waitFor(() => {
      expect(screen.getByTestId("kubernetes-completion-error")).toBeInTheDocument();
    });

    expect(screen.getByText("Failed to load installation configuration")).toBeInTheDocument();
    expect(screen.getByText("Network error occurred")).toBeInTheDocument();
  });

  it("displays error state with generic error message when no details provided", async () => {
    // Override the server handler to return an error without details
    server.use(
      http.get("*/api/kubernetes/install/installation/config", () => {
        return HttpResponse.json(
          { error: "Failed to fetch configuration" },
          { status: 500 }
        );
      })
    );

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    // Wait for error state to appear
    await waitFor(() => {
      expect(screen.getByTestId("kubernetes-completion-error")).toBeInTheDocument();
    });

    expect(screen.getByText("Failed to load installation configuration")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch install configuration")).toBeInTheDocument();
  });

  it("displays error state when API returns 404", async () => {
    server.use(
      http.get("*/api/kubernetes/install/installation/config", () => {
        return HttpResponse.json(
          { error: "Not found" },
          { status: 404 }
        );
      })
    );

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("kubernetes-completion-error")).toBeInTheDocument();
    });
  });

  it("displays error state with network error message", async () => {
    server.use(
      http.get("*/api/kubernetes/install/installation/config", () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(<KubernetesCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "kubernetes",
        mode: "install",
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("kubernetes-completion-error")).toBeInTheDocument();
    });
  });
});
