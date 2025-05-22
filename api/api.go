package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type API struct {
	installController install.Controller
	logger            logrus.FieldLogger
}

type APIOption func(*API)

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

func New(opts ...APIOption) (*API, error) {
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

	if api.logger == nil {
		api.logger = NewDiscardLogger()
	}

	return api, nil
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.getHealth).Methods("GET")

	installRouter := router.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/", a.getInstall).Methods("GET")
	installRouter.HandleFunc("/phase/set-config", a.postInstallPhaseSetConfig).Methods("POST")
	installRouter.HandleFunc("/phase/start", a.postInstallPhaseStart).Methods("POST")
}

func handleError(w http.ResponseWriter, err error) {
	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		apiErr = types.NewInternalServerError(err)
	}
	apiErr.JSON(w)
}
