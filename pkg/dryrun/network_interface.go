package dryrun

import (
	"net"

	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

var _ netutils.NetworkInterfaceProvider = (*NetworkInterfaceProvider)(nil)
var _ netutils.NetworkInterface = (*NetworkInterface)(nil)

// NetworkInterfaceProvider implementation for testing
type NetworkInterfaceProvider struct {
	Ifaces []netutils.NetworkInterface
	Err    error
}

func (p *NetworkInterfaceProvider) Interfaces() ([]netutils.NetworkInterface, error) {
	return p.Ifaces, p.Err
}

// NetworkInterface implementation for testing
type NetworkInterface struct {
	MockName  string
	MockFlags net.Flags
	MockAddrs []net.Addr
	MockErr   error
}

func (i *NetworkInterface) Name() string {
	return i.MockName
}

func (i *NetworkInterface) Flags() net.Flags {
	return i.MockFlags
}

func (i *NetworkInterface) Addrs() ([]net.Addr, error) {
	return i.MockAddrs, i.MockErr
}

// ChooseHostInterfaceImpl is a mock implementation that returns a fixed IP for testing
type ChooseInterfaceImpl struct {
	IP net.IP
}

func (c *ChooseInterfaceImpl) ChooseHostInterface() (net.IP, error) {
	return c.IP, nil
}
