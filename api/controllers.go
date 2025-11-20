package api

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/migration"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	migrationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/migration"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// initControllers initializes controllers that weren't provided via options
func (a *API) initControllers() error {
	// Initialize migration controller for Linux target if not already set
	if a.cfg.InstallTarget == types.InstallTargetLinux && a.migrationController == nil {
		installMgr := linuxinstallation.NewInstallationManager(
			linuxinstallation.WithLogger(a.logger),
		)
		mgr := migrationmanager.NewManager(
			migrationmanager.WithLogger(a.logger),
			migrationmanager.WithInstallationManager(installMgr),
		)
		controller, err := migration.NewMigrationController(
			migration.WithLogger(a.logger),
			migration.WithManager(mgr),
			migration.WithInstallationManager(installMgr),
		)
		if err != nil {
			return fmt.Errorf("create migration controller: %w", err)
		}
		a.migrationController = controller
	}

	return nil
}
