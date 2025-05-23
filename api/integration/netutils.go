package integration

import "github.com/replicatedhq/embedded-cluster/api/pkg/utils"

var _ utils.NetUtils = &mockNetUtils{}

type mockNetUtils struct {
	iface string
}

func (m *mockNetUtils) DetermineBestNetworkInterface() (string, error) {
	return m.iface, nil
}
