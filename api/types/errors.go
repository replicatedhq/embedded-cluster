package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

type APIError struct {
	StatusCode int         `json:"status_code,omitempty"`
	Message    string      `json:"message"`
	Field      string      `json:"field,omitempty"`
	Errors     []*APIError `json:"errors,omitempty"`

	err error `json:"-"`
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	} else if len(e.Errors) == 0 {
		return e.Message
	}
	var buf bytes.Buffer
	first := true
	for _, ee := range e.Errors {
		if first {
			first = false
		} else {
			buf.WriteString("; ")
		}
		buf.WriteString(ee.Message)
	}
	return fmt.Sprintf("%s: %s", e.Message, buf.String())
}

func (e *APIError) ErrorOrNil() error {
	if e == nil || len(e.Errors) == 0 {
		return nil
	}
	return e
}

func (e *APIError) Unwrap() error {
	return e.err
}

func NewBadRequestError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusBadRequest,
		Message:    err.Error(),
		err:        err,
	}
}

func NewConflictError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusConflict,
		Message:    err.Error(),
		err:        err,
	}
}

func NewForbiddenError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusForbidden,
		Message:    err.Error(),
		err:        err,
	}
}

func NewUnauthorizedError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    "Unauthorized",
		err:        err,
	}
}

func NewInternalServerError(err error) *APIError {
	return &APIError{
		StatusCode: http.StatusInternalServerError,
		Message:    err.Error(),
		err:        err,
	}
}

func AppendError(apiErr *APIError, errs ...*APIError) *APIError {
	var nonNilErrs []*APIError
	for _, err := range errs {
		if err != nil {
			nonNilErrs = append(nonNilErrs, err)
		}
	}
	if len(nonNilErrs) == 0 {
		return apiErr
	}
	if apiErr == nil {
		apiErr = NewInternalServerError(errors.New("errors"))
	}
	apiErr.Errors = append(apiErr.Errors, nonNilErrs...)
	return apiErr
}

func AppendFieldError(apiErr *APIError, field string, err error) *APIError {
	if apiErr == nil {
		apiErr = NewBadRequestError(errors.New("field errors"))
	}
	return AppendError(apiErr, &APIError{
		Message: err.Error(),
		Field:   field,
		err:     err,
	})
}

// JSON writes the APIError as JSON to the provided http.ResponseWriter
func (e *APIError) JSON(w http.ResponseWriter) {
	response, err := json.Marshal(e)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	w.Write(response)
}
