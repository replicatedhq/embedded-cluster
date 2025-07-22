import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ConfigurationStep from "../config/ConfigurationStep.tsx";
import { AppConfig, AppConfigGroup, AppConfigItem, AppConfigValues } from "../../../types";

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
    const updatedConfig = createMockConfigWithValues(body.values);
    return HttpResponse.json(updatedConfig);
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])("ConfigurationStep - $displayName", ({ target }) => {
  const mockOnNext = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
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
      },
    });

    // Check initial loading state
    expect(screen.getByTestId("configuration-step-loading")).toBeInTheDocument();

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
      },
    });

    // Check loading state
    expect(screen.getByTestId("configuration-step-loading")).toBeInTheDocument();
  });

  it("handles config fetch error gracefully", async () => {
    server.use(
      http.get(`*/api/${target}/install/app/config`, () => {
        return new HttpResponse(JSON.stringify({ message: "Failed to fetch config" }), { status: 500 });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch config")).toBeInTheDocument();
  });

  it("handles config values fetch error gracefully", async () => {
    server.use(
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return new HttpResponse(JSON.stringify({ message: "Failed to fetch config values" }), { status: 500 });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Failed to fetch config values")).toBeInTheDocument();
  });


  it("handles empty config gracefully", async () => {
    server.use(
      http.get(`*/api/${target}/install/app/config`, () => {
        return HttpResponse.json({ spec: { groups: [] } });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
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
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("config-item-app_name")).toBeInTheDocument();
    });

    // Initially, Settings tab should be active
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
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

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
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("bool-input-enable_feature")).toBeInTheDocument();
    });

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
      http.patch(`*/api/${target}/install/app/config/values`, () => {
        return new HttpResponse(JSON.stringify({ message: "Invalid configuration values" }), { status: 400 });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
      http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        const body = await request.json() as { values: AppConfigValues };
        submittedValues = body;
        const updatedConfig = createMockConfigWithValues(body.values);
        return HttpResponse.json(updatedConfig);
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

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
    expect(submittedValues!).toMatchObject({
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
      },
    });

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

    // Verify help text is displayed for the app_name field with default value
    expect(screen.getByText(/Enter the name of your application/)).toBeInTheDocument();
    expect(screen.getByText("Default App")).toBeInTheDocument();
  });

  it("handles unauthorized error correctly", async () => {
    server.use(
      http.get(`*/api/${target}/install/app/config`, () => {
        return new HttpResponse(JSON.stringify({ message: "Unauthorized" }), { status: 401 });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for error to be displayed
    await waitFor(() => {
      expect(screen.getByTestId("configuration-step-error")).toBeInTheDocument();
    });
    expect(screen.getByText("Failed to load configuration")).toBeInTheDocument();
    expect(screen.getByText("Session expired. Please log in again.")).toBeInTheDocument();
  });

  it("only submits changed values", async () => {
    let submittedValues: { values: AppConfigValues } | null = null;

    server.use(
      http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: AppConfigValues };
        submittedValues = body;
        const updatedConfig = createMockConfigWithValues(body.values);
        return HttpResponse.json(updatedConfig);
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.getByTestId("text-input-app_name")).toBeInTheDocument();
    });

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
    expect(submittedValues!).toMatchObject({
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
      http.get(`*/api/${target}/install/app/config`, () => {
        return HttpResponse.json(configWithEmptyValues);
      }),
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: {} }); // No changed values
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
        http.get(`*/api/${target}/install/app/config`, () => {
          return HttpResponse.json(comprehensiveConfig);
        }),
        http.get(`*/api/${target}/install/app/config/values`, () => {
          // Provide config values to test priority: configValues > item.value > item.default
          return HttpResponse.json({
            values: {
              notification_method: { value: "notification_method_slack" }, // configValues overrides schema default "notification_method_email"
              backup_schedule: { value: "backup_schedule_weekly" } // configValues overrides schema value "backup_schedule_daily"
            }
          });
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        // Check that all radio groups are rendered
        expect(screen.getByTestId("config-item-authentication_method")).toBeInTheDocument();
      });

      // Check that all radio groups are rendered
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
      expect(emailNotificationRadio).not.toBeChecked();
      expect(slackNotificationRadio).toBeChecked(); // configValues "notification_method_slack" overrides default "notification_method_email"

      // Test scenario 6: Has value, but configValues overrides (configValues should be selected)
      const dailyBackupRadio = screen.getByTestId("radio-input-backup_schedule_daily") as HTMLInputElement;
      const weeklyBackupRadio = screen.getByTestId("radio-input-backup_schedule_weekly") as HTMLInputElement;
      expect(dailyBackupRadio).not.toBeChecked();
      expect(weeklyBackupRadio).toBeChecked(); // configValues "backup_schedule_weekly" overrides value "backup_schedule_daily"

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
        http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
          const body = await request.json() as { values: AppConfigValues };
          submittedValues = body;
          return HttpResponse.json(comprehensiveConfig);
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

      // Verify the radio button change was submitted
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!).toMatchObject({
        values: {
          database_type: { value: "database_type_mysql" }
        }
      });
    });

    it("handles radio button group behavior (only one can be selected)", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Wait for radio buttons to appear
      await waitFor(() => {
        expect(screen.getByTestId("config-item-auth_type")).toBeInTheDocument();
      });

      // Get all radio buttons in the authentication group
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
        },
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
      });

      // Check that label config items are rendered
      expect(screen.getByTestId("label-info_label")).toBeInTheDocument();
      expect(screen.getByTestId("label-markdown_label")).toBeInTheDocument();
    });

    it("renders label content with automatic link detection", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
      });

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
        },
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
      });

      // Wait for label items to be rendered in the active tab
      await waitFor(() => {
        expect(screen.getByTestId("label-info_label")).toBeInTheDocument();
      });

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
        },
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
      });

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
        http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
          const body = await request.json() as { values: AppConfigValues };
          submittedValues = body;
          const updatedConfig = createMockConfigWithValues(body.values);
          return HttpResponse.json(updatedConfig);
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
      });

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
      expect(submittedValues!).toMatchObject({
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

      // Override the server to return our comprehensive config
      server.use(
        http.get(`*/api/${target}/install/app/config`, () => {
          return HttpResponse.json(comprehensiveConfig);
        }),
        http.get(`*/api/${target}/install/app/config/values`, () => {
          // Provide config values to test priority: configValues > item.value > item.default
          return HttpResponse.json({
            values: {
              notification_method: { value: "1" }, // configValues overrides schema default "0"
              backup_schedule: { value: "1" } // configValues overrides schema value "0"
            }
          });
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

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
      expect(notificationMethodCheckbox).toBeChecked(); // configValues "1" overrides default "0"

      // Test scenario 6: Has value, but configValues overrides (configValues should be used)
      const backupScheduleCheckbox = screen.getByTestId("bool-input-backup_schedule") as HTMLInputElement;
      expect(backupScheduleCheckbox).toBeChecked(); // configValues "1" overrides value "0"

      // Test checkbox toggling behavior
      fireEvent.click(authenticationMethodCheckbox);
      expect(authenticationMethodCheckbox).not.toBeChecked();

      fireEvent.click(authenticationMethodCheckbox);
      expect(authenticationMethodCheckbox).toBeChecked();

      // Test form submission with checkbox changes
      let submittedValues: { values: AppConfigValues } | null = null;
      server.use(
        http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
          const body = await request.json() as { values: AppConfigValues };
          submittedValues = body;
          return HttpResponse.json(comprehensiveConfig);
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

      // Verify the checkbox change was submitted
      expect(submittedValues).not.toBeNull();
      expect(submittedValues!).toMatchObject({
        values: {
          database_type: { value: "0" } // changed from checked to unchecked
        }
      });
    });
  });

  it("initializes changed values from retrieved config values and only submits changed values (PATCH behavior)", async () => {
    // Mock the config values endpoint to return only a subset of values
    const retrievedConfigValues = {
      app_name: { value: "Retrieved App Name" },
      auth_type: { value: "auth_type_anonymous" }
      // Note: enable_feature and db_host are NOT in retrieved values
    };

    let submittedValues: { values: AppConfigValues } | null = null;

    server.use(
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: retrievedConfigValues });
      }),
      http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: AppConfigValues };
        submittedValues = body;
        return HttpResponse.json(MOCK_APP_CONFIG);
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
    expect(submittedValues!).toMatchObject({
      values: {
        // Retrieved values that were changed
        app_name: { value: "" }, // cleared to empty string (should be submitted for deletion)
        // New changes to fields not in retrieved values
        db_host: { value: "new-db-host" }
        // auth_type should NOT be submitted since it wasn't changed
        // enable_feature should NOT be submitted since it wasn't retrieved and wasn't changed
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
              value: "masked_value",
              default: "default_password"
            }
          ]
        }
      ]
    };

    server.use(
      http.get(`*/api/${target}/install/app/config`, () => {
        return HttpResponse.json(configWithPassword);
      }),
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: { user_password: { value: "••••••••" } } });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

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
      http.get(`*/api/${target}/install/app/config`, () => {
        return HttpResponse.json(configWithDefaults);
      }),
      http.get(`*/api/${target}/install/app/config/values`, () => {
        // API returns empty string for db_name, nothing for db_password or db_port
        return HttpResponse.json({ values: { db_name: { value: "" } } });
      })
    );

    renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
      wrapperProps: {
        authenticated: true,
        target: target,
      },
    });

    // Wait for the content to be rendered
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
    });

    // Field with empty string in configValues should show empty string (not schema value or default)
    const dbNameField = screen.getByTestId("text-input-db_name") as HTMLInputElement;
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
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

      // Check that the file input is rendered
      expect(screen.getByTestId("config-item-ssl_certificate")).toBeInTheDocument();
      expect(screen.getByTestId("file-input-ssl_certificate")).toBeInTheDocument();
    });

    it("handles file upload, displays filename, and removes file", async () => {
      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

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
        http.patch(`*/api/${target}/install/app/config/values`, async ({ request }) => {
          const body = await request.json() as { values: AppConfigValues };
          submittedValues = body;
          const updatedConfig = createMockConfigWithValues(body.values);
          return HttpResponse.json(updatedConfig);
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
        },
      });

      // Wait for the content to be rendered
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step")).toBeInTheDocument();
      });

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
  });

  describe("Required field validation", () => {
    it("shows error message for required text field when empty on submit", async () => {
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
        http.get(`*/api/${target}/install/app/config`, () => {
          return HttpResponse.json(configWithRequiredField);
        }),
        http.get(`*/api/${target}/install/app/config/values`, () => {
          return HttpResponse.json({ values: {} });
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
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

      // Wait for validation error to appear
      await waitFor(() => {
        expect(screen.getByText("Required Field is required")).toBeInTheDocument();
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
        http.get(`*/api/${target}/install/app/config`, () => {
          return HttpResponse.json(configWithMultipleRequiredFields);
        }),
        http.get(`*/api/${target}/install/app/config/values`, () => {
          return HttpResponse.json({ values: {} });
        })
      );

      renderWithProviders(<ConfigurationStep onNext={mockOnNext} />, {
        wrapperProps: {
          authenticated: true,
          target: target,
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

      // Wait for validation errors to appear
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
  });
});
