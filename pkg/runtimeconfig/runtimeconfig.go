package runtimeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

var (
	runtimeConfig = ecv1beta1.GetDefaultRuntimeConfig()
)

func Set(rc *ecv1beta1.RuntimeConfigSpec) {
	if rc == nil {
		// runtime config is nil in old installation objects so this keeps the default.
		return
	}
	runtimeConfig = rc
}

func Get() *ecv1beta1.RuntimeConfigSpec {
	return runtimeConfig
}

func Cleanup() {
	emptyTmpDir()
}

func emptyTmpDir() {
	tmpDir := EmbeddedClusterTmpSubDir()
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		logrus.Errorf("error reading embedded-cluster tmp dir: %s", err)
		return
	}

	for _, entry := range entries {
		path := filepath.Join(tmpDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			logrus.Errorf("error removing item %s: %s", path, err)
		}
	}
}

// EmbeddedClusterHomeDirectory returns the parent directory. Inside this parent directory we
// store all the embedded-cluster related files.
func EmbeddedClusterHomeDirectory() string {
	if runtimeConfig.DataDir != "" {
		return runtimeConfig.DataDir
	}
	return ecv1beta1.DefaultDataDir
}

// EmbeddedClusterTmpSubDir returns the path to the tmp directory where embedded-cluster
// stores temporary files.
func EmbeddedClusterTmpSubDir() string {
	path := filepath.Join(EmbeddedClusterHomeDirectory(), "tmp")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster tmp dir: %s", err)
	}
	return path
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func EmbeddedClusterBinsSubDir() string {
	path := filepath.Join(EmbeddedClusterHomeDirectory(), "bin")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDir returns the path to the directory where embedded-cluster helm charts
// are stored.
func EmbeddedClusterChartsSubDir() string {
	path := filepath.Join(EmbeddedClusterHomeDirectory(), "charts")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster charts dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDirNoCreate returns the path to the directory where embedded-cluster helm charts
// are stored without creating the directory if it does not exist.
func EmbeddedClusterChartsSubDirNoCreate() string {
	return filepath.Join(EmbeddedClusterHomeDirectory(), "charts")
}

// EmbeddedClusterImagesSubDir returns the path to the directory where docker images are stored.
func EmbeddedClusterImagesSubDir() string {
	path := filepath.Join(EmbeddedClusterHomeDirectory(), "images")
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster images dir: %s", err)
	}
	return path
}

// EmbeddedClusterK0sSubDir returns the path to the directory where k0s data is stored.
func EmbeddedClusterK0sSubDir() string {
	if runtimeConfig.K0sDataDirOverride != "" {
		return runtimeConfig.K0sDataDirOverride
	}
	return filepath.Join(EmbeddedClusterHomeDirectory(), "k0s")
}

// EmbeddedClusterSeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func EmbeddedClusterSeaweedfsSubDir() string {
	return filepath.Join(EmbeddedClusterHomeDirectory(), "seaweedfs")
}

// EmbeddedClusterOpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func EmbeddedClusterOpenEBSLocalSubDir() string {
	if runtimeConfig.OpenEBSDataDirOverride != "" {
		return runtimeConfig.OpenEBSDataDirOverride
	}
	return filepath.Join(EmbeddedClusterHomeDirectory(), "openebs-local")
}

// PathToEmbeddedClusterBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster. This function does not check
// if the file exists.
func PathToEmbeddedClusterBinary(name string) string {
	return filepath.Join(EmbeddedClusterBinsSubDir(), name)
}

// PathToKubeConfig returns the path to the kubeconfig file.
func PathToKubeConfig() string {
	return filepath.Join(EmbeddedClusterK0sSubDir(), "pki/admin.conf")
}

// PathToKubeletConfig returns the path to the kubelet config file.
func PathToKubeletConfig() string {
	return filepath.Join(EmbeddedClusterK0sSubDir(), "kubelet.conf")
}

// EmbeddedClusterSupportSubDir returns the path to the directory where embedded-cluster
// support files are stored. Things that are useful when providing end user support in
// a running cluster should be stored into this directory.
func EmbeddedClusterSupportSubDir() string {
	path := filepath.Join(EmbeddedClusterHomeDirectory(), "support")
	if err := os.MkdirAll(path, 0700); err != nil {
		logrus.Fatalf("unable to create embedded-cluster support dir: %s", err)
	}
	return path
}

// PathToEmbeddedClusterSupportFile is an utility function that returns the full path to
// a materialized support file. This function does not check if the file exists.
func PathToEmbeddedClusterSupportFile(name string) string {
	return filepath.Join(EmbeddedClusterSupportSubDir(), name)
}

func WriteToDisk() error {
	location := PathToECConfig()

	err := os.MkdirAll(filepath.Dir(location), 0755)
	if err != nil {
		return fmt.Errorf("unable to create runtime config directory: %w", err)
	}

	// check if the file already exists, if it does delete it
	err = os.RemoveAll(location)
	if err != nil {
		return fmt.Errorf("unable to remove existing runtime config: %w", err)
	}

	yml, err := yaml.Marshal(runtimeConfig)
	if err != nil {
		return fmt.Errorf("unable to marshal runtime config: %w", err)
	}

	err = os.WriteFile(location, yml, 0644)
	if err != nil {
		return fmt.Errorf("unable to write runtime config: %w", err)
	}

	return nil
}

func ReadFromDisk() (*ecv1beta1.RuntimeConfigSpec, error) {
	location := PathToECConfig()

	data, err := os.ReadFile(location)
	if err != nil {
		return nil, fmt.Errorf("unable to read runtime config: %w", err)
	}

	var spec ecv1beta1.RuntimeConfigSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unable to unmarshal runtime config: %w", err)
	}

	return &spec, nil
}

func LocalArtifactMirrorPort() int {
	if runtimeConfig.LocalArtifactMirror.Port > 0 {
		return runtimeConfig.LocalArtifactMirror.Port
	}
	return ecv1beta1.DefaultLocalArtifactMirrorPort
}

func AdminConsolePort() int {
	if runtimeConfig.AdminConsole.Port > 0 {
		return runtimeConfig.AdminConsole.Port
	}
	return ecv1beta1.DefaultAdminConsolePort
}

func HostCABundlePath() string {
	return runtimeConfig.HostCABundlePath
}

func SetDataDir(dataDir string) {
	runtimeConfig.DataDir = dataDir
}

func SetLocalArtifactMirrorPort(port int) {
	runtimeConfig.LocalArtifactMirror.Port = port
}

func SetAdminConsolePort(port int) {
	runtimeConfig.AdminConsole.Port = port
}

func SetManagerPort(port int) {
	runtimeConfig.Manager.Port = port
}

func SetHostCABundlePath(hostCABundlePath string) {
	runtimeConfig.HostCABundlePath = hostCABundlePath
}
