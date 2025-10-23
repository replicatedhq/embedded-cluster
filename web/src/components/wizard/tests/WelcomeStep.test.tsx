import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from "vitest";
import { screen, fireEvent, waitFor } from "@testing-library/react";
import { setupServer } from "msw/node";
import { renderWithProviders } from "../../../test/setup.tsx";
import { mockHandlers, createHandler } from "../../../test/mockHandlers.ts";
import WelcomeStep from "../WelcomeStep.tsx";

const server = setupServer(
  // Default: Mock login endpoint returns 401 for incorrect password
  // Individual tests override this as needed
  mockHandlers.auth.login(false)
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
    server.use(mockHandlers.auth.login(true, "mock-token"));

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
    const callCounter = { callCount: 0 };
    server.use(
      createHandler.loginWithCounter(401, "Invalid password", callCounter)
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
    expect(callCounter.callCount).toBe(1);
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("retries once for server errors then shows error", async () => {
    const callCounter = { callCount: 0 };
    server.use(
      createHandler.loginWithCounter(500, "Internal server error", callCounter)
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
    expect(callCounter.callCount).toBe(2);
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it("retries once for network errors then succeeds", async () => {
    const callCounter = { callCount: 0 };
    server.use(
      createHandler.loginRetrySuccess(callCounter, 503, "Service unavailable")
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
    expect(callCounter.callCount).toBe(2);
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
