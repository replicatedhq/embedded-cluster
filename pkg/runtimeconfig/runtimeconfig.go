package runtimeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

var _ RuntimeConfig = &runtimeConfig{}

type Option func(*runtimeConfig)

type EnvSetter interface {
	Setenv(key string, val string) error
}

type runtimeConfig struct {
	spec      *ecv1beta1.RuntimeConfigSpec
	envSetter EnvSetter
}

type osEnvSetter struct{}

func (o *osEnvSetter) Setenv(key string, val string) error {
	return os.Setenv(key, val)
}

func WithEnvSetter(envSetter EnvSetter) Option {
	return func(rc *runtimeConfig) {
		rc.envSetter = envSetter
	}
}

// New creates a new RuntimeConfig instance
func New(spec *ecv1beta1.RuntimeConfigSpec, opts ...Option) RuntimeConfig {
	if spec == nil {
		spec = ecv1beta1.GetDefaultRuntimeConfig()
	}
	rc := &runtimeConfig{spec: spec}
	for _, opt := range opts {
		opt(rc)
	}

	if rc.envSetter == nil {
		rc.envSetter = &osEnvSetter{}
	}

	return rc
}

// NewFromDisk creates a new RuntimeConfig instance from the runtime config file on disk at path
// /etc/embedded-cluster/ec.yaml.
func NewFromDisk() (RuntimeConfig, error) {
	location := ECConfigPath
	data, err := os.ReadFile(location)
	if err != nil {
		return nil, fmt.Errorf("unable to read runtime config: %w", err)
	}

	var spec ecv1beta1.RuntimeConfigSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unable to unmarshal runtime config: %w", err)
	}

	return New(&spec), nil
}

// Get returns the spec for the RuntimeConfig.
func (rc *runtimeConfig) Get() *ecv1beta1.RuntimeConfigSpec {
	return rc.spec
}

// Set sets the spec for the RuntimeConfig.
func (rc *runtimeConfig) Set(spec *ecv1beta1.RuntimeConfigSpec) {
	if spec == nil {
		// runtime config is nil in old installation objects so this keeps the default.
		return
	}
	rc.spec = spec
}

// Cleanup removes all files in the runtime config's tmp directory.
func (rc *runtimeConfig) Cleanup() {
	tmpDir := rc.EmbeddedClusterTmpSubDir()
	// We should not delete the tmp dir, rather we should empty its contents leaving
	// it in place. This is because commands such as `kubectl edit <resource>`
	// will create files in the tmp dir
	if err := helpers.RemoveAll(tmpDir); err != nil {
		logrus.Errorf("error removing %s dir: %s", tmpDir, err)
	}
}

// EmbeddedClusterHomeDirectory returns the parent directory. Inside this parent directory we
// store all the embedded-cluster related files.
func (rc *runtimeConfig) EmbeddedClusterHomeDirectory() string {
	if rc.spec.DataDir != "" {
		return rc.spec.DataDir
	}
	return ecv1beta1.DefaultDataDir
}

// EmbeddedClusterTmpSubDir returns the path to the tmp directory where embedded-cluster
// stores temporary files.
func (rc *runtimeConfig) EmbeddedClusterTmpSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "tmp")
	mustMkdirAll(path)
	return path
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func (rc *runtimeConfig) EmbeddedClusterBinsSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "bin")
	mustMkdirAll(path)
	return path
}

// EmbeddedClusterChartsSubDir returns the path to the directory where embedded-cluster helm charts
// are stored.
func (rc *runtimeConfig) EmbeddedClusterChartsSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "charts")
	mustMkdirAll(path)
	return path
}

// EmbeddedClusterChartsSubDirNoCreate returns the path to the directory where embedded-cluster helm charts
// are stored without creating the directory if it does not exist.
func (rc *runtimeConfig) EmbeddedClusterChartsSubDirNoCreate() string {
	return filepath.Join(rc.EmbeddedClusterHomeDirectory(), "charts")
}

