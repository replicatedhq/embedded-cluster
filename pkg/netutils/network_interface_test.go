package netutils

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRealNetworkInterface(t *testing.T) {
	tests := []struct {
		name          string
		iface         net.Interface
		expectedName  string
		expectedFlags net.Flags
	}{
		{
			name: "basic interface",
			iface: net.Interface{
				Name:  "eth0",
				Flags: net.FlagUp | net.FlagBroadcast,
			},
			expectedName:  "eth0",
			expectedFlags: net.FlagUp | net.FlagBroadcast,
		},
		{
			name: "loopback interface",
			iface: net.Interface{
				Name:  "lo",
				Flags: net.FlagUp | net.FlagLoopback,
			},
			expectedName:  "lo",
			expectedFlags: net.FlagUp | net.FlagLoopback,
		},
		{
			name: "down interface",
			iface: net.Interface{
				Name:  "eth1",
				Flags: 0,
			},
			expectedName:  "eth1",
			expectedFlags: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewRealNetworkInterface(tt.iface)

			require.NotNil(t, result)
			assert.IsType(t, &RealNetworkInterface{}, result)
			assert.Equal(t, tt.expectedName, result.Name())
			assert.Equal(t, tt.expectedFlags, result.Flags())
		})
	}
}

// Validate that the underlying default network interface provider returns the same data as the `net` package
func TestRealNetworkInterface_DefaultNetworkInterfaceProvider(t *testing.T) {

	netInterfaces, err := net.Interfaces()
	require.NoError(t, err, "failed to get system interfaces")
	providerInterfaces, err := DefaultNetworkInterfaceProvider.Interfaces()
	require.NoError(t, err, "failed to get system interfaces through the provider interface")

	require.Equal(t, len(netInterfaces), len(providerInterfaces), "interface number should match")

	// Get the loopback interface
	var loopbackInterface *net.Interface
	for _, netIface := range netInterfaces {
		if netIface.Flags&net.FlagLoopback != 0 {
			loopbackInterface = &netIface
			break
		}
	}

	// If there's none skip the remainder of the test
	if loopbackInterface == nil {
		t.Skip("no loopback interface found on system")
	}

	// Get the loopback interface through the provider interface
	var providerLoopbackInterface NetworkInterface
	for _, pIface := range providerInterfaces {
		if pIface.Flags()&net.FlagLoopback != 0 {
			providerLoopbackInterface = pIface
			break
		}
	}

	// Both should match
	realIface := NewRealNetworkInterface(*loopbackInterface)
	assert.Equal(t, realIface, providerLoopbackInterface)

	// Both Addrs methods should return the same data
	providerAddrs, err := realIface.Addrs()
	require.NoError(t, err, "failed to get addrs through the network provider interface")
	addrs, err := loopbackInterface.Addrs()
	require.NoError(t, err, "failed to get addrs")
	assert.ElementsMatch(t, addrs, providerAddrs)
	// Both Flags methods should return the same data
	assert.Equal(t, loopbackInterface.Flags, providerLoopbackInterface.Flags(), "flags should match")
	// Both Name methods should return the same data
	assert.Equal(t, loopbackInterface.Name, providerLoopbackInterface.Name(), "name should match")
}
