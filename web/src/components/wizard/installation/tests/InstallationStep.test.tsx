import { describe, it, expect, vi, beforeAll, afterEach, afterAll } from 'vitest';
import { useEffect } from 'react';
import { screen, fireEvent, waitFor } from '@testing-library/react';
import { setupServer } from 'msw/node';
import { renderWithProviders } from '../../../../test/setup.tsx';
import InstallationStep from '../InstallationStep.tsx';
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
  linuxPreflight: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false,
    autoStateChange: null as { delay: number, state: State } | null
  },
  linuxInstallation: { 
    outcome: 'success' as PhaseOutcome, 
    buttonDisabled: false,
    autoStateChange: null as { delay: number, state: State } | null
  },
  kubernetesInstallation: { 
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
    if (phaseKey === 'linuxPreflight') {
    // The back button is visible and disabled by default during the linux-preflight phase
    // It is enabled on preflight failure, and permanently hidden on preflight success
      setBackButtonConfig({
        hidden: false,
        disabled: true,
        onClick: onBack,
      });
    } else {
      // All other phases have back button hidden
      setBackButtonConfig({
        hidden: true,
        onClick: onBack,
      });
    }
  }, []);

  useEffect(() => {
    onStateChange('Running');
    const config = phaseMockConfig[phaseKey];
    setNextButtonConfig({
      disabled: config.buttonDisabled,
      onClick: () => {
        if (config.outcome === 'failure') {
          onStateChange('Failed');
          // Update back button config when linux-preflight fails
          if (phaseKey === 'linuxPreflight') {
            setBackButtonConfig({
              hidden: false, // Show back button when linux-preflight fails
              disabled: false, // Enable back button when linux-preflight fails
              onClick: onBack,
            });
          }
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
        // Update back button config when auto state change happens
        if (phaseKey === 'linuxPreflight' && config.autoStateChange!.state === 'Failed') {
          setBackButtonConfig({
            onClick: onBack,
            hidden: false, // Show back button when linux-preflight auto-fails
            disabled: false, // Enable back button when linux-preflight auto-fails
          });
        }
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
vi.mock('../phases/LinuxPreflightPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Linux Preflight Phase', 'linuxPreflight', 'linux-preflight-phase')(props)
}));

vi.mock('../phases/LinuxInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Linux Installation Phase', 'linuxInstallation', 'linux-installation-phase')(props)
}));

vi.mock('../phases/KubernetesInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('Kubernetes Installation Phase', 'kubernetesInstallation', 'kubernetes-installation-phase')(props)
}));

vi.mock('../phases/AppPreflightPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Preflight Phase', 'appPreflight', 'app-preflight-phase')(props)
}));

