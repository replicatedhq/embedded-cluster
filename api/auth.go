package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type AuthRequest struct {
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			err := errors.New("authorization header is required")
			a.logError(r, err, "failed to authenticate")
			types.NewUnauthorizedError(err).JSON(w)
			return
		}

		if !strings.HasPrefix(token, "Bearer ") {
			err := errors.New("authorization header must start with Bearer ")
			a.logError(r, err, "failed to authenticate")
			types.NewUnauthorizedError(err).JSON(w)
			return
		}

		token = token[len("Bearer "):]

		err := a.authController.ValidateToken(r.Context(), token)
		if err != nil {
			a.logError(r, err, "failed to validate token")
			types.NewUnauthorizedError(err).JSON(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *API) postAuthLogin(w http.ResponseWriter, r *http.Request) {
	var request AuthRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		a.logError(r, err, "failed to decode auth request")
		types.NewBadRequestError(err).JSON(w)
		return
	}

	token, err := a.authController.Authenticate(r.Context(), request.Password)
	if errors.Is(err, auth.ErrInvalidPassword) {
		types.NewUnauthorizedError(err).JSON(w)
		return
	}

	if err != nil {
		a.logError(r, err, "failed to authenticate")
		types.NewInternalServerError(err).JSON(w)
		return
	}

	response := AuthResponse{
		Token: token,
	}

	json.NewEncoder(w).Encode(response)
}