// EmbeddedClusterImagesSubDir returns the path to the directory where docker images are stored.
func (rc *runtimeConfig) EmbeddedClusterImagesSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "images")
	mustMkdirAll(path)
	return path
}

// EmbeddedClusterK0sSubDir returns the path to the directory where k0s data is stored.
func (rc *runtimeConfig) EmbeddedClusterK0sSubDir() string {
	if rc.spec.K0sDataDirOverride != "" {
		return rc.spec.K0sDataDirOverride
	}
	return filepath.Join(rc.EmbeddedClusterHomeDirectory(), "k0s")
}

// EmbeddedClusterSeaweedFSSubDir returns the path to the directory where seaweedfs data is stored.
func (rc *runtimeConfig) EmbeddedClusterSeaweedFSSubDir() string {
	return filepath.Join(rc.EmbeddedClusterHomeDirectory(), "seaweedfs")
}

// EmbeddedClusterOpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func (rc *runtimeConfig) EmbeddedClusterOpenEBSLocalSubDir() string {
	if rc.spec.OpenEBSDataDirOverride != "" {
		return rc.spec.OpenEBSDataDirOverride
	}
	return filepath.Join(rc.EmbeddedClusterHomeDirectory(), "openebs-local")
}

// PathToEmbeddedClusterBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster. This function does not check
// if the file exists.
func (rc *runtimeConfig) PathToEmbeddedClusterBinary(name string) string {
	return filepath.Join(rc.EmbeddedClusterBinsSubDir(), name)
}

// PathToKubeConfig returns the path to the kubeconfig file.
func (rc *runtimeConfig) PathToKubeConfig() string {
	return filepath.Join(rc.EmbeddedClusterK0sSubDir(), "pki/admin.conf")
}

// PathToKubeletConfig returns the path to the kubelet config file.
func (rc *runtimeConfig) PathToKubeletConfig() string {
	return filepath.Join(rc.EmbeddedClusterK0sSubDir(), "kubelet.conf")
}

// EmbeddedClusterSupportSubDir returns the path to the directory where embedded-cluster
// support files are stored. Things that are useful when providing end user support in
// a running cluster should be stored into this directory.
func (rc *runtimeConfig) EmbeddedClusterSupportSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "support")
	mustMkdirAll(path)
	return path
}

// PathToEmbeddedClusterSupportFile is an utility function that returns the full path to
// a materialized support file. This function does not check if the file exists.
func (rc *runtimeConfig) PathToEmbeddedClusterSupportFile(name string) string {
	return filepath.Join(rc.EmbeddedClusterSupportSubDir(), name)
}

// SetEnv sets the environment variables for the RuntimeConfig.
func (rc *runtimeConfig) SetEnv() error {
	if err := rc.envSetter.Setenv("KUBECONFIG", rc.PathToKubeConfig()); err != nil {
		return fmt.Errorf("set KUBECONFIG: %w", err)
	}
	if err := rc.envSetter.Setenv("TMPDIR", rc.EmbeddedClusterTmpSubDir()); err != nil {
		return fmt.Errorf("set TMPDIR: %w", err)
	}
	return nil
}

