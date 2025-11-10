package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/cli/headless/install"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

// runV3InstallHeadless executes a headless installation workflow using the orchestrator
func runV3InstallHeadless(
	ctx context.Context,
	cancel context.CancelFunc,
	flags installFlags,
	apiOpts apiOptions,
	metricsReporter metrics.ReporterInterface,
) error {
	// Setup signal handler
	signalHandler(ctx, cancel, func(ctx context.Context, sig os.Signal) {
		metricsReporter.ReportSignalAborted(ctx, sig)
	})

	// Build API client
	apiClient, err := newOrchestratorAPIClient(ctx, flags, apiOpts)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Build orchestrator
	orchestrator, err := buildOrchestrator(ctx, apiClient, apiOpts)
	if err != nil {
		return fmt.Errorf("failed to build orchestrator: %w", err)
	}

	// Build install options
	opts := buildHeadlessInstallOptions(flags, apiOpts)

	resetNeeded, err := orchestrator.RunHeadlessInstall(ctx, opts)
	if err != nil {
		if errors.Is(err, terminal.InterruptErr) {
			metricsReporter.ReportSignalAborted(ctx, syscall.SIGINT)
		} else {
			metricsReporter.ReportInstallationFailed(ctx, err)
		}

		displayInstallErrorAndRecoveryInstructions(err, resetNeeded, runtimeconfig.AppSlug(), logrus.StandardLogger())

		return NewErrorNothingElseToAdd(err)
	}

	// Display success message
	logrus.Info("\nInstallation completed successfully")

	// API event handlers will report installation succeeded

	return nil
}

// newOrchestratorAPIClient creates a new API client for the orchestrator.
func newOrchestratorAPIClient(
	ctx context.Context,
	flags installFlags,
	apiOpts apiOptions,
) (client.Client, error) {
	// Construct API URL from manager port
	apiURL := fmt.Sprintf("https://localhost:%d", flags.managerPort)

	// Create HTTP client with InsecureSkipVerify for localhost
	// Since the API server is in-process and on localhost only, certificate
	// validation is not critical for this use case
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: nil, // No proxy for localhost
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // Acceptable for localhost in-process API
			},
		},
	}

	apiClient := client.New(
		apiURL, // e.g., "https://localhost:30000"
		client.WithHTTPClient(httpClient),
	)

	// Authenticate with the API server
	if err := apiClient.Authenticate(ctx, apiOpts.Password); err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	return apiClient, nil
}

// buildOrchestrator (Hop) creates an orchestrator from CLI inputs.
func buildOrchestrator(
	ctx context.Context,
	apiClient client.Client,
	apiOpts apiOptions,
) (install.Orchestrator, error) {
	// We do not yet support the "kubernetes" target
	if apiOpts.InstallTarget != apitypes.InstallTargetLinux {
		return nil, fmt.Errorf("%s target not supported", apiOpts.InstallTarget)
	}

	// Create orchestrator with authenticated client
	orchestrator, err := install.NewOrchestrator(
		ctx,
		apiClient,
		string(apiOpts.InstallTarget),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}

	return orchestrator, nil
}

// buildHeadlessInstallOptions (Hop) creates HeadlessInstallOptions from CLI inputs.
func buildHeadlessInstallOptions(
	flags installFlags,
	apiOpts apiOptions,
) install.HeadlessInstallOptions {
	// Build Linux installation config from flags
	linuxInstallationConfig := apitypes.LinuxInstallationConfig{
		AdminConsolePort:        flags.adminConsolePort,
		DataDirectory:           flags.dataDir,
		LocalArtifactMirrorPort: flags.localArtifactMirrorPort,
		HTTPProxy:               "",
		HTTPSProxy:              "",
		NoProxy:                 "",
		NetworkInterface:        flags.networkInterface,
		PodCIDR:                 "",
		ServiceCIDR:             "",
		GlobalCIDR:              "",
	}

	// Set proxy values from flags.proxySpec if present
	if flags.proxySpec != nil {
		linuxInstallationConfig.HTTPProxy = flags.proxySpec.HTTPProxy
		linuxInstallationConfig.HTTPSProxy = flags.proxySpec.HTTPSProxy
		linuxInstallationConfig.NoProxy = flags.proxySpec.NoProxy
	}

	// Set CIDR values from flags.cidrConfig if present
	if flags.cidrConfig != nil {
		linuxInstallationConfig.PodCIDR = flags.cidrConfig.PodCIDR
		linuxInstallationConfig.ServiceCIDR = flags.cidrConfig.ServiceCIDR
		if flags.cidrConfig.GlobalCIDR != nil {
			linuxInstallationConfig.GlobalCIDR = *flags.cidrConfig.GlobalCIDR
		}
	}

	return install.HeadlessInstallOptions{
		ConfigValues:            apiOpts.ConfigValues,
		LinuxInstallationConfig: linuxInstallationConfig,
		IgnoreHostPreflights:    flags.ignoreHostPreflights,
		IgnoreAppPreflights:     flags.ignoreAppPreflights,
		AirgapBundle:            flags.airgapBundle,
	}
}

// displayInstallErrorAndRecoveryInstructions (Hop) displays the error and recovery instructions to the user.
func displayInstallErrorAndRecoveryInstructions(err error, resetNeeded bool, binaryName string, logger logrus.FieldLogger) {
	logger.Errorf("\nError: %v\n", err)

	if resetNeeded {
		logger.Infof("To collect diagnostic information, run: %s support-bundle", binaryName)
		logger.Infof("To retry installation, run: %s reset and wait for server reboot", binaryName)
	} else {
		logger.Infof("For configuration options, run: %s install --help", binaryName)
		logger.Infof("Please correct the above issues and retry")
	}
}
