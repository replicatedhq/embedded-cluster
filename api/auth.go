package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			err := errors.New("authorization header is required")
			a.logError(r, err, "failed to authenticate")
			a.jsonError(w, r, types.NewUnauthorizedError(err))
			return
		}

		if !strings.HasPrefix(token, "Bearer ") {
			err := errors.New("authorization header must start with Bearer ")
			a.logError(r, err, "failed to authenticate")
			a.jsonError(w, r, types.NewUnauthorizedError(err))
			return
		}

		token = token[len("Bearer "):]

		err := a.authController.ValidateToken(r.Context(), token)
		if err != nil {
			a.logError(r, err, "failed to validate token")
			a.jsonError(w, r, types.NewUnauthorizedError(err))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// postAuthLogin handler to authenticate a user
//
//	@Summary		Authenticate a user
//	@Description	Authenticate a user
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.AuthRequest	true	"Auth Request"
//	@Success		200		{object}	types.AuthResponse
//	@Failure		401		{object}	types.APIError
//	@Router			/auth/login [post]
func (a *API) postAuthLogin(w http.ResponseWriter, r *http.Request) {
	var request types.AuthRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		a.logError(r, err, "failed to decode auth request")
		a.jsonError(w, r, types.NewBadRequestError(err))
		return
	}

	token, err := a.authController.Authenticate(r.Context(), request.Password)
	if errors.Is(err, auth.ErrInvalidPassword) {
		a.jsonError(w, r, types.NewUnauthorizedError(err))
		return
	}

	if err != nil {
		a.logError(r, err, "failed to authenticate")
		a.jsonError(w, r, types.NewInternalServerError(err))
		return
	}

	response := types.AuthResponse{
		Token: token,
	}

	a.json(w, r, http.StatusOK, response)
}
