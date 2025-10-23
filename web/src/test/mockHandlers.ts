import { http, HttpResponse } from 'msw';

/**
 * Type-safe MSW handler registry for v3 API endpoints.
 * Centralizes all mock API handlers to prevent duplication across test files.
 *
 * When backend endpoints change, update this file instead of searching through 17+ test files.
 *
 * Usage:
 * ```typescript
 * import { mockHandlers } from '../../test/mockHandlers';
 *
 * const server = setupServer(
 *   mockHandlers.installation.getStatus('Succeeded'),
 *   mockHandlers.preflights.host.run(true),
 * );
 * ```
 */

// ============================================================================
// Type Definitions
// ============================================================================

export type Target = 'linux' | 'kubernetes';
export type Mode = 'install' | 'upgrade';
export type State = 'Running' | 'Succeeded' | 'Failed' | 'Pending';

export interface Status {
  state: State;
  description?: string;
  lastUpdated?: string;
}

export interface PreflightOutput {
  fail: Array<{ title: string; message: string; strict?: boolean }>;
  warn: Array<{ title: string; message: string; strict?: boolean }>;
  pass: Array<{ title: string; message: string; strict?: boolean }>;
}

export interface PreflightStatusResponse {
  titles: string[];
  status: Status;
  output: PreflightOutput;
  allowIgnoreHostPreflights?: boolean;
  allowIgnoreAppPreflights?: boolean;
  hasStrictAppPreflightFailures?: boolean;
}

export interface AppConfigGroup {
  name: string;
  title: string;
  items: Array<{
    name: string;
    title: string;
    type: string;
    value?: string | number | boolean;
    default?: string | number | boolean;
    required?: boolean;
    validation?: {
      regex?: {
        pattern: string;
        message: string;
      };
    };
  }>;
}

export interface AppConfigResponse {
  groups: AppConfigGroup[];
}

