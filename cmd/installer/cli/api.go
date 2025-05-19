package cli

import (
	"context"
	"fmt"
	"net/http"
)

type API struct {
	Port int
}

func NewAPI(ctx context.Context, port int) (*API, error) {
	return &API{Port: port}, nil
}

func (a *API) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World!"))
	})
	return http.ListenAndServe(fmt.Sprintf(":%d", a.Port), mux)
}
