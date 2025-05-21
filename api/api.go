package api

import (
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
)

type API struct {
	logger logrus.FieldLogger

	installController install.Controller
}

type APIOption func(*API)

func WithLogger(logger logrus.FieldLogger) APIOption {
	return func(a *API) {
		a.logger = logger
	}
}

func New(opts ...APIOption) *API {
	api := &API{
		installController: install.NewInstallController(),
	}

	for _, opt := range opts {
		opt(api)
	}

	if api.logger == nil {
		api.logger = NewDiscardLogger()
	}

	return api
}

func (a *API) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/health", a.getHealth).Methods("GET")

	installRouter := router.PathPrefix("/install").Subrouter()
	installRouter.HandleFunc("/", a.getInstall).Methods("GET")
	installRouter.HandleFunc("/phase/set-config", a.postInstallPhaseSetConfig).Methods("POST")
	installRouter.HandleFunc("/phase/start", a.postInstallPhaseStart).Methods("POST")
}
