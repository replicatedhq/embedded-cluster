package install

import (
	"errors"
	"net/http"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	cfg             types.APIConfig
	controller      install.Controller
	appController   *appcontroller.AppController
	logger          logrus.FieldLogger
	hostUtils       interface{}
	metricsReporter interface{}
}

type Option func(*Handler)

func WithController(controller install.Controller) Option {
	return func(h *Handler) {
		h.controller = controller
	}
}

func WithAppController(appController *appcontroller.AppController) Option {
	return func(h *Handler) {
		h.appController = appController
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func WithHostUtils(hostUtils interface{}) Option {
	return func(h *Handler) {
		h.hostUtils = hostUtils
	}
}

func WithMetricsReporter(metricsReporter interface{}) Option {
	return func(h *Handler) {
		h.metricsReporter = metricsReporter
	}
}

func New(cfg types.APIConfig, opts ...Option) *Handler {
	h := &Handler{
		cfg: cfg,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// GetInstallationConfig handler to get the installation config
//
//	@ID				getLinuxInstallInstallationConfig
//	@Summary		Get the installation config
//	@Description	get the installation config
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.LinuxInstallationConfigResponse
//	@Router			/linux/install/installation/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.controller.GetInstallationConfig(r.Context())
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

	if err := h.controller.ConfigureInstallation(r.Context(), config); err != nil {
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
	status, err := h.controller.GetInstallationStatus(r.Context())
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

	err := h.controller.RunHostPreflights(r.Context(), install.RunHostPreflightsOptions{
		// TODO this should be inferred based on the user agent instead of requiring the client to send it via the request body
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
	titles, err := h.controller.GetHostPreflightTitles(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight titles")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	output, err := h.controller.GetHostPreflightOutput(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight output")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	status, err := h.controller.GetHostPreflightStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install host preflight status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	if status.State == types.StateSucceeded && output == nil {
		err := errors.New("preflight output is empty")
		utils.LogError(r, err, h.logger, "host preflights succeeded but output is nil")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.InstallHostPreflightsStatusResponse{
		Titles:                    titles,
		Output:                    output,
		Status:                    status,
		AllowIgnoreHostPreflights: h.cfg.AllowIgnoreHostPreflights,
	}

	if output != nil {
		response.HasFailures = output.HasFail()
		response.HasWarnings = output.HasWarn()
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}

// PostRunAppPreflights handler to run install app preflight checks
//
//	@ID				postLinuxInstallRunAppPreflights
//	@Summary		Run install app preflight checks
//	@Description	Run install app preflight checks using current app configuration
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/app-preflights/run [post]
func (h *Handler) PostRunAppPreflights(w http.ResponseWriter, r *http.Request) {
	registrySettings, err := h.controller.CalculateRegistrySettings(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to calculate registry settings")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	err = h.controller.RunAppPreflights(r.Context(), appcontroller.RunAppPreflightOptions{
		PreflightBinaryPath: h.cfg.RuntimeConfig.PathToEmbeddedClusterBinary("kubectl-preflight"),
		ProxySpec:           h.cfg.RuntimeConfig.ProxySpec(),
		RegistrySettings:    registrySettings,
		ExtraPaths:          []string{h.cfg.RuntimeConfig.EmbeddedClusterBinsSubDir()},
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
//	@ID				getLinuxInstallAppPreflightsStatus
//	@Summary		Get app preflight status for install
//	@Description	Get the current status and results of app preflight checks for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.InstallAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/app-preflights/status [get]
func (h *Handler) GetAppPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	titles, err := h.controller.GetAppPreflightTitles(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight titles")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	output, err := h.controller.GetAppPreflightOutput(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight output")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	status, err := h.controller.GetAppPreflightStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install app preflight status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	if status.State == types.StateSucceeded && output == nil {
		err := errors.New("preflight output is empty")
		utils.LogError(r, err, h.logger, "app preflights succeeded but output is nil")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.InstallAppPreflightsStatusResponse{
		Titles:                        titles,
		Output:                        output,
		Status:                        status,
		HasStrictAppPreflightFailures: false,
	}

	if status.State == types.StateSucceeded {
		response.AllowIgnoreAppPreflights = true // TODO: implement if we decide to support a ignore-app-preflights CLI flag for V3
	}

	if output != nil {
		response.HasFailures = output.HasFail()
		response.HasWarnings = output.HasWarn()
		response.HasStrictAppPreflightFailures = output.HasStrictFailures()
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

	err := h.controller.SetupInfra(r.Context(), req.IgnoreHostPreflights)
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
	infra, err := h.controller.GetInfra(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get install infra status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, infra, h.logger)
}

// PostProcessAirgap handler to process the airgap bundle
//
//	@ID				postLinuxInstallProcessAirgap
//	@Summary		Process the airgap bundle
//	@Description	Process the airgap bundle for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Airgap
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/airgap/process [post]
func (h *Handler) PostProcessAirgap(w http.ResponseWriter, r *http.Request) {
	err := h.controller.ProcessAirgap(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to process airgap")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAirgapStatus(w, r)
}

// GetAirgapStatus handler to get the status of the airgap processing
//
//	@ID				getLinuxInstallAirgapStatus
//	@Summary		Get the status of the airgap processing
//	@Description	Get the current status of the airgap processing for install
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Airgap
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/airgap/status [get]
func (h *Handler) GetAirgapStatus(w http.ResponseWriter, r *http.Request) {
	airgap, err := h.controller.GetAirgapStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get airgap status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, airgap, h.logger)
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

	appConfig, err := h.controller.TemplateAppConfig(r.Context(), req.Values, true)
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

	err := h.controller.PatchAppConfigValues(r.Context(), req.Values)
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
	values, err := h.controller.GetAppConfigValues(r.Context())
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
//	@ID				postLinuxInstallApp
//	@Summary		Install the app
//	@Description	Install the app using current configuration
//	@Tags			linux-install
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.InstallAppRequest	true	"Install App Request"
//	@Success		200		{object}	types.AppInstall
//	@Failure		400		{object}	types.APIError
//	@Router			/linux/install/app/install [post]
func (h *Handler) PostInstallApp(w http.ResponseWriter, r *http.Request) {
	var req types.InstallAppRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.controller.InstallApp(r.Context(), req.IgnoreAppPreflights)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to install app")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppInstallStatus(w, r)
}

// GetAppInstallStatus handler to get app install status
//
//	@ID				getLinuxInstallAppStatus
//	@Summary		Get app install status
//	@Description	Get the current status of app installation
//	@Tags			linux-install
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppInstall
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/install/app/status [get]
func (h *Handler) GetAppInstallStatus(w http.ResponseWriter, r *http.Request) {
	appInstall, err := h.controller.GetAppInstallStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app install status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appInstall, h.logger)
}
