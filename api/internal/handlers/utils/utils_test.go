package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func TestAPI_jsonError(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *types.APIError
		wantCode int
		wantJSON map[string]any
	}{
		{
			name: "simple error",
			apiErr: &types.APIError{
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
			apiErr: &types.APIError{
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
			apiErr: &types.APIError{
				StatusCode: http.StatusBadRequest,
				Message:    "multiple validation errors",
				Errors: []*types.APIError{
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
			JSONError(rec, httptest.NewRequest("GET", "/api/test", nil), tt.apiErr, logger.NewDiscardLogger())

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
