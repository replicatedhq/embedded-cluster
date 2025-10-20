import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ConfigurationStep from "../config/ConfigurationStep.tsx";
import { AppConfig, AppConfigGroup, AppConfigItem, AppConfigValues } from "../../../types";
import { mockHandlers, type Target, type Mode } from "../../../test/mockHandlers.ts";
import '@testing-library/jest-dom/vitest';

// Mock the debounced fetch to remove timing issues in tests
vi.mock("../../../utils/debouncedFetch", () => ({
  useDebouncedFetch: () => ({
    debouncedFetch: vi.fn().mockImplementation((url: string, options: RequestInit = {}) => {
      return fetch(url, options);
    }),
    cleanup: vi.fn()
  })
}));

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
          value: "My App",
          default: "Default App",
          help_text: "Enter the name of your application"
        },
        {
          name: "description",
          title: "Application Description",
          type: "textarea",
          value: "This is my application\nIt does amazing things",
          default: "Enter description here..."
        },
        {
          name: "enable_feature",
          title: "Enable Feature",
          type: "bool",
          value: "0",
          default: "0"
        },
        {
          name: "auth_type",
          title: "Authentication Type",
          type: "radio",
          value: "auth_type_password",
          default: "auth_type_anonymous",
          items: [
            {
              name: "auth_type_anonymous",
              title: "Anonymous"
            },
            {
              name: "auth_type_password",
              title: "Password"
            }
          ]
        },
        {
          name: "info_label",
          title: "Visit our documentation at https://docs.example.com for more information.",
          type: "label"
        },
        {
          name: "markdown_label",
          title: "This is **bold** text and *italic* text with a [link](https://example.com).",
          type: "label"
        },
        {
          name: "ssl_certificate",
          title: "SSL Certificate",
          type: "file",
          help_text: "Provide your SSL certificate file"
        }
      ]
    },
    {
      name: "database",
      title: "Database",
      description: "Configure database settings",
      items: [
        {
          name: "db_host",
          title: "Database Host",
          type: "text",
          value: "localhost",
          default: "localhost"
        },
        {
          name: "db_config",
          title: "Database Configuration",
          type: "textarea",
          value: "",
          default: "# Database configuration\nhost: localhost\nport: 5432"
        },
        {
          name: "db_warning",
          title: "**Important**: Changing database settings may require application restart. See our guide at https://help.example.com/database-config for details.",
          type: "label"
        },
      ]
    }
  ]
};

const createMockConfigWithValues = (values: AppConfigValues): AppConfig => {
  const config: AppConfig = JSON.parse(JSON.stringify(MOCK_APP_CONFIG));
  config.groups.forEach((group: AppConfigGroup) => {
    group.items.forEach((item: AppConfigItem) => {
      if (values[item.name]) {
        item.value = values[item.name].value;
      }
    });
  });
  return config;
};

