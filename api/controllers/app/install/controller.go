package install

import (
	"context"
	"errors"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	TemplateAppConfig(ctx context.Context, values types.AppConfigValues, maskPasswords bool) (types.AppConfig, error)
	PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) error
	GetAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
	RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) error
	GetAppPreflightStatus(ctx context.Context) (types.Status, error)
	GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error)
	GetAppPreflightTitles(ctx context.Context) ([]string, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	appConfigManager    appconfig.AppConfigManager
	appPreflightManager apppreflightmanager.AppPreflightManager
	appReleaseManager   appreleasemanager.AppReleaseManager
	stateMachine        statemachine.Interface
	logger              logrus.FieldLogger
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

	return controller, nil
}

func (c *InstallController) validateInit() error {
	if c.appConfigManager == nil {
		return errors.New("appConfigManager is required for App Install Controller")
	}
	if c.stateMachine == nil {
		return errors.New("stateMachine is required for App Install Controller")
	}
	if c.appPreflightManager == nil {
		return errors.New("appPreflightManager is required for App Install Controller")
	}
	if c.appReleaseManager == nil {
		return errors.New("appReleaseManager is required for App Install Controller")
	}
	return nil
}
