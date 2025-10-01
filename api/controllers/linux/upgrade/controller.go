package upgrade

import (
	"context"
	"errors"
	"fmt"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error)
	// App controller methods
	appcontroller.Controller
}

var _ Controller = (*UpgradeController)(nil)

type UpgradeController struct {
	installationManager installation.InstallationManager
	hostUtils           hostutils.HostUtilsInterface
	netUtils            utils.NetUtils
	releaseData         *release.ReleaseData
	license             []byte
	airgapBundle        string
	configValues        types.AppConfigValues
	clusterID           string
	store               store.Store
	stateMachine        statemachine.Interface
	logger              logrus.FieldLogger
	// App controller composition
	*appcontroller.AppController
}

type UpgradeControllerOption func(*UpgradeController)

func WithLogger(logger logrus.FieldLogger) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.logger = logger
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.hostUtils = hostUtils
	}
}

func WithNetUtils(netUtils utils.NetUtils) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.netUtils = netUtils
	}
}

func WithReleaseData(releaseData *release.ReleaseData) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.releaseData = releaseData
	}
}

func WithLicense(license []byte) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.license = license
	}
}

func WithAirgapBundle(airgapBundle string) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.airgapBundle = airgapBundle
	}
}

func WithConfigValues(configValues types.AppConfigValues) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.configValues = configValues
	}
}

func WithClusterID(clusterID string) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.clusterID = clusterID
	}
}

func WithAppController(appController *appcontroller.AppController) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.AppController = appController
	}
}

func WithStateMachine(stateMachine statemachine.Interface) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.stateMachine = stateMachine
	}
}

func WithStore(store store.Store) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.store = store
	}
}

func WithInstallationManager(installationManager installation.InstallationManager) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.installationManager = installationManager
	}
}

func NewUpgradeController(opts ...UpgradeControllerOption) (*UpgradeController, error) {
	controller := &UpgradeController{
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

	// Initialize the app controller with the state machine first
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
			appcontroller.WithPrivateCACertConfigMapName(adminconsole.PrivateCASConfigMapName), // Linux upgrades use the ConfigMap
		)
		if err != nil {
			return nil, fmt.Errorf("create app controller: %w", err)
		}
		controller.AppController = appController
	}

	return controller, nil
}

func (c *UpgradeController) validateReleaseData() error {
	if c.releaseData == nil {
		return errors.New("release data not found")
	}
	if c.releaseData.AppConfig == nil {
		return errors.New("app config not found")
	}
	return nil
}

// UpgradeApp delegates to the app controller
func (c *UpgradeController) UpgradeApp(ctx context.Context, ignoreAppPreflights bool) error {
	return c.AppController.UpgradeApp(ctx, ignoreAppPreflights)
}

// GetAppUpgradeStatus delegates to the app controller
func (c *UpgradeController) GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error) {
	return c.AppController.GetAppUpgradeStatus(ctx)
}

// CalculateRegistrySettings calculates registry settings for airgap installations
func (c *UpgradeController) CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
	return c.installationManager.CalculateRegistrySettings(ctx, rc)
}
