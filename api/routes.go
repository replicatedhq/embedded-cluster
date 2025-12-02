package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api/docs"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
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

	// Routes with logging middleware
	routerWithLogging := router.PathPrefix("/").Subrouter()
	routerWithLogging.Use(logger.HTTPLoggingMiddleware(a.logger))

	routerWithLogging.HandleFunc("/auth/login", a.handlers.auth.PostLogin).Methods("POST")

	authenticatedRouter := routerWithLogging.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(a.handlers.auth.Middleware)

	if a.cfg.InstallTarget == types.InstallTargetLinux {
		a.registerLinuxRoutes(authenticatedRouter)
	} else {
		a.registerKubernetesRoutes(authenticatedRouter)
	}

	a.registerConsoleRoutes(authenticatedRouter)
}

func (a *API) registerLinuxRoutes(router *mux.Router) {
	linuxRouter := router.PathPrefix("/linux").Subrouter()

	// kURL Migration routes (not mode-specific)
	a.registerKURLMigrationRoutes(linuxRouter)

	if a.cfg.Mode == types.ModeInstall {
		installRouter := linuxRouter.PathPrefix("/install").Subrouter()
		installRouter.HandleFunc("/installation/config", a.handlers.linux.Install.GetInstallationConfig).Methods("GET")
		installRouter.HandleFunc("/installation/configure", a.handlers.linux.Install.PostConfigureInstallation).Methods("POST")
		installRouter.HandleFunc("/installation/status", a.handlers.linux.Install.GetInstallationStatus).Methods("GET")

		installRouter.HandleFunc("/host-preflights/run", a.handlers.linux.Install.PostRunHostPreflights).Methods("POST")
		installRouter.HandleFunc("/host-preflights/status", a.handlers.linux.Install.GetHostPreflightsStatus).Methods("GET")

		installRouter.HandleFunc("/app-preflights/run", a.handlers.linux.Install.PostRunAppPreflights).Methods("POST")
		installRouter.HandleFunc("/app-preflights/status", a.handlers.linux.Install.GetAppPreflightsStatus).Methods("GET")

		installRouter.HandleFunc("/infra/setup", a.handlers.linux.Install.PostSetupInfra).Methods("POST")
		installRouter.HandleFunc("/infra/status", a.handlers.linux.Install.GetInfraStatus).Methods("GET")

		installRouter.HandleFunc("/airgap/process", a.handlers.linux.Install.PostProcessAirgap).Methods("POST")
		installRouter.HandleFunc("/airgap/status", a.handlers.linux.Install.GetAirgapStatus).Methods("GET")

		installRouter.HandleFunc("/app/config/template", a.handlers.linux.Install.PostTemplateAppConfig).Methods("POST")
		installRouter.HandleFunc("/app/config/values", a.handlers.linux.Install.GetAppConfigValues).Methods("GET")
		installRouter.HandleFunc("/app/config/values", a.handlers.linux.Install.PatchAppConfigValues).Methods("PATCH")

		installRouter.HandleFunc("/app/install", a.handlers.linux.Install.PostInstallApp).Methods("POST")
		installRouter.HandleFunc("/app/status", a.handlers.linux.Install.GetAppInstallStatus).Methods("GET")
	}

	if a.cfg.Mode == types.ModeUpgrade {
		upgradeRouter := linuxRouter.PathPrefix("/upgrade").Subrouter()
		upgradeRouter.HandleFunc("/app/config/template", a.handlers.linux.Upgrade.PostTemplateAppConfig).Methods("POST")
		upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.Upgrade.GetAppConfigValues).Methods("GET")
		upgradeRouter.HandleFunc("/app/config/values", a.handlers.linux.Upgrade.PatchAppConfigValues).Methods("PATCH")

		upgradeRouter.HandleFunc("/app-preflights/run", a.handlers.linux.Upgrade.PostRunAppPreflights).Methods("POST")
		upgradeRouter.HandleFunc("/app-preflights/status", a.handlers.linux.Upgrade.GetAppPreflightsStatus).Methods("GET")

		upgradeRouter.HandleFunc("/infra/upgrade", a.handlers.linux.Upgrade.PostUpgradeInfra).Methods("POST")
		upgradeRouter.HandleFunc("/infra/status", a.handlers.linux.Upgrade.GetInfraStatus).Methods("GET")

		upgradeRouter.HandleFunc("/airgap/process", a.handlers.linux.Upgrade.PostProcessAirgap).Methods("POST")
		upgradeRouter.HandleFunc("/airgap/status", a.handlers.linux.Upgrade.GetAirgapStatus).Methods("GET")

		upgradeRouter.HandleFunc("/app/upgrade", a.handlers.linux.Upgrade.PostUpgradeApp).Methods("POST")
		upgradeRouter.HandleFunc("/app/status", a.handlers.linux.Upgrade.GetAppUpgradeStatus).Methods("GET")
	}
}

