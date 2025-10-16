package upgrade

import (
	"slices"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/stretchr/testify/assert"
)

func TestStateMachineTransitions(t *testing.T) {
	tests := []struct {
		name                 string
		startState           statemachine.State
		validTransitions     []statemachine.State
		requiresInfraUpgrade bool
		isAirgap             bool
	}{
		// Common states across all flag combinations
		{
			name:                 `State "New" can transition to "ApplicationConfiguring"`,
			startState:           states.StateNew,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:                 `State "ApplicationConfiguring" can transition to "ApplicationConfigured" or "ApplicationConfigurationFailed"`,
			startState:           states.StateApplicationConfiguring,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfigured,
				states.StateApplicationConfigurationFailed,
			},
		},
		{
			name:                 `State "ApplicationConfigurationFailed" can transition to "ApplicationConfiguring"`,
			startState:           states.StateApplicationConfigurationFailed,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
			},
		},
		{
			name:                 `State "AppPreflightsRunning" can transition to "AppPreflightsSucceeded", "AppPreflightsFailed", or "AppPreflightsExecutionFailed"`,
			startState:           states.StateAppPreflightsRunning,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsSucceeded,
				states.StateAppPreflightsFailed,
				states.StateAppPreflightsExecutionFailed,
			},
		},
		{
			name:                 `State "AppPreflightsExecutionFailed" can transition to "AppPreflightsRunning"`,
			startState:           states.StateAppPreflightsExecutionFailed,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:                 `State "AppPreflightsFailed" can transition to "AppPreflightsRunning" or "AppPreflightsFailedBypassed"`,
			startState:           states.StateAppPreflightsFailed,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppPreflightsFailedBypassed,
			},
		},
		{
			name:                 `State "AppPreflightsSucceeded" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState:           states.StateAppPreflightsSucceeded,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:                 `State "AppPreflightsFailedBypassed" can transition to "AppPreflightsRunning" or "AppUpgrading"`,
			startState:           states.StateAppPreflightsFailedBypassed,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
				states.StateAppUpgrading,
			},
		},
		{
			name:                 `State "AppUpgrading" can transition to "Succeeded" or "AppUpgradeFailed"`,
			startState:           states.StateAppUpgrading,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateSucceeded,
				states.StateAppUpgradeFailed,
			},
		},
		{
			name:                 `State "AppUpgradeFailed" can not transition to any other state`,
			startState:           states.StateAppUpgradeFailed,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions:     []statemachine.State{},
		},
		{
			name:                 `State "Succeeded" can not transition to any other state`,
			startState:           states.StateSucceeded,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions:     []statemachine.State{},
		},

		// States specific to requiresInfraUpgrade=true, isAirgap=false
		{
			name:                 `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "InfrastructureUpgrading" (requiresInfraUpgrade=true, isAirgap=false)`,
			startState:           states.StateApplicationConfigured,
			requiresInfraUpgrade: true,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateInfrastructureUpgrading,
			},
		},
		{
			name:                 `State "InfrastructureUpgrading" can transition to "InfrastructureUpgraded" or "InfrastructureUpgradeFailed" (requiresInfraUpgrade=true, isAirgap=false)`,
			startState:           states.StateInfrastructureUpgrading,
			requiresInfraUpgrade: true,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateInfrastructureUpgraded,
				states.StateInfrastructureUpgradeFailed,
			},
		},
		{
			name:                 `State "InfrastructureUpgraded" can transition to "AppPreflightsRunning" (requiresInfraUpgrade=true, isAirgap=false)`,
			startState:           states.StateInfrastructureUpgraded,
			requiresInfraUpgrade: true,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:                 `State "InfrastructureUpgradeFailed" can not transition to any other state (requiresInfraUpgrade=true, isAirgap=false)`,
			startState:           states.StateInfrastructureUpgradeFailed,
			requiresInfraUpgrade: true,
			isAirgap:             false,
			validTransitions:     []statemachine.State{},
		},

		// States specific to requiresInfraUpgrade=false, isAirgap=false
		{
			name:                 `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "AppPreflightsRunning" (requiresInfraUpgrade=false, isAirgap=false)`,
			startState:           states.StateApplicationConfigured,
			requiresInfraUpgrade: false,
			isAirgap:             false,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateAppPreflightsRunning,
			},
		},

		// States specific to requiresInfraUpgrade=true, isAirgap=true
		{
			name:                 `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "AirgapProcessing" (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateApplicationConfigured,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateAirgapProcessing,
			},
		},
		{
			name:                 `State "AirgapProcessing" can transition to "AirgapProcessed" or "AirgapProcessingFailed" (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateAirgapProcessing,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateAirgapProcessed,
				states.StateAirgapProcessingFailed,
			},
		},
		{
			name:                 `State "AirgapProcessed" can transition to "InfrastructureUpgrading" (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateAirgapProcessed,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateInfrastructureUpgrading,
			},
		},
		{
			name:                 `State "AirgapProcessingFailed" can not transition to any other state (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateAirgapProcessingFailed,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions:     []statemachine.State{},
		},
		{
			name:                 `State "InfrastructureUpgrading" can transition to "InfrastructureUpgraded" or "InfrastructureUpgradeFailed" (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateInfrastructureUpgrading,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateInfrastructureUpgraded,
				states.StateInfrastructureUpgradeFailed,
			},
		},
		{
			name:                 `State "InfrastructureUpgraded" can transition to "AppPreflightsRunning" (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateInfrastructureUpgraded,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:                 `State "InfrastructureUpgradeFailed" can not transition to any other state (requiresInfraUpgrade=true, isAirgap=true)`,
			startState:           states.StateInfrastructureUpgradeFailed,
			requiresInfraUpgrade: true,
			isAirgap:             true,
			validTransitions:     []statemachine.State{},
		},

		// States specific to requiresInfraUpgrade=false, isAirgap=true
		{
			name:                 `State "ApplicationConfigured" can transition to "ApplicationConfiguring" or "AirgapProcessing" (requiresInfraUpgrade=false, isAirgap=true)`,
			startState:           states.StateApplicationConfigured,
			requiresInfraUpgrade: false,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateApplicationConfiguring,
				states.StateAirgapProcessing,
			},
		},
		{
			name:                 `State "AirgapProcessing" can transition to "AirgapProcessed" or "AirgapProcessingFailed" (requiresInfraUpgrade=false, isAirgap=true)`,
			startState:           states.StateAirgapProcessing,
			requiresInfraUpgrade: false,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateAirgapProcessed,
				states.StateAirgapProcessingFailed,
			},
		},
		{
			name:                 `State "AirgapProcessed" can transition to "AppPreflightsRunning" (requiresInfraUpgrade=false, isAirgap=true)`,
			startState:           states.StateAirgapProcessed,
			requiresInfraUpgrade: false,
			isAirgap:             true,
			validTransitions: []statemachine.State{
				states.StateAppPreflightsRunning,
			},
		},
		{
			name:                 `State "AirgapProcessingFailed" can not transition to any other state (requiresInfraUpgrade=false, isAirgap=true)`,
			startState:           states.StateAirgapProcessingFailed,
			requiresInfraUpgrade: false,
			isAirgap:             true,
			validTransitions:     []statemachine.State{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validStateTransitions := buildStateTransitions(tt.requiresInfraUpgrade, tt.isAirgap)

			for nextState := range validStateTransitions {
				sm := NewStateMachine(
					WithCurrentState(tt.startState),
					WithRequiresInfraUpgrade(tt.requiresInfraUpgrade),
					WithIsAirgap(tt.isAirgap),
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
		states.StateAirgapProcessingFailed,
	}

	testCases := []struct {
		name                 string
		requiresInfraUpgrade bool
		isAirgap             bool
	}{
		{
			name:                 "requiresInfraUpgrade=true, isAirgap=false",
			requiresInfraUpgrade: true,
			isAirgap:             false,
		},
		{
			name:                 "requiresInfraUpgrade=false, isAirgap=false",
			requiresInfraUpgrade: false,
			isAirgap:             false,
		},
		{
			name:                 "requiresInfraUpgrade=true, isAirgap=true",
			requiresInfraUpgrade: true,
			isAirgap:             true,
		},
		{
			name:                 "requiresInfraUpgrade=false, isAirgap=true",
			requiresInfraUpgrade: false,
			isAirgap:             true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validStateTransitions := buildStateTransitions(tc.requiresInfraUpgrade, tc.isAirgap)
			for state := range validStateTransitions {
				var isFinal bool
				if slices.Contains(finalStates, state) {
					isFinal = true
				}

				sm := NewStateMachine(
					WithCurrentState(state),
					WithRequiresInfraUpgrade(tc.requiresInfraUpgrade),
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
