package kurlmigration

import (
	"context"

	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	kurlmigrationstore "github.com/replicatedhq/embedded-cluster/api/internal/store/kurlmigration"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

var _ Manager = &kurlMigrationManager{}

// Manager provides methods for managing kURL to EC migrations
type Manager interface {
	// GetKurlConfig extracts configuration from the running kURL cluster
	GetKurlConfig(ctx context.Context) (types.LinuxInstallationConfig, error)

	// GetECDefaults returns EC default configuration
	GetECDefaults(ctx context.Context) (types.LinuxInstallationConfig, error)

	// MergeConfigs merges user, kURL, and default configs with proper precedence
	// Precedence order: userConfig > kurlConfig > defaults
	MergeConfigs(userConfig, kurlConfig, defaults types.LinuxInstallationConfig) types.LinuxInstallationConfig

	// ValidateTransferMode validates the transfer mode is "copy" or "move"
	ValidateTransferMode(mode types.TransferMode) error

	// ExecutePhase executes a kURL migration phase
	ExecutePhase(ctx context.Context, phase types.KURLMigrationPhase) error
}

// kurlMigrationManager is an implementation of the Manager interface
type kurlMigrationManager struct {
	store               kurlmigrationstore.Store
	installationManager linuxinstallation.InstallationManager
	logger              logrus.FieldLogger
}

type ManagerOption func(*kurlMigrationManager)

func WithStore(store kurlmigrationstore.Store) ManagerOption {
	return func(m *kurlMigrationManager) {
		m.store = store
	}
}

func WithLogger(logger logrus.FieldLogger) ManagerOption {
	return func(m *kurlMigrationManager) {
		m.logger = logger
	}
}

func WithInstallationManager(im linuxinstallation.InstallationManager) ManagerOption {
	return func(m *kurlMigrationManager) {
		m.installationManager = im
	}
}

// NewManager creates a new migration Manager with the provided options
func NewManager(opts ...ManagerOption) *kurlMigrationManager {
	manager := &kurlMigrationManager{}

	for _, opt := range opts {
		opt(manager)
	}

	if manager.logger == nil {
		manager.logger = logger.NewDiscardLogger()
	}

	return manager
}

// GetKurlConfig extracts configuration from the running kURL cluster
func (m *kurlMigrationManager) GetKurlConfig(ctx context.Context) (types.LinuxInstallationConfig, error) {
	// TODO(sc-130962): Implement kURL cluster configuration extraction
	// This will query the kURL cluster for:
	// - Pod and Service CIDRs from kube-controller-manager
	// - Network configuration
	// - Admin console port
	// - Data directory
	// - Proxy settings
	// - Namespace discovery: Query cluster to find kotsadm namespace by looking for
	//   pods/services with app.kubernetes.io/name=kotsadm label. This is necessary
	//   because kURL can install KOTS in any namespace, not just "default".
	m.logger.Debug("GetKurlConfig: Skeleton implementation, returning empty config")
	return types.LinuxInstallationConfig{}, nil
}

// GetECDefaults returns EC default configuration
func (m *kurlMigrationManager) GetECDefaults(ctx context.Context) (types.LinuxInstallationConfig, error) {
	// TODO(sc-130962): Implement EC defaults extraction
	// This will use the installation manager to get EC defaults
	// For now, return empty config as skeleton implementation
	m.logger.Debug("GetECDefaults: Skeleton implementation, returning empty config")
	return types.LinuxInstallationConfig{}, nil
}

// MergeConfigs merges user, kURL, and default configs with proper precedence
// Precedence order: userConfig > kurlConfig > defaults
func (m *kurlMigrationManager) MergeConfigs(userConfig, kurlConfig, defaults types.LinuxInstallationConfig) types.LinuxInstallationConfig {
	// Start with defaults as the base
	merged := defaults

	// Apply kURL config, overwriting defaults only for non-zero values
	if kurlConfig.DataDirectory != "" {
		merged.DataDirectory = kurlConfig.DataDirectory
	}
	if kurlConfig.HTTPProxy != "" {
		merged.HTTPProxy = kurlConfig.HTTPProxy
	}
	if kurlConfig.HTTPSProxy != "" {
		merged.HTTPSProxy = kurlConfig.HTTPSProxy
	}
	if kurlConfig.NoProxy != "" {
		merged.NoProxy = kurlConfig.NoProxy
	}
	if kurlConfig.NetworkInterface != "" {
		merged.NetworkInterface = kurlConfig.NetworkInterface
	}
	if kurlConfig.PodCIDR != "" {
		merged.PodCIDR = kurlConfig.PodCIDR
	}
	if kurlConfig.ServiceCIDR != "" {
		merged.ServiceCIDR = kurlConfig.ServiceCIDR
	}
	if kurlConfig.GlobalCIDR != "" {
		merged.GlobalCIDR = kurlConfig.GlobalCIDR
	}

	// Apply user config, overwriting merged values only for non-zero values
	// This gives user config the highest precedence
	if userConfig.DataDirectory != "" {
		merged.DataDirectory = userConfig.DataDirectory
	}
	if userConfig.HTTPProxy != "" {
		merged.HTTPProxy = userConfig.HTTPProxy
	}
	if userConfig.HTTPSProxy != "" {
		merged.HTTPSProxy = userConfig.HTTPSProxy
	}
	if userConfig.NoProxy != "" {
		merged.NoProxy = userConfig.NoProxy
	}
	if userConfig.NetworkInterface != "" {
		merged.NetworkInterface = userConfig.NetworkInterface
	}
	if userConfig.PodCIDR != "" {
		merged.PodCIDR = userConfig.PodCIDR
	}
	if userConfig.ServiceCIDR != "" {
		merged.ServiceCIDR = userConfig.ServiceCIDR
	}
	if userConfig.GlobalCIDR != "" {
		merged.GlobalCIDR = userConfig.GlobalCIDR
	}

	m.logger.WithFields(logrus.Fields{
		"userConfig":   userConfig,
		"kurlConfig":   kurlConfig,
		"defaults":     defaults,
		"mergedConfig": merged,
	}).Debug("MergeConfigs: Merged configuration with precedence user > kURL > defaults")

	return merged
}

// ValidateTransferMode validates the transfer mode is "copy" or "move"
func (m *kurlMigrationManager) ValidateTransferMode(mode types.TransferMode) error {
	switch mode {
	case types.TransferModeCopy, types.TransferModeMove:
		return nil
	default:
		return types.ErrInvalidTransferMode
	}
}

// ExecutePhase executes a kURL migration phase
func (m *kurlMigrationManager) ExecutePhase(ctx context.Context, phase types.KURLMigrationPhase) error {
	// TODO(sc-130983): Implement phase execution
	// This will handle:
	// - Discovery phase: GetKurlConfig, validate cluster
	// - Preparation phase: Backup, pre-migration checks
	// - ECInstall phase: Install EC alongside kURL
	// - DataTransfer phase: Copy/move data from kURL to EC
	// - Completed phase: Final validation, cleanup
	m.logger.WithField("phase", phase).Debug("ExecutePhase: Skeleton implementation")
	return types.ErrKURLMigrationPhaseNotImplemented
}
