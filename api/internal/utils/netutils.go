package utils

import (
	"net"

	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

type NetUtils interface {
	ListValidNetworkInterfaces() ([]string, error)
	DetermineBestNetworkInterface() (string, error)
	FirstValidIPNet(networkInterface string) (*net.IPNet, error)
	FirstValidAddress(networkInterface string) (string, error)
}

type netUtils struct {
}

var _ NetUtils = &netUtils{}

func NewNetUtils() NetUtils {
	return &netUtils{}
}

func (n *netUtils) ListValidNetworkInterfaces() ([]string, error) {
	ifs, err := netutils.ListValidNetworkInterfaces()
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, i := range ifs {
		names = append(names, i.Name())
	}
	return names, nil
}

func (n *netUtils) DetermineBestNetworkInterface() (string, error) {
	return newconfig.DetermineBestNetworkInterface()
}

func (n *netUtils) FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	return netutils.FirstValidIPNet(networkInterface)
}

func (n *netUtils) FirstValidAddress(networkInterface string) (string, error) {
	return netutils.FirstValidAddress(networkInterface)
}
