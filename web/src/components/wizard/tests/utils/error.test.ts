import { describe, it, expect } from "vitest";
import { formatErrorMessage } from "../../../../utils/error";

describe("formatErrorMessage", () => {
   it("handles empty string", () => {
      expect(formatErrorMessage("")).toBe("");
   });

   it("handles basic camelCase words", () => {
      expect(formatErrorMessage("helloWorld")).toBe("Hello World");
      expect(formatErrorMessage("helloWorldFooBar")).toBe("Hello World Foo Bar");
   });

   it("preserves non-camelCase words", () => {
      expect(formatErrorMessage("hello world")).toBe("hello world");
      expect(formatErrorMessage("Hello World")).toBe("Hello World");
      expect(formatErrorMessage("HELLO WORLD")).toBe("HELLO WORLD");
   });

   it("handles special cases", () => {
      expect(formatErrorMessage("cidr")).toBe("CIDR");
      expect(formatErrorMessage("Cidr")).toBe("CIDR");
      expect(formatErrorMessage("CIDR")).toBe("CIDR");
      expect(formatErrorMessage("podCidr")).toBe("Pod CIDR");
      expect(formatErrorMessage("pod cidr")).toBe("pod CIDR");
      expect(formatErrorMessage("globalCidr is required")).toBe("Global CIDR is required");
      expect(formatErrorMessage("The provided CIDR block is not valid")).toBe("The provided CIDR block is not valid");
      expect(formatErrorMessage("httpProxy and httpsProxy")).toBe("HTTP Proxy and HTTPS Proxy");
   });

   it("handles multiple special cases in one message", () => {
      expect(formatErrorMessage("podCidr is required when globalCidr is not set")).toBe("Pod CIDR is required when Global CIDR is not set");
   });

   it("handles special characters", () => {
      expect(formatErrorMessage("admin_console_port and localArtifactMirrorPort cannot be equal.")).toBe("admin_console_port and Local Artifact Mirror Port cannot be equal.");
   });
});
