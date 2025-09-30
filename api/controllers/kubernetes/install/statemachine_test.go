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
			name:       `State "InstallationConfigured" can transition to "ApplicationConfiguring" or "InstallationConfiguring" or "InfrastructureInstalling"`,
			startState: states.StateInstallationConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInstallationConfiguring,
				states.StateInfrastructureInstalling,
			},
		},
		{
			name:       `State "InfrastructureInstalling" can transition to "InfrastructureInstalled" or "InfrastructureInstallFailed"`,
			startState: states.StateInfrastructureInstalling,
			validTransitions: []statemachine.State{
				states.StateInfrastructureInstalled,
				states.StateInfrastructureInstallFailed,
			},
		},
		{
			name:       `State "InfrastructureInstalled" can transition to "AppPreflightsRunning"`,
			startState: states.StateInfrastructureInstalled,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:             `State "InfrastructureInstallFailed" can not transition to any other state`,
			startState:       states.StateInfrastructureInstallFailed,
			validTransitions: []statemachine.State{},
		},
		{
			name:       `State "AppPreflightsRunning" can transition to "AppPreflightsSucceeded", "AppPreflightsFailed", or "AppPreflightsExecutionFailed"`,
			startState: states.StateAppPreflightsRunning,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsSucceeded,
				states.StateAppPreflightsFailed,
				states.StateAppPreflightsExecutionFailed,
			},
		},
		{
			name:       `State "AppPreflightsExecutionFailed" can transition to "AppPreflightsRunning"`,
			startState: states.StateAppPreflightsExecutionFailed,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:       `State "AppPreflightsFailed" can transition to "AppPreflightsRunning" or "AppPreflightsFailedBypassed"`,
			startState: states.StateAppPreflightsFailed,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppPreflightsFailedBypassed,
			},
		},
		{
			name:       `State "AppPreflightsSucceeded" can transition to "AppPreflightsRunning" and "AppInstalling"`,
			startState: states.StateAppPreflightsSucceeded,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppInstalling,
			},
		},
		{
			name:       `State "AppPreflightsFailedBypassed" can transition to "AppPreflightsRunning" and "AppInstalling"`,
			startState: states.StateAppPreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppInstalling,
			},
		},
		{
			name:       `State "AppInstalling" can transition to "Succeeded" or "AppInstallFailed"`,
			startState: states.StateAppInstalling,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateAppInstallFailed,
			},
		},
		{
			name:             `State "AppInstallFailed" can not transition to any other state`,
			startState:       states.StateAppInstallFailed,
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
		states.StateAppInstallFailed,
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
