package install

import (
	"context"
	"sync"

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
)

type Controller interface {
	GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfig, error)
	ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error
	GetInstallationStatus(ctx context.Context) (types.Status, error)
	SetupInfra(ctx context.Context, ignoreHostPreflights bool) error
	GetInfra(ctx context.Context) (types.KubernetesInfra, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	installationManager installation.InstallationManager
	infraManager        infra.InfraManager
	metricsReporter     metrics.ReporterInterface
	releaseData         *release.ReleaseData
	endUserConfig       *ecv1beta1.Config
	password            string
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

func WithReleaseData(releaseData *release.ReleaseData) InstallControllerOption {
	return func(c *InstallController) {
		c.releaseData = releaseData
	}
}

func WithEndUserConfig(endUserConfig *ecv1beta1.Config) InstallControllerOption {
	return func(c *InstallController) {
		c.endUserConfig = endUserConfig
	}
}

func WithPassword(password string) InstallControllerOption {
	return func(c *InstallController) {
		c.password = password
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

func WithStateMachine(stateMachine statemachine.Interface) InstallControllerOption {
	return func(c *InstallController) {
		c.stateMachine = stateMachine
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{
		store:        store.NewMemoryStore(),
		logger:       logger.NewDiscardLogger(),
		stateMachine: NewStateMachine(),
		// TODO NOW: remove this
		ki: kubernetesinstallation.New(nil),
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
			infra.WithInfraStore(controller.store.KubernetesInfraStore()),
			infra.WithPassword(controller.password),
			infra.WithReleaseData(controller.releaseData),
			infra.WithEndUserConfig(controller.endUserConfig),
		)
	}

	return controller, nil
}
