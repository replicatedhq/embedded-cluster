package cli

import (
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func TestCanBypassHostPreflights(t *testing.T) {
	tests := []struct {
		name                 string
		output               apitypes.PreflightsOutput
		ignoreHostPreflights bool
		expected             bool
	}{
		{
			name: "non-strict failure with the flag set can be bypassed",
			output: apitypes.PreflightsOutput{
				Fail: []apitypes.PreflightsRecord{{Title: "Check 1", Strict: false}},
			},
			ignoreHostPreflights: true,
			expected:             true,
		},
		{
			name: "strict failure cannot be bypassed even with the flag set",
			output: apitypes.PreflightsOutput{
				Fail: []apitypes.PreflightsRecord{{Title: "Check 1", Strict: true}},
			},
			ignoreHostPreflights: true,
			expected:             false,
		},
		{
			name: "a single strict failure blocks a bypass of the whole run",
			output: apitypes.PreflightsOutput{
				Fail: []apitypes.PreflightsRecord{
					{Title: "Check 1", Strict: false},
					{Title: "Check 2", Strict: true},
				},
			},
			ignoreHostPreflights: true,
			expected:             false,
		},
		{
			name: "non-strict failure without the flag cannot be bypassed",
			output: apitypes.PreflightsOutput{
				Fail: []apitypes.PreflightsRecord{{Title: "Check 1", Strict: false}},
			},
			ignoreHostPreflights: false,
			expected:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, canBypassHostPreflights(&tt.output, tt.ignoreHostPreflights))
		})
	}
}
