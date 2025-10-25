package install

import (
	"context"
	"errors"
	"fmt"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfigResponse, error)
	ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	SetupInfra(ctx context.Context) error
	GetInfra(ctx context.Context) (types.Infra, error)
	// App controller methods
	appcontroller.Controller
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager   installation.InstallationManager
	infraManager          infra.InfraManager
	metricsReporter       metrics.ReporterInterface
	hcli                  helm.Client
	kcli                  client.Client
	kubernetesEnvSettings *helmcli.EnvSettings
	releaseData           *release.ReleaseData
	password              string
	tlsConfig             types.TLSConfig
	license               []byte
	airgapBundle          string
	configValues          types.AppConfigValues
	endUserConfig         *ecv1beta1.Config
	store                 store.Store
	ki                    kubernetesinstallation.Installation
	stateMachine          statemachine.Interface
	logger                logrus.FieldLogger
	// App controller composition
	*appcontroller.AppController
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

func WithKubernetesEnvSettings(envSettings *helmcli.EnvSettings) InstallControllerOption {
	return func(c *InstallController) {
		c.kubernetesEnvSettings = envSettings
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

	// If none is provided, use the default env settings from helm
	if controller.kubernetesEnvSettings == nil {
		controller.kubernetesEnvSettings = helmcli.New()
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager(
			installation.WithLogger(controller.logger),
			installation.WithInstallationStore(controller.store.KubernetesInstallationStore()),
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
			appcontroller.WithAirgapBundle(controller.airgapBundle),
			appcontroller.WithPrivateCACertConfigMapName(""), // Private CA ConfigMap functionality not yet implemented for Kubernetes installations
			appcontroller.WithHelmClient(controller.hcli),
			appcontroller.WithKubeClient(controller.kcli),
			appcontroller.WithKubernetesEnvSettings(controller.kubernetesEnvSettings),
		)
		if err != nil {
			return nil, fmt.Errorf("create app install controller: %w", err)
		}
		controller.AppController = appController
	}

	if controller.infraManager == nil {
		infraManager, err := infra.NewInfraManager(
			infra.WithLogger(controller.logger),
			infra.WithInfraStore(controller.store.KubernetesInfraStore()),
			infra.WithKubernetesEnvSettings(controller.kubernetesEnvSettings),
			infra.WithPassword(controller.password),
			infra.WithTLSConfig(controller.tlsConfig),
			infra.WithLicense(controller.license),
			infra.WithAirgapBundle(controller.airgapBundle),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
			infra.WithHelmClient(controller.hcli),
		)
		if err != nil {
			return nil, fmt.Errorf("create infra manager: %w", err)
		}
		controller.infraManager = infraManager
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
