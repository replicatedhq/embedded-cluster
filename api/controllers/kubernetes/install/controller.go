package install

import (
	"context"
	"errors"
	"fmt"
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
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	SetupInfra(ctx context.Context) error
	GetInfra(ctx context.Context) (types.Infra, error)
	GetAppConfig(ctx context.Context) (types.AppConfig, error)
	TemplateAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error)
	PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) error
	GetAppConfigValues(ctx context.Context, maskPasswords bool) (types.AppConfigValues, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager installation.InstallationManager
	infraManager        infra.InfraManager
	appConfigManager    appconfig.AppConfigManager
	metricsReporter     metrics.ReporterInterface
	restClientGetter    genericclioptions.RESTClientGetter
	releaseData         *release.ReleaseData
	password            string
	tlsConfig           types.TLSConfig
	license             []byte
	airgapBundle        string
	configValues        types.AppConfigValues
	endUserConfig       *ecv1beta1.Config
	store               store.Store
	ki                  kubernetesinstallation.Installation
	stateMachine        statemachine.Interface
	logger              logrus.FieldLogger
	mu                  sync.RWMutex
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

func WithRESTClientGetter(restClientGetter genericclioptions.RESTClientGetter) InstallControllerOption {
	return func(c *InstallController) {
		c.restClientGetter = restClientGetter
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
		store:  store.NewMemoryStore(),
		logger: logger.NewDiscardLogger(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if err := controller.validateReleaseData(); err != nil {
		return nil, err
	}

	if controller.stateMachine == nil {
		controller.stateMachine = NewStateMachine(WithStateMachineLogger(controller.logger))
	}

	// If none is provided, use the default env settings from helm to create a RESTClientGetter
	if controller.restClientGetter == nil {
		controller.restClientGetter = helmcli.New().RESTClientGetter()
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
			installation.WithInstallationStore(controller.store.KubernetesInstallationStore()),
		)
	}

	if controller.infraManager == nil {
		infraManager, err := infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.LinuxInfraStore()),
			infra.WithRESTClientGetter(controller.restClientGetter),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicense(controller.license),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
		)
		if err != nil {
			return nil, fmt.Errorf("create infra manager: %w", err)
		}
		controller.infraManager = infraManager
	}

	if controller.appConfigManager == nil {
		appConfigManager, err := appconfig.NewAppConfigManager(
			*controller.releaseData.AppConfig,
			appconfig.WithLogger(controller.logger),
			appconfig.WithAppConfigStore(controller.store.AppConfigStore()),
		)
		if err != nil {
			return nil, fmt.Errorf("create app config manager: %w", err)
		}
		controller.appConfigManager = appConfigManager
	}

	if controller.configValues != nil {
		err := controller.appConfigManager.ValidateConfigValues(controller.configValues, controller.ki)
		if err != nil {
			return nil, fmt.Errorf("validate app config values: %w", err)
		}
		err = controller.appConfigManager.PatchConfigValues(controller.configValues, controller.ki)
		if err != nil {
			return nil, fmt.Errorf("patch app config values: %w", err)
		}
	}

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
