package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      Status
		expectedErr bool
	}{
		{
			name: "valid status - pending",
			status: Status{
				State:       StatePending,
				Description: "Installation pending",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - running",
			status: Status{
				State:       StateRunning,
				Description: "Installation in progress",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - succeeded",
			status: Status{
				State:       StateSucceeded,
				Description: "Installation completed successfully",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name: "valid status - failed",
			status: Status{
				State:       StateFailed,
				Description: "Installation failed",
				LastUpdated: time.Now(),
			},
			expectedErr: false,
		},
		{
			name:        "empty status",
			status:      Status{},
			expectedErr: true,
		},
		{
			name: "invalid state",
			status: Status{
				State:       "Invalid",
				Description: "Invalid state",
				LastUpdated: time.Now(),
			},
			expectedErr: true,
		},
		{
			name: "missing description",
			status: Status{
				State:       StateRunning,
				Description: "",
				LastUpdated: time.Now(),
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatus(tt.status)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
