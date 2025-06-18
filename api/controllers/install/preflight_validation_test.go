package install

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func TestValidatePreflightStatus(t *testing.T) {
	tests := []struct {
		name                 string
		preflightStatus      *types.HostPreflights
		ignoreFailures       bool
		expectedError        bool
		expectedErrorMessage string
	}{
		{
			name: "preflights passed - should allow setup",
			preflightStatus: &types.HostPreflights{
				Status: &types.Status{State: "Succeeded"},
				Output: &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{{Title: "Test", Message: "OK"}},
					Fail: []types.HostPreflightsRecord{},
				},
				AllowIgnoreHostPreflights: false,
			},
			ignoreFailures: false,
			expectedError:  false,
		},
		{
			name: "preflights failed + CLI flag + client wants to ignore - should allow setup",
			preflightStatus: &types.HostPreflights{
				Status: &types.Status{State: "Failed"},
				Output: &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{},
					Fail: []types.HostPreflightsRecord{{Title: "Test", Message: "Failed"}},
				},
				AllowIgnoreHostPreflights: true, // CLI flag was used
			},
			ignoreFailures: true, // Client wants to ignore
			expectedError:  false,
		},
		{
			name: "preflights failed + no CLI flag - should block setup",
			preflightStatus: &types.HostPreflights{
				Status: &types.Status{State: "Failed"},
				Output: &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{},
					Fail: []types.HostPreflightsRecord{{Title: "Test", Message: "Failed"}},
				},
				AllowIgnoreHostPreflights: false, // No CLI flag
			},
			ignoreFailures:       true, // Client wants to ignore but can't
			expectedError:        true,
			expectedErrorMessage: "Cannot ignore preflight failures without --ignore-host-preflights flag",
		},
		{
			name: "preflights failed + CLI flag + client doesn't want to ignore - should block setup",
			preflightStatus: &types.HostPreflights{
				Status: &types.Status{State: "Failed"},
				Output: &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{},
					Fail: []types.HostPreflightsRecord{{Title: "Test", Message: "Failed"}},
				},
				AllowIgnoreHostPreflights: true, // CLI flag was used
			},
			ignoreFailures:       false, // Client doesn't want to ignore
			expectedError:        true,
			expectedErrorMessage: "Preflight checks failed",
		},
		{
			name: "preflights failed + no CLI flag + client doesn't want to ignore - should block setup",
			preflightStatus: &types.HostPreflights{
				Status: &types.Status{State: "Failed"},
				Output: &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{},
					Fail: []types.HostPreflightsRecord{{Title: "Test", Message: "Failed"}},
				},
				AllowIgnoreHostPreflights: false, // No CLI flag
			},
			ignoreFailures:       false, // Client doesn't want to ignore
			expectedError:        true,
			expectedErrorMessage: "Preflight checks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePreflightStatus(tt.preflightStatus, tt.ignoreFailures)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
