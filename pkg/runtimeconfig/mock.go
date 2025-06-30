package runtimeconfig

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/mock"
)

var _ RuntimeConfig = (*MockRuntimeConfig)(nil)

// MockRuntimeConfig is a mock implementation of the RuntimeConfig interface
type MockRuntimeConfig struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockRuntimeConfig) Get() *ecv1beta1.RuntimeConfigSpec {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ecv1beta1.RuntimeConfigSpec)
}

// Set mocks the Set method
func (m *MockRuntimeConfig) Set(spec *ecv1beta1.RuntimeConfigSpec) {
	m.Called(spec)
}

// Cleanup mocks the Cleanup method
func (m *MockRuntimeConfig) Cleanup() {
	m.Called()
}

// EmbeddedClusterHomeDirectory mocks the EmbeddedClusterHomeDirectory method
func (m *MockRuntimeConfig) EmbeddedClusterHomeDirectory() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterTmpSubDir mocks the EmbeddedClusterTmpSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterTmpSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterBinsSubDir mocks the EmbeddedClusterBinsSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterBinsSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterChartsSubDir mocks the EmbeddedClusterChartsSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterChartsSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterChartsSubDirNoCreate mocks the EmbeddedClusterChartsSubDirNoCreate method
func (m *MockRuntimeConfig) EmbeddedClusterChartsSubDirNoCreate() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterImagesSubDir mocks the EmbeddedClusterImagesSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterImagesSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterK0sSubDir mocks the EmbeddedClusterK0sSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterK0sSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterSeaweedFSSubDir mocks the EmbeddedClusterSeaweedFSSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterSeaweedFSSubDir() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterOpenEBSLocalSubDir mocks the EmbeddedClusterOpenEBSLocalSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterOpenEBSLocalSubDir() string {
	args := m.Called()
	return args.String(0)
}

// PathToEmbeddedClusterBinary mocks the PathToEmbeddedClusterBinary method
func (m *MockRuntimeConfig) PathToEmbeddedClusterBinary(name string) string {
	args := m.Called(name)
	return args.String(0)
}

// PathToKubeConfig mocks the PathToKubeConfig method
func (m *MockRuntimeConfig) PathToKubeConfig() string {
	args := m.Called()
	return args.String(0)
}

// PathToKubeletConfig mocks the PathToKubeletConfig method
func (m *MockRuntimeConfig) PathToKubeletConfig() string {
	args := m.Called()
	return args.String(0)
}

// EmbeddedClusterSupportSubDir mocks the EmbeddedClusterSupportSubDir method
func (m *MockRuntimeConfig) EmbeddedClusterSupportSubDir() string {
	args := m.Called()
	return args.String(0)
}

// PathToEmbeddedClusterSupportFile mocks the PathToEmbeddedClusterSupportFile method
func (m *MockRuntimeConfig) PathToEmbeddedClusterSupportFile(name string) string {
	args := m.Called(name)
	return args.String(0)
}

// SetEnv mocks the SetEnv method
func (m *MockRuntimeConfig) SetEnv() error {
	args := m.Called()
	return args.Error(0)
}

// WriteToDisk mocks the WriteToDisk method
func (m *MockRuntimeConfig) WriteToDisk() error {
	args := m.Called()
	return args.Error(0)
}

// LocalArtifactMirrorPort mocks the LocalArtifactMirrorPort method
func (m *MockRuntimeConfig) LocalArtifactMirrorPort() int {
	args := m.Called()
	return args.Int(0)
}

// AdminConsolePort mocks the AdminConsolePort method
func (m *MockRuntimeConfig) AdminConsolePort() int {
	args := m.Called()
	return args.Int(0)
}

// ManagerPort mocks the ManagerPort method
func (m *MockRuntimeConfig) ManagerPort() int {
	args := m.Called()
	return args.Int(0)
}

// ProxySpec mocks the ProxySpec method
func (m *MockRuntimeConfig) ProxySpec() *ecv1beta1.ProxySpec {
	args := m.Called()
	return args.Get(0).(*ecv1beta1.ProxySpec)
}

// GlobalCIDR returns the configured global CIDR or the default if not configured.
func (m *MockRuntimeConfig) GlobalCIDR() string {
	args := m.Called()
	return args.String(0)
}

// PodCIDR returns the configured pod CIDR or the default if not configured.
func (m *MockRuntimeConfig) PodCIDR() string {
	args := m.Called()
	return args.String(0)
}

// ServiceCIDR returns the configured service CIDR or the default if not configured.
func (m *MockRuntimeConfig) ServiceCIDR() string {
	args := m.Called()
	return args.String(0)
}

// NetworkInterface returns the configured network interface or the default if not configured.
func (m *MockRuntimeConfig) NetworkInterface() string {
	args := m.Called()
	return args.String(0)
}

// NodePortRange returns the configured node port range or the default if not configured.
func (m *MockRuntimeConfig) NodePortRange() string {
	args := m.Called()
	return args.String(0)
}

// HostCABundlePath mocks the HostCABundlePath method
func (m *MockRuntimeConfig) HostCABundlePath() string {
	args := m.Called()
	return args.String(0)
}

// SetDataDir mocks the SetDataDir method
func (m *MockRuntimeConfig) SetDataDir(dataDir string) {
	m.Called(dataDir)
}

// SetLocalArtifactMirrorPort mocks the SetLocalArtifactMirrorPort method
func (m *MockRuntimeConfig) SetLocalArtifactMirrorPort(port int) {
	m.Called(port)
}

// SetAdminConsolePort mocks the SetAdminConsolePort method
func (m *MockRuntimeConfig) SetAdminConsolePort(port int) {
	m.Called(port)
}

// SetManagerPort mocks the SetManagerPort method
func (m *MockRuntimeConfig) SetManagerPort(port int) {
	m.Called(port)
}

// SetProxySpec mocks the SetProxySpec method
func (m *MockRuntimeConfig) SetProxySpec(proxySpec *ecv1beta1.ProxySpec) {
	m.Called(proxySpec)
}

// SetNetworkSpec mocks the SetNetworkSpec method
func (m *MockRuntimeConfig) SetNetworkSpec(networkSpec ecv1beta1.NetworkSpec) {
	m.Called(networkSpec)
}

// SetHostCABundlePath mocks the SetHostCABundlePath method
func (m *MockRuntimeConfig) SetHostCABundlePath(hostCABundlePath string) {
	m.Called(hostCABundlePath)
}
