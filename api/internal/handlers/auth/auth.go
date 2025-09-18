package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

type Handler struct {
	logger         logrus.FieldLogger
	authController auth.Controller
}

type Option func(*Handler)

func WithAuthController(controller auth.Controller) Option {
	return func(h *Handler) {
		h.authController = controller
	}
}

func WithLogger(logger logrus.FieldLogger) Option {
	return func(h *Handler) {
		h.logger = logger
	}
}

func New(passwordHash []byte, opts ...Option) (*Handler, error) {
	h := &Handler{}

	for _, opt := range opts {
		opt(h)
	}

	if h.logger == nil {
		h.logger = logger.NewDiscardLogger()
	}

	if h.authController == nil {
		authController, err := auth.NewAuthController(passwordHash)
		if err != nil {
			return nil, fmt.Errorf("new auth controller: %w", err)
		}
		h.authController = authController
	}

	return h, nil
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			err := errors.New("authorization header is required")
			utils.LogError(r, err, h.logger, "failed to authenticate")
			utils.JSONError(w, r, types.NewUnauthorizedError(err), h.logger)
			return
		}

		if !strings.HasPrefix(token, "Bearer ") {
			err := errors.New("authorization header must start with Bearer ")
			utils.LogError(r, err, h.logger, "failed to authenticate")
			utils.JSONError(w, r, types.NewUnauthorizedError(err), h.logger)
			return
		}

		token = token[len("Bearer "):]

		err := h.authController.ValidateToken(r.Context(), token)
		if err != nil {
			utils.LogError(r, err, h.logger, "failed to validate token")
			utils.JSONError(w, r, types.NewUnauthorizedError(err), h.logger)
			return
		}

		next.ServeHTTP(w, r)
	})
}
