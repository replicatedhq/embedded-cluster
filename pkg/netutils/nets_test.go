package netutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitNetworkCIDR(t *testing.T) {
	for _, tt := range []struct {
		name            string
		cidr            string
		expectedPodCIDR string
		expectedSvcCIDR string
		err             string
	}{
		{
			name:            "valid cidr",
			cidr:            "10.0.0.0/8",
			expectedPodCIDR: "10.0.0.0/9",
			expectedSvcCIDR: "10.128.0.0/9",
		},
		{
			name:            "a /16 cidr",
			cidr:            "10.1.0.0/16",
			expectedPodCIDR: "10.1.0.0/17",
			expectedSvcCIDR: "10.1.128.0/17",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			podnet, svcnet, err := SplitNetworkCIDR(tt.cidr)
			if err != nil {
				assert.NotEmpty(t, tt.err, "received unexpected error")
				assert.Contains(t, err.Error(), tt.err, "unexpected error message")
			}
			assert.Empty(t, tt.err, "unexpected error received")
			assert.Equal(t, tt.expectedPodCIDR, podnet, "unexpected pod cidr")
			assert.Equal(t, tt.expectedSvcCIDR, svcnet, "unexpected service cidr")
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	for _, tt := range []struct {
		name string
		cidr string
		err  string
	}{
		{
			name: "valid cidr",
			cidr: "10.0.0.0/16",
		},
		{
			name: "small cidr",
			cidr: "10.0.0.0/24",
			err:  "cidr needs to be at least a /16",
		},
		{
			name: "invalid cidr",
			cidr: "500.0.0.0/16",
			err:  "invalid CIDR address",
		},
		{
			name: "a /32 cidr",
			cidr: "10.0.0.0/32",
			err:  "cidr needs to be at least a /16",
		},
		{
			name: "a public cidr",
			cidr: "100.0.0.0/16",
			err:  "cidr is not in private ranges",
		},
		{
			name: "matching the whole private range",
			cidr: "192.168.0.0/16",
		},
		{
			name: "matching the whole 172 range",
			cidr: "172.16.0.0/12",
		},
		{
			name: "not a cidr address",
			cidr: "192.168.1.1/16",
			err:  "the provided address is not a valid CIDR",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateCIDR(tt.cidr, 16, true); err != nil {
				assert.NotEmpty(t, tt.err, "received unexpected error")
				assert.Contains(t, err.Error(), tt.err, "unexpected error message")
				return
			}
			assert.Empty(t, tt.err, "unexpected error received")
		})
	}
}

func TestAreAdjacentAndSameSize(t *testing.T) {
	for _, tt := range []struct {
		name    string
		a       string
		b       string
		want    bool
		wantErr bool
	}{
		{
			name: "two adjacent networks",
			a:    "10.96.0.0/16",
			b:    "10.97.0.0/16",
			want: true,
		},
		{
			name: "another two adjacent networks",
			a:    "10.1.0.0/17",
			b:    "10.1.128.0/17",
			want: true,
		},
		{
			name: "not adjacent networks",
			a:    "192.168.0.0/24",
			b:    "192.168.2.0/24",
			want: false,
		},
		{
			name: "adjacent with different masks",
			a:    "192.168.0.0/23",
			b:    "192.168.2.0/24",
			want: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := AreAdjacentAndSameSize(tt.a, tt.b)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitAndMergeNetworkCIDR(t *testing.T) {
	for _, tt := range []struct {
		name            string
		cidr            string
		expectedPodCIDR string
		expectedSvcCIDR string
	}{
		{
			name: "two adjacent networks (/8)",
			cidr: "10.0.0.0/8",
		},
		{
			name: "two adjacent networks (/24)",
			cidr: "10.0.0.0/24",
		},
		{
			name: "small network (/30)",
			cidr: "192.168.1.0/30",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			podnet, svcnet, err := SplitNetworkCIDR(tt.cidr)
			assert.NoError(t, err)
			adjacent, supernet, err := AreAdjacentAndSameSize(podnet, svcnet)
			assert.NoError(t, err)
			assert.True(t, adjacent)
			assert.Equal(t, tt.cidr, supernet)
		})
	}
}
