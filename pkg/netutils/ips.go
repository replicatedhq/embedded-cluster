package netutils

import (
	"fmt"
	"net"
	"strings"
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
