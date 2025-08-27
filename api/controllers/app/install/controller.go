package install

import (
	"context"
	"errors"
	"fmt"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"k8s.io/cli-runtime/pkg/genericclioptions"
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
	InstallApp(ctx context.Context, opts InstallAppOptions) error
	GetAppInstallStatus(ctx context.Context) (types.AppInstall, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	appConfigManager           appconfig.AppConfigManager
	appInstallManager          appinstallmanager.AppInstallManager
	appPreflightManager        apppreflightmanager.AppPreflightManager
	appReleaseManager          appreleasemanager.AppReleaseManager
	stateMachine               statemachine.Interface
	logger                     logrus.FieldLogger
	license                    []byte
	releaseData                *release.ReleaseData
	store                      store.Store
	configValues               types.AppConfigValues
	clusterID                  string
	airgapBundle               string
	privateCACertConfigMapName string
	k8sVersion                 string
	restClientGetter           genericclioptions.RESTClientGetter
	kubeConfigPath             string
}

type InstallControllerOption func(*InstallController)

func WithLogger(logger logrus.FieldLogger) InstallControllerOption {
	return func(c *InstallController) {
		c.logger = logger
	}
}

func WithAppConfigManager(appConfigManager appconfig.AppConfigManager) InstallControllerOption {
	return func(c *InstallController) {
		c.appConfigManager = appConfigManager
	}
}

func WithAppInstallManager(appInstallManager appinstallmanager.AppInstallManager) InstallControllerOption {
	return func(c *InstallController) {
		c.appInstallManager = appInstallManager
	}
}

func WithStateMachine(stateMachine statemachine.Interface) InstallControllerOption {
	return func(c *InstallController) {
		c.stateMachine = stateMachine
	}
}

func WithAppPreflightManager(appPreflightManager apppreflightmanager.AppPreflightManager) InstallControllerOption {
	return func(c *InstallController) {
		c.appPreflightManager = appPreflightManager
	}
}

func WithAppReleaseManager(appReleaseManager appreleasemanager.AppReleaseManager) InstallControllerOption {
	return func(c *InstallController) {
		c.appReleaseManager = appReleaseManager
	}
}

func WithStore(store store.Store) InstallControllerOption {
	return func(c *InstallController) {
		c.store = store
	}
}

func WithLicense(license []byte) InstallControllerOption {
	return func(c *InstallController) {
		c.license = license
	}
}

func WithReleaseData(releaseData *release.ReleaseData) InstallControllerOption {
	return func(c *InstallController) {
		c.releaseData = releaseData
	}
}

func WithConfigValues(configValues types.AppConfigValues) InstallControllerOption {
	return func(c *InstallController) {
		c.configValues = configValues
	}
}

func WithClusterID(clusterID string) InstallControllerOption {
	return func(c *InstallController) {
		c.clusterID = clusterID
	}
}

func WithAirgapBundle(airgapBundle string) InstallControllerOption {
	return func(c *InstallController) {
		c.airgapBundle = airgapBundle
	}
}

func WithPrivateCACertConfigMapName(configMapName string) InstallControllerOption {
	return func(c *InstallController) {
		c.privateCACertConfigMapName = configMapName
	}
}

func WithK8sVersion(k8sVersion string) InstallControllerOption {
	return func(c *InstallController) {
		c.k8sVersion = k8sVersion
	}
}

func WithRESTClientGetter(restClientGetter genericclioptions.RESTClientGetter) InstallControllerOption {
	return func(c *InstallController) {
		c.restClientGetter = restClientGetter
	}
}

func WithKubeConfigPath(kubeConfigPath string) InstallControllerOption {
	return func(c *InstallController) {
		c.kubeConfigPath = kubeConfigPath
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{
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
			appconfig.WithPrivateCACertConfigMapName(controller.privateCACertConfigMapName),
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
			appreleasemanager.WithPrivateCACertConfigMapName(controller.privateCACertConfigMapName),
			appreleasemanager.WithK8sVersion(controller.k8sVersion),
			appreleasemanager.WithRESTClientGetter(controller.restClientGetter),
			appreleasemanager.WithKubeConfigPath(controller.kubeConfigPath),
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
			appinstallmanager.WithK8sVersion(controller.k8sVersion),
			appinstallmanager.WithRESTClientGetter(controller.restClientGetter),
			appinstallmanager.WithKubeConfigPath(controller.kubeConfigPath),
		)
		if err != nil {
			return nil, fmt.Errorf("create app install manager: %w", err)
		}
		controller.appInstallManager = appInstallManager
	}

	return controller, nil
}

func (c *InstallController) validateInit() error {
	if c.stateMachine == nil {
		return errors.New("stateMachine is required for App Install Controller")
	}
	if c.store == nil {
		return errors.New("store is required for App Install Controller")
	}
	if c.releaseData == nil {
		return errors.New("releaseData is required for App Install Controller")
	}
	return nil
}
