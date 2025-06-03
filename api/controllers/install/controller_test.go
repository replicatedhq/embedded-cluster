package install

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

// MockInstallationManager is a mock implementation of installation.InstallationManager
type MockInstallationManager struct {
	mock.Mock
}

func (m *MockInstallationManager) ReadConfig() (*types.InstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.InstallationConfig), args.Error(1)
}

func (m *MockInstallationManager) WriteConfig(config types.InstallationConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockInstallationManager) ReadStatus() (*types.InstallationStatus, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.InstallationStatus), args.Error(1)
}

func (m *MockInstallationManager) WriteStatus(status types.InstallationStatus) error {
	args := m.Called(status)
	return args.Error(0)
}

func (m *MockInstallationManager) ValidateConfig(config *types.InstallationConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func (m *MockInstallationManager) ValidateStatus(status *types.InstallationStatus) error {
	args := m.Called(status)
	return args.Error(0)
}

func (m *MockInstallationManager) SetDefaults(config *types.InstallationConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

func TestGet(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockInstallationManager)
		expectedErr   bool
		expectedValue *types.Install
	}{
		{
			name: "successful get",
			setupMock: func(m *MockInstallationManager) {
				config := &types.InstallationConfig{
					AdminConsolePort: 9000,
					GlobalCIDR:       "10.0.0.1/16",
				}
				status := &types.InstallationStatus{
					State: "Running",
				}

				m.On("ReadConfig").Return(config, nil)
				m.On("SetDefaults", config).Return(nil)
				m.On("ValidateConfig", config).Return(nil)
				m.On("ReadStatus").Return(status, nil)
			},
			expectedErr: false,
			expectedValue: &types.Install{
				Config: types.InstallationConfig{
					AdminConsolePort: 9000,
					GlobalCIDR:       "10.0.0.1/16",
				},
				Status: types.InstallationStatus{
					State: "Running",
				},
			},
		},
		{
			name: "read config error",
			setupMock: func(m *MockInstallationManager) {
				m.On("ReadConfig").Return(nil, errors.New("read error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
		{
			name: "set defaults error",
			setupMock: func(m *MockInstallationManager) {
				config := &types.InstallationConfig{}
				m.On("ReadConfig").Return(config, nil)
				m.On("SetDefaults", config).Return(errors.New("defaults error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
		{
			name: "validate error",
			setupMock: func(m *MockInstallationManager) {
				config := &types.InstallationConfig{}
				m.On("ReadConfig").Return(config, nil)
				m.On("SetDefaults", config).Return(nil)
				m.On("ValidateConfig", config).Return(errors.New("validation error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
		{
			name: "read status error",
			setupMock: func(m *MockInstallationManager) {
				config := &types.InstallationConfig{}
				m.On("ReadConfig").Return(config, nil)
				m.On("SetDefaults", config).Return(nil)
				m.On("ValidateConfig", config).Return(nil)
				m.On("ReadStatus").Return(nil, errors.New("status error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &MockInstallationManager{}
			tt.setupMock(mockManager)

			controller := &InstallController{
				installationManager: mockManager,
			}

			result, err := controller.Get(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestSetConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *types.InstallationConfig
		setupMock   func(*MockInstallationManager, *types.InstallationConfig)
		expectedErr bool
	}{
		{
			name: "successful set config",
			config: &types.InstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           "/data/dir",
			},
			setupMock: func(m *MockInstallationManager, config *types.InstallationConfig) {
				m.On("ValidateConfig", config).Return(nil)
				m.On("WriteConfig", *config).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:   "validate error",
			config: &types.InstallationConfig{},
			setupMock: func(m *MockInstallationManager, config *types.InstallationConfig) {
				m.On("ValidateConfig", config).Return(errors.New("validation error"))
			},
			expectedErr: true,
		},
		{
			name:   "write config error",
			config: &types.InstallationConfig{},
			setupMock: func(m *MockInstallationManager, config *types.InstallationConfig) {
				m.On("ValidateConfig", config).Return(nil)
				m.On("WriteConfig", *config).Return(errors.New("write error"))
			},
			expectedErr: true,
		},
		{
			name: "with global CIDR",
			config: &types.InstallationConfig{
				GlobalCIDR: "10.0.0.0/16",
			},
			setupMock: func(m *MockInstallationManager, config *types.InstallationConfig) {
				// Create a copy with expected CIDR values after computation
				configWithCIDRs := *config
				configWithCIDRs.PodCIDR = "10.0.0.0/17"
				configWithCIDRs.ServiceCIDR = "10.0.128.0/17"

				m.On("ValidateConfig", config).Return(nil)
				m.On("WriteConfig", configWithCIDRs).Return(nil)
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &MockInstallationManager{}

			// Create a copy of the config to avoid modifying the original
			configCopy := *tt.config

			tt.setupMock(mockManager, &configCopy)

			controller := &InstallController{
				installationManager: mockManager,
			}

			err := controller.SetConfig(context.Background(), tt.config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestSetStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      *types.InstallationStatus
		setupMock   func(*MockInstallationManager, *types.InstallationStatus)
		expectedErr bool
	}{
		{
			name: "successful set status",
			status: &types.InstallationStatus{
				State: types.InstallationStateFailed,
			},
			setupMock: func(m *MockInstallationManager, status *types.InstallationStatus) {
				m.On("ValidateStatus", status).Return(nil)
				m.On("WriteStatus", *status).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:   "validate error",
			status: &types.InstallationStatus{},
			setupMock: func(m *MockInstallationManager, status *types.InstallationStatus) {
				m.On("ValidateStatus", status).Return(errors.New("validation error"))
			},
			expectedErr: true,
		},
		{
			name:   "write status error",
			status: &types.InstallationStatus{},
			setupMock: func(m *MockInstallationManager, status *types.InstallationStatus) {
				m.On("ValidateStatus", status).Return(nil)
				m.On("WriteStatus", *status).Return(errors.New("write error"))
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &MockInstallationManager{}
			tt.setupMock(mockManager, tt.status)

			controller := &InstallController{
				installationManager: mockManager,
			}

			err := controller.SetStatus(context.Background(), tt.status)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestReadStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockInstallationManager)
		expectedErr   bool
		expectedValue *types.InstallationStatus
	}{
		{
			name: "successful read status",
			setupMock: func(m *MockInstallationManager) {
				status := &types.InstallationStatus{
					State: types.InstallationStateFailed,
				}
				m.On("ReadStatus").Return(status, nil)
			},
			expectedErr: false,
			expectedValue: &types.InstallationStatus{
				State: types.InstallationStateFailed,
			},
		},
		{
			name: "read error",
			setupMock: func(m *MockInstallationManager) {
				m.On("ReadStatus").Return(nil, errors.New("read error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &MockInstallationManager{}
			tt.setupMock(mockManager)

			controller := &InstallController{
				installationManager: mockManager,
			}

			result, err := controller.ReadStatus(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

// TestControllerWithRealManager tests the controller with the real installation manager
func TestControllerWithRealManager(t *testing.T) {
	// Create controller with real manager
	controller, err := NewInstallController()
	assert.NoError(t, err)
	assert.NotNil(t, controller)

	// Test the full cycle of operations

	// 1. Set an invalid config
	testConfig := &types.InstallationConfig{
		AdminConsolePort: 8800,
		GlobalCIDR:       "10.0.0.0/24",
		DataDirectory:    "/data/dir",
	}

	err = controller.SetConfig(context.Background(), testConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validate")

	// 2. Verify we can read the config with defaults
	install, err := controller.Get(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, install)
	assert.Equal(t, 50000, install.Config.LocalArtifactMirrorPort, "Default LocalArtifactMirrorPort should be set")
	assert.Equal(t, "/var/lib/embedded-cluster", install.Config.DataDirectory, "Default DataDirectory should be set")

	// 3. Set a valid config
	install.Config.LocalArtifactMirrorPort = 9000
	install.Config.DataDirectory = "/data/dir"
	err = controller.SetConfig(context.Background(), &install.Config)
	assert.NoError(t, err)

	// 4. Verify we can read the config again and it has the new values
	install, err = controller.Get(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, install)
	assert.Equal(t, 9000, install.Config.LocalArtifactMirrorPort, "LocalArtifactMirrorPort should be set to 9000")
	assert.Equal(t, "/data/dir", install.Config.DataDirectory, "DataDirectory should be set to /data/dir")

	// 5. Set an invalid status
	testStatus := &types.InstallationStatus{
		State:       "Not a real state",
		Description: "Installation in progress",
		LastUpdated: time.Now(),
	}

	err = controller.SetStatus(context.Background(), testStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validate")

	// 6. Set a valid status
	testStatus = &types.InstallationStatus{
		State:       types.InstallationStateRunning,
		Description: "Installation in progress",
		LastUpdated: time.Now(),
	}

	err = controller.SetStatus(context.Background(), testStatus)
	assert.NoError(t, err)

	// 7. Verify we can read status directly
	status, err := controller.ReadStatus(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, types.InstallationStateRunning, status.State)
	assert.Equal(t, "Installation in progress", status.Description)
}

// TestIntegrationComputeCIDRs tests the CIDR computation with real networking utility
func TestIntegrationComputeCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		globalCIDR  string
		expectedPod string
		expectedSvc string
		expectedErr bool
	}{
		{
			name:        "valid cidr 10.0.0.0/16",
			globalCIDR:  "10.0.0.0/16",
			expectedPod: "10.0.0.0/17",
			expectedSvc: "10.0.128.0/17",
			expectedErr: false,
		},
		{
			name:        "valid cidr 192.168.0.0/16",
			globalCIDR:  "192.168.0.0/16",
			expectedPod: "192.168.0.0/17",
			expectedSvc: "192.168.128.0/17",
			expectedErr: false,
		},
		{
			name:        "no global cidr",
			globalCIDR:  "",
			expectedPod: "", // Should remain unchanged
			expectedSvc: "", // Should remain unchanged
			expectedErr: false,
		},
		{
			name:        "invalid cidr",
			globalCIDR:  "not-a-cidr",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &InstallController{
				installationManager: &MockInstallationManager{},
			}

			config := &types.InstallationConfig{
				GlobalCIDR: tt.globalCIDR,
			}

			err := controller.computeCIDRs(config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPod, config.PodCIDR)
				assert.Equal(t, tt.expectedSvc, config.ServiceCIDR)
			}
		})
	}
}
