import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import React from 'react';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import InstallationStep from '../InstallationStep.tsx';

// Type for phase component props
type PhaseProps = {
  onNext: () => void;
  setNextButtonConfig: (config: { disabled: boolean; onClick: () => void }) => void;
  onStateChange: (state: string) => void;
};

type PhaseOutcome = 'success' | 'failure';

// Mock configuration for each phase
const phaseMockConfig = {
  linuxPreflight: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false 
  },
  linuxInstallation: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false 
  },
  kubernetesInstallation: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false 
  },
  appPreflight: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false 
  },
  appInstallation: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false 
  }
};

const createPhaseMock = (phaseName: string, phaseKey: keyof typeof phaseMockConfig) => ({ onNext, setNextButtonConfig, onStateChange }: PhaseProps) => {
  React.useEffect(() => {
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
  }, []);
  
  const config = phaseMockConfig[phaseKey];
  const suffix = config.outcome === 'failure' ? ' Failed' : '';
  const testId = phaseName.toLowerCase().replace(/\s+/g, '-');
  return <div data-testid={testId}>{phaseName}{suffix}</div>;
};

// Mock all the phase components
vi.mock('../phases/LinuxPreflightPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Linux Preflight Phase', 'linuxPreflight')(props)
}));

vi.mock('../phases/LinuxInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Linux Installation Phase', 'linuxInstallation')(props)
}));

vi.mock('../phases/KubernetesInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Kubernetes Installation Phase', 'kubernetesInstallation')(props)
}));

vi.mock('../phases/AppPreflightPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Preflight Phase', 'appPreflight')(props)
}));

vi.mock('../phases/AppInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Installation Phase', 'appInstallation')(props)
}));

const server = setupServer();

