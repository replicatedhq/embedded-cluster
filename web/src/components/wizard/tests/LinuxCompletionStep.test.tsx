import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import LinuxCompletionStep from "../completion/LinuxCompletionStep.tsx";
import { mockHandlers } from "../../../test/mockHandlers.ts";
import type { components } from "../../../types/api";

type LinuxConfigResponse = components["schemas"]["types.LinuxInstallationConfigResponse"];

const MOCK_CONFIG: LinuxConfigResponse = {
  values: {
    adminConsolePort: 8800,
    dataDirectory: "/var/lib/embedded-cluster",
  },
  defaults: {
    adminConsolePort: 8800,
    dataDirectory: "/var/lib/embedded-cluster",
  },
  resolved: {
    adminConsolePort: 8800,
    dataDirectory: "/var/lib/embedded-cluster",
  },
};

const createServer = () => setupServer(
  mockHandlers.installation.getConfig(MOCK_CONFIG as unknown as Record<string, unknown>, 'linux', 'install')
);

// Mock window.open
const mockOpen = vi.fn();
Object.defineProperty(window, 'open', {
  value: mockOpen,
  writable: true,
});

describe("LinuxCompletionStep", () => {
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer();
    server.listen();
  });

  beforeEach(() => {
    mockOpen.mockClear();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("shows loading state initially", () => {
    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    expect(screen.getByTestId("linux-completion-loading")).toBeInTheDocument();
    expect(screen.getByText("Loading installation configuration...")).toBeInTheDocument();
  });

  it("renders completion message and button after loading", async () => {
    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("linux-completion-loading")).not.toBeInTheDocument();
    });

    expect(screen.getByTestId("completion-message")).toBeInTheDocument();
    expect(screen.getByTestId("admin-console-button")).toBeInTheDocument();
  });

  it("opens admin console when button is clicked", async () => {
    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("linux-completion-loading")).not.toBeInTheDocument();
    });

    const button = screen.getByTestId("admin-console-button");
    fireEvent.click(button);

    // Verify window.open was called with the correct port (hostname will vary in tests)
    expect(mockOpen).toHaveBeenCalledTimes(1);
    const [url, target] = mockOpen.mock.calls[0];
    expect(url).toContain(`:${MOCK_CONFIG.resolved.adminConsolePort}`);
    expect(target).toBe("_blank");
  });

  it("displays error state when API returns ApiError with details", async () => {
    server.use(
      mockHandlers.installation.getConfig({ error: { statusCode: 500, message: "Internal server error" } }, 'linux', 'install')
    );

    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    // Wait for error state to appear
    await waitFor(() => {
      expect(screen.getByTestId("linux-completion-error")).toBeInTheDocument();
    });

    expect(screen.getByText("Failed to load installation configuration")).toBeInTheDocument();
    expect(screen.getByText("Internal server error")).toBeInTheDocument();
  });

  it("displays error state with generic error message when no details provided", async () => {
    server.use(
      mockHandlers.installation.getConfig({ error: { statusCode: 500, message: "Failed to fetch configuration" } }, 'linux', 'install')
    );

    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    // Wait for error state to appear
    await waitFor(() => {
      expect(screen.getByTestId("linux-completion-error")).toBeInTheDocument();
    });

    expect(screen.getByText("Failed to load installation configuration")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch configuration")).toBeInTheDocument();
  });

  it("displays error state when API returns 401 unauthorized", async () => {
    server.use(
      mockHandlers.installation.getConfig({ error: { statusCode: 401, message: "Unauthorized" } }, 'linux', 'install')
    );

    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("linux-completion-error")).toBeInTheDocument();
    });
  });

  it("displays error state with network error message", async () => {
    server.use(
      mockHandlers.installation.getConfig({ networkError: true }, 'linux', 'install')
    );

    renderWithProviders(<LinuxCompletionStep />, {
      wrapperProps: {
        authenticated: true,
        target: "linux",
        mode: "install",
      },
    });

    await waitFor(() => {
      expect(screen.getByTestId("linux-completion-error")).toBeInTheDocument();
    });
  });
});
