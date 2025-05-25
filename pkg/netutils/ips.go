package netutils

import (
	"fmt"
	"net"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// adapted from https://github.com/k0sproject/k0s/blob/v1.30.4%2Bk0s.0/internal/pkg/iface/iface.go#L61
func FirstValidAddress(networkInterface string) (string, error) {
	ipnet, err := FirstValidIPNet(networkInterface)
	if err != nil {
		return "", fmt.Errorf("get ipnet for interface %s: %w", networkInterface, err)
	}
	if ipnet.IP.To4() == nil {
		return "", fmt.Errorf("interface %s has no ipv4 addresses", networkInterface)
	}
	return ipnet.IP.String(), nil
}

func FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	ifs, err := listValidInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list valid network interfaces: %w", err)
	}
	if len(ifs) == 0 {
		return nil, fmt.Errorf("no valid network interfaces found on this machine")
	}
	if networkInterface == "" {
		return firstValidIPNet(ifs[0])
	}
	for _, i := range ifs {
		if i.Name == networkInterface {
			return firstValidIPNet(i)
		}
	}
	var ifNames []string
	for _, i := range ifs {
		ifNames = append(ifNames, i.Name)
	}
	return nil, fmt.Errorf("interface %s not found or is not valid. The following interfaces were detected: %s", networkInterface, strings.Join(ifNames, ", "))
}

// ListValidNetworkInterfaces returns a list of valid network interfaces that are up and not
// loopback.
func ListValidNetworkInterfaces() ([]net.Interface, error) {
	ifs, err := listValidInterfaces()
	if err != nil {
		return nil, err
	}

	validIfs := []net.Interface{}
	for _, i := range ifs {
		if i.Flags&net.FlagUp == 0 {
			continue
		}
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		validIfs = append(validIfs, i)
	}
	return validIfs, nil
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

func ListAllValidIPAddresses() ([]net.IP, error) {
	ipAddresses := []net.IP{}

	ifs, err := ListValidNetworkInterfaces()
	if err != nil {
		return nil, fmt.Errorf("list valid network interfaces: %w", err)
	}
	for _, i := range ifs {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, fmt.Errorf("get addresses: %w", err)
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP)
				}
			}
		}
	}

	publicIP := runtimeconfig.TryDiscoverPublicIP()
	if publicIP != "" {
		ipAddresses = append(ipAddresses, net.ParseIP(publicIP))
	}

	return ipAddresses, nil
}