const createServer = (target: Target, mode: Mode = 'install') => setupServer(
  // Mock template app config endpoint - dynamically applies values
  mockHandlers.appConfig.getTemplate((body: Record<string, unknown>) => {
    const values = body.values as AppConfigValues;
    return createMockConfigWithValues(values);
  }, target, mode),

  // Mock config values submission endpoint
  mockHandlers.appConfig.updateValues(true, target, mode)
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
  { target: "kubernetes" as const, mode: "install" as const, displayName: "Kubernetes Install" },
  { target: "linux" as const, mode: "install" as const, displayName: "Linux Install" },
  { target: "kubernetes" as const, mode: "upgrade" as const, displayName: "Kubernetes Upgrade" },
  { target: "linux" as const, mode: "upgrade" as const, displayName: "Linux Upgrade" }
])("ConfigurationStep - $displayName", ({ target, mode }) => {
  const mockOnNext = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target, mode);
    server.listen();
  });

  beforeEach(() => {
    // No need to set localStorage token anymore as it's handled by the test setup
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("renders the configuration form with card, title, and next button", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Check initial loading state
    expect(screen.getByTestId("configuration-step-loading")).toBeInTheDocument();

    // Wait for config to load
    await waitForForm();

    // Check for title and description
    await screen.findByText("Configuration");
    await screen.findByText("Configure your My App installation by providing the information below.");

    // Check that tabs are rendered
    expect(screen.getByTestId("config-tab-settings")).toBeInTheDocument();
    expect(screen.getByTestId("config-tab-database")).toBeInTheDocument();

    // Check that form fields are rendered for the active tab
    expect(screen.getByTestId("config-item-app_name")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-description")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-enable_feature")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-auth_type")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-ssl_certificate")).toBeInTheDocument();

    // Check that the database tab is not rendered
    expect(screen.queryByTestId("config-item-db_host")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-db_config")).not.toBeInTheDocument();

    // Check next button
    const nextButton = screen.getByTestId("config-next-button");
    expect(nextButton).toBeInTheDocument();
  });

  it("shows loading state while fetching config", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Check loading state
    expect(screen.getByTestId("configuration-step-loading")).toBeInTheDocument();
  });

  it("handles config fetch error gracefully", async () => {
    server.use(
      mockHandlers.appConfig.getTemplate({ error: { message: "Failed to template configuration" } }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Failed to template configuration")).toBeInTheDocument();
  });

  it("handles template config error gracefully", async () => {
    server.use(
      mockHandlers.appConfig.getTemplate({ error: { message: "Template processing failed" } }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Template processing failed")).toBeInTheDocument();
  });


  it("handles empty config gracefully", async () => {
    server.use(
      mockHandlers.appConfig.getTemplate({ groups: [] }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for empty state to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-empty")).toBeInTheDocument();
    });
    expect(screen.getByText("No configuration available")).toBeInTheDocument();
  });

  it("switches between tabs correctly", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Initially, Settings tab should be active
    expect(screen.getByTestId("config-item-app_name")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-description")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-enable_feature")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-auth_type")).toBeInTheDocument();

    // Check that the database tab is not rendered
    expect(screen.queryByTestId("config-item-db_host")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-db_config")).not.toBeInTheDocument();

    // Click on Database tab
    fireEvent.click(screen.getByTestId("config-tab-database"));

    // Database tab content should be visible
    expect(screen.getByTestId("config-item-db_host")).toBeInTheDocument();
    expect(screen.getByTestId("config-item-db_config")).toBeInTheDocument();

    // Settings tab content should not be visible
    expect(screen.queryByTestId("config-item-app_name")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-description")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-enable_feature")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-auth_type")).not.toBeInTheDocument();
    expect(screen.queryByTestId("config-item-ssl_certificate")).not.toBeInTheDocument();
  });

  it("handles text input changes correctly", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Find and update text input
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "New App Name" } });

    // Verify the value was updated
    expect(appNameInput).toHaveValue("New App Name");
  });

  it("handles textarea input changes correctly", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Find and update textarea input
    const descriptionTextarea = screen.getByTestId("textarea-input-description");
    fireEvent.change(descriptionTextarea, { target: { value: "New multi-line\ndescription text" } });

    // Verify the value was updated
    expect(descriptionTextarea).toHaveValue("New multi-line\ndescription text");
  });

  it("renders textarea with correct initial values", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Check textarea with value
    const descriptionTextarea = screen.getByTestId("textarea-input-description");
    expect(descriptionTextarea).toHaveValue("This is my application\nIt does amazing things");

    // Switch to database tab
    fireEvent.click(screen.getByTestId("config-tab-database"));

    // Check textarea with empty value
    const dbConfigTextarea = screen.getByTestId("textarea-input-db_config");
    expect(dbConfigTextarea).toHaveValue("");
  });

  it("handles checkbox changes correctly", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Find and toggle checkbox
    const enableFeatureCheckbox = screen.getByTestId("bool-input-enable_feature");
    expect(enableFeatureCheckbox).not.toBeChecked();

    fireEvent.click(enableFeatureCheckbox);
    expect(enableFeatureCheckbox).toBeChecked();

    fireEvent.click(enableFeatureCheckbox);
    expect(enableFeatureCheckbox).not.toBeChecked();
  });

  it("handles form submission error gracefully", async () => {
    server.use(
      mockHandlers.appConfig.updateValues({ error: { message: "Invalid configuration values" } }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Submit form
    const nextButton = screen.getByTestId("config-next-button");
    fireEvent.click(nextButton);

    // Verify error message is displayed
    await waitFor(() => {
      expect(screen.getByTestId("config-submit-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Invalid configuration values")).toBeInTheDocument();

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("submits the form successfully and returns updated config", async () => {
    let submittedValues: { values: AppConfigValues } | null = null;

    server.use(
      mockHandlers.appConfig.updateValues({
        captureRequest: (body: Record<string, unknown>, headers: Headers) => {
          // Verify auth header
          expect(headers.get("Authorization")).toBe("Bearer test-token");
          submittedValues = body as { values: AppConfigValues };
        }
      }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Make changes to form fields
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "Updated App Name" } });

    const descriptionTextarea = screen.getByTestId("textarea-input-description");
    fireEvent.change(descriptionTextarea, { target: { value: "Updated multi-line\ndescription text" } });

    const enableFeatureCheckbox = screen.getByTestId("bool-input-enable_feature");
    fireEvent.click(enableFeatureCheckbox);

    // Change radio button selection
    const anonymousRadio = screen.getByTestId("radio-input-auth_type_anonymous");
    fireEvent.click(anonymousRadio);

    // Click on Database tab
    fireEvent.click(screen.getByTestId("config-tab-database"));

    // Change text input
    const dbHostInput = screen.getByTestId("text-input-db_host");
    fireEvent.change(dbHostInput, { target: { value: "Updated DB Host" } });

    // Change textarea input
    const dbConfigTextarea = screen.getByTestId("textarea-input-db_config");
    fireEvent.change(dbConfigTextarea, { target: { value: "# Updated config\nhost: updated-host\nport: 5432" } });

    // Submit form
    const nextButton = screen.getByTestId("config-next-button");
    fireEvent.click(nextButton);

    // Wait for the mutation to complete and verify onNext was called
    await waitFor(
      () => {
        expect(mockOnNext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );

    // Verify the submitted values
    expect(submittedValues).not.toBeNull();
    expect(submittedValues!).toEqual({
      values: {
        app_name: { value: "Updated App Name" },
        description: { value: "Updated multi-line\ndescription text" },
        enable_feature: { value: "1" },
        auth_type: { value: "auth_type_anonymous" },
        db_host: { value: "Updated DB Host" },
        db_config: { value: "# Updated config\nhost: updated-host\nport: 5432" }
      }
    });
  });

  it("displays help text for text inputs", async () => {
    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Verify help text is displayed for the app_name field with default value
    expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    expect(screen.getByText(/Enter the name of your application/)).toBeInTheDocument();
    expect(screen.getByText("Default App")).toBeInTheDocument();
  });

  it("handles unauthorized error correctly", async () => {
    server.use(
      mockHandlers.appConfig.getTemplate({ error: { message: "Unauthorized", statusCode: 401 } }, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Unauthorized")).toBeInTheDocument();
  });

  it("only submits changed values", async () => {
    let submittedValues: { values: AppConfigValues } | null = null;

    server.use(
      mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Change the app name
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "Only Changed Field" } });

    // Change the description textarea
    const descriptionTextarea = screen.getByTestId("textarea-input-description");
    fireEvent.change(descriptionTextarea, { target: { value: "Only changed description" } });

    // Change the auth type
    const anonymousRadio = screen.getByTestId("radio-input-auth_type_anonymous");
    fireEvent.click(anonymousRadio);

    // Submit form
    const nextButton = screen.getByTestId("config-next-button");
    fireEvent.click(nextButton);

    // Wait for the mutation to complete
    await waitFor(
      () => {
        expect(mockOnNext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );

    // Verify only the changed values were submitted
    expect(submittedValues).not.toBeNull();
    expect(submittedValues!).toEqual({
      values: {
        app_name: { value: "Only Changed Field" },
        description: { value: "Only changed description" },
        auth_type: { value: "auth_type_anonymous" }
      }
    });
    expect(submittedValues!.values).not.toHaveProperty("enable_feature");
    expect(submittedValues!.values).not.toHaveProperty("db_host");
    expect(submittedValues!.values).not.toHaveProperty("db_config");
  });

  it("does not display default values for text and textarea", async () => {
    // Create a config with empty values but with defaults
    const configWithEmptyValues: AppConfig = {
      groups: [
        {
          name: "empty_values_test",
          title: "Empty Values Test",
          description: "Testing display behavior with empty values",
          items: [
            {
              name: "empty_text_field",
              title: "Empty Text Field",
              type: "text",
              value: "", // Empty value
              default: "Default Text Value" // Has default but should not show
            },
            {
              name: "empty_textarea_field",
              title: "Empty Textarea Field",
              type: "textarea",
              value: "", // Empty value
              default: "Default Textarea Value" // Has default but should not show
            }
          ]
        }
      ]
    };

    server.use(
      mockHandlers.appConfig.getTemplate(configWithEmptyValues, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Check that text input shows empty value (not default)
    const emptyTextInput = screen.getByTestId("text-input-empty_text_field");
    expect(emptyTextInput).toHaveValue("");

    // Check that textarea shows empty value (not default)
    const emptyTextareaInput = screen.getByTestId("textarea-input-empty_textarea_field");
    expect(emptyTextareaInput).toHaveValue("");
  });

  describe("Radio button behavior", () => {
    it("tests all radio button scenarios with different value/default combinations", async () => {
      // Override the mock config with multiple radio groups for different scenarios
      const comprehensiveConfig: AppConfig = {
        groups: [
          {
            name: "radio_test_scenarios",
            title: "Radio Test Scenarios",
            description: "Testing different radio button scenarios",
            items: [
              {
                name: "authentication_method",
                title: "Authentication Method",
                type: "radio",
                value: "authentication_method_ldap",
                items: [
                  {
                    name: "authentication_method_local",
                    title: "Local Authentication"
                  },
                  {
                    name: "authentication_method_ldap",
                    title: "LDAP Authentication"
                  }
                ]
              },
              {
                name: "database_type",
                title: "Database Type",
                type: "radio",
                default: "database_type_postgresql",
                items: [
                  {
                    name: "database_type_mysql",
                    title: "MySQL"
                  },
                  {
                    name: "database_type_postgresql",
                    title: "PostgreSQL"
                  }
                ]
              },
              {
                name: "logging_level",
                title: "Logging Level",
                type: "radio",
                value: "logging_level_debug",
                default: "logging_level_info",
                items: [
                  {
                    name: "logging_level_info",
                    title: "Info"
                  },
                  {
                    name: "logging_level_debug",
                    title: "Debug"
                  },
                  {
                    name: "logging_level_error",
                    title: "Error Only"
                  }
                ]
              },
              {
                name: "ssl_mode",
                title: "SSL Mode",
                type: "radio",
                items: [
                  {
                    name: "ssl_mode_disabled",
                    title: "Disabled"
                  },
                  {
                    name: "ssl_mode_required",
                    title: "Required"
                  }
                ]
              },
              {
                name: "notification_method",
                title: "Notification Method",
                type: "radio",
                default: "notification_method_email",
                items: [
                  {
                    name: "notification_method_email",
                    title: "Email"
                  },
                  {
                    name: "notification_method_slack",
                    title: "Slack"
                  }
                ]
              },
              {
                name: "backup_schedule",
                title: "Backup Schedule",
                type: "radio",
                value: "backup_schedule_daily",
                items: [
                  {
                    name: "backup_schedule_daily",
                    title: "Daily"
                  },
                  {
                    name: "backup_schedule_weekly",
                    title: "Weekly"
                  }
                ]
              }
            ]
          }
        ]
      };

      // Override the server to return our comprehensive config
      server.use(
        mockHandlers.appConfig.getTemplate(comprehensiveConfig, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that all radio groups are rendered
      expect(screen.getByTestId("config-item-authentication_method")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-database_type")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-logging_level")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-ssl_mode")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-notification_method")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-backup_schedule")).toBeInTheDocument();

      // Test scenario 1: Has value, no default (value should be selected)
      const localAuthRadio = screen.getByTestId("radio-input-authentication_method_local") as HTMLInputElement;
      const ldapAuthRadio = screen.getByTestId("radio-input-authentication_method_ldap") as HTMLInputElement;
      expect(localAuthRadio).not.toBeChecked();
      expect(ldapAuthRadio).toBeChecked(); // value is "authentication_method_ldap"

      // Test scenario 2: Has default, no value (default should be selected)
      const mysqlRadio = screen.getByTestId("radio-input-database_type_mysql") as HTMLInputElement;
      const postgresqlRadio = screen.getByTestId("radio-input-database_type_postgresql") as HTMLInputElement;
      expect(mysqlRadio).not.toBeChecked();
      expect(postgresqlRadio).toBeChecked(); // default is "database_type_postgresql"

      // Test scenario 3: Has both value and default (value should take precedence)
      const infoLogRadio = screen.getByTestId("radio-input-logging_level_info") as HTMLInputElement;
      const debugLogRadio = screen.getByTestId("radio-input-logging_level_debug") as HTMLInputElement;
      const errorLogRadio = screen.getByTestId("radio-input-logging_level_error") as HTMLInputElement;
      expect(infoLogRadio).not.toBeChecked();
      expect(debugLogRadio).toBeChecked(); // value is "logging_level_debug"
      expect(errorLogRadio).not.toBeChecked(); // default is "logging_level_info" but value takes precedence

      // Test scenario 4: Has neither value nor default (none should be selected)
      const sslDisabledRadio = screen.getByTestId("radio-input-ssl_mode_disabled") as HTMLInputElement;
      const sslRequiredRadio = screen.getByTestId("radio-input-ssl_mode_required") as HTMLInputElement;
      expect(sslDisabledRadio).not.toBeChecked();
      expect(sslRequiredRadio).not.toBeChecked();

      // Test scenario 5: Has default, but configValues overrides (configValues should be selected)
      const emailNotificationRadio = screen.getByTestId("radio-input-notification_method_email") as HTMLInputElement;
      const slackNotificationRadio = screen.getByTestId("radio-input-notification_method_slack") as HTMLInputElement;
      expect(emailNotificationRadio).toBeChecked(); // this is the default
      expect(slackNotificationRadio).not.toBeChecked();
      fireEvent.click(slackNotificationRadio); // User changes notification_method from default email to slack
      expect(emailNotificationRadio).not.toBeChecked();
      expect(slackNotificationRadio).toBeChecked(); // configValues "notification_method_slack" overrides default "notification_method_email"

      // Test scenario 6: Has value, but configValues overrides (configValues should be selected)
      const dailyBackupRadio = screen.getByTestId("radio-input-backup_schedule_daily") as HTMLInputElement;
      const weeklyBackupRadio = screen.getByTestId("radio-input-backup_schedule_weekly") as HTMLInputElement;
      expect(dailyBackupRadio).toBeChecked(); // this is the value in the config
      expect(weeklyBackupRadio).not.toBeChecked();
      fireEvent.click(weeklyBackupRadio); // User changes backup_schedule from daily to weekly
      expect(dailyBackupRadio).not.toBeChecked();
      expect(weeklyBackupRadio).toBeChecked(); // User changed from daily to weekly

      // Test radio button selection behavior
      fireEvent.click(localAuthRadio);
      expect(localAuthRadio).toBeChecked();
      expect(ldapAuthRadio).not.toBeChecked();

      // Test radio group behavior (only one can be selected)
      fireEvent.click(ldapAuthRadio);
      expect(localAuthRadio).not.toBeChecked();
      expect(ldapAuthRadio).toBeChecked();

      // Test form submission with radio button changes
      let submittedValues: { values: AppConfigValues } | null = null;
      server.use(
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      // Change a radio button selection
      fireEvent.click(mysqlRadio);

      // Submit form
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );

      // Verify the radio button changes were submitted
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!).toEqual({
        values: {
          authentication_method: { value: "authentication_method_ldap" }, // User interacted with this field (clicked local then ldap)
          notification_method: { value: "notification_method_slack" }, // User changed from default email to slack
          backup_schedule: { value: "backup_schedule_weekly" }, // User changed from daily to weekly
          database_type: { value: "database_type_mysql" } // User changed from default postgresql to mysql
        }
      });
    });

    it("handles radio button group behavior (only one can be selected)", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Get all radio buttons in the authentication group
      expect(screen.getByTestId("config-item-auth_type")).toBeInTheDocument();
      const anonymousRadio = screen.getByTestId("radio-input-auth_type_anonymous") as HTMLInputElement;
      const passwordRadio = screen.getByTestId("radio-input-auth_type_password") as HTMLInputElement;

      // Initially, Password should be selected
      expect(passwordRadio).toBeChecked();
      expect(anonymousRadio).not.toBeChecked();

      // Click on Anonymous
      fireEvent.click(anonymousRadio);

      // Now only Anonymous should be selected
      expect(anonymousRadio).toBeChecked();
      expect(passwordRadio).not.toBeChecked();

      // Click on Password
      fireEvent.click(passwordRadio);

      // Now only Password should be selected
      expect(anonymousRadio).not.toBeChecked();
      expect(passwordRadio).toBeChecked();
    });
  });

  describe("Label functionality", () => {
    it("renders label config items correctly", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that label config items are rendered
      expect(screen.getByTestId("label-info_label")).toBeInTheDocument();
      expect(screen.getByTestId("label-markdown_label")).toBeInTheDocument();
    });

    it("renders label content with automatic link detection", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that URL is converted to a clickable link
      const infoLabel = screen.getByTestId("label-info_label");
      expect(infoLabel).toBeInTheDocument();

      // Check that the link is present and has correct attributes
      const docsLink = screen.getByRole("link", { name: /docs.example.com/ });
      expect(docsLink).toBeInTheDocument();
      expect(docsLink).toHaveAttribute("href", "https://docs.example.com");
      expect(docsLink).toHaveAttribute("target", "_blank");
    });

    it("renders label content with markdown formatting", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that label items are rendered in the active tab
      expect(screen.getByTestId("label-info_label")).toBeInTheDocument();

      // Check that markdown is rendered correctly
      const markdownLabel = screen.getByTestId("label-markdown_label");
      expect(markdownLabel).toBeInTheDocument();

      // Check for bold text
      const boldText = markdownLabel.querySelector("strong");
      expect(boldText).toBeInTheDocument();
      expect(boldText).toHaveTextContent("bold");

      // Check for italic text
      const italicText = markdownLabel.querySelector("em");
      expect(italicText).toBeInTheDocument();
      expect(italicText).toHaveTextContent("italic");

      // Check for markdown link
      const markdownLink = screen.getByRole("link", { name: "link" });
      expect(markdownLink).toBeInTheDocument();
      expect(markdownLink).toHaveAttribute("href", "https://example.com");
      expect(markdownLink).toHaveAttribute("target", "_blank");
    });

    it("renders label content in database tab with markdown and links", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Switch to database tab
      fireEvent.click(screen.getByTestId("config-tab-database"));

      // Check that the database warning label is rendered
      const dbWarningLabel = screen.getByTestId("label-db_warning");
      expect(dbWarningLabel).toBeInTheDocument();

      // Check for bold text in the warning
      const importantText = dbWarningLabel.querySelector("strong");
      expect(importantText).toBeInTheDocument();
      expect(importantText).toHaveTextContent("Important");

      // Check for automatic link detection
      const helpLink = screen.getByRole("link", { name: /help.example.com/ });
      expect(helpLink).toBeInTheDocument();
      expect(helpLink).toHaveAttribute("href", "https://help.example.com/database-config");
      expect(helpLink).toHaveAttribute("target", "_blank");
    });

    it("label items do not affect form submission", async () => {
      let submittedValues: { values: AppConfigValues } | null = null;

      server.use(
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Make a change to a non-label field
      const appNameInput = screen.getByTestId("text-input-app_name");
      fireEvent.change(appNameInput, { target: { value: "Test App" } });

      // Submit form
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );

      // Verify only the changed input field was submitted (not labels)
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!).toEqual({
        values: {
          app_name: { value: "Test App" }
        }
      });

      // Verify label fields are not included in submission
      expect(submittedValues!.values).not.toHaveProperty("info_label");
      expect(submittedValues!.values).not.toHaveProperty("markdown_label");
      expect(submittedValues!.values).not.toHaveProperty("db_warning");
    });
  });

  describe("Bool field behavior", () => {
    it("tests all bool field scenarios with different value/default combinations", async () => {
      // Create config with bool fields that mirror the radio button test scenarios
      const comprehensiveConfig: AppConfig = {
        groups: [
          {
            name: "bool_test_scenarios",
            title: "Bool Test Scenarios",
            description: "Testing different bool field scenarios",
            items: [
              {
                name: "authentication_method",
                title: "Authentication Method",
                type: "bool",
                value: "1"
              },
              {
                name: "database_type",
                title: "Database Type",
                type: "bool",
                default: "1"
              },
              {
                name: "logging_level",
                title: "Logging Level",
                type: "bool",
                value: "1",
                default: "0"
              },
              {
                name: "ssl_mode",
                title: "SSL Mode",
                type: "bool"
              },
              {
                name: "notification_method",
                title: "Notification Method",
                type: "bool",
                default: "0"
              },
              {
                name: "backup_schedule",
                title: "Backup Schedule",
                type: "bool",
                value: "0"
              }
            ]
          }
        ]
      };

      // Override the server to return our comprehensive config as-is
      server.use(
        mockHandlers.appConfig.getTemplate(comprehensiveConfig, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that all bool fields are rendered
      expect(screen.getByTestId("config-item-authentication_method")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-database_type")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-logging_level")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-ssl_mode")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-notification_method")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-backup_schedule")).toBeInTheDocument();

      // Test scenario 1: Has value, no default (value should be used)
      const authenticationMethodCheckbox = screen.getByTestId("bool-input-authentication_method") as HTMLInputElement;
      expect(authenticationMethodCheckbox).toBeChecked(); // value is "1"

      // Test scenario 2: Has default, no value (default should be used)
      const databaseTypeCheckbox = screen.getByTestId("bool-input-database_type") as HTMLInputElement;
      expect(databaseTypeCheckbox).toBeChecked(); // default is "1"

      // Test scenario 3: Has both value and default (value should take precedence)
      const loggingLevelCheckbox = screen.getByTestId("bool-input-logging_level") as HTMLInputElement;
      expect(loggingLevelCheckbox).toBeChecked(); // value is "1" takes precedence over default "0"

      // Test scenario 4: Has neither value nor default (should be unchecked)
      const sslModeCheckbox = screen.getByTestId("bool-input-ssl_mode") as HTMLInputElement;
      expect(sslModeCheckbox).not.toBeChecked(); // no value or default

      // Test scenario 5: Has default, but configValues overrides (configValues should be used)
      const notificationMethodCheckbox = screen.getByTestId("bool-input-notification_method") as HTMLInputElement;
      expect(notificationMethodCheckbox).not.toBeChecked(); // default is "0"
      fireEvent.click(notificationMethodCheckbox); // User changes notification_method from default unchecked to checked
      expect(notificationMethodCheckbox).toBeChecked(); // configValues "1" overrides default "0"

      // Test scenario 6: Has value, but configValues overrides (configValues should be used)
      const backupScheduleCheckbox = screen.getByTestId("bool-input-backup_schedule") as HTMLInputElement;
      expect(backupScheduleCheckbox).not.toBeChecked(); // value is "0"
      fireEvent.click(backupScheduleCheckbox); // User changes backup_schedule from value "0" to checked
      expect(backupScheduleCheckbox).toBeChecked(); // configValues "1" overrides value "0"

      // Test checkbox toggling behavior
      fireEvent.click(authenticationMethodCheckbox);
      expect(authenticationMethodCheckbox).not.toBeChecked();

      fireEvent.click(authenticationMethodCheckbox);
      expect(authenticationMethodCheckbox).toBeChecked();

      // Test form submission with checkbox changes
      let submittedValues: { values: AppConfigValues } | null = null;
      server.use(
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      // Change a checkbox
      fireEvent.click(databaseTypeCheckbox);

      // Submit form
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );

      // Verify the checkbox changes were submitted
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!).toEqual({
        values: {
          authentication_method: { value: "1" }, // User interacted with this field (toggled from unchecked to checked)
          notification_method: { value: "1" }, // User changed from default unchecked to checked
          backup_schedule: { value: "1" }, // User changed from unchecked to checked
          database_type: { value: "0" } // User changed from default checked to unchecked
        }
      });
    });
  });

  it("only submits changed values (PATCH behavior)", async () => {
    let submittedValues: { values: AppConfigValues } | null = null;

    server.use(
      mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Make changes to form fields
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "" } }); // Clear app_name to test empty string submission

    // Switch to database tab and change a field that wasn't in retrieved values
    fireEvent.click(screen.getByTestId("config-tab-database"));
    const dbHostInput = screen.getByTestId("text-input-db_host");
    fireEvent.change(dbHostInput, { target: { value: "new-db-host" } });

    // Submit form
    const nextButton = screen.getByTestId("config-next-button");
    fireEvent.click(nextButton);

    // Wait for the mutation to complete
    await waitFor(
      () => {
        expect(mockOnNext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );

    // Verify that only changed values are submitted (PATCH behavior)
    expect(submittedValues).not.toBeNull();
    expect(submittedValues!).toEqual({
      values: {
        app_name: { value: "" }, // cleared to empty string (should be submitted)
        // New changes to fields not in retrieved values
        db_host: { value: "new-db-host" }
      }
    });

    // Explicitly verify unchanged values are not submitted
    expect(submittedValues!.values).not.toHaveProperty("enable_feature");
    expect(submittedValues!.values).not.toHaveProperty("auth_type");
  });

  it("clears password field on first keystroke when field is not dirty", async () => {
    // Create config with password field
    const configWithPassword: AppConfig = {
      groups: [
        {
          name: "auth",
          title: "Authentication",
          items: [
            {
              name: "user_password",
              title: "User Password",
              type: "password",
              value: "••••••••",
              default: "default_password"
            }
          ]
        }
      ]
    };

    server.use(
      mockHandlers.appConfig.getTemplate(configWithPassword, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Get the password input
    const passwordInput = screen.getByTestId("password-input-user_password") as HTMLInputElement;

    // Test the clear-on-keystroke behavior - field should show API masked value
    expect(passwordInput.value).toBe("••••••••");

    // Simulate first keystroke - keydown should clear the field
    fireEvent.keyDown(passwordInput, { key: 'a' });

    // Field should be empty after keydown (before change event)
    expect(passwordInput.value).toBe('');

    // Then change event sets the typed character  
    fireEvent.change(passwordInput, { target: { value: 'a' } });

    // Field should now show only the typed character
    expect(passwordInput.value).toBe('a');

    // Type more characters normally
    fireEvent.change(passwordInput, { target: { value: 'abc' } });
    expect(passwordInput.value).toBe('abc');
  });

  it("controls password visibility toggle based on user input for password fields", async () => {
    // Create config with password field
    const configWithPassword: AppConfig = {
      groups: [
        {
          name: "auth",
          title: "Authentication",
          items: [
            {
              name: "user_password",
              title: "User Password",
              type: "password",
              value: "••••••••",
              default: "default_password"
            }
          ]
        }
      ]
    };

    server.use(
      mockHandlers.appConfig.getTemplate(configWithPassword, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Get the password input
    const passwordInput = screen.getByTestId("password-input-user_password") as HTMLInputElement;

    // Initially, the password visibility toggle should NOT be available
    // because allowShowPassword returns false when no user value is set
    expect(screen.queryByTestId("password-visibility-toggle-password-input-user_password")).not.toBeInTheDocument();
    expect(passwordInput).toHaveValue("••••••••"); // API masked value

    // Simulate user typing in the password field
    fireEvent.keyDown(passwordInput, { key: 'a' }); // This clears the field
    fireEvent.change(passwordInput, { target: { value: 'mypassword' } });

    // Wait for the component to re-render and the visibility toggle to appear
    await waitFor(() => {
      expect(screen.getByTestId("password-visibility-toggle-password-input-user_password")).toBeInTheDocument();
    });

    // Verify the eye icon is shown (password is hidden by default)
    const visibilityToggle = screen.getByTestId("password-visibility-toggle-password-input-user_password");
    expect(screen.getByTestId("eye-icon-password-input-user_password")).toBeInTheDocument();
    expect(screen.queryByTestId("eye-off-icon-password-input-user_password")).not.toBeInTheDocument();

    // Password input should be of type "password" (hidden)
    expect(passwordInput.type).toBe("password");

    // Click the visibility toggle to show password
    fireEvent.click(visibilityToggle);

    // Verify the eye-off icon is now shown and input type changed to text
    expect(screen.getByTestId("eye-off-icon-password-input-user_password")).toBeInTheDocument();
    expect(screen.queryByTestId("eye-icon-password-input-user_password")).not.toBeInTheDocument();
    expect(passwordInput.type).toBe("text");

    // Click the visibility toggle again to hide password
    fireEvent.click(visibilityToggle);

    // Verify we're back to the eye icon and password type
    expect(screen.getByTestId("eye-icon-password-input-user_password")).toBeInTheDocument();
    expect(screen.queryByTestId("eye-off-icon-password-input-user_password")).not.toBeInTheDocument();
    expect(passwordInput.type).toBe("password");

    // Clear the password field to test that toggle disappears
    fireEvent.change(passwordInput, { target: { value: '' } });

    // Wait for the visibility toggle to disappear
    await waitFor(() => {
      expect(screen.queryByTestId("password-visibility-toggle-password-input-user_password")).not.toBeInTheDocument();
    });
  });
  it("handles empty string values vs undefined/null values correctly", async () => {
    // Create config with realistic field names
    const configWithDefaults: AppConfig = {
      groups: [
        {
          name: "database",
          title: "Database Settings",
          items: [
            {
              name: "db_name",
              title: "Database Name",
              type: "text",
              value: "myapp",
              default: "default_db"
            },
            {
              name: "db_password",
              title: "Database Password",
              type: "text",
              value: "secret123",
              default: "changeme"
            },
            {
              name: "db_port",
              title: "Database Port",
              type: "text",
              default: "5432"
            },
            {
              name: "enable_ssl",
              title: "Enable SSL",
              type: "bool",
              default: "1"
            }
          ]
        }
      ]
    };

    server.use(
      mockHandlers.appConfig.getTemplate(configWithDefaults, target, mode)
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
        mode: mode,
      },
    });

    // Wait for config to load
    await waitForForm();

    // Field updated with empty string in configValues should show empty string (not schema value or default)
    const dbNameField = screen.getByTestId("text-input-db_name") as HTMLInputElement;
    fireEvent.change(dbNameField, { target: { value: "" } });
    expect(dbNameField.value).toBe(""); // configValues empty string takes precedence

    // Field with no configValues entry should show schema value (current behavior with getDisplayValue)
    const dbPasswordField = screen.getByTestId("text-input-db_password") as HTMLInputElement;
    expect(dbPasswordField.value).toBe("secret123"); // schema value shows since no configValues entry

    // Field with no value in config but has default should show empty string (getDisplayValue doesn't use default)
    const dbPortField = screen.getByTestId("text-input-db_port") as HTMLInputElement;
    expect(dbPortField.value).toBe(""); // no value in config, default is not used as getDisplayValue doesn't use default

    // Bool field with no value in config but has default should use default (getEffectiveValue includes default)
    const enableSslField = screen.getByTestId("bool-input-enable_ssl") as HTMLInputElement;
    expect(enableSslField.checked).toBe(true); // default "1" is used since getEffectiveValue includes default
  });

  describe("File input functionality", () => {
    it("renders file input field correctly", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that the file input is rendered
      expect(screen.getByTestId("config-item-ssl_certificate")).toBeInTheDocument();
      expect(screen.getByTestId("file-input-ssl_certificate")).toBeInTheDocument();
    });

    it("handles file upload, displays filename, and removes file", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      const fileInput = screen.getByTestId("file-input-ssl_certificate");

      // Create a mock file
      const file = new File(['certificate content'], 'cert.pem', { type: 'text/plain' });

      // Mock FileReader
      const mockFileReader = {
        readAsDataURL: vi.fn().mockImplementation(() => {
          // Simulate async file reading completing
          setTimeout(() => {
            if (mockFileReader.onload) {
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              mockFileReader.onload({ target: { result: 'data:text/plain;base64,Y2VydGlmaWNhdGUgY29udGVudA==' } } as any);
            }
          }, 0);
        }),
        result: 'data:text/plain;base64,Y2VydGlmaWNhdGUgY29udGVudA==',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onload: null as any,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onerror: null as any
      };

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      vi.spyOn(global, 'FileReader').mockImplementation(() => mockFileReader as any);

      // Simulate file selection
      fireEvent.change(fileInput, { target: { files: [file] } });

      // Wait for the async file reading to complete
      await waitFor(() => {
        expect(mockFileReader.readAsDataURL).toHaveBeenCalledWith(file);
      });

      // Wait for the filename to appear
      await waitFor(() => {
        expect(screen.getByTestId('file-input-ssl_certificate-filename')).toBeInTheDocument();
      });

      const fileName = screen.getByTestId('file-input-ssl_certificate-filename');
      expect(fileName).toHaveTextContent('cert.pem');

      // Remove file
      const removeButton = screen.getByTestId('file-input-ssl_certificate-remove');
      fireEvent.click(removeButton);

      // File should be removed
      expect(fileName).not.toBeInTheDocument();
    });

    it("submits file content and filename in form", async () => {
      let submittedValues: { values: AppConfigValues } | null = null;

      server.use(
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      const fileInput = screen.getByTestId("file-input-ssl_certificate");

      // Create a mock file
      const file = new File(['certificate content'], 'cert.pem', { type: 'text/plain' });

      // Mock FileReader
      const mockFileReader = {
        readAsDataURL: vi.fn().mockImplementation(() => {
          // Simulate async file reading completing
          setTimeout(() => {
            if (mockFileReader.onload) {
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              mockFileReader.onload({ target: { result: 'data:text/plain;base64,Y2VydGlmaWNhdGUgY29udGVudA==' } } as any);
            }
          }, 0);
        }),
        result: 'data:text/plain;base64,Y2VydGlmaWNhdGUgY29udGVudA==',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onload: null as any,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onerror: null as any
      };

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      vi.spyOn(global, 'FileReader').mockImplementation(() => mockFileReader as any);

      // Simulate file selection
      fireEvent.change(fileInput, { target: { files: [file] } });

      // Wait for the async file reading to complete
      await waitFor(() => {
        expect(mockFileReader.readAsDataURL).toHaveBeenCalledWith(file);
      });

      // Wait for the filename to appear
      await waitFor(() => {
        expect(screen.getByTestId('file-input-ssl_certificate-filename')).toBeInTheDocument();
      });

      const fileName = screen.getByTestId('file-input-ssl_certificate-filename');
      expect(fileName).toHaveTextContent('cert.pem');

      // Submit form
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );

      // Verify the file content and filename were submitted
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!.values.ssl_certificate).toEqual({
        value: 'Y2VydGlmaWNhdGUgY29udGVudA==', // base64 encoded "certificate content"
        filename: 'cert.pem'
      });
    });

    it("handles file config items with filename from backend template engine", async () => {
      // Create config with file item that has filename set by backend
      const configWithFilename: AppConfig = {
        groups: [
          {
            name: "files",
            title: "File Configuration",
            description: "Configure file uploads",
            items: [
              {
                name: "config_file",
                title: "Configuration File",
                type: "file",
                value: "base64_encoded_content_here",
                filename: "config.yaml", // This filename comes from backend template engine
                help_text: "Upload your configuration file"
              },
              {
                name: "cert_file",
                title: "Certificate File",
                type: "file",
                value: "another_base64_content",
                filename: "cert.pem", // This filename comes from backend template engine
                help_text: "Upload your certificate file"
              }
            ]
          }
        ]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithFilename, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that file inputs are rendered
      expect(screen.getByTestId("config-item-config_file")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-cert_file")).toBeInTheDocument();

      // Check that the filenames from backend are displayed
      expect(screen.getByTestId("file-input-config_file-filename")).toBeInTheDocument();
      expect(screen.getByTestId("file-input-cert_file-filename")).toBeInTheDocument();
      expect(screen.getByText("config.yaml")).toBeInTheDocument();
      expect(screen.getByText("cert.pem")).toBeInTheDocument();

      // Test that we can still upload new files and they override the backend filename
      const configFileInput = screen.getByTestId("file-input-config_file");
      const newFile = new File(['new content'], 'new-config.yml', { type: 'text/plain' });

      // Mock FileReader for new file upload
      const mockFileReader = {
        readAsDataURL: vi.fn().mockImplementation(() => {
          // Use setTimeout with a small delay to simulate async file reading
          setTimeout(() => {
            if (mockFileReader.onload) {
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              mockFileReader.onload({ target: { result: 'data:text/plain;base64,bmV3IGNvbnRlbnQ=' } } as any);
            }
          }, 10);
        }),
        result: 'data:text/plain;base64,bmV3IGNvbnRlbnQ=',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onload: null as any,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onerror: null as any
      };

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      vi.spyOn(global, 'FileReader').mockImplementation(() => mockFileReader as any);

      // Upload new file
      fireEvent.change(configFileInput, { target: { files: [newFile] } });

      // Wait for the new filename to appear and file processing to complete
      await waitFor(() => {
        expect(screen.getByText("new-config.yml")).toBeInTheDocument();
      });

      // Wait a bit more to ensure the file processing is fully complete
      await new Promise(resolve => setTimeout(resolve, 20));

      // Verify the old filename is no longer displayed
      expect(screen.queryByText("config.yaml")).not.toBeInTheDocument();

      // Test form submission with the new file
      let submittedValues: { values: AppConfigValues } | null = null;
      server.use(
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for the mutation to complete
      await waitFor(
        () => {
          expect(mockOnNext).toHaveBeenCalled();
        },
        { timeout: 3000 }
      );

      // Verify the new file was submitted with correct filename
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!.values.config_file).toEqual({
        value: 'bmV3IGNvbnRlbnQ=', // base64 encoded "new content"
        filename: 'new-config.yml'
      });

      // Verify the cert file is not submitted since it wasn't changed
      expect(submittedValues!.values.cert_file).toBeUndefined();
    });

    it("handles file config items with no filename gracefully", async () => {
      // Create config with file item that has no filename
      const configWithoutFilename: AppConfig = {
        groups: [
          {
            name: "files",
            title: "File Configuration",
            description: "Configure file uploads",
            items: [
              {
                name: "no_filename_file",
                title: "File Without Filename",
                type: "file",
                value: "base64_encoded_content",
                // No filename field
                help_text: "Upload a file"
              }
            ]
          }
        ]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithoutFilename, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for config to load
      await waitForForm();

      // Check that file input is rendered
      expect(screen.getByTestId("config-item-no_filename_file")).toBeInTheDocument();

      // Check that no filename is displayed initially (since no filename from backend)
      expect(screen.queryByTestId("file-input-no_filename_file-filename")).not.toBeInTheDocument();

      // Test that we can still upload a file and it gets a filename
      const fileInput = screen.getByTestId("file-input-no_filename_file");
      const newFile = new File(['content'], 'uploaded-file.txt', { type: 'text/plain' });

      // Mock FileReader
      const mockFileReader = {
        readAsDataURL: vi.fn().mockImplementation(() => {
          // Use setTimeout with a small delay to simulate async file reading
          setTimeout(() => {
            if (mockFileReader.onload) {
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              mockFileReader.onload({ target: { result: 'data:text/plain;base64,Y29udGVudA==' } } as any);
            }
          }, 10);
        }),
        result: 'data:text/plain;base64,Y29udGVudA==',
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onload: null as any,
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        onerror: null as any
      };

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      vi.spyOn(global, 'FileReader').mockImplementation(() => mockFileReader as any);

      // Upload file
      fireEvent.change(fileInput, { target: { files: [newFile] } });

      // Wait for the filename to appear and file processing to complete
      await waitFor(() => {
        expect(screen.getByTestId("file-input-no_filename_file-filename")).toBeInTheDocument();
      });

      // Wait a bit more to ensure the file processing is fully complete
      await new Promise(resolve => setTimeout(resolve, 20));

      expect(screen.getByText("uploaded-file.txt")).toBeInTheDocument();
    });
  });

  describe("Server-driven validation", () => {
    it("shows server validation error for required field when submission fails", async () => {
      // Create config with required field for this specific test
      const configWithRequiredField: AppConfig = {
        groups: [
          {
            name: "settings",
            title: "Settings",
            description: "Configure application settings",
            items: [
              {
                name: "required_field",
                title: "Required Field",
                type: "text",
                value: "",
                required: true,
                help_text: "This field is required"
              }
            ]
          }
        ]
      };

      server.use(
        // Mock template endpoint to return config with required field
        mockHandlers.appConfig.getTemplate(configWithRequiredField, target, mode),
        // Mock server validation error response with field-level errors
        mockHandlers.appConfig.updateValues(
          {
            error: {
              message: "required fields not completed",
              errors: [
                {
                  field: "required_field",
                  message: "Required Field is required"
                }
              ]
            }
          },
          target,
          mode
        )
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for the required field to be rendered
      await waitFor(() => {
        expect(screen.getByTestId("text-input-required_field")).toBeInTheDocument();
      });

      // Ensure the required field is empty
      const requiredInput = screen.getByTestId("text-input-required_field");
      expect(requiredInput).toHaveValue("");

      // Submit form without filling required field
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for server validation error to appear (raw server message)
      await waitFor(() => {
        expect(screen.getByText("Required Field is required")).toBeInTheDocument();
      });

      // Verify the raw server error message at the bottom
      await waitFor(() => {
        expect(screen.getByText("required fields not completed")).toBeInTheDocument();
      });

      // Verify onNext was not called due to validation error
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("autofocuses on first required field with validation error on submit", async () => {
      // Create config with multiple required fields to test focus priority
      const configWithMultipleRequiredFields: AppConfig = {
        groups: [
          {
            name: "settings",
            title: "Settings",
            description: "Configure application settings",
            items: [
              {
                name: "optional_field",
                title: "Optional Field",
                type: "text",
                value: "",
                required: false,
                help_text: "This field is optional"
              },
              {
                name: "first_required_field",
                title: "First Required Field",
                type: "text",
                value: "",
                required: true,
                help_text: "This is the first required field"
              },
              {
                name: "second_required_field",
                title: "Second Required Field",
                type: "text",
                value: "",
                required: true,
                help_text: "This is the second required field"
              }
            ]
          }
        ]
      };

      server.use(
        // Mock template endpoint to return config with multiple required fields
        mockHandlers.appConfig.getTemplate(configWithMultipleRequiredFields, target, mode),
        // Mock server validation error response with multiple field errors
        mockHandlers.appConfig.updateValues(
          {
            error: {
              message: "required fields not completed",
              errors: [
                {
                  field: "first_required_field",
                  message: "First Required Field is required"
                },
                {
                  field: "second_required_field",
                  message: "Second Required Field is required"
                }
              ]
            }
          },
          target,
          mode
        )
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for the required fields to be rendered
      await waitFor(() => {
        expect(screen.getByTestId("text-input-first_required_field")).toBeInTheDocument();
      });

      // Ensure both required fields are empty
      const firstRequiredInput = screen.getByTestId("text-input-first_required_field");
      const secondRequiredInput = screen.getByTestId("text-input-second_required_field");
      expect(firstRequiredInput).toHaveValue("");
      expect(secondRequiredInput).toHaveValue("");

      // Submit form without filling required fields
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for server validation errors to appear (raw server message)
      await waitFor(() => {
        expect(screen.getByText("First Required Field is required")).toBeInTheDocument();
      });

      // Verify that the first required field (in DOM order) is focused
      await waitFor(() => {
        expect(firstRequiredInput).toHaveFocus();
      });

      // Verify onNext was not called due to validation error
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("autofocus switches to correct tab when required field is in non-active tab", async () => {
      // Create config with required fields in different tabs
      const configWithMultipleTabsAndRequiredFields: AppConfig = {
        groups: [
          {
            name: "settings",
            title: "Settings",
            description: "Configure application settings",
            items: [
              {
                name: "optional_setting",
                title: "Optional Setting",
                type: "text",
                value: "filled",
                required: false,
                help_text: "This field is optional"
              }
            ]
          },
          {
            name: "database",
            title: "Database",
            description: "Configure database settings",
            items: [
              {
                name: "db_required_field",
                title: "Database Required Field",
                type: "text",
                value: "",
                required: true,
                help_text: "This database field is required"
              },
              {
                name: "db_optional_field",
                title: "Database Optional Field",
                type: "text",
                value: "",
                required: false,
                help_text: "This database field is optional"
              }
            ]
          }
        ]
      };

      server.use(
        // Mock template endpoint to return config with multiple tabs and required fields
        mockHandlers.appConfig.getTemplate(configWithMultipleTabsAndRequiredFields, target, mode),
        // Mock server validation error response
        mockHandlers.appConfig.updateValues(
          {
            error: {
              message: "required fields not completed",
              errors: [
                {
                  field: "db_required_field",
                  message: "Database Required Field is required"
                }
              ]
            }
          },
          target,
          mode
        )
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for tabs and fields to be rendered - should default to first tab (settings)
      await waitFor(() => {
        expect(screen.getByTestId("config-tab-settings")).toBeInTheDocument();
        expect(screen.getByTestId("config-tab-database")).toBeInTheDocument();
      });

      // Verify we're on the settings tab initially (first tab is active by default)
      const settingsTab = screen.getByTestId("config-tab-settings");
      const databaseTab = screen.getByTestId("config-tab-database");

      // Settings tab should be active (has the blue color styling)
      expect(settingsTab).toHaveStyle("color: rgb(49, 109, 230)");

      // Database tab should be inactive (has gray color)
      expect(databaseTab).toHaveStyle("color: rgb(107, 114, 128)");

      // Verify settings tab field is visible, database tab field is not visible  
      expect(screen.getByTestId("text-input-optional_setting")).toBeInTheDocument();
      expect(screen.queryByTestId("text-input-db_required_field")).not.toBeInTheDocument();

      // Submit form without filling required field (which is in database tab)
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for server validation errors to appear (raw server message)
      await waitFor(() => {
        expect(screen.getByText("Database Required Field is required")).toBeInTheDocument();
      });

      // Verify that the system switched to the database tab
      await waitFor(() => {
        expect(databaseTab).toHaveStyle("color: rgb(49, 109, 230)"); // Now active
      });

      // Verify that the database required field is now visible and focused
      await waitFor(() => {
        const dbRequiredInput = screen.getByTestId("text-input-db_required_field");
        expect(dbRequiredInput).toBeInTheDocument();
        expect(dbRequiredInput).toHaveFocus();
      });

      // Verify settings tab field is no longer visible (tab switched)
      expect(screen.queryByTestId("text-input-optional_setting")).not.toBeInTheDocument();

      // Verify onNext was not called due to validation error
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("shows red border for required text input when empty on submit", async () => {
      // Create config with required text field
      const configWithRequiredField: AppConfig = {
        groups: [
          {
            name: "settings",
            title: "Settings",
            description: "Configure application settings",
            items: [
              {
                name: "required_text_field",
                title: "Required Text Field",
                type: "text",
                value: "",
                required: true,
                help_text: "This field is required"
              }
            ]
          }
        ]
      };

      server.use(
        // Mock template endpoint to return config with required field
        mockHandlers.appConfig.getTemplate(configWithRequiredField, target, mode),
        // Mock server validation error response with field-level errors
        mockHandlers.appConfig.updateValues(
          {
            error: {
              message: "required fields not completed",
              errors: [
                {
                  field: "required_text_field",
                  message: "Required Text Field is required"
                }
              ]
            }
          },
          target,
          mode
        )
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for the required field to be rendered
      await waitFor(() => {
        expect(screen.getByTestId("text-input-required_text_field")).toBeInTheDocument();
      });

      const requiredInput = screen.getByTestId("text-input-required_text_field");

      // Initially, field should have normal gray border
      expect(requiredInput).toHaveClass("border-gray-300");
      expect(requiredInput).not.toHaveClass("border-red-500");

      // Submit form without filling required field
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for server validation error to appear (raw server message)
      await waitFor(() => {
        expect(screen.getByText("Required Text Field is required")).toBeInTheDocument();
      });

      // Verify the input now has red border
      await waitFor(() => {
        expect(requiredInput).toHaveClass("border-red-500");
        expect(requiredInput).not.toHaveClass("border-gray-300");
      });

      // Verify onNext was not called due to validation error
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it("autofocuses on first radio button option when radio field is required and empty", async () => {
      // Create config with required radio field
      const configWithRequiredRadioField: AppConfig = {
        groups: [
          {
            name: "settings",
            title: "Settings",
            description: "Configure application settings",
            items: [
              {
                name: "auth_method",
                title: "Authentication Method",
                type: "radio",
                value: "", // Empty - no option selected
                required: true,
                help_text: "Choose your authentication method",
                items: [
                  {
                    name: "auth_method_local",
                    title: "Local Authentication"
                  },
                  {
                    name: "auth_method_ldap",
                    title: "LDAP Authentication"
                  }
                ]
              }
            ]
          }
        ]
      };

      server.use(
        // Mock template endpoint to return config with required radio field
        mockHandlers.appConfig.getTemplate(configWithRequiredRadioField, target, mode),
        // Mock server validation error response
        mockHandlers.appConfig.updateValues(
          {
            error: {
              message: "required fields not completed",
              errors: [
                {
                  field: "auth_method",
                  message: "Authentication Method is required"
                }
              ]
            }
          },
          target,
          mode
        )
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for the radio field to be rendered
      await waitFor(() => {
        expect(screen.getByTestId("config-item-auth_method")).toBeInTheDocument();
      });

      // Ensure no radio button is selected initially
      const localAuthRadio = screen.getByTestId("radio-input-auth_method_local");
      const ldapAuthRadio = screen.getByTestId("radio-input-auth_method_ldap");
      expect(localAuthRadio).not.toBeChecked();
      expect(ldapAuthRadio).not.toBeChecked();

      // Submit form without selecting required radio field
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      // Wait for server validation error to appear (raw server message)
      await waitFor(() => {
        expect(screen.getByText("Authentication Method is required")).toBeInTheDocument();
      });

      // Verify that the first radio button option is focused
      // Since radio buttons use individual option IDs, we focus the first option
      await waitFor(() => {
        expect(localAuthRadio).toHaveFocus();
      });

      // Verify onNext was not called due to validation error
      expect(mockOnNext).not.toHaveBeenCalled();
    });
  });

  describe("Config Frontend Parity - New Config Types", () => {
    it("renders dropdown config type with items", async () => {
      const configWithDropdown: AppConfig = {
        groups: [{
          name: 'database',
          title: 'Database Settings',
          items: [{
            name: 'db_type',
            type: 'dropdown',
            title: 'Database Type',
            help_text: 'Select your database type',
            default: 'postgres',
            items: [
              { name: 'postgres', title: 'PostgreSQL' },
              { name: 'mysql', title: 'MySQL' },
              { name: 'mariadb', title: 'MariaDB' }
            ]
          }]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithDropdown, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      // Check dropdown renders
      const dropdown = screen.getByTestId("dropdown-input-db_type");
      expect(dropdown).toBeInTheDocument();
      expect(dropdown).toBeInstanceOf(HTMLSelectElement);

      // Check options are present
      expect(screen.getByText('Select an option')).toBeInTheDocument();
      expect(screen.getByText('PostgreSQL')).toBeInTheDocument();
      expect(screen.getByText('MySQL')).toBeInTheDocument();
      expect(screen.getByText('MariaDB')).toBeInTheDocument();
    });

    it("handles dropdown selection changes correctly", async () => {
      const configWithDropdown: AppConfig = {
        groups: [{
          name: 'database',
          title: 'Database Settings',
          items: [{
            name: 'db_type',
            type: 'dropdown',
            title: 'Database Type',
            default: 'postgres',
            items: [
              { name: 'postgres', title: 'PostgreSQL' },
              { name: 'mysql', title: 'MySQL' }
            ]
          }]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithDropdown, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      const dropdown = screen.getByTestId("dropdown-input-db_type") as HTMLSelectElement;

      // Change selection
      fireEvent.change(dropdown, { target: { value: 'mysql' } });

      // Verify value changed
      expect(dropdown.value).toBe('mysql');
    });

    it("renders select_one config type as radio buttons", async () => {
      const configWithSelectOne: AppConfig = {
        groups: [{
          name: 'auth',
          title: 'Authentication',
          items: [{
            name: 'auth_type',
            type: 'select_one',
            title: 'Authentication Type',
            value: 'basic',
            items: [
              { name: 'basic', title: 'Basic Auth' },
              { name: 'oauth', title: 'OAuth' },
              { name: 'saml', title: 'SAML' }
            ]
          }]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithSelectOne, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      // Should render as radio buttons
      expect(screen.getByTestId("radio-input-basic")).toBeInTheDocument();
      expect(screen.getByTestId("radio-input-oauth")).toBeInTheDocument();
      expect(screen.getByTestId("radio-input-saml")).toBeInTheDocument();

      // Verify basic is selected
      const basicRadio = screen.getByTestId("radio-input-basic") as HTMLInputElement;
      expect(basicRadio).toBeChecked();
    });

    it("renders heading config type with ARIA attributes", async () => {
      const configWithHeading: AppConfig = {
        groups: [{
          name: 'settings',
          title: 'Settings',
          items: [
            {
              name: 'section_1',
              type: 'heading',
              title: 'Database Settings'
            },
            {
              name: 'db_host',
              type: 'text',
              title: 'Host',
              value: 'localhost'
            },
            {
              name: 'section_2',
              type: 'heading',
              title: 'Advanced Options'
            },
            {
              name: 'debug',
              type: 'bool',
              title: 'Debug Mode',
              value: '0'
            }
          ]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithHeading, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      // Check heading renders
      const heading1 = screen.getByTestId("heading-section_1");
      expect(heading1).toBeInTheDocument();
      expect(heading1).toHaveTextContent('Database Settings');
      expect(heading1).toHaveAttribute('role', 'heading');
      expect(heading1).toHaveAttribute('aria-level', '3');

      const heading2 = screen.getByTestId("heading-section_2");
      expect(heading2).toBeInTheDocument();
      expect(heading2).toHaveTextContent('Advanced Options');
    });

    it("renders readonly text fields as disabled", async () => {
      const configWithReadonly: AppConfig = {
        groups: [{
          name: 'info',
          title: 'Information',
          items: [
            {
              name: 'cluster_id',
              type: 'text',
              title: 'Cluster ID',
              value: 'cluster-12345',
              readonly: true,
              help_text: 'This value cannot be changed'
            },
            {
              name: 'app_name',
              type: 'text',
              title: 'Application Name',
              value: 'My App',
              readonly: false
            }
          ]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithReadonly, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      // Check readonly field is disabled
      const readonlyInput = screen.getByTestId("text-input-cluster_id") as HTMLInputElement;
      expect(readonlyInput).toBeDisabled();
      expect(readonlyInput).toHaveValue('cluster-12345');

      // Check non-readonly field is not disabled
      const editableInput = screen.getByTestId("text-input-app_name") as HTMLInputElement;
      expect(editableInput).not.toBeDisabled();
      expect(editableInput).toHaveValue('My App');
    });

    it("renders readonly checkbox as disabled", async () => {
      const configWithReadonlyCheckbox: AppConfig = {
        groups: [{
          name: 'settings',
          title: 'Settings',
          items: [
            {
              name: 'feature_enabled',
              type: 'bool',
              title: 'Feature Enabled',
              value: '1',
              readonly: true
            }
          ]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithReadonlyCheckbox, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      const checkbox = screen.getByTestId("bool-input-feature_enabled") as HTMLInputElement;
      expect(checkbox).toBeDisabled();
      expect(checkbox).toBeChecked();
    });

    it("renders readonly radio buttons as disabled", async () => {
      const configWithReadonlyRadio: AppConfig = {
        groups: [{
          name: 'settings',
          title: 'Settings',
          items: [
            {
              name: 'auth_type',
              type: 'radio',
              title: 'Authentication Type',
              value: 'oauth',
              readonly: true,
              items: [
                { name: 'basic', title: 'Basic' },
                { name: 'oauth', title: 'OAuth' }
              ]
            }
          ]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithReadonlyRadio, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      const basicRadio = screen.getByTestId("radio-input-basic") as HTMLInputElement;
      const oauthRadio = screen.getByTestId("radio-input-oauth") as HTMLInputElement;

      expect(basicRadio).toBeDisabled();
      expect(oauthRadio).toBeDisabled();
      expect(oauthRadio).toBeChecked();
    });

    it("renders readonly dropdown as disabled", async () => {
      const configWithReadonlyDropdown: AppConfig = {
        groups: [{
          name: 'database',
          title: 'Database',
          items: [{
            name: 'db_type',
            type: 'dropdown',
            title: 'Database Type',
            value: 'postgres',
            readonly: true,
            items: [
              { name: 'postgres', title: 'PostgreSQL' },
              { name: 'mysql', title: 'MySQL' }
            ]
          }]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithReadonlyDropdown, target, mode)
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      const dropdown = screen.getByTestId("dropdown-input-db_type") as HTMLSelectElement;
      expect(dropdown).toBeDisabled();
      expect(dropdown.value).toBe('postgres');
    });

    it("submits dropdown values correctly", async () => {
      let submittedValues: { values: AppConfigValues } | null = null;

      const configWithDropdown: AppConfig = {
        groups: [{
          name: 'database',
          title: 'Database',
          items: [{
            name: 'db_type',
            type: 'dropdown',
            title: 'Database Type',
            default: 'postgres',
            items: [
              { name: 'postgres', title: 'PostgreSQL' },
              { name: 'mysql', title: 'MySQL' }
            ]
          }]
        }]
      };

      server.use(
        mockHandlers.appConfig.getTemplate(configWithDropdown, target, mode),
        mockHandlers.appConfig.updateValues(true, target, mode, (body) => {
        submittedValues = body as { values: AppConfigValues };
      })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
          mode: mode,
        },
      });

      await waitForForm();

      // Change dropdown value
      const dropdown = screen.getByTestId("dropdown-input-db_type") as HTMLSelectElement;
      fireEvent.change(dropdown, { target: { value: 'mysql' } });

      // Submit form
      const nextButton = screen.getByTestId("config-next-button");
      fireEvent.click(nextButton);

      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalled();
      }, { timeout: 3000 });

      // Verify submitted values
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!.values.db_type).toEqual({ value: 'mysql' });
    });
  });
});
