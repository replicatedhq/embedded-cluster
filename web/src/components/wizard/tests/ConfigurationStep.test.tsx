import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ConfigurationStep from "../config/ConfigurationStep.tsx";
import { AppConfig, AppConfigGroup, AppConfigItem } from "../../../types";

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
          default: "Default App"
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
        }
      ]
    }
  ]
};

const createMockConfigWithValues = (values: Record<string, string>): AppConfig => {
  const config: AppConfig = JSON.parse(JSON.stringify(MOCK_APP_CONFIG));
  config.groups.forEach((group: AppConfigGroup) => {
    group.items.forEach((item: AppConfigItem) => {
      if (values[item.name]) {
        item.value = values[item.name];
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
  http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
    const body = await request.json() as { values: Record<string, string> };
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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
    });

    // Check main container is rendered
    expect(screen.getByTestId("configuration-step")).toBeInTheDocument();

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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
    });

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
  });

  it("handles text input changes correctly", async () => {
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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
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
      http.post(`*/api/${target}/install/app/config/values`, () => {
        return new HttpResponse(JSON.stringify({ message: "Invalid configuration values" }), { status: 400 });
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
    let submittedValues: { values: Record<string, string> } | null = null;

    server.use(
      http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        const body = await request.json() as { values: Record<string, string> };
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
        app_name: "Updated App Name",
        description: "Updated multi-line\ndescription text",
        enable_feature: "1",
        auth_type: "auth_type_anonymous",
        db_host: "Updated DB Host",
        db_config: "# Updated config\nhost: updated-host\nport: 5432"
      }
    });
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
    let submittedValues: { values: Record<string, string> } | null = null;

    server.use(
      http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: Record<string, string> };
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
        app_name: "Only Changed Field",
        description: "Only changed description",
        auth_type: "auth_type_anonymous"
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
      expect(screen.getByTestId("configuration-step")).toBeInTheDocument();
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
              }
            ]
          }
        ]
      };

      // Override the server to return our comprehensive config
      server.use(
        http.get(`*/api/${target}/install/app/config`, () => {
          return HttpResponse.json(comprehensiveConfig);
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

      // Check that all radio groups are rendered
      expect(screen.getByTestId("config-item-authentication_method")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-database_type")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-logging_level")).toBeInTheDocument();
      expect(screen.getByTestId("config-item-ssl_mode")).toBeInTheDocument();

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

      // Test radio button selection behavior
      fireEvent.click(localAuthRadio);
      expect(localAuthRadio).toBeChecked();
      expect(ldapAuthRadio).not.toBeChecked();

      // Test radio group behavior (only one can be selected)
      fireEvent.click(ldapAuthRadio);
      expect(localAuthRadio).not.toBeChecked();
      expect(ldapAuthRadio).toBeChecked();

      // Test form submission with radio button changes
      let submittedValues: { values: Record<string, string> } | null = null;
      server.use(
        http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
          const body = await request.json() as { values: Record<string, string> };
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
          database_type: "database_type_mysql"
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

      // Wait for loading to complete
      await waitFor(() => {
        expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
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

  it("initializes changed values from retrieved config values and only submits retrieved values plus changes", async () => {
    // Mock the config values endpoint to return only a subset of values
    const retrievedConfigValues = {
      app_name: "Retrieved App Name",
      auth_type: "auth_type_anonymous"
      // Note: enable_feature and db_host are NOT in retrieved values
    };

    let submittedValues: { values: Record<string, string> } | null = null;

    server.use(
      http.get(`*/api/${target}/install/app/config/values`, () => {
        return HttpResponse.json({ values: retrievedConfigValues });
      }),
      http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as { values: Record<string, string> };
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

    // Wait for loading to complete
    await waitFor(() => {
      expect(screen.queryByTestId("configuration-step-loading")).not.toBeInTheDocument();
    });

    // Make changes to form fields
    const appNameInput = screen.getByTestId("text-input-app_name");
    fireEvent.change(appNameInput, { target: { value: "Updated App Name" } });

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

    // Verify that only retrieved values + changes are submitted
    expect(submittedValues).not.toBeNull();
    expect(submittedValues!).toMatchObject({
      values: {
        // Retrieved values that were initialized
        app_name: "Updated App Name", // changed from retrieved value
        auth_type: "auth_type_anonymous", // unchanged retrieved value
        // New changes to fields not in retrieved values
        db_host: "new-db-host"
        // enable_feature should NOT be submitted since it wasn't retrieved and wasn't changed
      }
    });
    
    // Explicitly verify enable_feature is not submitted
    expect(submittedValues!.values).not.toHaveProperty("enable_feature");
  });
});