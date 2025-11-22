package migration

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error {
	args := m.Called(migrationID, transferMode, config)
	return args.Error(0)
}

func (m *MockStore) GetMigrationID() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockStore) GetStatus() (types.MigrationStatusResponse, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.MigrationStatusResponse{}, args.Error(1)
	}
	return args.Get(0).(types.MigrationStatusResponse), args.Error(1)
}

func (m *MockStore) SetState(state types.MigrationState) error {
	args := m.Called(state)
	return args.Error(0)
}

func (m *MockStore) SetPhase(phase types.MigrationPhase) error {
	args := m.Called(phase)
	return args.Error(0)
}

func (m *MockStore) SetMessage(message string) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockStore) SetProgress(progress int) error {
	args := m.Called(progress)
	return args.Error(0)
}

func (m *MockStore) SetError(err string) error {
	args := m.Called(err)
	return args.Error(0)
}

func (m *MockStore) GetTransferMode() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

func (m *MockStore) GetConfig() (types.LinuxInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}
