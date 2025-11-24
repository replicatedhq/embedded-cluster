package kurlmigration

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	kurlmigrationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/kurlmigration"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	kurlmigrationstore "github.com/replicatedhq/embedded-cluster/api/internal/store/kurlmigration"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

// Controller provides methods for managing kURL to Embedded Cluster migrations
type Controller interface {
	// GetInstallationConfig extracts kURL config, gets EC defaults, and returns merged config
	GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error)

	// StartKURLMigration validates config, generates UUID, initializes state, launches background goroutine
	StartKURLMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error)

	// GetKURLMigrationStatus returns current migration status
	GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error)

	// Run is the internal orchestration loop (skeleton for this PR, implemented in PR 8)
	Run(ctx context.Context) error
}

var _ Controller = (*KURLMigrationController)(nil)

// KURLMigrationController implements the Controller interface
type KURLMigrationController struct {
	manager             kurlmigrationmanager.Manager
	store               kurlmigrationstore.Store
	installationManager linuxinstallation.InstallationManager
	logger              logrus.FieldLogger
}

// ControllerOption is a functional option for configuring the KURLMigrationController
type ControllerOption func(*KURLMigrationController)

// WithManager sets the migration manager
func WithManager(manager kurlmigrationmanager.Manager) ControllerOption {
	return func(c *KURLMigrationController) {
		c.manager = manager
	}
}

// WithStore sets the migration store
func WithStore(store kurlmigrationstore.Store) ControllerOption {
	return func(c *KURLMigrationController) {
		c.store = store
	}
}

// WithInstallationManager sets the installation manager
func WithInstallationManager(manager linuxinstallation.InstallationManager) ControllerOption {
	return func(c *KURLMigrationController) {
		c.installationManager = manager
	}
}

// WithLogger sets the logger
func WithLogger(log logrus.FieldLogger) ControllerOption {
	return func(c *KURLMigrationController) {
		c.logger = log
	}
}

// NewKURLMigrationController creates a new migration controller with the provided options.
// The controller creates its manager and installation manager internally if not provided via options.
func NewKURLMigrationController(opts ...ControllerOption) (*KURLMigrationController, error) {
	controller := &KURLMigrationController{
		store:  kurlmigrationstore.NewMemoryStore(),
		logger: logger.NewDiscardLogger(),
	}

	for _, opt := range opts {
		opt(controller)
	}

	// Create installation manager internally if not provided via option
	if controller.installationManager == nil {
		controller.installationManager = linuxinstallation.NewInstallationManager(
			linuxinstallation.WithLogger(controller.logger),
		)
	}

	// Create manager internally if not provided via option
	if controller.manager == nil {
		controller.manager = kurlmigrationmanager.NewManager(
			kurlmigrationmanager.WithStore(controller.store),
			kurlmigrationmanager.WithLogger(controller.logger),
			kurlmigrationmanager.WithInstallationManager(controller.installationManager),
		)
	}

	return controller, nil
}

// GetInstallationConfig extracts kURL config, gets EC defaults, and returns merged config
func (c *KURLMigrationController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	c.logger.Debug("fetching kurl config, ec defaults, and merging")

	// Get kURL config from the running cluster
	kurlConfig, err := c.manager.GetKurlConfig(ctx)
	if err != nil {
		c.logger.WithError(err).Error("get kurl config")
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get kurl config: %w", err)
	}

	// Get EC defaults
	defaults, err := c.manager.GetECDefaults(ctx)
	if err != nil {
		c.logger.WithError(err).Error("get ec defaults")
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get ec defaults: %w", err)
	}

	// Get user config from store (will be empty if user hasn't submitted config yet)
	userConfig, err := c.store.GetUserConfig()
	if err != nil {
		c.logger.WithError(err).Error("get user config")
		return types.LinuxInstallationConfigResponse{}, fmt.Errorf("get user config: %w", err)
	}

	// Merge configs: userConfig > kurlConfig > defaults
	resolved := c.manager.MergeConfigs(userConfig, kurlConfig, defaults)

	c.logger.WithFields(logrus.Fields{
		"kurlConfig": kurlConfig,
		"defaults":   defaults,
		"resolved":   resolved,
	}).Debug("config merged successfully")

	return types.LinuxInstallationConfigResponse{
		Values:   kurlConfig,
		Defaults: defaults,
		Resolved: resolved,
	}, nil
}

