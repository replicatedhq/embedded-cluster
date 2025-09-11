package kubernetes

import (
	"net/http"

	appinstall "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
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
//	@Success		200	{object}	types.KubernetesInstallationConfigResponse
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
//	@Failure		400					{object}	types.APIError
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

// PostRunAppPreflights handler to run install app preflight checks
//
//	@ID				postKubernetesInstallRunAppPreflights
//	@Summary		Run install app preflight checks
//	@Description	Run install app preflight checks using current app configuration
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/install/app-preflights/run [post]
func (h *Handler) PostRunAppPreflights(w http.ResponseWriter, r *http.Request) {
	preflightBinary, err := h.cfg.Installation.PathToEmbeddedBinary("kubectl-preflight")
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to materialize preflight binary")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	err = h.installController.RunAppPreflights(r.Context(), appinstall.RunAppPreflightOptions{
		PreflightBinaryPath: preflightBinary,
		ProxySpec:           h.cfg.Installation.ProxySpec(),
		CleanupBinary:       true,
	})
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to run app preflights")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppPreflightsStatus(w, r)
}

// GetAppPreflightsStatus handler to get app preflight status for install
//
//	@ID				getKubernetesInstallAppPreflightsStatus
//	@Summary		Get app preflight status for install
//	@Description	Get the current status and results of app preflight checks for install
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/install/app-preflights/status [get]
func (h *Handler) GetAppPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	titles, err := h.installController.GetAppPreflightTitles(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight titles")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	output, err := h.installController.GetAppPreflightOutput(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight output")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	status, err := h.installController.GetAppPreflightStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.InstallAppPreflightsStatusResponse{
		Titles:                        titles,
		Output:                        output,
		Status:                        status,
		HasStrictAppPreflightFailures: false,
		AllowIgnoreAppPreflights:      true, // TODO: implement if we decide to support a ignore-app-preflights CLI flag for V3
	}

	// Set hasStrictAppPreflightFailures based on app preflights output
	if output != nil {
		response.HasStrictAppPreflightFailures = output.HasStrictFailures()
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
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

// PostTemplateAppConfig handler to template the app config with provided values
//
//	@ID				postKubernetesInstallTemplateAppConfig
//	@Summary		Template the app config with provided values
//	@Description	Template the app config with provided values and return the templated config
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.TemplateAppConfigRequest	true	"Template App Config Request"
//	@Success		200		{object}	types.AppConfig
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/install/app/config/template [post]
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
//	@ID				patchKubernetesInstallAppConfigValues
//	@Summary		Set the app config values
//	@Description	Set the app config values with partial updates
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PatchAppConfigValuesRequest	true	"Patch App Config Values Request"
//	@Success		200		{object}	types.AppConfigValuesResponse
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/install/app/config/values [patch]
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
//	@ID				getKubernetesInstallAppConfigValues
//	@Summary		Get the app config values
//	@Description	Get the current app config values
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfigValuesResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/install/app/config/values [get]
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

// PostInstallApp handler to install the app
//
//	@ID				postKubernetesInstallApp
//	@Summary		Install the app
//	@Description	Install the app using current configuration
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.InstallAppRequest	true	"Install App Request"
//	@Success		200		{object}	types.AppInstall
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/install/app/install [post]
func (h *Handler) PostInstallApp(w http.ResponseWriter, r *http.Request) {
	var req types.InstallAppRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.installController.InstallApp(r.Context(), req.IgnoreAppPreflights)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to install app")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppInstallStatus(w, r)
}

// GetAppInstallStatus handler to get app install status
//
//	@ID				getKubernetesInstallAppStatus
//	@Summary		Get app install status
//	@Description	Get the current status of app installation
//	@Tags			kubernetes-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppInstall
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/install/app/status [get]
func (h *Handler) GetAppInstallStatus(w http.ResponseWriter, r *http.Request) {
	appInstall, err := h.installController.GetAppInstallStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app install status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appInstall, h.logger)
}
