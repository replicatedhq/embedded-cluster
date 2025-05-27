package api

import (
	"net/http"
)

func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
