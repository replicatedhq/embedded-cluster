package config

import (
	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitemplate "github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

var _ AppConfigManager = &appConfigManager{}

// AppConfigManager provides methods for managing appConfig structure setup
type AppConfigManager interface {
	// GetConfig returns the config templated with the stored values
	GetConfig(config apitemplate.InstallationConfig) (types.AppConfig, error)
	// GetConfigValues returns the current config values
	GetConfigValues(maskPasswords bool, config apitemplate.InstallationConfig) (types.AppConfigValues, error)
	// ValidateConfigValues validates the config values
	ValidateConfigValues(values types.AppConfigValues, config apitemplate.InstallationConfig) error
	// PatchConfigValues patches the current config values
	PatchConfigValues(values types.AppConfigValues, config apitemplate.InstallationConfig) error
	// GetKotsadmConfigValues merges the config values with the app config defaults and returns a
	// kotsv1beta1.ConfigValues struct.
	GetKotsadmConfigValues(config apitemplate.InstallationConfig) (kotsv1beta1.ConfigValues, error)
	// TemplateConfig templates the config with provided values and returns the templated config
	TemplateConfig(configValues types.AppConfigValues, config apitemplate.InstallationConfig) (types.AppConfig, error)
}

// appConfigManager is an implementation of the AppConfigManager interface
type appConfigManager struct {
	rawConfig      kotsv1beta1.Config
	appConfigStore configstore.Store
	releaseData    *release.ReleaseData
	license        *kotsv1beta1.License
	logger         logrus.FieldLogger
	templateEngine *apitemplate.Engine
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

func WithReleaseData(releaseData *release.ReleaseData) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.releaseData = releaseData
	}
}

func WithLicense(license *kotsv1beta1.License) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.license = license
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

	if manager.templateEngine == nil {
		manager.templateEngine = apitemplate.NewEngine(
			&manager.rawConfig,
			apitemplate.WithMode(apitemplate.ModeConfig),
			apitemplate.WithLicense(manager.license),
			apitemplate.WithReleaseData(manager.releaseData),
		)
	}

	return manager, nil
}
