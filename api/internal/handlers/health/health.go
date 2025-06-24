package health

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	logger logrus.FieldLogger
}

type Option func(*Handler)

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

	return h, nil
}

// GetHealth handler to get the health of the API
//
//	@ID				getHealth
//	@Summary		Get the health of the API
//	@Description	get the health of the API
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	types.Health
//	@Router			/health [get]
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	response := types.Health{
		Status: types.HealthStatusOK,
	}
	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
