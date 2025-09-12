import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import AppPreflightPhase from '../phases/AppPreflightPhase.tsx';
import { withNextButtonOnly } from './TestWrapper.tsx';

const TestAppPreflightPhase = withNextButtonOnly(AppPreflightPhase);

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  // Mock start app installation endpoint
  http.post(`*/api/${target}/install/app/install`, () => {
    return HttpResponse.json({ success: true });
  }),
  // Mock app preflight run endpoint
  http.post(`*/api/${target}/install/app-preflights/run`, () => {
    return HttpResponse.json({ success: true });
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppPreflightPhase - $displayName', ({ target }) => {
  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it('enables Start Installation button when allowIgnoreAppPreflights is true and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreAppPreflights: true
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: true
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    }, { timeout: 2000 });

    // Button should be enabled when CLI flag allows ignoring failures
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    }, { timeout: 2000 });
  });

  it('disables Start Installation button when allowIgnoreAppPreflights is false and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreAppPreflights: false
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: false
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be disabled when CLI flag doesn't allow ignoring failures
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
      // Try to click disabled button - nothing should happen
      fireEvent.click(nextButton);
    });

    // No modal should appear and onNext should not be called
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('shows modal when Start Installation clicked and allowIgnoreAppPreflights is true and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreAppPreflights: true
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: true
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be enabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    }, { timeout: 2000 });

    // Click Next button - should show modal with warning
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Modal appears when allowIgnoreAppPreflights is true and preflights fail (but none are strict failures)
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    // User can choose to continue anyway
    const continueButton = screen.getByText('Continue Anyway');
    fireEvent.click(continueButton);

    // Should proceed to next step after confirming
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('proceeds automatically when allowIgnoreAppPreflights is true and preflights pass', async () => {
    // Mock preflight status endpoint - returns success with allowIgnoreAppPreflights: true
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [
              { title: 'Disk Space', message: 'Sufficient disk space available' }
            ]
          },
          allowIgnoreAppPreflights: true
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Button should be enabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });

    // Click Next button - should proceed directly without modal (normal success case)
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // No modal should appear when preflights pass
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('proceeds normally when allowIgnoreAppPreflights is false and preflights pass', async () => {
    // Mock preflight status endpoint - returns success with allowIgnoreAppPreflights: false
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [
              { title: 'Disk Space', message: 'Sufficient disk space available' }
            ]
          },
          allowIgnoreAppPreflights: false
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Button should be enabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });

    // Wait for button to be enabled and click it
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // No modal should appear when preflights pass
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  // Verify ignoreAppPreflights parameter is sent
  it('sends ignoreAppPreflights parameter when starting installation with failed preflights', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreAppPreflights: true
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [{ title: 'Disk Space', message: 'Not enough disk space available' }],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: true
        });
      }),
      // Mock app install endpoint to capture request body
      http.post(`*/api/${target}/install/app/install`, async ({ request }) => {
        const body = await request.json();

        // Verify the request includes ignoreAppPreflights parameter
        expect(body).toHaveProperty('ignoreAppPreflights', true);

        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Wait for button to be enabled and click it
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Confirm in modal
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    const continueButton = screen.getByText('Continue Anyway');
    fireEvent.click(continueButton);

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('sends ignoreAppPreflights false when starting installation with passed preflights', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [{ title: 'Disk Space', message: 'Sufficient disk space available' }]
          },
          allowIgnoreAppPreflights: false
        });
      }),
      // Mock app install endpoint to capture request body
      http.post(`*/api/${target}/install/app/install`, async ({ request }) => {
        const body = await request.json();

        // Verify the request includes ignoreAppPreflights parameter as false
        expect(body).toHaveProperty('ignoreAppPreflights', false);

        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Wait for button to be enabled and click it
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  // New tests for strict preflight blocking
  it('disables Start Installation button when strict failures exist regardless of allowIgnoreAppPreflights', async () => {
    // Mock preflight status endpoint - returns strict failures with allowIgnoreAppPreflights: true
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Critical Security Check', message: 'Security requirement not met', strict: true },
              { title: 'Disk Space', message: 'Not enough disk space available', strict: false }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: true,
          hasStrictAppPreflightFailures: true
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be disabled even though allowIgnoreAppPreflights is true
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });
  });

  it('does not show modal when strict failures exist because button is disabled', async () => {
    // Mock preflight status endpoint - returns strict failures
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Critical Security Check', message: 'Security requirement not met', strict: true }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: true,
          hasStrictAppPreflightFailures: true
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be disabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).toBeDisabled();
    });

    // Button is disabled for strict failures, so no modal should be possible
    // This test verifies the button is properly disabled and no modal interaction occurs
  });
});