// WriteToDisk writes the spec for the RuntimeConfig to the runtime config file on disk at path
// /etc/embedded-cluster/ec.yaml.
func (rc *runtimeConfig) WriteToDisk() error {
	location := ECConfigPath
	if err := mkdirAll(filepath.Dir(location)); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// check if the file already exists, if it does delete it
	if err := os.RemoveAll(location); err != nil {
		return fmt.Errorf("remove existing file: %w", err)
	}

	yml, err := yaml.Marshal(rc.spec)
	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	if err := os.WriteFile(location, yml, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// LocalArtifactMirrorPort returns the configured port for the local artifact mirror or the default
// if not configured.
func (rc *runtimeConfig) LocalArtifactMirrorPort() int {
	if rc.spec.LocalArtifactMirror.Port > 0 {
		return rc.spec.LocalArtifactMirror.Port
	}
	return ecv1beta1.DefaultLocalArtifactMirrorPort
}

// AdminConsolePort returns the configured port for the admin console or the default if not
// configured.
func (rc *runtimeConfig) AdminConsolePort() int {
	if rc.spec.AdminConsole.Port > 0 {
		return rc.spec.AdminConsole.Port
	}
	return ecv1beta1.DefaultAdminConsolePort
}

// ManagerPort returns the configured port for the manager or the default if not
// configured.
func (rc *runtimeConfig) ManagerPort() int {
	if rc.spec.Manager.Port > 0 {
		return rc.spec.Manager.Port
	}
	return ecv1beta1.DefaultManagerPort
}

// ProxySpec returns the configured proxy spec or nil if not configured.
func (rc *runtimeConfig) ProxySpec() *ecv1beta1.ProxySpec {
	return rc.spec.Proxy
}

// GlobalCIDR returns the configured global CIDR or the default if not configured.
func (rc *runtimeConfig) GlobalCIDR() string {
	if rc.spec.Network.GlobalCIDR == "" && rc.spec.Network.PodCIDR == "" && rc.spec.Network.ServiceCIDR == "" {
		return ecv1beta1.DefaultNetworkCIDR
	}
	return rc.spec.Network.GlobalCIDR
}

// PodCIDR returns the configured pod CIDR or the default if not configured.
func (rc *runtimeConfig) PodCIDR() string {
	return rc.spec.Network.PodCIDR
}

// ServiceCIDR returns the configured service CIDR or the default if not configured.
func (rc *runtimeConfig) ServiceCIDR() string {
	return rc.spec.Network.ServiceCIDR
}

// NetworkInterface returns the configured network interface or the default if not configured.
func (rc *runtimeConfig) NetworkInterface() string {
	return rc.spec.Network.NetworkInterface
}

// NodePortRange returns the configured node port range or the default if not configured.
func (rc *runtimeConfig) NodePortRange() string {
	if rc.spec.Network.NodePortRange == "" {
		return ecv1beta1.DefaultNetworkNodePortRange
	}
	return rc.spec.Network.NodePortRange
}

// HostCABundlePath returns the path to the host CA bundle.
func (rc *runtimeConfig) HostCABundlePath() string {
	return rc.spec.HostCABundlePath
}

// SetDataDir sets the data directory for the runtime configuration.
func (rc *runtimeConfig) SetDataDir(dataDir string) {
	rc.spec.DataDir = dataDir
}

// SetLocalArtifactMirrorPort sets the port for the local artifact mirror.
func (rc *runtimeConfig) SetLocalArtifactMirrorPort(port int) {
	rc.spec.LocalArtifactMirror.Port = port
}

// SetAdminConsolePort sets the port for the admin console.
func (rc *runtimeConfig) SetAdminConsolePort(port int) {
	rc.spec.AdminConsole.Port = port
}

// SetManagerPort sets the port for the manager.
func (rc *runtimeConfig) SetManagerPort(port int) {
	rc.spec.Manager.Port = port
}

// SetProxySpec sets the proxy spec for the runtime configuration.
func (rc *runtimeConfig) SetProxySpec(proxySpec *ecv1beta1.ProxySpec) {
	rc.spec.Proxy = proxySpec
}

// SetNetworkSpec sets the network spec for the runtime configuration.
func (rc *runtimeConfig) SetNetworkSpec(networkSpec ecv1beta1.NetworkSpec) {
	rc.spec.Network = networkSpec
}

// SetHostCABundlePath sets the path to the host CA bundle.
func (rc *runtimeConfig) SetHostCABundlePath(hostCABundlePath string) {
	rc.spec.HostCABundlePath = hostCABundlePath
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

func mustMkdirAll(path string) {
	if err := mkdirAll(path); err != nil {
		logrus.Fatalf("unable to create dir %q: %v", path, err)
	}
}
