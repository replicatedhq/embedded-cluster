package install

import (
	"context"
	"errors"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Controller interface {
	TemplateAppConfig(ctx context.Context, values types.AppConfigValues, maskPasswords bool) (types.AppConfig, error)
	PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) error
	GetAppConfigValues(ctx context.Context) (types.AppConfigValues, error)
}

var _ Controller = (*InstallController)(nil)

type InstallController struct {
	appConfigManager appconfig.AppConfigManager
	configValues     types.AppConfigValues
	stateMachine     statemachine.Interface
	logger           logrus.FieldLogger
}

type InstallControllerOption func(*InstallController)

func WithLogger(logger logrus.FieldLogger) InstallControllerOption {
	return func(c *InstallController) {
		c.logger = logger
	}
}

func WithConfigValues(configValues types.AppConfigValues) InstallControllerOption {
	return func(c *InstallController) {
		c.configValues = configValues
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
	return nil
}
