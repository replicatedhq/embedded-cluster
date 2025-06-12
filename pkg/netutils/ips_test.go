package netutils

import (
	"net"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg-new/cloudutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock NetworkInterface implementation for testing
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

// Mock NetworkInterfaceProvider implementation for testing
type mockNetworkInterfaceProvider struct {
	interfaces []NetworkInterface
	err        error
}

func (m *mockNetworkInterfaceProvider) Interfaces() ([]NetworkInterface, error) {
	return m.interfaces, m.err
}

func TestFirstValidAddress(t *testing.T) {
	// Save original provider
	originalProvider := networkInterfaceProvider
	defer func() {
		networkInterfaceProvider = originalProvider
	}()

	tests := []struct {
		name                  string
		networkInterface      string
		mockProvider          NetworkInterfaceProvider
		expectedAddress       string
		expectedError         bool
		expectedErrorContains string
	}{
		{
			name:             "empty interface name - returns first valid",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedAddress: "192.168.1.100",
			expectedError:   false,
		},
		{
			name:             "specific interface name found",
			networkInterface: "eth0",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.0.0.50"), Mask: net.CIDRMask(16, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "eth1",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedAddress: "10.0.0.50",
			expectedError:   false,
		},
		{
			name:             "interface not found",
			networkInterface: "foo",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedAddress:       "",
			expectedError:         true,
			expectedErrorContains: "interface foo not found or is not valid",
		},
		{
			name:             "no valid interfaces",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{},
			},
			expectedAddress:       "",
			expectedError:         true,
			expectedErrorContains: "no valid network interfaces found",
		},
		{
			name:             "interface with IPv6 only",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)},
						},
					},
				},
			},
			expectedAddress:       "",
			expectedError:         true,
			expectedErrorContains: "no valid network interfaces found",
		},
		{
			name:             "provider returns error",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				err: assert.AnError,
			},
			expectedAddress:       "",
			expectedError:         true,
			expectedErrorContains: "list network interfaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkInterfaceProvider = tt.mockProvider

			result, err := FirstValidAddress(tt.networkInterface)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Empty(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedAddress, result)
			}
		})
	}
}

func TestFirstValidIPNet(t *testing.T) {
	// Save original provider
	originalProvider := networkInterfaceProvider
	defer func() {
		networkInterfaceProvider = originalProvider
	}()

	tests := []struct {
		name                  string
		networkInterface      string
		mockProvider          NetworkInterfaceProvider
		expectedIP            string
		expectedMask          string
		expectedError         bool
		expectedErrorContains string
	}{
		{
			name:             "empty interface name - returns first valid",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedIP:    "192.168.1.100",
			expectedMask:  "ffffff00",
			expectedError: false,
		},
		{
			name:             "specific interface found",
			networkInterface: "eth1",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "eth1",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.0.0.50"), Mask: net.CIDRMask(16, 32)},
						},
					},
				},
			},
			expectedIP:    "10.0.0.50",
			expectedMask:  "ffff0000",
			expectedError: false,
		},
		{
			name:             "interface not found",
			networkInterface: "nonexistent",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedError:         true,
			expectedErrorContains: "interface nonexistent not found or is not valid",
		},
		{
			name:             "no valid interfaces",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{},
			},
			expectedError:         true,
			expectedErrorContains: "no valid network interfaces found",
		},
		{
			name:             "interface with only loopback addresses",
			networkInterface: "",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "lo",
						flags: net.FlagUp | net.FlagLoopback,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
						},
					},
				},
			},
			expectedError:         true,
			expectedErrorContains: "no valid network interfaces found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkInterfaceProvider = tt.mockProvider

			result, err := FirstValidIPNet(tt.networkInterface)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expectedIP, result.IP.String())
				assert.Equal(t, tt.expectedMask, result.Mask.String())
			}
		})
	}
}

func TestListValidNetworkInterfaces(t *testing.T) {
	// Save original provider
	originalProvider := networkInterfaceProvider
	defer func() {
		networkInterfaceProvider = originalProvider
	}()

	tests := []struct {
		name                   string
		mockProvider           NetworkInterfaceProvider
		expectedInterfaceNames []string
		expectedError          bool
		expectedErrorContains  string
	}{
		{
			name: "multiple valid interfaces",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "eth1",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.0.0.50"), Mask: net.CIDRMask(16, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "lo",
						flags: net.FlagUp | net.FlagLoopback,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
						},
					},
				},
			},
			expectedInterfaceNames: []string{"eth0", "eth1"},
			expectedError:          false,
		},
		{
			name: "filter out down interfaces",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "eth1",
						flags: 0, // Interface is down
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.0.0.50"), Mask: net.CIDRMask(16, 32)},
						},
					},
				},
			},
			expectedInterfaceNames: []string{"eth0"},
			expectedError:          false,
		},
		{
			name: "filter out loopback interfaces",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "lo",
						flags: net.FlagUp | net.FlagLoopback,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
						},
					},
				},
			},
			expectedInterfaceNames: []string{"eth0"},
			expectedError:          false,
		},
		{
			name: "filter out invalid interface names",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "veth123",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("172.17.0.1"), Mask: net.CIDRMask(16, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "cali456",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.244.0.1"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			expectedInterfaceNames: []string{"eth0"},
			expectedError:          false,
		},
		{
			name: "no valid interfaces",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "lo",
						flags: net.FlagUp | net.FlagLoopback,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
						},
					},
				},
			},
			expectedInterfaceNames: []string{},
			expectedError:          false,
		},
		{
			name: "provider returns error",
			mockProvider: &mockNetworkInterfaceProvider{
				err: assert.AnError,
			},
			expectedInterfaceNames: nil,
			expectedError:          true,
			expectedErrorContains:  "list network interfaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkInterfaceProvider = tt.mockProvider

			result, err := ListValidNetworkInterfaces()

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, len(tt.expectedInterfaceNames))

				actualNames := make([]string, len(result))
				for i, iface := range result {
					actualNames[i] = iface.Name()
				}
				assert.Equal(t, tt.expectedInterfaceNames, actualNames)
			}
		})
	}
}

