package install

import (
	"context"
	"errors"
	"fmt"
	"time"

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
// The apiClient must be authenticated before calling this function.
func NewOrchestrator(ctx context.Context, apiClient client.Client, target string, opts ...OrchestratorOption) (Orchestrator, error) {
	// We do not yet support the "kubernetes" target
	installTarget := apitypes.InstallTarget(target)
	if installTarget != apitypes.InstallTargetLinux {
		return nil, fmt.Errorf("%s target not supported", target)
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

	// Configure installation settings
	if err := o.configureInstallation(ctx, opts); err != nil {
		return false, err // Can retry without reset
	}

	// Run host preflights (allow bypass when --ignore-host-preflights flag is set)
	if err := o.runHostPreflights(ctx, opts.IgnoreHostPreflights); err != nil {
		return false, err // Can retry without reset
	}

	// Setup infrastructure (POINT OF NO RETURN)
	// After this point, any failure requires running 'embedded-cluster reset'
	if err := o.setupInfrastructure(ctx, opts.IgnoreHostPreflights); err != nil {
		return true, err // Reset required
	}

	// Process airgap if needed
	if opts.AirgapBundle != "" {
		if err := o.processAirgap(ctx); err != nil {
			return true, err // Reset required
		}
	}

	// Run app preflights (allow bypass when --ignore-app-preflights flag is set)
	if err := o.runAppPreflights(ctx, opts.IgnoreAppPreflights); err != nil {
		return true, err // Reset required
	}

	// Install application
	if err := o.installApp(ctx, opts.IgnoreAppPreflights); err != nil {
		return true, err // Reset required
	}

	return false, nil // Success
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

// configureInstallation configures the installation settings by submitting the LinuxInstallationConfig to the API.
// It validates the provided installation configuration and handles any validation errors.
// This step can be retried without requiring a system reset.
func (o *orchestrator) configureInstallation(ctx context.Context, opts HeadlessInstallOptions) error {
	o.logger.Debug("Starting installation configuration")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Configuring installation settings...")

	// Configure installation settings via API
	status, err := o.apiClient.ConfigureLinuxInstallation(ctx, opts.LinuxInstallationConfig)
	if err != nil {
		loading.ErrorClosef("Installation configuration failed")

		// Check if it's an APIError with field details
		var apiErr *apitypes.APIError
		if errors.As(err, &apiErr) && len(apiErr.Errors) > 0 {
			// Format and display the structured error
			formattedErr := formatAPIError(apiErr)
			return fmt.Errorf("installation configuration validation failed: %s", formattedErr)
		}

		return fmt.Errorf("configure linux installation: %w", err)
	}

	// Check if configuration failed immediately
	if status.State == apitypes.StateFailed {
		loading.ErrorClosef("Installation configuration failed")
		return fmt.Errorf("installation configuration failed: %s", status.Description)
	}

	// Poll for host configuration to complete
	// ConfigureLinuxInstallation spawns a background goroutine that configures the host,
	// so we need to wait for it to complete before moving to the next step
	getStatus := func() (apitypes.State, string, error) {
		status, err := o.apiClient.GetLinuxInstallationStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return status.State, status.Description, nil
	}

	state, message, err := pollUntilComplete(ctx, getStatus)
	if err != nil {
		loading.ErrorClosef("Installation configuration failed")
		return fmt.Errorf("poll until complete: %w", err)
	}

	if state == apitypes.StateSucceeded {
		loading.Closef("Installation configuration complete")
		o.logger.Debug("Installation configuration complete")
		return nil
	}

	loading.ErrorClosef("Installation configuration failed")
	return fmt.Errorf("installation configuration failed: %s", message)
}

// runHostPreflights executes host preflight checks and polls until completion.
// If ignoreFailures is true, the installation will continue even if checks fail.
// This step can be retried without requiring a system reset.
func (o *orchestrator) runHostPreflights(ctx context.Context, ignoreFailures bool) error {
	o.logger.Debug("Starting host preflights")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Running host preflights...")

	// Trigger preflights
	resp, err := o.apiClient.RunLinuxInstallHostPreflights(ctx)
	if err != nil {
		loading.ErrorClosef("Host preflights failed")
		return fmt.Errorf("run linux install host preflights: %w", err)
	}

	_, _, err = pollUntilComplete(ctx, func() (apitypes.State, string, error) {
		resp, err = o.apiClient.GetLinuxInstallHostPreflightsStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return resp.Status.State, resp.Status.Description, nil
	})
	if err != nil {
		loading.ErrorClosef("Host preflights failed")
		return fmt.Errorf("poll preflights until complete: %w", err)
	}

	if resp.Status.State == apitypes.StateFailed {
		// Check if there are any failures in the preflight results
		hasFailures := resp.Output != nil && resp.Output.HasFail()
		if hasFailures {
			loading.ErrorClosef("Host preflights completed with failures")

			o.logger.Warn("\n⚠ Warning: Host preflight checks completed with failures\n")

			// Display failed checks
			for _, result := range resp.Output.Fail {
				o.logger.Warnf("  [ERROR] %s: %s", result.Title, result.Message)
			}
			for _, result := range resp.Output.Warn {
				o.logger.Warnf("  [WARN] %s: %s", result.Title, result.Message)
			}

			if ignoreFailures {
				// Display failures but continue installation
				o.logger.Warn("\nInstallation will continue, but the system may not meet requirements (failures bypassed with flag).\n")
			} else {
				// Failures are not being bypassed - return error
				o.logger.Warn("\nPlease correct the above issues and retry, or run with --ignore-host-preflights to bypass (not recommended).\n")
				return fmt.Errorf("host preflight checks completed with failures")
			}
		} else {
			// Otherwise, we assume preflight execution failed
			loading.ErrorClosef("Host preflights execution failed")
			errMsg := resp.Status.Description
			if errMsg == "" {
				errMsg = "host preflights execution failed"
			}
			return errors.New(errMsg)
		}
	} else {
		loading.Closef("Host preflights passed")
		o.logger.Debug("Host preflights passed")
	}

	return nil
}

// setupInfrastructure sets up the Kubernetes infrastructure (K0s and addons).
// This is the POINT OF NO RETURN - after this step, failures require a full reset.
func (o *orchestrator) setupInfrastructure(ctx context.Context, ignoreHostPreflights bool) error {
	o.logger.Debug("Starting infrastructure setup")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Setting up infrastructure...")

	// Initiate infra setup using api/client.Client
	infra, err := o.apiClient.SetupLinuxInfra(ctx, ignoreHostPreflights)
	if err != nil {
		loading.ErrorClosef("Infrastructure setup failed")
		return fmt.Errorf("setup linux infra: %w", err)
	}

	// Poll for completion
	getStatus := func() (apitypes.State, string, error) {
		infra, err = o.apiClient.GetLinuxInfraStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return infra.Status.State, infra.Status.Description, nil
	}

	state, message, err := pollUntilComplete(ctx, getStatus)
	if err != nil {
		loading.ErrorClosef("Infrastructure setup failed")
		return fmt.Errorf("poll until complete: %w", err)
	}

	if state == apitypes.StateSucceeded {
		loading.Closef("Infrastructure setup complete")
		o.logger.Debug("Infrastructure setup complete")
		return nil
	}

	loading.ErrorClosef("Infrastructure setup failed")
	return fmt.Errorf("infrastructure setup failed: %s", message)
}

// processAirgap processes the airgap bundle and polls until completion.
// This step requires a reset if it fails.
func (o *orchestrator) processAirgap(ctx context.Context) error {
	o.logger.Debug("Starting airgap bundle processing")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Processing airgap bundle...")

	// Initiate airgap processing
	airgap, err := o.apiClient.ProcessLinuxAirgap(ctx)
	if err != nil {
		loading.ErrorClosef("Airgap processing failed")
		return fmt.Errorf("process linux airgap: %w", err)
	}

	// Poll for completion
	getStatus := func() (apitypes.State, string, error) {
		airgap, err = o.apiClient.GetLinuxAirgapStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return airgap.Status.State, airgap.Status.Description, nil
	}

	state, message, err := pollUntilComplete(ctx, getStatus)
	if err != nil {
		loading.ErrorClosef("Airgap processing failed")
		return fmt.Errorf("poll until complete: %w", err)
	}

	if state == apitypes.StateSucceeded {
		loading.Closef("Airgap processing complete")
		o.logger.Debug("Airgap processing complete")
		return nil
	}

	loading.ErrorClosef("Airgap processing failed")
	return fmt.Errorf("airgap processing failed: %s", message)
}

// runAppPreflights executes application preflight checks and polls until completion.
// If ignoreFailures is true, the installation will continue even if checks fail.
// This step requires a reset if it fails (when not bypassed).
func (o *orchestrator) runAppPreflights(ctx context.Context, ignoreFailures bool) error {
	o.logger.Debug("Starting app preflights")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Running app preflights...")

	// Trigger preflights
	resp, err := o.apiClient.RunLinuxInstallAppPreflights(ctx)
	if err != nil {
		loading.ErrorClosef("App preflights failed")
		return fmt.Errorf("run linux install app preflights: %w", err)
	}

	// Poll for completion
	// For preflights, we poll until the operation completes (either succeeded or failed),
	// then check if there are failures and decide whether to continue based on ignoreFailures
	_, _, err = pollUntilComplete(ctx, func() (apitypes.State, string, error) {
		resp, err = o.apiClient.GetLinuxInstallAppPreflightsStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return resp.Status.State, resp.Status.Description, nil
	})
	if err != nil {
		loading.ErrorClosef("App preflights failed")
		return fmt.Errorf("poll preflights until complete: %w", err)
	}

	if resp.Status.State == apitypes.StateFailed {
		// Check if there are any failures in the preflight results
		hasFailures := resp.Output != nil && resp.Output.HasFail()
		if hasFailures {
			loading.ErrorClosef("App preflights completed with failures")

			o.logger.Warn("\n⚠ Warning: Application preflight checks completed with failures\n")

			// Display failed checks
			for _, result := range resp.Output.Fail {
				o.logger.Warnf("  [ERROR] %s: %s", result.Title, result.Message)
			}
			for _, result := range resp.Output.Warn {
				o.logger.Warnf("  [WARN] %s: %s", result.Title, result.Message)
			}

			if ignoreFailures {
				// Display failures but continue installation
				o.logger.Warn("\nInstallation will continue, but the application may not function correctly (failures bypassed with flag).\n")
			} else {
				// Failures are not being bypassed - return error
				o.logger.Warn("\nPlease correct the above issues and retry, or run with --ignore-app-preflights to bypass (not recommended).\n")
				return fmt.Errorf("app preflight checks completed with failures")
			}
		} else {
			// Otherwise, we assume preflight execution failed
			loading.ErrorClosef("App preflights execution failed")
			errMsg := resp.Status.Description
			if errMsg == "" {
				errMsg = "app preflights execution failed"
			}
			return errors.New(errMsg)
		}
	} else {
		loading.Closef("App preflights passed")
		o.logger.Debug("App preflights passed")
	}

	return nil
}

// installApp installs the application and polls until completion.
// This step requires a reset if it fails.
func (o *orchestrator) installApp(ctx context.Context, ignoreAppPreflights bool) error {
	o.logger.Debug("Starting application installation")

	loading := spinner.Start(spinner.WithWriter(o.progressWriter))
	loading.Infof("Installing application...")

	// Initiate app installation
	appInstall, err := o.apiClient.InstallLinuxApp(ctx, ignoreAppPreflights)
	if err != nil {
		loading.ErrorClosef("Application installation failed")
		return fmt.Errorf("install linux app: %w", err)
	}

	// Poll for completion
	getStatus := func() (apitypes.State, string, error) {
		appInstall, err = o.apiClient.GetLinuxAppInstallStatus(ctx)
		if err != nil {
			return apitypes.State(""), "", err
		}
		return appInstall.Status.State, appInstall.Status.Description, nil
	}

	state, message, err := pollUntilComplete(ctx, getStatus)
	if err != nil {
		loading.ErrorClosef("Application installation failed")
		return fmt.Errorf("poll until complete: %w", err)
	}

	if state == apitypes.StateSucceeded {
		loading.Closef("Application is ready")
		o.logger.Debug("Application installation complete")
		return nil
	}

	loading.ErrorClosef("Application installation failed")
	return fmt.Errorf("application installation failed: %s", message)
}

// pollUntilComplete polls a status endpoint until the operation reaches a terminal state.
// It retries getStatus() up to 3 times on transient errors before failing.
// The getStatus function should return (State, Description, error).
func pollUntilComplete(ctx context.Context, getStatus func() (apitypes.State, string, error)) (apitypes.State, string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-ticker.C:
			// Retry getStatus() up to 3 times on conflict errors (api is still transitioning)
			var state apitypes.State
			var message string
			var err error

			attempt := 0
			for timer := time.NewTimer(0); attempt < 3; timer.Reset(1 * time.Second) {
				<-timer.C

				state, message, err = getStatus()
				if !apitypes.IsConflictError(err) {
					break
				}
				attempt++
			}

			// If still erroring after 3 attempts, fail
			if err != nil {
				return "", "", fmt.Errorf("get status failed after 3 attempts: %w", err)
			}

			// Check for terminal states
			switch state {
			case apitypes.StateSucceeded, apitypes.StateFailed:
				return state, message, nil
			case apitypes.StatePending, apitypes.StateRunning,
				// "" is possible if the status is not yet set, infer pending
				"":
				continue
			default:
				return "", "", fmt.Errorf("unknown state: %s", state)
			}
		}
	}
}
