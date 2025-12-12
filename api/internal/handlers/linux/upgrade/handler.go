package upgrade

import (
	"net/http"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	cfg           types.APIConfig
	controller    upgrade.Controller
	appController *appcontroller.AppController
	logger        logrus.FieldLogger
}

type Option func(*Handler)

func WithController(controller upgrade.Controller) Option {
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

func New(cfg types.APIConfig, opts ...Option) *Handler {
	h := &Handler{
		cfg: cfg,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

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

	err := h.controller.UpgradeApp(r.Context(), req.IgnoreAppPreflights)
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
	appUpgrade, err := h.controller.GetAppUpgradeStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get app upgrade status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appUpgrade, h.logger)
}

// PostTemplateAppConfig handler to template app config for upgrade
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
func (h *Handler) PostTemplateAppConfig(w http.ResponseWriter, r *http.Request) {
	var req types.TemplateAppConfigRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	appConfig, err := h.controller.TemplateAppConfig(r.Context(), req.Values, true)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to template app config for upgrade")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, appConfig, h.logger)
}

// PatchAppConfigValues handler to set the app config values for upgrade
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
func (h *Handler) PatchAppConfigValues(w http.ResponseWriter, r *http.Request) {
	var req types.PatchAppConfigValuesRequest
	if err := utils.BindJSON(w, r, &req, h.logger); err != nil {
		return
	}

	err := h.controller.PatchAppConfigValues(r.Context(), req.Values)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to set app config values for upgrade")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetAppConfigValues(w, r)
}

// GetAppConfigValues handler to get the app config values for upgrade
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
func (h *Handler) GetAppConfigValues(w http.ResponseWriter, r *http.Request) {
	values, err := h.controller.GetAppConfigValues(r.Context())
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

// PostRunAppPreflights handler to run upgrade app preflight checks
//
//	@ID				postLinuxUpgradeRunAppPreflights
//	@Summary		Run upgrade app preflight checks
//	@Description	Run upgrade app preflight checks using current app configuration
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.UpgradeAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/app-preflights/run [post]
func (h *Handler) PostRunAppPreflights(w http.ResponseWriter, r *http.Request) {
	var registrySettings *types.RegistrySettings
	var err error

	if h.cfg.Mode == types.ModeInstall {
		registrySettings, err = h.controller.CalculateRegistrySettings(r.Context(), h.cfg.RuntimeConfig)
		if err != nil {
			utils.LogError(r, err, h.logger, "failed to calculate registry settings")
			utils.JSONError(w, r, err, h.logger)
			return
		}
	} else {
		registrySettings, err = h.controller.GetRegistrySettings(r.Context(), h.cfg.RuntimeConfig)
		if err != nil {
			utils.LogError(r, err, h.logger, "failed to get registry settings")
			utils.JSONError(w, r, err, h.logger)
			return
		}
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

// GetAppPreflightsStatus handler to get app preflight status for upgrade
//
//	@ID				getLinuxUpgradeAppPreflightsStatus
//	@Summary		Get app preflight status for upgrade
//	@Description	Get the current status and results of app preflight checks for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.UpgradeAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/app-preflights/status [get]
func (h *Handler) GetAppPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	titles, err := h.controller.GetAppPreflightTitles(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get upgrade app preflight titles")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	output, err := h.controller.GetAppPreflightOutput(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get upgrade app preflight output")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	status, err := h.controller.GetAppPreflightStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get upgrade app preflight status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.UpgradeAppPreflightsStatusResponse{
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

// PostUpgradeInfra handler to upgrade the infrastructure
//
//	@ID				postLinuxUpgradeInfra
//	@Summary		Upgrade the infrastructure
//	@Description	Upgrade the infrastructure (k0s, addons, extensions)
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Failure		400	{object}	types.APIError
//	@Failure		401	{object}	types.APIError
//	@Failure		409	{object}	types.APIError
//	@Failure		500	{object}	types.APIError
//	@Router			/linux/upgrade/infra/upgrade [post]
func (h *Handler) PostUpgradeInfra(w http.ResponseWriter, r *http.Request) {
	err := h.controller.UpgradeInfra(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to upgrade infra")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetInfraStatus(w, r)
}

// GetInfraStatus handler to get the status of the infra upgrade
//
//	@ID				getLinuxUpgradeInfraStatus
//	@Summary		Get the status of the infra upgrade
//	@Description	Get the current status of the infrastructure upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Infra
//	@Failure		401	{object}	types.APIError
//	@Failure		500	{object}	types.APIError
//	@Router			/linux/upgrade/infra/status [get]
func (h *Handler) GetInfraStatus(w http.ResponseWriter, r *http.Request) {
	infra, err := h.controller.GetInfra(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get upgrade infra status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, infra, h.logger)
}

// PostProcessAirgap handler to process the airgap bundle
//
//	@ID				postLinuxUpgradeProcessAirgap
//	@Summary		Process the airgap bundle
//	@Description	Process the airgap bundle for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Airgap
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/airgap/process [post]
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
//	@ID				getLinuxUpgradeAirgapStatus
//	@Summary		Get the status of the airgap processing
//	@Description	Get the current status of the airgap processing for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.Airgap
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/airgap/status [get]
func (h *Handler) GetAirgapStatus(w http.ResponseWriter, r *http.Request) {
	airgap, err := h.controller.GetAirgapStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get airgap status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, airgap, h.logger)
}

// PostRunHostPreflights handler to run upgrade host preflight checks
//
//	@ID				postLinuxUpgradeRunHostPreflights
//	@Summary		Run upgrade host preflight checks
//	@Description	Run upgrade host preflight checks before infrastructure upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.HostPreflights
//	@Failure		400	{object}	types.APIError
//	@Failure		409	{object}	types.APIError
//	@Router			/linux/upgrade/host-preflights/run [post]
func (h *Handler) PostRunHostPreflights(w http.ResponseWriter, r *http.Request) {
	err := h.controller.RunHostPreflights(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to run host preflights")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetHostPreflightsStatus(w, r)
}

// GetHostPreflightsStatus handler to get host preflight status for upgrade
//
//	@ID				getLinuxUpgradeHostPreflightsStatus
//	@Summary		Get host preflight status for upgrade
//	@Description	Get the current status and results of host preflight checks for upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.HostPreflights
//	@Failure		400	{object}	types.APIError
//	@Router			/linux/upgrade/host-preflights/status [get]
func (h *Handler) GetHostPreflightsStatus(w http.ResponseWriter, r *http.Request) {
	hostPreflights, err := h.controller.GetHostPreflightsStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get upgrade host preflights status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, hostPreflights, h.logger)
}

// PostBypassHostPreflights handler to bypass failed host preflights
//
//	@ID				postLinuxUpgradeBypassHostPreflights
//	@Summary		Bypass failed host preflights
//	@Description	Bypass failed host preflight checks to continue with upgrade
//	@Tags			linux-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.HostPreflights
//	@Failure		400	{object}	types.APIError
//	@Failure		409	{object}	types.APIError
//	@Router			/linux/upgrade/host-preflights/bypass [post]
func (h *Handler) PostBypassHostPreflights(w http.ResponseWriter, r *http.Request) {
	err := h.controller.BypassHostPreflights(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to bypass host preflights")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.GetHostPreflightsStatus(w, r)
}
