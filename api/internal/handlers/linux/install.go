package linux

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// GetInstallationConfig handler to get the installation config
//
//	@ID				getLinuxInstallInstallationConfig
//	@Summary		Get the installation config
//	@Description	get the installation config
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.LinuxInstallationConfig
//	@Router			/linux/install/installation/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.installController.GetInstallationConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get installation config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, config, h.logger)
}

// PostConfigureInstallation handler to configure the installation for install
//
//	@ID				postLinuxInstallConfigureInstallation
//	@Summary		Configure the installation for install
//	@Description	configure the installation for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			installationConfig	body		types.LinuxInstallationConfig	true	"Installation config"
//	@Success		200					{object}	types.Status
//	@Failure		400					{object}	types.APIError
//	@Router			/linux/install/installation/configure [post]
func (h *Handler) PostConfigureInstallation(w http.ResponseWriter, r *http.Request) {
	var config types.LinuxInstallationConfig
	if err := utils.BindJSON(w, r, &config, h.logger); err != nil {
		return
	}

	if err := h.installController.ConfigureInstallation(r.Context(), config); err != nil {
		utils.LogError(r, err, h.logger, "failed to set installation config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetInstallationStatus(w, r)
}

// GetInstallationStatus handler to get the status of the installation configuration for install
//
//	@ID				getLinuxInstallInstallationStatus
//	@Summary		Get installation configuration status for install
//	@Description	Get the current status of the installation configuration for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Status
//	@Router			/linux/install/installation/status [get]
func (h *Handler) GetInstallationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.installController.GetInstallationStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get installation status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, status, h.logger)
}

// PostRunHostPreflights handler to run install host preflight checks
//
//	@ID				postLinuxInstallRunHostPreflights
//	@Summary		Run install host preflight checks
//	@Description	Run install host preflight checks using installation config and client-provided data
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PostInstallRunHostPreflightsRequest	true	"Post Install Run Host Preflights Request"
//	@Success		200		{object}	types.InstallHostPreflightsStatusResponse
//	@Router			/linux/install/host-preflights/run [post]
func (h *Handler) PostRunHostPreflights(w http.ResponseWriter, r *http.Request) {
	var req types.PostInstallRunHostPreflightsRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.installController.RunHostPreflights(r.Context(), install.RunHostPreflightsOptions{
		IsUI: req.IsUI,
	})
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to run install host preflights")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetHostPreflightsStatus(w, r)
}

// GetHostPreflightsStatus handler to get host preflight status for install
//
//	@ID				getLinuxInstallHostPreflightsStatus
//	@Summary		Get host preflight status for install
//	@Description	Get the current status and results of host preflight checks for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallHostPreflightsStatusResponse
//	@Router			/linux/install/host-preflights/status [get]
func (h *Handler) GetHostPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	titles, err := h.installController.GetHostPreflightTitles(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight titles")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	output, err := h.installController.GetHostPreflightOutput(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight output")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	status, err := h.installController.GetHostPreflightStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.InstallHostPreflightsStatusResponse{
		Titles:                    titles,
		Output:                    output,
		Status:                    status,
		AllowIgnoreHostPreflights: h.cfg.AllowIgnoreHostPreflights,
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}

// PostSetupInfra handler to setup infra components
//
//	@ID				postLinuxInstallSetupInfra
//	@Summary		Setup infra components
//	@Description	Setup infra components
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.LinuxInfraSetupRequest	true	"Infra Setup Request"
//	@Success		200		{object}	types.Infra
//	@Router			/linux/install/infra/setup [post]
func (h *Handler) PostSetupInfra(w http.ResponseWriter, r *http.Request) {
	var req types.LinuxInfraSetupRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.installController.SetupInfra(r.Context(), req.IgnoreHostPreflights)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to setup infra")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetInfraStatus(w, r)
}

// GetInfraStatus handler to get the status of the infra
//
//	@ID				getLinuxInstallInfraStatus
//	@Summary		Get the status of the infra
//	@Description	Get the current status of the infra
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Router			/linux/install/infra/status [get]
func (h *Handler) GetInfraStatus(w http.ResponseWriter, r *http.Request) {
	infra, err := h.installController.GetInfra(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install infra status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, infra, h.logger)
}

// PostTemplateAppConfig handler to template the app config with provided values
//
//	@ID				postLinuxInstallTemplateAppConfig
//	@Summary		Template the app config with provided values
//	@Description	Template the app config with provided values and return the templated config
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.TemplateAppConfigRequest	true	"Template App Config Request"
//	@Success		200		{object}	types.AppConfig
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/install/app/config/template [post]
func (h *Handler) PostTemplateAppConfig(w http.ResponseWriter, r *http.Request) {
	var req types.TemplateAppConfigRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	appConfig, err := h.installController.TemplateAppConfig(r.Context(), req.Values, true)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to template app config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appConfig, h.logger)
}

// PatchAppConfigValues handler to set the app config values
//
//	@ID				patchLinuxInstallAppConfigValues
//	@Summary		Set the app config values
//	@Description	Set the app config values with partial updates
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PatchAppConfigValuesRequest	true	"Patch App Config Values Request"
//	@Success		200		{object}	types.AppConfigValuesResponse
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/install/app/config/values [patch]
func (h *Handler) PatchAppConfigValues(w http.ResponseWriter, r *http.Request) {
	var req types.PatchAppConfigValuesRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.installController.PatchAppConfigValues(r.Context(), req.Values)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to set app config values")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppConfigValues(w, r)
}

// GetAppConfigValues handler to get the app config values
//
//	@ID				getLinuxInstallAppConfigValues
//	@Summary		Get the app config values
//	@Description	Get the current app config values
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfigValuesResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/app/config/values [get]
func (h *Handler) GetAppConfigValues(w http.ResponseWriter, r *http.Request) {
	values, err := h.installController.GetAppConfigValues(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config values")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.AppConfigValuesResponse{
		Values: values,
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
