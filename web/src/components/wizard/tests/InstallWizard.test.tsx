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

const createServer = (target: string) => setupServer(
  // Mock app config endpoint
  http.get(`*/api/${target}/install/app/config`, () => {
    return HttpResponse.json(MOCK_APP_CONFIG);
  }),

  // Mock config values fetch endpoint
  http.get(`*/api/${target}/install/app/config/values`, () => {
    return HttpResponse.json({ values: {} });
  }),

  // Mock config values submission endpoint
  http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
    const body = await request.json() as { values: AppConfigValues };
    // Return the submitted values as saved values
    return HttpResponse.json({ values: body.values });
  })
);

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
    // Reset any mocks
    vi.clearAllMocks();
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

    let savedConfigValues: AppConfigValues = {};

    // Mock the server to track what gets saved
    server.use(
      http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: AppConfigValues };
        savedConfigValues = body.values;
        return HttpResponse.json({ values: body.values });
      }),

      // When fetching config values, return the saved values
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: savedConfigValues });
      })
    );

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for configuration step to load
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for form to be ready
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

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

    // Wait for configuration step to load again
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for form to be ready
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

    // The correct updated value should be present
    const appNameInputAfterBack = screen.getByTestId("text-input-app_name");
    expect(appNameInputAfterBack).toHaveValue("My Custom App");
  });

  it("handles multiple field changes during navigation", async () => {
    let savedConfigValues: AppConfigValues = {};

    server.use(
      http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: AppConfigValues };
        savedConfigValues = body.values;
        return HttpResponse.json({ values: body.values });
      }),

      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: savedConfigValues });
      })
    );

    renderWithProviders(<InstallWizard />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for configuration step
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

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

    // Wait for configuration step to reload
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

    // Verify both saved values are restored
    const appNameInputAfterBack = screen.getByTestId("text-input-app_name");
    const enableFeatureCheckboxAfterBack = screen.getByTestId("bool-input-enable_feature");

    expect(appNameInputAfterBack).toHaveValue("Multi Field App");
    expect(enableFeatureCheckboxAfterBack).toBeChecked();
  });
});
