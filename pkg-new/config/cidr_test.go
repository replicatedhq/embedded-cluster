package config

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name    string
		cidr    string
		wantErr bool
	}{
		{
			name:    "valid CIDR /16",
			cidr:    "10.0.0.0/16",
			wantErr: false,
		},
		{
			name:    "valid CIDR /8",
			cidr:    "10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "invalid CIDR - small subnet /24",
			cidr:    "192.168.1.0/24",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - non private subnet",
			cidr:    "4.0.0.0/8",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - empty string",
			cidr:    "",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - not a CIDR",
			cidr:    "not-a-cidr",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - missing prefix",
			cidr:    "10.0.0.0",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - invalid IP",
			cidr:    "999.999.999.999/16",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - negative prefix",
			cidr:    "10.0.0.0/-1",
			wantErr: true,
		},
		{
			name:    "invalid CIDR - prefix too large",
			cidr:    "10.0.0.0/33",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCIDR(tt.cidr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSplitCIDR(t *testing.T) {
	tests := []struct {
		name            string
		cidr            string
		wantPodCIDR     string
		wantServiceCIDR string
		wantErr         bool
	}{
		{
			name:            "valid CIDR /16",
			cidr:            "10.0.0.0/16",
			wantPodCIDR:     "10.0.0.0/17",
			wantServiceCIDR: "10.0.128.0/17",
			wantErr:         false,
		},
		{
			name:            "valid CIDR /8",
			cidr:            "10.0.0.0/8",
			wantPodCIDR:     "10.0.0.0/9",
			wantServiceCIDR: "10.128.0.0/9",
			wantErr:         false,
		},
		{
			name:            "invalid CIDR - empty string",
			cidr:            "",
			wantPodCIDR:     "",
			wantServiceCIDR: "",
			wantErr:         true,
		},
		{
			name:            "invalid CIDR - not a CIDR",
			cidr:            "not-a-cidr",
			wantPodCIDR:     "",
			wantServiceCIDR: "",
			wantErr:         true,
		},
		{
			name:            "invalid CIDR - cannot split /32",
			cidr:            "10.0.0.1/32",
			wantPodCIDR:     "",
			wantServiceCIDR: "",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPodCIDR, gotServiceCIDR, err := SplitCIDR(tt.cidr)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantPodCIDR, gotPodCIDR)
			assert.Equal(t, tt.wantServiceCIDR, gotServiceCIDR)
		})
	}
}

func TestCleanCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "clean CIDR with host bits set",
			input:   "192.168.1.5/24",
			want:    "192.168.1.0/24",
			wantErr: false,
		},
		{
			name:    "clean CIDR with host bits set /16",
			input:   "10.0.50.100/16",
			want:    "10.0.0.0/16",
			wantErr: false,
		},
		{
			name:    "already clean CIDR",
			input:   "192.168.0.0/24",
			want:    "192.168.0.0/24",
			wantErr: false,
		},
		{
			name:    "clean CIDR /8",
			input:   "10.50.100.200/8",
			want:    "10.0.0.0/8",
			wantErr: false,
		},
		{
			name:    "clean CIDR /32",
			input:   "192.168.1.5/32",
			want:    "192.168.1.5/32",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the input to create an IPNet
			_, ipnet, err := net.ParseCIDR(tt.input)
			require.NoError(t, err, "Failed to parse test input CIDR %q", tt.input)

			got, err := cleanCIDR(ipnet)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
