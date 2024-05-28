package helpers

import (
	"testing"
)

func TestGetLowerBandIP(t *testing.T) {
	validTests := []struct {
		cidr   string
		index  int
		expect string
	}{
		{"10.96.0.0/24", 0, "10.96.0.1"},
		{"10.96.0.0/24", 4, "10.96.0.5"},
		{"10.96.0.0/24", 15, "10.96.0.16"},
		{"192.168.1.0/24", 0, "192.168.1.1"},
		{"192.168.1.0/24", 7, "192.168.1.8"},
		{"172.16.0.0/28", 0, "172.16.0.1"},
		{"172.16.0.0/28", 14, "172.16.0.15"},
	}
	for _, tt := range validTests {
		t.Run(tt.cidr, func(t *testing.T) {
			ip, err := GetLowerBandIP(tt.cidr, tt.index)
			if err != nil {
				t.Errorf("GetLowerBandIP() error = %v", err)
				return
			}
			if ip.String() != tt.expect {
				t.Errorf("GetLowerBandIP() = %v, want %v", ip.String(), tt.expect)
			}
		})
	}

	invalidTests := []struct {
		cidr  string
		index int
	}{
		{"10.96.0.0/24", 16},
		{"192.168.1.0/24", 255},
		{"172.16.0.0/28", 16},
	}
	for _, tt := range invalidTests {
		t.Run(tt.cidr, func(t *testing.T) {
			ip, err := GetLowerBandIP(tt.cidr, tt.index)
			if err == nil {
				t.Errorf("GetLowerBandIP() = %v, want error", ip)
			}
		})
	}
}
