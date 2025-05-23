package utils

import (
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

type NetUtils interface {
	ListValidNetworkInterfaces() ([]string, error)
	DetermineBestNetworkInterface() (string, error)
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
		names = append(names, i.Name)
	}
	return names, nil
}

func (n *netUtils) DetermineBestNetworkInterface() (string, error) {
	return newconfig.DetermineBestNetworkInterface()
}
