package app

import (
	"context"
	"errors"
	"fmt"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	appupgrademanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	helmcli "helm.sh/helm/v3/pkg/cli"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

type Controller interface {
	TemplateAppConfig(ctx context.Context, values types.AppConfigValues, maskPasswords bool) (types.AppConfig, error)
	PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) error
	GetAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) error
	GetAppPreflightStatus(ctx context.Context) (types.Status, error)
	GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error)
	GetAppPreflightTitles(ctx context.Context) ([]string, error)
	InstallApp(ctx context.Context, ignoreAppPreflights bool) error
	GetAppInstallStatus(ctx context.Context) (types.AppInstall, error)
	UpgradeApp(ctx context.Context, ignoreAppPreflights bool) error
	GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error)
}

var _ Controller = (*AppController)(nil)

type AppController struct {
	appConfigManager           appconfig.AppConfigManager
	appInstallManager          appinstallmanager.AppInstallManager
	appPreflightManager        apppreflightmanager.AppPreflightManager
	appReleaseManager          appreleasemanager.AppReleaseManager
	appUpgradeManager          appupgrademanager.AppUpgradeManager
	stateMachine               statemachine.Interface
	logger                     logrus.FieldLogger
	license                    []byte
	releaseData                *release.ReleaseData
	hcli                       helm.Client
	kcli                       client.Client
	kubernetesEnvSettings      *helmcli.EnvSettings
	store                      store.Store
	configValues               types.AppConfigValues
	clusterID                  string
	airgapBundle               string
	privateCACertConfigMapName string
}

type AppControllerOption func(*AppController)

func WithLogger(logger logrus.FieldLogger) AppControllerOption {
	return func(c *AppController) {
		c.logger = logger
	}
}

func WithAppConfigManager(appConfigManager appconfig.AppConfigManager) AppControllerOption {
	return func(c *AppController) {
		c.appConfigManager = appConfigManager
	}
}

func WithAppInstallManager(appInstallManager appinstallmanager.AppInstallManager) AppControllerOption {
	return func(c *AppController) {
		c.appInstallManager = appInstallManager
	}
}

func WithStateMachine(stateMachine statemachine.Interface) AppControllerOption {
	return func(c *AppController) {
		c.stateMachine = stateMachine
	}
}

func WithAppPreflightManager(appPreflightManager apppreflightmanager.AppPreflightManager) AppControllerOption {
	return func(c *AppController) {
		c.appPreflightManager = appPreflightManager
	}
}

func WithAppReleaseManager(appReleaseManager appreleasemanager.AppReleaseManager) AppControllerOption {
	return func(c *AppController) {
		c.appReleaseManager = appReleaseManager
	}
}

func WithStore(store store.Store) AppControllerOption {
	return func(c *AppController) {
		c.store = store
	}
}

func WithLicense(license []byte) AppControllerOption {
	return func(c *AppController) {
		c.license = license
	}
}

func WithReleaseData(releaseData *release.ReleaseData) AppControllerOption {
	return func(c *AppController) {
		c.releaseData = releaseData
	}
}

func WithHelmClient(hcli helm.Client) AppControllerOption {
	return func(c *AppController) {
		c.hcli = hcli
	}
}

func WithKubeClient(kcli client.Client) AppControllerOption {
	return func(c *AppController) {
		c.kcli = kcli
	}
}

func WithKubernetesEnvSettings(envSettings *helmcli.EnvSettings) AppControllerOption {
	return func(c *AppController) {
		c.kubernetesEnvSettings = envSettings
	}
}

func WithConfigValues(configValues types.AppConfigValues) AppControllerOption {
	return func(c *AppController) {
		c.configValues = configValues
	}
}

func WithClusterID(clusterID string) AppControllerOption {
	return func(c *AppController) {
		c.clusterID = clusterID
	}
}

func WithAirgapBundle(airgapBundle string) AppControllerOption {
	return func(c *AppController) {
		c.airgapBundle = airgapBundle
	}
}

func WithPrivateCACertConfigMapName(configMapName string) AppControllerOption {
	return func(c *AppController) {
		c.privateCACertConfigMapName = configMapName
	}
}

func WithAppUpgradeManager(appUpgradeManager appupgrademanager.AppUpgradeManager) AppControllerOption {
	return func(c *AppController) {
		c.appUpgradeManager = appUpgradeManager
	}
}

