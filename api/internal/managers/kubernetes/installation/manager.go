package installation

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/sirupsen/logrus"
)

var _ InstallationManager = &installationManager{}

// InstallationManager provides methods for validating and setting defaults for installation configuration
type InstallationManager interface {
	GetConfig() (types.KubernetesInstallationConfig, error)
	SetConfig(config types.KubernetesInstallationConfig) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	ValidateConfig(config types.KubernetesInstallationConfig, managerPort int) error
	SetConfigDefaults(config *types.KubernetesInstallationConfig) error
	ConfigureInstallation(ctx context.Context, ki kubernetesinstallation.Installation, config types.KubernetesInstallationConfig) error
}

// installationManager is an implementation of the InstallationManager interface
type installationManager struct {
	installationStore installation.Store
	logger            logrus.FieldLogger
}

type InstallationManagerOption func(*installationManager)

func WithLogger(logger logrus.FieldLogger) InstallationManagerOption {
	return func(c *installationManager) {
		c.logger = logger
	}
}

func WithInstallationStore(installationStore installation.Store) InstallationManagerOption {
	return func(c *installationManager) {
		c.installationStore = installationStore
	}
}

// NewInstallationManager creates a new InstallationManager with the provided options
func NewInstallationManager(opts ...InstallationManagerOption) *installationManager {
	manager := &installationManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	if manager.installationStore == nil {
		manager.installationStore = installation.NewMemoryStore()
	}

	return manager
}
