package cli

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg-new/kurl"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

// detectKurlMigration checks if this is a kURL cluster that needs migration to EC.
//
// Migration detection works by checking two SEPARATE clusters:
//  1. kURL cluster - accessed via /etc/kubernetes/admin.conf
//  2. EC cluster - accessed via EC's kubeconfig path (if it exists)
//
// The migration scenario is: kURL cluster exists, but EC cluster does not.
//
// Returns:
//   - (true, nil): Migration is needed (kURL cluster exists without EC cluster)
//   - (false, nil): Not a migration scenario, caller should continue with normal upgrade
//   - (false, error): Detection failed
func detectKurlMigration(ctx context.Context) (bool, error) {
	// Check if this is a kURL cluster
	kurlCfg, err := kurl.GetConfig(ctx)
	if err != nil {
		logrus.Debugf("Failed to detect kURL cluster: %v", err)
		return false, nil // Not fatal, continue with normal flow
	}
	if kurlCfg == nil {
		return false, nil // Not kURL, continue normally
	}

	logrus.Debugf("Detected kURL cluster with install directory: %s", kurlCfg.InstallDir)

	// Check if EC is already installed (checks separate EC cluster)
	ecInstalled, err := isECInstalled(ctx)
	if err != nil {
		logrus.Debugf("Failed to check EC installation status: %v", err)
		return false, nil // Not fatal, continue with normal flow
	}
	if ecInstalled {
		logrus.Debugf("Embedded Cluster already installed, proceeding with normal upgrade")
		return false, nil // EC already installed, do normal upgrade
	}

	// Migration needed - kURL cluster exists without EC cluster
	return true, nil
}

// isECInstalled checks if Embedded Cluster is already installed by checking for
// an EC Installation resource.
func isECInstalled(ctx context.Context) (bool, error) {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		// EC kubeconfig doesn't exist or can't connect
		return false, nil
	}

	// Check if Installation CRD exists by trying to get the latest installation
	// This leverages the existing kubeutils.GetLatestInstallation function
	// which returns ErrNoInstallations{} if no installations exist
	_, err = kubeutils.GetLatestInstallation(ctx, kcli)
	if err != nil {
		// If the error is ErrNoInstallations, EC is not installed
		if _, ok := err.(kubeutils.ErrNoInstallations); ok {
			return false, nil
		}
		// Other errors (like CRD doesn't exist) also mean not installed
		return false, nil
	}

	// Installation exists, EC is installed
	return true, nil
}
