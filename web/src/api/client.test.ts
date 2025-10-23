import { describe, it, expect, vi } from "vitest";
import {
  getApiBasePath,
  getWizardBasePath,
  getAppInstallPath,
  createBaseClient,
  createAuthedClient,
} from "./client";
import { ApiError } from "./error";

describe("getApiBasePath", () => {
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

describe("getWizardBasePath", () => {
  it("returns /linux/install for linux install mode", () => {
    expect(getWizardBasePath("linux", "install")).toBe("/linux/install");
  });

  it("returns /linux/upgrade for linux upgrade mode", () => {
    expect(getWizardBasePath("linux", "upgrade")).toBe("/linux/upgrade");
  });

  it("returns /kubernetes/install for kubernetes install mode", () => {
    expect(getWizardBasePath("kubernetes", "install")).toBe(
      "/kubernetes/install",
    );
  });

  it("returns /kubernetes/upgrade for kubernetes upgrade mode", () => {
    expect(getWizardBasePath("kubernetes", "upgrade")).toBe(
      "/kubernetes/upgrade",
    );
  });
});

describe("getAppInstallPath", () => {
  it("returns /linux/install/app/install for linux install mode", () => {
    expect(getAppInstallPath("linux", "install")).toBe(
      "/linux/install/app/install",
    );
  });

  it("returns /linux/upgrade/app/upgrade for linux upgrade mode", () => {
    expect(getAppInstallPath("linux", "upgrade")).toBe(
      "/linux/upgrade/app/upgrade",
    );
  });

  it("returns /kubernetes/install/app/install for kubernetes install mode", () => {
    expect(getAppInstallPath("kubernetes", "install")).toBe(
      "/kubernetes/install/app/install",
    );
  });

  it("returns /kubernetes/upgrade/app/upgrade for kubernetes upgrade mode", () => {
    expect(getAppInstallPath("kubernetes", "upgrade")).toBe(
      "/kubernetes/upgrade/app/upgrade",
    );
  });
});

describe("createBaseClient", () => {
  it("creates client with correct base URL", () => {
    const client = createBaseClient();
    expect(client).toBeDefined();
  });

  it("middleware throws ApiError on 400 response", async () => {
    const mockResponse = new Response(
      JSON.stringify({ message: "Bad request" }),
      {
        status: 400,
        headers: { "Content-Type": "application/json" },
      },
    );

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow(ApiError);
  });

  it("middleware throws ApiError on 401 response", async () => {
    const mockResponse = new Response(
      JSON.stringify({ message: "Unauthorized" }),
      {
        status: 401,
        headers: { "Content-Type": "application/json" },
      },
    );

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow(ApiError);
  });

  it("middleware throws ApiError on 404 response", async () => {
    const mockResponse = new Response(
      JSON.stringify({ message: "Not found" }),
      {
        status: 404,
        headers: { "Content-Type": "application/json" },
      },
    );

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow(ApiError);
  });

  it("middleware throws ApiError on 500 response", async () => {
    const mockResponse = new Response(
      JSON.stringify({ message: "Server error" }),
      {
        status: 500,
        headers: { "Content-Type": "application/json" },
      },
    );

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow(ApiError);
  });

  it("middleware does not throw on successful response", async () => {
    const mockResponse = new Response(JSON.stringify({ data: "success" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).resolves.not.toThrow();
  });

  it("middleware does not throw on 204 no content response", async () => {
    const mockResponse = new Response(null, {
      status: 204,
    });

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).resolves.not.toThrow();
  });

  it("propagates network errors when fetch fails", async () => {
    const networkError = new Error("Network request failed");
    const mockFetch = vi.fn().mockRejectedValue(networkError);
    const client = createBaseClient({ fetchClient: mockFetch });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow("Network request failed");
  });
});

describe("createAuthedClient", () => {
  it("throws error when token is null", () => {
    expect(() => createAuthedClient(null)).toThrow(
      "Auth token must be provided and it must be a valid string",
    );
  });

  it("throws error when token is undefined", () => {
    expect(() => createAuthedClient(undefined)).toThrow(
      "Auth token must be provided and it must be a valid string",
    );
  });

  it("throws error when token is empty string", () => {
    expect(() => createAuthedClient("")).toThrow(
      "Auth token must be provided and it must be a valid string",
    );
  });

  it("creates client with valid token", () => {
    const client = createAuthedClient("valid-token");
    expect(client).toBeDefined();
  });

  it("adds Authorization header to requests", async () => {
    const token = "test-bearer-token";
    let capturedHeaders: Headers | undefined;

    const mockResponse = new Response(JSON.stringify({ data: "success" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    const mockFetch = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (input instanceof Request) {
        capturedHeaders = input.headers;
      }
      return Promise.resolve(mockResponse);
    });

    const client = createAuthedClient(token, { fetchClient: mockFetch });
    await client.GET("/linux/install/installation/status");

    expect(capturedHeaders?.get("Authorization")).toBe(`Bearer ${token}`);
  });

  it("middleware throws ApiError on non-OK responses", async () => {
    const mockResponse = new Response(
      JSON.stringify({ message: "Forbidden" }),
      {
        status: 403,
        headers: { "Content-Type": "application/json" },
      },
    );

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createAuthedClient("valid-token", {
      fetchClient: mockFetch,
    });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).rejects.toThrow(ApiError);
  });

  it("middleware does not throw on successful responses", async () => {
    const mockResponse = new Response(JSON.stringify({ data: "success" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    const mockFetch = vi.fn().mockResolvedValue(mockResponse);
    const client = createAuthedClient("valid-token", {
      fetchClient: mockFetch,
    });

    await expect(
      client.GET("/linux/install/installation/status"),
    ).resolves.not.toThrow();
  });

  it("works with different token formats", () => {
    expect(() => createAuthedClient("simple-token")).not.toThrow();
    expect(() => createAuthedClient("jwt.token.here")).not.toThrow();
    expect(() =>
      createAuthedClient("very-long-token-with-special-chars-!@#"),
    ).not.toThrow();
  });

  it("sets correct Authorization header format", async () => {
    const token = "my-secret-token";
    let capturedAuthHeader = "";

    const mockResponse = new Response(JSON.stringify({ data: "ok" }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });

    const mockFetch = vi.fn().mockImplementation((input: RequestInfo | URL) => {
      if (input instanceof Request) {
        const header = input.headers.get("Authorization");
        if (typeof header === "string") {
          capturedAuthHeader = header;
        }
      }
      return Promise.resolve(mockResponse);
    });

    const client = createAuthedClient(token, { fetchClient: mockFetch });
    await client.GET("/linux/install/installation/status");

    expect(capturedAuthHeader).toBe("Bearer my-secret-token");
    expect(capturedAuthHeader?.startsWith("Bearer ")).toBe(true);
  });

  it("wraps network errors in ApiError with status 0", async () => {
    const networkError = new Error("Network connection timeout");
    const mockFetch = vi.fn().mockRejectedValue(networkError);
    const client = createAuthedClient("valid-token", {
      fetchClient: mockFetch,
    });

    try {
      await client.GET("/linux/install/installation/status");
      expect.fail("Should have thrown an error");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).statusCode).toBe(0);
      expect((error as ApiError).message).toBe(
        "Error: Network connection timeout",
      );
    }
  });

  it("wraps TypeError from fetch in ApiError", async () => {
    const typeError = new TypeError("Failed to fetch");
    const mockFetch = vi.fn().mockRejectedValue(typeError);
    const client = createAuthedClient("valid-token", {
      fetchClient: mockFetch,
    });

    try {
      await client.GET("/linux/install/installation/status");
      expect.fail("Should have thrown an error");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).statusCode).toBe(0);
      expect((error as ApiError).message).toBe("TypeError: Failed to fetch");
    }
  });

  it("wraps generic fetch errors in ApiError", async () => {
    const mockFetch = vi.fn().mockRejectedValue("Connection refused");
    const client = createAuthedClient("valid-token", {
      fetchClient: mockFetch,
    });

    try {
      await client.GET("/linux/install/installation/status");
      expect.fail("Should have thrown an error");
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).statusCode).toBe(0);
      expect((error as ApiError).message).toBe("Connection refused");
    }
  });
});
