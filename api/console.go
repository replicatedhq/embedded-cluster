package api

import (
	"encoding/json"
	"net/http"
)

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

	json.NewEncoder(w).Encode(response)
}
