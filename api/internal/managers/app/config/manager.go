package config

import (
	"fmt"

	configstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	apitemplate "github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ AppConfigManager = &appConfigManager{}

// AppConfigManager provides methods for managing appConfig structure setup
type AppConfigManager interface {
	// ValidateConfigValues validates the config values
	ValidateConfigValues(values types.AppConfigValues) error
	// PatchConfigValues patches the current config values
	PatchConfigValues(values types.AppConfigValues) error
	// TemplateConfig templates the config with provided values and returns the templated config
	TemplateConfig(configValues types.AppConfigValues, maskPasswords bool, filterHiddenItems bool) (types.AppConfig, error)
	// GetConfigValues gets the current config values
	GetConfigValues() (types.AppConfigValues, error)
	// GetKotsadmConfigValues merges the config values with the app config defaults and returns a
	// kotsv1beta1.ConfigValues struct.
	GetKotsadmConfigValues() (kotsv1beta1.ConfigValues, error)
}

// appConfigManager is an implementation of the AppConfigManager interface
type appConfigManager struct {
	rawConfig                  kotsv1beta1.Config
	appConfigStore             configstore.Store
	releaseData                *release.ReleaseData
	license                    *licensewrapper.LicenseWrapper
	isAirgap                   bool
	privateCACertConfigMapName string
	kcli                       client.Client

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

func WithLicense(license *licensewrapper.LicenseWrapper) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.license = license
	}
}

func WithIsAirgap(isAirgap bool) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.isAirgap = isAirgap
	}
}

func WithPrivateCACertConfigMapName(configMapName string) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.privateCACertConfigMapName = configMapName
	}
}

func WithKubeClient(kcli client.Client) AppConfigManagerOption {
	return func(c *appConfigManager) {
		c.kcli = kcli
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
			apitemplate.WithReleaseData(manager.releaseData),
			apitemplate.WithLicense(manager.license),
			apitemplate.WithIsAirgap(manager.isAirgap),
			apitemplate.WithPrivateCACertConfigMapName(manager.privateCACertConfigMapName),
			apitemplate.WithKubeClient(manager.kcli),
			apitemplate.WithLogger(manager.logger),
		)
	}

	// load existing config values from kube
	configValues, err := manager.readConfigValuesFromKube()
	if err != nil {
		return nil, fmt.Errorf("failed to read existing config values from kube: %w", err)
	}

	if len(configValues) > 0 {
		// initialize store with existing config values if any
		if err := manager.PatchConfigValues(configValues); err != nil {
			return nil, fmt.Errorf("failed to initialize config store with existing config values: %w", err)
		}
	}

	return manager, nil
}
