package upgrade

import (
	"context"
	"errors"
	"fmt"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Controller interface {
	// App upgrade methods
	UpgradeApp(ctx context.Context, ignoreAppPreflights bool) error
	GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error)
}

var _ Controller = (*UpgradeController)(nil)

type UpgradeController struct {
	restClientGetter genericclioptions.RESTClientGetter
	releaseData      *release.ReleaseData
	license          []byte
	airgapBundle     string
	configValues     types.AppConfigValues
	store            store.Store
	stateMachine     statemachine.Interface
	logger           logrus.FieldLogger
	// App controller composition
	*appcontroller.AppController
}

type UpgradeControllerOption func(*UpgradeController)

func WithLogger(logger logrus.FieldLogger) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.logger = logger
	}
}

func WithRESTClientGetter(restClientGetter genericclioptions.RESTClientGetter) UpgradeControllerOption {
	return func(c *UpgradeController) {
		c.restClientGetter = restClientGetter
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

	// If none is provided, use the default env settings from helm to create a RESTClientGetter
	if controller.restClientGetter == nil {
		controller.restClientGetter = helmcli.New().RESTClientGetter()
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
