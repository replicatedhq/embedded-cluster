package kurlmigration

import (
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/kurlmigration"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	cfg        types.APIConfig
	logger     logrus.FieldLogger
	controller kurlmigration.Controller
}

type Option func(*Handler)

func WithLogger(log logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = log
	}
}

func WithController(controller kurlmigration.Controller) Option {
	return func(h *Handler) {
		h.controller = controller
	}
}

func New(cfg types.APIConfig, opts ...Option) (*Handler, error) {
	h := &Handler{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(h)
	}

	if h.logger == nil {
		h.logger = logger.NewDiscardLogger()
	}

	// Create controller internally if not provided via option
	if h.controller == nil {
		// Create file-based store for state persistence
		dataDir := h.cfg.RuntimeConfig.EmbeddedClusterHomeDirectory()
		s := store.NewStoreWithDataDir(dataDir)

		controller, err := kurlmigration.NewKURLMigrationController(
			kurlmigration.WithStore(s),
			kurlmigration.WithLogger(h.logger),
		)
		if err != nil {
			return nil, fmt.Errorf("create kurl migration controller: %w", err)
		}
		h.controller = controller
	}

	return h, nil
}

// GetInstallationConfig handler to get the installation config for kURL migration
//
//	@ID				getKURLMigrationInstallationConfig
//	@Summary		Get the installation config for kURL migration
//	@Description	Get the installation config extracted from kURL merged with EC defaults
//	@Tags			kurl-migration
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.LinuxInstallationConfigResponse
//	@Router			/linux/kurl-migration/config [get]
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
//	@Tags			kurl-migration
//	@Security		bearerauth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.StartKURLMigrationRequest	true	"Start kURL Migration Request"
//	@Success		200		{object}	types.StartKURLMigrationResponse
//	@Failure		400		{object}	types.APIError
//	@Failure		409		{object}	types.APIError
//	@Router			/linux/kurl-migration/start [post]
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
		utils.LogError(r, err, h.logger, "failed to start kURL migration")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	response := types.StartKURLMigrationResponse{
		MigrationID: migrationID,
		Message:     "kURL migration started successfully",
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}

// GetMigrationStatus handler to get the status of the kURL migration
//
//	@ID				getKURLMigrationStatus
//	@Summary		Get the status of the kURL migration
//	@Description	Get the current status and progress of the kURL migration
//	@Tags			kurl-migration
//	@Security		bearerauth
//	@Produce		json
//	@Success		200	{object}	types.KURLMigrationStatusResponse
//	@Failure		404	{object}	types.APIError
//	@Router			/linux/kurl-migration/status [get]
func (h *Handler) GetMigrationStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.controller.GetKURLMigrationStatus(r.Context())
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to get kURL migration status")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	utils.JSON(w, r, http.StatusOK, status, h.logger)
}
