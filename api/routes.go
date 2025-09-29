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

	installRouter.HandleFunc("/app-preflights/run", a.handlers.linux.PostRunAppPreflights).Methods("POST")
	installRouter.HandleFunc("/app-preflights/status", a.handlers.linux.GetAppPreflightsStatus).Methods("GET")

	installRouter.HandleFunc("/infra/setup", a.handlers.linux.PostSetupInfra).Methods("POST")
	installRouter.HandleFunc("/infra/status", a.handlers.linux.GetInfraStatus).Methods("GET")

	installRouter.HandleFunc("/app/config/template", a.handlers.linux.PostTemplateAppConfig).Methods("POST")
	installRouter.HandleFunc("/app/config/values", a.handlers.linux.GetAppConfigValues).Methods("GET")
	installRouter.HandleFunc("/app/config/values", a.handlers.linux.PatchAppConfigValues).Methods("PATCH")

	installRouter.HandleFunc("/app/install", a.handlers.linux.PostInstallApp).Methods("POST")
	installRouter.HandleFunc("/app/status", a.handlers.linux.GetAppInstallStatus).Methods("GET")

	upgradeRouter := linuxRouter.PathPrefix("/upgrade").Subrouter()
	upgradeRouter.HandleFunc("/app/config/template", a.handlers.linux.PostUpgradeTemplateAppConfig).Methods("POST")
	upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.GetUpgradeAppConfigValues).Methods("GET")
	upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.PatchUpgradeAppConfigValues).Methods("PATCH")

	upgradeRouter.HandleFunc("/app/upgrade", a.handlers.linux.PostUpgradeApp).Methods("POST")
	upgradeRouter.HandleFunc("/app/status", a.handlers.linux.GetAppUpgradeStatus).Methods("GET")
}

func (a *API) registerKubernetesRoutes(router *mux.Router) {
	kubernetesRouter := router.PathPrefix("/kubernetes").Subrouter()

	installRouter := kubernetesRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/installation/config", a.handlers.kubernetes.GetInstallationConfig).Methods("GET")
	installRouter.HandleFunc("/installation/configure", a.handlers.kubernetes.PostConfigureInstallation).Methods("POST")
	installRouter.HandleFunc("/installation/status", a.handlers.kubernetes.GetInstallationStatus).Methods("GET")

	installRouter.HandleFunc("/app-preflights/run", a.handlers.kubernetes.PostRunAppPreflights).Methods("POST")
	installRouter.HandleFunc("/app-preflights/status", a.handlers.kubernetes.GetAppPreflightsStatus).Methods("GET")

	installRouter.HandleFunc("/infra/setup", a.handlers.kubernetes.PostSetupInfra).Methods("POST")
	installRouter.HandleFunc("/infra/status", a.handlers.kubernetes.GetInfraStatus).Methods("GET")

	installRouter.HandleFunc("/app/config/template", a.handlers.kubernetes.PostTemplateAppConfig).Methods("POST")
	installRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.GetAppConfigValues).Methods("GET")
	installRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.PatchAppConfigValues).Methods("PATCH")

	installRouter.HandleFunc("/app/install", a.handlers.kubernetes.PostInstallApp).Methods("POST")
	installRouter.HandleFunc("/app/status", a.handlers.kubernetes.GetAppInstallStatus).Methods("GET")

	upgradeRouter := kubernetesRouter.PathPrefix("/upgrade").Subrouter()
	upgradeRouter.HandleFunc("/app/config/template", a.handlers.kubernetes.PostUpgradeTemplateAppConfig).Methods("POST")
	upgradeRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.GetUpgradeAppConfigValues).Methods("GET")
	upgradeRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.PatchUpgradeAppConfigValues).Methods("PATCH")

	upgradeRouter.HandleFunc("/app/upgrade", a.handlers.kubernetes.PostUpgradeApp).Methods("POST")
	upgradeRouter.HandleFunc("/app/status", a.handlers.kubernetes.GetAppUpgradeStatus).Methods("GET")
}

func (a *API) registerConsoleRoutes(router *mux.Router) {
	consoleRouter := router.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.handlers.console.GetListAvailableNetworkInterfaces).Methods("GET")
}
