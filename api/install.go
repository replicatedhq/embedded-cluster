package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type InstallationConfigRequest struct {
	DataDirectory string `json:"dataDirectory"`
}

func (a *API) getInstall(w http.ResponseWriter, r *http.Request) {
	install, err := a.installController.Get(r.Context())
	if err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to get installation")
		handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(install)
}

func (a *API) setInstallConfig(w http.ResponseWriter, r *http.Request) {
	var config types.InstallationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Info("failed to decode installation config")
		types.NewBadRequestError(err).JSON(w)
		return
	}

	if err := a.installController.SetConfig(r.Context(), &config); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to set installation config")
		handleError(w, err)
		return
	}

	a.getInstall(w, r)

	// TODO: this is a hack to get the config to the CLI
	if a.configChan != nil {
		a.configChan <- &config
	}
}

func (a *API) setInstallStatus(w http.ResponseWriter, r *http.Request) {
	var status types.InstallationStatus
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Info("failed to decode installation status")
		types.NewBadRequestError(err).JSON(w)
		return
	}

	if err := a.installController.SetStatus(r.Context(), &status); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to set installation status")
		handleError(w, err)
		return
	}

	a.getInstall(w, r)
}

func logrusFieldsFromRequest(r *http.Request) logrus.Fields {
	return logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}
}
