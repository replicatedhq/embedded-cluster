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
type api struct {
	cfg types.APIConfig

	logger          logrus.FieldLogger
	metricsReporter metrics.ReporterInterface

	authController         auth.Controller
	consoleController      console.Controller
	linuxInstallController linuxinstall.Controller

	handlers handlers
}

type apiOption func(*api)

func WithAuthController(authController auth.Controller) apiOption {
	return func(a *api) {
		a.authController = authController
	}
}

func WithConsoleController(consoleController console.Controller) apiOption {
	return func(a *api) {
		a.consoleController = consoleController
	}
}

func WithLinuxInstallController(linuxInstallController linuxinstall.Controller) apiOption {
	return func(a *api) {
		a.linuxInstallController = linuxInstallController
	}
}

func WithLogger(logger logrus.FieldLogger) apiOption {
	return func(a *api) {
		a.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) apiOption {
	return func(a *api) {
		a.metricsReporter = metricsReporter
	}
}

func New(cfg types.APIConfig, opts ...apiOption) (*api, error) {
	api := &api{
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
