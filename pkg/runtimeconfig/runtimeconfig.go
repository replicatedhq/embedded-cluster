package runtimeconfig

import (
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/paths"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
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
	tmpDir := EmbeddedClusterTmpSubDir()
	// We should not delete the tmp dir, rather we should empty its contents leaving
	// it in place. This is because commands such as `kubectl edit <resource>`
	// will create files in the tmp dir
	if err := helpers.RemoveAll(tmpDir); err != nil {
		logrus.Errorf("error removing %s dir: %s", tmpDir, err)
	}
}

// EmbeddedClusterDataDirectory returns the parent directory. Inside this parent directory we
// store all the embedded-cluster related files.
func EmbeddedClusterDataDirectory() string {
	if runtimeConfig.DataDir != "" {
		return runtimeConfig.DataDir
	}
	return ecv1beta1.DefaultDataDir
}

// EmbeddedClusterTmpSubDir returns the path to the tmp directory where embedded-cluster
// stores temporary files.
func EmbeddedClusterTmpSubDir() string {
	path := paths.TmpSubDir(EmbeddedClusterDataDirectory())

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster tmp dir: %s", err)
	}
	return path
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func EmbeddedClusterBinsSubDir() string {
	path := paths.BinsSubDir(EmbeddedClusterDataDirectory())

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDir returns the path to the directory where embedded-cluster helm charts
// are stored.
func EmbeddedClusterChartsSubDir() string {
	path := paths.ChartsSubDir(EmbeddedClusterDataDirectory())

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster charts dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDirNoCreate returns the path to the directory where embedded-cluster helm charts
// are stored without creating the directory if it does not exist.
func EmbeddedClusterChartsSubDirNoCreate() string {
	return paths.ChartsSubDir(EmbeddedClusterDataDirectory())
}

// EmbeddedClusterImagesSubDir returns the path to the directory where docker images are stored.
func EmbeddedClusterImagesSubDir() string {
	path := paths.ImagesSubDir(EmbeddedClusterDataDirectory())
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
	return paths.K0sSubDir(EmbeddedClusterDataDirectory())
}

// EmbeddedClusterSeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func EmbeddedClusterSeaweedfsSubDir() string {
	return paths.SeaweedfsSubDir(EmbeddedClusterDataDirectory())
}

// EmbeddedClusterOpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func EmbeddedClusterOpenEBSLocalSubDir() string {
	if runtimeConfig.OpenEBSDataDirOverride != "" {
		return runtimeConfig.OpenEBSDataDirOverride
	}
	return paths.OpenEBSLocalSubDir(EmbeddedClusterDataDirectory())
}

// PathToEmbeddedClusterBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster. This function does not check
// if the file exists.
func PathToEmbeddedClusterBinary(name string) string {
	return paths.PathToECBinary(EmbeddedClusterDataDirectory(), name)
}

// PathToKubeConfig returns the path to the kubeconfig file.
func PathToKubeConfig() string {
	return paths.PathToKubeConfig(EmbeddedClusterK0sSubDir())
}

// PathToKubeletConfig returns the path to the kubelet config file.
func PathToKubeletConfig() string {
	return paths.PathToKubeletConfig(EmbeddedClusterK0sSubDir())
}

// EmbeddedClusterSupportSubDir returns the path to the directory where embedded-cluster
// support files are stored. Things that are useful when providing end user support in
// a running cluster should be stored into this directory.
func EmbeddedClusterSupportSubDir() string {
	path := paths.SupportSubDir(EmbeddedClusterDataDirectory())
	if err := os.MkdirAll(path, 0700); err != nil {
		logrus.Fatalf("unable to create embedded-cluster support dir: %s", err)
	}
	return path
}

// PathToEmbeddedClusterSupportFile is an utility function that returns the full path to
// a materialized support file. This function does not check if the file exists.
func PathToEmbeddedClusterSupportFile(name string) string {
	return paths.PathToSupportFile(EmbeddedClusterDataDirectory(), name)
}

func WriteToDisk() error {
	location := paths.PathToECConfig()

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
	location := paths.PathToECConfig()

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
