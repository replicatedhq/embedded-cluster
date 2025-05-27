package api

import (
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type getBrandingResponse struct {
	Branding types.Branding `json:"branding"`
}

func (a *API) getBranding(w http.ResponseWriter, r *http.Request) {
	branding, err := a.consoleController.GetBranding()
	if err != nil {
		a.logError(r, err, "failed to get branding")
		a.JSONError(w, r, err)
		return
	}

	response := getBrandingResponse{
		Branding: branding,
	}

	a.JSON(w, r, http.StatusOK, response)
}

type getListAvailableNetworkInterfacesResponse struct {
	NetworkInterfaces []string `json:"networkInterfaces"`
}

func (a *API) getListAvailableNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := a.consoleController.ListAvailableNetworkInterfaces()
	if err != nil {
		a.logError(r, err, "failed to list available network interfaces")
		a.JSONError(w, r, err)
		return
	}

	a.logger.WithFields(logrusFieldsFromRequest(r)).
		WithField("interfaces", interfaces).
		Info("got available network interfaces")

	response := getListAvailableNetworkInterfacesResponse{
		NetworkInterfaces: interfaces,
	}

	a.JSON(w, r, http.StatusOK, response)
}
