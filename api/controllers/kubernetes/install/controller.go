package install

import (
	"context"
	"sync"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	SetupInfra(ctx context.Context) error
	GetInfra(ctx context.Context) (types.Infra, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager     installation.InstallationManager
	infraManager            infra.InfraManager
	appConfigManager        appconfig.AppConfigManager
	metricsReporter         metrics.ReporterInterface
	restClientGetterFactory func(namespace string) genericclioptions.RESTClientGetter
	releaseData             *release.ReleaseData
	password                string
	tlsConfig               types.TLSConfig
	license                 []byte
	airgapBundle            string
	configValues            string
	endUserConfig           *ecv1beta1.Config
	store                   store.Store
	ki                      kubernetesinstallation.Installation
	stateMachine            statemachine.Interface
	logger                  logrus.FieldLogger
	mu                      sync.RWMutex
}

type InstallControllerOption func(*InstallController)

func WithInstallation(ki kubernetesinstallation.Installation) InstallControllerOption {
	return func(c *InstallController) {
		c.ki = ki
	}
}

func WithLogger(logger logrus.FieldLogger) InstallControllerOption {
	return func(c *InstallController) {
		c.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) InstallControllerOption {
	return func(c *InstallController) {
		c.metricsReporter = metricsReporter
	}
}

func WithRESTClientGetterFactory(restClientGetterFactory func(namespace string) genericclioptions.RESTClientGetter) InstallControllerOption {
	return func(c *InstallController) {
		c.restClientGetterFactory = restClientGetterFactory
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

func WithInstallationManager(installationManager installation.InstallationManager) InstallControllerOption {
	return func(c *InstallController) {
		c.installationManager = installationManager
	}
}

func WithInfraManager(infraManager infra.InfraManager) InstallControllerOption {
	return func(c *InstallController) {
		c.infraManager = infraManager
	}
}

func WithAppConfigManager(appConfigManager appconfig.AppConfigManager) InstallControllerOption {
	return func(c *InstallController) {
		c.appConfigManager = appConfigManager
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
		store:        store.NewMemoryStore(),
		logger:       logger.NewDiscardLogger(),
		stateMachine: NewStateMachine(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
			installation.WithInstallationStore(controller.store.KubernetesInstallationStore()),
		)
	}

	if controller.infraManager == nil {
		controller.infraManager = infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.LinuxInfraStore()),
			infra.WithRESTClientGetterFactory(controller.restClientGetterFactory),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicense(controller.license),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithConfigValues(controller.configValues),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
		)
	}

	if controller.appConfigManager == nil {
		controller.appConfigManager = appconfig.NewAppConfigManager(
			appconfig.WithLogger(controller.logger),
			appconfig.WithAppConfigStore(controller.store.AppConfigStore()),
		)
	}

	return controller, nil
}
