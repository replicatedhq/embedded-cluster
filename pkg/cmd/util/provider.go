package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewProviderFromCluster discovers the provider from the installation object. If there is no
// runtime config, this is probably a prior version of EC so we will have to fall back to the
// filesystem.
func NewProviderFromCluster(ctx context.Context, kcli client.Client) (*defaults.Provider, error) {
	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("get latest installation: %w", err)
	}

	if in.Spec.RuntimeConfig == nil || in.Spec.RuntimeConfig.DataDir == "" {
		// If there is no runtime config, this is probably a prior version of EC so we will have to
		// fall back to the filesystem.
		return NewProviderFromFilesystem()
	}
	provider := defaults.NewProviderFromRuntimeConfig(in.Spec.RuntimeConfig)
	logrus.Debugf("Got runtime config from installation with k0s data dir %s", provider.EmbeddedClusterK0sSubDir())
	return provider, nil
}

// NewProviderFromFilesystem returns a new provider from the filesystem. It supports older versions
// of EC that used a different directory for k0s and openebs.
func NewProviderFromFilesystem() (*defaults.Provider, error) {
	provider := defaults.NewProvider(ecv1beta1.DefaultDataDir)
	// ca.crt is available on both control plane and worker nodes
	_, err := os.Stat(filepath.Join(provider.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", provider.EmbeddedClusterK0sSubDir())
		return provider, nil
	}
	// Handle versions prior to consolidation of data dirs
	provider = defaults.NewProviderFromRuntimeConfig(&ecv1beta1.RuntimeConfigSpec{
		DataDir:                ecv1beta1.DefaultDataDir,
		K0sDataDirOverride:     "/var/lib/k0s",
		OpenEBSDataDirOverride: "/var/openebs",
	})
	// ca.crt is available on both control plane and worker nodes
	_, err = os.Stat(filepath.Join(provider.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", provider.EmbeddedClusterK0sSubDir())
		return provider, nil
	}
	return nil, fmt.Errorf("unable to discover provider from filesystem")
}
