package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendFieldError(t *testing.T) {
	tests := []struct {
		name string
		fe   func() *APIError
		want string
	}{
		{
			name: "empty",
			fe: func() *APIError {
				return nil
			},
			want: "",
		},
		{
			name: "single error",
			fe: func() *APIError {
				var fe *APIError
				fe = AppendFieldError(fe, "field1", errors.New("error1"))
				b, _ := json.Marshal(fe)
				fmt.Println(string(b))
				return fe
			},
			want: "field errors: error1",
		},
		{
			name: "multiple errors",
			fe: func() *APIError {
				var fe *APIError
				fe = AppendFieldError(fe, "field1", errors.New("error1"))
				fe = AppendFieldError(fe, "field2", errors.New("error2"))
				return fe
			},
			want: "field errors: error1; error2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fe := tt.fe()
			if got := fe.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAPIError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		newFunction func(err error) *APIError
		want        struct {
			statusCode int
			message    string
		}
	}{
		{
			name:        "BadRequestError",
			err:         errors.New("bad request"),
			newFunction: NewBadRequestError,
			want: struct {
				statusCode int
				message    string
			}{
				statusCode: http.StatusBadRequest,
				message:    "bad request",
			},
		},
		{
			name:        "UnauthorizedError",
			err:         errors.New("auth failed"),
			newFunction: NewUnauthorizedError,
			want: struct {
				statusCode int
				message    string
			}{
				statusCode: http.StatusUnauthorized,
				message:    "Unauthorized",
			},
		},
		{
			name:        "InternalServerError",
			newFunction: NewInternalServerError,
			err:         errors.New("internal server error"),
			want: struct {
				statusCode int
				message    string
			}{
				statusCode: http.StatusInternalServerError,
				message:    "internal server error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := tt.newFunction(tt.err)

			assert.Equal(t, tt.want.statusCode, apiErr.StatusCode, "StatusCode should match")
			assert.Equal(t, tt.want.message, apiErr.Message, "Message should match")
			assert.Equal(t, tt.err, apiErr.err, "Original error should be stored")
		})
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		apiError *APIError
		expected string
	}{
		{
			name:     "nil error",
			apiError: nil,
			expected: "",
		},
		{
			name: "error with message, no sub-errors",
			apiError: &APIError{
				Message: "main error message",
				Errors:  []*APIError{},
			},
			expected: "main error message",
		},
		{
			name: "error with message and one sub-error",
			apiError: &APIError{
				Message: "main error message",
				Errors: []*APIError{
					{Message: "sub-error 1"},
				},
			},
			expected: "main error message: sub-error 1",
		},
		{
			name: "error with message and multiple sub-errors",
			apiError: &APIError{
				Message: "main error message",
				Errors: []*APIError{
					{Message: "sub-error 1"},
					{Message: "sub-error 2"},
					{Message: "sub-error 3"},
				},
			},
			expected: "main error message: sub-error 1; sub-error 2; sub-error 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.apiError.Error()
			assert.Equal(t, tt.expected, got, "Error() should return the expected string")
		})
	}
}

func TestAPIError_ErrorOrNil(t *testing.T) {
	tests := []struct {
		name    string
		err     *APIError
		wantNil bool
	}{
		{
			name:    "nil error",
			err:     (*APIError)(nil),
			wantNil: true,
		},
		{
			name: "error without child errors",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "bad request",
				Errors:     nil,
			},
			wantNil: true,
		},
		{
			name: "error with empty errors slice",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "bad request",
				Errors:     []*APIError{},
			},
			wantNil: true,
		},
		{
			name: "error with child errors",
			err: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "field errors",
				Errors: []*APIError{
					{
						Message: "field1: invalid value",
						Field:   "field1",
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.ErrorOrNil()

			if tt.wantNil {
				assert.Nil(t, result, "ErrorOrNil() should return nil")
			} else {
				assert.NotNil(t, result, "ErrorOrNil() should not return nil")
				assert.Equal(t, tt.err, result, "ErrorOrNil() should return the error itself")
			}
		})
	}
}
