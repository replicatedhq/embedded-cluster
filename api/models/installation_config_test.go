package models

import (
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO: this should test the API handler
func TestInstallationConfig_setCIDRDefaults(t *testing.T) {
	defaultServiceCIDR := k0sv1beta1.DefaultNetwork().ServiceCIDR

	tests := []struct {
		name                 string
		installationConfig   InstallationConfig
		expected             InstallationConfig
		expectVaidationError bool
	}{
		{
			name: "with pod and service flags",
			installationConfig: InstallationConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  "",
			},
			expected: InstallationConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  "",
			},
		},
		{
			name: "with pod flag",
			installationConfig: InstallationConfig{
				PodCIDR: "10.0.0.0/24",
			},
			expected: InstallationConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: defaultServiceCIDR,
				GlobalCIDR:  "",
			},
		},
		{
			name: "with pod, service and cidr flags",
			installationConfig: InstallationConfig{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
				GlobalCIDR:  "10.1.0.0/16",
			},
			expected: InstallationConfig{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
				GlobalCIDR:  "10.1.0.0/16",
			},
		},
		{
			name: "with pod and cidr flags",
			installationConfig: InstallationConfig{
				PodCIDR:    "10.0.0.0/17",
				GlobalCIDR: "10.2.0.0/16",
			},
			expected: InstallationConfig{
				PodCIDR:     "10.0.0.0/17",
				ServiceCIDR: defaultServiceCIDR,
				GlobalCIDR:  "10.2.0.0/16",
			},
			expectVaidationError: true,
		},
		{
			name: "with cidr flag",
			installationConfig: InstallationConfig{
				GlobalCIDR: "10.2.0.0/16",
			},
			expected: InstallationConfig{
				PodCIDR:     "10.2.0.0/17",
				ServiceCIDR: "10.2.128.0/17",
				GlobalCIDR:  "10.2.0.0/16",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.installationConfig.setCIDRDefaults()
			require.NoError(t, err)
			assert.Equal(t, test.expected, test.installationConfig)

			validateErr := test.installationConfig.validateCIDR()
			if test.expectVaidationError {
				assert.Error(t, validateErr)
			} else {
				assert.NoError(t, validateErr)
			}
		})
	}
}
