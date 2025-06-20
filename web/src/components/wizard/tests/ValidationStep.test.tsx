import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import React from 'react';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../test/setup.tsx';
import ValidationStep from '../ValidationStep';

const server = setupServer(
  // Mock installation status endpoint
  http.get('*/api/install/installation/status', () => {
    return HttpResponse.json({
      state: 'Succeeded',
      description: 'Installation initialized',
      lastUpdated: '2024-01-01T00:00:00Z',
    });
  }),

  // Mock start installation endpoint
  http.post('*/api/install/infra/setup', () => {
    return HttpResponse.json({ success: true });
  })
);

describe('ValidationStep', () => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  it('enables Start Installation button when allowIgnoreHostPreflights is true and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreHostPreflights: true
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be enabled when CLI flag allows ignoring failures
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();
  });

  it('disables Start Installation button when allowIgnoreHostPreflights is false and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreHostPreflights: false
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: false
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be disabled when CLI flag doesn't allow ignoring failures
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).toBeDisabled();

    // Try to click disabled button - nothing should happen
    fireEvent.click(nextButton);

    // No modal should appear and onNext should not be called
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('shows modal when Start Installation clicked and allowIgnoreHostPreflights is true and preflights fail', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreHostPreflights: true
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [
              { title: 'Disk Space', message: 'Not enough disk space available' }
            ],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Button should be enabled
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();

    // Click Next button - should show modal with warning
    fireEvent.click(nextButton);

    // Modal SHOULD appear when allowIgnoreHostPreflights is true and preflights fail
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

  it('proceeds automatically when allowIgnoreHostPreflights is true and preflights pass', async () => {
    // Mock preflight status endpoint - returns success with allowIgnoreHostPreflights: true
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [
              { title: 'Disk Space', message: 'Sufficient disk space available' }
            ]
          },
          allowIgnoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Button should be enabled
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();

    // Click Next button - should proceed directly without modal (normal success case)
    fireEvent.click(nextButton);

    // No modal should appear when preflights pass
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('proceeds normally when allowIgnoreHostPreflights is false and preflights pass', async () => {
    // Mock preflight status endpoint - returns success with allowIgnoreHostPreflights: false
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [
              { title: 'Disk Space', message: 'Sufficient disk space available' }
            ]
          },
          allowIgnoreHostPreflights: false
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          authenticated: true
        }
      }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Button should be enabled
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();

    // Click Next button - should proceed directly without modal (normal success case)
    fireEvent.click(nextButton);

    // No modal should appear when preflights pass
    expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  // Phase 4.1 RED Tests: Verify ignorePreflightFailures parameter is sent
  it('sends ignorePreflightFailures parameter when starting installation with failed preflights', async () => {
    // Mock preflight status endpoint - returns failures with allowIgnoreHostPreflights: true
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [{ title: 'Disk Space', message: 'Not enough disk space available' }],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: true
        });
      }),
      // Mock infra setup endpoint to capture request body
      http.post('*/api/install/infra/setup', async ({ request }) => {
        const body = await request.json();
        
        // Verify the request includes ignorePreflightFailures parameter
        expect(body).toHaveProperty('ignorePreflightFailures', true);
        
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for preflights to complete and show failures
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Click Start Installation button
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

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

  it('sends ignorePreflightFailures false when starting installation with passed preflights', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: {
            fail: [],
            warn: [],
            pass: [{ title: 'Disk Space', message: 'Sufficient disk space available' }]
          },
          allowIgnoreHostPreflights: false
        });
      }),
      // Mock infra setup endpoint to capture request body
      http.post('*/api/install/infra/setup', async ({ request }) => {
        const body = await request.json();
        
        // Verify the request includes ignorePreflightFailures parameter as false
        expect(body).toHaveProperty('ignorePreflightFailures', false);
        
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for preflights to complete and show success
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Click Start Installation button
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });
});

