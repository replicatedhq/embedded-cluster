package install

import (
	"strings"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func Test_formatAPIError(t *testing.T) {
	tests := []struct {
		name            string
		apiErr          *apitypes.APIError
		expectedResult  string
		containsStrings []string
		notContains     []string
	}{
		{
			name: "with field errors",
			apiErr: &apitypes.APIError{
				Message: "validation failed",
				Errors: []*apitypes.APIError{
					{
						Field:   "field1",
						Message: "error1",
					},
					{
						Field:   "field2",
						Message: "error2",
					},
				},
			},
			containsStrings: []string{
				"validation failed",
				"Field 'field1': error1",
				"Field 'field2': error2",
			},
		},
		{
			name: "without field errors",
			apiErr: &apitypes.APIError{
				Message: "simple error",
				Errors:  []*apitypes.APIError{},
			},
			expectedResult: "simple error",
		},
		{
			name: "with message only errors",
			apiErr: &apitypes.APIError{
				Message: "main error",
				Errors: []*apitypes.APIError{
					{
						Field:   "",
						Message: "sub error 1",
					},
					{
						Field:   "",
						Message: "sub error 2",
					},
				},
			},
			containsStrings: []string{
				"main error",
				"- sub error 1",
				"- sub error 2",
			},
			notContains: []string{"Field"},
		},
		{
			name:           "nil error",
			apiErr:         nil,
			expectedResult: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAPIError(tt.apiErr)

			// Check exact match if specified
			if tt.expectedResult != "" {
				assert.Equal(t, tt.expectedResult, result)
				return
			}

			// Check contains strings
			for _, str := range tt.containsStrings {
				assert.Contains(t, result, str)
			}

			// Check not contains strings
			for _, str := range tt.notContains {
				assert.NotContains(t, result, str)
			}

			// Ensure result doesn't end with newline
			assert.False(t, strings.HasSuffix(result, "\n"), "result should not end with newline")
		})
	}
}
