package console

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

type API struct {
	cfg    configStore
	logger *logrus.Logger
}

func NewAPI(logger *logrus.Logger) *API {
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

	if err := applyConfigToRuntimeConfig(config); err != nil {
		a.logger.Errorf("unable to apply config to runtime config: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	proxySpec, err := config.GetProxySpec()
	if err != nil {
		a.logger.Errorf("unable to get proxy spec: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := a.cfg.write(&config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := runtimeconfig.WriteToDisk(); err != nil {
		err = fmt.Errorf("unable to write runtime config to disk: %w", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	os.Setenv("KUBECONFIG", runtimeconfig.PathToKubeConfig())
	os.Setenv("TMPDIR", runtimeconfig.EmbeddedClusterTmpSubDir())

	if proxySpec != nil {
		if proxySpec.HTTPProxy != "" {
			os.Setenv("HTTP_PROXY", proxySpec.HTTPProxy)
		}
		if proxySpec.HTTPSProxy != "" {
			os.Setenv("HTTPS_PROXY", proxySpec.HTTPSProxy)
		}
		if proxySpec.NoProxy != "" {
			os.Setenv("NO_PROXY", proxySpec.NoProxy)
		}
	}

	if err := os.Chmod(runtimeconfig.EmbeddedClusterHomeDirectory(), 0755); err != nil {
		// don't fail as there are cases where we can't change the permissions (bind mounts, selinux, etc...),
		// and we handle and surface those errors to the user later (host preflights, checking exec errors, etc...)
		logrus.Debugf("unable to chmod embedded-cluster home dir: %s", err)
	}

	a.getConfig(w, r)
}
