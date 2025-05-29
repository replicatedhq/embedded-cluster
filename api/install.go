package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// getInstall handler to get the install object
//
//	@Summary		Get the install object
//	@Description	get the install object
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Install
//	@Router			/install [get]
func (a *API) getInstall(w http.ResponseWriter, r *http.Request) {
	install, err := a.installController.Get(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, install)
}

// setInstallConfig handler to set the installation config
//
//	@Summary		Set the installation config
//	@Description	set the installation config
//	@Tags				install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body	types.InstallationConfig	true	"Installation config"
//	@Success		200	{object}	types.Install
//	@Router			/install/config [post]
func (a *API) setInstallConfig(w http.ResponseWriter, r *http.Request) {
	var config types.InstallationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logError(r, err, "failed to decode installation config")
		a.jsonError(w, r, types.NewBadRequestError(err))
		return
	}

	if err := a.installController.SetConfig(r.Context(), &config); err != nil {
		a.logError(r, err, "failed to set installation config")
		a.jsonError(w, r, err)
		return
	}

	a.getInstall(w, r)

	// TODO: this is a hack to get the config to the CLI
	if a.configChan != nil {
		a.configChan <- &config
	}
}

// setInstallStatus handler to set the installation status
//
//	@Summary		Set the installation status
//	@Description	set the installation status
//	@Tags				install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body	types.InstallationStatus	true	"Installation status"
//	@Success		200	{object}	types.Install
//	@Router			/install/status [post]
func (a *API) setInstallStatus(w http.ResponseWriter, r *http.Request) {
	var status types.InstallationStatus
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		a.logError(r, err, "failed to decode installation status")
		a.jsonError(w, r, types.NewBadRequestError(err))
		return
	}

	if err := a.installController.SetStatus(r.Context(), &status); err != nil {
		a.logError(r, err, "failed to set installation status")
		a.jsonError(w, r, err)
		return
	}

	a.getInstall(w, r)
}

// getInstallStatus handler to get the installation status
//
//	@Summary		Get the installation status
//	@Description	get the installation status
//	@Tags				install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallationStatus
//	@Router			/install/status [get]
func (a *API) getInstallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.ReadStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, status)
}

// runInstallHostPreflights handler to run install host preflight checks
//
//	@Summary		Run install host preflight checks
//	@Description	Run install host preflight checks using installation config and client-provided data
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.RunHostPreflightResponse
//	@Router			/install/host-preflights [post]
func (a *API) runInstallHostPreflights(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
}

// getInstallHostPreflightStatus handler to get host preflight status
//
//	@Summary		Get host preflight status
//	@Description	Get the current status and results of host preflight checks
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.HostPreflightStatusResponse
//	@Router			/install/host-preflights [get]
func (a *API) getInstallHostPreflightStatus(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
}
