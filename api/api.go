package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/docs"
	"github.com/replicatedhq/embedded-cluster/api/types"
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
	authController    auth.Controller
	consoleController console.Controller
	installController install.Controller
	configChan        chan<- *types.InstallationConfig
	logger            logrus.FieldLogger
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

func WithInstallController(installController install.Controller) APIOption {
	return func(a *API) {
		a.installController = installController
	}
}

func WithLogger(logger logrus.FieldLogger) APIOption {
	return func(a *API) {
		a.logger = logger
	}
}

func WithConfigChan(configChan chan<- *types.InstallationConfig) APIOption {
	return func(a *API) {
		a.configChan = configChan
	}
}

func New(password string, opts ...APIOption) (*API, error) {
	api := &API{}
	for _, opt := range opts {
		opt(api)
	}

	if api.authController == nil {
		authController, err := auth.NewAuthController(password)
		if err != nil {
			return nil, fmt.Errorf("new auth controller: %w", err)
		}
		api.authController = authController
	}

	if api.consoleController == nil {
		consoleController, err := console.NewConsoleController()
		if err != nil {
			return nil, fmt.Errorf("new console controller: %w", err)
		}
		api.consoleController = consoleController
	}

	if api.installController == nil {
		installController, err := install.NewInstallController()
		if err != nil {
			return nil, fmt.Errorf("new install controller: %w", err)
		}
		api.installController = installController
	}

	if api.logger == nil {
		api.logger = NewDiscardLogger()
	}

	return api, nil
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.getHealth).Methods("GET")

	// Hack to fix issue
	// https://github.com/swaggo/swag/issues/1588#issuecomment-2797801240
	router.HandleFunc("/swagger/doc.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(docs.SwaggerInfo.ReadDoc()))
	}).Methods("GET")
	router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	router.HandleFunc("/auth/login", a.postAuthLogin).Methods("POST")

	authenticatedRouter := router.PathPrefix("/").Subrouter()
	authenticatedRouter.Use(a.authMiddleware)

	installRouter := authenticatedRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("", a.getInstall).Methods("GET")
	installRouter.HandleFunc("/config", a.setInstallConfig).Methods("POST")
	installRouter.HandleFunc("/status", a.setInstallStatus).Methods("POST")
	installRouter.HandleFunc("/status", a.getInstallStatus).Methods("GET")

	consoleRouter := authenticatedRouter.PathPrefix("/console").Subrouter()
	consoleRouter.HandleFunc("/available-network-interfaces", a.getListAvailableNetworkInterfaces).Methods("GET")
}

func (a *API) json(w http.ResponseWriter, r *http.Request, code int, payload any) {
	response, err := json.Marshal(payload)
	if err != nil {
		a.logError(r, err, "failed to encode response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func (a *API) jsonError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		apiErr = types.NewInternalServerError(err)
	}

	response, err := json.Marshal(apiErr)
	if err != nil {
		a.logError(r, err, "failed to encode response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode)
	w.Write(response)
}

func (a *API) logError(r *http.Request, err error, args ...any) {
	a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).Error(args...)
}

func logrusFieldsFromRequest(r *http.Request) logrus.Fields {
	return logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}
}
