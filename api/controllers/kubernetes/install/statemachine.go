package install

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/sirupsen/logrus"
)

const (
	// StateNew is the initial state of the install process
	StateNew statemachine.State = "New"
	// StateInstallationConfigurationFailed is the state of the install process when the installation failed to be configured
	StateInstallationConfigurationFailed statemachine.State = "InstallationConfigurationFailed"
	// StateInstallationConfigured is the state of the install process when the installation is configured
	StateInstallationConfigured statemachine.State = "InstallationConfigured"
	// StateInfrastructureInstalling is the state of the install process when the infrastructure is being installed
	StateInfrastructureInstalling statemachine.State = "InfrastructureInstalling"
	// StateInfrastructureInstallFailed is a final state of the install process when the infrastructure failed to isntall
	StateInfrastructureInstallFailed statemachine.State = "InfrastructureInstallFailed"
	// StateSucceeded is the final state of the install process when the install has succeeded
	StateSucceeded statemachine.State = "Succeeded"
)

var validStateTransitions = map[statemachine.State][]statemachine.State{
	StateNew:                             {StateInstallationConfigured, StateInstallationConfigurationFailed},
	StateInstallationConfigurationFailed: {StateInstallationConfigured, StateInstallationConfigurationFailed},
	StateInstallationConfigured:          {StateInstallationConfigured, StateInfrastructureInstalling, StateInstallationConfigurationFailed},
	StateInfrastructureInstalling:        {StateSucceeded, StateInfrastructureInstallFailed},
	StateInfrastructureInstallFailed:     {},
	StateSucceeded:                       {},
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
	return statemachine.New(options.CurrentState, validStateTransitions)
}
