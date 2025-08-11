package states

import "github.com/replicatedhq/embedded-cluster/api/internal/statemachine"

const (
	// StateNew is the initial state of the install process
	StateNew statemachine.State = "New"
	// StateApplicationConfiguring is the state of the install process when the application is being configured
	StateApplicationConfiguring statemachine.State = "ApplicationConfiguring"
	// StateApplicationConfigurationFailed is the state of the install process when the application failed to be configured
	StateApplicationConfigurationFailed statemachine.State = "ApplicationConfigurationFailed"
	// StateApplicationConfigured is the state of the install process when the application is configured
	StateApplicationConfigured statemachine.State = "ApplicationConfigured"
	// StateInstallationConfiguring is the state of the install process when the installation is being configured
	StateInstallationConfiguring statemachine.State = "InstallationConfiguring"
	// StateInstallationConfigurationFailed is the state of the install process when the installation failed to be configured
	StateInstallationConfigurationFailed statemachine.State = "InstallationConfigurationFailed"
	// StateInstallationConfigured is the state of the install process when the installation is configured
	StateInstallationConfigured statemachine.State = "InstallationConfigured"
	// StateHostConfiguring is the state of the install process when the host is being configured
	StateHostConfiguring statemachine.State = "HostConfiguring"
	// StateHostConfigurationFailed is the state of the install process when the installation failed to be configured
	StateHostConfigurationFailed statemachine.State = "HostConfigurationFailed"
	// StateHostConfigured is the state of the install process when the host is configured
	StateHostConfigured statemachine.State = "HostConfigured"
	// StateHostPreflightsRunning is the state of the install process when the preflights are running
	StateHostPreflightsRunning statemachine.State = "HostPreflightsRunning"
	// StateHostPreflightsExecutionFailed is the state of the install process when the preflights failed to execute due to an underlying system error
	StateHostPreflightsExecutionFailed statemachine.State = "HostPreflightsExecutionFailed"
	// StateHostPreflightsSucceeded is the state of the install process when the preflights have succeeded
	StateHostPreflightsSucceeded statemachine.State = "HostPreflightsSucceeded"
	// StateHostPreflightsFailed is the state of the install process when the preflights execution succeeded but the preflights detected issues on the host
	StateHostPreflightsFailed statemachine.State = "HostPreflightsFailed"
	// StateHostPreflightsFailedBypassed is the state of the install process when, despite preflights failing, the user has chosen to bypass the preflights and continue with the installation
	StateHostPreflightsFailedBypassed statemachine.State = "HostPreflightsFailedBypassed"
	// StateInfrastructureInstalling is the state of the install process when the infrastructure is being installed
	StateInfrastructureInstalling statemachine.State = "InfrastructureInstalling"
	// StateInfrastructureInstallFailed is a final state of the install process when the infrastructure failed to isntall
	StateInfrastructureInstallFailed statemachine.State = "InfrastructureInstallFailed"
	// StateInfrastructureInstalled is the state of the install process when the infrastructure install has succeeded
	StateInfrastructureInstalled statemachine.State = "InfrastructureInstalled"
	// StateAppPreflightsRunning is the state of the install process when the preflights are running
	StateAppPreflightsRunning statemachine.State = "AppPreflightsRunning"
	// StateAppPreflightsExecutionFailed is the state of the install process when the preflights failed to execute due to an underlying system error
	StateAppPreflightsExecutionFailed statemachine.State = "AppPreflightsExecutionFailed"
	// StateAppPreflightsSucceeded is the state of the install process when the preflights have succeeded
	StateAppPreflightsSucceeded statemachine.State = "AppPreflightsSucceeded"
	// StateAppPreflightsFailed is the state of the install process when the preflights execution succeeded but the preflights detected issues on the host
	StateAppPreflightsFailed statemachine.State = "AppPreflightsFailed"
	// StateAppPreflightsFailedBypassed is the state of the install process when, despite preflights failing, the user has chosen to bypass the preflights and continue with the installation
	StateAppPreflightsFailedBypassed statemachine.State = "AppPreflightsFailedBypassed"
	// StateAppInstalling is the state of the install process when the app is being installed
	StateAppInstalling statemachine.State = "AppInstalling"
	// StateAppInstallFailed is a final state of the install process when the app failed to install
	StateAppInstallFailed statemachine.State = "AppInstallFailed"
	// StateSucceeded is the final state of the install process when the install has succeeded
	StateSucceeded statemachine.State = "Succeeded"
)
