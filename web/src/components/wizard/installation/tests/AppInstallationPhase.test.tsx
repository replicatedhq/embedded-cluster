import { describe, it, expect, vi, beforeAll, beforeEach, afterEach, afterAll } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import AppInstallationPhase from '../phases/AppInstallationPhase.tsx';
import { withNextButtonOnly } from './TestWrapper.tsx';
import { mockHandlers, createHandler, type Target, type Mode } from '../../../../test/mockHandlers.ts';

const TestAppInstallationPhase = withNextButtonOnly(AppInstallationPhase);

const createServer = (target: Target, mode: Mode = 'install') => setupServer(
  mockHandlers.app.getStatus('Running', target, mode)
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppInstallationPhase - $displayName', ({ target }) => {
  describe.each([
    { mode: "install" as const, modeDisplayName: "Install Mode" },
    { mode: "upgrade" as const, modeDisplayName: "Upgrade Mode" }
  ])('$modeDisplayName', ({ mode }) => {
    const mockOnNext = vi.fn();
    const mockOnStateChange = vi.fn();
    let server: ReturnType<typeof createServer>;

    beforeAll(() => {
      server = createServer(target, mode);
      server.listen();
    });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it('calls onStateChange with "Running" immediately when component mounts', async () => {
    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Running" immediately on mount
    const calls = mockOnStateChange.mock.calls.map(args => args[0]);
    expect(calls).toEqual(['Running']);
    expect(mockOnStateChange).toHaveBeenCalledTimes(1);
  });

  it('shows loading state during installation', async () => {
    server.use(
      mockHandlers.app.getStatus('Running', target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show loading state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-loading')).toBeInTheDocument();
      expect(screen.getByTestId('app-installation-loading-description')).toBeInTheDocument();
    });

    // Next button should be disabled during installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('shows success state when installation completes successfully', async () => {
    server.use(
      mockHandlers.app.getStatus('Succeeded', target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show success state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-success')).toBeInTheDocument();
    });

    // Next button should be enabled after successful installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });

    // Should call onStateChange with "Succeeded"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toEqual(['Running', 'Succeeded']);
      expect(mockOnStateChange).toHaveBeenCalledTimes(2);
    });
  });

  it('shows error state when installation fails', async () => {
    server.use(
      mockHandlers.app.getStatus('Failed', target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show error state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-error')).toBeInTheDocument();
      expect(screen.getByTestId('app-installation-error-message')).toBeInTheDocument();
    });

    // Next button should be disabled after failed installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });

    // Should call onStateChange with "Failed"
    await waitFor(() => {
      const calls = mockOnStateChange.mock.calls.map(args => args[0]);
      expect(calls).toEqual(['Running', 'Failed']);
      expect(mockOnStateChange).toHaveBeenCalledTimes(2);
    });
  });

  it('handles API error gracefully', async () => {
    server.use(
      mockHandlers.app.getStatus(
        { error: { statusCode: 500, message: 'Internal server error' } },
        target,
        mode
      )
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show error message when API fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('handles network error gracefully', async () => {
    server.use(
      mockHandlers.app.getStatus({ networkError: true }, target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show error message when network fails
    await waitFor(() => {
      expect(screen.getByTestId('error-message')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('shows default loading state when no status is available', async () => {
    server.use(
      mockHandlers.app.getStatus({ empty: true }, target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should show default loading state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-loading')).toBeInTheDocument();
    });

    // Next button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('stops polling when installation succeeds', async () => {
    const callCounter = { callCount: 0 };
    server.use(
      createHandler.withCallCounter(
        `*/api/${target}/${mode}/app/status`,
        { status: { state: 'Succeeded' } },
        callCounter
      )
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-success')).toBeInTheDocument();
    });

    const initialCallCount = callCounter.callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after success
    expect(callCounter.callCount).toBeLessThanOrEqual(initialCallCount + 1); // Allow for one potential additional call due to timing
  });

  it('stops polling when installation fails', async () => {
    const callCounter = { callCount: 0 };
    server.use(
      createHandler.withCallCounter(
        `*/api/${target}/${mode}/app/status`,
        { status: { state: 'Failed' } },
        callCounter
      )
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Wait for failure state
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-error')).toBeInTheDocument();
    });

    const initialCallCount = callCounter.callCount;

    // Wait a bit more and ensure no additional calls are made
    await new Promise(resolve => setTimeout(resolve, 3000));

    // Should not make additional API calls after failure
    expect(callCounter.callCount).toBeLessThanOrEqual(initialCallCount + 1); // Allow for one potential additional call due to timing
  });

  it('displays custom status description when available', async () => {
    const customDescription = 'Configuring application settings and finalizing setup...';

    server.use(
      mockHandlers.app.getStatus(
        { state: 'Running', description: customDescription },
        target,
        mode
      )
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should display custom description
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-loading-description')).toBeInTheDocument();
    });
  });

  it('displays fallback message when no status description is available', async () => {
    server.use(
      mockHandlers.app.getStatus({ state: 'Running' }, target, mode)
    );

    renderWithProviders(
      <TestAppInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
        ignoreAppPreflights={false}
      />,
      {
        wrapperProps: {
          target,
          mode,
          authenticated: true
        }
      }
    );

    // Should display fallback message
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-loading-description')).toBeInTheDocument();
    });
  });

  }); // end of mode describe
}); // end of target describe

// Tests for mutation behavior (errors, success, validation)
describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppInstallationPhase - Mutation Behavior - $displayName', ({ target }) => {
  describe.each([
    { mode: "install" as const, modeDisplayName: "Install Mode" },
    { mode: "upgrade" as const, modeDisplayName: "Upgrade Mode" }
  ])('$modeDisplayName', ({ mode }) => {
    let server: ReturnType<typeof createServer>;

    beforeAll(() => {
      server = createServer(target, mode);
      server.listen();
    });

    afterEach(() => server.resetHandlers());
    afterAll(() => server.close());

    const mockOnNext = vi.fn();
    const mockOnStateChange = vi.fn();

    beforeEach(() => {
      vi.clearAllMocks();
    });

    it('handles API error responses gracefully when starting installation', async () => {
      // Mock app status endpoint to return Pending state to trigger mutation
      server.use(
        mockHandlers.app.getStatus('Pending', target, mode),
        // Mock app install/upgrade endpoint to return API error
        mockHandlers.app.start(
          { error: { statusCode: 400, message: 'Application preflight checks failed. Cannot proceed with installation.' } },
          target,
          mode
        )
      );

      renderWithProviders(
        <TestAppInstallationPhase
          onNext={mockOnNext}
          onStateChange={mockOnStateChange}
          ignoreAppPreflights={false}
        />,
        { wrapperProps: { target, mode, authenticated: true } }
      );

      // Should show error message when mutation fails
      await waitFor(() => {
        expect(screen.getByTestId('error-message')).toBeInTheDocument();
      });

      // Should call onStateChange with "Failed"
      expect(mockOnStateChange).toHaveBeenCalledWith('Failed');

      // Should NOT proceed to next step
      expect(mockOnNext).not.toHaveBeenCalled();
    });

    it('handles network failure during installation start', async () => {
      // Mock app status endpoint to return Pending state to trigger mutation
      server.use(
        mockHandlers.app.getStatus('Pending', target, mode),
        // Mock app install/upgrade endpoint to return network error
        mockHandlers.app.start({ networkError: true }, target, mode)
      );

      renderWithProviders(
        <TestAppInstallationPhase
          onNext={mockOnNext}
          onStateChange={mockOnStateChange}
          ignoreAppPreflights={false}
        />,
        { wrapperProps: { target, mode, authenticated: true } }
      );

      // Should show error message when network fails
      await waitFor(() => {
        expect(screen.getByTestId('error-message')).toBeInTheDocument();
      });

      // Should call onStateChange with "Failed"
      expect(mockOnStateChange).toHaveBeenCalledWith('Failed');

      // Should NOT proceed to next step
      expect(mockOnNext).not.toHaveBeenCalled();
    });
  });
});
