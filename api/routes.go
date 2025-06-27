package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api/docs"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// RegisterRoutes registers the routes for the API. A router is passed in to allow for the routes
// to be registered on a subrouter.
func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.handlers.health.GetHealth).Methods("GET")

	// Hack to fix issue
	// https://github.com/swaggo/swag/issues/1588#issuecomment-2797801240
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	}).Methods("GET")
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	router.HandleFunc("/auth/login", a.handlers.auth.PostLogin).Methods("POST")

	authenticatedRouter := router.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(a.handlers.auth.Middleware)

	a.registerLinuxRoutes(authenticatedRouter)
	a.registerKubernetesRoutes(authenticatedRouter)
	a.registerConsoleRoutes(authenticatedRouter)
}

func (a *API) registerLinuxRoutes(router *mux.Router) {
	linuxRouter := router.PathPrefix("/linux").Subrouter()

	installRouter := linuxRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/installation/config", a.handlers.linux.GetInstallationConfig).Methods("GET")
	installRouter.HandleFunc("/installation/configure", a.handlers.linux.PostConfigureInstallation).Methods("POST")
	installRouter.HandleFunc("/installation/status", a.handlers.linux.GetInstallationStatus).Methods("GET")

	installRouter.HandleFunc("/host-preflights/run", a.handlers.linux.PostRunHostPreflights).Methods("POST")
	installRouter.HandleFunc("/host-preflights/status", a.handlers.linux.GetHostPreflightsStatus).Methods("GET")

	installRouter.HandleFunc("/infra/setup", a.handlers.linux.PostSetupInfra).Methods("POST")
	installRouter.HandleFunc("/infra/status", a.handlers.linux.GetInfraStatus).Methods("GET")

	// TODO (@salah): remove this once the cli isn't responsible for setting the install status
	// and the ui isn't polling for it to know if the entire install is complete
	installRouter.HandleFunc("/status", a.handlers.linux.GetStatus).Methods("GET")
	installRouter.HandleFunc("/status", a.handlers.linux.PostSetStatus).Methods("POST")
}

func (a *API) registerKubernetesRoutes(router *mux.Router) {
	// kubernetesRouter := router.PathPrefix("/kubernetes").Subrouter()
}

func (a *API) registerConsoleRoutes(router *mux.Router) {
	consoleRouter := router.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.handlers.console.GetListAvailableNetworkInterfaces).Methods("GET")
}
