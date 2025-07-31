package install

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

var validStateTransitions = map[statemachine.State][]statemachine.State{
	states.StateNew: {states.StateApplicationConfiguring},
	states.StateApplicationConfigurationFailed:  {states.StateApplicationConfiguring},
	states.StateApplicationConfiguring:          {states.StateApplicationConfigured, states.StateApplicationConfigurationFailed},
	states.StateApplicationConfigured:           {states.StateApplicationConfiguring, states.StateInstallationConfiguring},
	states.StateInstallationConfiguring:         {states.StateInstallationConfigured, states.StateInstallationConfigurationFailed},
	states.StateInstallationConfigurationFailed: {states.StateApplicationConfiguring, states.StateInstallationConfiguring},
	states.StateInstallationConfigured:          {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring},
	states.StateHostConfiguring:                 {states.StateHostConfigured, states.StateHostConfigurationFailed},
	states.StateHostConfigurationFailed:         {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring},
	states.StateHostConfigured:                  {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring, states.StateHostPreflightsRunning},
	states.StateHostPreflightsRunning:           {states.StateHostPreflightsSucceeded, states.StateHostPreflightsFailed, states.StateHostPreflightsExecutionFailed},
	states.StateHostPreflightsExecutionFailed:   {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring, states.StateHostPreflightsRunning},
	states.StateHostPreflightsFailed:            {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring, states.StateHostPreflightsRunning, states.StateHostPreflightsFailedBypassed},
	states.StateHostPreflightsSucceeded:         {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring, states.StateHostPreflightsRunning, states.StateInfrastructureInstalling},
	states.StateHostPreflightsFailedBypassed:    {states.StateApplicationConfiguring, states.StateInstallationConfiguring, states.StateHostConfiguring, states.StateHostPreflightsRunning, states.StateInfrastructureInstalling},
	states.StateInfrastructureInstalling:        {states.StateSucceeded, states.StateInfrastructureInstallFailed},
	// final states
	states.StateInfrastructureInstallFailed: {},
	states.StateSucceeded:                   {},
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
		CurrentState: states.StateNew,
		Logger:       logger.NewDiscardLogger(),
	}
	for _, opt := range opts {
		opt(options)
	}
	return statemachine.New(options.CurrentState, validStateTransitions, statemachine.WithLogger(options.Logger))
}
