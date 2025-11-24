package kurlmigration

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/kurlmigration"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	logger     logrus.FieldLogger
	controller kurlmigration.Controller
}

type Option func(*Handler)

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func WithController(controller kurlmigration.Controller) Option {
	return func(h *Handler) {
		h.controller = controller
	}
}

func New(opts ...Option) *Handler {
	h := &Handler{}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// GetInstallationConfig handler to get the installation config for migration
//
//	@ID				getMigrationInstallationConfig
//	@Summary		Get the installation config for migration
//	@Description	Get the installation config extracted from kURL merged with EC defaults
//	@Tags			migration
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.LinuxInstallationConfigResponse
//	@Router			/kurl-migration/config [get]
func (h *Handler) GetInstallationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := h.controller.GetInstallationConfig(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get installation config")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, config, h.logger)
}

// PostStartMigration handler to start a migration from kURL to Embedded Cluster
//
//	@ID				postStartMigration
//	@Summary		Start a migration from kURL to Embedded Cluster
//	@Description	Start a migration from kURL to Embedded Cluster with the provided configuration
//	@Tags			migration
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.StartKURLMigrationRequest	true	"Start Migration Request"
//	@Success		200		{object}	types.StartKURLMigrationResponse
//	@Failure		400		{object}	types.APIError
//	@Failure		409		{object}	types.APIError
//	@Router			/kurl-migration/start [post]
func (h *Handler) PostStartMigration(w http.ResponseWriter, r *http.Request) {
	var request types.StartKURLMigrationRequest
	if err := utils.BindJSON(w, r, &request, h.logger); err != nil {
		return
	}

	// Default transfer mode to "copy" if empty
	if request.TransferMode == "" {
		request.TransferMode = types.TransferModeCopy
	}

	// Use empty config if not provided
	config := types.LinuxInstallationConfig{}
	if request.Config != nil {
		config = *request.Config
	}

	migrationID, err := h.controller.StartKURLMigration(r.Context(), request.TransferMode, config)
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to start migration")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.StartKURLMigrationResponse{
		MigrationID: migrationID,
		Message:     "migration started successfully",
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}

// GetMigrationStatus handler to get the status of the migration
//
//	@ID				getMigrationStatus
//	@Summary		Get the status of the migration
//	@Description	Get the current status and progress of the migration
//	@Tags			migration
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.KURLMigrationStatusResponse
//	@Failure		404	{object}	types.APIError
//	@Router			/kurl-migration/status [get]
func (h *Handler) GetMigrationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.controller.GetKURLMigrationStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get migration status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, status, h.logger)
}
