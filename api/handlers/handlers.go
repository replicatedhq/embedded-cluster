package handlers

import (
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	authhandler "github.com/replicatedhq/embedded-cluster/api/handlers/auth"
	consolehandler "github.com/replicatedhq/embedded-cluster/api/handlers/console"
	healthhandler "github.com/replicatedhq/embedded-cluster/api/handlers/health"
	linuxhandler "github.com/replicatedhq/embedded-cluster/api/handlers/linux"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/sirupsen/logrus"
)

type Handlers struct {
	Auth    *authhandler.Handler
	Console *consolehandler.Handlers
	Health  *healthhandler.Handlers
	Linux   *linuxhandler.Handler

	authController         auth.Controller
	consoleController      console.Controller
	linuxInstallController linuxinstall.Controller

	logger          logrus.FieldLogger
	metricsReporter metrics.ReporterInterface
}

type Option func(*Handlers)

func WithAuthController(authController auth.Controller) Option {
	return func(h *Handlers) {
		h.authController = authController
	}
}

func WithConsoleController(consoleController console.Controller) Option {
	return func(h *Handlers) {
		h.consoleController = consoleController
	}
}

func WithLinuxInstallController(linuxInstallController linuxinstall.Controller) Option {
	return func(h *Handlers) {
		h.linuxInstallController = linuxInstallController
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handlers) {
		h.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) Option {
	return func(h *Handlers) {
		h.metricsReporter = metricsReporter
	}
}

func New(cfg types.APIConfig, opts ...Option) (*Handlers, error) {
	h := &Handlers{}

	for _, opt := range opts {
		opt(h)
	}

	if h.logger == nil {
		h.logger = logger.NewDiscardLogger()
	}

	// Auth handler
	authHandler, err := authhandler.New(
		cfg.Password,
		authhandler.WithLogger(h.logger),
		authhandler.WithAuthController(h.authController),
	)
	if err != nil {
		return nil, err
	}
	h.Auth = authHandler

	// Console handler
	consoleHandler, err := consolehandler.New(
		consolehandler.WithLogger(h.logger),
		consolehandler.WithConsoleController(h.consoleController),
	)
	if err != nil {
		return nil, err
	}
	h.Console = consoleHandler

	// Health handler
	healthHandler, err := healthhandler.New(
		healthhandler.WithLogger(h.logger),
	)
	if err != nil {
		return nil, err
	}
	h.Health = healthHandler

	// Linux handler
	linuxHandler, err := linuxhandler.New(
		cfg,
		linuxhandler.WithLogger(h.logger),
		linuxhandler.WithMetricsReporter(h.metricsReporter),
		linuxhandler.WithInstallController(h.linuxInstallController),
	)
	if err != nil {
		return nil, err
	}
	h.Linux = linuxHandler

	return h, nil
}
