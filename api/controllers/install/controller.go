package install

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
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
	RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error
	GetHostPreflightStatus(ctx context.Context) (*types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetupInfra(ctx context.Context, ignorePreflightFailures bool) (bool, error)
	GetInfra(ctx context.Context) (*types.Infra, error)
	SetStatus(ctx context.Context, status *types.Status) error
	GetStatus(ctx context.Context) (*types.Status, error)
}

type RunHostPreflightsOptions struct {
	IsUI bool
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	install                   *types.Install
	installationManager       installation.InstallationManager
	hostPreflightManager      preflight.HostPreflightManager
	infraManager              infra.InfraManager
	rc                        runtimeconfig.RuntimeConfig
	logger                    logrus.FieldLogger
	hostUtils                 hostutils.HostUtilsInterface
	netUtils                  utils.NetUtils
	metricsReporter           metrics.ReporterInterface
	releaseData               *release.ReleaseData
	password                  string
	tlsConfig                 types.TLSConfig
	licenseFile               string
	airgapBundle              string
	configValues              string
	endUserConfig             *ecv1beta1.Config
	allowIgnoreHostPreflights bool
	mu                        sync.RWMutex
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

func WithNetUtils(netUtils utils.NetUtils) InstallControllerOption {
	return func(c *InstallController) {
		c.netUtils = netUtils
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

func WithPassword(password string) InstallControllerOption {
	return func(c *InstallController) {
		c.password = password
	}
}

func WithTLSConfig(tlsConfig types.TLSConfig) InstallControllerOption {
	return func(c *InstallController) {
		c.tlsConfig = tlsConfig
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

func WithConfigValues(configValues string) InstallControllerOption {
	return func(c *InstallController) {
		c.configValues = configValues
	}
}

func WithEndUserConfig(endUserConfig *ecv1beta1.Config) InstallControllerOption {
	return func(c *InstallController) {
		c.endUserConfig = endUserConfig
	}
}

func WithAllowIgnoreHostPreflights(allowIgnoreHostPreflights bool) InstallControllerOption {
	return func(c *InstallController) {
		c.allowIgnoreHostPreflights = allowIgnoreHostPreflights
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

	if controller.netUtils == nil {
		controller.netUtils = utils.NewNetUtils()
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithRuntimeConfig(controller.rc),
			installation.WithLogger(controller.logger),
			installation.WithInstallation(controller.install.Steps.Installation),
			installation.WithLicenseFile(controller.licenseFile),
			installation.WithAirgapBundle(controller.airgapBundle),
			installation.WithHostUtils(controller.hostUtils),
			installation.WithNetUtils(controller.netUtils),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithRuntimeConfig(controller.rc),
			preflight.WithLogger(controller.logger),
			preflight.WithMetricsReporter(controller.metricsReporter),
			preflight.WithHostPreflightStore(preflight.NewMemoryStore(controller.install.Steps.HostPreflight)),
			preflight.WithNetUtils(controller.netUtils),
		)
	}

	if controller.infraManager == nil {
		controller.infraManager = infra.NewInfraManager(
			infra.WithRuntimeConfig(controller.rc),
			infra.WithLogger(controller.logger),
			infra.WithInfra(controller.install.Steps.Infra),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicenseFile(controller.licenseFile),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithConfigValues(controller.configValues),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
		)
	}

	return controller, nil
}
