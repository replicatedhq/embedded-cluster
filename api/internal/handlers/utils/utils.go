package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/sirupsen/logrus"
)

// Shared helper functions for all handler packages

func BindJSON(w http.ResponseWriter, r *http.Request, v any, logger logrus.FieldLogger) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		LogError(r, err, logger, fmt.Sprintf("failed to decode %s %s request", strings.ToLower(r.Method), r.URL.Path))
		JSONError(w, r, types.NewBadRequestError(err), logger)
		return err
	}
	return nil
}

func JSON(w http.ResponseWriter, r *http.Request, code int, payload any, logger logrus.FieldLogger) {
	response, err := json.Marshal(payload)
	if err != nil {
		LogError(r, err, logger, "failed to encode response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

func JSONError(w http.ResponseWriter, r *http.Request, err error, logger logrus.FieldLogger) {
	var apiErr *types.APIError
	if !errors.As(err, &apiErr) {
		apiErr = types.NewInternalServerError(err)
	}
	response, err := json.Marshal(apiErr)
	if err != nil {
		LogError(r, err, logger, "failed to encode response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.StatusCode)
	_, _ = w.Write(response)
}

func LogError(r *http.Request, err error, logger logrus.FieldLogger, args ...any) {
	logger.WithFields(LogrusFieldsFromRequest(r)).WithError(err).Error(args...)
}

func LogrusFieldsFromRequest(r *http.Request) logrus.Fields {
	return logrus.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	}
}
