package install

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config types.LinuxInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error
	GetHostPreflightStatus(ctx context.Context) (types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetupInfra(ctx context.Context, ignoreHostPreflights bool) error
	GetInfra(ctx context.Context) (types.LinuxInfra, error)
}

type RunHostPreflightsOptions struct {
	IsUI bool
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager       installation.InstallationManager
	hostPreflightManager      preflight.HostPreflightManager
	infraManager              infra.InfraManager
	hostUtils                 hostutils.HostUtilsInterface
	netUtils                  utils.NetUtils
	metricsReporter           metrics.ReporterInterface
	releaseData               *release.ReleaseData
	password                  string
	tlsConfig                 types.TLSConfig
	license                   []byte
	airgapBundle              string
	configValues              string
	endUserConfig             *ecv1beta1.Config
	store                     store.Store
	rc                        runtimeconfig.RuntimeConfig
	stateMachine              statemachine.Interface
	logger                    logrus.FieldLogger
	mu                        sync.RWMutex
	allowIgnoreHostPreflights bool
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

func WithLicense(license []byte) InstallControllerOption {
	return func(c *InstallController) {
		c.license = license
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

func WithInfraManager(infraManager infra.InfraManager) InstallControllerOption {
	return func(c *InstallController) {
		c.infraManager = infraManager
	}
}

func WithStateMachine(stateMachine statemachine.Interface) InstallControllerOption {
	return func(c *InstallController) {
		c.stateMachine = stateMachine
	}
}

func WithStore(store store.Store) InstallControllerOption {
	return func(c *InstallController) {
		c.store = store
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{
		store:  store.NewMemoryStore(),
		rc:     runtimeconfig.New(nil),
		logger: logger.NewDiscardLogger(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.stateMachine == nil {
		controller.stateMachine = NewStateMachine(WithStateMachineLogger(controller.logger))
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
			installation.WithLogger(controller.logger),
			installation.WithInstallationStore(controller.store.LinuxInstallationStore()),
			installation.WithLicense(controller.license),
			installation.WithAirgapBundle(controller.airgapBundle),
			installation.WithHostUtils(controller.hostUtils),
			installation.WithNetUtils(controller.netUtils),
		)
	}

	if controller.hostPreflightManager == nil {
		controller.hostPreflightManager = preflight.NewHostPreflightManager(
			preflight.WithLogger(controller.logger),
			preflight.WithHostPreflightStore(controller.store.LinuxPreflightStore()),
			preflight.WithNetUtils(controller.netUtils),
		)
	}

	if controller.infraManager == nil {
		controller.infraManager = infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.LinuxInfraStore()),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicense(controller.license),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithConfigValues(controller.configValues),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
		)
	}

	controller.registerReportingHandlers()

	return controller, nil
}