// Phase 5: Additional robust frontend tests for error handling and edge cases
describe('ValidationStep - Error Handling & Edge Cases', () => {
  beforeAll(() => server.listen());
  afterEach(() => server.resetHandlers());
  afterAll(() => server.close());

  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('handles API error responses gracefully when starting installation', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreHostPreflights: false
        });
      }),
      // Mock infra setup endpoint to return API error
      http.post('*/api/install/infra/setup', () => {
        return HttpResponse.json(
          { 
            statusCode: 400,
            message: 'Preflight checks failed. Cannot proceed with installation.' 
          },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Click Start Installation
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Should show error message instead of proceeding
    await waitFor(() => {
      expect(screen.getByText(/Preflight checks failed/)).toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles network failure during installation start', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'], 
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreHostPreflights: false
        });
      }),
      // Mock infra setup endpoint to return network error
      http.post('*/api/install/infra/setup', () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Click Start Installation
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Should show network error message (matches actual fetch error)
    await waitFor(() => {
      expect(screen.getByText(/Failed to fetch/)).toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles button states during API interactions', async () => {
    // Mock preflight status endpoint - returns success
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreHostPreflights: false
        });
      }),
      // Mock successful infra setup
      http.post('*/api/install/infra/setup', () => {
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // Button should be enabled initially
    const nextButton = screen.getByText('Next: Start Installation');
    expect(nextButton).not.toBeDisabled();

    // Click Start Installation - should succeed
    fireEvent.click(nextButton);

    // Should proceed to next step
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('handles error when ignoring preflights without CLI flag', async () => {
    // Mock preflight status endpoint - returns failures
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [{ title: 'Disk Space', message: 'Not enough disk space' }],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: true
        });
      }),
      // Mock infra setup endpoint to return CLI flag error
      http.post('*/api/install/infra/setup', () => {
        return HttpResponse.json(
          { 
            statusCode: 400,
            message: 'Cannot ignore preflight failures without --ignore-host-preflights flag' 
          },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for failed state
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Click Start Installation button (should show modal)
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Confirm in modal
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    const continueButton = screen.getByText('Continue Anyway');
    fireEvent.click(continueButton);

    // Should show error about CLI flag instead of proceeding
    await waitFor(() => {
      expect(screen.getByText(/Cannot ignore preflight failures without --ignore-host-preflights flag/)).toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('clears previous errors when new installation attempt succeeds', async () => {
    let shouldFail = true;

    // Mock preflight status endpoint - returns success
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Succeeded' },
          output: { fail: [], warn: [], pass: [{ title: 'Test', message: 'Pass' }] },
          allowIgnoreHostPreflights: false
        });
      }),
      // Mock infra setup endpoint that fails first, succeeds second
      http.post('*/api/install/infra/setup', () => {
        if (shouldFail) {
          shouldFail = false; // Succeed on next attempt
          return HttpResponse.json(
            { statusCode: 500, message: 'Internal server error' },
            { status: 500 }
          );
        }
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for success state
    await waitFor(() => {
      expect(screen.getByText('Host validation successful!')).toBeInTheDocument();
    });

    // First attempt - should fail
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    await waitFor(() => {
      expect(screen.getByText(/Internal server error/)).toBeInTheDocument();
    });

    // Second attempt - should succeed and clear error
    fireEvent.click(nextButton);

    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });

    // Error message should be gone
    expect(screen.queryByText(/Internal server error/)).not.toBeInTheDocument();
  });

  it('properly handles modal cancellation flow', async () => {
    // Mock preflight status endpoint - returns failures
    server.use(
      http.get('*/api/install/host-preflights/status', () => {
        return HttpResponse.json({
          titles: ['Host Check'],
          status: { state: 'Failed' },
          output: {
            fail: [{ title: 'Disk Space', message: 'Not enough disk space' }],
            warn: [],
            pass: []
          },
          allowIgnoreHostPreflights: true
        });
      })
    );

    renderWithProviders(
      <ValidationStep onNext={mockOnNext} onBack={mockOnBack} />,
      { wrapperProps: { authenticated: true } }
    );

    // Wait for failed state
    await waitFor(() => {
      expect(screen.getByText('Host Requirements Not Met')).toBeInTheDocument();
    });

    // Click Start Installation button (should show modal)
    const nextButton = screen.getByText('Next: Start Installation');
    fireEvent.click(nextButton);

    // Modal should appear
    await waitFor(() => {
      expect(screen.getByText('Proceed with Failed Checks?')).toBeInTheDocument();
    });

    // Cancel the modal
    const cancelButton = screen.getByText('Cancel');
    fireEvent.click(cancelButton);

    // Modal should disappear
    await waitFor(() => {
      expect(screen.queryByText('Proceed with Failed Checks?')).not.toBeInTheDocument();
    });

    // Should NOT proceed to next step
    expect(mockOnNext).not.toHaveBeenCalled();

    // Button should still be available for another attempt
    expect(screen.getByText('Next: Start Installation')).toBeInTheDocument();
  });
}); 