package linux

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
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

// GetAppConfig handler to get the app config
//
//	@ID				getLinuxInstallAppConfig
//	@Summary		Get the app config
//	@Description	get the app config
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	kotsv1beta1.Config
//	@Router			/linux/install/app/config [get]
func (h *Handler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.installController.GetAppConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, config, h.logger)
}

// PostSetAppConfig handler to set the app config
//
//	@ID				postLinuxInstallSetAppConfig
//	@Summary		Set the app config
//	@Description	set the app config
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			appConfig	body		kotsv1beta1.Config	true	"App config"
//	@Success		200			{object}	kotsv1beta1.Config
//	@Router			/linux/install/app/configure [post]
func (h *Handler) PostSetAppConfig(w http.ResponseWriter, r *http.Request) {
	var config kotsv1beta1.Config
	if err := utils.BindJSON(w, r, &config, h.logger); err != nil {
		return
	}

	if err := h.installController.SetAppConfig(r.Context(), config); err != nil {
		utils.LogError(r, err, h.logger, "failed to set app config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	// Return the set config to confirm it was stored
	storedConfig, err := h.installController.GetAppConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config after setting")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, storedConfig, h.logger)
}
