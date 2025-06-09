package install

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error
	GetInstallationStatus(ctx context.Context) (*types.Status, error)
	RunHostPreflights(ctx context.Context) error
	GetHostPreflightStatus(ctx context.Context) (*types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetupNode(ctx context.Context) error
	SetStatus(ctx context.Context, status *types.Status) error
	GetStatus(ctx context.Context) (*types.Status, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	install              *types.Install
	installationManager  installation.InstallationManager
	hostPreflightManager preflight.HostPreflightManager
	rc                   runtimeconfig.RuntimeConfig
	logger               logrus.FieldLogger
	hostUtils            hostutils.HostUtilsInterface
	metricsReporter      metrics.ReporterInterface
	releaseData          *release.ReleaseData
	licenseFile          string
	airgapBundle         string
	mu                   sync.RWMutex
}

type InstallControllerOption func(*InstallController)

func WithRuntimeConfig(rc runtimeconfig.RuntimeConfig) InstallControllerOption {
	return func(c *InstallController) {
		c.rc = rc
	}
}

func WithLogger(logger logrus.FieldLogger) InstallControllerOption {
	return func(c *InstallController) {
		c.logger = logger
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) InstallControllerOption {
	return func(c *InstallController) {
		c.hostUtils = hostUtils
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) InstallControllerOption {
	return func(c *InstallController) {
		c.metricsReporter = metricsReporter
	}
}

func WithReleaseData(releaseData *release.ReleaseData) InstallControllerOption {
	return func(c *InstallController) {
		c.releaseData = releaseData
	}
}

func WithLicenseFile(licenseFile string) InstallControllerOption {
	return func(c *InstallController) {
		c.licenseFile = licenseFile
	}
}

func WithAirgapBundle(airgapBundle string) InstallControllerOption {
	return func(c *InstallController) {
		c.airgapBundle = airgapBundle
	}
}

func WithInstallationManager(installationManager installation.InstallationManager) InstallControllerOption {
	return func(c *InstallController) {
		c.installationManager = installationManager
	}
}

func WithHostPreflightManager(hostPreflightManager preflight.HostPreflightManager) InstallControllerOption {
	return func(c *InstallController) {
		c.hostPreflightManager = hostPreflightManager
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{
		install: types.NewInstall(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.rc == nil {
		controller.rc = runtimeconfig.New(nil)
	}

	if controller.logger == nil {
		controller.logger = logger.NewDiscardLogger()
	}

	if controller.hostUtils == nil {
		controller.hostUtils = hostutils.New(
			hostutils.WithLogger(controller.logger),
		)
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithRuntimeConfig(controller.rc),
			installation.WithLogger(controller.logger),
			installation.WithInstallation(controller.install.Steps.Installation),
			installation.WithLicenseFile(controller.licenseFile),
			installation.WithAirgapBundle(controller.airgapBundle),
			installation.WithHostUtils(controller.hostUtils),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithRuntimeConfig(controller.rc),
			preflight.WithLogger(controller.logger),
			preflight.WithMetricsReporter(controller.metricsReporter),
			preflight.WithHostPreflight(controller.install.Steps.HostPreflight),
		)
	}

	return controller, nil
}
