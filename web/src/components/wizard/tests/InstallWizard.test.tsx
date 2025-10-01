import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import InstallWizard from "../InstallWizard";
import { AppConfig, AppConfigValues } from "../../../types";

const MOCK_APP_CONFIG: AppConfig = {
  groups: [
    {
      name: "settings",
      title: "Settings",
      description: "Configure application settings",
      items: [
        {
          name: "app_name",
          title: "Application Name",
          type: "text",
          value: "Default App",
          default: "Default App",
          help_text: "Enter the name of your application"
        },
        {
          name: "enable_feature",
          title: "Enable Feature",
          type: "bool",
          value: "0",
          default: "0"
        }
      ]
    }
  ]
};

const createMockConfigWithValues = (values: AppConfigValues): AppConfig => {
  const config: AppConfig = JSON.parse(JSON.stringify(MOCK_APP_CONFIG));
  config.groups.forEach((group) => {
    group.items.forEach((item) => {
      if (values[item.name]) {
        item.value = values[item.name].value;
      }
    });
  });
  return config;
};

// Shared state for saved config values across tests
let savedConfigValues: AppConfigValues = {};

const createServer = (target: string) => setupServer(
  // Mock template app config endpoint - applies saved values to config
  http.post(`*/api/${target}/install/app/config/template`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    // Merge saved values with any new template values
    const mergedValues = { ...savedConfigValues, ...body.values };
    const templatedConfig = createMockConfigWithValues(mergedValues);
    return HttpResponse.json(templatedConfig);
  }),

  // Mock kubernetes template app config endpoint for cross-target calls
  http.post(`*/api/kubernetes/install/app/config/template`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    const mergedValues = { ...savedConfigValues, ...body.values };
    const templatedConfig = createMockConfigWithValues(mergedValues);
    return HttpResponse.json(templatedConfig);
  }),

  // Mock config values submission endpoint - saves values
  http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    savedConfigValues = body.values;
    return HttpResponse.json(body);
  }),

  // Mock upgrade template app config endpoint
  http.post(`*/api/${target}/upgrade/app/config/template`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    const mergedValues = { ...savedConfigValues, ...body.values };
    const templatedConfig = createMockConfigWithValues(mergedValues);
    return HttpResponse.json(templatedConfig);
  }),

  // Mock upgrade config values submission endpoint
  http.patch(`*/api/${target}/upgrade/app/config/values`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    savedConfigValues = body.values;
    return HttpResponse.json(body);
  }),

  // Mock installation config endpoint
  http.get(`*/api/${target}/install/installation/config`, () => {
    return HttpResponse.json({
      defaults: { adminConsolePort: 8800 },
      values: {}
    });
  }),

  // Mock upgrade installation config endpoint
  http.get(`*/api/${target}/upgrade/installation/config`, () => {
    return HttpResponse.json({
      defaults: { adminConsolePort: 8800 },
      values: {}
    });
  }),

  // Mock network interfaces endpoint
  http.get(`*/api/console/available-network-interfaces`, () => {
    return HttpResponse.json([]);
  }),

  // Mock app upgrade status endpoint
  http.get(`*/api/${target}/upgrade/app/status`, () => {
    return HttpResponse.json({
      status: { state: 'Pending', description: 'Waiting to start upgrade...' }
    });
  }),

  // Mock app upgrade start endpoint
  http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
    return HttpResponse.json({ success: true });
  })
);

