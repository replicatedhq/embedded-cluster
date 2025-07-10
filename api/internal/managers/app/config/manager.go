package config

import (
	"context"

	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/sirupsen/logrus"
)

var _ AppConfigManager = &appConfigManager{}

// AppConfigManager provides methods for managing appConfigstructure setup
type AppConfigManager interface {
	GetConfigValues() (map[string]string, error)
	SetConfigValues(ctx context.Context, values map[string]string) error
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
