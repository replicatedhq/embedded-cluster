import { expect, vi } from "vitest";
import * as matchers from "@testing-library/jest-dom/matchers";
import { act } from "react";
import { faker } from "@faker-js/faker";

expect.extend(matchers);

vi.mock("@/query-client", async () => {
  const queryClient =
    await vi.importActual<typeof import("./query-client")>("./query-client");
  return {
    createQueryClient: queryClient.createQueryClient,
    getQueryClient: vi.fn(),
  };
});

declare global {
  interface Window {
    REPLICATED: {
      WEB_BUILD_VERSION: string;
      API_V1_ENDPOINT: string;
      API_V3_ENDPOINT: string;
      REPLICATED_APP_ENDPOINT: string;
      KURL_ENDPOINT: string;
      ENVIRONMENT: string;
      REGISTRY_ENDPOINT: string;
      PROXY_ENDPOINT: string;
      PUSH_ENDPOINT: string;
    };
  }
}

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
  window.REPLICATED = {
    WEB_BUILD_VERSION: import.meta.env.VITE_BUILD_VERSION,
    API_V1_ENDPOINT: `${import.meta.env.VITE_MARKET_API_V1_ENDPOINT}/v1`,
    API_V3_ENDPOINT: `${import.meta.env.VITE_MARKET_API_V3_ENDPOINT}/v3`,
    REPLICATED_APP_ENDPOINT: import.meta.env.VITE_REPLICATED_APP_ENDPOINT,
    KURL_ENDPOINT: import.meta.env.VITE_KURL_ENDPOINT,
    ENVIRONMENT: import.meta.env.VITE_ENVIRONMENT,
    REGISTRY_ENDPOINT: import.meta.env.VITE_REGISTRY_ENDPOINT,
    PROXY_ENDPOINT: import.meta.env.VITE_PROXY_ENDPOINT,
    PUSH_ENDPOINT: import.meta.env.VITE_PUSH_ENDPOINT,
  };
});

afterEach(async () => {
  // unmock window.location
  Object.defineProperty(window, "location", {
    value: location,
  });

  // flush all pending requests
  await act(() => new Promise((resolve) => setTimeout(resolve)));
  vi.restoreAllMocks();
});
