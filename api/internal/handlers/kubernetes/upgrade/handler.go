package upgrade

import (
	"errors"
	"net/http"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
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
//	@ID				postKubernetesUpgradeAppConfigTemplate
//	@Summary		Template app config for upgrade
//	@Description	Template the app configuration with values for upgrade
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.TemplateAppConfigRequest	true	"Template App Config Request"
//	@Success		200		{object}	types.AppConfig
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/upgrade/app/config/template [post]
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
//	@ID				patchKubernetesUpgradeAppConfigValues
//	@Summary		Set the app config values for upgrade
//	@Description	Set the app config values with partial updates for upgrade
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.PatchAppConfigValuesRequest	true	"Patch App Config Values Request"
//	@Success		200		{object}	types.AppConfigValuesResponse
//	@Failure		400		{object}	types.APIError
//	@Router			/kubernetes/upgrade/app/config/values [patch]
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
//	@ID				getKubernetesUpgradeAppConfigValues
//	@Summary		Get the app config values for upgrade
//	@Description	Get the current app config values for upgrade
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.AppConfigValuesResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/upgrade/app/config/values [get]
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
//	@ID				postKubernetesUpgradeRunAppPreflights
//	@Summary		Run upgrade app preflight checks
//	@Description	Run upgrade app preflight checks using current app configuration
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.UpgradeAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/upgrade/app-preflights/run [post]
func (h *Handler) PostRunAppPreflights(w http.ResponseWriter, r *http.Request) {
	preflightBinary, err := h.cfg.Installation.PathToEmbeddedBinary("kubectl-preflight")
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to materialize preflight binary")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	err = h.controller.RunAppPreflights(r.Context(), appcontroller.RunAppPreflightOptions{
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

// GetAppPreflightsStatus handler to get app preflight status for upgrade
//
//	@ID				getKubernetesUpgradeAppPreflightsStatus
//	@Summary		Get app preflight status for upgrade
//	@Description	Get the current status and results of app preflight checks for upgrade
//	@Tags			kubernetes-upgrade
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.UpgradeAppPreflightsStatusResponse
//	@Failure		400	{object}	types.APIError
//	@Router			/kubernetes/upgrade/app-preflights/status [get]
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

	if status.State == types.StateSucceeded && output == nil {
		err := errors.New("preflight output is empty")
		utils.LogError(r, err, h.logger, "app preflights succeeded but output is nil")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.UpgradeAppPreflightsStatusResponse{
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
