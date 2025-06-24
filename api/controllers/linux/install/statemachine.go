package install

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/sirupsen/logrus"
)

const (
	// StateNew is the initial state of the install process
	StateNew statemachine.State = "New"
	// StateInstallationConfigured is the state of the install process when the installation is configured
	StateInstallationConfigured statemachine.State = "InstallationConfigured"
	// StateHostConfigured is the state of the install process when the host is configured
	StateHostConfigured statemachine.State = "HostConfigured"
	// StatePreflightsRunning is the state of the install process when the preflights are running
	StatePreflightsRunning statemachine.State = "PreflightsRunning"
	// StatePreflightsSucceeded is the state of the install process when the preflights have succeeded
	StatePreflightsSucceeded statemachine.State = "PreflightsSucceeded"
	// StatePreflightsFailed is the state of the install process when the preflights have failed
	StatePreflightsFailed statemachine.State = "PreflightsFailed"
	// StatePreflightsFailedBypassed is the state of the install process when the preflights have failed bypassed
	StatePreflightsFailedBypassed statemachine.State = "PreflightsFailedBypassed"
	// StateInfrastructureInstalling is the state of the install process when the infrastructure is being installed
	StateInfrastructureInstalling statemachine.State = "InfrastructureInstalling"
	// StateSucceeded is the final state of the install process when the install has succeeded
	StateSucceeded statemachine.State = "Succeeded"
	// StateFailed is the final state of the install process when the install has failed
	StateFailed statemachine.State = "Failed"
)

var validStateTransitions = map[statemachine.State][]statemachine.State{
	StateNew:                      {StateInstallationConfigured},
	StateInstallationConfigured:   {StateHostConfigured, StateInstallationConfigured},
	StateHostConfigured:           {StatePreflightsRunning, StateInstallationConfigured},
	StatePreflightsRunning:        {StatePreflightsSucceeded, StatePreflightsFailed},
	StatePreflightsSucceeded:      {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured},
	StatePreflightsFailed:         {StatePreflightsFailedBypassed, StatePreflightsRunning, StateInstallationConfigured},
	StatePreflightsFailedBypassed: {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured},
	StateInfrastructureInstalling: {StateSucceeded, StateFailed},
	StateSucceeded:                {},
	StateFailed:                   {},
}

type StateMachineOptions struct {
	CurrentState statemachine.State
	Logger       logrus.FieldLogger
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

// NewStateMachine creates a new state machine starting in the New state
func NewStateMachine(opts ...StateMachineOption) statemachine.Interface {
	options := &StateMachineOptions{
		CurrentState: StateNew,
	}
	for _, opt := range opts {
		opt(options)
	}
	return statemachine.New(options.CurrentState, validStateTransitions, statemachine.WithLogger(options.Logger))
}
