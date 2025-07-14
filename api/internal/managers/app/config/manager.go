package config

import (
	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

var _ AppConfigManager = &appConfigManager{}

// AppConfigManager provides methods for managing appConfigstructure setup
type AppConfigManager interface {
	// GetConfig returns the config with disabled groups and items filtered out
	GetConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error)
	// GetConfigValues returns the current config values
	GetConfigValues(config kotsv1beta1.Config, maskPasswords bool) (map[string]string, error)
	// ValidateConfigValues validates the config values
	ValidateConfigValues(config kotsv1beta1.Config, values map[string]string) error
	// PatchConfigValues patches the current config values
	PatchConfigValues(config kotsv1beta1.Config, values map[string]string) error
	// GetKotsadmConfigValues merges the config values with the app config defaults and returns a
	// kotsv1beta1.ConfigValues struct.
	GetKotsadmConfigValues(config kotsv1beta1.Config) (kotsv1beta1.ConfigValues, error)
}

// appConfigManager is an implementation of the AppConfigManager interface
type appConfigManager struct {
	appConfigStore configstore.Store
	logger         logrus.FieldLogger
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

// NewAppConfigManager creates a new AppConfigManager with the provided options
func NewAppConfigManager(opts ...AppConfigManagerOption) *appConfigManager {
	manager := &appConfigManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.appConfigStore == nil {
		manager.appConfigStore = configstore.NewMemoryStore()
	}

	return manager
}