vi.mock('../phases/AppInstallationPhase', () => ({
  default: (props: PhaseProps) => createPhaseMock('App Installation Phase', 'appInstallation', 'app-installation-phase')(props)
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

    // Reset all phases
    phaseMockConfig.linuxPreflight = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.linuxInstallation = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.kubernetesInstallation = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.appPreflight = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
    phaseMockConfig.appInstallation = { outcome: 'success', buttonDisabled: false, autoStateChange: null };
  });

  afterAll(() => {
    server.close();
  });

  const renderInstallationStep = (target: 'linux' | 'kubernetes' = 'linux') => {
    const mockOnBack = vi.fn();
    return {
      ...renderWithProviders(
        <InstallationStep onNext={mockOnNext} onBack={mockOnBack} />,
        {
          wrapperProps: {
            target,
            authenticated: true
          }
        }
      ),
      mockOnBack
    };
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
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
    });

    it('progresses through all Linux phases in correct order', async () => {
      renderInstallationStep('linux');

      // Start with Linux preflight - button should be enabled by default
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Click next to go to Linux installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        // Button should still be enabled for this phase
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
      expect(screen.getByTestId('kubernetes-installation-phase')).toBeInTheDocument();
    });

    it('progresses through all Kubernetes phases in correct order', async () => {
      renderInstallationStep('kubernetes');

      // Start with Kubernetes installation
      expect(screen.getByTestId('kubernetes-installation-phase')).toBeInTheDocument();

      // Click next to go to App preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Click next to go to App installation
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
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

    it('keeps completed and current phases mounted when switching between phases', async () => {
      renderInstallationStep('linux');

      // Start with Linux preflight - should be visible
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-preflight-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('hidden');

      // Move to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Second phase should now be visible
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');
        
        // First phase should now be hidden but still mounted
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('block');

        // Third phase should not be mounted at all
        expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-preflight-container')).not.toBeInTheDocument();

        // Fourth phase should not be mounted at all
        expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
      });

      // Move to third phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Third phase should now be visible
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('block');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('hidden');
        
        // First two phases should be hidden but still mounted
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('block');
        
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('block');

        // Fourth phase should not be mounted at all
        expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
      });

      // Click back on first completed phase in timeline
      fireEvent.click(screen.getByTestId('timeline-linux-preflight'));

      await waitFor(() => {
        // First phase container should be visible
        expect(screen.getByTestId('linux-preflight-container')).toHaveClass('block');
        expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('hidden');
        
        // Second phase should still exist in DOM but hidden
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('block');
        
        // Third phase should still exist in DOM but hidden
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('block');
        
        // Future phases should not be mounted at all
        expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
      });

      // Click on second completed phase
      fireEvent.click(screen.getByTestId('timeline-linux-installation'));

      await waitFor(() => {
        // Second phase container should now be visible
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');
        
        // First and third phases should be hidden but still mounted
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('block');
        
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('app-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('app-preflight-container')).not.toHaveClass('block');

        // Fourth phase should not be mounted at all
        expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
      });
    });

    it('allows clicking on completed phases to view them', async () => {
      renderInstallationStep('linux');

      // Start with Linux preflight
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();

      // Complete first phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });

      // Now click back on the completed Linux preflight phase in timeline
      fireEvent.click(screen.getByTestId('timeline-linux-preflight'));
      
      await waitFor(() => {
        // Should show the previous phase content 
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
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
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.queryByTestId('linux-installation-phase')).not.toBeInTheDocument();
    });

    it('allows clicking on failed phases to view them', async () => {
      // Configure second phase to fail
      phaseMockConfig.linuxInstallation.outcome = 'failure';
      
      renderInstallationStep('linux');

      // Complete first phase (preflight) - should succeed and move to second phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should now be on the second phase (which is configured to fail)
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });

      // Try to complete the second phase - should fail and stay on same phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should show failure icon in timeline
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
        // Should still be showing the failed phase
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });

      // Should be able to click on the failed phase button in timeline
      const failedPhaseButton = screen.getByTestId('timeline-linux-installation');
      expect(failedPhaseButton).not.toBeDisabled();
      
      // Click on completed phase first
      fireEvent.click(screen.getByTestId('timeline-linux-preflight'));
      await waitFor(() => {
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      });
      
      // Then click back to failed phase
      fireEvent.click(failedPhaseButton);
      await waitFor(() => {
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });
    });

    it('shows mixed success and failure states in timeline', async () => {
      // Configure: first succeeds, second fails, rest pending
      phaseMockConfig.linuxInstallation.outcome = 'failure';
      
      renderInstallationStep('linux');

      // Complete first phase successfully
      fireEvent.click(screen.getByTestId('installation-next-button'));

      await waitFor(() => {
        // Should be on the second phase
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
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
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.queryByTestId('linux-installation-phase')).not.toBeInTheDocument();
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
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('installation-next-button')).toBeDisabled();
      });
    });
  });

  describe('Auto-advance functionality', () => {
    it('automatically advances when phase succeeds via auto state change', async () => {
      // Configure first phase to automatically succeed after 100ms
      phaseMockConfig.linuxPreflight.autoStateChange = { delay: 100, state: 'Succeeded' };
  
      renderInstallationStep('linux');
 
      // Should start with Linux preflight
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-preflight-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('hidden');
   
      // Wait for the phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });
   
      // Should automatically advance to Linux Installation Phase
      await waitFor(() => {
        // Second phase should now be visible
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-installation-container')).toHaveClass('block');
        expect(screen.getByTestId('linux-installation-container')).not.toHaveClass('hidden');

        // First phase should now be hidden but still mounted
        expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
        expect(screen.getByTestId('linux-preflight-container')).toHaveClass('hidden');
        expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('block');
 
        // Third phase should not be mounted at all
        expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-preflight-container')).not.toBeInTheDocument();
 
        // Fourth phase should not be mounted at all
        expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
        expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
      });
    });

    it('does not auto-advance when phase fails', async () => {
      // Configure first phase to automatically fail after 100ms
      phaseMockConfig.linuxPreflight.autoStateChange = { delay: 100, state: 'Failed' };

      renderInstallationStep('linux');

      // Should start with Linux preflight
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-preflight-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('hidden');

      // Wait for the phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Wait for the phase to auto-fail (this should happen)
      await waitFor(() => {
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });
      
      // Should still be on the same phase, not advance
      expect(screen.getByTestId('linux-preflight-phase')).toBeInTheDocument();
      expect(screen.getByTestId('linux-preflight-container')).toHaveClass('block');
      expect(screen.getByTestId('linux-preflight-container')).not.toHaveClass('hidden');

      // Second phase should not be mounted at all
      expect(screen.queryByTestId('linux-installation-phase')).not.toBeInTheDocument();
      expect(screen.queryByTestId('linux-installation-container')).not.toBeInTheDocument();

      // Third phase should not be mounted at all
      expect(screen.queryByTestId('app-preflight-phase')).not.toBeInTheDocument();
      expect(screen.queryByTestId('app-preflight-container')).not.toBeInTheDocument();

      // Fourth phase should not be mounted at all
      expect(screen.queryByTestId('app-installation-phase')).not.toBeInTheDocument();
      expect(screen.queryByTestId('app-installation-container')).not.toBeInTheDocument();
    });
  });

  describe('Back Button Behavior', () => {
    it('back button is only present for linux target when preflight fails', async () => {
      // Configure linux-preflight to auto-fail
      phaseMockConfig.linuxPreflight.autoStateChange = { delay: 100, state: 'Failed' };

      renderInstallationStep('linux');

      // Wait for phase to auto-fail
      await waitFor(() => {
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });

      // Back button should be present for linux target when preflight fails
      expect(screen.getByTestId('installation-back-button')).toBeInTheDocument();
    });

    it('back button is disabled during linux-preflight phase when status is Running', async () => {
      renderInstallationStep('linux');

      // Wait for phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Back button should be present but disabled during linux-preflight when running
      const backButton = screen.getByTestId('installation-back-button');
      expect(backButton).toBeInTheDocument();
      expect(backButton).toBeDisabled();
    });

    it('back button is enabled during linux-preflight phase when status is Failed', async () => {
      // Configure linux-preflight to auto-fail
      phaseMockConfig.linuxPreflight.autoStateChange = { delay: 100, state: 'Failed' };

      renderInstallationStep('linux');

      // Wait for phase to auto-fail
      await waitFor(() => {
        expect(screen.getByTestId('icon-failed')).toBeInTheDocument();
      });

      // Back button should be enabled when linux-preflight has failed
      const backButton = screen.getByTestId('installation-back-button');
      expect(backButton).not.toBeDisabled();
    });

    it('back button is not present after linux-preflight phase completes successfully', async () => {
      renderInstallationStep('linux');

      // Wait for phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Complete linux-preflight phase
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Wait for transition to linux-installation phase
      await waitFor(() => {
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });

      // Back button should not be present since linux-preflight succeeded
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });

    it('back button remains hidden during all phases after linux-preflight succeeds', async () => {
      renderInstallationStep('linux');

      // Progress through linux-preflight
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });
      fireEvent.click(screen.getByTestId('installation-next-button'));

      // Move to linux-installation phase
      await waitFor(() => {
        expect(screen.getByTestId('linux-installation-phase')).toBeInTheDocument();
      });

      // Back button should not be present since linux-preflight succeeded
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();

      // Progress to app-preflight
      fireEvent.click(screen.getByTestId('installation-next-button'));
      await waitFor(() => {
        expect(screen.getByTestId('app-preflight-phase')).toBeInTheDocument();
      });

      // Back button should still not be present since linux-preflight succeeded
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();

      // Progress to app-installation
      fireEvent.click(screen.getByTestId('installation-next-button'));
      await waitFor(() => {
        expect(screen.getByTestId('app-installation-phase')).toBeInTheDocument();
      });

      // Back button should still not be present since linux-preflight succeeded
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });

    it('back button is not present for kubernetes target since it has no host preflights', async () => {
      renderInstallationStep('kubernetes');

      // Wait for phase to initialize
      await waitFor(() => {
        expect(screen.getByTestId('installation-next-button')).not.toBeDisabled();
      });

      // Back button should not be present for kubernetes target
      expect(screen.queryByTestId('installation-back-button')).not.toBeInTheDocument();
    });
  });
});
