package install

import (
	"slices"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/stretchr/testify/assert"
)

func TestStateMachineTransitions(t *testing.T) {
	tests := []struct {
		name             string
		startState       statemachine.State
		validTransitions []statemachine.State
		isAirgap         bool
	}{
		// Common states across all flag combinations
		{
			name:       `State "New" can transition to "ApplicationConfiguring"`,
			startState: states.StateNew,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:       `State "ApplicationConfiguring" can transition to "ApplicationConfigured" or "ApplicationConfigurationFailed"`,
			startState: states.StateApplicationConfiguring,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfigured,
				states.StateApplicationConfigurationFailed,
			},
		},
		{
			name:       `State "ApplicationConfigurationFailed" can transition to "ApplicationConfiguring"`,
			startState: states.StateApplicationConfigurationFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:       `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "InstallationConfiguring"`,
			startState: states.StateApplicationConfigured,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
			},
		},
		{
			name:       `State "InstallationConfiguring" can transition to "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: states.StateInstallationConfiguring,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateInstallationConfigured,
				states.StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "InstallationConfigurationFailed" can transition to "ApplicationConfiguring" or "InstallationConfiguring"`,
			startState: states.StateInstallationConfigurationFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
			},
		},
		{
			name:       `State "InstallationConfigured" can transition to "ApplicationConfiguring", "InstallationConfiguring", or "HostConfiguring"`,
			startState: states.StateInstallationConfigured,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
			},
		},
		{
			name:       `State "HostConfiguring" can transition to "HostConfigured" or "HostConfigurationFailed"`,
			startState: states.StateHostConfiguring,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateHostConfigured,
				states.StateHostConfigurationFailed,
			},
		},
		{
			name:       `State "HostConfigurationFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", or "HostConfiguring"`,
			startState: states.StateHostConfigurationFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
			},
		},
		{
			name:       `State "HostConfigured" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", or "PreflightsRunning"`,
			startState: states.StateHostConfigured,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StateHostPreflightsRunning,
			},
		},
		{
			name:       `State "PreflightsRunning" can transition to "PreflightsSucceeded", "PreflightsFailed", or "PreflightsExecutionFailed"`,
			startState: states.StateHostPreflightsRunning,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateHostPreflightsSucceeded,
				states.StateHostPreflightsFailed,
				states.StateHostPreflightsExecutionFailed,
			},
		},
		{
			name:       `State "PreflightsExecutionFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", or "PreflightsRunning"`,
			startState: states.StateHostPreflightsExecutionFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StateHostPreflightsRunning,
			},
		},
		{
			name:       `State "PreflightsSucceeded" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "InfrastructureInstalling"`,
			startState: states.StateHostPreflightsSucceeded,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StateHostPreflightsRunning,
				states.StateInfrastructureInstalling,
			},
		},
		{
			name:       `State "PreflightsFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "PreflightsFailedBypassed"`,
			startState: states.StateHostPreflightsFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StateHostPreflightsRunning,
				states.StateHostPreflightsFailedBypassed,
			},
		},
		{
			name:       `State "PreflightsFailedBypassed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "InfrastructureInstalling"`,
			startState: states.StateHostPreflightsFailedBypassed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StateHostPreflightsRunning,
				states.StateInfrastructureInstalling,
			},
		},
		{
			name:       `State "InfrastructureInstalling" can transition to "InfrastructureInstalled" or "InfrastructureInstallFailed"`,
			startState: states.StateInfrastructureInstalling,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateInfrastructureInstalled,
				states.StateInfrastructureInstallFailed,
			},
		},
		{
			name:             `State "InfrastructureInstallFailed" can not transition to any other state`,
			startState:       states.StateInfrastructureInstallFailed,
			isAirgap:         false,
			validTransitions: []statemachine.State{},
		},
		{
			name:       `State "AppPreflightsRunning" can transition to "AppPreflightsSucceeded", "AppPreflightsFailed", or "AppPreflightsExecutionFailed"`,
			startState: states.StateAppPreflightsRunning,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsSucceeded,
				states.StateAppPreflightsFailed,
				states.StateAppPreflightsExecutionFailed,
			},
		},
		{
			name:       `State "AppPreflightsExecutionFailed" can transition to "AppPreflightsRunning"`,
			startState: states.StateAppPreflightsExecutionFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:       `State "AppPreflightsFailed" can transition to "AppPreflightsRunning" or "AppPreflightsFailedBypassed"`,
			startState: states.StateAppPreflightsFailed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppPreflightsFailedBypassed,
			},
		},
		{
			name:       `State "AppPreflightsSucceeded" can transition to "AppPreflightsRunning" and "AppInstalling"`,
			startState: states.StateAppPreflightsSucceeded,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppInstalling,
			},
		},
		{
			name:       `State "AppPreflightsFailedBypassed" can transition to "AppPreflightsRunning" and "AppInstalling"`,
			startState: states.StateAppPreflightsFailedBypassed,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppInstalling,
			},
		},
		{
			name:       `State "AppInstalling" can transition to "Succeeded" or "AppInstallFailed"`,
			startState: states.StateAppInstalling,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateAppInstallFailed,
			},
		},
		{
			name:             `State "AppInstallFailed" can not transition to any other state`,
			startState:       states.StateAppInstallFailed,
			isAirgap:         false,
			validTransitions: []statemachine.State{},
		},
		{
			name:             `State "Succeeded" can not transition to any other state`,
			startState:       states.StateSucceeded,
			isAirgap:         false,
			validTransitions: []statemachine.State{},
		},

		// States specific to isAirgap=true
		{
			name:       `State "InfrastructureInstalled" can transition to "AirgapProcessing" (isAirgap=true)`,
			startState: states.StateInfrastructureInstalled,
			isAirgap:   true,
			validTransitions: []statemachine.State{
				states.StateAirgapProcessing,
			},
		},
		{
			name:       `State "AirgapProcessing" can transition to "AirgapProcessed" or "AirgapProcessingFailed" (isAirgap=true)`,
			startState: states.StateAirgapProcessing,
			isAirgap:   true,
			validTransitions: []statemachine.State{
				states.StateAirgapProcessed,
				states.StateAirgapProcessingFailed,
			},
		},
		{
			name:       `State "AirgapProcessed" can transition to "AppPreflightsRunning" (isAirgap=true)`,
			startState: states.StateAirgapProcessed,
			isAirgap:   true,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:             `State "AirgapProcessingFailed" can not transition to any other state (isAirgap=true)`,
			startState:       states.StateAirgapProcessingFailed,
			isAirgap:         true,
			validTransitions: []statemachine.State{},
		},

		// States specific to isAirgap=false
		{
			name:       `State "InfrastructureInstalled" can transition to "AppPreflightsRunning" (isAirgap=false)`,
			startState: states.StateInfrastructureInstalled,
			isAirgap:   false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validStateTransitions := buildStateTransitions(tt.isAirgap)

			for nextState := range validStateTransitions {
				sm := NewStateMachine(
					WithCurrentState(tt.startState),
					WithIsAirgap(tt.isAirgap),
				)

				lock, err := sm.AcquireLock()
				if err != nil {
					t.Fatalf("failed to acquire lock: %v", err)
				}
				defer lock.Release()

				err = sm.Transition(lock, nextState, nil)
				if !slices.Contains(tt.validTransitions, nextState) {
					assert.Error(t, err, "expected error for transition from %s to %s", tt.startState, nextState)
				} else {
					assert.NoError(t, err, "unexpected error for transition from %s to %s", tt.startState, nextState)

					// Verify state has changed
					assert.Equal(t, nextState, sm.CurrentState(), "state should change after commit")
				}
			}
		})
	}
}

func TestIsFinalState(t *testing.T) {
	finalStates := []statemachine.State{
		states.StateSucceeded,
		states.StateInfrastructureInstallFailed,
		states.StateAppInstallFailed,
		states.StateAirgapProcessingFailed,
	}

	testCases := []struct {
		name     string
		isAirgap bool
	}{
		{
			name:     "isAirgap=false",
			isAirgap: false,
		},
		{
			name:     "isAirgap=true",
			isAirgap: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validStateTransitions := buildStateTransitions(tc.isAirgap)
			for state := range validStateTransitions {
				var isFinal bool
				if slices.Contains(finalStates, state) {
					isFinal = true
				}

				sm := NewStateMachine(
					WithCurrentState(state),
					WithIsAirgap(tc.isAirgap),
				)

				if isFinal {
					assert.True(t, sm.IsFinalState(), "expected state %s to be final", state)
				} else {
					assert.False(t, sm.IsFinalState(), "expected state %s to not be final", state)
				}
			}
		})
	}
}
