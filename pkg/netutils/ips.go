package netutils

import (
	"fmt"
	"net"
	"strings"
)

// FirstValidAddress returns the first found non-local IPv4 address
// that's not part of pod network for the given interface.
// If an interface is not provided, the first found interface with a valid address is used.
func FirstValidAddress(iface string) (string, error) {
	ipnet, err := FirstValidIPNet(iface)
	if err != nil {
		return "", fmt.Errorf("get ipnet for interface %s: %w", iface, err)
	}
	if ipnet.IP.To4() == nil {
		return "", fmt.Errorf("interface %s has no ipv4 addresses", iface)
	}
	return ipnet.IP.String(), nil
}

func FirstValidIPNet(iface string) (*net.IPNet, error) {
	ifs, err := listValidInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list valid network interfaces: %w", err)
	}
	if len(ifs) == 0 {
		return nil, fmt.Errorf("no valid network interfaces found on this machine")
	}
	if iface == "" {
		return firstValidIPNet(ifs[0])
	}
	for _, i := range ifs {
		if i.Name == iface {
			return firstValidIPNet(i)
		}
	}
	return nil, fmt.Errorf("interface %s not found or is not valid", iface)
}

// listValidInterfaces returns a list of valid network interfaces for the node.
func listValidInterfaces() ([]net.Interface, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	validIfs := []net.Interface{}
	for _, i := range ifs {
		if !isValidInterface(i) {
			continue
		}
		validIfs = append(validIfs, i)
	}
	return validIfs, nil
}

// adapted from https://github.com/k0sproject/k0s/blob/v1.30.4%2Bk0s.0/internal/pkg/iface/iface.go#L61
func isValidInterface(i net.Interface) bool {
	switch {
	case i.Name == "vxlan.calico":
		return false
	case i.Name == "kube-bridge":
		return false
	case i.Name == "dummyvip0":
		return false
	case strings.HasPrefix(i.Name, "veth"):
		return false
	case strings.HasPrefix(i.Name, "cali"):
		return false
	}
	return hasValidIPNet(i)
}

func hasValidIPNet(i net.Interface) bool {
	ipnet, err := firstValidIPNet(i)
	return err == nil && ipnet != nil
}

func firstValidIPNet(i net.Interface) (*net.IPNet, error) {
	addresses, err := i.Addrs()
	if err != nil {
		return nil, fmt.Errorf("get addresses: %w", err)
	}
	for _, a := range addresses {
		// check the address type and skip if loopback
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet, nil
			}
		}
	}
	return nil, fmt.Errorf("could not find any non-local, non podnetwork ipv4 addresses")
}
