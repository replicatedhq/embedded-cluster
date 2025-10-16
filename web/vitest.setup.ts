import { expect, vi, beforeEach, afterEach } from "vitest";
import * as matchers from "@testing-library/jest-dom/matchers";
import { act, cleanup } from "@testing-library/react";
import { faker } from "@faker-js/faker";
import "./src/test/setup";

expect.extend(matchers);

// Mock URL for test environment
const originalURL = globalThis.URL;
class URLWithMocks extends originalURL {
  static createObjectURL = vi.fn(() => "blob:test-url");
  static revokeObjectURL = vi.fn();
}

// Set up global URL mock
vi.stubGlobal("URL", URLWithMocks);

vi.mock("@/query-client", async () => {
  const queryClient =
    await vi.importActual<typeof import("./src/query-client")>(
      ".src/query-client",
    );
  return {
    createQueryClient: queryClient.createQueryClient,
    getQueryClient: vi.fn(),
  };
});

const location = window.location;

beforeEach(() => {
  // mock window.location
  const url = faker.internet.url();
  const mockLocation = new URL(url);

  // Add reload method to mock location
  Object.defineProperty(mockLocation, 'reload', {
    configurable: true,
    writable: true,
    value: vi.fn(),
  });

  Object.defineProperties(window, {
    location: {
      value: mockLocation,
    },
  });
  window.location.href = url;
});

afterEach(async () => {
  // Clean up DOM
  cleanup();

  // unmock window.location
  Object.defineProperty(window, "location", {
    value: location,
  });

  // Clear sessionStorage after each test
  sessionStorage.clear();

  // flush all pending requests
  await act(() => new Promise((resolve) => setTimeout(resolve)));
  vi.restoreAllMocks();
});
