import React from "react";
import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import WelcomeStep from "../WelcomeStep.tsx";
import { MOCK_PROTOTYPE_SETTINGS } from "../../../test/testData.ts";

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
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Check if welcome content is rendered
    expect(screen.getByText(/Welcome to/)).toBeInTheDocument();
    expect(screen.getByText(/Enter Password/)).toBeInTheDocument();
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
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Fill in password
    const passwordInput = screen.getByLabelText(/Enter Password/);
    fireEvent.change(passwordInput, { target: { value: "password" } });

    // Submit form
    const submitButton = screen.getByRole("button", { name: /Start/ });
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
    renderWithProviders(<WelcomeStep onNext={mockOnNext} />, {
      wrapperProps: {
        preloadedState: {
          prototypeSettings: MOCK_PROTOTYPE_SETTINGS,
        },
      },
    });

    // Fill in incorrect password
    const passwordInput = screen.getByLabelText(/Enter Password/);
    fireEvent.change(passwordInput, { target: { value: "wrong-password" } });

    // Submit form
    const submitButton = screen.getByRole("button", { name: /Start/ });
    fireEvent.click(submitButton);

    // Wait for error message
    await waitFor(() => {
      expect(screen.getByText(/Invalid password/)).toBeInTheDocument();
    });

    // Verify onNext was not called
    expect(mockOnNext).not.toHaveBeenCalled();
  });
});
