package defaults

import (
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/sirupsen/logrus"
)

// NewProvider returns a new Provider using the provided data dir.
// Data is the base directory inside which all the other directories are
// created.
func NewProvider(dataDir string) *Provider {
	runtimeConfig := ecv1beta1.GetDefaultRuntimeConfig()
	runtimeConfig.DataDir = dataDir
	return NewProviderFromRuntimeConfig(runtimeConfig)
}

// NewProviderFromRuntimeConfig returns a new Provider using the provided runtime config.
func NewProviderFromRuntimeConfig(runtimeConfig *ecv1beta1.RuntimeConfigSpec) *Provider {
	obj := &Provider{
		runtimeConfig: runtimeConfig,
	}
	return obj
}

// Provider is an entity that provides default values used during
// EmbeddedCluster installation.
type Provider struct {
	runtimeConfig *ecv1beta1.RuntimeConfigSpec
}

// EmbeddedClusterHomeDirectory returns the parent directory. Inside this parent directory we
// store all the embedded-cluster related files.
func (d *Provider) EmbeddedClusterHomeDirectory() string {
	if d.runtimeConfig != nil && d.runtimeConfig.DataDir != "" {
		return d.runtimeConfig.DataDir
	}
	return ecv1beta1.DefaultDataDir
}

// EmbeddedClusterTmpSubDir returns the path to the tmp directory where embedded-cluster
// stores temporary files.
func (d *Provider) EmbeddedClusterTmpSubDir() string {
	path := filepath.Join(d.EmbeddedClusterHomeDirectory(), "tmp")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster tmp dir: %s", err)
	}
	return path
}

// EmbeddedClusterBinsSubDir returns the path to the directory where embedded-cluster binaries
// are stored.
func (d *Provider) EmbeddedClusterBinsSubDir() string {
	path := filepath.Join(d.EmbeddedClusterHomeDirectory(), "bin")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster bin dir: %s", err)
	}
	return path
}

// EmbeddedClusterChartsSubDir returns the path to the directory where embedded-cluster helm charts
// are stored.
func (d *Provider) EmbeddedClusterChartsSubDir() string {
	path := filepath.Join(d.EmbeddedClusterHomeDirectory(), "charts")

	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster charts dir: %s", err)
	}
	return path
}

// EmbeddedClusterImagesSubDir returns the path to the directory where docker images are stored.
func (d *Provider) EmbeddedClusterImagesSubDir() string {
	path := filepath.Join(d.EmbeddedClusterHomeDirectory(), "images")
	if err := os.MkdirAll(path, 0755); err != nil {
		logrus.Fatalf("unable to create embedded-cluster images dir: %s", err)
	}
	return path
}

// EmbeddedClusterK0sSubDir returns the path to the directory where k0s data is stored.
func (d *Provider) EmbeddedClusterK0sSubDir() string {
	if d.runtimeConfig != nil && d.runtimeConfig.K0sDataDirOverride != "" {
		return d.runtimeConfig.K0sDataDirOverride
	}
	return filepath.Join(d.EmbeddedClusterHomeDirectory(), "k0s")
}

// EmbeddedClusterSeaweedfsSubDir returns the path to the directory where seaweedfs data is stored.
func (d *Provider) EmbeddedClusterSeaweedfsSubDir() string {
	return filepath.Join(d.EmbeddedClusterHomeDirectory(), "seaweedfs")
}

// EmbeddedClusterOpenEBSLocalSubDir returns the path to the directory where OpenEBS local data is stored.
func (d *Provider) EmbeddedClusterOpenEBSLocalSubDir() string {
	if d.runtimeConfig != nil && d.runtimeConfig.OpenEBSDataDirOverride != "" {
		return d.runtimeConfig.OpenEBSDataDirOverride
	}
	return filepath.Join(d.EmbeddedClusterHomeDirectory(), "openebs-local")
}

// PathToEmbeddedClusterBinary is an utility function that returns the full path to a
// materialized binary that belongs to embedded-cluster. This function does not check
// if the file exists.
func (d *Provider) PathToEmbeddedClusterBinary(name string) string {
	return filepath.Join(d.EmbeddedClusterBinsSubDir(), name)
}

// PathToKubeConfig returns the path to the kubeconfig file.
func (d *Provider) PathToKubeConfig() string {
	return filepath.Join(d.EmbeddedClusterK0sSubDir(), "pki/admin.conf")
}

// EmbeddedClusterSupportSubDir returns the path to the directory where embedded-cluster
// support files are stored. Things that are useful when providing end user support in
// a running cluster should be stored into this directory.
func (d *Provider) EmbeddedClusterSupportSubDir() string {
	path := filepath.Join(d.EmbeddedClusterHomeDirectory(), "support")
	if err := os.MkdirAll(path, 0700); err != nil {
		logrus.Fatalf("unable to create embedded-cluster support dir: %s", err)
	}
	return path
}

// PathToEmbeddedClusterSupportFile is an utility function that returns the full path to
// a materialized support file. This function does not check if the file exists.
func (d *Provider) PathToEmbeddedClusterSupportFile(name string) string {
	return filepath.Join(d.EmbeddedClusterSupportSubDir(), name)
}

func (d *Provider) LocalArtifactMirrorPort() int {
	if d.runtimeConfig != nil && d.runtimeConfig.LocalArtifactMirror.Port > 0 {
		return d.runtimeConfig.LocalArtifactMirror.Port
	}
	return ecv1beta1.DefaultLocalArtifactMirrorPort
}

func (d *Provider) AdminConsolePort() int {
	if d.runtimeConfig != nil && d.runtimeConfig.AdminConsole.Port > 0 {
		return d.runtimeConfig.AdminConsole.Port
	}
	return ecv1beta1.DefaultAdminConsolePort
}

func (d *Provider) SetAdminConsolePort(port int) {
	if d.runtimeConfig != nil {
		d.runtimeConfig.AdminConsole.Port = port
	}
}

func (d *Provider) SetLocalArtifactMirrorPort(port int) {
	if d.runtimeConfig != nil {
		d.runtimeConfig.LocalArtifactMirror.Port = port
	}
}

func (d *Provider) SetDataDir(dataDir string) {
	if d.runtimeConfig != nil {
		d.runtimeConfig.DataDir = dataDir
	}
}

func (p *Provider) GetRuntimeConfig() *ecv1beta1.RuntimeConfigSpec {
	return p.runtimeConfig
}

func (p *Provider) SetRuntimeConfig(runtimeConfig *ecv1beta1.RuntimeConfigSpec) {
	p.runtimeConfig = runtimeConfig
}
