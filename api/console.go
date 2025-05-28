package api

import (
	"net/http"
)

type getListAvailableNetworkInterfacesResponse struct {
	NetworkInterfaces []string `json:"networkInterfaces"`
}

func (a *API) getListAvailableNetworkInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := a.consoleController.ListAvailableNetworkInterfaces()
	if err != nil {
		a.logError(r, err, "failed to list available network interfaces")
		a.jsonError(w, r, err)
		return
	}

	a.logger.WithFields(logrusFieldsFromRequest(r)).
		WithField("interfaces", interfaces).
		Info("got available network interfaces")

	response := getListAvailableNetworkInterfacesResponse{
		NetworkInterfaces: interfaces,
	}

	a.json(w, r, http.StatusOK, response)
}
