import { describe, it, expect } from "vitest";
import { getApiBase } from "./api-base";

describe("getApiBase", () => {
  it("returns install base for install mode", () => {
    expect(getApiBase("linux", "install")).toBe("/api/linux/install");
    expect(getApiBase("kubernetes", "install")).toBe("/api/kubernetes/install");
  });

  it("returns upgrade base for upgrade mode", () => {
    expect(getApiBase("linux", "upgrade")).toBe("/api/linux/upgrade");
    expect(getApiBase("kubernetes", "upgrade")).toBe("/api/kubernetes/upgrade");
  });
});