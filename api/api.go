package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/docs"
	"github.com/replicatedhq/embedded-cluster/api/handlers"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	httpSwagger "github.com/swaggo/http-swagger/v2"
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
type API struct {
	cfg types.APIConfig

	logger          logrus.FieldLogger
	metricsReporter metrics.ReporterInterface

	authController         auth.Controller
	consoleController      console.Controller
	linuxInstallController linuxinstall.Controller

	handlers *handlers.Handlers
}

type APIOption func(*API)

func WithAuthController(authController auth.Controller) APIOption {
	return func(a *API) {
		a.authController = authController
	}
}

func WithConsoleController(consoleController console.Controller) APIOption {
	return func(a *API) {
		a.consoleController = consoleController
	}
}

func WithLinuxInstallController(linuxInstallController linuxinstall.Controller) APIOption {
	return func(a *API) {
		a.linuxInstallController = linuxInstallController
	}
}

func WithLogger(logger logrus.FieldLogger) APIOption {
	return func(a *API) {
		a.logger = logger
	}
}

func WithMetricsReporter(metricsReporter metrics.ReporterInterface) APIOption {
	return func(a *API) {
		a.metricsReporter = metricsReporter
	}
}

func New(cfg types.APIConfig, opts ...APIOption) (*API, error) {
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

	handlers, err := handlers.New(
		api.cfg,
		handlers.WithAuthController(api.authController),
		handlers.WithConsoleController(api.consoleController),
		handlers.WithLinuxInstallController(api.linuxInstallController),
	)
	if err != nil {
		return nil, fmt.Errorf("new handlers: %w", err)
	}
	api.handlers = handlers

	return api, nil
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.handlers.Health.GetHealth).Methods("GET")

	// Hack to fix issue
	// https://github.com/swaggo/swag/issues/1588#issuecomment-2797801240
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	}).Methods("GET")
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	router.HandleFunc("/auth/login", a.handlers.Auth.PostLogin).Methods("POST")

	authenticatedRouter := router.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(a.handlers.Auth.Middleware)

	a.registerLinuxRoutes(authenticatedRouter)
	a.registerKubernetesRoutes(authenticatedRouter)
	a.registerConsoleRoutes(authenticatedRouter)
}

func (a *API) registerLinuxRoutes(router *mux.Router) {
	linuxRouter := router.PathPrefix("/linux").Subrouter()

	installRouter := linuxRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/installation/config", a.handlers.Linux.GetInstallationConfig).Methods("GET")
	installRouter.HandleFunc("/installation/configure", a.handlers.Linux.PostConfigureInstallation).Methods("POST")
	installRouter.HandleFunc("/installation/status", a.handlers.Linux.GetInstallationStatus).Methods("GET")

	installRouter.HandleFunc("/host-preflights/run", a.handlers.Linux.PostRunHostPreflights).Methods("POST")
	installRouter.HandleFunc("/host-preflights/status", a.handlers.Linux.GetHostPreflightsStatus).Methods("GET")

	installRouter.HandleFunc("/infra/setup", a.handlers.Linux.PostSetupInfra).Methods("POST")
	installRouter.HandleFunc("/infra/status", a.handlers.Linux.GetInfraStatus).Methods("GET")

	// TODO (@salah): remove this once the cli isn't responsible for setting the install status
	// and the ui isn't polling for it to know if the entire install is complete
	installRouter.HandleFunc("/status", a.handlers.Linux.GetStatus).Methods("GET")
	installRouter.HandleFunc("/status", a.handlers.Linux.PostSetStatus).Methods("POST")
}

func (a *API) registerKubernetesRoutes(router *mux.Router) {
	// kubernetesRouter := router.PathPrefix("/kubernetes").Subrouter()
}

func (a *API) registerConsoleRoutes(router *mux.Router) {
	consoleRouter := router.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.handlers.Console.GetListAvailableNetworkInterfaces).Methods("GET")
}
