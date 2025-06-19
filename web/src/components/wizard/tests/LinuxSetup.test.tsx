import { describe, it, expect } from "vitest";
import { formatErrorMessage } from "../setup/LinuxSetup";

describe("formatErrorMessage", () => {
   it("handles empty string", () => {
      expect(formatErrorMessage("")).toBe("");
   });

   it("replaces field names with their proper format", () => {
      expect(formatErrorMessage("adminConsolePort")).toBe("Admin Console Port");
      expect(formatErrorMessage("dataDirectory")).toBe("Data Directory");
      expect(formatErrorMessage("localArtifactMirrorPort")).toBe("Local Artifact Mirror Port");
      expect(formatErrorMessage("httpProxy")).toBe("HTTP Proxy");
      expect(formatErrorMessage("httpsProxy")).toBe("HTTPS Proxy");
      expect(formatErrorMessage("noProxy")).toBe("Proxy Bypass List");
      expect(formatErrorMessage("networkInterface")).toBe("Network Interface");
      expect(formatErrorMessage("podCidr")).toBe("Pod CIDR");
      expect(formatErrorMessage("serviceCidr")).toBe("Service CIDR");
      expect(formatErrorMessage("globalCidr")).toBe("Reserved Network Range (CIDR)");
      expect(formatErrorMessage("cidr")).toBe("CIDR");
   });

   it("handles multiple field names in one message", () => {
      expect(formatErrorMessage("podCidr and serviceCidr are required")).toBe("Pod CIDR and Service CIDR are required");
      expect(formatErrorMessage("httpProxy and httpsProxy must be set")).toBe("HTTP Proxy and HTTPS Proxy must be set");
   });

   it("preserves non-field words", () => {
      expect(formatErrorMessage("The podCidr is invalid")).toBe("The Pod CIDR is invalid");
      expect(formatErrorMessage("Please set the httpProxy")).toBe("Please set the HTTP Proxy");
   });

   it("handles case insensitivity correctly", () => {
      expect(formatErrorMessage("PodCidr")).toBe("Pod CIDR");
      expect(formatErrorMessage("HTTPPROXY")).toBe("HTTP Proxy");
      expect(formatErrorMessage("cidr")).toBe("CIDR");
      expect(formatErrorMessage("Cidr")).toBe("CIDR");
      expect(formatErrorMessage("CIDR")).toBe("CIDR");
   });

   it("handles real-world error messages", () => {
      expect(formatErrorMessage("The podCidr 10.0.0.0/24 overlaps with serviceCidr 10.0.0.0/16")).toBe(
         "The Pod CIDR 10.0.0.0/24 overlaps with Service CIDR 10.0.0.0/16"
      );
      expect(formatErrorMessage("httpProxy and httpsProxy cannot be empty when noProxy is set")).toBe(
         "HTTP Proxy and HTTPS Proxy cannot be empty when Proxy Bypass List is set"
      );
      expect(formatErrorMessage("adminConsolePort must be between 1024 and 65535")).toBe(
         "Admin Console Port must be between 1024 and 65535"
      );
      expect(formatErrorMessage("dataDirectory /var/lib/k0s is not writable")).toBe(
         "Data Directory /var/lib/k0s is not writable"
      );
      expect(formatErrorMessage("globalCidr must be a valid CIDR block")).toBe(
         "Reserved Network Range (CIDR) must be a valid CIDR block"
      );
   });

   it("handles special characters and formatting", () => {
      expect(formatErrorMessage("admin_console_port and localArtifactMirrorPort cannot be equal.")).toBe(
         "admin_console_port and Local Artifact Mirror Port cannot be equal."
      );
      expect(formatErrorMessage("httpProxy: invalid URL format")).toBe("HTTP Proxy: invalid URL format");
      expect(formatErrorMessage("podCidr: 192.168.0.0/24 (invalid)")).toBe("Pod CIDR: 192.168.0.0/24 (invalid)");
   });
});
