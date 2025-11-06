package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
)

// HeadlessInstallOptions contains the configuration options for a headless installation
type HeadlessInstallOptions struct {
	// ConfigValues are the application config values to use for installation
	ConfigValues apitypes.AppConfigValues

	// LinuxInstallationConfig contains the installation settings for the Linux target
	LinuxInstallationConfig apitypes.LinuxInstallationConfig

	// IgnoreHostPreflights indicates whether to bypass host preflight check failures
	IgnoreHostPreflights bool

	// IgnoreAppPreflights indicates whether to bypass app preflight check failures
	IgnoreAppPreflights bool

	// AirgapBundle is the path to the airgap bundle file (empty string for online installs)
	AirgapBundle string
}

// Orchestrator defines the interface for headless installation operations.
// It orchestrates the installation process by interacting with the v3 API server
// running in-process via HTTP calls to localhost.
type Orchestrator interface {
	// RunHeadlessInstall executes a complete headless installation workflow.
	// It performs the following steps in order:
	//   1. Configure application with config values
	//   2. Configure installation settings
	//   3. Run host preflights (with optional bypass)
	//   4. Setup infrastructure
	//   5. Process airgap bundle (if provided)
	//   6. Run app preflights (with optional bypass)
	//   7. Install application
	//
	// The installation cannot be resumed if it fails after infrastructure setup begins.
	// Any failure after that point requires running 'embedded-cluster reset' and retrying.
	//
	// Returns:
	//   - resetNeeded: true if the failure requires running 'embedded-cluster reset' before retrying
	//   - err: the error that occurred, or nil on success
	RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) (resetNeeded bool, err error)
}

// orchestrator is the concrete implementation of the Orchestrator interface
type orchestrator struct {
	// apiClient is the HTTP client for communicating with the v3 API server
	apiClient client.Client
	// target specifies the installation target platform ("linux" or "kubernetes")
	target apitypes.InstallTarget
	// progressWriter is the output function for user-visible progress messages
	progressWriter spinner.WriteFn
	// logger is used for detailed debug and diagnostic logging
	logger logrus.FieldLogger
}

// OrchestratorOption is a functional option for configuring the orchestrator
type OrchestratorOption func(*orchestrator)

// WithProgressWriter sets a custom progress writer for the orchestrator
func WithProgressWriter(writer spinner.WriteFn) OrchestratorOption {
	return func(o *orchestrator) {
		o.progressWriter = writer
	}
}

// WithLogger sets a custom logger for the orchestrator
func WithLogger(logger logrus.FieldLogger) OrchestratorOption {
	return func(o *orchestrator) {
		o.logger = logger
	}
}

// NewOrchestrator creates a new Orchestrator instance.
// It authenticates with the API server using the provided password.
func NewOrchestrator(ctx context.Context, apiClient client.Client, password string, target string, opts ...OrchestratorOption) (Orchestrator, error) {
	// We do not yet support the "kubernetes" target
	installTarget := apitypes.InstallTarget(target)
	if installTarget != apitypes.InstallTargetLinux {
		return nil, fmt.Errorf("%s target not supported", target)
	}

	// Authenticate and set token
	if err := apiClient.Authenticate(ctx, password); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	o := &orchestrator{
		apiClient:      apiClient,
		target:         installTarget,
		progressWriter: fmt.Printf,
		logger:         logrus.StandardLogger(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return o, nil
}

// RunHeadlessInstall executes a complete headless installation workflow
func (o *orchestrator) RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) (bool, error) {
	// Configure application with config values
	if err := o.configureApplication(ctx, opts); err != nil {
		return false, err // Can retry without reset
	}

	// TODO: Implement remaining steps
	return false, fmt.Errorf("headless installation is not yet fully implemented - coming in a future release")
}

// configureApplication configures the application by submitting config values to the API.
// It validates the provided config values, patches them via the API, and handles any
// validation errors by formatting them for user display. This is the first step in the
// headless installation workflow and can be retried without requiring a system reset.
func (o *orchestrator) configureApplication(ctx context.Context, opts HeadlessInstallOptions) error {
	o.logger.Debug("Starting application configuration")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Configuring application...")

	// Use the wrapped api/client.Client to patch config values
	_, err := o.apiClient.PatchLinuxInstallAppConfigValues(ctx, opts.ConfigValues)
	if err != nil {
		loading.ErrorClosef("Application configuration failed")

		// Check if it's an APIError with field details
		var apiErr *apitypes.APIError
		if errors.As(err, &apiErr) && len(apiErr.Errors) > 0 {
			// Format and display the structured error
			formattedErr := formatAPIError(apiErr)
			return fmt.Errorf("application configuration validation failed: %s", formattedErr)
		}

		return fmt.Errorf("patch app config values: %w", err)
	}

	loading.Closef("Application configuration complete")
	o.logger.Debug("Application configuration complete")
	return nil
}
