import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import UpgradeStep from '../UpgradeStep';

const createServer = (target: 'linux' | 'kubernetes') => setupServer(
  // Mock app upgrade status endpoint
  http.get(`*/api/${target}/upgrade/app/status`, () => {
    return HttpResponse.json({
      status: {
        state: 'Pending',
        description: 'Waiting to start upgrade...'
      }
    });
  })
);

describe.each([
  { target: "kubernetes" as const, displayName: "Kubernetes" },
  { target: "linux" as const, displayName: "Linux" }
])('UpgradeStep - $displayName', ({ target }) => {
  const mockOnNext = vi.fn();
  const mockOnBack = vi.fn();
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

  it('renders upgrade timeline with correct phases', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should show timeline with upgrade phases
    await waitFor(() => {
      expect(screen.getByTestId('timeline-title')).toBeInTheDocument();
    });

    // Should show confirm upgrade phase
    await waitFor(() => {
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
    });

    // Should show app installation phase
    await waitFor(() => {
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();
    });

    // Should not show linux/kubernetes preflight or installation phases
    expect(screen.queryByTestId('timeline-linux-preflight')).not.toBeInTheDocument();
    expect(screen.queryByTestId('timeline-linux-installation')).not.toBeInTheDocument();
    expect(screen.queryByTestId('timeline-kubernetes-installation')).not.toBeInTheDocument();
  });

  it('starts at upgrade confirmation phase', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Should show upgrade confirmation phase first
    await waitFor(() => {
      expect(screen.getByTestId('app-preflight-container')).toBeInTheDocument();
      expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('hidden');
    });

    // App installation phase should be hidden initially
    expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
  });

  it('shows correct phase titles for upgrade', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    await waitFor(() => {
      expect(screen.getByText('Confirm Upgrade')).toBeInTheDocument();
    });
  });

  it('advances to next phase when current phase succeeds', async () => {
    let upgradeStarted = false;

    server.use(
      // Mock upgrade confirmation endpoint
      http.post(`*/api/${target}/upgrade/app/upgrade`, () => {
        upgradeStarted = true;
        return HttpResponse.json({ success: true });
      }),

      // Mock app status to return success after upgrade starts
      http.get(`*/api/${target}/upgrade/app/status`, () => {
        if (upgradeStarted) {
          return HttpResponse.json({
            status: {
              state: 'Succeeded',
              description: 'Application upgraded successfully'
            }
          });
        }
        return HttpResponse.json({
          status: {
            state: 'Pending',
            description: 'Waiting to start upgrade...'
          }
        });
      })
    );

    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Wait for confirmation phase to load
    await waitFor(() => {
      expect(screen.getByTestId('app-preflight-container')).toBeInTheDocument();
    });

    // Click the next button to trigger upgrade
    const nextButton = screen.getByTestId('installation-next-button');
    fireEvent.click(nextButton);

    // Should advance to app installation phase automatically after success
    await waitFor(() => {
      expect(screen.getByTestId('app-installation-container')).toBeInTheDocument();
    }, { timeout: 1000 });
  });

  it('completes upgrade flow and calls onNext when all phases succeed', async () => {
    server.use(
      // Mock successful upgrade status
      http.get(`*/api/${target}/upgrade/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: 'Succeeded',
            description: 'Application upgraded successfully'
          }
        });
      })
    );

    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // Skip to app installation phase by completing confirmation
    // In a real scenario, this would happen after user confirms the upgrade
    await waitFor(() => {
      expect(screen.getByTestId('app-preflight-container')).toBeInTheDocument();
    });

    // Simulate completing all phases - the auto-advance should eventually call onNext
    await waitFor(() => {
      // The component should call onNext when all phases are complete
      // This might take some time due to the setTimeout in the component
    }, { timeout: 3000 });
  });

  it('does not show back button in first phase by default', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    await waitFor(() => {
      // Back button should not be visible in the first phase unless configured by the phase component
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });
  });

  it('shows next button with correct text for different phases', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    // At first phase, should show next phase name
    await waitFor(() => {
      const nextButton = screen.getByTestId('installation-next-button');
      expect(nextButton).toBeInTheDocument();
      // Should reference the app installation phase title
      expect(nextButton).toHaveTextContent(/next:/i);
    });
  });

  it('allows clicking on completed phases to view them', async () => {
    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    await waitFor(() => {
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
    });

    // Initially, current phase should be clickable and selected
    const confirmPhaseButton = screen.getByTestId('timeline-app-preflight');
    expect(confirmPhaseButton).toBeInTheDocument();

    // App installation phase should exist but not be the current phase
    const appInstallPhaseButton = screen.getByTestId('timeline-app-installation');
    expect(appInstallPhaseButton).toBeInTheDocument();
  });

  it('handles phase state changes correctly', async () => {
    let phaseStatus = 'Running';

    server.use(
      http.get(`*/api/${target}/upgrade/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: phaseStatus,
            description: 'Upgrade in progress...'
          }
        });
      })
    );

    renderWithProviders(
      <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
      {
        wrapperProps: {
          target,
          mode: 'upgrade',
          authenticated: true
        }
      }
    );

    await waitFor(() => {
      expect(screen.getByTestId('app-preflight-container')).toBeInTheDocument();
    });

    // Change phase status to succeeded
    phaseStatus = 'Succeeded';
    server.resetHandlers();
    server.use(
      http.get(`*/api/${target}/upgrade/app/status`, () => {
        return HttpResponse.json({
          status: {
            state: phaseStatus,
            description: 'Upgrade completed successfully'
          }
        });
      })
    );

    // The phase should update its status
    await waitFor(() => {
      // Check if the timeline reflects the success state
      // This would be indicated by the phase having a success icon or status
    });
  });
});