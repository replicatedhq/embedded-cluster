package util

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
)

// InitBestRuntimeConfig returns the best runtime config available. It will try to get the runtime config from
// the cluster (if it's up) and will fall back to the /etc/embedded-cluster/ec.yaml file, the filesystem, or the default.
func InitBestRuntimeConfig(ctx context.Context) runtimeconfig.RuntimeConfig {
	// It's possible that the cluster is not up
	if rc, err := GetRuntimeConfigFromCluster(ctx); err == nil {
		return rc
	}

	// There might be a runtime config file
	if rc, err := runtimeconfig.NewFromDisk(); err == nil {
		return rc
	}

	// Otherwise, fall back to the filesystem
	if rc, err := GetRuntimeConfigFromFilesystem(); err == nil {
		return rc
	}

	// If we can't find a runtime config, return the default
	return runtimeconfig.New(nil)
}

// GetRuntimeConfigFromCluster discovers the runtime config from the installation object. If there is no
// runtime config, this is probably a prior version of EC so we will have to fall back to the
// filesystem.
func GetRuntimeConfigFromCluster(ctx context.Context) (runtimeconfig.RuntimeConfig, error) {
	status, err := k0s.GetStatus(ctx)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s does not seem to be installed on this node", runtimeconfig.BinaryName())
		}
		return nil, fmt.Errorf("get k0s status: %w", err)
	}

	kubeconfigPath := status.Vars.AdminKubeConfigPath
	os.Setenv("KUBECONFIG", kubeconfigPath)

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	in, err := kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		return nil, fmt.Errorf("get latest installation: %w", err)
	}

	if in.Spec.RuntimeConfig == nil || in.Spec.RuntimeConfig.DataDir == "" {
		// If there is no runtime config, this is probably a prior version of EC so we will have to
		// fall back to the filesystem.
		return GetRuntimeConfigFromFilesystem()
	}

	rc := runtimeconfig.New(in.Spec.RuntimeConfig)
	logrus.Debugf("Got runtime config from installation with k0s data dir %s", rc.EmbeddedClusterK0sSubDir())

	return rc, nil
}

// GetRuntimeConfigFromFilesystem returns initializes the runtime config from the filesystem. It supports older versions
// of EC that used a different directory for k0s and openebs.
func GetRuntimeConfigFromFilesystem() (runtimeconfig.RuntimeConfig, error) {
	rc := runtimeconfig.New(nil)

	// ca.crt is available on both control plane and worker nodes
	_, err := os.Stat(filepath.Join(rc.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", rc.EmbeddedClusterK0sSubDir())
		return rc, nil
	}

	// Handle versions prior to consolidation of data dirs
	rc.Set(&ecv1beta1.RuntimeConfigSpec{
		DataDir:                ecv1beta1.DefaultDataDir,
		K0sDataDirOverride:     "/var/lib/k0s",
		OpenEBSDataDirOverride: "/var/openebs",
	})

	// ca.crt is available on both control plane and worker nodes
	_, err = os.Stat(filepath.Join(rc.EmbeddedClusterK0sSubDir(), "pki/ca.crt"))
	if err == nil {
		logrus.Debugf("Got runtime config from filesystem with k0s data dir %s", rc.EmbeddedClusterK0sSubDir())
		return rc, nil
	}

	return nil, fmt.Errorf("unable to discover runtime config from filesystem")
}
