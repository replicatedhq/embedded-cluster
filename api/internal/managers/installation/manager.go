package installation

import (
	"context"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

var _ InstallationManager = &installationManager{}

// InstallationManager provides methods for validating and setting defaults for installation configuration
type InstallationManager interface {
	GetConfig() (*types.InstallationConfig, error)
	SetConfig(config types.InstallationConfig) error
	GetStatus() (*types.Status, error)
	SetStatus(status types.Status) error
	ValidateConfig(config *types.InstallationConfig, managerPort int) error
	SetConfigDefaults(config *types.InstallationConfig) error
	ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig) error
}

// installationManager is an implementation of the InstallationManager interface
type installationManager struct {
	installation      *types.Installation
	installationStore InstallationStore
	licenseFile       string
	airgapBundle      string
	netUtils          utils.NetUtils
	hostUtils         hostutils.HostUtilsInterface
	logger            logrus.FieldLogger
	mu                sync.RWMutex
}

type InstallationManagerOption func(*installationManager)

func WithLogger(logger logrus.FieldLogger) InstallationManagerOption {
	return func(c *installationManager) {
		c.logger = logger
	}
}

func WithInstallation(installation *types.Installation) InstallationManagerOption {
	return func(c *installationManager) {
		c.installation = installation
	}
}

func WithInstallationStore(installationStore InstallationStore) InstallationManagerOption {
	return func(c *installationManager) {
		c.installationStore = installationStore
	}
}

func WithLicenseFile(licenseFile string) InstallationManagerOption {
	return func(c *installationManager) {
		c.licenseFile = licenseFile
	}
}

func WithAirgapBundle(airgapBundle string) InstallationManagerOption {
	return func(c *installationManager) {
		c.airgapBundle = airgapBundle
	}
}

func WithNetUtils(netUtils utils.NetUtils) InstallationManagerOption {
	return func(c *installationManager) {
		c.netUtils = netUtils
	}
}

func WithHostUtils(hostUtils hostutils.HostUtilsInterface) InstallationManagerOption {
	return func(c *installationManager) {
		c.hostUtils = hostUtils
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

	if manager.installation == nil {
		manager.installation = types.NewInstallation()
	}

	if manager.installationStore == nil {
		manager.installationStore = NewMemoryStore(manager.installation)
	}

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	if manager.hostUtils == nil {
		manager.hostUtils = hostutils.New()
	}

	return manager
}
