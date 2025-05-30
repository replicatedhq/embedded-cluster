package types

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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
	return AppendError(apiErr, newFieldError(field, err))
}

func camelCaseToWords(s string) string {
	// Handle special cases
	specialCases := map[string]string{
		"cidr": "CIDR",
		"Cidr": "CIDR",
		"CIDR": "CIDR",
	}

	// Check if the entire string is a special case
	if replacement, ok := specialCases[strings.ToLower(s)]; ok {
		return replacement
	}

	// Split on capital letters
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	words := re.ReplaceAllString(s, "$1 $2")

	// Split the words and handle special cases
	wordList := strings.Split(strings.ToLower(words), " ")
	for i, word := range wordList {
		if replacement, ok := specialCases[word]; ok {
			wordList[i] = replacement
		} else {
			// Capitalize other words
			c := cases.Title(language.English)
			wordList[i] = c.String(word)
		}
	}

	return strings.Join(wordList, " ")
}

func newFieldError(field string, err error) *APIError {
	msg := err.Error()

	// Try different patterns to replace the field name
	patterns := []string{
		field,                  // exact match
		strings.ToLower(field), // lowercase
		strings.ToUpper(field), // uppercase
		"cidr",                 // special case for CIDR
	}

	for _, pattern := range patterns {
		if strings.Contains(msg, pattern) {
			msg = strings.Replace(msg, pattern, camelCaseToWords(field), 1)
			break
		}
	}

	return &APIError{
		Message: msg,
		Field:   field,
		err:     err,
	}
}
