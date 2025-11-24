import { describe, it, expect } from 'vitest';
import { formatErrorMessage } from "./errorMessage";

const kubernetesFieldNames = {
  localArtifactMirrorPort: "Admin Console Port",
  httpProxy: "HTTP Proxy",
  httpsProxy: "HTTPS Proxy",
  noProxy: "Proxy Bypass List",
}

describe("formatErrorMessage Kubernetes", () => {
  it("handles empty string", () => {
    expect(formatErrorMessage("", kubernetesFieldNames)).toBe("");
  });

  it("replaces field names with their proper format", () => {
    expect(formatErrorMessage("localArtifactMirrorPort", kubernetesFieldNames)).toBe("Local Artifact Mirror Port");
    expect(formatErrorMessage("httpProxy", kubernetesFieldNames)).toBe("HTTP Proxy");
    expect(formatErrorMessage("httpsProxy", kubernetesFieldNames)).toBe("HTTPS Proxy");
    expect(formatErrorMessage("noProxy", kubernetesFieldNames)).toBe("Proxy Bypass List");
  });

  it("handles multiple field names in one message", () => {
    expect(formatErrorMessage("httpProxy and httpsProxy are required", kubernetesFieldNames)).toBe("HTTP Proxy and HTTPS Proxy are required");
    expect(formatErrorMessage("localArtifactMirrorPort and noProxy must be set", kubernetesFieldNames)).toBe("Local Artifact Mirror Port and Proxy Bypass List must be set");
  });

  it("preserves non-field words", () => {
    expect(formatErrorMessage("The localArtifactMirrorPort is invalid", kubernetesFieldNames)).toBe("The Local Artifact Mirror Port is invalid");
    expect(formatErrorMessage("Please set the httpProxy", kubernetesFieldNames)).toBe("Please set the HTTP Proxy");
  });

  it("handles case insensitivity correctly", () => {
    expect(formatErrorMessage("LocalArtifactMirrorPort", kubernetesFieldNames)).toBe("Local Artifact Mirror Port");
    expect(formatErrorMessage("HTTPPROXY", kubernetesFieldNames)).toBe("HTTP Proxy");
    expect(formatErrorMessage("NoProxy", kubernetesFieldNames)).toBe("Proxy Bypass List");
  });

  it("handles real-world error messages", () => {
    expect(formatErrorMessage("httpProxy and httpsProxy cannot be empty when noProxy is set", kubernetesFieldNames)).toBe(
      "HTTP Proxy and HTTPS Proxy cannot be empty when Proxy Bypass List is set"
    );
    expect(formatErrorMessage("localArtifactMirrorPort must be between 1024 and 65535", kubernetesFieldNames)).toBe(
      "Local Artifact Mirror Port must be between 1024 and 65535"
    );
  });

  it("handles special characters and formatting", () => {
    expect(formatErrorMessage("httpProxy: invalid URL format", kubernetesFieldNames)).toBe("HTTP Proxy: invalid URL format");
    expect(formatErrorMessage("localArtifactMirrorPort: 30000 (invalid)", kubernetesFieldNames)).toBe("Local Artifact Mirror Port: 30000 (invalid)");
  });
});

const linuxFieldNames = {
  dataDirectory: "Data Directory",
  localArtifactMirrorPort: "Local Artifact Mirror Port",
  httpProxy: "HTTP Proxy",
  httpsProxy: "HTTPS Proxy",
  noProxy: "Proxy Bypass List",
  networkInterface: "Network Interface",
  podCidr: "Pod CIDR",
  serviceCidr: "Service CIDR",
  globalCidr: "Reserved Network Range (CIDR)",
  cidr: "CIDR",
}