export interface NetworkInterface {
  name: string;
  addresses: string[];
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Creates a path for API endpoints with target/mode parameters
 */
function apiPath(path: string, target: Target = 'linux', mode: Mode = 'install'): string {
  return `*/api/${target}/${mode}${path}`;
}

/**
 * Creates a successful JSON response
 */
function success(data: Record<string, unknown> = { success: true }) {
  return HttpResponse.json(data);
}

/**
 * Creates an error response
 */
function error(statusCode: number, message: string) {
  return HttpResponse.json({ message }, { status: statusCode });
}

/**
 * Verifies auth header and returns 401 if invalid
 */
function verifyAuthHeader(request: Request): Response | null {
  const authHeader = request.headers.get("Authorization");
  if (!authHeader || !authHeader.startsWith("Bearer ")) {
    return new HttpResponse(null, { status: 401 });
  }
  return null;
}

/**
 * Creates a handler with auth verification
 */
function withAuth(handler: () => Response | Promise<Response>) {
  return async (context: { request: Request }) => {
    const authError = verifyAuthHeader(context.request);
    if (authError) return authError;
    return handler();
  };
}

/**
 * Creates a handler with auth verification and request body capture
 */
function withAuthAndCapture(
  handler: (body: Record<string, unknown>, headers: Headers) => Response | Promise<Response>
) {
  return async (context: { request: Request }) => {
    const authError = verifyAuthHeader(context.request);
    if (authError) return authError;
    
    let body: Record<string, unknown> = {};
    try {
      body = await context.request.json() as Record<string, unknown>;
    } catch {
      // If there's no body or it's not JSON, use empty object
      body = {};
    }
    const headers = context.request.headers;
    return handler(body, headers);
  };
}

// ============================================================================
// Mock Handlers Registry
// ============================================================================

export const mockHandlers = {
  /**
   * Authentication endpoints
   */
  auth: {
    /**
     * POST /api/auth/login
     * @param shouldSucceed - Whether login should succeed
     * @param token - Optional custom token to return
     */
    login: (shouldSucceed: boolean = true, token: string = 'test-token') =>
      http.post('*/api/auth/login', async () => {
        if (shouldSucceed) {
          return HttpResponse.json({ token });
        }
        return error(401, 'Invalid credentials');
      }),
  },

  /**
   * Health check endpoint
   */
  health: {
    /**
     * GET /api/health
     * @param healthy - Whether the service is healthy
     */
    check: (healthy: boolean = true) =>
      http.get('*/api/health', () => {
        if (healthy) {
          return success({ status: 'ok' });
        }
        return error(503, 'Service unavailable');
      }),
  },

  /**
   * Console endpoints
   */
  console: {
    /**
     * GET /api/console/available-network-interfaces
     * @param interfaces - Array of network interfaces to return
     */
    getNetworkInterfaces: (
      interfaces: NetworkInterface[] = [
        { name: 'eth0', addresses: ['192.168.1.100'] },
      ]
    ) =>
      http.get('*/api/console/available-network-interfaces', () => {
        return HttpResponse.json({ interfaces });
      }),
  },

  /**
   * Installation endpoints
   */
  installation: {
    /**
     * GET /api/{target}/{mode}/installation/config
     * @param config - Configuration response (full object with values/defaults/resolved, or just values for simple cases)
     * @param target - Installation target
     * @param mode - Operation mode
     */
    getConfig: (
      config:
        | Record<string, unknown>
        | { values?: Record<string, unknown>; defaults?: Record<string, unknown>; resolved?: Record<string, unknown> }
        | { error: { statusCode: number; message: string } }
        | { networkError: true }
        = {},
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.get(apiPath('/installation/config', target, mode), () => {
        // Handle network error
        if ('networkError' in config) {
          return HttpResponse.error();
        }

        // Handle API error
        if ('error' in config) {
          const errorConfig = config as { error: { statusCode: number; message: string } };
          return HttpResponse.json(
            { message: errorConfig.error.message },
            { status: errorConfig.error.statusCode }
          );
        }

        // Check if this is a full config response with values/defaults/resolved
        if ('values' in config || 'defaults' in config || 'resolved' in config) {
          return HttpResponse.json(config);
        }

        // Otherwise, treat it as simple values and wrap it
        return HttpResponse.json({
          values: config,
          defaults: {},
          resolved: config,
        });
      }),

    /**
     * POST /api/{target}/{mode}/installation/configure
     * @param response - Configuration response (boolean for success/failure, object for custom errors/validation)
     * @param target - Installation target
     * @param mode - Operation mode
     */
    configure: (
      response:
        | boolean
        | {
            error?: { message: string; fields?: Array<{ field: string; message: string }> };
            captureRequest?: (body: Record<string, unknown>, headers: Headers) => void;
          } = true,
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.post(apiPath('/installation/configure', target, mode), withAuthAndCapture(async (body, headers) => {
        // Handle captureRequest if provided
        if (typeof response === 'object' && response.captureRequest) {
          response.captureRequest(body, headers);
        }

        // Handle boolean shorthand
        if (typeof response === 'boolean') {
          return response ? success() : error(400, 'Configuration failed');
        }

        // Handle custom error with optional field validation errors
        if (response.error) {
          const errorBody: Record<string, unknown> = {
            message: response.error.message
          };
          if (response.error.fields && response.error.fields.length > 0) {
            errorBody.errors = response.error.fields;
          }
          return HttpResponse.json(errorBody, { status: 400 });
        }

        return success();
      })),

    /**
     * GET /api/linux/install/installation/status
     * @param response - Installation status response (shorthand or detailed)
     */
    getStatus: (
      response:
        | State
        | {
            state: State;
            description?: string;
            sequence?: Array<{ state: State; description?: string }>;
            counter?: { callCount: number };
          } = 'Succeeded'
    ) =>
      http.get('*/api/linux/install/installation/status', () => {
        // Handle string shorthand
        if (typeof response === 'string') {
          return HttpResponse.json({
            state: response,
            description: 'Installation initialized',
            lastUpdated: new Date().toISOString(),
          });
        }

        // Handle sequence responses
        if (response.sequence && response.counter) {
          const callIndex = response.counter.callCount;
          response.counter.callCount++;
          const current = response.sequence[Math.min(callIndex, response.sequence.length - 1)];
          return HttpResponse.json({
            state: current.state,
            description: current.description || '',
            lastUpdated: new Date().toISOString(),
          });
        }

        // Handle single response
        return HttpResponse.json({
          state: response.state,
          description: response.description || 'Installation initialized',
          lastUpdated: new Date().toISOString(),
        });
      }),
  },

  /**
   * Host preflight check endpoints
   */
  preflights: {
    host: {
      /**
       * GET /api/linux/install/host-preflights/status
       * @param response - Full preflight status response
       */
      getStatus: (response: Partial<PreflightStatusResponse>) =>
        http.get('*/api/linux/install/host-preflights/status', withAuth(() => {
          const defaultResponse: PreflightStatusResponse = {
            titles: ['Host Check'],
            status: { state: 'Succeeded' },
            output: { fail: [], warn: [], pass: [] },
            allowIgnoreHostPreflights: false,
            ...response,
          };
          return HttpResponse.json(defaultResponse);
        })),

      /**
       * POST /api/linux/install/host-preflights/run
       * @param shouldSucceed - Whether preflight run should succeed
       */
      run: (shouldSucceed: boolean = true) =>
        http.post('*/api/linux/install/host-preflights/run', withAuth(() => {
          return shouldSucceed ? success() : error(500, 'Failed to run preflights');
        })),

      /**
       * GET /api/linux/install/host-preflights/output
       * @param output - Preflight output data
       */
      getOutput: (output: PreflightOutput) =>
        http.get('*/api/linux/install/host-preflights/output', () => {
          return HttpResponse.json(output);
        }),
    },

    /**
     * App preflight check endpoints (supports both linux/kubernetes and install/upgrade)
     */
    app: {
      /**
       * GET /api/{target}/{mode}/app-preflights/status
       * @param response - Full preflight status response
       * @param target - Installation target (linux or kubernetes)
       * @param mode - Operation mode (install or upgrade)
       */
      getStatus: (
        response: Partial<PreflightStatusResponse>,
        target: Target = 'linux',
        mode: Mode = 'install'
      ) =>
        http.get(apiPath('/app-preflights/status', target, mode), withAuth(() => {
          const defaultResponse: PreflightStatusResponse = {
            titles: ['App Check'],
            status: { state: 'Succeeded' },
            output: { fail: [], warn: [], pass: [] },
            ...response,
          };
          return HttpResponse.json(defaultResponse);
        })),

      /**
       * POST /api/{target}/{mode}/app-preflights/run
       * @param shouldSucceed - Whether preflight run should succeed
       * @param target - Installation target
       * @param mode - Operation mode
       */
      run: (shouldSucceed: boolean = true, target: Target = 'linux', mode: Mode = 'install') =>
        http.post(apiPath('/app-preflights/run', target, mode), withAuth(() => {
          return shouldSucceed ? success() : error(500, 'Failed to run app preflights');
        })),
    },
  },

  /**
   * Infrastructure setup endpoints
   */
  infra: {
    /**
     * POST /api/{target}/{mode}/infra/setup (or /api/{target}/{mode}/infra/upgrade)
     * @param shouldSucceed - Whether infra setup should succeed
     * @param target - Installation target
     * @param mode - Operation mode
     * @param captureRequest - Optional callback to capture and verify request body
     */
    setup: (
      shouldSucceed: boolean = true,
      target: Target = 'linux',
      mode: Mode = 'install',
      captureRequest?: (body: Record<string, unknown>) => void
    ) =>
      http.post(apiPath(mode === 'install' ? '/infra/setup' : '/infra/upgrade', target, mode), withAuthAndCapture(async (body) => {
        if (captureRequest) {
          captureRequest(body);
        }

        return shouldSucceed ? success() : error(500, 'Infrastructure setup failed');
      })),

    /**
     * GET /api/{target}/{mode}/infra/status
     * @param response - Infrastructure status response
     * @param target - Installation target
     * @param mode - Operation mode
     */
    getStatus: (
      response: {
        state: State;
        description?: string;
        components?: Array<{ name: string; status: { state: State } }>;
        logs?: string;
      } = { state: 'Succeeded' },
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.get(apiPath('/infra/status', target, mode), withAuth(() => {
        const defaultResponse = {
          status: { state: response.state, description: response.description },
          components: response.components || [],
          logs: response.logs
        };
        return HttpResponse.json(defaultResponse);
      })),
  },

  /**
   * App configuration endpoints (supports both linux/kubernetes and install/upgrade)
   */
  appConfig: {
    /**
     * POST /api/{target}/{mode}/app/config/template
     * @param config - App configuration template, function to process request, or error object
     * @param target - Installation target
     * @param mode - Operation mode
     */
    getTemplate: (
      config: Partial<AppConfigResponse> | ((body: Record<string, unknown>) => Partial<AppConfigResponse>) | { error: { message: string; statusCode?: number } } = {},
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.post(apiPath('/app/config/template', target, mode), async ({ request }) => {
        // Handle error response
        if (typeof config === 'object' && 'error' in config && config.error) {
          return HttpResponse.json(
            { message: config.error.message },
            { status: config.error.statusCode || 500 }
          );
        }

        let responseConfig: Partial<AppConfigResponse>;

        if (typeof config === 'function') {
          const body = await request.json() as Record<string, unknown>;
          responseConfig = config(body);
        } else {
          // TypeScript now knows config is Partial<AppConfigResponse>
          responseConfig = config as Partial<AppConfigResponse>;
        }

        const defaultConfig: AppConfigResponse = {
          groups: [],
          ...responseConfig,
        };
        return HttpResponse.json(defaultConfig);
      }),

    /**
     * PATCH /api/{target}/{mode}/app/config/values
     * @param response - true = success (echo body), false = generic error, object = custom error with optional field validation errors
     * @param target - Installation target
     * @param mode - Operation mode
     * @param captureRequest - Optional callback to capture and verify request body
     */
    updateValues: (
      response: boolean | { error?: { message: string; errors?: Array<{ field: string; message: string }> }; captureRequest?: (body: Record<string, unknown>, headers: Headers) => void } = true,
      target: Target = 'linux',
      mode: Mode = 'install',
      captureRequest?: (body: Record<string, unknown>, headers: Headers) => void
    ) =>
      http.patch(apiPath('/app/config/values', target, mode), withAuthAndCapture(async (body, headers) => {
        // Handle request capture - support both old and new patterns
        const requestCapture = typeof response === 'object' ? response.captureRequest : captureRequest;
        if (requestCapture) {
          requestCapture(body, headers);
        }

        // Handle boolean shorthand
        if (typeof response === 'boolean') {
          return response ? HttpResponse.json(body) : error(400, 'Failed to update config values');
        }

        // Handle custom error with optional field validation errors
        if (response.error) {
          const errorBody: Record<string, unknown> = {
            message: response.error.message,
            statusCode: 400
          };
          if (response.error.errors && response.error.errors.length > 0) {
            errorBody.errors = response.error.errors;
          }
          return HttpResponse.json(errorBody, { status: 400 });
        }

        // Default success - echo the request body back (matches API behavior)
        return HttpResponse.json(body);
      })),
  },

  /**
   * App installation/upgrade endpoints
   */
  app: {
    /**
     * GET /api/{target}/{mode}/app/status
     * @param response - App status response configuration
     * @param target - Installation target
     * @param mode - Operation mode
     */
    getStatus: (
      response:
        | State
        | {
            state?: State;
            description?: string;
            empty?: boolean;
            error?: { statusCode: number; message: string };
            networkError?: boolean;
          } = 'Succeeded',
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.get(apiPath('/app/status', target, mode), withAuth(() => {
        // Handle string shorthand for state
        if (typeof response === 'string') {
          return HttpResponse.json({ status: { state: response } });
        }

        // Handle network error
        if (response.networkError) {
          return HttpResponse.error();
        }

        // Handle API error
        if (response.error) {
          return HttpResponse.json(
            {
              statusCode: response.error.statusCode,
              message: response.error.message
            },
            { status: response.error.statusCode }
          );
        }

        // Handle empty response
        if (response.empty) {
          return HttpResponse.json({});
        }

        // Handle normal response with optional description
        return HttpResponse.json({
          status: {
            state: response.state || 'Running',
            ...(response.description && { description: response.description })
          }
        });
      })),

    /**
     * POST /api/{target}/{mode}/app/{mode}
     * Starts app installation or upgrade
     * @param response - Whether operation should succeed or error details
     * @param target - Installation target
     * @param mode - Operation mode (install or upgrade)
     */
    start: (
      response: boolean | { error?: { statusCode: number; message: string }; networkError?: boolean; captureRequest?: (body: Record<string, unknown>) => void } = true,
      target: Target = 'linux',
      mode: Mode = 'install'
    ) =>
      http.post(apiPath(`/app/${mode}`, target, mode), withAuthAndCapture(async (body) => {
        // Handle captureRequest if provided
        if (typeof response === 'object' && response.captureRequest) {
          response.captureRequest(body);
        }

        // Handle boolean shorthand
        if (typeof response === 'boolean') {
          return response ? success() : error(500, `Failed to ${mode} app`);
        }

        // Handle network error
        if (response.networkError) {
          return HttpResponse.error();
        }

        // Handle API error
        if (response.error) {
          return HttpResponse.json(
            {
              statusCode: response.error.statusCode,
              message: response.error.message
            },
            { status: response.error.statusCode }
          );
        }

        return success();
      })),
  },
};

/**
 * Helper to create custom handlers with common patterns
 */
export const createHandler = {
  /**
   * Creates a handler that returns different responses on subsequent calls
   * @param responses - Array of responses to return in order
   * @returns MSW handler
   */
  sequence: (path: string, responses: Record<string, unknown>[]) => {
    let callCount = 0;
    return http.get(path, () => {
      const response = responses[Math.min(callCount, responses.length - 1)];
      callCount++;
      return HttpResponse.json(response);
    });
  },

  /**
   * Creates a handler that simulates a delayed response
   * @param path - API path
   * @param response - Response data
   * @param delayMs - Delay in milliseconds
   */
  delayed: (path: string, response: Record<string, unknown>, delayMs: number) => {
    return http.get(path, async () => {
      await new Promise((resolve) => setTimeout(resolve, delayMs));
      return HttpResponse.json(response);
    });
  },

  /**
   * Creates a handler that tracks call count and returns the same response
   * Useful for testing polling behavior
   * @param path - API path
   * @param response - Response data to return
   * @param callCounter - Object with callCount property that will be incremented
   * @returns MSW handler
   */
  withCallCounter: (
    path: string,
    response: Record<string, unknown>,
    callCounter: { callCount: number }
  ) => {
    return http.get(path, () => {
      callCounter.callCount++;
      return HttpResponse.json(response);
    });
  },

  /**
   * Creates a login handler that returns error response and tracks call count
   * Useful for testing retry behavior
   * @param statusCode - HTTP status code (401, 500, 503, etc.)
   * @param message - Error message
   * @param callCounter - Object with callCount property that will be incremented
   * @returns MSW handler
   */
  loginWithCounter: (
    statusCode: number,
    message: string,
    callCounter: { callCount: number }
  ) => {
    return http.post('*/api/auth/login', () => {
      callCounter.callCount++;
      return new HttpResponse(JSON.stringify({ message }), {
        status: statusCode,
        headers: { 'Content-Type': 'application/json' },
      });
    });
  },

  /**
   * Creates a login handler that returns different responses on subsequent calls
   * Useful for testing retry-then-success scenarios
   * @param callCounter - Object with callCount property that will be incremented
   * @param firstStatusCode - Status code for first request
   * @param firstMessage - Message for first request
   * @param secondResponse - Response for second request (success)
   * @returns MSW handler
   */
  loginRetrySuccess: (
    callCounter: { callCount: number },
    firstStatusCode: number,
    firstMessage: string,
    secondResponse: { token: string } = { token: 'mock-token' }
  ) => {
    return http.post('*/api/auth/login', () => {
      callCounter.callCount++;
      if (callCounter.callCount === 1) {
        return new HttpResponse(JSON.stringify({ message: firstMessage }), {
          status: firstStatusCode,
          headers: { 'Content-Type': 'application/json' },
        });
      } else {
        return new HttpResponse(JSON.stringify(secondResponse), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
    });
  },

  /**
   * Creates a health check handler that fails first then succeeds
   * Useful for testing retry/reconnection behavior
   * @param callCounter - Object with callCount property that will be incremented
   * @returns MSW handler
   */
  healthRetrySuccess: (callCounter: { callCount: number }) => {
    return http.get('*/api/health', () => {
      callCounter.callCount++;

      // Fail first time, succeed on retry
      if (callCounter.callCount === 1) {
        return HttpResponse.error();
      }

      return new HttpResponse(JSON.stringify({ status: 'ok' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
  },
};

/**
 * Common preflight response presets
 */
export const preflightPresets = {
  success: (): PreflightOutput => ({
    fail: [],
    warn: [],
    pass: [{ title: 'Disk Space', message: 'Sufficient disk space available' }],
  }),

  failed: (message: string = 'Not enough disk space available'): PreflightOutput => ({
    fail: [{ title: 'Disk Space', message }],
    warn: [],
    pass: [],
  }),

  warning: (message: string = 'Low disk space warning'): PreflightOutput => ({
    fail: [],
    warn: [{ title: 'Disk Space', message }],
    pass: [],
  }),

  mixed: (): PreflightOutput => ({
    fail: [{ title: 'Memory', message: 'Insufficient memory' }],
    warn: [{ title: 'CPU', message: 'CPU usage high' }],
    pass: [{ title: 'Disk Space', message: 'Sufficient disk space' }],
  }),
};

/**
 * App config presets for common test scenarios
 */
export const appConfigPresets = {
  simple: (): AppConfigResponse => ({
    groups: [
      {
        name: 'settings',
        title: 'Settings',
        items: [
          {
            name: 'hostname',
            title: 'Hostname',
            type: 'text',
            value: 'test.example.com',
            required: true,
          },
        ],
      },
    ],
  }),

  withValidation: (): AppConfigResponse => ({
    groups: [
      {
        name: 'settings',
        title: 'Settings',
        items: [
          {
            name: 'email',
            title: 'Email',
            type: 'text',
            required: true,
            validation: {
              regex: {
                pattern: '^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$',
                message: 'Must be a valid email address',
              },
            },
          },
        ],
      },
    ],
  }),

  multiGroup: (): AppConfigResponse => ({
    groups: [
      {
        name: 'basic',
        title: 'Basic Settings',
        items: [
          {
            name: 'hostname',
            title: 'Hostname',
            type: 'text',
            value: 'test.example.com',
          },
        ],
      },
      {
        name: 'advanced',
        title: 'Advanced Settings',
        items: [
          {
            name: 'debug',
            title: 'Debug Mode',
            type: 'bool',
            value: false,
          },
        ],
      },
    ],
  }),
};
