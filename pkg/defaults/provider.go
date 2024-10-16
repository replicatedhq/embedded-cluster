package defaults

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewProvider returns a new Provider using the provided data dir.
// Data is the base directory inside which all the other directories are
// created.
func NewProvider(dataDir string) *Provider {
	return NewProviderFromRuntimeConfig(&ecv1beta1.RuntimeConfigSpec{
		DataDir: dataDir,
	})
}

// NewProviderFromRuntimeConfig returns a new Provider using the provided runtime config.
func NewProviderFromRuntimeConfig(runtimeConfig *ecv1beta1.RuntimeConfigSpec) *Provider {
	obj := &Provider{
		runtimeConfig: runtimeConfig,
	}
	return obj
}

// NewProviderFromCluster discovers the provider from the installation object. If there is no
// runtime config, this is probably a prior version of EC so we will have to fall back to the
// filesystem.
func NewProviderFromCluster(ctx context.Context, cli client.Client) (*Provider, error) {
	in, err := kubeutils.GetLatestInstallation(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("get latest installation: %w", err)
	}

	if in.Spec.RuntimeConfig == nil {
		// If there is no runtime config, this is probably a prior version of EC so we will have to
		// fall back to the filesystem.
		return NewProviderFromFilesystem()
	}
	return NewProviderFromRuntimeConfig(in.Spec.RuntimeConfig), nil
}

// NewProviderFromFilesystem returns a new provider from the filesystem. It supports older versions
// of EC that used a different directory for k0s and openebs.
func NewProviderFromFilesystem() (*Provider, error) {
	provider := NewProvider(ecv1beta1.DefaultDataDir)
	_, err := os.Stat(provider.PathToKubeConfig())
	if err == nil {
		return provider, nil
	}
	// Handle versions prior to consolidation of data dirs
	provider = NewProviderFromRuntimeConfig(&ecv1beta1.RuntimeConfigSpec{
		DataDir:                ecv1beta1.DefaultDataDir,
		K0sDataDirOverride:     "/var/lib/k0s",
		OpenEBSDataDirOverride: "/var/openebs",
	})
	_, err = os.Stat(provider.PathToKubeConfig())
	if err == nil {
		return provider, nil
	}
	return nil, fmt.Errorf("unable to discover provider from filesystem")
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

// PodAndServiceCIDRs returns the pod and service CIDRs for the cluster.
func (d *Provider) PodAndServiceCIDRs() (string, string, error) {
	cidr := ecv1beta1.DefaultNetworkCIDR
	if d.runtimeConfig != nil && d.runtimeConfig.NetworkCIDR != "" {
		cidr = d.runtimeConfig.NetworkCIDR
	}

	if err := netutils.ValidateCIDR(cidr, 16, true); err != nil {
		return "", "", fmt.Errorf("invalid network cidr: %w", err)
	}

	return netutils.SplitNetworkCIDR(cidr)
}