describe("formatErrorMessage Linux", () => {
  it("handles empty string", () => {
     expect(formatErrorMessage("", linuxFieldNames)).toBe("");
  });

  it("replaces field names with their proper format", () => {
     expect(formatErrorMessage("dataDirectory", linuxFieldNames)).toBe("Data Directory");
     expect(formatErrorMessage("localArtifactMirrorPort", linuxFieldNames)).toBe("Local Artifact Mirror Port");
     expect(formatErrorMessage("httpProxy", linuxFieldNames)).toBe("HTTP Proxy");
     expect(formatErrorMessage("httpsProxy", linuxFieldNames)).toBe("HTTPS Proxy");
     expect(formatErrorMessage("noProxy", linuxFieldNames)).toBe("Proxy Bypass List");
     expect(formatErrorMessage("networkInterface", linuxFieldNames)).toBe("Network Interface");
     expect(formatErrorMessage("podCidr", linuxFieldNames)).toBe("Pod CIDR");
     expect(formatErrorMessage("serviceCidr", linuxFieldNames)).toBe("Service CIDR");
     expect(formatErrorMessage("globalCidr", linuxFieldNames)).toBe("Reserved Network Range (CIDR)");
     expect(formatErrorMessage("cidr", linuxFieldNames)).toBe("CIDR");
  });

  it("handles multiple field names in one message", () => {
     expect(formatErrorMessage("podCidr and serviceCidr are required", linuxFieldNames)).toBe("Pod CIDR and Service CIDR are required");
     expect(formatErrorMessage("httpProxy and httpsProxy must be set", linuxFieldNames)).toBe("HTTP Proxy and HTTPS Proxy must be set");
  });

  it("preserves non-field words", () => {
     expect(formatErrorMessage("The podCidr is invalid", linuxFieldNames)).toBe("The Pod CIDR is invalid");
     expect(formatErrorMessage("Please set the httpProxy", linuxFieldNames)).toBe("Please set the HTTP Proxy");
  });

  it("handles case insensitivity correctly", () => {
     expect(formatErrorMessage("PodCidr", linuxFieldNames)).toBe("Pod CIDR");
     expect(formatErrorMessage("HTTPPROXY", linuxFieldNames)).toBe("HTTP Proxy");
     expect(formatErrorMessage("cidr", linuxFieldNames)).toBe("CIDR");
     expect(formatErrorMessage("Cidr", linuxFieldNames)).toBe("CIDR");
     expect(formatErrorMessage("CIDR", linuxFieldNames)).toBe("CIDR");
  });

  it("handles real-world error messages", () => {
     expect(formatErrorMessage("The podCidr 10.0.0.0/24 overlaps with serviceCidr 10.0.0.0/16", linuxFieldNames)).toBe(
        "The Pod CIDR 10.0.0.0/24 overlaps with Service CIDR 10.0.0.0/16"
     );
     expect(formatErrorMessage("httpProxy and httpsProxy cannot be empty when noProxy is set", linuxFieldNames)).toBe(
        "HTTP Proxy and HTTPS Proxy cannot be empty when Proxy Bypass List is set"
     );
     expect(formatErrorMessage("localArtifactMirrorPort must be between 1024 and 65535", linuxFieldNames)).toBe(
        "Local Artifact Mirror Port must be between 1024 and 65535"
     );
     expect(formatErrorMessage("dataDirectory /var/lib/k0s is not writable", linuxFieldNames)).toBe(
        "Data Directory /var/lib/k0s is not writable"
     );
     expect(formatErrorMessage("globalCidr must be a valid CIDR block", linuxFieldNames)).toBe(
        "Reserved Network Range (CIDR) must be a valid CIDR block"
     );
  });

  it("handles special characters and formatting", () => {
     expect(formatErrorMessage("httpProxy: invalid URL format", linuxFieldNames)).toBe("HTTP Proxy: invalid URL format");
     expect(formatErrorMessage("podCidr: 192.168.0.0/24 (invalid)", linuxFieldNames)).toBe("Pod CIDR: 192.168.0.0/24 (invalid)");
  });
});
