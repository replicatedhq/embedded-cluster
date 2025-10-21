import { describe, it, expect } from "vitest";
import { getApiBasePath } from "./client";

describe("getApiBase", () => {
  it("returns install base for install mode", () => {
    expect(getApiBasePath("linux", "install")).toBe("/api/linux/install");
    expect(getApiBasePath("kubernetes", "install")).toBe(
      "/api/kubernetes/install",
    );
  });

  it("returns upgrade base for upgrade mode", () => {
    expect(getApiBasePath("linux", "upgrade")).toBe("/api/linux/upgrade");
    expect(getApiBasePath("kubernetes", "upgrade")).toBe(
      "/api/kubernetes/upgrade",
    );
  });
});