func TestListAllValidIPAddresses(t *testing.T) {
	// Save original provider
	originalProvider := networkInterfaceProvider
	defer func() {
		networkInterfaceProvider = originalProvider
		cloudutils.Set(cloudutils.New())
	}()

	tests := []struct {
		name                  string
		mockProvider          NetworkInterfaceProvider
		setupMockCloudUtils   func(m *cloudutils.MockCloudUtils)
		expectedIPs           []string
		expectedError         bool
		expectedErrorContains string
	}{
		{
			name: "multiple interfaces with IPv4 addresses",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
							&net.IPNet{IP: net.ParseIP("192.168.1.101"), Mask: net.CIDRMask(24, 32)},
						},
					},
					&mockNetworkInterface{
						name:  "eth1",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("10.0.0.50"), Mask: net.CIDRMask(16, 32)},
						},
					},
				},
			},
			setupMockCloudUtils: func(m *cloudutils.MockCloudUtils) {
				m.On("TryDiscoverPublicIP").Once().Return("")
			},
			expectedIPs:   []string{"192.168.1.100", "192.168.1.101", "10.0.0.50"},
			expectedError: false,
		},
		{
			name: "filter out IPv6 and loopback addresses",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
							&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)}, // IPv6
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},     // Loopback
						},
					},
				},
			},
			setupMockCloudUtils: func(m *cloudutils.MockCloudUtils) {
				m.On("TryDiscoverPublicIP").Once().Return("")
			},
			expectedIPs:   []string{"192.168.1.100"},
			expectedError: false,
		},
		{
			name: "interface.Addrs returns error",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						err:   assert.AnError,
					},
				},
			},
			setupMockCloudUtils: func(m *cloudutils.MockCloudUtils) {
				m.On("TryDiscoverPublicIP").Once().Return("")
			},
			expectedIPs:   []string{},
			expectedError: false,
		},
		{
			name: "no valid interfaces",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "lo",
						flags: net.FlagUp | net.FlagLoopback,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)},
						},
					},
				},
			},
			setupMockCloudUtils: func(m *cloudutils.MockCloudUtils) {
				m.On("TryDiscoverPublicIP").Once().Return("")
			},
			expectedIPs:   []string{},
			expectedError: false,
		},
		{
			name: "provider returns error",
			mockProvider: &mockNetworkInterfaceProvider{
				err: assert.AnError,
			},
			setupMockCloudUtils:   func(m *cloudutils.MockCloudUtils) {},
			expectedIPs:           nil,
			expectedError:         true,
			expectedErrorContains: "list valid network interfaces",
		},
		{
			name: "includes discovered public IP address from cloud provider",
			mockProvider: &mockNetworkInterfaceProvider{
				interfaces: []NetworkInterface{
					&mockNetworkInterface{
						name:  "eth0",
						flags: net.FlagUp,
						addrs: []net.Addr{
							&net.IPNet{IP: net.ParseIP("192.168.1.100"), Mask: net.CIDRMask(24, 32)},
						},
					},
				},
			},
			setupMockCloudUtils: func(m *cloudutils.MockCloudUtils) {
				m.On("TryDiscoverPublicIP").Once().Return("203.0.113.45")
			},
			expectedIPs:   []string{"192.168.1.100", "203.0.113.45"},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			networkInterfaceProvider = tt.mockProvider

			mockCloudUtils := &cloudutils.MockCloudUtils{}
			cloudutils.Set(mockCloudUtils)
			tt.setupMockCloudUtils(mockCloudUtils)

			result, err := ListAllValidIPAddresses()

			mockCloudUtils.AssertExpectations(t)

			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)

				// Convert result IPs to strings for comparison
				actualIPs := make([]string, 0, len(result))
				for _, ip := range result {
					actualIPs = append(actualIPs, ip.String())
				}

				assert.ElementsMatch(t, tt.expectedIPs, actualIPs)
			}
		})
	}
}
