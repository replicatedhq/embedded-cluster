package integration

import "github.com/replicatedhq/embedded-cluster/api/pkg/utils"

var _ utils.NetUtils = &mockNetUtils{}

type mockNetUtils struct {
	ifaces []string
}

func (m *mockNetUtils) ListValidNetworkInterfaces() ([]string, error) {
	return m.ifaces, nil
}

func (m *mockNetUtils) DetermineBestNetworkInterface() (string, error) {
	return m.ifaces[0], nil
}
