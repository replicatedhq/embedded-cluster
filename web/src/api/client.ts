import createClient from "openapi-fetch";
import type { paths } from "../types/api";
import { ApiError } from "./error";

import { InstallationTarget } from "../types/installation-target";
import { WizardMode } from "../types/wizard-mode";

// Helper constant used for type inference for getApiBasePath function
const API_BASE_PATHS = {
  kubernetes: {
    install: "/kubernetes/install",
    upgrade: "/kubernetes/upgrade",
  },
  linux: {
    install: "/linux/install",
    upgrade: "/linux/upgrade",
  },
} as const;

// Helper constant used for type inference for getAppInstallPath function
const APP_INSTALL_PATHS = {
  kubernetes: {
    install: "/kubernetes/install/app/install",
    upgrade: "/kubernetes/upgrade/app/upgrade",
  },
  linux: {
    install: "/linux/install/app/install",
    upgrade: "/linux/upgrade/app/upgrade",
  },
} as const;

/**
 * Returns base path for wizard operations
 * Dynamically builds the path based on installation target and wizard mode
 */
export function getWizardBasePath<
  T extends InstallationTarget,
  M extends WizardMode,
>(target: T, mode: M): (typeof API_BASE_PATHS)[T][M] {
  return API_BASE_PATHS[target][mode];
}

/**
 * Returns API base path
 * Dynamically builds the path based on installation target and wizard mode
 */
export function getApiBasePath<
  T extends InstallationTarget,
  M extends WizardMode,
>(target: T, mode: M): `/api${(typeof API_BASE_PATHS)[T][M]}` {
  return `/api${getWizardBasePath(target, mode)}` as `/api${(typeof API_BASE_PATHS)[T][M]}`;
}

/**
 * Returns the app install path
 * Dynamically builds the path based on installation target and wizard mode
 */
export function getAppInstallPath<
  T extends InstallationTarget,
  M extends WizardMode,
>(target: T, mode: M): (typeof APP_INSTALL_PATHS)[T][M] {
  return APP_INSTALL_PATHS[target][mode];
}

/**
 * Helper to get the base URL
 * In tests (Node.js), we need absolute URLs because Node's fetch doesn't support relative URLs
 */
function getBaseUrl(): string {
  // In test environment (vitest), use absolute URL
  if (typeof window !== "undefined" && window.location) {
    const origin =
      window.location.origin ||
      `${window.location.protocol}//${window.location.host}`;
    return `${origin}/api`;
  }

  // Fallback, use relative URL
  return "/api";
}

/**
 * Default API client configured with base URL
 * Use this for unauthenticated requests or when auth is handled externally
 */
export const apiClient = createClient<paths>({
  baseUrl: getBaseUrl(),
});

// Middleware: Automatic error handling
apiClient.use({
  async onResponse({ response }) {
    if (!response.ok) {
      throw await ApiError.fromResponse(
        response,
        `API request failed: ${response.status}`,
      );
    }
    return response;
  },
});

/**
 * Creates an API client with authentication token
 * Use this factory function when you need authenticated requests
 *
 * @param token - Bearer token for authentication (must be a non-empty string)
 * @returns Configured API client with auth middleware
 * @throws {Error} If token is null, undefined, or an empty string
 *
 * @example
 * const client = createAuthedClient(token);
 * const { data, error } = await client.POST('/auth/login', {
 *   body: { password: 'secret' }
 * });
 */
export function createAuthedClient(token: string | null | undefined) {
  if (typeof token !== "string" || token == "") {
    throw new Error(
      "Auth token must be provided and it must be a valid string",
    );
  }

  const client = createClient<paths>({
    baseUrl: getBaseUrl(),
  });

  // Add auth middleware
  client.use({
    onRequest({ request }) {
      request.headers.set("Authorization", `Bearer ${token}`);
      return request;
    },
    async onResponse({ response }) {
      if (!response.ok) {
        throw await ApiError.fromResponse(
          response,
          `API request failed: ${response.status}`,
        );
      }
      return response;
    },
  });

  return client;
}
