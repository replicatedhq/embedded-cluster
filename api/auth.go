package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type AuthRequest struct {
	Password string `json:"password"`
}

type AuthResponse struct {
	SessionToken string `json:"sessionToken"`
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionToken := r.Header.Get("Authorization")
		if sessionToken == "" {
			types.NewUnauthorizedError(errors.New("authorization header is required")).JSON(w)
			return
		}

		err := a.authController.ValidateToken(r.Context(), sessionToken)
		if err != nil {
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
		types.NewBadRequestError(err).JSON(w)
		return
	}

	sessionToken, err := a.authController.Authenticate(r.Context(), request.Password)
	if errors.Is(err, auth.ErrInvalidPassword) {
		types.NewUnauthorizedError(err).JSON(w)
		return
	} else if err != nil {
		types.NewInternalServerError(err).JSON(w)
		return
	}

	response := AuthResponse{
		SessionToken: sessionToken,
	}

	json.NewEncoder(w).Encode(response)
}
