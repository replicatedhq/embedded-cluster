package upgrade

import (
	"slices"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/stretchr/testify/assert"
)

func TestStateMachineTransitionsWithInfraUpgrade(t *testing.T) {
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
			name:       `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "InfrastructureUpgrading"`,
			startState: states.StateApplicationConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInfrastructureUpgrading,
			},
		},
		{
			name:       `State "InfrastructureUpgrading" can transition to "InfrastructureUpgraded" or "InfrastructureUpgradeFailed"`,
			startState: states.StateInfrastructureUpgrading,
			validTransitions: []statemachine.State{
				states.StateInfrastructureUpgraded,
				states.StateInfrastructureUpgradeFailed,
			},
		},
		{
			name:       `State "InfrastructureUpgraded" can transition to "AppPreflightsRunning"`,
			startState: states.StateInfrastructureUpgraded,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:             `State "InfrastructureUpgradeFailed" can not transition to any other state`,
			startState:       states.StateInfrastructureUpgradeFailed,
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
			name:       `State "AppPreflightsSucceeded" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState: states.StateAppPreflightsSucceeded,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:       `State "AppPreflightsFailedBypassed" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState: states.StateAppPreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:       `State "AppUpgrading" can transition to "Succeeded" or "AppUpgradeFailed"`,
			startState: states.StateAppUpgrading,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateAppUpgradeFailed,
			},
		},
		{
			name:             `State "AppUpgradeFailed" can not transition to any other state`,
			startState:       states.StateAppUpgradeFailed,
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
			validStateTransitions := buildStateTransitions(true)

			for nextState := range validStateTransitions {
				sm := NewStateMachine(
					WithCurrentState(tt.startState),
					WithRequiresInfraUpgrade(true),
				)

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
					assert.Equal(t, nextState, sm.CurrentState(), "state should change after transition")
				}
			}
		})
	}
}

func TestStateMachineTransitionsWithoutInfraUpgrade(t *testing.T) {
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
			name:       `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "AppPreflightsRunning"`,
			startState: states.StateApplicationConfigured,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateAppPreflightsRunning,
			},
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
			name:       `State "AppPreflightsSucceeded" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState: states.StateAppPreflightsSucceeded,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:       `State "AppPreflightsFailedBypassed" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState: states.StateAppPreflightsFailedBypassed,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:       `State "AppUpgrading" can transition to "Succeeded" or "AppUpgradeFailed"`,
			startState: states.StateAppUpgrading,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateAppUpgradeFailed,
			},
		},
		{
			name:             `State "AppUpgradeFailed" can not transition to any other state`,
			startState:       states.StateAppUpgradeFailed,
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
			validStateTransitions := buildStateTransitions(false)

			for nextState := range validStateTransitions {
				sm := NewStateMachine(
					WithCurrentState(tt.startState),
					WithRequiresInfraUpgrade(false),
				)

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
					assert.Equal(t, nextState, sm.CurrentState(), "state should change after transition")
				}
			}
		})
	}
}

func TestIsFinalStateUpgrade(t *testing.T) {
	finalStates := []statemachine.State{
		states.StateSucceeded,
		states.StateInfrastructureUpgradeFailed,
		states.StateAppUpgradeFailed,
	}

	// Test with infra upgrade required
	validStateTransitionsWithInfra := buildStateTransitions(true)
	for state := range validStateTransitionsWithInfra {
		var isFinal bool
		if slices.Contains(finalStates, state) {
			isFinal = true
		}

		sm := NewStateMachine(
			WithCurrentState(state),
			WithRequiresInfraUpgrade(true),
		)

		if isFinal {
			assert.True(t, sm.IsFinalState(), "expected state %s to be final", state)
		} else {
			assert.False(t, sm.IsFinalState(), "expected state %s to not be final", state)
		}
	}

	// Test without infra upgrade required
	validStateTransitionsWithoutInfra := buildStateTransitions(false)
	for state := range validStateTransitionsWithoutInfra {
		var isFinal bool
		if slices.Contains(finalStates, state) {
			isFinal = true
		}

		sm := NewStateMachine(
			WithCurrentState(state),
			WithRequiresInfraUpgrade(false),
		)

		if isFinal {
			assert.True(t, sm.IsFinalState(), "expected state %s to be final", state)
		} else {
			assert.False(t, sm.IsFinalState(), "expected state %s to not be final", state)
		}
	}
}

func TestBuildStateTransitions(t *testing.T) {
	t.Run("with infra upgrade required", func(t *testing.T) {
		transitions := buildStateTransitions(true)

		// Verify infra states are included
		assert.Contains(t, transitions, states.StateInfrastructureUpgrading)
		assert.Contains(t, transitions, states.StateInfrastructureUpgraded)
		assert.Contains(t, transitions, states.StateInfrastructureUpgradeFailed)

		// Verify ApplicationConfigured can transition to InfrastructureUpgrading
		assert.Contains(t, transitions[states.StateApplicationConfigured], states.StateInfrastructureUpgrading)

		// Verify ApplicationConfigured cannot transition to AppPreflightsRunning
		assert.NotContains(t, transitions[states.StateApplicationConfigured], states.StateAppPreflightsRunning)
	})

	t.Run("without infra upgrade required", func(t *testing.T) {
		transitions := buildStateTransitions(false)

		// Verify infra states are NOT included
		assert.NotContains(t, transitions, states.StateInfrastructureUpgrading)
		assert.NotContains(t, transitions, states.StateInfrastructureUpgraded)
		assert.NotContains(t, transitions, states.StateInfrastructureUpgradeFailed)

		// Verify ApplicationConfigured can transition to AppPreflightsRunning
		assert.Contains(t, transitions[states.StateApplicationConfigured], states.StateAppPreflightsRunning)

		// Verify ApplicationConfigured cannot transition to InfrastructureUpgrading
		assert.NotContains(t, transitions[states.StateApplicationConfigured], states.StateInfrastructureUpgrading)
	})
}
