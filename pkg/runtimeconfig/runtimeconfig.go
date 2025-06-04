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

type runtimeConfig struct {
	spec *ecv1beta1.RuntimeConfigSpec
}

// New creates a new RuntimeConfig instance
func New(spec *ecv1beta1.RuntimeConfigSpec) RuntimeConfig {
	if spec == nil {
		spec = ecv1beta1.GetDefaultRuntimeConfig()
	}
	return &runtimeConfig{spec: spec}
}

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

func (rc *runtimeConfig) Get() *ecv1beta1.RuntimeConfigSpec {
	return rc.spec
}

func (rc *runtimeConfig) Set(spec *ecv1beta1.RuntimeConfigSpec) {
	if spec == nil {
		// runtime config is nil in old installation objects so this keeps the default.
		return
	}
	rc.spec = spec
}

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
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster tmp dir: %s", err)
	}
	return path
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func (rc *runtimeConfig) EmbeddedClusterBinsSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "bin")
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDir returns the path to the directory where embedded-cluster helm charts
// are stored.
func (rc *runtimeConfig) EmbeddedClusterChartsSubDir() string {
	path := filepath.Join(rc.EmbeddedClusterHomeDirectory(), "charts")
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster charts dir: %s", err)
	}
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
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster images dir: %s", err)
	}
	return path
}

// EmbeddedClusterK0sSubDir returns the path to the directory where k0s data is stored.
func (rc *runtimeConfig) EmbeddedClusterK0sSubDir() string {
	if rc.spec.K0sDataDirOverride != "" {
		return rc.spec.K0sDataDirOverride
	}
	return filepath.Join(rc.EmbeddedClusterHomeDirectory(), "k0s")
}

// EmbeddedClusterSeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func (rc *runtimeConfig) EmbeddedClusterSeaweedfsSubDir() string {
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
	if err := os.MkdirAll(path, 0700); err != nil {
		logrus.Fatalf("unable to create embedded-cluster support dir: %s", err)
	}
	return path
}

// PathToEmbeddedClusterSupportFile is an utility function that returns the full path to
// a materialized support file. This function does not check if the file exists.
func (rc *runtimeConfig) PathToEmbeddedClusterSupportFile(name string) string {
	return filepath.Join(rc.EmbeddedClusterSupportSubDir(), name)
}

func (rc *runtimeConfig) WriteToDisk() error {
	location := ECConfigPath
	err := os.MkdirAll(filepath.Dir(location), 0755)
	if err != nil {
		return fmt.Errorf("unable to create runtime config directory: %w", err)
	}

	// check if the file already exists, if it does delete it
	err = os.RemoveAll(location)
	if err != nil {
		return fmt.Errorf("unable to remove existing runtime config: %w", err)
	}

	yml, err := yaml.Marshal(rc.spec)
	if err != nil {
		return fmt.Errorf("unable to marshal runtime config: %w", err)
	}

	err = os.WriteFile(location, yml, 0644)
	if err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	return nil
}

func (rc *runtimeConfig) LocalArtifactMirrorPort() int {
	if rc.spec.LocalArtifactMirror.Port > 0 {
		return rc.spec.LocalArtifactMirror.Port
	}
	return ecv1beta1.DefaultLocalArtifactMirrorPort
}

func (rc *runtimeConfig) AdminConsolePort() int {
	if rc.spec.AdminConsole.Port > 0 {
		return rc.spec.AdminConsole.Port
	}
	return ecv1beta1.DefaultAdminConsolePort
}

func (rc *runtimeConfig) HostCABundlePath() string {
	return rc.spec.HostCABundlePath
}

func (rc *runtimeConfig) SetDataDir(dataDir string) {
	rc.spec.DataDir = dataDir
}

func (rc *runtimeConfig) SetLocalArtifactMirrorPort(port int) {
	rc.spec.LocalArtifactMirror.Port = port
}

func (rc *runtimeConfig) SetAdminConsolePort(port int) {
	rc.spec.AdminConsole.Port = port
}

func (rc *runtimeConfig) SetManagerPort(port int) {
	rc.spec.Manager.Port = port
}

func (rc *runtimeConfig) SetHostCABundlePath(hostCABundlePath string) {
	rc.spec.HostCABundlePath = hostCABundlePath
}
