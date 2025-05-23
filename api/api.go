package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type API struct {
	installController install.Controller
	authController    auth.Controller
	configChan        chan<- *types.InstallationConfig
	logger            logrus.FieldLogger
}

type APIOption func(*API)

func WithInstallController(installController install.Controller) APIOption {
	return func(a *API) {
		a.installController = installController
	}
}

func WithAuthController(authController auth.Controller) APIOption {
	return func(a *API) {
		a.authController = authController
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

	if api.installController == nil {
		installController, err := install.NewInstallController()
		if err != nil {
			return nil, fmt.Errorf("new install controller: %w", err)
		}
		api.installController = installController
	}

	if api.authController == nil {
		authController, err := auth.NewAuthController(password)
		if err != nil {
			return nil, fmt.Errorf("new auth controller: %w", err)
		}
		api.authController = authController
	}

	if api.logger == nil {
		api.logger = NewDiscardLogger()
	}

	return api, nil
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.getHealth).Methods("GET")

	router.HandleFunc("/auth/login", a.postAuthLogin).Methods("POST")

	authenticatedRouter := router.PathPrefix("").Subrouter()
	authenticatedRouter.Use(a.authMiddleware)

	installRouter := authenticatedRouter.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("", a.getInstall).Methods("GET")
	installRouter.HandleFunc("/phase/set-config", a.setInstallConfig).Methods("POST")
	installRouter.HandleFunc("/set-status", a.setInstallStatus).Methods("POST")
}

func handleError(w http.ResponseWriter, err error) {
	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		apiErr = types.NewInternalServerError(err)
	}
	apiErr.JSON(w)
}
