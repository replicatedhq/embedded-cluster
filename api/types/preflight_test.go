package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostPreflightsAllowIgnore(t *testing.T) {
	tests := []struct {
		name                string
		hostPreflights      HostPreflights
		expectedAllowIgnore bool
	}{
		{
			name: "allow ignore host preflights true",
			hostPreflights: HostPreflights{
				Titles: []string{"Test"},
				Output: &HostPreflightsOutput{
					Pass: []HostPreflightsRecord{{Title: "Pass", Message: "OK"}},
				},
				Status:                    NewStatus(),
				AllowIgnoreHostPreflights: true,
			},
			expectedAllowIgnore: true,
		},
		{
			name: "allow ignore host preflights false",
			hostPreflights: HostPreflights{
				Titles: []string{"Test"},
				Output: &HostPreflightsOutput{
					Pass: []HostPreflightsRecord{{Title: "Pass", Message: "OK"}},
				},
				Status:                    NewStatus(),
				AllowIgnoreHostPreflights: false,
			},
			expectedAllowIgnore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedAllowIgnore, tt.hostPreflights.AllowIgnoreHostPreflights)
		})
	}
}
