package config

import (
	"net"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/stretchr/testify/assert"
)

// Mock NetworkInterface implementation
type mockNetworkInterface struct {
	name  string
	flags net.Flags
	addrs []net.Addr
	err   error
}

func (m *mockNetworkInterface) Name() string {
	return m.name
}

func (m *mockNetworkInterface) Flags() net.Flags {
	return m.flags
}

func (m *mockNetworkInterface) Addrs() ([]net.Addr, error) {
	return m.addrs, m.err
}

// Mock NetworkInterfaceProvider implementation
type mockNetworkInterfaceProvider struct {
	interfaces []netutils.NetworkInterface
	err        error
}

func (m *mockNetworkInterfaceProvider) Interfaces() ([]netutils.NetworkInterface, error) {
	return m.interfaces, m.err
}

func TestDetermineBestNetworkInterface(t *testing.T) {
	// Save original variables
	originalChooseHostInterface := ChooseHostInterface
	originalNetworkInterfaceProvider := NetworkInterfaceProvider
	defer func() {
		ChooseHostInterface = originalChooseHostInterface
		NetworkInterfaceProvider = originalNetworkInterfaceProvider
	}()

	tests := []struct {
		name                         string
		mockChooseHostInterface      func() (net.IP, error)
		mockNetworkInterfaceProvider netutils.NetworkInterfaceProvider
		expectedResult               string
		expectedError                error
	}{
		{
			name: "successful interface determination",
			mockChooseHostInterface: func() (net.IP, error) {
				return net.ParseIP("192.168.1.100"), nil
			},
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name: "eth0",
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedResult: "eth0",
			expectedError:  nil,
		},
		{
			name: "ChooseHostInterface returns error",
			mockChooseHostInterface: func() (net.IP, error) {
				return nil, assert.AnError
			},
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{},
			},
			expectedResult: "",
			expectedError:  ErrNoAutoInterface,
		},
		{
			name: "ChooseHostInterface returns nil IP",
			mockChooseHostInterface: func() (net.IP, error) {
				return nil, nil
			},
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{},
			},
			expectedResult: "",
			expectedError:  ErrNoAutoInterface,
		},
		{
			name: "ChooseHostInterface returns IPv6",
			mockChooseHostInterface: func() (net.IP, error) {
				return net.ParseIP("2001:db8::1"), nil
			},
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{},
			},
			expectedResult: "",
			expectedError:  ErrBestInterfaceWas6,
		},
		{
			name: "cannot find interface name for IP",
			mockChooseHostInterface: func() (net.IP, error) {
				return net.ParseIP("192.168.1.100"), nil
			},
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name: "eth0",
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.200"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedResult: "",
			expectedError:  ErrCannotDetermineInterfaceName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ChooseHostInterface = tt.mockChooseHostInterface
			NetworkInterfaceProvider = tt.mockNetworkInterfaceProvider

			result, err := DetermineBestNetworkInterface()

			assert.Equal(t, tt.expectedResult, result)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestFindInterfaceNameByIP(t *testing.T) {
	// Save original variable
	originalNetworkInterfaceProvider := NetworkInterfaceProvider
	defer func() {
		NetworkInterfaceProvider = originalNetworkInterfaceProvider
	}()

	tests := []struct {
		name                         string
		ip                           net.IP
		mockNetworkInterfaceProvider netutils.NetworkInterfaceProvider
		expectedName                 string
		expectedError                bool
	}{
		{
			name: "IP found on interface",
			ip:   net.ParseIP("192.168.1.100"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)}},
					},
					&mockNetworkInterface{
						name:  "lo",
						addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}},
					},
				},
			},
			expectedName:  "eth0",
			expectedError: false,
		},
		{
			name: "IP found on loopback interface",
			ip:   net.ParseIP("127.0.0.1"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name:  "lo",
						addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}},
					},
				},
			},
			expectedName:  "lo",
			expectedError: false,
		},
		{
			name: "IP not found on any interface",
			ip:   net.ParseIP("10.0.0.1"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)}},
					},
				},
			},
			expectedName:  "",
			expectedError: true,
		},
		{
			name: "NetworkInterfaceProvider returns error",
			ip:   net.ParseIP("192.168.1.100"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				err: assert.AnError,
			},
			expectedName:  "",
			expectedError: true,
		},
		{
			name: "nil IP",
			ip:   nil,
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						addrs: []net.Addr{&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)}},
					},
				},
			},
			expectedName:  "",
			expectedError: true,
		},
		{
			name: "IP found with IPAddr type",
			ip:   net.ParseIP("192.168.1.100"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						addrs: []net.Addr{&net.IPAddr{IP: net.ParseIP("192.168.1.100")}},
					},
				},
			},
			expectedName:  "eth0",
			expectedError: false,
		},
		{
			name: "interface.Addrs returns error",
			ip:   net.ParseIP("192.168.1.100"),
			mockNetworkInterfaceProvider: &mockNetworkInterfaceProvider{
				interfaces: []netutils.NetworkInterface{
					&mockNetworkInterface{
						name: "eth0",
						err:  assert.AnError,
					},
				},
			},
			expectedName:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NetworkInterfaceProvider = tt.mockNetworkInterfaceProvider

			result, err := findInterfaceNameByIP(tt.ip)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedName, result)
			}
		})
	}
}
