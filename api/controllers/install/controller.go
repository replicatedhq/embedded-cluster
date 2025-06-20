package install

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.InstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config types.InstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error
	GetHostPreflightStatus(ctx context.Context) (types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetupInfra(ctx context.Context) error
	GetInfra(ctx context.Context) (types.Infra, error)
	SetStatus(ctx context.Context, status types.Status) error
	GetStatus(ctx context.Context) (types.Status, error)
}

type RunHostPreflightsOptions struct {
	IsUI bool
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	install              types.Install
	store                store.Store
	installationManager  installation.InstallationManager
	hostPreflightManager preflight.HostPreflightManager
	infraManager         infra.InfraManager
	rc                   runtimeconfig.RuntimeConfig
	logger               logrus.FieldLogger
	hostUtils            hostutils.HostUtilsInterface
	netUtils             utils.NetUtils
	metricsReporter      metrics.ReporterInterface
	releaseData          *release.ReleaseData
	password             string
	tlsConfig            types.TLSConfig
	licenseFile          string
	airgapBundle         string
	airgapInfo           *kotsv1beta1.Airgap
	configValues         string
	endUserConfig        *ecv1beta1.Config
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

func WithAirgapInfo(airgapInfo *kotsv1beta1.Airgap) InstallControllerOption {
	return func(c *InstallController) {
		c.airgapInfo = airgapInfo
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

func WithInfraManager(infraManager infra.InfraManager) InstallControllerOption {
	return func(c *InstallController) {
		c.infraManager = infraManager
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{}

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

	if controller.store == nil {
		controller.store = store.NewMemoryStore()
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
			installation.WithInstallationStore(controller.store.InstallationStore()),
			installation.WithLicenseFile(controller.licenseFile),
			installation.WithAirgapBundle(controller.airgapBundle),
			installation.WithHostUtils(controller.hostUtils),
			installation.WithNetUtils(controller.netUtils),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithLogger(controller.logger),
			preflight.WithMetricsReporter(controller.metricsReporter),
			preflight.WithHostPreflightStore(controller.store.PreflightStore()),
			preflight.WithNetUtils(controller.netUtils),
		)
	}

	if controller.infraManager == nil {
		controller.infraManager = infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.InfraStore()),
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
