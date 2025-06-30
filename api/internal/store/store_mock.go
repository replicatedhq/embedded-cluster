package store

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/store/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/store/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/store/preflight"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	PreflightMockStore    preflight.MockStore
	InfraMockStore        infra.MockStore
	InstallationMockStore installation.MockStore
}

// PreflightStore returns the mock preflight store
func (m *MockStore) PreflightStore() preflight.Store {
	return &m.PreflightMockStore
}

// InstallationStore returns the mock installation store
func (m *MockStore) InstallationStore() installation.Store {
	return &m.InstallationMockStore
}

// InfraStore returns the mock infra store
func (m *MockStore) InfraStore() infra.Store {
	return &m.InfraMockStore
}
