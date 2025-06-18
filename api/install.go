package api

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
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
		a.logError(r, err, "failed to get installation config")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, config)
}

// postInstallConfigureInstallation handler to configure the installation for install
//
//	@Summary		Configure the installation for install
//	@Description	configure the installation for install
//	@Tags			install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			installationConfig	body		types.InstallationConfig	true	"Installation config"
//	@Success		200					{object}	types.Status
//	@Router			/install/installation/configure [post]
func (a *API) postInstallConfigureInstallation(w http.ResponseWriter, r *http.Request) {
	var config types.InstallationConfig
	if err := a.bindJSON(w, r, &config); err != nil {
		return
	}

	if err := a.installController.ConfigureInstallation(r.Context(), &config); err != nil {
		a.logError(r, err, "failed to set installation config")
		a.jsonError(w, r, err)
		return
	}

	a.getInstallInstallationStatus(w, r)
}

// getInstallInstallationStatus handler to get the status of the installation configuration for install
//
//	@Summary		Get installation configuration status for install
//	@Description	Get the current status of the installation configuration for install
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Status
//	@Router			/install/installation/status [get]
func (a *API) getInstallInstallationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.GetInstallationStatus(r.Context())
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
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PostInstallRunHostPreflightsRequest	true	"Post Install Run Host Preflights Request"
//	@Success		200		{object}	types.InstallHostPreflightsStatusResponse
//	@Router			/install/host-preflights/run [post]
func (a *API) postInstallRunHostPreflights(w http.ResponseWriter, r *http.Request) {
	var req types.PostInstallRunHostPreflightsRequest
	if err := a.bindJSON(w, r, &req); err != nil {
		return
	}

	err := a.installController.RunHostPreflights(r.Context(), install.RunHostPreflightsOptions{
		IsUI: req.IsUI,
	})
	if err != nil {
		a.logError(r, err, "failed to run install host preflights")
		a.jsonError(w, r, err)
		return
	}

	a.getInstallHostPreflightsStatus(w, r)
}

// getInstallHostPreflightsStatus handler to get host preflight status for install
//
//	@Summary		Get host preflight status for install
//	@Description	Get the current status and results of host preflight checks for install
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallHostPreflightsStatusResponse
//	@Router			/install/host-preflights/status [get]
func (a *API) getInstallHostPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	titles, err := a.installController.GetHostPreflightTitles(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install host preflight titles")
		a.jsonError(w, r, err)
		return
	}

	output, err := a.installController.GetHostPreflightOutput(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install host preflight output")
		a.jsonError(w, r, err)
		return
	}

	status, err := a.installController.GetHostPreflightStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install host preflight status")
		a.jsonError(w, r, err)
		return
	}

	response := types.InstallHostPreflightsStatusResponse{
		Titles:                    titles,
		Output:                    output,
		Status:                    status,
		AllowIgnoreHostPreflights: a.ignoreHostPreflights,
	}

	a.json(w, r, http.StatusOK, response)
}

// postInstallSetupInfra handler to setup infra components
//
//	@Summary		Setup infra components
//	@Description	Setup infra components
//	@Tags			install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.InfraSetupRequest	true	"Infra Setup Request"
//	@Success		200		{object}	types.InfraSetupResponse
//	@Router			/install/infra/setup [post]
func (a *API) postInstallSetupInfra(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req types.InfraSetupRequest
	if err := a.bindJSON(w, r, &req); err != nil {
		return
	}

	// Setup infrastructure with preflight validation handled internally
	preflightsIgnored, err := a.installController.SetupInfra(r.Context(), req.IgnorePreflightFailures)
	if err != nil {
		a.logError(r, err, "failed to setup infra")
		// Determine appropriate status code based on error type
		statusCode := http.StatusInternalServerError
		if err.Error() == "Preflight checks failed" {
			statusCode = http.StatusBadRequest
		}
		w.WriteHeader(statusCode)
		a.json(w, r, statusCode, types.APIError{
			StatusCode: statusCode,
			Message:    err.Error(),
		})
		return
	}

	// Get the infra status for the response
	infra, err := a.installController.GetInfra(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install infra status after setup")
		a.jsonError(w, r, err)
		return
	}

	// Create response with preflight context
	response := types.InfraSetupResponse{
		Infra:             infra,
		PreflightsIgnored: preflightsIgnored,
	}

	a.json(w, r, http.StatusOK, response)
}

// getInstallInfraStatus handler to get the status of the infra
//
//	@Summary		Get the status of the infra
//	@Description	Get the current status of the infra
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Router			/install/infra/status [get]
func (a *API) getInstallInfraStatus(w http.ResponseWriter, r *http.Request) {
	infra, err := a.installController.GetInfra(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install infra status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, infra)
}

// postInstallSetInstallStatus handler to set the status of the install workflow
//
//	@Summary		Set the status of the install workflow
//	@Description	Set the status of the install workflow
//	@Tags			install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			status	body		types.Status	true	"Status"
//	@Success		200		{object}	types.Status
//	@Router			/install/status [post]
func (a *API) setInstallStatus(w http.ResponseWriter, r *http.Request) {
	var status types.Status
	if err := a.bindJSON(w, r, &status); err != nil {
		return
	}

	if err := types.ValidateStatus(&status); err != nil {
		a.logError(r, err, "invalid install status")
		a.jsonError(w, r, err)
		return
	}

	if err := a.installController.SetStatus(r.Context(), &status); err != nil {
		a.logError(r, err, "failed to set install status")
		a.jsonError(w, r, err)
		return
	}

	a.getInstallStatus(w, r)
}

// getInstallStatus handler to get the status of the install workflow
//
//	@Summary		Get the status of the install workflow
//	@Description	Get the current status of the install workflow
//	@Tags			install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Status
//	@Router			/install/status [get]
func (a *API) getInstallStatus(w http.ResponseWriter, r *http.Request) {
	status, err := a.installController.GetStatus(r.Context())
	if err != nil {
		a.logError(r, err, "failed to get install status")
		a.jsonError(w, r, err)
		return
	}

	a.json(w, r, http.StatusOK, status)
}
