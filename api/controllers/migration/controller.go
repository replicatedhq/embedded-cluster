package migration

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	migrationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/migration"
	migrationstore "github.com/replicatedhq/embedded-cluster/api/internal/store/migration"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

// Controller provides methods for managing kURL to Embedded Cluster migrations
type Controller interface {
	// GetInstallationConfig extracts kURL config, gets EC defaults, and returns merged config
	GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error)

	// StartMigration validates config, generates UUID, initializes state, launches background goroutine
	StartMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error)

	// GetMigrationStatus returns current migration status
	GetMigrationStatus(ctx context.Context) (types.MigrationStatusResponse, error)

	// Run is the internal orchestration loop (skeleton for this PR, implemented in PR 8)
	Run(ctx context.Context) error
}

var _ Controller = (*MigrationController)(nil)

// MigrationController implements the Controller interface
type MigrationController struct {
	manager             migrationmanager.Manager
	store               migrationstore.Store
	installationManager linuxinstallation.InstallationManager
	logger              logrus.FieldLogger
}

// ControllerOption is a functional option for configuring the MigrationController
type ControllerOption func(*MigrationController)

// WithManager sets the migration manager
func WithManager(manager migrationmanager.Manager) ControllerOption {
	return func(c *MigrationController) {
		c.manager = manager
	}
}

// WithStore sets the migration store
func WithStore(store migrationstore.Store) ControllerOption {
	return func(c *MigrationController) {
		c.store = store
	}
}

// WithInstallationManager sets the installation manager
func WithInstallationManager(manager linuxinstallation.InstallationManager) ControllerOption {
	return func(c *MigrationController) {
		c.installationManager = manager
	}
}

// WithLogger sets the logger
func WithLogger(log logrus.FieldLogger) ControllerOption {
	return func(c *MigrationController) {
		c.logger = log
	}
}

// NewMigrationController creates a new migration controller with the provided options
func NewMigrationController(opts ...ControllerOption) (*MigrationController, error) {
	controller := &MigrationController{
		store:  migrationstore.NewMemoryStore(),
		logger: logger.NewDiscardLogger(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	// Validate required dependencies
	if controller.manager == nil {
		return nil, fmt.Errorf("migration manager is required")
	}

	return controller, nil
}

// GetInstallationConfig extracts kURL config, gets EC defaults, and returns merged config
func (c *MigrationController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	c.logger.Debug("GetInstallationConfig: Fetching kURL config, EC defaults, and merging")

	// Get kURL config from the running cluster
	kurlConfig, err := c.manager.GetKurlConfig(ctx)
	if err != nil {
		c.logger.WithError(err).Error("GetInstallationConfig: Failed to get kURL config")
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get kurl config: %w", err)
	}

	// Get EC defaults
	defaults, err := c.manager.GetECDefaults(ctx)
	if err != nil {
		c.logger.WithError(err).Error("GetInstallationConfig: Failed to get EC defaults")
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get ec defaults: %w", err)
	}

	// Merge configs with empty user config (user hasn't submitted config yet)
	emptyUserConfig := types.LinuxInstallationConfig{}
	resolved := c.manager.MergeConfigs(emptyUserConfig, kurlConfig, defaults)

	c.logger.WithFields(logrus.Fields{
		"kurlConfig": kurlConfig,
		"defaults":   defaults,
		"resolved":   resolved,
	}).Debug("GetInstallationConfig: Config merged successfully")

	return types.LinuxInstallationConfigResponse{
		Values:   kurlConfig,
		Defaults: defaults,
		Resolved: resolved,
	}, nil
}

// StartMigration validates config, generates UUID, initializes state, launches background goroutine
func (c *MigrationController) StartMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"transferMode": transferMode,
		"config":       config,
	}).Info("StartMigration: Starting migration")

	// Validate transfer mode
	if err := c.manager.ValidateTransferMode(transferMode); err != nil {
		c.logger.WithError(err).Error("StartMigration: Invalid transfer mode")
		return "", types.NewBadRequestError(err)
	}

	// Check if migration already exists
	if _, err := c.store.GetMigrationID(); err == nil {
		c.logger.Warn("StartMigration: Migration already in progress")
		return "", types.NewConflictError(types.ErrMigrationAlreadyStarted)
	}

	// Generate UUID for migration
	migrationID := uuid.New().String()
	c.logger.WithField("migrationID", migrationID).Debug("StartMigration: Generated migration ID")

	// Get defaults and merge with user config
	kurlConfig, err := c.manager.GetKurlConfig(ctx)
	if err != nil {
		c.logger.WithError(err).Error("StartMigration: Failed to get kURL config")
		return "", fmt.Errorf("get kurl config: %w", err)
	}

	defaults, err := c.manager.GetECDefaults(ctx)
	if err != nil {
		c.logger.WithError(err).Error("StartMigration: Failed to get EC defaults")
		return "", fmt.Errorf("get ec defaults: %w", err)
	}

	resolvedConfig := c.manager.MergeConfigs(config, kurlConfig, defaults)
	c.logger.WithField("resolvedConfig", resolvedConfig).Debug("StartMigration: Config merged")

	// Initialize migration in store
	if err := c.store.InitializeMigration(migrationID, string(transferMode), resolvedConfig); err != nil {
		c.logger.WithError(err).Error("StartMigration: Failed to initialize migration")
		return "", fmt.Errorf("initialize migration: %w", err)
	}

	// Set initial state to NotStarted
	if err := c.store.SetState(types.MigrationStateNotStarted); err != nil {
		c.logger.WithError(err).Error("StartMigration: Failed to set initial state")
		return "", fmt.Errorf("set initial state: %w", err)
	}

	c.logger.WithField("migrationID", migrationID).Info("StartMigration: Migration initialized, launching background goroutine")

	// Launch background goroutine with detached context
	// We use WithoutCancel to inherit context values (trace IDs, logger fields)
	// but detach from the request's cancellation so migration continues after HTTP response
	backgroundCtx := context.WithoutCancel(ctx)
	go func() {
		if err := c.Run(backgroundCtx); err != nil {
			c.logger.WithError(err).Error("StartMigration: Background migration failed")
		}
	}()

	return migrationID, nil
}

