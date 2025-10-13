import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { useEffect } from 'react';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import UpgradeStep from '../UpgradeStep.tsx';
import { State } from '../../../../types/index.ts';

// Type for phase component props
type PhaseProps = {
  onNext: () => void;
  onBack: () => void;
  setNextButtonConfig: (config: { disabled: boolean; onClick: () => void }) => void;
  setBackButtonConfig: (config: { hidden: boolean; disabled?: boolean; onClick: () => void }) => void;
  onStateChange: (state: string) => void;
};

type PhaseOutcome = 'success' | 'failure';

// Mock configuration for each phase
const phaseMockConfig = {
  upgradeInstallation: {
    outcome: 'success' as PhaseOutcome,
    buttonDisabled: false,
    autoStateChange: null as { delay: number, state: State } | null
  },
  appPreflight: {
    outcome: 'success' as PhaseOutcome,
    buttonDisabled: false,
    autoStateChange: null as { delay: number, state: State } | null
  },
  appInstallation: {
    outcome: 'success' as PhaseOutcome,
    buttonDisabled: false,
    autoStateChange: null as { delay: number, state: State } | null
  }
};

const createPhaseMock = (phaseName: string, phaseKey: keyof typeof phaseMockConfig, phaseTestId: string) => ({ onNext, onBack, setNextButtonConfig, setBackButtonConfig, onStateChange }: PhaseProps) => {
  // Set up initial back button config
  useEffect(() => {
    // All upgrade phases have back button hidden
    setBackButtonConfig({
      hidden: true,
      onClick: onBack,
    });
  }, []);

  useEffect(() => {
    onStateChange('Running');
    const config = phaseMockConfig[phaseKey];
    setNextButtonConfig({
      disabled: config.buttonDisabled,
      onClick: () => {
        if (config.outcome === 'failure') {
          onStateChange('Failed');
          // Don't call onNext() when failed - stay on same phase
        } else {
          onStateChange('Succeeded');
          onNext();
        }
      }
    });

    // Set up auto state change if configured
    if (config.autoStateChange) {
      const timeout = setTimeout(() => {
        onStateChange(config.autoStateChange!.state);
      }, config.autoStateChange.delay);

      return () => {
        clearTimeout(timeout);
      };
    }
  }, []);

  const config = phaseMockConfig[phaseKey];
  const suffix = config.outcome === 'failure' ? ' Failed' : '';
  return <div data-testid={phaseTestId}>{phaseName}{suffix}</div>;
};

// Mock all the phase components
vi.mock('../phases/InfraUpgradePhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Infra Upgrade Phase', 'upgradeInstallation', 'upgrade-installation-phase')(props)
}));

vi.mock('../phases/AppPreflightPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Preflight Phase', 'appPreflight', 'app-preflight-phase')(props)
}));

vi.mock('../phases/AppInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Installation Phase', 'appInstallation', 'app-installation-phase')(props)
}));

const server = setupServer(
  // Default: infrastructure upgrade is required
  http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
    return HttpResponse.json({ requiresUpgrade: true });
  })
);

