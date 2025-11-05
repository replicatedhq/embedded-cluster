import { describe, it, expect } from "vitest";
import {
  processInputValue,
  extractFieldError,
  determineLoadingText,
  shouldExpandAdvancedSettings,
  evaluateInstallationStatus,
  determineLoadingState,
  fieldNames,
} from "../setup/LinuxSetupStepHops";

describe("LinuxSetupStep Hops", () => {
  describe("processInputValue", () => {
    it("converts valid integer string to number for adminConsolePort", () => {
      const result = processInputValue(
        "adminConsolePort",
        "8080",
        { dataDirectory: "/var" }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        adminConsolePort: 8080,
      });
    });

    it("converts empty string to undefined for port fields", () => {
      const result = processInputValue(
        "adminConsolePort",
        "",
        { dataDirectory: "/var", adminConsolePort: 8080 }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        adminConsolePort: undefined,
      });
    });

    it("rejects decimal values for port fields", () => {
      const currentValues = { dataDirectory: "/var", adminConsolePort: 8080 };
      const result = processInputValue(
        "adminConsolePort",
        "8080.5",
        currentValues
      );

      expect(result).toBe(currentValues); // Reference equality - unchanged
    });

    it("rejects non-numeric values for port fields", () => {
      const currentValues = { adminConsolePort: 8080 };
      const result = processInputValue(
        "adminConsolePort",
        "abc",
        currentValues
      );

      expect(result).toBe(currentValues); // Unchanged
    });

    it("handles localArtifactMirrorPort as a port field", () => {
      const result = processInputValue(
        "localArtifactMirrorPort",
        "9090",
        {}
      );

      expect(result).toEqual({
        localArtifactMirrorPort: 9090,
      });
    });

    it("stores string values for non-port fields", () => {
      const result = processInputValue(
        "dataDirectory",
        "/opt/data",
        {}
      );

      expect(result).toEqual({
        dataDirectory: "/opt/data",
      });
    });

    it("handles proxy field as string", () => {
      const result = processInputValue(
        "httpProxy",
        "http://proxy:8080",
        { dataDirectory: "/var" }
      );

      expect(result).toEqual({
        dataDirectory: "/var",
        httpProxy: "http://proxy:8080",
      });
    });
  });

  describe("extractFieldError", () => {
    it("returns formatted error for matching field", () => {
      const errors = [
        { field: "adminConsolePort", message: "adminConsolePort must be between 1024 and 65535" },
      ];

      const result = extractFieldError("adminConsolePort", errors, fieldNames);

      expect(result).toBe("Admin Console Port must be between 1024 and 65535");
    });

    it("returns undefined when field has no error", () => {
      const errors = [
        { field: "adminConsolePort", message: "some error" },
      ];

      const result = extractFieldError("dataDirectory", errors, fieldNames);

      expect(result).toBeUndefined();
    });

    it("returns undefined when fieldErrors is undefined", () => {
      const result = extractFieldError("dataDirectory", undefined, fieldNames);

      expect(result).toBeUndefined();
    });

    it("handles empty error array", () => {
      const result = extractFieldError("dataDirectory", [], fieldNames);

      expect(result).toBeUndefined();
    });
  });

  describe("determineLoadingText", () => {
    it("shows installation message when polling", () => {
      const result = determineLoadingText(true);
      expect(result).toBe("Preparing the host.");
    });

    it("shows configuration message when not polling", () => {
      const result = determineLoadingText(false);
      expect(result).toBe("Loading configuration...");
    });
  });

  describe("shouldExpandAdvancedSettings", () => {
    it("expands when networkInterface has error", () => {
      const errors = [{ field: "networkInterface", message: "required" }];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(true);
    });

    it("expands when globalCidr has error", () => {
      const errors = [{ field: "globalCidr", message: "invalid CIDR" }];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(true);
    });

    it("does not expand for non-advanced field errors", () => {
      const errors = [
        { field: "dataDirectory", message: "required" },
        { field: "adminConsolePort", message: "invalid" },
      ];
      const result = shouldExpandAdvancedSettings(errors);
      expect(result).toBe(false);
    });

    it("handles undefined errors", () => {
      const result = shouldExpandAdvancedSettings(undefined);
      expect(result).toBe(false);
    });

    it("handles empty error array", () => {
      const result = shouldExpandAdvancedSettings([]);
      expect(result).toBe(false);
    });
  });

  describe("evaluateInstallationStatus", () => {
    it("continues polling when status is undefined", () => {
      const result = evaluateInstallationStatus(undefined);

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });

    it("succeeds and proceeds to next step", () => {
      const result = evaluateInstallationStatus({
        state: "Succeeded",
        description: "Installation configured successfully",
      });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: true,
        errorMessage: null,
      });
    });

    it("fails with description", () => {
      const result = evaluateInstallationStatus({
        state: "Failed",
        description: "Network configuration error",
      });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: false,
        errorMessage: "Installation configuration failed with: Network configuration error",
      });
    });

    it("fails without description", () => {
      const result = evaluateInstallationStatus({ state: "Failed" });

      expect(result).toEqual({
        shouldStopPolling: true,
        shouldProceedToNext: false,
        errorMessage: "Installation configuration failed",
      });
    });

    it("continues polling for Running state", () => {
      const result = evaluateInstallationStatus({
        state: "Running",
        description: "Processing",
      });

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });

    it("continues polling for unknown states", () => {
      const result = evaluateInstallationStatus({
        state: "Pending",
      });

      expect(result).toEqual({
        shouldStopPolling: false,
        shouldProceedToNext: false,
        errorMessage: null,
      });
    });
  });

  describe("determineLoadingState", () => {
    it("returns true when config is loading", () => {
      const result = determineLoadingState(true, false, false);
      expect(result).toBe(true);
    });

    it("returns true when interfaces are loading", () => {
      const result = determineLoadingState(false, true, false);
      expect(result).toBe(true);
    });

    it("returns true when polling installation", () => {
      const result = determineLoadingState(false, false, true);
      expect(result).toBe(true);
    });

    it("returns true when multiple sources loading", () => {
      const result = determineLoadingState(true, true, true);
      expect(result).toBe(true);
    });

    it("returns false when nothing is loading", () => {
      const result = determineLoadingState(false, false, false);
      expect(result).toBe(false);
    });
  });
});
