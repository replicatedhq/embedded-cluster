package auth

import (
	"errors"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/handlers/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// PostLogin handler to authenticate a user
//
//	@ID				postAuthLogin
//	@Summary		Authenticate a user
//	@Description	Authenticate a user
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		types.AuthRequest	true	"Auth Request"
//	@Success		200		{object}	types.AuthResponse
//	@Failure		401		{object}	types.APIError
//	@Router			/auth/login [post]
func (h *Handler) PostLogin(w http.ResponseWriter, r *http.Request) {
	var request types.AuthRequest
	if err := utils.BindJSON(w, r, &request, h.logger); err != nil {
		return
	}

	token, err := h.authController.Authenticate(r.Context(), request.Password)
	if errors.Is(err, auth.ErrInvalidPassword) {
		utils.JSONError(w, r, types.NewUnauthorizedError(err), h.logger)
		return
	}

	if err != nil {
		utils.LogError(r, err, h.logger, "failed to authenticate")
		utils.JSONError(w, r, types.NewInternalServerError(err), h.logger)
		return
	}

	response := types.AuthResponse{
		Token: token,
	}

	utils.JSON(w, r, http.StatusOK, response, h.logger)
}
