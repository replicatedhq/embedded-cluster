package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallationConfigIgnoreHostPreflights(t *testing.T) {
	tests := []struct {
		name     string
		config   InstallationConfig
		expected bool
	}{
		{
			name: "ignore host preflights true",
			config: InstallationConfig{
				AdminConsolePort:     8800,
				IgnoreHostPreflights: true,
			},
			expected: true,
		},
		{
			name: "ignore host preflights false",
			config: InstallationConfig{
				AdminConsolePort:     8800,
				IgnoreHostPreflights: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.config.IgnoreHostPreflights)
		})
	}
}
