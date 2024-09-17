package netutils

import (
	"fmt"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
)

// GetDefaultIPAndMask returns the default interface for the node, and the subnet mask for that node, using the same logic as k0s
func GetDefaultIPAndMask() (*net.IPNet, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list network interfaces: %w", err)
	}
	for _, i := range ifs {
		if isPodnetworkInterface(i.Name) {
			continue
		}
		addresses, err := i.Addrs()
		if err != nil {
			logrus.Debugf("failed to get addresses for interface %s: %s", i.Name, err.Error())
			continue
		}
		for _, a := range addresses {
			// check the address type and skip if loopback
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("failed to find any non-local, non podnetwork ipv4 addresses on host")
}

func isPodnetworkInterface(name string) bool {
	switch {
	case name == "vxlan.calico":
		return true
	case name == "kube-bridge":
		return true
	case name == "dummyvip0":
		return true
	case strings.HasPrefix(name, "veth"):
		return true
	case strings.HasPrefix(name, "cali"):
		return true
	}
	return false
}
