package config

import (
	"fmt"
	"text/template"

	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

var _ AppConfigManager = &appConfigManager{}

// AppConfigManager provides methods for managing appConfigstructure setup
type AppConfigManager interface {
	// GetConfig returns the config with disabled groups and items filtered out
	GetConfig() (kotsv1beta1.Config, error)
	// GetConfigValues returns the current config values
	GetConfigValues(maskPasswords bool) (map[string]string, error)
	// ValidateConfigValues validates the config values
	ValidateConfigValues(values map[string]string) error
	// PatchConfigValues patches the current config values
	PatchConfigValues(values map[string]string) error
	// GetKotsadmConfigValues merges the config values with the app config defaults and returns a
	// kotsv1beta1.ConfigValues struct.
	GetKotsadmConfigValues() (kotsv1beta1.ConfigValues, error)
}

// appConfigManager is an implementation of the AppConfigManager interface
type appConfigManager struct {
	rawConfig      kotsv1beta1.Config
	appConfigStore configstore.Store
	logger         logrus.FieldLogger
	configTemplate *template.Template
}

type AppConfigManagerOption func(*appConfigManager)

func WithLogger(logger logrus.FieldLogger) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.logger = logger
	}
}

func WithAppConfigStore(store configstore.Store) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.appConfigStore = store
	}
}

func WithConfigTemplate(tmpl *template.Template) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.configTemplate = tmpl
	}
}

// NewAppConfigManager creates a new AppConfigManager with the provided options
func NewAppConfigManager(config kotsv1beta1.Config, opts ...AppConfigManagerOption) (*appConfigManager, error) {
	manager := &appConfigManager{
		rawConfig: config,
	}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.appConfigStore == nil {
		manager.appConfigStore = configstore.NewMemoryStore()
	}

	if manager.configTemplate == nil {
		if err := manager.initConfigTemplate(); err != nil {
			return nil, fmt.Errorf("initialize config template: %w", err)
		}
	}

	return manager, nil
}
