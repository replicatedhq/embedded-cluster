import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import WelcomeStep from "../WelcomeStep.tsx";

const server = setupServer(
  // Mock login endpoint
  http.post("*/api/auth/login", async ({ request }) => {
    console.log("Mock server received request to /api/auth/login");
    const body = (await request.json()) as { password: string };
    console.log("Request body:", body);
    if (body?.password === "password") {
      console.log("Password matched, returning success");
      return new HttpResponse(JSON.stringify({ token: "mock-token" }), {
        status: 200,
        headers: {
          "Content-Type": "application/json",
        },
      });
    }
    console.log("Password did not match, returning 401");
    return new HttpResponse(JSON.stringify({ message: "Invalid password" }), {
      status: 401,
      headers: {
        "Content-Type": "application/json",
      },
    });
  }),

  // Catch-all handler for any other requests
  http.all("*", ({ request }) => {
    console.log("Unhandled request:", request.method, request.url);
    return new HttpResponse(null, { status: 404 });
  })
);

describe("WelcomeStep", () => {
  const mockOnNext = vi.fn();

  beforeAll(() => {
    server.listen({ onUnhandledRequest: "warn" });
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it("renders welcome content correctly", () => {
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Check if welcome content is rendered
    expect(screen.getByTestId("welcome-step")).toBeInTheDocument();
    expect(screen.getByTestId("password-input")).toBeInTheDocument();
  });

  it("handles password submission successfully", async () => {
    server.use(
      http.post("*/api/auth/login", () => {
        return new HttpResponse(JSON.stringify({ token: "mock-token" }), {
          status: 200,
          headers: {
            "Content-Type": "application/json",
          },
        });
      })
    );
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in password
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "password" } });

    // Submit form
    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for the mutation to complete and verify onNext was called
    await waitFor(
      () => {
        expect(mockOnNext).toHaveBeenCalled();
      },
      { timeout: 3000 }
    );
  });

  it("shows error message for invalid password", async () => {
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in incorrect password
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "wrong-password" } });

    // Submit form
    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for error message
    await waitFor(() => {
      expect(screen.getByText(/Incorrect password/)).toBeInTheDocument();
    });

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("does not retry on 401 authentication errors", async () => {
    let requestCount = 0;
    server.use(
      http.post("*/api/auth/login", () => {
        requestCount++;
        return new HttpResponse(JSON.stringify({ message: "Invalid password" }), {
          status: 401,
          headers: {
            "Content-Type": "application/json",
          },
        });
      })
    );

    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in password and submit
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "wrong-password" } });

    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for error to appear
    await waitFor(() => {
      expect(screen.getByText(/Incorrect password/)).toBeInTheDocument();
    });

    // Should only make one request, no retries for 401
    expect(requestCount).toBe(1);
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("retries once for server errors then shows error", async () => {
    let requestCount = 0;
    server.use(
      http.post("*/api/auth/login", () => {
        requestCount++;
        return new HttpResponse(JSON.stringify({ message: "Internal server error" }), {
          status: 500,
          headers: {
            "Content-Type": "application/json",
          },
        });
      })
    );

    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in password and submit
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "password" } });

    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for error to appear
    await waitFor(() => {
      expect(screen.getByText(/Login failed/)).toBeInTheDocument();
    }, { timeout: 5000 });

    // Should make 2 requests: initial + 1 retry
    expect(requestCount).toBe(2);
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("retries once for network errors then succeeds", async () => {
    let requestCount = 0;
    server.use(
      http.post("*/api/auth/login", () => {
        requestCount++;
        if (requestCount === 1) {
          // First request fails with 503
          return new HttpResponse(JSON.stringify({ message: "Service unavailable" }), {
            status: 503,
            headers: {
              "Content-Type": "application/json",
            },
          });
        } else {
          // Second request (retry) succeeds
          return new HttpResponse(JSON.stringify({ token: "mock-token" }), {
            status: 200,
            headers: {
              "Content-Type": "application/json",
            },
          });
        }
      })
    );

    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in password and submit
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "password" } });

    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for successful login
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalled();
    }, { timeout: 5000 });

    // Should make 2 requests: initial failure + 1 successful retry
    expect(requestCount).toBe(2);
  });

  it("shows error message for empty password", async () => {
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />);

    // Fill in incorrect password
    const passwordInput = screen.getByTestId("password-input");
    fireEvent.change(passwordInput, { target: { value: "" } });

    // Submit form
    const submitButton = screen.getByTestId("welcome-button-next");
    fireEvent.click(submitButton);

    // Wait for error message
    await waitFor(() => {
      expect(screen.getByText(/Incorrect password/)).toBeInTheDocument();
    });

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

});
