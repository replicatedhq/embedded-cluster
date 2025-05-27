package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
			want: "field errors: field1: error1",
		},
		{
			name: "multiple errors",
			fe: func() *APIError {
				var fe *APIError
				fe = AppendFieldError(fe, "field1", errors.New("error1"))
				fe = AppendFieldError(fe, "field2", errors.New("error2"))
				return fe
			},
			want: "field errors: field1: error1; field2: error2",
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
				Message:    "validation failed",
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

func TestAPIError_JSON(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		wantCode int
		wantJSON map[string]any
	}{
		{
			name: "simple error",
			apiErr: &APIError{
				StatusCode: http.StatusInternalServerError,
				Message:    "invalid request",
			},
			wantCode: http.StatusInternalServerError,
			wantJSON: map[string]any{
				"status_code": float64(http.StatusInternalServerError),
				"message":     "invalid request",
			},
		},
		{
			name: "field error",
			apiErr: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "validation error",
				Field:      "username",
			},
			wantCode: http.StatusBadRequest,
			wantJSON: map[string]any{
				"status_code": float64(http.StatusBadRequest),
				"message":     "validation error",
				"field":       "username",
			},
		},
		{
			name: "error with nested errors",
			apiErr: &APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "multiple validation errors",
				Errors: []*APIError{
					{
						Message: "field1 is required",
						Field:   "field1",
					},
					{
						Message: "field2 must be a number",
						Field:   "field2",
					},
				},
			},
			wantCode: http.StatusBadRequest,
			wantJSON: map[string]any{
				"status_code": float64(http.StatusBadRequest),
				"message":     "multiple validation errors",
				"errors": []any{
					map[string]any{
						"message": "field1 is required",
						"field":   "field1",
					},
					map[string]any{
						"message": "field2 must be a number",
						"field":   "field2",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP response recorder
			rec := httptest.NewRecorder()

			// Call the JSON method
			tt.apiErr.JSON(rec)

			// Check status code
			assert.Equal(t, tt.wantCode, rec.Code, "Status code should match")

			// Check content type header
			contentType := rec.Header().Get("Content-Type")
			assert.Equal(t, "application/json", contentType, "Content-Type header should be application/json")

			// Parse and check the JSON response
			var gotJSON map[string]any
			err := json.Unmarshal(rec.Body.Bytes(), &gotJSON)
			assert.NoError(t, err, "Should be able to parse the JSON response")
			assert.Equal(t, tt.wantJSON, gotJSON, "JSON response should match expected structure")
		})
	}
}
