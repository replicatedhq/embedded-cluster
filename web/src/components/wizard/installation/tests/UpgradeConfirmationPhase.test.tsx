import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { screen, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import UpgradeConfirmationPhase from '../phases/UpgradeConfirmationPhase';

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  // Mock app upgrade endpoint
  http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
    return HttpResponse.json({ success: true });
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('UpgradeConfirmationPhase - $displayName', ({ target }) => {
  const mockOnNext = vi.fn();
  const mockSetNextButtonConfig = vi.fn();
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

  it('renders upgrade confirmation content', async () => {
    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should show upgrade confirmation title and description
    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 2 })).toBeInTheDocument();
    });
  });

  it('configures next button on mount', async () => {
    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should configure the next button
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalledWith({
        disabled: false,
        onClick: expect.any(Function)
      });
    });
  });

  it('starts upgrade when next button is clicked', async () => {
    let upgradeCalled = false;

    server.use(
      http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
        upgradeCalled = true;
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for component to mount and configure next button
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalled();
    });

    // Get the onClick handler that was passed to setNextButtonConfig
    const nextButtonConfig = mockSetNextButtonConfig.mock.calls[0][0];
    const handleNextClick = nextButtonConfig.onClick;

    // Simulate clicking the next button
    handleNextClick();

    // Should call the upgrade API
    await waitFor(() => {
      expect(upgradeCalled).toBe(true);
    });

    // Should call onNext after successful upgrade start
    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
    });
  });

  it('shows error message when upgrade fails to start', async () => {
    const errorMessage = 'Failed to start upgrade due to insufficient resources';

    server.use(
      http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
        return HttpResponse.json(
          { message: errorMessage },
          { status: 500 }
        );
      })
    );

    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for component to mount
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalled();
    });

    // Get the onClick handler and simulate clicking
    const nextButtonConfig = mockSetNextButtonConfig.mock.calls[0][0];
    nextButtonConfig.onClick();

    // Should show error message
    await waitFor(() => {
      expect(screen.getByText(errorMessage)).toBeInTheDocument();
    });

    // Should not call onNext when there's an error
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('handles network error gracefully', async () => {
    server.use(
      http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
        return HttpResponse.error();
      })
    );

    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for component to mount
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalled();
    });

    // Get the onClick handler and simulate clicking
    const nextButtonConfig = mockSetNextButtonConfig.mock.calls[0][0];
    nextButtonConfig.onClick();

    // Should show error message for network error (actual message is "Failed to fetch" for network errors)
    await waitFor(() => {
      expect(screen.getByText(/Failed to fetch/)).toBeInTheDocument();
    });

    // Should not call onNext when there's an error
    expect(mockOnNext).not.toHaveBeenCalled();
  });

  it('clears previous error on successful upgrade start', async () => {
    let shouldFail = true;

    server.use(
      http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
        if (shouldFail) {
          return HttpResponse.json(
            { message: 'First attempt failed' },
            { status: 500 }
          );
        }
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for component to mount
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalled();
    });

    const nextButtonConfig = mockSetNextButtonConfig.mock.calls[0][0];

    // First attempt - should fail
    nextButtonConfig.onClick();

    await waitFor(() => {
      expect(screen.getByText('First attempt failed')).toBeInTheDocument();
    });

    // Second attempt - should succeed
    shouldFail = false;
    nextButtonConfig.onClick();

    await waitFor(() => {
      expect(mockOnNext).toHaveBeenCalledTimes(1);
      // Error message should be cleared
      expect(screen.queryByText('First attempt failed')).not.toBeInTheDocument();
    });
  });

  it('sends correct upgrade request payload', async () => {
    let receivedPayload: any = null;

    server.use(
      http.post(`*/api/${target}/upgrade/app/upgrade`, async ({ request }) => {
        receivedPayload = await request.json();
        return HttpResponse.json({ success: true });
      })
    );

    renderWithProviders(
      <UpgradeConfirmationPhase
        onNext={mockOnNext}
        setNextButtonConfig={mockSetNextButtonConfig}
        onStateChange={mockOnStateChange}
      />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for component to mount
    await waitFor(() => {
      expect(mockSetNextButtonConfig).toHaveBeenCalled();
    });

    // Trigger upgrade
    const nextButtonConfig = mockSetNextButtonConfig.mock.calls[0][0];
    nextButtonConfig.onClick();

    await waitFor(() => {
      expect(receivedPayload).toEqual({
        ignoreAppPreflights: false
      });
    });
  });
});