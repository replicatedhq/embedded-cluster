package api

import (
	"fmt"

	kurlmigration "github.com/replicatedhq/embedded-cluster/api/controllers/kurlmigration"
	authhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/auth"
	consolehandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/console"
	healthhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/health"
	kuberneteshandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kubernetes"
	kurlmigrationhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kurlmigration"
	linuxhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/linux"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type handlers struct {
	auth          *authhandler.Handler
	console       *consolehandler.Handler
	health        *healthhandler.Handler
	linux         *linuxhandler.Handler
	kubernetes    *kuberneteshandler.Handler
	kurlmigration *kurlmigrationhandler.Handler
}

func (a *API) initHandlers() error {
	// Auth handler
	authHandler, err := authhandler.New(
		a.cfg.PasswordHash,
		authhandler.WithLogger(a.logger),
		authhandler.WithAuthController(a.authController),
	)
	if err != nil {
		return fmt.Errorf("new auth handler: %w", err)
	}
	a.handlers.auth = authHandler

	// Console handler
	consoleHandler, err := consolehandler.New(
		consolehandler.WithLogger(a.logger),
		consolehandler.WithConsoleController(a.consoleController),
	)
	if err != nil {
		return fmt.Errorf("new console handler: %w", err)
	}
	a.handlers.console = consoleHandler

	// Health handler
	healthHandler, err := healthhandler.New(
		healthhandler.WithLogger(a.logger),
	)
	if err != nil {
		return fmt.Errorf("new health handler: %w", err)
	}
	a.handlers.health = healthHandler

	if a.cfg.InstallTarget == types.InstallTargetLinux {
		// Linux handler
		linuxHandler, err := linuxhandler.New(
			a.cfg,
			linuxhandler.WithLogger(a.logger),
			linuxhandler.WithMetricsReporter(a.metricsReporter),
			linuxhandler.WithInstallController(a.linuxInstallController),
			linuxhandler.WithUpgradeController(a.linuxUpgradeController),
			linuxhandler.WithHelmClient(a.hcli),
			linuxhandler.WithKubeClient(a.kcli),
			linuxhandler.WithMetadataClient(a.mcli),
			linuxhandler.WithPreflightRunner(a.preflightRunner),
		)
		if err != nil {
			return fmt.Errorf("new linux handler: %w", err)
		}
		a.handlers.linux = linuxHandler

		// Initialize kURL migration controller if not already set
		if a.kurlMigrationController == nil {
			// Create installation manager for kURL migration
			installMgr := linuxinstallation.NewInstallationManager(
				linuxinstallation.WithLogger(a.logger),
			)

			// Controller creates manager internally with store passed as dependency
			kurlMigrationController, err := kurlmigration.NewKURLMigrationController(
				kurlmigration.WithLogger(a.logger),
				kurlmigration.WithInstallationManager(installMgr),
			)
			if err != nil {
				return fmt.Errorf("create kurl migration controller: %w", err)
			}
			a.kurlMigrationController = kurlMigrationController
		}

		// kURL Migration handler (Linux only)
		kurlMigrationHandler := kurlmigrationhandler.New(
			kurlmigrationhandler.WithLogger(a.logger),
			kurlmigrationhandler.WithController(a.kurlMigrationController),
		)
		a.handlers.kurlmigration = kurlMigrationHandler
	} else {
		// Kubernetes handler
		kubernetesHandler, err := kuberneteshandler.New(
			a.cfg,
			kuberneteshandler.WithLogger(a.logger),
			kuberneteshandler.WithInstallController(a.kubernetesInstallController),
			kuberneteshandler.WithUpgradeController(a.kubernetesUpgradeController),
			kuberneteshandler.WithHelmClient(a.hcli),
			kuberneteshandler.WithKubeClient(a.kcli),
			kuberneteshandler.WithMetadataClient(a.mcli),
			kuberneteshandler.WithPreflightRunner(a.preflightRunner),
		)
		if err != nil {
			return fmt.Errorf("new kubernetes handler: %w", err)
		}
		a.handlers.kubernetes = kubernetesHandler
	}

	return nil
}
