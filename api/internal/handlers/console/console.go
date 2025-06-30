package console

import (
	"fmt"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	logger            logrus.FieldLogger
	consoleController console.Controller
}

type Option func(*Handler)

func WithConsoleController(controller console.Controller) Option {
	return func(h *Handler) {
		h.consoleController = controller
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func New(opts ...Option) (*Handler, error) {
	h := &Handler{}

	for _, opt := range opts {
		opt(h)
	}

	if h.logger == nil {
		h.logger = logger.NewDiscardLogger()
	}

	if h.consoleController == nil {
		consoleController, err := console.NewConsoleController()
		if err != nil {
			return nil, fmt.Errorf("new console controller: %w", err)
		}
		h.consoleController = consoleController
	}

	return h, nil
}

// GetListAvailableNetworkInterfaces handler to list available network interfaces
//
//	@ID				getConsoleListAvailableNetworkInterfaces
//	@Summary		List available network interfaces
//	@Description	List available network interfaces
//	@Tags			console
//	@Produce		json
//	@Success		200	{object}	types.GetListAvailableNetworkInterfacesResponse
//	@Router			/console/available-network-interfaces [get]
func (h *Handler) GetListAvailableNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := h.consoleController.ListAvailableNetworkInterfaces()
	if err != nil {
		utils.LogError(r, err, h.logger, "failed to list available network interfaces")
		utils.JSONError(w, r, err, h.logger)
		return
	}

	h.logger.WithFields(utils.LogrusFieldsFromRequest(r)).
		WithField("interfaces", interfaces).
		Info("got available network interfaces")

	response := types.GetListAvailableNetworkInterfacesResponse{
		NetworkInterfaces: interfaces,
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
