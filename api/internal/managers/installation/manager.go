package installation

import (
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

var _ InstallationManager = &installationManager{}

// InstallationManager provides methods for validating and setting defaults for installation configuration
type InstallationManager interface {
	ReadConfig() (*types.InstallationConfig, error)
	WriteConfig(config types.InstallationConfig) error
	ReadStatus() (*types.InstallationStatus, error)
	WriteStatus(status types.InstallationStatus) error
	ValidateConfig(config *types.InstallationConfig) error
	SetConfigDefaults(config *types.InstallationConfig) error
}

// installationManager is an implementation of the InstallationManager interface
type installationManager struct {
	installationStore InstallationStore
	netUtils          utils.NetUtils
	logger            logrus.FieldLogger
}

type InstallationManagerOption func(*installationManager)

func WithLogger(logger logrus.FieldLogger) InstallationManagerOption {
	return func(c *installationManager) {
		c.logger = logger
	}
}

func WithInstallationStore(installationStore InstallationStore) InstallationManagerOption {
	return func(c *installationManager) {
		c.installationStore = installationStore
	}
}

func WithNetUtils(netUtils utils.NetUtils) InstallationManagerOption {
	return func(c *installationManager) {
		c.netUtils = netUtils
	}
}

// NewInstallationManager creates a new InstallationManager with the provided network utilities
func NewInstallationManager(opts ...InstallationManagerOption) *installationManager {
	manager := &installationManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.installationStore == nil {
		manager.installationStore = NewMemoryStore()
	}

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	return manager
}
