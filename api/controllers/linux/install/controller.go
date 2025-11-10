package install

import (
	"context"
	"errors"
	"fmt"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	airgapmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/airgap"
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
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error)
	ConfigureInstallation(ctx context.Context, config types.LinuxInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error
	GetHostPreflightStatus(ctx context.Context) (types.Status, error)
	GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error)
	GetHostPreflightTitles(ctx context.Context) ([]string, error)
	SetupInfra(ctx context.Context, ignoreHostPreflights bool) error
	GetInfra(ctx context.Context) (types.Infra, error)
	ProcessAirgap(ctx context.Context) error
	GetAirgapStatus(ctx context.Context) (types.Airgap, error)
	CalculateRegistrySettings(ctx context.Context) (*types.RegistrySettings, error)
	// App controller methods
	appcontroller.Controller
}

type RunHostPreflightsOptions struct {
	IsUI bool
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager       installation.InstallationManager
	hostPreflightManager      preflight.HostPreflightManager
	infraManager              infra.InfraManager
	airgapManager             airgapmanager.AirgapManager
	hostUtils                 hostutils.HostUtilsInterface
	netUtils                  utils.NetUtils
	metricsReporter           metrics.ReporterInterface
	releaseData               *release.ReleaseData
	password                  string
	tlsConfig                 types.TLSConfig
	license                   []byte
	airgapBundle              string
	airgapMetadata            *airgap.AirgapMetadata
	embeddedAssetsSize        int64
	configValues              types.AppConfigValues
	endUserConfig             *ecv1beta1.Config
	clusterID                 string
	store                     store.Store
	rc                        runtimeconfig.RuntimeConfig
	hcli                      helm.Client
	kcli                      client.Client
	mcli                      metadata.Interface
	stateMachine              statemachine.Interface
	logger                    logrus.FieldLogger
	allowIgnoreHostPreflights bool
	// App controller composition
	*appcontroller.AppController
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

func WithAirgapMetadata(airgapMetadata *airgap.AirgapMetadata) InstallControllerOption {
	return func(c *InstallController) {
		c.airgapMetadata = airgapMetadata
	}
}

func WithEmbeddedAssetsSize(embeddedAssetsSize int64) InstallControllerOption {
	return func(c *InstallController) {
		c.embeddedAssetsSize = embeddedAssetsSize
	}
}

func WithConfigValues(configValues types.AppConfigValues) InstallControllerOption {
	return func(c *InstallController) {
		c.configValues = configValues
	}
}

func WithEndUserConfig(endUserConfig *ecv1beta1.Config) InstallControllerOption {
	return func(c *InstallController) {
		c.endUserConfig = endUserConfig
	}
}

func WithClusterID(clusterID string) InstallControllerOption {
	return func(c *InstallController) {
		c.clusterID = clusterID
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

func WithAirgapManager(airgapManager airgapmanager.AirgapManager) InstallControllerOption {
	return func(c *InstallController) {
		c.airgapManager = airgapManager
	}
}

func WithAppController(appController *appcontroller.AppController) InstallControllerOption {
	return func(c *InstallController) {
		c.AppController = appController
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

func WithHelmClient(hcli helm.Client) InstallControllerOption {
	return func(c *InstallController) {
		c.hcli = hcli
	}
}

func WithKubeClient(kcli client.Client) InstallControllerOption {
	return func(c *InstallController) {
		c.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) InstallControllerOption {
	return func(c *InstallController) {
		c.mcli = mcli
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

	if err := controller.validateReleaseData(); err != nil {
		return nil, err
	}

	if controller.stateMachine == nil {
		controller.stateMachine = NewStateMachine(
			WithStateMachineLogger(controller.logger),
			WithIsAirgap(controller.airgapBundle != ""),
		)
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
			installation.WithReleaseData(controller.releaseData),
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

	// Initialize the app controller with the state machine
	if controller.AppController == nil {
		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(controller.stateMachine),
			appcontroller.WithLogger(controller.logger),
			appcontroller.WithStore(controller.store),
			appcontroller.WithLicense(controller.license),
			appcontroller.WithReleaseData(controller.releaseData),
			appcontroller.WithConfigValues(controller.configValues),
			appcontroller.WithClusterID(controller.clusterID),
			appcontroller.WithAirgapBundle(controller.airgapBundle),
			appcontroller.WithPrivateCACertConfigMapName(adminconsole.PrivateCASConfigMapName), // Linux installations use the ConfigMap
			appcontroller.WithHelmClient(controller.hcli),
			appcontroller.WithKubeClient(controller.kcli),
			appcontroller.WithKubernetesEnvSettings(controller.rc.GetKubernetesEnvSettings()),
		)
		if err != nil {
			return nil, fmt.Errorf("create app controller: %w", err)
		}
		controller.AppController = appController
	}

	if controller.infraManager == nil {
		controller.infraManager = infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.LinuxInfraStore()),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicense(controller.license),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithAirgapMetadata(controller.airgapMetadata),
			infra.WithEmbeddedAssetsSize(controller.embeddedAssetsSize),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
			infra.WithClusterID(controller.clusterID),
			infra.WithHelmClient(controller.hcli),
			infra.WithKubeClient(controller.kcli),
			infra.WithMetadataClient(controller.mcli),
		)
	}

	if controller.airgapManager == nil {
		manager, err := airgapmanager.NewAirgapManager(
			airgapmanager.WithLogger(controller.logger),
			airgapmanager.WithAirgapStore(controller.store.AirgapStore()),
			airgapmanager.WithAirgapBundle(controller.airgapBundle),
			airgapmanager.WithClusterID(controller.clusterID),
		)
		if err != nil {
			return nil, fmt.Errorf("create airgap manager: %w", err)
		}
		controller.airgapManager = manager
	}

	controller.registerReportingHandlers()

	return controller, nil
}

func (c *InstallController) validateReleaseData() error {
	if c.releaseData == nil {
		return errors.New("release data not found")
	}
	if c.releaseData.AppConfig == nil {
		return errors.New("app config not found")
	}
	return nil
}
