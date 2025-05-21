package api

import (
	"fmt"
	"net/http"
)

func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Health check")
	w.WriteHeader(http.StatusOK)
}
