package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (a *API) getInstall(w http.ResponseWriter, r *http.Request) {
	install, err := a.installController.Get(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation")
		a.JSONError(w, r, err)
		return
	}

	a.JSON(w, r, http.StatusOK, install)
}

func (a *API) setInstallConfig(w http.ResponseWriter, r *http.Request) {
	var config types.InstallationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logError(r, err, "failed to decode installation config")
		types.NewBadRequestError(err).JSON(w)
		return
	}

	if err := a.installController.SetConfig(r.Context(), &config); err != nil {
		a.logError(r, err, "failed to set installation config")
		a.JSONError(w, r, err)
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
		a.logError(r, err, "failed to decode installation status")
		types.NewBadRequestError(err).JSON(w)
		return
	}

	if err := a.installController.SetStatus(r.Context(), &status); err != nil {
		a.logError(r, err, "failed to set installation status")
		a.JSONError(w, r, err)
		return
	}

	a.getInstall(w, r)
}

func (a *API) getInstallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.ReadStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation status")
		a.JSONError(w, r, err)
		return
	}

	a.JSON(w, r, http.StatusOK, status)
}