describe('UpgradeStep', () => {
  const mockOnNext = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();

    // Reset all phases
    phaseMockConfig.upgradeInstallation = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.appPreflight = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.appInstallation = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
  });

  afterAll(() => {
    server.close();
  });

  const renderUpgradeStep = (target: 'linux' | 'kubernetes' = 'linux') => {
    const mockOnBack = vi.fn();
    return {
      ...renderWithProviders(
        <UpgradeStep onNext={mockOnNext} onBack={mockOnBack} />,
        {
          wrapperProps: {
            mode: 'upgrade',
            target,
            authenticated: true
          }
        }
      ),
      mockOnBack
    };
  };

  describe('Linux Target', () => {
    it('renders correct phase order for Linux upgrade', async () => {
      renderUpgradeStep('linux');

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should show timeline with correct phases
      expect(screen.getByTestId('timeline-title')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-linux-installation')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start with Upgrade Installation phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
    });

    it('progresses through all Linux phases in correct order', async () => {
      renderUpgradeStep('linux');

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Start with Upgrade Installation phase - button should be enabled by default
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
        // Button should still be enabled for this phase
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click finish
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Kubernetes Target', () => {
    it('renders correct phase order for Kubernetes upgrade', async () => {
      renderUpgradeStep('kubernetes');

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should show timeline with correct phases
      expect(screen.getByTestId('timeline-title')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-kubernetes-installation')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start with Upgrade Installation phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
    });

    it('progresses through all Kubernetes phases in correct order', async () => {
      renderUpgradeStep('kubernetes');

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Start with Upgrade Installation phase - button should be enabled by default
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
        // Button should still be enabled for this phase
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click finish
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalledTimes(1);
      });
    });
  });

  describe('Timeline Integration', () => {
    it('updates timeline status when phases change state', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should show running icon initially
      await waitFor(() => {
        expect(screen.getByTestId('icon-running')).toBeInTheDocument();
      });

      // Click next to complete the phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Timeline should show success icon for completed phase
      await waitFor(() => {
        expect(screen.getByTestId('icon-succeeded')).toBeInTheDocument();
      });
    });

    it('keeps completed and current phases mounted when switching between phases', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Start with Upgrade Installation phase - should be visible
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

      // Move to second phase (App preflight)
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Second phase should now be visible
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('block');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('hidden');

        // First phase should now be hidden but still mounted
        expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('block');
      });

      // Click back on first completed phase in timeline
      fireEvent.click(screen.getByTestId('timeline-linux-installation'));

      await waitFor(() => {
        // First phase container should be visible
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

        // Second phase should still exist in DOM but hidden
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('block');
      });
    });

    it('allows clicking on completed phases to view them', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Start with Upgrade Installation phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();

      // Complete first phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Now click back on the completed Upgrade Installation phase in timeline
      fireEvent.click(screen.getByTestId('timeline-linux-installation'));

      await waitFor(() => {
        // Should show the previous phase content
        expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      });
    });

    it('prevents clicking on pending phases', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Pending phases should be disabled
      const appInstallButton = screen.getByTestId('timeline-app-installation');
      expect(appInstallButton).toBeDisabled();
    });
  });

  describe('Failure State Handling', () => {
    it('displays failure state in timeline when a phase fails', async () => {
      // Configure Upgrade Installation phase to fail
      phaseMockConfig.upgradeInstallation.outcome = 'failure';

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should show running state initially and button should be enabled
      await waitFor(() => {
        expect(screen.getByTestId('icon-running')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next button - this will trigger failure
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Should show failure icon in timeline
      await waitFor(() => {
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });

      // Should not progress to next phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
    });

    it('allows clicking on failed phases to view them', async () => {
      // Configure second phase (app-preflight) to fail
      phaseMockConfig.appPreflight.outcome = 'failure';

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Complete first phase (upgrade installation) - should succeed and move to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should now be on the second phase (which is configured to fail)
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Try to complete the second phase - should fail and stay on same phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should show failure icon in timeline
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
        // Should still be showing the failed phase
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Should be able to click on the failed phase button in timeline
      const failedPhaseButton = screen.getByTestId('timeline-app-preflight');
      expect(failedPhaseButton).not.toBeDisabled();

      // Click on completed phase first
      fireEvent.click(screen.getByTestId('timeline-linux-installation'));
      await waitFor(() => {
        expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      });

      // Then click back to failed phase
      fireEvent.click(failedPhaseButton);
      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });
    });

    it('shows mixed success and failure states in timeline', async () => {
      // Configure: first two phases succeed, third fails
      phaseMockConfig.appInstallation.outcome = 'failure';

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Complete first phase successfully (upgrade installation)
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should be on the second phase (app preflight)
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Complete second phase successfully
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should be on the third phase (app installation)
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
      });

      // Fail third phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should have multiple success icons (for completed phases) and one failure icon
        expect(screen.getAllByTestId('icon-succeeded').length).toBeGreaterThan(0);
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });
    });

    it('shows correct phase status icons for different states', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Initially should show running (current) and pending states
      await waitFor(() => {
        // Running phase
        expect(screen.getByTestId('icon-running')).toBeInTheDocument();
        // Pending phases
        expect(screen.getAllByTestId('icon-pending').length).toBeGreaterThan(0);
      });

      // Complete first phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should now have success icon for completed phase
        expect(screen.getByTestId('icon-succeeded')).toBeInTheDocument();
        // Still have running icon for current phase
        expect(screen.getByTestId('icon-running')).toBeInTheDocument();
      });
    });

    it('handles disabled button states from phases', async () => {
      // Configure first phase to disable the button
      phaseMockConfig.upgradeInstallation.buttonDisabled = true;

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Button should be disabled
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).toBeDisabled();
      });

      // Clicking disabled button should not progress (button won't respond)
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Should still be on first phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
    });

    it('enables button when phase configuration changes', async () => {
      // Test that different phases can have different button states
      phaseMockConfig.upgradeInstallation.buttonDisabled = false;  // enabled
      phaseMockConfig.appPreflight.buttonDisabled = true; // disabled

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // First phase button should be enabled
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Progress to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Second phase button should be disabled
      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).toBeDisabled();
      });
    });
  });

  describe('Auto-advance functionality', () => {
    it('automatically advances when phase succeeds via auto state change', async () => {
      // Configure first phase to automatically succeed after 100ms
      phaseMockConfig.upgradeInstallation.autoStateChange = { delay: 100, state: 'Succeeded' };

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should start with Upgrade Installation phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

      // Wait for the phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Should automatically advance to App Preflight Phase
      await waitFor(() => {
        // Second phase should now be visible
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('block');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('hidden');

        // First phase should now be hidden but still mounted
        expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('block');
      });
    });

    it('does not auto-advance when phase fails', async () => {
      // Configure first phase to automatically fail after 100ms
      phaseMockConfig.upgradeInstallation.autoStateChange = { delay: 100, state: 'Failed' };

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should start with Upgrade Installation phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

      // Wait for the phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Wait for the phase to auto-fail (this should happen)
      await waitFor(() => {
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });

      // Should still be on the same phase, not advance
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

      // Second phase should not be mounted at all
      expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
      expect(screen.queryByTestId('app-preflight-container')).not.toBeInTheDocument();
    });
  });

  describe('Back Button Behavior', () => {
    it('back button is not present for upgrade flow', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Wait for phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Back button should not be present for upgrade flow (no host preflights)
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });

    it('back button remains hidden during all phases', async () => {
      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // First phase
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();

      // Progress to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));
      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
      });

      // Back button should still not be present
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });
  });

  describe('Infrastructure Upgrade Check', () => {
    it('shows loading state while checking if infrastructure upgrade is required', async () => {
      // Delay the response to see loading state
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', async () => {
          await new Promise(resolve => setTimeout(resolve, 100));
          return HttpResponse.json({ requiresUpgrade: true });
        })
      );

      renderUpgradeStep();

      // Should show checking requirements loading state
      expect(screen.getByTestId('upgrade-step-checking-requirements')).toBeInTheDocument();
      expect(screen.getByTestId('checking-requirements-spinner')).toBeInTheDocument();
      expect(screen.getByTestId('checking-requirements-message')).toHaveTextContent('Checking upgrade requirements...');
    });

    it('includes infrastructure phase when upgrade is required', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.json({ requiresUpgrade: true });
        })
      );

      renderUpgradeStep();

      // Wait for check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should show infrastructure upgrade phase in timeline
      expect(screen.getByTestId('timeline-linux-installation')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start with infrastructure upgrade phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();
    });

    it('skips infrastructure phase when upgrade is not required', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.json({ requiresUpgrade: false });
        })
      );

      renderUpgradeStep();

      // Wait for check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should NOT show infrastructure upgrade phase in timeline
      expect(screen.queryByTestId('timeline-linux-installation')).not.toBeInTheDocument();
      expect(screen.queryByTestId('timeline-kubernetes-installation')).not.toBeInTheDocument();

      // Should show only app phases
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start directly with app preflight phase
      expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
    });

    it('shows error state when infrastructure check fails', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.json(
            {
              statusCode: 500,
              message: 'Failed to determine upgrade requirements'
            },
            { status: 500 }
          );
        })
      );

      renderUpgradeStep();

      // Should show error state
      await waitFor(() => {
        expect(screen.getByTestId('upgrade-step-check-error')).toBeInTheDocument();
      });

      expect(screen.getByTestId('check-error-icon')).toBeInTheDocument();
      expect(screen.getByTestId('check-error-title')).toHaveTextContent('Failed to Check Upgrade Requirements');
      expect(screen.getByTestId('check-error-message')).toBeInTheDocument();
    });

    it('handles network error during infrastructure check', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.error();
        })
      );

      renderUpgradeStep();

      // Should show error state
      await waitFor(() => {
        expect(screen.getByTestId('upgrade-step-check-error')).toBeInTheDocument();
      });

      expect(screen.getByTestId('check-error-title')).toHaveTextContent('Failed to Check Upgrade Requirements');
    });

    it('progresses through all phases when infrastructure upgrade is required', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.json({ requiresUpgrade: true });
        })
      );

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should start with infrastructure upgrade phase
      expect(screen.getByTestId('upgrade-installation-phase')).toBeInTheDocument();

      // Complete infrastructure phase
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Move to app preflight
      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Complete app preflight phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Move to app installation
      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
      });

      // Complete app installation phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Should call onNext to go to completion step
      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalledTimes(1);
      });
    });

    it('progresses through only app phases when infrastructure upgrade is not required', async () => {
      server.use(
        http.get('*/api/*/upgrade/infra/requires-upgrade', () => {
          return HttpResponse.json({ requiresUpgrade: false });
        })
      );

      renderUpgradeStep();

      // Wait for infrastructure check to complete
      await waitFor(() => {
        expect(screen.queryByTestId('upgrade-step-checking-requirements')).not.toBeInTheDocument();
      });

      // Should start directly with app preflight phase (skipping infrastructure)
      expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      expect(screen.queryByTestId('upgrade-installation-phase')).not.toBeInTheDocument();

      // Complete app preflight phase
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Move to app installation
      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
      });

      // Complete app installation phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Should call onNext to go to completion step
      await waitFor(() => {
        expect(mockOnNext).toHaveBeenCalledTimes(1);
      });
    });
  });
});