func (a *API) registerKubernetesRoutes(router *mux.Router) {
	kubernetesRouter := router.PathPrefix("/kubernetes").Subrouter()

	if a.cfg.Mode == types.ModeInstall {
		installRouter := kubernetesRouter.PathPrefix("/install").Subrouter()
		installRouter.HandleFunc("/installation/config", a.handlers.kubernetes.Install.GetInstallationConfig).Methods("GET")
		installRouter.HandleFunc("/installation/configure", a.handlers.kubernetes.Install.PostConfigureInstallation).Methods("POST")
		installRouter.HandleFunc("/installation/status", a.handlers.kubernetes.Install.GetInstallationStatus).Methods("GET")

		installRouter.HandleFunc("/app-preflights/run", a.handlers.kubernetes.Install.PostRunAppPreflights).Methods("POST")
		installRouter.HandleFunc("/app-preflights/status", a.handlers.kubernetes.Install.GetAppPreflightsStatus).Methods("GET")

		installRouter.HandleFunc("/infra/setup", a.handlers.kubernetes.Install.PostSetupInfra).Methods("POST")
		installRouter.HandleFunc("/infra/status", a.handlers.kubernetes.Install.GetInfraStatus).Methods("GET")

		installRouter.HandleFunc("/app/config/template", a.handlers.kubernetes.Install.PostTemplateAppConfig).Methods("POST")
		installRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.Install.GetAppConfigValues).Methods("GET")
		installRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.Install.PatchAppConfigValues).Methods("PATCH")

		installRouter.HandleFunc("/app/install", a.handlers.kubernetes.Install.PostInstallApp).Methods("POST")
		installRouter.HandleFunc("/app/status", a.handlers.kubernetes.Install.GetAppInstallStatus).Methods("GET")
	}

	if a.cfg.Mode == types.ModeUpgrade {
		upgradeRouter := kubernetesRouter.PathPrefix("/upgrade").Subrouter()
		upgradeRouter.HandleFunc("/app/config/template", a.handlers.kubernetes.Upgrade.PostTemplateAppConfig).Methods("POST")
		upgradeRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.Upgrade.GetAppConfigValues).Methods("GET")
		upgradeRouter.HandleFunc("/app/config/values", a.handlers.kubernetes.Upgrade.PatchAppConfigValues).Methods("PATCH")

		upgradeRouter.HandleFunc("/app-preflights/run", a.handlers.kubernetes.Upgrade.PostRunAppPreflights).Methods("POST")
		upgradeRouter.HandleFunc("/app-preflights/status", a.handlers.kubernetes.Upgrade.GetAppPreflightsStatus).Methods("GET")

		upgradeRouter.HandleFunc("/app/upgrade", a.handlers.kubernetes.Upgrade.PostUpgradeApp).Methods("POST")
		upgradeRouter.HandleFunc("/app/status", a.handlers.kubernetes.Upgrade.GetAppUpgradeStatus).Methods("GET")
	}
}

func (a *API) registerConsoleRoutes(router *mux.Router) {
	consoleRouter := router.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.handlers.console.GetListAvailableNetworkInterfaces).Methods("GET")
}

func (a *API) registerKURLMigrationRoutes(router *mux.Router) {
	kurlMigrationRouter := router.PathPrefix("/kurl-migration").Subrouter()
	kurlMigrationRouter.HandleFunc("/config", a.handlers.kurlmigration.GetInstallationConfig).Methods("GET")
	kurlMigrationRouter.HandleFunc("/start", a.handlers.kurlmigration.PostStartMigration).Methods("POST")
	kurlMigrationRouter.HandleFunc("/status", a.handlers.kurlmigration.GetMigrationStatus).Methods("GET")
}
