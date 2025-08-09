package config

import (
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	apimachinerynet "k8s.io/apimachinery/pkg/util/net"
)

// Dependency injection variables for testing
var (
	ChooseHostInterface      = apimachinerynet.ChooseHostInterface
	NetworkInterfaceProvider = netutils.DefaultNetworkInterfaceProvider
)

var (
	ErrNoAutoInterface              = fmt.Errorf("no auto interface found")
	ErrBestInterfaceWas6            = fmt.Errorf("best interface was IPv6")
	ErrCannotDetermineInterfaceName = fmt.Errorf("cannot determine interface name")
)

// DetermineBestNetworkInterface attempts to determine the best network interface to use for the cluster.
func DetermineBestNetworkInterface() (string, error) {
	iface, err := ChooseHostInterface()

	if err != nil || iface == nil {
		return "", ErrNoAutoInterface
	}

	if iface.To4() == nil {
		return "", ErrBestInterfaceWas6
	}

	ifaceName, err := findInterfaceNameByIP(iface)
	if err != nil {
		return "", ErrCannotDetermineInterfaceName
	}

	return ifaceName, nil
}

func findInterfaceNameByIP(ip net.IP) (string, error) {
	interfaces, err := NetworkInterfaceProvider.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list interfaces: %v", err)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return "", fmt.Errorf("failed to get addresses for interface %s: %v", iface.Name(), err)
		}

		for _, addr := range addrs {
			var ifaceIP net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ifaceIP = v.IP
			case *net.IPAddr:
				ifaceIP = v.IP
			}

			if ifaceIP != nil && ifaceIP.Equal(ip) {
				return iface.Name(), nil
			}
		}
	}

	return "", fmt.Errorf("no interface found for IP %s", ip)
}
