package install

import (
	"slices"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/stretchr/testify/assert"
)

func TestStateMachineTransitions(t *testing.T) {
	tests := []struct {
		name             string
		startState       statemachine.State
		validTransitions []statemachine.State
	}{
		{
			name:       `State "New" can transition to "ApplicationConfiguring"`,
			startState: states.StateNew,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:       `State "ApplicationConfiguring" can transition to "ApplicationConfigured" or "ApplicationConfigurationFailed"`,
			startState: states.StateApplicationConfiguring,
			validTransitions: []statemachine.State{
				states.StateApplicationConfigured,
				states.StateApplicationConfigurationFailed,
			},
		},
		{
			name:       `State "ApplicationConfigurationFailed" can transition to "ApplicationConfiguring"`,
			startState: states.StateApplicationConfigurationFailed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:       `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "InstallationConfiguring"`,
			startState: states.StateApplicationConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
			},
		},
		{
			name:       `State "InstallationConfiguring" can transition to "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: states.StateInstallationConfiguring,
			validTransitions: []statemachine.State{
				states.StateInstallationConfigured,
				states.StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "InstallationConfigurationFailed" can transition to "ApplicationConfiguring" or "InstallationConfiguring"`,
			startState: states.StateInstallationConfigurationFailed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
			},
		},
		{
			name:       `State "InstallationConfigured" can transition to "ApplicationConfiguring", "InstallationConfiguring", or "HostConfiguring"`,
			startState: states.StateInstallationConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
			},
		},
		{
			name:       `State "HostConfiguring" can transition to "HostConfigured" or "HostConfigurationFailed"`,
			startState: states.StateHostConfiguring,
			validTransitions: []statemachine.State{
				states.StateHostConfigured,
				states.StateHostConfigurationFailed,
			},
		},
		{
			name:       `State "HostConfigurationFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", or "HostConfiguring"`,
			startState: states.StateHostConfigurationFailed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
			},
		},
		{
			name:       `State "HostConfigured" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", or "PreflightsRunning"`,
			startState: states.StateHostConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StatePreflightsRunning,
			},
		},
		{
			name:       `State "PreflightsRunning" can transition to "PreflightsSucceeded", "PreflightsFailed", or "PreflightsExecutionFailed"`,
			startState: states.StatePreflightsRunning,
			validTransitions: []statemachine.State{
				states.StatePreflightsSucceeded,
				states.StatePreflightsFailed,
				states.StatePreflightsExecutionFailed,
			},
		},
		{
			name:       `State "PreflightsExecutionFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", or "PreflightsRunning"`,
			startState: states.StatePreflightsExecutionFailed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StatePreflightsRunning,
			},
		},
		{
			name:       `State "PreflightsSucceeded" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "InfrastructureInstalling"`,
			startState: states.StatePreflightsSucceeded,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StatePreflightsRunning,
				states.StateInfrastructureInstalling,
			},
		},
		{
			name:       `State "PreflightsFailed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "PreflightsFailedBypassed"`,
			startState: states.StatePreflightsFailed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StatePreflightsRunning,
				states.StatePreflightsFailedBypassed,
			},
		},
		{
			name:       `State "PreflightsFailedBypassed" can transition to "ApplicationConfiguring", "InstallationConfiguring", "HostConfiguring", "PreflightsRunning", or "InfrastructureInstalling"`,
			startState: states.StatePreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateHostConfiguring,
				states.StatePreflightsRunning,
				states.StateInfrastructureInstalling,
			},
		},
		{
			name:       `State "InfrastructureInstalling" can transition to "Succeeded" or "InfrastructureInstallFailed"`,
			startState: states.StateInfrastructureInstalling,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateInfrastructureInstallFailed,
			},
		},
		{
			name:             `State "InfrastructureInstallFailed" can not transition to any other state`,
			startState:       states.StateInfrastructureInstallFailed,
			validTransitions: []statemachine.State{},
		},
		{
			name:             `State "Succeeded" can not transition to any other state`,
			startState:       states.StateSucceeded,
			validTransitions: []statemachine.State{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for nextState := range validStateTransitions {
				sm := NewStateMachine(WithCurrentState(tt.startState))

				lock, err := sm.AcquireLock()
				if err != nil {
					t.Fatalf("failed to acquire lock: %v", err)
				}
				defer lock.Release()

				err = sm.Transition(lock, nextState)
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
	}

	for state := range validStateTransitions {
		var isFinal bool
		if slices.Contains(finalStates, state) {
			isFinal = true
		}

		sm := NewStateMachine(WithCurrentState(state))

		if isFinal {
			assert.True(t, sm.IsFinalState(), "expected state %s to be final", state)
		} else {
			assert.False(t, sm.IsFinalState(), "expected state %s to not be final", state)
		}
	}
}
