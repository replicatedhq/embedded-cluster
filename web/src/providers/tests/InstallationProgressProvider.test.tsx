import { describe, it, expect, beforeEach, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { InstallationProgressProvider } from "../InstallationProgressProvider";
import { useInstallationProgress } from "../../contexts/InstallationProgressContext";
import { InitialStateContext } from "../../contexts/InitialStateContext";

const STORAGE_KEY = "embedded-cluster-install-progress";

describe("InstallationProgressProvider", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.clearAllMocks();
  });

  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <InitialStateContext.Provider value={{ title: "Test App", installTarget: "linux", mode: "install" }}>
      <InstallationProgressProvider>{children}</InstallationProgressProvider>
    </InitialStateContext.Provider>
  );

  describe("Initialization", () => {
    it("initializes with default state when sessionStorage is empty", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      expect(result.current.wizardStep).toBe("welcome");
      expect(result.current.installationPhase).toBeUndefined();
    });

    it("restores wizardStep from sessionStorage on mount", () => {
      sessionStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          wizardStep: "configuration",
          installationPhase: undefined,
        })
      );

      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      expect(result.current.wizardStep).toBe("configuration");
      expect(result.current.installationPhase).toBeUndefined();
    });

    it("restores installationPhase from sessionStorage on mount", () => {
      sessionStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          wizardStep: "installation",
          installationPhase: "linux-preflight",
        })
      );

      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      expect(result.current.wizardStep).toBe("installation");
      expect(result.current.installationPhase).toBe("linux-preflight");
    });

    it("handles corrupted sessionStorage data gracefully", () => {
      sessionStorage.setItem(STORAGE_KEY, "invalid-json{");
      const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      expect(result.current.wizardStep).toBe("welcome");
      expect(result.current.installationPhase).toBeUndefined();
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to restore installation progress:",
        expect.any(Error)
      );

      consoleErrorSpy.mockRestore();
    });

    it("handles missing sessionStorage gracefully", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      expect(result.current.wizardStep).toBe("welcome");
      expect(result.current.installationPhase).toBeUndefined();
    });
  });

  describe("State Updates", () => {
    it("setWizardStep updates state and persists to sessionStorage", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setWizardStep("configuration");
      });

      expect(result.current.wizardStep).toBe("configuration");

      const stored = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || "{}");
      expect(stored.wizardStep).toBe("configuration");
    });

    it("setInstallationPhase updates state and persists to sessionStorage", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setInstallationPhase("linux-preflight");
      });

      expect(result.current.installationPhase).toBe("linux-preflight");

      const stored = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || "{}");
      expect(stored.installationPhase).toBe("linux-preflight");
    });

    it("updates both wizardStep and installationPhase together", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setWizardStep("installation");
        result.current.setInstallationPhase("app-installation");
      });

      expect(result.current.wizardStep).toBe("installation");
      expect(result.current.installationPhase).toBe("app-installation");

      const stored = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || "{}");
      expect(stored.wizardStep).toBe("installation");
      expect(stored.installationPhase).toBe("app-installation");
    });

    it("allows setting installationPhase to undefined", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setInstallationPhase("linux-preflight");
      });

      expect(result.current.installationPhase).toBe("linux-preflight");

      act(() => {
        result.current.setInstallationPhase(undefined);
      });

      expect(result.current.installationPhase).toBeUndefined();

      const stored = JSON.parse(sessionStorage.getItem(STORAGE_KEY) || "{}");
      expect(stored.installationPhase).toBeUndefined();
    });
  });

  describe("clearProgress", () => {
    it("removes data from sessionStorage", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setWizardStep("installation");
        result.current.setInstallationPhase("app-installation");
      });

      expect(sessionStorage.getItem(STORAGE_KEY)).not.toBeNull();

      act(() => {
        result.current.clearProgress();
      });

      expect(sessionStorage.getItem(STORAGE_KEY)).toBeNull();
    });

    it("does not reset state in memory when clearing", () => {
      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setWizardStep("installation");
        result.current.setInstallationPhase("app-installation");
      });

      act(() => {
        result.current.clearProgress();
      });

      expect(result.current.wizardStep).toBe("installation");
      expect(result.current.installationPhase).toBe("app-installation");
    });
  });

  describe("Error Handling", () => {
    it("handles sessionStorage.setItem errors gracefully", () => {
      const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      const originalSetItem = sessionStorage.setItem;

      // Mock setItem to throw an error
      sessionStorage.setItem = vi.fn(() => {
        throw new Error("Storage quota exceeded");
      });

      const { result } = renderHook(() => useInstallationProgress(), { wrapper });

      act(() => {
        result.current.setWizardStep("configuration");
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Failed to save installation progress:",
        expect.any(Error)
      );

      // Restore original implementation
      sessionStorage.setItem = originalSetItem;
      consoleErrorSpy.mockRestore();
    });
  });

  describe("Context Usage", () => {
    it("throws error when useInstallationProgress is used outside provider", () => {
      expect(() => {
        renderHook(() => useInstallationProgress());
      }).toThrow("useInstallationProgress must be used within an InstallationProgressProvider");
    });
  });
});
