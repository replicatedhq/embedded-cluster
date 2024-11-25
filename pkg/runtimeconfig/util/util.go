package util

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

// InitBestRuntimeConfig initializes the runtime config from the cluster (if it's up) and will fall back to
// the /etc/embedded-cluster/ec.yaml file, the filesystem, or the default.
func InitBestRuntimeConfig(ctx context.Context) {
	// It's possible that the cluster is not up
	if err := InitRuntimeConfigFromCluster(ctx); err == nil {
		return
	}

	// There might be a runtime config file
	if runtimeConfig, err := runtimeconfig.ReadFromDisk(); err == nil {
		runtimeconfig.Set(runtimeConfig)
		return
	}

	// Otherwise, fall back to the filesystem
	if err := InitRuntimeConfigFromFilesystem(); err == nil {
		return
	}

	// If we can't find a runtime config, keep the default
	return
}

// InitRuntimeConfigFromCluster discovers the runtime config from the installation object. If there is no
// runtime config, this is probably a prior version of EC so we will have to fall back to the
// filesystem.
func InitRuntimeConfigFromCluster(ctx context.Context) error {
	status, err := k0s.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("get k0s status: %w", err)
	}

	kubeconfigPath := status.Vars.AdminKubeConfigPath
	os.Setenv("KUBECONFIG", kubeconfigPath)

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return fmt.Errorf("get latest installation: %w", err)
	}

	if in.Spec.RuntimeConfig == nil || in.Spec.RuntimeConfig.DataDir == "" {
		// If there is no runtime config, this is probably a prior version of EC so we will have to
		// fall back to the filesystem.
		return InitRuntimeConfigFromFilesystem()
	}

	runtimeconfig.Set(in.Spec.RuntimeConfig)
	logrus.Debugf("Got runtime config from installation with k0s data dir %s", runtimeconfig.EmbeddedClusterK0sSubDir())

	return nil
}

// InitRuntimeConfigFromFilesystem returns initializes the runtime config from the filesystem. It supports older versions
// of EC that used a different directory for k0s and openebs.
func InitRuntimeConfigFromFilesystem() error {
	// ca.crt is available on both control plane and worker nodes
	_, err := os.Stat(filepath.Join(runtimeconfig.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", runtimeconfig.EmbeddedClusterK0sSubDir())
		return nil
	}

	// Handle versions prior to consolidation of data dirs
	runtimeconfig.Set(&ecv1beta1.RuntimeConfigSpec{
		DataDir:                ecv1beta1.DefaultDataDir,
		K0sDataDirOverride:     "/var/lib/k0s",
		OpenEBSDataDirOverride: "/var/openebs",
	})

	// ca.crt is available on both control plane and worker nodes
	_, err = os.Stat(filepath.Join(runtimeconfig.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", runtimeconfig.EmbeddedClusterK0sSubDir())
		return nil
	}

	return fmt.Errorf("unable to discover runtime config from filesystem")
}
