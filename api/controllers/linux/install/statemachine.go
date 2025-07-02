package install

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

const (
	// StateNew is the initial state of the install process
	StateNew statemachine.State = "New"
	// StateInstallationConfigurationFailed is the state of the install process when the installation failed to be configured
	StateInstallationConfigurationFailed statemachine.State = "InstallationConfigurationFailed"
	// StateInstallationConfigured is the state of the install process when the installation is configured
	StateInstallationConfigured statemachine.State = "InstallationConfigured"
	// StateHostConfigurationFailed is the state of the install process when the installation failed to be configured
	StateHostConfigurationFailed statemachine.State = "HostConfigurationFailed"
	// StateHostConfigured is the state of the install process when the host is configured
	StateHostConfigured statemachine.State = "HostConfigured"
	// StatePreflightsRunning is the state of the install process when the preflights are running
	StatePreflightsRunning statemachine.State = "PreflightsRunning"
	// StatePreflightsExecutionFailed is the state of the install process when the preflights failed to execute due to an underlying system error
	StatePreflightsExecutionFailed statemachine.State = "PreflightsExecutionFailed"
	// StatePreflightsSucceeded is the state of the install process when the preflights have succeeded
	StatePreflightsSucceeded statemachine.State = "PreflightsSucceeded"
	// StatePreflightsFailed is the state of the install process when the preflights execution succeeded but the preflights detected issues on the host
	StatePreflightsFailed statemachine.State = "PreflightsFailed"
	// StatePreflightsFailedBypassed is the state of the install process when, despite preflights failing, the user has chosen to bypass the preflights and continue with the installation
	StatePreflightsFailedBypassed statemachine.State = "PreflightsFailedBypassed"
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
	StateInstallationConfigured:          {StateInstallationConfigured, StateHostConfigured, StateHostConfigurationFailed},
	StateHostConfigurationFailed:         {StateInstallationConfigured, StateInstallationConfigurationFailed},
	StateHostConfigured:                  {StatePreflightsRunning, StateInstallationConfigured, StateInstallationConfigurationFailed},
	StatePreflightsRunning:               {StatePreflightsSucceeded, StatePreflightsFailed, StatePreflightsExecutionFailed},
	StatePreflightsExecutionFailed:       {StatePreflightsRunning, StateInstallationConfigured, StateInstallationConfigurationFailed},
	StatePreflightsSucceeded:             {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured, StateInstallationConfigurationFailed},
	StatePreflightsFailed:                {StatePreflightsFailedBypassed, StatePreflightsRunning, StateInstallationConfigured, StateInstallationConfigurationFailed},
	StatePreflightsFailedBypassed:        {StateInfrastructureInstalling, StatePreflightsRunning, StateInstallationConfigured, StateInstallationConfigurationFailed},
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
		Logger:       logger.NewDiscardLogger(),
	}
	for _, opt := range opts {
		opt(options)
	}
	return statemachine.New(options.CurrentState, validStateTransitions, statemachine.WithLogger(options.Logger))
}
