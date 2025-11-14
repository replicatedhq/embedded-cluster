package cli

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg-new/kurl"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// detectKurlMigration checks if this is a kURL cluster that needs migration to EC.
// Returns:
//   - (true, nil): Migration is needed (kURL cluster without EC installed)
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

	// Check if EC is already installed using the kURL client
	ecInstalled, err := isECInstalled(ctx, kurlCfg)
	if err != nil {
		logrus.Debugf("Failed to check EC installation status: %v", err)
		return false, nil // Not fatal, continue with normal flow
	}
	if ecInstalled {
		logrus.Debugf("Embedded Cluster already installed, proceeding with normal upgrade")
		return false, nil // EC already installed, do normal upgrade
	}

	// Migration needed - kURL cluster without EC
	return true, nil
}

// isECInstalled checks if Embedded Cluster is already installed by:
// 1. Attempting to create a kubernetes client using the EC kubeconfig
// 2. Checking if an Installation resource exists using kubeutils.GetLatestInstallation
func isECInstalled(ctx context.Context, kurlCfg *kurl.Config) (bool, error) {
	rc := runtimeconfig.New(nil)
	kubeconfigPath := rc.PathToKubeConfig()

	// Try to create a client using EC kubeconfig
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		// EC kubeconfig doesn't exist or can't connect
		return false, nil
	}

	kcli, err := client.New(cfg, client.Options{Scheme: kubeutils.Scheme})
	if err != nil {
		// Can't create client
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
