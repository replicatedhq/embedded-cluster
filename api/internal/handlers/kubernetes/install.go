package kubernetes

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// GetInstallationConfig handler to get the Kubernetes installation config
//
//	@ID				getKubernetesInstallInstallationConfig
//	@Summary		Get the Kubernetes installation config
//	@Description	get the Kubernetes installation config
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.KubernetesInstallationConfig
//	@Router			/kubernetes/install/installation/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.installController.GetInstallationConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get installation config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, config, h.logger)
}

// PostConfigureInstallation handler to configure the Kubernetes installation for install
//
//	@ID				postKubernetesInstallConfigureInstallation
//	@Summary		Configure the Kubernetes installation for install
//	@Description	configure the Kubernetes installation for install
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			installationConfig	body		types.KubernetesInstallationConfig	true	"Installation config"
//	@Success		200					{object}	types.Status
//	@Router			/kubernetes/install/installation/configure [post]
func (h *Handler) PostConfigureInstallation(w http.ResponseWriter, r *http.Request) {
	var config types.KubernetesInstallationConfig
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
//	@ID				getKubernetesInstallInstallationStatus
//	@Summary		Get installation configuration status for install
//	@Description	Get the current status of the installation configuration for install
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Status
//	@Router			/kubernetes/install/installation/status [get]
func (h *Handler) GetInstallationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.installController.GetInstallationStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get installation status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, status, h.logger)
}

// PostSetupInfra handler to setup infra components
//
//	@ID				postKubernetesInstallSetupInfra
//	@Summary		Setup infra components
//	@Description	Setup infra components
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Router			/kubernetes/install/infra/setup [post]
func (h *Handler) PostSetupInfra(w http.ResponseWriter, r *http.Request) {
	err := h.installController.SetupInfra(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to setup infra")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetInfraStatus(w, r)
}

// GetInfraStatus handler to get the status of the infra
//
//	@ID				getKubernetesInstallInfraStatus
//	@Summary		Get the status of the infra
//	@Description	Get the current status of the infra
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Router			/kubernetes/install/infra/status [get]
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
//	@ID				getKubernetesInstallAppConfig
//	@Summary		Get the app config
//	@Description	get the app config
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfig
//	@Router			/kubernetes/install/app/config [get]
func (h *Handler) GetAppConfig(w http.ResponseWriter, r *http.Request) {
	appConfig, err := h.installController.GetAppConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, types.AppConfig(appConfig.Spec), h.logger)
}

// PostSetAppConfigValues handler to set the app config values
//
//	@ID				postKubernetesInstallSetAppConfigValues
//	@Summary		Set the app config values
//	@Description	Set the app config values
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.SetAppConfigValuesRequest	true	"Set App Config Values Request"
//	@Success		200		{object}	types.AppConfig
//	@Router			/kubernetes/install/app/config/values [post]
func (h *Handler) PostSetAppConfigValues(w http.ResponseWriter, r *http.Request) {
	var req types.SetAppConfigValuesRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.installController.SetAppConfigValues(r.Context(), req.Values)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to set app config values")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppConfig(w, r)
}

// GetAppConfigValues handler to get the app config values
//
//	@ID				getKubernetesInstallAppConfigValues
//	@Summary		Get the app config values
//	@Description	Get the app config values
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfigValuesResponse
//	@Router			/kubernetes/install/app/config/values [get]
func (h *Handler) GetAppConfigValues(w http.ResponseWriter, r *http.Request) {
	configValues, err := h.installController.GetAppConfigValues(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app config values")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.AppConfigValuesResponse{
		Values: configValues,
	}
	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
