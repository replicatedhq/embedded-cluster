import { expect, vi, beforeEach, afterEach } from "vitest";
import * as matchers from "@testing-library/jest-dom/matchers";
import { act, cleanup } from "@testing-library/react";
import { faker } from "@faker-js/faker";

expect.extend(matchers);

// Mock URL for test environment
const originalURL = globalThis.URL;
class URLWithMocks extends originalURL {
  static createObjectURL = vi.fn(() => 'blob:test-url');
  static revokeObjectURL = vi.fn();
}

// Set up global URL mock
vi.stubGlobal('URL', URLWithMocks);

vi.mock("@/query-client", async () => {
  const queryClient =
    await vi.importActual<typeof import("./src/query-client")>("./query-client");
  return {
    createQueryClient: queryClient.createQueryClient,
    getQueryClient: vi.fn(),
  };
});

const location = window.location;

beforeEach(() => {
  // mock window.location
  const url = faker.internet.url();
  Object.defineProperties(window, {
    location: {
      value: new URL(url),
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

  // flush all pending requests
  await act(() => new Promise((resolve) => setTimeout(resolve)));
  vi.restoreAllMocks();
});