// Additional robust frontend tests for error handling and edge cases
describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppPreflightPhase - Error Handling & Edge Cases - $displayName', ({ target }) => {
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
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
    // Mock preflight status endpoint - returns success
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreAppPreflights: false
        });
      }),
      // Mock app install endpoint to return API error
      http.post(`*/api/${target}/install/app/install`, () => {
        return HttpResponse.json(
          {
            statusCode: 400,
            message: 'Application preflight checks failed. Cannot proceed with installation.'
          },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Click Start Installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Should show error message instead of proceeding
    await waitFor(() => {
      expect(screen.getByText(/Application preflight checks failed. Cannot proceed with installation./)).toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles network failure during installation start', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreAppPreflights: false
        });
      }),
      // Mock app install endpoint to return network error
      http.post(`*/api/${target}/install/app/install`, () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Click Start Installation
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Should show network error message (matches actual fetch error)
    await waitFor(() => {
      expect(screen.getByText(/Failed to fetch/)).toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });
});

// Tests specifically for onStateChange callback
describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('AppPreflightPhase - onStateChange Tests - $displayName', ({ target }) => {
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer(target);
    server.listen();
  });

  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('calls onStateChange with "Running" immediately when component mounts', async () => {
    // Mock preflight status endpoint - returns running state initially
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Running' },
          output: { fail: [], warn: [], pass: [] },
          allowIgnoreAppPreflights: false
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');
    expect(mockOnStateChange).toHaveBeenCalledTimes(1);
  });

  it('calls onStateChange with "Succeeded" when preflights complete successfully', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [
              { title: 'Disk Space', message: 'Sufficient disk space available' }
            ]
          },
          allowIgnoreAppPreflights: false
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Clear the mock after the initial mount call
    mockOnStateChange.mockClear();

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Application validation successful!')).toBeInTheDocument();
    });

    // Should call onStateChange exactly twice: once for onRun('Running') and once for final state
    expect(mockOnStateChange).toHaveBeenCalledWith('Running'); // from onRun
    expect(mockOnStateChange).toHaveBeenCalledWith('Succeeded'); // from onComplete
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });

  it('calls onStateChange with "Failed" when preflights complete with failures', async () => {
    // Mock preflight status endpoint - returns failures
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: false
        });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Clear the mock after the initial mount call
    mockOnStateChange.mockClear();

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Should call onStateChange exactly twice: once for onRun('Running') and once for final state
    expect(mockOnStateChange).toHaveBeenCalledWith('Running'); // from onRun
    expect(mockOnStateChange).toHaveBeenCalledWith('Failed'); // from onComplete
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });

  it('calls onStateChange("Running") when rerun button is clicked', async () => {
    // Mock preflight status to show failures initially
    server.use(
      http.get(`*/api/${target}/install/app-preflights/status`, () => {
        return HttpResponse.json({
          titles: ['App Check'],
          status: { state: 'Failed' },
          output: {
            fail: [{ title: 'Disk Space', message: 'Not enough disk space' }],
            warn: [],
            pass: []
          },
          allowIgnoreAppPreflights: false
        });
      }),
      // Mock preflight run endpoint
      http.post(`*/api/${target}/install/app-preflights/run`, () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <TestAppPreflightPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      { wrapperProps: { target, authenticated: true } }
    );

    // Wait for initial failures to load
    await waitFor(() => {
      expect(screen.getByText('Application Requirements Not Met')).toBeInTheDocument();
    });

    // Clear the initial onStateChange calls
    mockOnStateChange.mockClear();

    // Find and click the "Run Validation Again" button
    const runValidationButton = screen.getByRole('button', { name: 'Run Validation Again' });
    fireEvent.click(runValidationButton);

    // Should call onStateChange('Running') when onRun is triggered
    await waitFor(() => {
      expect(mockOnStateChange).toHaveBeenCalledWith('Running');
    });
  });
});