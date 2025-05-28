package integration

import "github.com/replicatedhq/embedded-cluster/api/pkg/utils"

var _ utils.NetUtils = &mockNetUtils{}

type mockNetUtils struct {
	err    error
	ifaces []string
}

func (m *mockNetUtils) ListValidNetworkInterfaces() ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ifaces, nil
}

func (m *mockNetUtils) DetermineBestNetworkInterface() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.ifaces[0], nil
}