func NewAppController(opts ...AppControllerOption) (*AppController, error) {
	controller := &AppController{
		logger: logger.NewDiscardLogger(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	if err := controller.validateInit(); err != nil {
		return nil, err
	}

	var license *kotsv1beta1.License
	if len(controller.license) > 0 {
		license = &kotsv1beta1.License{}
		if err := kyaml.Unmarshal(controller.license, license); err != nil {
			return nil, fmt.Errorf("parse license: %w", err)
		}
	}

	if controller.appConfigManager == nil {
		appConfigManager, err := appconfig.NewAppConfigManager(
			*controller.releaseData.AppConfig,
			appconfig.WithLogger(controller.logger),
			appconfig.WithAppConfigStore(controller.store.AppConfigStore()),
			appconfig.WithReleaseData(controller.releaseData),
			appconfig.WithLicense(license),
			appconfig.WithIsAirgap(controller.airgapBundle != ""),
			appconfig.WithPrivateCACertConfigMapName(controller.privateCACertConfigMapName),
			appconfig.WithKubeClient(controller.kcli),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create app config manager: %w", err)
		}
		controller.appConfigManager = appConfigManager
	}

	if controller.configValues != nil {
		err := controller.appConfigManager.ValidateConfigValues(controller.configValues)
		if err != nil {
			return nil, fmt.Errorf("validate app config values: %w", err)
		}
		err = controller.appConfigManager.PatchConfigValues(controller.configValues)
		if err != nil {
			return nil, fmt.Errorf("patch app config values: %w", err)
		}
	}

	if controller.appPreflightManager == nil {
		controller.appPreflightManager = apppreflightmanager.NewAppPreflightManager(
			apppreflightmanager.WithLogger(controller.logger),
			apppreflightmanager.WithAppPreflightStore(controller.store.AppPreflightStore()),
		)
	}

	if controller.appReleaseManager == nil {
		appReleaseManager, err := appreleasemanager.NewAppReleaseManager(
			*controller.releaseData.AppConfig,
			appreleasemanager.WithLogger(controller.logger),
			appreleasemanager.WithReleaseData(controller.releaseData),
			appreleasemanager.WithLicense(license),
			appreleasemanager.WithIsAirgap(controller.airgapBundle != ""),
			appreleasemanager.WithPrivateCACertConfigMapName(controller.privateCACertConfigMapName),
			appreleasemanager.WithHelmClient(controller.hcli),
			appreleasemanager.WithKubeClient(controller.kcli),
		)
		if err != nil {
			return nil, fmt.Errorf("create app release manager: %w", err)
		}
		controller.appReleaseManager = appReleaseManager
	}

	if controller.appInstallManager == nil {
		appInstallManager, err := appinstallmanager.NewAppInstallManager(
			appinstallmanager.WithLogger(controller.logger),
			appinstallmanager.WithReleaseData(controller.releaseData),
			appinstallmanager.WithLicense(controller.license),
			appinstallmanager.WithClusterID(controller.clusterID),
			appinstallmanager.WithAirgapBundle(controller.airgapBundle),
			appinstallmanager.WithAppInstallStore(controller.store.AppInstallStore()),
			appinstallmanager.WithKubeClient(controller.kcli),
			appinstallmanager.WithKubernetesEnvSettings(controller.kubernetesEnvSettings),
		)
		if err != nil {
			return nil, fmt.Errorf("create app install manager: %w", err)
		}
		controller.appInstallManager = appInstallManager
	}

	if controller.appUpgradeManager == nil {
		appUpgradeManager, err := appupgrademanager.NewAppUpgradeManager(
			appupgrademanager.WithLogger(controller.logger),
			appupgrademanager.WithReleaseData(controller.releaseData),
			appupgrademanager.WithLicense(controller.license),
			appupgrademanager.WithClusterID(controller.clusterID),
			appupgrademanager.WithAirgapBundle(controller.airgapBundle),
			appupgrademanager.WithAppUpgradeStore(controller.store.AppUpgradeStore()),
			appupgrademanager.WithKubeClient(controller.kcli),
			appupgrademanager.WithKubernetesEnvSettings(controller.kubernetesEnvSettings),
		)
		if err != nil {
			return nil, fmt.Errorf("create app upgrade manager: %w", err)
		}
		controller.appUpgradeManager = appUpgradeManager
	}

	return controller, nil
}

func (c *AppController) validateInit() error {
	if c.stateMachine == nil {
		return errors.New("stateMachine is required for App Controller")
	}
	if c.store == nil {
		return errors.New("store is required for App Controller")
	}
	if c.releaseData == nil {
		return errors.New("releaseData is required for App Controller")
	}
	return nil
}
