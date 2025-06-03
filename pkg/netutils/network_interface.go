package netutils

import "net"

var _ NetworkInterface = (*RealNetworkInterface)(nil)

// NetworkInterface abstracts net.Interface for testing and dependency injection
type NetworkInterface interface {
	// Name returns the name of the interface
	Name() string
	// Flags returns the flags for the interface
	Flags() net.Flags
	// Addrs returns the addresses assigned to the interface
	Addrs() ([]net.Addr, error)
}

// NetworkInterfaceProvider abstracts the network interface discovery
type NetworkInterfaceProvider interface {
	// Interfaces returns all network interfaces
	Interfaces() ([]NetworkInterface, error)
}

// RealNetworkInterface wraps net.Interface to implement NetworkInterface
type RealNetworkInterface struct {
	iface net.Interface
}

// NewRealNetworkInterface creates a new RealNetworkInterface
func NewRealNetworkInterface(iface net.Interface) NetworkInterface {
	return &RealNetworkInterface{iface: iface}
}

// Name returns the name of the interface
func (r *RealNetworkInterface) Name() string {
	return r.iface.Name
}

// Flags returns the flags for the interface
func (r *RealNetworkInterface) Flags() net.Flags {
	return r.iface.Flags
}

// Addrs returns the addresses assigned to the interface
func (r *RealNetworkInterface) Addrs() ([]net.Addr, error) {
	return r.iface.Addrs()
}

// RealNetworkInterfaceProvider implements NetworkInterfaceProvider using net.Interfaces
type RealNetworkInterfaceProvider struct{}

// NewRealNetworkInterfaceProvider creates a new RealNetworkInterfaceProvider
func NewRealNetworkInterfaceProvider() NetworkInterfaceProvider {
	return &RealNetworkInterfaceProvider{}
}

// Interfaces returns all network interfaces wrapped in NetworkInterface
func (r *RealNetworkInterfaceProvider) Interfaces() ([]NetworkInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	result := make([]NetworkInterface, len(interfaces))
	for i, iface := range interfaces {
		result[i] = NewRealNetworkInterface(iface)
	}
	return result, nil
}

// Default provider instance - can be overridden for testing
var DefaultNetworkInterfaceProvider NetworkInterfaceProvider = NewRealNetworkInterfaceProvider()
