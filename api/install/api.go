package install

import (
	"net/http"

	"github.com/gorilla/mux"
)

type API struct {
}

func NewAPI() *API {
	return &API{}
}

func (c *API) RegisterRoutes(router *mux.Router) {
	router.Handle("/health", http.HandlerFunc(c.getHealth)).Methods(http.MethodGet)
}

func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
