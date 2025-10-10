import { describe, it, expect, vi, beforeAll, afterEach, afterAll, beforeEach } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import UpgradeInstallationPhase from '../phases/UpgradeInstallationPhase.tsx';
import { withTestButton } from './TestWrapper.tsx';

const TestUpgradeInstallationPhase = withTestButton(UpgradeInstallationPhase);

const createServer = () => setupServer(
  // Mock app preflight run endpoint - matches both install and upgrade modes for both linux and kubernetes targets
  http.post('*/api/*/install/app-preflights/run', () => {
    return HttpResponse.json({ success: true });
  }),
  http.post('*/api/*/upgrade/app-preflights/run', () => {
    return HttpResponse.json({ success: true });
  })
);

describe('UpgradeInstallationPhase', () => {
  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();
  let server: ReturnType<typeof createServer>;

  beforeAll(() => {
    server = createServer();
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    server.close();
  });

  // Basic Rendering Tests
  it('renders basic UI elements', async () => {
    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    expect(screen.getByText('Installation')).toBeInTheDocument();
    expect(screen.getByText('Start App Upgrade')).toBeInTheDocument();

    // Next button should be enabled
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
    });
  });

  // API Call Tests
  it('calls app-preflights/run endpoint when Next is clicked', async () => {
    let requestBody: { isUi: boolean } | undefined;
    server.use(
      http.post('*/api/*/upgrade/app-preflights/run', async ({ request }) => {
        expect(request.headers.get('Authorization')).toBe('Bearer test-token');
        expect(request.headers.get('Content-Type')).toBe('application/json');
        requestBody = await request.json() as { isUi: boolean };
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Verify request body contains isUi: true
    await waitFor(() => {
      expect(requestBody).toEqual({ isUi: true });
    });
  });

  // Success Flow Tests
  it('calls onNext when API call succeeds', async () => {
    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Should call onNext after successful API call
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });

    // No error message should be displayed
    expect(screen.queryByTestId('error-message')).not.toBeInTheDocument();
  });

  // Error Handling Tests
  it('displays error message when API call fails', async () => {
    server.use(
      http.post('*/api/*/upgrade/app-preflights/run', () => {
        return HttpResponse.json(
          {
            statusCode: 500,
            message: 'Internal server error'
          },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      expect(nextButton).not.toBeDisabled();
      fireEvent.click(nextButton);
    });

    // Should show error message
    await waitFor(() => {
      expect(screen.getByText(/Failed to start app preflight checks/)).toBeInTheDocument();
    });

    // Should NOT call onNext
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('does not call onNext when API call fails', async () => {
    server.use(
      http.post('*/api/*/upgrade/app-preflights/run', () => {
        return HttpResponse.json(
          { statusCode: 400, message: 'Bad request' },
          { status: 400 }
        );
      })
    );

    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      fireEvent.click(nextButton);
    });

    // Wait for error to appear
    await waitFor(() => {
      expect(screen.getByText(/Failed to start app preflight checks/)).toBeInTheDocument();
    });

    // onNext should not have been called
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles network failure gracefully', async () => {
    server.use(
      http.post('*/api/*/upgrade/app-preflights/run', () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      fireEvent.click(nextButton);
    });

    // Should show network error message
    await waitFor(() => {
      expect(screen.getByText(/Failed to fetch/)).toBeInTheDocument();
    });

    // Should NOT call onNext
    expect(mockOnNext).not.toHaveBeenCalled();
  });
});

// onStateChange Tests
describe('UpgradeInstallationPhase - onStateChange Tests', () => {
  let server: ReturnType<typeof createServer>;
  const mockOnNext = vi.fn();
  const mockOnStateChange = vi.fn();

  beforeAll(() => {
    server = createServer();
    server.listen();
  });

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    server.resetHandlers();
  });

  afterAll(() => {
    server.close();
  });

  it('calls onStateChange with "Running" immediately when component mounts', async () => {
    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');
    expect(mockOnStateChange).toHaveBeenCalledTimes(1);
  });

  it('calls onStateChange with "Succeeded" when API call succeeds', async () => {
    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      fireEvent.click(nextButton);
    });

    // Should call onStateChange with "Succeeded" when API call succeeds
    await waitFor(() => {
      expect(mockOnStateChange).toHaveBeenCalledWith('Succeeded');
    });

    // Expect sequence: Running (mount), Succeeded (success)
    const calls = mockOnStateChange.mock.calls.map(args => args[0]);
    expect(calls).toEqual(['Running', 'Succeeded']);
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });

  it('calls onStateChange with "Failed" when API call fails', async () => {
    server.use(
      http.post('*/api/*/upgrade/app-preflights/run', () => {
        return HttpResponse.json(
          { statusCode: 500, message: 'Internal server error' },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <TestUpgradeInstallationPhase
        onNext={mockOnNext}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should call onStateChange with "Running" immediately on mount
    expect(mockOnStateChange).toHaveBeenCalledWith('Running');

    // Click Next button
    await waitFor(() => {
      const nextButton = screen.getByTestId('next-button');
      fireEvent.click(nextButton);
    });

    // Should call onStateChange with "Failed" when API call fails
    await waitFor(() => {
      expect(mockOnStateChange).toHaveBeenCalledWith('Failed');
    });

    // Expect sequence: Running (mount), Failed (error)
    const calls = mockOnStateChange.mock.calls.map(args => args[0]);
    expect(calls).toEqual(['Running', 'Failed']);
    expect(mockOnStateChange).toHaveBeenCalledTimes(2);
  });
});
