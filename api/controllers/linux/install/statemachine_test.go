package install

import (
	"slices"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/stretchr/testify/assert"
)

func TestStateMachineTransitions(t *testing.T) {
	tests := []struct {
		name             string
		startState       statemachine.State
		validTransitions []statemachine.State
	}{
		{
			name:       `State "New" can transition to "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: StateNew,
			validTransitions: []statemachine.State{
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "InstallationConfigurationFailed" can transition to "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: StateInstallationConfigurationFailed,
			validTransitions: []statemachine.State{
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "InstallationConfigured" can transition to "HostConfigured" or "HostConfigurationFailed"`,
			startState: StateInstallationConfigured,
			validTransitions: []statemachine.State{
				StateHostConfigured,
				StateHostConfigurationFailed,
			},
		},
		{
			name:       `State "HostConfigurationFailed" can transition to "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: StateHostConfigurationFailed,
			validTransitions: []statemachine.State{
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "HostConfigured" can transition to "PreflightsRunning" or "InstallationConfigured" or "InstallationConfigurationFailed"`,
			startState: StateHostConfigured,
			validTransitions: []statemachine.State{
				StatePreflightsRunning,
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "PreflightsRunning" can transition to "PreflightsSucceeded", "PreflightsFailed", or "PreflightsExecutionFailed"`,
			startState: StatePreflightsRunning,
			validTransitions: []statemachine.State{
				StatePreflightsSucceeded,
				StatePreflightsFailed,
				StatePreflightsExecutionFailed,
			},
		},
		{
			name:       `State "PreflightsExecutionFailed" can transition to "PreflightsRunning", "InstallationConfigured", or "InstallationConfigurationFailed"`,
			startState: StatePreflightsExecutionFailed,
			validTransitions: []statemachine.State{
				StatePreflightsRunning,
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "PreflightsSucceeded" can transition to "InfrastructureInstalling", "PreflightsRunning", "InstallationConfigured", or "InstallationConfigurationFailed"`,
			startState: StatePreflightsSucceeded,
			validTransitions: []statemachine.State{
				StateInfrastructureInstalling,
				StatePreflightsRunning,
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "PreflightsFailed" can transition to "PreflightsFailedBypassed", "PreflightsRunning", "InstallationConfigured", or "InstallationConfigurationFailed"`,
			startState: StatePreflightsFailed,
			validTransitions: []statemachine.State{
				StatePreflightsFailedBypassed,
				StatePreflightsRunning,
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "PreflightsFailedBypassed" can transition to "InfrastructureInstalling", "PreflightsRunning", "InstallationConfigured", or "InstallationConfigurationFailed"`,
			startState: StatePreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				StateInfrastructureInstalling,
				StatePreflightsRunning,
				StateInstallationConfigured,
				StateInstallationConfigurationFailed,
			},
		},
		{
			name:       `State "InfrastructureInstalling" can transition to "Succeeded" or "InfrastructureInstallFailed"`,
			startState: StateInfrastructureInstalling,
			validTransitions: []statemachine.State{
				StateSucceeded,
				StateInfrastructureInstallFailed,
			},
		},
		{
			name:             `State "InfrastructureInstallFailed" can not transition to any other state`,
			startState:       StateInfrastructureInstallFailed,
			validTransitions: []statemachine.State{},
		},
		{
			name:             `State "Succeeded" can not transition to any other state`,
			startState:       StateSucceeded,
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
		StateSucceeded,
		StateInfrastructureInstallFailed,
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