// Helper function to wait for configuration to fully load with config items
const waitForForm = async () => {
  // First wait for the configuration step container to appear
  await waitFor(() => {
    expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
  });

  // Then wait for at least one config item to appear (indicates config has loaded)
  await waitFor(() => {
    const configItems = screen.queryAllByTestId(/^config-item-/);
    expect(configItems.length).toBeGreaterThan(0);
  });
};

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])("InstallWizard - $displayName", ({ target }) => {
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
    server.listen();
  });

  beforeEach(() => {
    // Reset any mocks and saved state
    vi.clearAllMocks();
    savedConfigValues = {};
  });

  afterEach(() => {
    server.resetHandlers();
  });

  afterAll(() => {
    server.close();
  });

  it("preserves saved config values when navigating back from next step", async () => {
    // This test simulates the original bug scenario:
    // 1. User changes value in configuration step
    // 2. User clicks Next (saves config and goes to next step)
    // 3. User clicks Back (returns to configuration step)
    // 4. The value should show the saved value, not the original value

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for configuration step to load
    await waitForForm();

    // Change the app name from "Default App" to "My Custom App"
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "My Custom App" } });

    // Verify the value was changed
    expect(appNameInput).toHaveValue("My Custom App");

    // Click Next to save and go to next step
    const configNextButton = screen.getByTestId("config-next-button");
    fireEvent.click(configNextButton);

    // Wait for next step to load (setup step)
    await waitFor(() => {
      expect(screen.getByTestId(`${target}-setup`)).toBeInTheDocument();
    });

    // Verify the config was saved
    expect(savedConfigValues).toEqual({
      app_name: { value: "My Custom App" }
    });

    // Now navigate back to configuration step
    const backButton = screen.getByTestId(`${target}-setup-button-back`);
    fireEvent.click(backButton);

    // Wait for configuration step to load again with the saved values
    await waitForForm();

    // The correct updated value should be present (now showing in the templated config)
    const appNameInputAfterBack = screen.getByTestId("text-input-app_name");
    expect(appNameInputAfterBack).toHaveValue("My Custom App");
  });

  it("handles multiple field changes during navigation", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for configuration step to load
    await waitForForm();

    // Change multiple fields
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "Multi Field App" } });

    const enableFeatureCheckbox = screen.getByTestId("bool-input-enable_feature");
    fireEvent.click(enableFeatureCheckbox);

    // Verify changes
    expect(appNameInput).toHaveValue("Multi Field App");
    expect(enableFeatureCheckbox).toBeChecked();

    // Click Next to save and go to next step
    const configNextButton = screen.getByTestId("config-next-button");
    fireEvent.click(configNextButton);

    // Wait for next step to load (setup step)
    await waitFor(() => {
      expect(screen.getByTestId(`${target}-setup`)).toBeInTheDocument();
    });

    // Verify both fields were saved
    expect(savedConfigValues).toEqual({
      app_name: { value: "Multi Field App" },
      enable_feature: { value: "1" }
    });

    // Now navigate back to configuration step
    const backButton = screen.getByTestId(`${target}-setup-button-back`);
    fireEvent.click(backButton);

    // Wait for configuration step to reload with saved values
    await waitForForm();

    // Verify both saved values are restored (now showing in the templated config)
    const appNameInputAfterBack = screen.getByTestId("text-input-app_name");
    const enableFeatureCheckboxAfterBack = screen.getByTestId("bool-input-enable_feature");

    expect(appNameInputAfterBack).toHaveValue("Multi Field App");
    expect(enableFeatureCheckboxAfterBack).toBeChecked();
  });

  it("includes configuration step in upgrade mode", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: "upgrade"
      },
    });

    // Wait for configuration step to load
    await waitForForm();

    // Should show configuration step
    expect(screen.getByTestId("configuration-step")).toBeInTheDocument();

    // Setup step should not be accessible in upgrade mode
    expect(screen.queryByTestId(`${target}-setup`)).not.toBeInTheDocument();
  });

  it("keeps all steps for install mode", async () => {
    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: "install"
      },
    });

    // Wait for welcome step
    await waitFor(() => {
      expect(screen.getByText("Welcome")).toBeInTheDocument();
    });

    // In install mode, the welcome step should lead to configuration step
    // We need to wait for the form to load which means configuration step is being shown
    await waitForForm();

    expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
  });
});
