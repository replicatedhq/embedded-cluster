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
			name:       `State "New" can transition to "InstallationConfigured"`,
			startState: StateNew,
			validTransitions: []statemachine.State{
				StateInstallationConfigured,
			},
		},
		{
			name:       `State "InstallationConfigured" can transition to "PreflightsRunning" or "InstallationConfigured"`,
			startState: StateInstallationConfigured,
			validTransitions: []statemachine.State{
				StatePreflightsRunning,
				StateInstallationConfigured,
			},
		},
		{
			name:       `State "PreflightsRunning" can transition to "PreflightsSucceeded" or "PreflightsFailed"`,
			startState: StatePreflightsRunning,
			validTransitions: []statemachine.State{
				StatePreflightsSucceeded,
				StatePreflightsFailed,
			},
		},
		{
			name:       `State "PreflightsSucceeded" can transition to "InfrastructureInstalling", "PreflightsRunning" or "InstallationConfigured"`,
			startState: StatePreflightsSucceeded,
			validTransitions: []statemachine.State{
				StateInfrastructureInstalling,
				StatePreflightsRunning,
				StateInstallationConfigured,
			},
		},
		{
			name:       `State "PreflightsFailed" can transition to "PreflightsFailedBypassed" , "PreflightsRunning" or "InstallationConfigured"`,
			startState: StatePreflightsFailed,
			validTransitions: []statemachine.State{
				StatePreflightsFailedBypassed,
				StatePreflightsRunning,
				StateInstallationConfigured,
			},
		},
		{
			name:       `State "PreflightsFailedBypassed" can transition to "InfrastructureInstalling", "PreflightsRunning" or "InstallationConfigured"`,
			startState: StatePreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				StateInfrastructureInstalling,
				StatePreflightsRunning,
				StateInstallationConfigured,
			},
		},
		{
			name:       `State "InfrastructureInstalling" can transition to "Succeeded" or "Failed"`,
			startState: StateInfrastructureInstalling,
			validTransitions: []statemachine.State{
				StateSucceeded,
				StateFailed,
			},
		},
		{
			name:             `State "Succeeded" can not transition to any other state`,
			startState:       StateSucceeded,
			validTransitions: []statemachine.State{},
		},
		{
			name:             `State "Failed" can not transition to any other state`,
			startState:       StateFailed,
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
		StateFailed,
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
