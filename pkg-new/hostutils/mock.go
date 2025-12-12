package hostutils

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

var _ HostUtilsInterface = (*MockHostUtils)(nil)

// MockHostUtils is a mock implementation of the HostUtilsInterface
type MockHostUtils struct {
	mock.Mock
}

// ConfigureSELinuxFcontext implements HostUtilsInterface.
func (m *MockHostUtils) ConfigureSELinuxFcontext(rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(rc)
	return args.Error(0)
}

// RestoreSELinuxContext implements HostUtilsInterface.
func (m *MockHostUtils) RestoreSELinuxContext(rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(rc)
	return args.Error(0)
}

// ConfigureHost mocks the ConfigureHost method
func (m *MockHostUtils) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, opts InitForInstallOptions) error {
	args := m.Called(ctx, rc, channelRelease, opts)
	return args.Error(0)
}

// ConfigureSysctl mocks the ConfigureSysctl method
func (m *MockHostUtils) ConfigureSysctl() error {
	args := m.Called()
	return args.Error(0)
}

// ConfigureKernelModules mocks the ConfigureKernelModules method
func (m *MockHostUtils) ConfigureKernelModules() error {
	args := m.Called()
	return args.Error(0)
}

// ConfigureNetworkManager mocks the ConfigureNetworkManager method
func (m *MockHostUtils) ConfigureNetworkManager(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(ctx, rc)
	return args.Error(0)
}

// ConfigureFirewalld mocks the ConfigureFirewalld method
func (m *MockHostUtils) ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error {
	args := m.Called(ctx, podNetwork, serviceNetwork)
	return args.Error(0)
}

// ResetFirewalld mocks the ResetFirewalld method
func (m *MockHostUtils) ResetFirewalld(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MaterializeFiles mocks the MaterializeFiles method
func (m *MockHostUtils) MaterializeFiles(rc runtimeconfig.RuntimeConfig, channelRelease *release.ChannelRelease, airgapBundle string) error {
	args := m.Called(rc, channelRelease, airgapBundle)
	return args.Error(0)
}

// CreateSystemdUnitFiles mocks the CreateSystemdUnitFiles method
func (m *MockHostUtils) CreateSystemdUnitFiles(ctx context.Context, logger logrus.FieldLogger, rc runtimeconfig.RuntimeConfig, hostname string, isWorker bool) error {
	args := m.Called(ctx, logger, rc, hostname, isWorker)
	return args.Error(0)
}

// WriteLocalArtifactMirrorDropInFile mocks the WriteLocalArtifactMirrorDropInFile method
func (m *MockHostUtils) WriteLocalArtifactMirrorDropInFile(rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(rc)
	return args.Error(0)
}

// AddInsecureRegistry mocks the AddInsecureRegistry method
func (m *MockHostUtils) AddInsecureRegistry(registry string) error {
	args := m.Called(registry)
	return args.Error(0)
}
