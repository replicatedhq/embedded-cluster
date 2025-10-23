import { describe, it, expect } from "vitest";
import { ApiError, convertToFieldErrors } from "./error";

describe("ApiError", () => {
  describe("constructor", () => {
    it("creates error with correct status code", () => {
      const error = new ApiError(404, "Not found");
      expect(error.statusCode).toBe(404);
    });

    it("creates error with correct message", () => {
      const error = new ApiError(500, "Internal server error");
      expect(error.message).toBe("Internal server error");
    });

    it("sets correct error name", () => {
      const error = new ApiError(400, "Bad request");
      expect(error.name).toBe("ApiError");
    });

    it("details field defaults to empty string", () => {
      const error = new ApiError(403, "Forbidden");
      expect(error.details).toBe("");
    });

    it("extends Error class", () => {
      const error = new ApiError(401, "Unauthorized");
      expect(error instanceof Error).toBe(true);
      expect(error instanceof ApiError).toBe(true);
    });
  });

  describe("fromResponse", () => {
    it("successfully parses JSON response body and extracts error details", async () => {
      const mockResponse = {
        status: 400,
        json: async () => ({
          message: "Validation failed",
          errors: [],
        }),
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "Request failed");

      expect(error.statusCode).toBe(400);
      expect(error.message).toBe("Request failed");
      expect(error.details).toBe("Validation failed");
    });

    it("sets details from response body message field", async () => {
      const mockResponse = {
        status: 422,
        json: async () => ({
          message: "Invalid input provided",
        }),
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "Failed");
      expect(error.details).toBe("Invalid input provided");
    });

    it("converts API errors to field errors", async () => {
      const mockResponse = {
        status: 400,
        json: async () => ({
          message: "Multiple errors",
          errors: [
            { field: "email", message: "Invalid email format" },
            { field: "password", message: "Password too short" },
          ],
        }),
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "Failed");

      expect(error.fieldErrors).toEqual([
        { field: "email", message: "Invalid email format" },
        { field: "password", message: "Password too short" },
      ]);
    });

    it("handles JSON parse errors gracefully", async () => {
      const mockResponse = {
        status: 500,
        json: async (): Promise<unknown> => {
          throw new Error("Invalid JSON");
        },
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "Server error");

      expect(error.statusCode).toBe(500);
      expect(error.message).toBe("Server error");
      expect(error.details).toBe("");
      expect(error.fieldErrors).toBeUndefined();
    });

    it("works with Response objects that have no body", async () => {
      const mockResponse = {
        status: 204,
        json: async () => ({}),
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "No content");

      expect(error.statusCode).toBe(204);
      expect(error.message).toBe("No content");
    });

    it("works with malformed JSON responses", async () => {
      const mockResponse = {
        status: 502,
        json: async (): Promise<unknown> => {
          throw new SyntaxError("Unexpected token");
        },
      } as Response;

      const error = await ApiError.fromResponse(mockResponse, "Bad gateway");

      expect(error.statusCode).toBe(502);
      expect(error.details).toBe("");
    });

    it("preserves status code and message when JSON parsing fails", async () => {
      const mockResponse = {
        status: 503,
        json: async (): Promise<unknown> => {
          throw new Error("Parse error");
        },
      } as Response;

      const error = await ApiError.fromResponse(
        mockResponse,
        "Service unavailable",
      );

      expect(error.statusCode).toBe(503);
      expect(error.message).toBe("Service unavailable");
    });
  });
});