// GetMigrationStatus returns current migration status
func (c *MigrationController) GetMigrationStatus(ctx context.Context) (types.MigrationStatusResponse, error) {
	c.logger.Debug("GetMigrationStatus: Fetching migration status")

	status, err := c.store.GetStatus()
	if err != nil {
		if err == types.ErrNoActiveMigration {
			c.logger.Warn("GetMigrationStatus: No active migration found")
			return types.MigrationStatusResponse{}, types.NewNotFoundError(err)
		}
		c.logger.WithError(err).Error("GetMigrationStatus: Failed to get status")
		return types.MigrationStatusResponse{}, fmt.Errorf("get status: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"state":    status.State,
		"phase":    status.Phase,
		"progress": status.Progress,
	}).Debug("GetMigrationStatus: Status retrieved")

	return status, nil
}

// Run is the internal orchestration loop (skeleton for this PR, implemented in PR 8)
func (c *MigrationController) Run(ctx context.Context) error {
	c.logger.Info("Run: Starting migration orchestration")

	// TODO: Phase implementations added in PR 8
	// This is a skeleton implementation that will be expanded in the next PR

	// Get current state from store
	status, err := c.store.GetStatus()
	if err != nil {
		c.logger.WithError(err).Error("Run: Failed to get status")
		return fmt.Errorf("get status: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"state": status.State,
		"phase": status.Phase,
	}).Debug("Run: Current migration state")

	// If InProgress, resume from current phase
	if status.State == types.MigrationStateInProgress {
		c.logger.WithField("phase", status.Phase).Info("Run: Resuming from current phase")
		// TODO: Resume logic in PR 8
	}

	// Execute phases: Discovery → Preparation → ECInstall → DataTransfer → Completed
	phases := []types.MigrationPhase{
		types.MigrationPhaseDiscovery,
		types.MigrationPhasePreparation,
		types.MigrationPhaseECInstall,
		types.MigrationPhaseDataTransfer,
		types.MigrationPhaseCompleted,
	}

	for _, phase := range phases {
		c.logger.WithField("phase", phase).Info("Run: Executing phase (skeleton)")

		// Set state to InProgress
		if err := c.store.SetState(types.MigrationStateInProgress); err != nil {
			c.logger.WithError(err).Error("Run: Failed to set state to InProgress")
			if setErr := c.store.SetState(types.MigrationStateFailed); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set state to Failed")
			}
			if setErr := c.store.SetError(err.Error()); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set error message")
			}
			return fmt.Errorf("set state: %w", err)
		}

		// Set current phase
		if err := c.store.SetPhase(phase); err != nil {
			c.logger.WithError(err).Error("Run: Failed to set phase")
			if setErr := c.store.SetState(types.MigrationStateFailed); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set state to Failed")
			}
			if setErr := c.store.SetError(err.Error()); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set error message")
			}
			return fmt.Errorf("set phase: %w", err)
		}

		// Execute phase
		if err := c.manager.ExecutePhase(ctx, phase); err != nil {
			c.logger.WithError(err).WithField("phase", phase).Error("Run: Phase execution failed")
			if setErr := c.store.SetState(types.MigrationStateFailed); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set state to Failed")
			}
			if setErr := c.store.SetError(err.Error()); setErr != nil {
				c.logger.WithError(setErr).Error("Run: Failed to set error message")
			}
			return fmt.Errorf("execute phase %s: %w", phase, err)
		}
	}

	// Set state to Completed
	if err := c.store.SetState(types.MigrationStateCompleted); err != nil {
		c.logger.WithError(err).Error("Run: Failed to set state to Completed")
		return fmt.Errorf("set completed state: %w", err)
	}

	c.logger.Info("Run: Migration orchestration completed (skeleton)")
	return nil
}
