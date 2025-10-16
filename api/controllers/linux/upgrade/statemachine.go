package upgrade

import (
	"maps"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

// Base state transitions that are common to both upgrade flows regardless of whether infra upgrade is required or not
var baseStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateNew:                            {states.StateApplicationConfiguring},
	states.StateApplicationConfiguring:         {states.StateApplicationConfigured, states.StateApplicationConfigurationFailed},
	states.StateApplicationConfigurationFailed: {states.StateApplicationConfiguring},
	states.StateApplicationConfigured:          {states.StateApplicationConfiguring}, // Common transition, will add more based on infra requirement
	states.StateAppPreflightsRunning:           {states.StateAppPreflightsSucceeded, states.StateAppPreflightsFailed, states.StateAppPreflightsExecutionFailed},
	states.StateAppPreflightsExecutionFailed:   {states.StateAppPreflightsRunning},
	states.StateAppPreflightsSucceeded:         {states.StateAppPreflightsRunning, states.StateAppUpgrading},
	states.StateAppPreflightsFailed:            {states.StateAppPreflightsRunning, states.StateAppPreflightsFailedBypassed},
	states.StateAppPreflightsFailedBypassed:    {states.StateAppPreflightsRunning, states.StateAppUpgrading},
	states.StateAppUpgrading:                   {states.StateSucceeded, states.StateAppUpgradeFailed},
	// final states
	states.StateSucceeded:        {},
	states.StateAppUpgradeFailed: {},
}

// Infrastructure-specific state transitions
var infraStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateInfrastructureUpgrading: {states.StateInfrastructureUpgraded, states.StateInfrastructureUpgradeFailed},
	states.StateInfrastructureUpgraded:  {states.StateAppPreflightsRunning},
	// final states
	states.StateInfrastructureUpgradeFailed: {},
}

// Build state transitions based on whether infra upgrade is required
func buildStateTransitions(requiresInfraUpgrade bool) map[statemachine.State][]statemachine.State {
	transitions := make(map[statemachine.State][]statemachine.State)

	// Copy base transitions
	maps.Copy(transitions, baseStateTransitions)

	// Add infrastructure-specific transitions if needed
	if requiresInfraUpgrade {
		maps.Copy(transitions, infraStateTransitions)

		// Add infrastructure upgrade as next state from ApplicationConfigured
		transitions[states.StateApplicationConfigured] = append(
			transitions[states.StateApplicationConfigured],
			states.StateInfrastructureUpgrading,
		)
	} else {
		// Add app preflights as next state from ApplicationConfigured
		transitions[states.StateApplicationConfigured] = append(
			transitions[states.StateApplicationConfigured],
			states.StateAppPreflightsRunning,
		)
	}

	return transitions
}

type StateMachineOptions struct {
	CurrentState         statemachine.State
	Logger               logrus.FieldLogger
	RequiresInfraUpgrade bool
}

type StateMachineOption func(*StateMachineOptions)

func WithCurrentState(currentState statemachine.State) StateMachineOption {
	return func(o *StateMachineOptions) {
		o.CurrentState = currentState
	}
}

func WithStateMachineLogger(logger logrus.FieldLogger) StateMachineOption {
	return func(o *StateMachineOptions) {
		o.Logger = logger
	}
}

func WithRequiresInfraUpgrade(requiresInfraUpgrade bool) StateMachineOption {
	return func(o *StateMachineOptions) {
		o.RequiresInfraUpgrade = requiresInfraUpgrade
	}
}

// NewStateMachine creates a new state machine starting in the New state
func NewStateMachine(opts ...StateMachineOption) statemachine.Interface {
	options := &StateMachineOptions{
		CurrentState: states.StateNew,
		Logger:       logger.NewDiscardLogger(),
	}
	for _, opt := range opts {
		opt(options)
	}

	validStateTransitions := buildStateTransitions(options.RequiresInfraUpgrade)

	return statemachine.New(options.CurrentState, validStateTransitions, statemachine.WithLogger(options.Logger))
}
