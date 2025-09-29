package linux

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// PostUpgradeApp handler to upgrade the app
//
//	@ID				postLinuxUpgradeApp
//	@Summary		Upgrade the app
//	@Description	Upgrade the app using current configuration
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.UpgradeAppRequest	true	"Upgrade App Request"
//	@Success		200		{object}	types.AppUpgrade
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/upgrade/app/upgrade [post]
func (h *Handler) PostUpgradeApp(w http.ResponseWriter, r *http.Request) {
	var req types.UpgradeAppRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.upgradeController.UpgradeApp(r.Context(), req.IgnoreAppPreflights)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to upgrade app")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppUpgradeStatus(w, r)
}

// GetAppUpgradeStatus handler to get app upgrade status
//
//	@ID				getLinuxUpgradeAppStatus
//	@Summary		Get app upgrade status
//	@Description	Get the current status of app upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppUpgrade
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/app/status [get]
func (h *Handler) GetAppUpgradeStatus(w http.ResponseWriter, r *http.Request) {
	appUpgrade, err := h.upgradeController.GetAppUpgradeStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app upgrade status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appUpgrade, h.logger)
}

// PostUpgradeTemplateAppConfig handler to template app config for upgrade
//
//	@ID				postLinuxUpgradeAppConfigTemplate
//	@Summary		Template app config for upgrade
//	@Description	Template the app configuration with values for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.TemplateAppConfigRequest	true	"Template App Config Request"
//	@Success		200		{object}	types.AppConfig
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/upgrade/app/config/template [post]
func (h *Handler) PostUpgradeTemplateAppConfig(w http.ResponseWriter, r *http.Request) {
	var req types.TemplateAppConfigRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	appConfig, err := h.upgradeController.TemplateAppConfig(r.Context(), req.Values, true)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to template app config for upgrade")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appConfig, h.logger)
}

// PatchUpgradeAppConfigValues handler to set the app config values for upgrade
//
//	@ID				patchLinuxUpgradeAppConfigValues
//	@Summary		Set the app config values for upgrade
//	@Description	Set the app config values with partial updates for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PatchAppConfigValuesRequest	true	"Patch App Config Values Request"
//	@Success		200		{object}	types.AppConfigValuesResponse
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/upgrade/app/config/values [patch]
func (h *Handler) PatchUpgradeAppConfigValues(w http.ResponseWriter, r *http.Request) {
	var req types.PatchAppConfigValuesRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.upgradeController.PatchAppConfigValues(r.Context(), req.Values)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to set app config values for upgrade")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetUpgradeAppConfigValues(w, r)
}

// GetUpgradeAppConfigValues handler to get the app config values for upgrade
//
//	@ID				getLinuxUpgradeAppConfigValues
//	@Summary		Get the app config values for upgrade
//	@Description	Get the current app config values for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfigValuesResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/app/config/values [get]
func (h *Handler) GetUpgradeAppConfigValues(w http.ResponseWriter, r *http.Request) {
	values, err := h.upgradeController.GetAppConfigValues(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config values for upgrade")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.AppConfigValuesResponse{
		Values: values,
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
