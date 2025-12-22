package upgrade

import (
	"maps"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

// Base state transitions that are common to all upgrade flows
var baseStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateNew:                            {states.StateApplicationConfiguring},
	states.StateApplicationConfiguring:         {states.StateApplicationConfigured, states.StateApplicationConfigurationFailed},
	states.StateApplicationConfigurationFailed: {states.StateApplicationConfiguring},
	states.StateApplicationConfigured:          {states.StateApplicationConfiguring}, // Will add more based on airgap/infra requirements
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

// Airgap-specific state transitions
var airgapStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateAirgapProcessing:       {states.StateAirgapProcessed, states.StateAirgapProcessingFailed},
	states.StateAirgapProcessed:        {}, // Will add transitions based on infra requirement
	states.StateAirgapProcessingFailed: {},
}

// Infrastructure-specific state transitions (includes host preflights that run before infra upgrade)
var infraStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateHostPreflightsRunning:         {states.StateHostPreflightsSucceeded, states.StateHostPreflightsFailed, states.StateHostPreflightsExecutionFailed},
	states.StateHostPreflightsExecutionFailed: {states.StateHostPreflightsRunning},
	states.StateHostPreflightsSucceeded:       {states.StateHostPreflightsRunning, states.StateInfrastructureUpgrading},
	states.StateHostPreflightsFailed:          {states.StateHostPreflightsRunning, states.StateHostPreflightsFailedBypassed},
	states.StateHostPreflightsFailedBypassed:  {states.StateHostPreflightsRunning, states.StateInfrastructureUpgrading},
	states.StateInfrastructureUpgrading:       {states.StateInfrastructureUpgraded, states.StateInfrastructureUpgradeFailed},
	states.StateInfrastructureUpgraded:        {states.StateAppPreflightsRunning},
	// final states
	states.StateInfrastructureUpgradeFailed: {},
}

// Build state transitions based on whether infra upgrade is required and whether airgap is present
func buildStateTransitions(requiresInfraUpgrade bool, isAirgap bool) map[statemachine.State][]statemachine.State {
	transitions := make(map[statemachine.State][]statemachine.State)

	// Copy base transitions
	maps.Copy(transitions, baseStateTransitions)

	// Add infrastructure-specific transitions if needed (includes host preflights)
	if requiresInfraUpgrade {
		maps.Copy(transitions, infraStateTransitions)
	}

	// Add airgap-specific transitions if needed
	if isAirgap {
		maps.Copy(transitions, airgapStateTransitions)

		// Add airgap processing as next state from ApplicationConfigured
		transitions[states.StateApplicationConfigured] = append(
			transitions[states.StateApplicationConfigured],
			states.StateAirgapProcessing,
		)

		// Add next state from AirgapProcessed based on infra requirement
		if requiresInfraUpgrade {
			transitions[states.StateAirgapProcessed] = []statemachine.State{states.StateHostPreflightsRunning}
		} else {
			transitions[states.StateAirgapProcessed] = []statemachine.State{states.StateAppPreflightsRunning}
		}
	} else {
		// No airgap, add transitions based on infra requirement
		if requiresInfraUpgrade {
			transitions[states.StateApplicationConfigured] = append(
				transitions[states.StateApplicationConfigured],
				states.StateHostPreflightsRunning,
			)
		} else {
			transitions[states.StateApplicationConfigured] = append(
				transitions[states.StateApplicationConfigured],
				states.StateAppPreflightsRunning,
			)
		}
	}

	return transitions
}

type StateMachineOptions struct {
	CurrentState         statemachine.State
	Logger               logrus.FieldLogger
	RequiresInfraUpgrade bool
	IsAirgap             bool
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

func WithIsAirgap(isAirgap bool) StateMachineOption {
	return func(o *StateMachineOptions) {
		o.IsAirgap = isAirgap
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

	validStateTransitions := buildStateTransitions(options.RequiresInfraUpgrade, options.IsAirgap)

	return statemachine.New(options.CurrentState, validStateTransitions, statemachine.WithLogger(options.Logger))
}