describe('InstallationStep', () => {
  const mockOnNext = vi.fn();

  beforeAll(() => {
    server.listen();
  });

  afterEach(() => {
    server.resetHandlers();
    vi.clearAllMocks();
    // Reset all phases to success with enabled buttons
    phaseMockConfig.linuxPreflight = { outcome: 'success', buttonDisabled: false };
    phaseMockConfig.linuxInstallation = { outcome: 'success', buttonDisabled: false };
    phaseMockConfig.kubernetesInstallation = { outcome: 'success', buttonDisabled: false };
    phaseMockConfig.appPreflight = { outcome: 'success', buttonDisabled: false };
    phaseMockConfig.appInstallation = { outcome: 'success', buttonDisabled: false };
  });

  afterAll(() => {
    server.close();
  });

  const renderInstallationStep = (target: 'linux' | 'kubernetes' = 'linux') => {
    return renderWithProviders(
      <InstallationStep onNext={mockOnNext} />,
      {
        wrapperProps: {
          target,
          authenticated: true
        }
      }
    );
  };

  describe('Linux Target', () => {
    it('renders correct phase order for Linux target', () => {
      renderInstallationStep('linux');

      // Should show timeline with correct phases
      expect(screen.getByTestId('timeline-title')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-linux-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-linux-installation')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start with Linux preflight phase
      expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();
    });

    it('progresses through all Linux phases in correct order', async () => {
      renderInstallationStep('linux');

      // Start with Linux preflight - button should be enabled by default
      expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to Linux installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('Linux Installation Phase')).toBeInTheDocument();
        // Button should still be enabled for this phase
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('App Preflight Phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to App installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('App Installation Phase')).toBeInTheDocument();
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
    it('renders correct phase order for Kubernetes target', () => {
      renderInstallationStep('kubernetes');

      // Should show timeline with correct phases (no Linux phases)
      expect(screen.getByTestId('timeline-title')).toBeInTheDocument();
      expect(screen.queryByTestId('timeline-linux-preflight')).not.toBeInTheDocument();
      expect(screen.queryByTestId('timeline-linux-installation')).not.toBeInTheDocument();
      expect(screen.getByTestId('timeline-kubernetes-installation')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-preflight')).toBeInTheDocument();
      expect(screen.getByTestId('timeline-app-installation')).toBeInTheDocument();

      // Should start with Kubernetes installation phase
      expect(screen.getByText('Kubernetes Installation Phase')).toBeInTheDocument();
    });

    it('progresses through all Kubernetes phases in correct order', async () => {
      renderInstallationStep('kubernetes');

      // Start with Kubernetes installation
      expect(screen.getByText('Kubernetes Installation Phase')).toBeInTheDocument();

      // Click next to go to App preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('App Preflight Phase')).toBeInTheDocument();
      });

      // Click next to go to App installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('App Installation Phase')).toBeInTheDocument();
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
      renderInstallationStep('linux');

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

    it('allows clicking on completed phases to view them', async () => {
      renderInstallationStep('linux');

      // Start with Linux preflight
      expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();

      // Complete first phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByText('Linux Installation Phase')).toBeInTheDocument();
      });

      // Now click back on the completed Linux preflight phase in timeline
      fireEvent.click(screen.getByTestId('timeline-linux-preflight'));
      
      await waitFor(() => {
        // Should show the previous phase content 
        expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();
      });
    });

    it('prevents clicking on pending phases', () => {
      renderInstallationStep('linux');

      // Pending phases should be disabled
      const appInstallButton = screen.getByTestId('timeline-app-installation');
      expect(appInstallButton).toBeDisabled();
    });
  });

  describe('Failure State Handling', () => {

    it('displays failure state in timeline when a phase fails', async () => {
      // Configure Linux preflight to fail
      phaseMockConfig.linuxPreflight.outcome = 'failure';
      
      renderInstallationStep('linux');

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
      expect(screen.getByText('Linux Preflight Phase Failed')).toBeInTheDocument();
      expect(screen.queryByText('Linux Installation Phase')).not.toBeInTheDocument();
    });

    it('allows clicking on failed phases to view them', async () => {
      // Configure second phase to fail
      phaseMockConfig.linuxInstallation.outcome = 'failure';
      
      renderInstallationStep('linux');

      // Complete first phase (preflight) - should succeed and move to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should now be on the second phase (which is configured to fail)
        expect(screen.getByText('Linux Installation Phase Failed')).toBeInTheDocument();
      });

      // Try to complete the second phase - should fail and stay on same phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should show failure icon in timeline
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
        // Should still be showing the failed phase
        expect(screen.getByText('Linux Installation Phase Failed')).toBeInTheDocument();
      });

      // Should be able to click on the failed phase button in timeline
      const failedPhaseButton = screen.getByTestId('timeline-linux-installation');
      expect(failedPhaseButton).not.toBeDisabled();
      
      // Click on completed phase first
      fireEvent.click(screen.getByTestId('timeline-linux-preflight'));
      await waitFor(() => {
        expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();
      });
      
      // Then click back to failed phase
      fireEvent.click(failedPhaseButton);
      await waitFor(() => {
        expect(screen.getByText('Linux Installation Phase Failed')).toBeInTheDocument();
      });
    });

    it('shows mixed success and failure states in timeline', async () => {
      // Configure: first succeeds, second fails, rest pending
      phaseMockConfig.linuxInstallation.outcome = 'failure';
      
      renderInstallationStep('linux');

      // Complete first phase successfully
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Use a more flexible text matcher since the component may render "Linux Installation Phase Failed"
        expect(screen.getByText(/Linux Installation Phase/)).toBeInTheDocument();
      });

      // Fail second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should have both success and failure icons
        expect(screen.getByTestId('icon-succeeded')).toBeInTheDocument();
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
        // Should still have pending icons for remaining phases
        expect(screen.getAllByTestId('icon-pending').length).toBeGreaterThan(0);
      });
    });

    it('shows correct phase status icons for different states', async () => {
      renderInstallationStep('linux');

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
      phaseMockConfig.linuxPreflight.buttonDisabled = true;
      
      renderInstallationStep('linux');

      // Button should be disabled
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).toBeDisabled();
      });

      // Clicking disabled button should not progress (button won't respond)
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Should still be on first phase
      expect(screen.getByText('Linux Preflight Phase')).toBeInTheDocument();
      expect(screen.queryByText('Linux Installation Phase')).not.toBeInTheDocument();
    });

    it('enables button when phase configuration changes', async () => {
      // Test that different phases can have different button states
      phaseMockConfig.linuxPreflight.buttonDisabled = false;  // enabled
      phaseMockConfig.linuxInstallation.buttonDisabled = true; // disabled
      
      renderInstallationStep('linux');

      // First phase button should be enabled
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Progress to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Second phase button should be disabled
      await waitFor(() => {
        expect(screen.getByText('Linux Installation Phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).toBeDisabled();
      });
    });
  });

});
