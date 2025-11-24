package kurlmigration

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateTransferMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    types.TransferMode
		wantErr bool
	}{
		{
			name:    "valid copy mode",
			mode:    types.TransferModeCopy,
			wantErr: false,
		},
		{
			name:    "valid move mode",
			mode:    types.TransferModeMove,
			wantErr: false,
		},
		{
			name:    "invalid empty mode",
			mode:    "",
			wantErr: true,
		},
		{
			name:    "invalid unknown mode",
			mode:    "unknown",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			err := m.ValidateTransferMode(tt.mode)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, types.ErrInvalidTransferMode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	tests := []struct {
		name       string
		userConfig types.LinuxInstallationConfig
		kurlConfig types.LinuxInstallationConfig
		defaults   types.LinuxInstallationConfig
		want       types.LinuxInstallationConfig
	}{
		{
			name: "user config takes precedence",
			userConfig: types.LinuxInstallationConfig{
				DataDirectory: "/user/data",
				PodCIDR:       "10.10.0.0/16",
			},
			kurlConfig: types.LinuxInstallationConfig{
				DataDirectory: "/kurl/data",
				PodCIDR:       "10.32.0.0/20",
			},
			defaults: types.LinuxInstallationConfig{
				DataDirectory: "/default/data",
				PodCIDR:       "10.0.0.0/16",
			},
			want: types.LinuxInstallationConfig{
				DataDirectory: "/user/data",
				PodCIDR:       "10.10.0.0/16",
			},
		},
		{
			name:       "kurl config takes precedence over defaults",
			userConfig: types.LinuxInstallationConfig{},
			kurlConfig: types.LinuxInstallationConfig{
				DataDirectory: "/kurl/data",
				ServiceCIDR:   "10.96.0.0/12",
			},
			defaults: types.LinuxInstallationConfig{
				DataDirectory: "/default/data",
				ServiceCIDR:   "10.1.0.0/16",
			},
			want: types.LinuxInstallationConfig{
				DataDirectory: "/kurl/data",
				ServiceCIDR:   "10.96.0.0/12",
			},
		},
		{
			name:       "defaults used when no overrides",
			userConfig: types.LinuxInstallationConfig{},
			kurlConfig: types.LinuxInstallationConfig{},
			defaults: types.LinuxInstallationConfig{
				DataDirectory: "/default/data",
				GlobalCIDR:    "10.2.0.0/16",
			},
			want: types.LinuxInstallationConfig{
				DataDirectory: "/default/data",
				GlobalCIDR:    "10.2.0.0/16",
			},
		},
		{
			name: "partial user override",
			userConfig: types.LinuxInstallationConfig{
				PodCIDR: "10.10.0.0/16",
			},
			kurlConfig: types.LinuxInstallationConfig{
				DataDirectory: "/kurl/data",
			},
			defaults: types.LinuxInstallationConfig{
				DataDirectory: "/default/data",
				HTTPProxy:     "http://proxy.example.com",
				PodCIDR:       "10.0.0.0/16",
			},
			want: types.LinuxInstallationConfig{
				PodCIDR:       "10.10.0.0/16",
				DataDirectory: "/kurl/data",
				HTTPProxy:     "http://proxy.example.com",
			},
		},
		{
			name: "all fields merged correctly",
			userConfig: types.LinuxInstallationConfig{
				PodCIDR: "10.10.0.0/16",
			},
			kurlConfig: types.LinuxInstallationConfig{
				DataDirectory:    "/kurl/data",
				NetworkInterface: "eth0",
			},
			defaults: types.LinuxInstallationConfig{
				DataDirectory:    "/default/data",
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "localhost",
				NetworkInterface: "eth1",
				PodCIDR:          "10.0.0.0/16",
				ServiceCIDR:      "10.1.0.0/16",
				GlobalCIDR:       "10.2.0.0/16",
			},
			want: types.LinuxInstallationConfig{
				PodCIDR:          "10.10.0.0/16",
				DataDirectory:    "/kurl/data",
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "localhost",
				NetworkInterface: "eth0",
				ServiceCIDR:      "10.1.0.0/16",
				GlobalCIDR:       "10.2.0.0/16",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			got := m.MergeConfigs(tt.userConfig, tt.kurlConfig, tt.defaults)
			assert.Equal(t, tt.want, got)
		})
	}
}