// StartKURLMigration validates config, generates UUID, initializes state, launches background goroutine
func (c *KURLMigrationController) StartKURLMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"transferMode": transferMode,
		"config":       config,
	}).Info("starting migration")

	// Validate transfer mode
	if err := c.manager.ValidateTransferMode(transferMode); err != nil {
		c.logger.WithError(err).Error("invalid transfer mode")
		return "", types.NewBadRequestError(err)
	}

	// Check if migration already exists
	if _, err := c.store.GetMigrationID(); err == nil {
		c.logger.Warn("migration already in progress")
		return "", types.NewConflictError(types.ErrKURLMigrationAlreadyStarted)
	}

	// Generate UUID for migration
	migrationID := uuid.New().String()
	c.logger.WithField("migrationID", migrationID).Debug("generated migration id")

	// Get defaults and merge with user config
	kurlConfig, err := c.manager.GetKurlConfig(ctx)
	if err != nil {
		c.logger.WithError(err).Error("get kurl config")
		return "", fmt.Errorf("get kurl config: %w", err)
	}

	defaults, err := c.manager.GetECDefaults(ctx)
	if err != nil {
		c.logger.WithError(err).Error("get ec defaults")
		return "", fmt.Errorf("get ec defaults: %w", err)
	}

	resolvedConfig := c.manager.MergeConfigs(config, kurlConfig, defaults)
	c.logger.WithField("resolvedConfig", resolvedConfig).Debug("config merged")

	// Store user-provided config for future reference
	if err := c.store.SetUserConfig(config); err != nil {
		c.logger.WithError(err).Error("store user config")
		return "", fmt.Errorf("store user config: %w", err)
	}

	// Initialize migration in store with resolved config
	if err := c.store.InitializeMigration(migrationID, string(transferMode), resolvedConfig); err != nil {
		c.logger.WithError(err).Error("initialize migration")
		return "", fmt.Errorf("initialize migration: %w", err)
	}

	// Set initial state to NotStarted
	if err := c.store.SetState(types.KURLMigrationStateNotStarted); err != nil {
		c.logger.WithError(err).Error("set initial state")
		return "", fmt.Errorf("set initial state: %w", err)
	}

	c.logger.WithField("migrationID", migrationID).Info("migration initialized, launching background goroutine")

	// Launch background goroutine with detached context
	// We use WithoutCancel to inherit context values (trace IDs, logger fields)
	// but detach from the request's cancellation so migration continues after HTTP response
	backgroundCtx := context.WithoutCancel(ctx)
	go func() {
		if err := c.Run(backgroundCtx); err != nil {
			c.logger.WithError(err).Error("background migration failed")
		}
	}()

	return migrationID, nil
}

// GetKURLMigrationStatus returns current migration status
func (c *KURLMigrationController) GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error) {
	c.logger.Debug("fetching migration status")

	status, err := c.store.GetStatus()
	if err != nil {
		if err == types.ErrNoActiveKURLMigration {
			c.logger.Warn("no active migration found")
			return types.KURLMigrationStatusResponse{}, types.NewNotFoundError(err)
		}
		c.logger.WithError(err).Error("get status")
		return types.KURLMigrationStatusResponse{}, fmt.Errorf("get status: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"state":    status.State,
		"phase":    status.Phase,
		"progress": status.Progress,
	}).Debug("status retrieved")

	return status, nil
}

// Run is the internal orchestration loop (skeleton for this PR, implemented in PR 8)
func (c *KURLMigrationController) Run(ctx context.Context) (finalErr error) {
	c.logger.Info("starting migration orchestration")

	// Small delay to ensure HTTP response completes before any state changes
	// This prevents race condition where migration could fail and update state
	// before the client receives the success response with migrationID
	time.Sleep(100 * time.Millisecond)

	// Defer handles all error cases by updating migration state
	defer func() {
		// Recover from panics
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic in kurl migration orchestration: %v", r)
			c.logger.WithField("panic", r).Error("migration panicked")
		}

		// Handle any error by updating state to Failed
		if finalErr != nil {
			if err := c.store.SetState(types.KURLMigrationStateFailed); err != nil {
				c.logger.WithError(err).Error("failed to set migration state to failed")
			}
			if err := c.store.SetError(finalErr.Error()); err != nil {
				c.logger.WithError(err).Error("failed to set migration error message")
			}
		}
	}()

	// TODO(sc-130983): Phase implementations will be added in the orchestration story
	// This is a skeleton implementation that will be expanded in that PR

	// Get current state from store
	status, err := c.store.GetStatus()
	if err != nil {
		c.logger.WithError(err).Error("get status")
		return fmt.Errorf("get status: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"state": status.State,
		"phase": status.Phase,
	}).Debug("current migration state")

	// If InProgress, resume from current phase
	if status.State == types.KURLMigrationStateInProgress {
		c.logger.WithField("phase", status.Phase).Info("resuming from current phase")
		// TODO(sc-130983): Resume logic will be implemented in the orchestration story
	}

	// Execute phases: Discovery → Preparation → ECInstall → DataTransfer → Completed
	phases := []types.KURLMigrationPhase{
		types.KURLMigrationPhaseDiscovery,
		types.KURLMigrationPhasePreparation,
		types.KURLMigrationPhaseECInstall,
		types.KURLMigrationPhaseDataTransfer,
		types.KURLMigrationPhaseCompleted,
	}

	for _, phase := range phases {
		c.logger.WithField("phase", phase).Info("executing phase (skeleton)")

		// Set state to InProgress
		if err := c.store.SetState(types.KURLMigrationStateInProgress); err != nil {
			c.logger.WithError(err).Error("set state to in progress")
			return fmt.Errorf("set state: %w", err)
		}

		// Set current phase
		if err := c.store.SetPhase(phase); err != nil {
			c.logger.WithError(err).Error("set phase")
			return fmt.Errorf("set phase: %w", err)
		}

		// Execute phase
		if err := c.manager.ExecutePhase(ctx, phase); err != nil {
			c.logger.WithError(err).WithField("phase", phase).Error("phase execution failed")
			return fmt.Errorf("execute phase %s: %w", phase, err)
		}
	}

	// Set state to Completed
	// Note: If this fails, we log it but don't return an error because the migration itself succeeded.
	// Returning an error here would cause the defer to mark the migration as Failed, which is incorrect
	// since all phases completed successfully. The state update failure is a separate concern.
	if err := c.store.SetState(types.KURLMigrationStateCompleted); err != nil {
		c.logger.WithError(err).Error("migration completed successfully but failed to update state to Completed")
		// Don't return error - migration succeeded even if state update failed
	} else {
		c.logger.Info("migration orchestration completed successfully")
	}

	return nil
}
