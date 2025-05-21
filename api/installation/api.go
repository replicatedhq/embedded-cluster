package installation

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/sirupsen/logrus"
)

type API struct {
	logger logrus.FieldLogger
}

func NewAPI(logger logrus.FieldLogger) *API {
	if logger == nil {
		logger = api.NewDiscardLogger()
	}
	return &API{
		logger: logger,
	}
}

func (c *API) RegisterRoutes(router *mux.Router) {
	router.Handle("/health", http.HandlerFunc(c.getHealth)).Methods(http.MethodGet)
}

func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
