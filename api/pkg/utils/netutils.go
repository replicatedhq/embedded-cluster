package utils

import (
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
)

type NetUtils interface {
	DetermineBestNetworkInterface() (string, error)
}

type netUtils struct {
}

var _ NetUtils = &netUtils{}

func NewNetUtils() NetUtils {
	return &netUtils{}
}

func (n *netUtils) DetermineBestNetworkInterface() (string, error) {
	return newconfig.DetermineBestNetworkInterface()
}
