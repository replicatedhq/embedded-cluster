package api

import (
	"encoding/json"
	"net/http"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type getBrandingResponse struct {
	Branding types.Branding `json:"branding"`
}

func (a *API) getBranding(w http.ResponseWriter, r *http.Request) {
	branding, err := a.consoleController.GetBranding()
	if err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to get branding")
		handleError(w, err)
		return
	}

	response := getBrandingResponse{
		Branding: branding,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to encode branding")
	}
}

type getListAvailableNetworkInterfacesResponse struct {
	NetworkInterfaces []string `json:"networkInterfaces"`
}

func (a *API) getListAvailableNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := a.consoleController.ListAvailableNetworkInterfaces()
	if err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to list available network interfaces")
		handleError(w, err)
		return
	}

	a.logger.WithFields(logrusFieldsFromRequest(r)).
		WithField("interfaces", interfaces).
		Info("got available network interfaces")

	response := getListAvailableNetworkInterfacesResponse{
		NetworkInterfaces: interfaces,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.logger.WithFields(logrusFieldsFromRequest(r)).WithError(err).
			Error("failed to encode available network interfaces")
	}
}
