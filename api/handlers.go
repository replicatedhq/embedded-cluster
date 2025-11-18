package api

import (
	"fmt"

	authhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/auth"
	consolehandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/console"
	healthhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/health"
	kuberneteshandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kubernetes"
	linuxhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/linux"
	migrationhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/migration"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type handlers struct {
	auth       *authhandler.Handler
	console    *consolehandler.Handler
	health     *healthhandler.Handler
	linux      *linuxhandler.Handler
	kubernetes *kuberneteshandler.Handler
	migration  *migrationhandler.Handler
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
		)
		if err != nil {
			return fmt.Errorf("new linux handler: %w", err)
		}
		a.handlers.linux = linuxHandler

		// Migration handler (Linux only)
		migrationHandler := migrationhandler.New(
			migrationhandler.WithLogger(a.logger),
			migrationhandler.WithController(a.migrationController),
		)
		a.handlers.migration = migrationHandler
	} else {
		// Kubernetes handler
		kubernetesHandler, err := kuberneteshandler.New(
			a.cfg,
			kuberneteshandler.WithLogger(a.logger),
			kuberneteshandler.WithInstallController(a.kubernetesInstallController),
			kuberneteshandler.WithUpgradeController(a.kubernetesUpgradeController),
			kuberneteshandler.WithHelmClient(a.hcli),
		)
		if err != nil {
			return fmt.Errorf("new kubernetes handler: %w", err)
		}
		a.handlers.kubernetes = kubernetesHandler
	}

	return nil
}
