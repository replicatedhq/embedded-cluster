package api

import (
	"encoding/json"
	"net/http"
)

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
}

func (e *APIError) Error() string {
	return e.Err.Error()
}

func (e *APIError) JSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(e)
}

func NewBadRequestError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusBadRequest,
		Message:    err.Error(),
		Err:        err,
	}
}
