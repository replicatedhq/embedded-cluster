package linux

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// GetConfigValues handler to get the config values for Linux installation
//
//	@ID				getLinuxConfigValues
//	@Summary		Get Linux config values
//	@Description	Get app config values converted from boolean config items for Linux installation
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	kotsv1beta1.ConfigValues
//	@Failure		401	{object}	types.APIError
//	@Failure		500	{object}	types.APIError
//	@Router			/linux/install/app/config/values [get]
func (h *Handler) GetAppConfigValues(w http.ResponseWriter, r *http.Request) {
	// Import is used in Swagger annotation above
	_ = kotsv1beta1.ConfigValues{}

	configValues, err := h.installController.GetAppConfigValues(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get config values")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, configValues, h.logger)
}
