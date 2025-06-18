package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfraSetupRequest(t *testing.T) {
	tests := []struct {
		name                   string
		request                InfraSetupRequest
		expectedIgnoreFailures bool
	}{
		{
			name: "ignore preflight failures true",
			request: InfraSetupRequest{
				IgnorePreflightFailures: true,
			},
			expectedIgnoreFailures: true,
		},
		{
			name: "ignore preflight failures false",
			request: InfraSetupRequest{
				IgnorePreflightFailures: false,
			},
			expectedIgnoreFailures: false,
		},
		{
			name:                   "ignore preflight failures default",
			request:                InfraSetupRequest{},
			expectedIgnoreFailures: false, // Default should be false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedIgnoreFailures, tt.request.IgnorePreflightFailures)
		})
	}
}
