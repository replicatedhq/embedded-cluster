package api

import (
	"fmt"

	authhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/auth"
	consolehandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/console"
	healthhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/health"
	kuberneteshandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/kubernetes"
	linuxhandler "github.com/replicatedhq/embedded-cluster/api/internal/handlers/linux"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

type handlers struct {
	auth       *authhandler.Handler
	console    *consolehandler.Handler
	health     *healthhandler.Handler
	linux      *linuxhandler.Handler
	kubernetes *kuberneteshandler.Handler
}

func (a *API) initHandlers() error {
	// Auth handler
	authHandler, err := authhandler.New(
		a.cfg.Password,
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

	switch a.cfg.InstallTarget {
	case apitypes.InstallTargetLinux:
		linuxHandler, err := linuxhandler.New(
			a.cfg,
			linuxhandler.WithLogger(a.logger),
			linuxhandler.WithMetricsReporter(a.metricsReporter),
			linuxhandler.WithInstallController(a.linuxInstallController),
		)
		if err != nil {
			return fmt.Errorf("new linux handler: %w", err)
		}
		a.handlers.linux = linuxHandler

	case apitypes.InstallTargetKubernetes:
		kubernetesHandler, err := kuberneteshandler.New(
			a.cfg,
			kuberneteshandler.WithLogger(a.logger),
			kuberneteshandler.WithInstallController(a.kubernetesInstallController),
		)
		if err != nil {
			return fmt.Errorf("new kubernetes handler: %w", err)
		}
		a.handlers.kubernetes = kubernetesHandler
	}

	return nil
}
