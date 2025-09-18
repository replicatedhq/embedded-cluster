package kubernetes

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// PostUpgradeApp handler to upgrade the app
//
//	@ID				postKubernetesUpgradeApp
//	@Summary		Upgrade the app
//	@Description	Upgrade the app using current configuration
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.UpgradeAppRequest	true	"Upgrade App Request"
//	@Success		200		{object}	types.AppUpgrade
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/upgrade/app/upgrade [post]
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
//	@ID				getKubernetesUpgradeAppStatus
//	@Summary		Get app upgrade status
//	@Description	Get the current status of app upgrade
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppUpgrade
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/upgrade/app/status [get]
func (h *Handler) GetAppUpgradeStatus(w http.ResponseWriter, r *http.Request) {
	appUpgrade, err := h.upgradeController.GetAppUpgradeStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app upgrade status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appUpgrade, h.logger)
}
