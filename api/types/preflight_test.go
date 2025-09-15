package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasStrictFailures(t *testing.T) {
	tests := []struct {
		name     string
		output   PreflightsOutput
		expected bool
	}{
		{
			name: "no failures",
			output: PreflightsOutput{
				Pass: []PreflightsRecord{{Title: "Check 1", Message: "OK"}},
				Fail: []PreflightsRecord{},
			},
			expected: false,
		},
		{
			name: "non-strict failures only",
			output: PreflightsOutput{
				Fail: []PreflightsRecord{
					{Title: "Check 1", Message: "Failed", Strict: false},
					{Title: "Check 2", Message: "Failed", Strict: false},
				},
			},
			expected: false,
		},
		{
			name: "strict failure exists",
			output: PreflightsOutput{
				Fail: []PreflightsRecord{
					{Title: "Check 1", Message: "Failed", Strict: true},
					{Title: "Check 2", Message: "Failed", Strict: false},
				},
			},
			expected: true,
		},
		{
			name: "multiple strict failures",
			output: PreflightsOutput{
				Fail: []PreflightsRecord{
					{Title: "Check 1", Message: "Failed", Strict: true},
					{Title: "Check 2", Message: "Failed", Strict: true},
				},
			},
			expected: true,
		},
		{
			name: "strict failure in mixed results",
			output: PreflightsOutput{
				Pass: []PreflightsRecord{{Title: "Check 1", Message: "OK"}},
				Warn: []PreflightsRecord{{Title: "Check 2", Message: "Warning"}},
				Fail: []PreflightsRecord{
					{Title: "Check 3", Message: "Failed", Strict: false},
					{Title: "Check 4", Message: "Critical failure", Strict: true},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.output.HasStrictFailures()
			assert.Equal(t, tt.expected, result)
		})
	}
}
