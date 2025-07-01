package api

import (
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

// API represents the main HTTP API server for the Embedded Cluster application.
//
//	@title			Embedded Cluster API
//	@version		0.1
//	@description	This is the API for the Embedded Cluster project.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	API Support
//	@contact.url	https://github.com/replicatedhq/embedded-cluster/issues
//	@contact.email	support@replicated.com

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath	/api

//	@securityDefinitions.bearerauth	bearerauth

// @externalDocs.description	OpenAPI
// @externalDocs.url			https://swagger.io/resources/open-api/
type API struct {
	cfg types.APIConfig

	logger          logrus.FieldLogger
	metricsReporter metrics.ReporterInterface

	authController         auth.Controller
	consoleController      console.Controller
	linuxInstallController linuxinstall.Controller

	handlers handlers
}

// Option is a function that configures the API.
type Option func(*API)

// WithAuthController configures the auth controller for the API.
func WithAuthController(authController auth.Controller) Option {
	return func(a *API) {
		a.authController = authController
	}
}

// WithConsoleController configures the console controller for the API.
func WithConsoleController(consoleController console.Controller) Option {
	return func(a *API) {
		a.consoleController = consoleController
	}
}

// /testingggg
// WithLinuxInstallController configures the linux install controller for the API.
func WithLinuxInstallController(linuxInstallController linuxinstall.Controller) Option {
	return func(a *API) {
		a.linuxInstallController = linuxInstallController
	}
}

// WithLogger configures the logger for the API. If not provided, a default logger will be created.
func WithLogger(logger logrus.FieldLogger) Option {
	return func(a *API) {
		a.logger = logger
	}
}

// WithMetricsReporter configures the metrics reporter for the API.
func WithMetricsReporter(metricsReporter metrics.ReporterInterface) Option {
	return func(a *API) {
		a.metricsReporter = metricsReporter
	}
}

// New creates a new API instance.
func New(cfg types.APIConfig, opts ...Option) (*API, error) {
	api := &API{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(api)
	}

	if api.cfg.RuntimeConfig == nil {
		api.cfg.RuntimeConfig = runtimeconfig.New(nil)
	}

	if api.logger == nil {
		l, err := logger.NewLogger()
		if err != nil {
			return nil, fmt.Errorf("create logger: %w", err)
		}
		api.logger = l
	}

	if err := api.initHandlers(); err != nil {
		return nil, fmt.Errorf("init handlers: %w", err)
	}

	return api, nil
}
