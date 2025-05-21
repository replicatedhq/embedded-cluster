package console

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/sirupsen/logrus"
)

type API struct {
	cfg    configStore
	logger logrus.FieldLogger
}

func NewAPI(logger logrus.FieldLogger) *API {
	if logger == nil {
		logger = api.NewDiscardLogger()
	}
	return &API{
		cfg:    &configMemoryStore{},
		logger: logger,
	}
}

func (c *API) RegisterRoutes(router *mux.Router) {
	router.Handle("/health", http.HandlerFunc(c.getHealth)).Methods(http.MethodGet)

	router.Handle("/config", http.HandlerFunc(c.getConfig)).Methods(http.MethodGet)
	router.Handle("/config", http.HandlerFunc(c.putConfig)).Methods(http.MethodPut)
}

func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (a *API) getConfig(w http.ResponseWriter, r *http.Request) {
	config, err := a.cfg.read()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := configSetDefaults(a.logger, config); err != nil {
		a.logger.Errorf("unable to set defaults: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := validateConfig(*config); err != nil {
		// this should never happen
		a.logger.Errorf("unable to validate config: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(config)
}

func (a *API) putConfig(w http.ResponseWriter, r *http.Request) {
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logger.Errorf("unable to decode config: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateConfig(config); err != nil {
		a.logger.Errorf("unable to validate config: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := configSetDefaults(a.logger, &config); err != nil {
		a.logger.Errorf("unable to set defaults: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := validateConfig(config); err != nil {
		// this should never happen
		a.logger.Errorf("unable to validate config: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := a.cfg.write(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.getConfig(w, r)
}
