package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// getInstallInstallationConfig handler to get the installation config
//
//	@Summary		Get the installation config
//	@Description	get the installation config
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallationConfig
//	@Router			/install/installation/config [get]
func (a *API) getInstallInstallationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := a.installController.GetInstallationConfig(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, config)
}

func (a *API) postInstallConfigureInstallation(w http.ResponseWriter, r *http.Request) {
	var config types.InstallationConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		a.logError(r, err, "failed to decode installation config")
		a.jsonError(w, r, types.NewBadRequestError(err))
		return
	}

	if err := a.installController.ConfigureInstallation(r.Context(), &config); err != nil {
		a.logError(r, err, "failed to set installation config")
		a.jsonError(w, r, err)
		return
	}

	a.getInstallInstallationConfig(w, r)

	// TODO: this is a hack to get the config to the CLI
	if a.configChan != nil {
		a.configChan <- &config
	}
}

// getInstallInstallationStatus handler to get the status of the installation configuration
//
//	@Summary		Get installation configuration status
//	@Description	Get the current status of the installation configuration
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallationConfig
//	@Router			/install/installation/status [get]
func (a *API) getInstallInstallationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.GetStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, status)
}

// postInstallRunHostPreflights handler to run install host preflight checks
//
//	@Summary		Run install host preflight checks
//	@Description	Run install host preflight checks using installation config and client-provided data
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.RunHostPreflightResponse
//	@Router			/install/host-preflights [post]
func (a *API) postInstallRunHostPreflights(w http.ResponseWriter, r *http.Request) {
	response, err := a.installController.RunHostPreflights(r.Context())
	if err != nil {
		a.logError(r, err, "failed to run install host preflights")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, response)
}

// getInstallHostPreflightsStatus handler to get host preflight status
//
//	@Summary		Get host preflight status
//	@Description	Get the current status and results of host preflight checks
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.HostPreflightStatusResponse
//	@Router			/install/host-preflights [get]
func (a *API) getInstallHostPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	response, err := a.installController.GetHostPreflightStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install host preflight status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, response)
}

func (a *API) setInstallStatus(w http.ResponseWriter, r *http.Request) {
	var status types.Status
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

	a.getInstallStatus(w, r)
}

func (a *API) getInstallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.GetStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get installation status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, status)
}
