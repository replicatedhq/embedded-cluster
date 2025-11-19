package installation

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ InstallationManager = &installationManager{}

// InstallationManager provides methods for validating and setting defaults for installation configuration
type InstallationManager interface {
	GetConfig(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error)
	GetConfigValues() (types.LinuxInstallationConfig, error)
	SetConfigValues(config types.LinuxInstallationConfig) error
	GetDefaults(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error)
	ValidateConfig(config types.LinuxInstallationConfig, managerPort int) error
	ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig) error
	CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error)
	GetRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error)
}

// installationManager is an implementation of the InstallationManager interface
type installationManager struct {
	installationStore installation.Store
	license           []byte
	airgapBundle      string
	releaseData       *release.ReleaseData
	kcli              client.Client
	mcli              metadata.Interface
	netUtils          utils.NetUtils
	hostUtils         hostutils.HostUtilsInterface
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

func WithLicense(license []byte) InstallationManagerOption {
	return func(c *installationManager) {
		c.license = license
	}
}

func WithAirgapBundle(airgapBundle string) InstallationManagerOption {
	return func(c *installationManager) {
		c.airgapBundle = airgapBundle
	}
}

func WithKubeClient(kcli client.Client) InstallationManagerOption {
	return func(c *installationManager) {
		c.kcli = kcli
	}
}

func WithMetadataClient(mcli metadata.Interface) InstallationManagerOption {
	return func(c *installationManager) {
		c.mcli = mcli
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

func WithReleaseData(releaseData *release.ReleaseData) InstallationManagerOption {
	return func(c *installationManager) {
		c.releaseData = releaseData
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

	if manager.netUtils == nil {
		manager.netUtils = utils.NewNetUtils()
	}

	if manager.hostUtils == nil {
		manager.hostUtils = hostutils.New()
	}

	return manager
}
