import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from "vitest";
import { screen, waitFor, fireEvent } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import ConfigurationStep from "../config/ConfigurationStep.tsx";

const MOCK_APP_CONFIG = {
  spec: {
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
            name: "enable_feature",
            title: "Enable Feature",
            type: "bool",
            value: "0",
            default: "0"
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
          }
        ]
      }
    ]
  }
};

const createMockConfigWithValues = (values: Record<string, string>) => {
  const config = JSON.parse(JSON.stringify(MOCK_APP_CONFIG));
  config.spec.groups.forEach((group: any) => {
    group.items.forEach((item: any) => {
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

  // Mock config values submission endpoint
  http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
    const body = await request.json() as any;
    const updatedConfig = createMockConfigWithValues(body.values);
    return HttpResponse.json(updatedConfig);
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])("ConfigurationStep - $displayName", ({ target }) => {
  const mockOnNext = vi.fn();
  let server: any;

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
    expect(screen.getByLabelText("Application Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Enable Feature")).toBeInTheDocument();

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
    expect(screen.getByLabelText("Application Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Enable Feature")).toBeInTheDocument();

    // Click on Database tab
    fireEvent.click(screen.getByTestId("config-tab-database"));

    // Database tab content should be visible
    expect(screen.getByLabelText("Database Host")).toBeInTheDocument();

    // Settings tab content should not be visible
    expect(screen.queryByLabelText("Application Name")).not.toBeInTheDocument();
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
    const appNameInput = screen.getByLabelText("Application Name");
    fireEvent.change(appNameInput, { target: { value: "New App Name" } });

    // Verify the value was updated
    expect(appNameInput).toHaveValue("New App Name");
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
    const enableFeatureCheckbox = screen.getByLabelText("Enable Feature");
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
    let submittedValues: any = null;

    server.use(
      http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        // Verify auth header
        expect(request.headers.get("Authorization")).toBe("Bearer test-token");
        const body = await request.json() as any;
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
    const appNameInput = screen.getByLabelText("Application Name");
    fireEvent.change(appNameInput, { target: { value: "Updated App Name" } });

    const enableFeatureCheckbox = screen.getByLabelText("Enable Feature");
    fireEvent.click(enableFeatureCheckbox);

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
    expect(submittedValues).toMatchObject({
      values: {
        app_name: "Updated App Name",
        enable_feature: "1"
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
    let submittedValues: any = null;

    server.use(
      http.post(`*/api/${target}/install/app/config/values`, async ({ request }) => {
        const body = await request.json() as any;
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

    // Only change one field
    const appNameInput = screen.getByLabelText("Application Name");
    fireEvent.change(appNameInput, { target: { value: "Only Changed Field" } });

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

    // Verify only the changed value was submitted
    expect(submittedValues).toMatchObject({
      values: {
        app_name: "Only Changed Field"
      }
    });
    expect(submittedValues.values).not.toHaveProperty("enable_feature");
  });
});