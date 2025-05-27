package api

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// getHealth handler to get the health of the API
//
//	@Summary		Get the health of the API
//	@Description	get the health of the API
//	@Tags			health
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	types.Health
//	@Router			/health [get]
func (a *API) getHealth(w http.ResponseWriter, r *http.Request) {
	response := types.Health{
		Status: types.HealthStatusOK,
	}
	a.json(w, r, http.StatusOK, response)
}
