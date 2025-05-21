package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/models"
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(install)
}

func (a *API) postInstallPhaseSetConfig(w http.ResponseWriter, r *http.Request) {
	var config models.InstallationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Info("failed to decode installation config")
		NewBadRequestError(err).JSON(w)
		return
	}

	if err := a.installController.SetConfig(r.Context(), config); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to set installation config")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.getInstall(w, r)
}

func (a *API) postInstallPhaseStart(w http.ResponseWriter, r *http.Request) {
	if err := a.installController.StartInstall(r.Context()); err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to start installation")
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
