import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { InitialStateContext, useInitialState, InitialStateProvider } from "../InitialStateContext";
import { InstallationTarget } from "../../types/installation-target";

describe("InitialStateContext", () => {
  const originalWindow = global.window;

  beforeEach(() => {
    // Reset window mock before each test
    global.window = {
      ...originalWindow,
      __INITIAL_STATE__: undefined,
    } as any;
  });

  afterEach(() => {
    // Restore original window
    global.window = originalWindow;
    vi.clearAllMocks();
  });

  describe("useInitialState hook", () => {
    it("returns context value when used within provider", () => {
      const mockContext = {
        title: "Test App",
        icon: "test-icon.png",
        installTarget: "linux" as InstallationTarget,
      };

      const TestComponent = () => {
        const state = useInitialState();
        return <div>{state.title}</div>;
      };

      render(
        <InitialStateContext.Provider value={mockContext}>
          <TestComponent />
        </InitialStateContext.Provider>
      );

      expect(screen.getByText("Test App")).toBeInTheDocument();
    });
  });

  describe("parseInstallationTarget function", () => {
    it("linux is a valid installation target", () => {

      const TestComponent = () => {
        const state = useInitialState();
        return <div>{state.installTarget}</div>;
      };

      // Set up window.__INITIAL_STATE__ with valid target
      global.window.__INITIAL_STATE__ = {
        installTarget: "linux",
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByText("linux")).toBeInTheDocument();
    });

    it("kubernetes is a valid installation target", () => {

      const TestComponent = () => {
        const state = useInitialState();
        return <div>{state.installTarget}</div>;
      };

      // Set up window.__INITIAL_STATE__ with valid target
      global.window.__INITIAL_STATE__ = {
        installTarget: "kubernetes",
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByText("kubernetes")).toBeInTheDocument();
    });

    it("throws error for invalid installation target", () => {

      // Set up window.__INITIAL_STATE__ with invalid target
      global.window.__INITIAL_STATE__ = {
        installTarget: "invalid-target",
      };

      // Mock console.error to prevent error output in tests
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => { });

      expect(() => {
        render(
          <InitialStateProvider>
            <div>Test</div>
          </InitialStateProvider>
        );
      }).toThrow("Invalid installation target: invalid-target");

      consoleSpy.mockRestore();
    });
  });

  describe("InitialStateProvider", () => {
    it("provides default values when window.__INITIAL_STATE__ is not available", () => {

      const TestComponent = () => {
        const state = useInitialState();
        return (
          <div>
            <span data-testid="title">{state.title}</span>
            <span data-testid="icon">{state.icon || "no-icon"}</span>
            <span data-testid="target">{state.installTarget}</span>
          </div>
        );
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByTestId("title")).toHaveTextContent("My App");
      expect(screen.getByTestId("icon")).toHaveTextContent("no-icon");
      expect(screen.getByTestId("target")).toHaveTextContent("linux");
    });

    it("uses values from window.__INITIAL_STATE__ when available", () => {

      global.window.__INITIAL_STATE__ = {
        title: "Custom App Title",
        icon: "custom-icon.png",
        installTarget: "kubernetes",
      };

      const TestComponent = () => {
        const state = useInitialState();
        return (
          <div>
            <span data-testid="title">{state.title}</span>
            <span data-testid="icon">{state.icon}</span>
            <span data-testid="target">{state.installTarget}</span>
          </div>
        );
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByTestId("title")).toHaveTextContent("Custom App Title");
      expect(screen.getByTestId("icon")).toHaveTextContent("custom-icon.png");
      expect(screen.getByTestId("target")).toHaveTextContent("kubernetes");
    });

    it("falls back to defaults for missing properties", () => {

      global.window.__INITIAL_STATE__ = {
        title: "Partial Config",
        // icon and installTarget are missing
      };

      const TestComponent = () => {
        const state = useInitialState();
        return (
          <div>
            <span data-testid="title">{state.title}</span>
            <span data-testid="icon">{state.icon || "no-icon"}</span>
            <span data-testid="target">{state.installTarget}</span>
          </div>
        );
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByTestId("title")).toHaveTextContent("Partial Config");
      expect(screen.getByTestId("icon")).toHaveTextContent("no-icon");
      expect(screen.getByTestId("target")).toHaveTextContent("linux");
    });

    it("handles empty window.__INITIAL_STATE__ object", () => {

      global.window.__INITIAL_STATE__ = {};

      const TestComponent = () => {
        const state = useInitialState();
        return (
          <div>
            <span data-testid="title">{state.title}</span>
            <span data-testid="icon">{state.icon || "no-icon"}</span>
            <span data-testid="target">{state.installTarget}</span>
          </div>
        );
      };

      render(
        <InitialStateProvider>
          <TestComponent />
        </InitialStateProvider>
      );

      expect(screen.getByTestId("title")).toHaveTextContent("My App");
      expect(screen.getByTestId("icon")).toHaveTextContent("no-icon");
      expect(screen.getByTestId("target")).toHaveTextContent("linux");
    });

    it("renders children correctly", () => {

      render(
        <InitialStateProvider>
          <div data-testid="child">Child Component</div>
        </InitialStateProvider>
      );

      expect(screen.getByTestId("child")).toHaveTextContent("Child Component");
    });
  });

  describe("InitialStateContext default value", () => {
    it("has correct default values", () => {
      const TestComponent = () => {
        return (
          <InitialStateContext.Consumer>
            {(value) => (
              <div>
                <span data-testid="title">{value.title}</span>
                <span data-testid="target">{value.installTarget}</span>
              </div>
            )}
          </InitialStateContext.Consumer>
        );
      };

      render(<TestComponent />);

      expect(screen.getByTestId("title")).toHaveTextContent("My App");
      expect(screen.getByTestId("target")).toHaveTextContent("linux");
    });
  });
});
