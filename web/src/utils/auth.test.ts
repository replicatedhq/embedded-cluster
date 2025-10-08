import { describe, it, expect, beforeEach, vi } from "vitest";
import { handleUnauthorized } from "./auth";

describe("auth utilities", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sessionStorage.clear();
    localStorage.clear();
  });

  describe("handleUnauthorized", () => {
    it("clears localStorage and sessionStorage when status is 401", () => {
      const reloadSpy = vi.spyOn(window.location, "reload").mockImplementation(() => {});
      localStorage.setItem("auth", "test-token");
      sessionStorage.setItem("embedded-cluster-install-progress", JSON.stringify({ wizardStep: "installation" }));

      const error = { statusCode: 401 };
      const result = handleUnauthorized(error);

      expect(result).toBe(true);
      expect(localStorage.getItem("auth")).toBeFalsy();
      expect(sessionStorage.getItem("embedded-cluster-install-progress")).toBeFalsy();
      expect(reloadSpy).toHaveBeenCalledOnce();

      reloadSpy.mockRestore();
    });

    it("clears sessionStorage for fetch errors with status 401", () => {
      const reloadSpy = vi.spyOn(window.location, "reload").mockImplementation(() => {});
      sessionStorage.setItem("embedded-cluster-install-progress", JSON.stringify({ wizardStep: "installation" }));

      const error = { status: 401 };
      const result = handleUnauthorized(error);

      expect(result).toBe(true);
      expect(sessionStorage.getItem("embedded-cluster-install-progress")).toBeFalsy();
      expect(reloadSpy).toHaveBeenCalledOnce();

      reloadSpy.mockRestore();
    });

    it("does not clear sessionStorage for non-401 status codes", () => {
      const reloadSpy = vi.spyOn(window.location, "reload").mockImplementation(() => {});
      sessionStorage.setItem("embedded-cluster-install-progress", JSON.stringify({ wizardStep: "installation" }));

      const error = { statusCode: 500 };
      const result = handleUnauthorized(error);

      expect(result).toBe(false);
      expect(sessionStorage.getItem("embedded-cluster-install-progress")).not.toBeNull();
      expect(reloadSpy).not.toHaveBeenCalled();

      reloadSpy.mockRestore();
    });

    it("does not clear sessionStorage for 403 forbidden errors", () => {
      const reloadSpy = vi.spyOn(window.location, "reload").mockImplementation(() => {});
      sessionStorage.setItem("embedded-cluster-install-progress", JSON.stringify({ wizardStep: "installation" }));

      const error = { statusCode: 403 };
      const result = handleUnauthorized(error);

      expect(result).toBe(false);
      expect(sessionStorage.getItem("embedded-cluster-install-progress")).not.toBeNull();
      expect(reloadSpy).not.toHaveBeenCalled();

      reloadSpy.mockRestore();
    });

    it("returns false for errors without status property", () => {
      const reloadSpy = vi.spyOn(window.location, "reload").mockImplementation(() => {});
      sessionStorage.setItem("embedded-cluster-install-progress", JSON.stringify({ wizardStep: "installation" }));

      const error = { message: "Unknown error" };
      const result = handleUnauthorized(error);

      expect(result).toBe(false);
      expect(sessionStorage.getItem("embedded-cluster-install-progress")).not.toBeNull();
      expect(reloadSpy).not.toHaveBeenCalled();

      reloadSpy.mockRestore();
    });
  });
});
