package cli

import (
	"context"
	"fmt"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kurlConfigMapName        = "kurl-config"
	kurlConfigMapKey         = "kurl_install_directory"
	kurlDefaultInstallDir    = "/var/lib/kurl"
	kotsadmNamespace         = "kotsadm"
	kubeSystemNamespace      = "kube-system"
	kotsadmPasswordSecret    = "kotsadm-password"
	kotsadmPasswordSecretKey = "passwordBcrypt"
	kurlKubeconfigPath       = "/etc/kubernetes/admin.conf"
)

// isKurlCluster checks if the current system is a kURL cluster by:
// 1. Attempting to create a kubernetes client using the kURL kubeconfig
// 2. Checking if the kURL ConfigMap exists in kube-system namespace
// Returns whether it's a kURL cluster, the installer directory path, and any error.
func isKurlCluster(ctx context.Context) (bool, string, client.Client, error) {
	// Try to create a client using kURL kubeconfig
	kcli, err := createKubeClientFromPath(kurlKubeconfigPath)
	if err != nil {
		// kURL kubeconfig doesn't exist or can't connect
		return false, "", nil, nil
	}

	// Check for kURL ConfigMap and get install directory
	installDir, err := getKurlInstallDirectory(ctx, kcli)
	if err != nil {
		// ConfigMap doesn't exist, not a kURL cluster
		return false, "", nil, nil
	}

	return true, installDir, kcli, nil
}

// isECInstalled checks if Embedded Cluster is already installed by:
// 1. Attempting to create a kubernetes client using the EC kubeconfig
// 2. Checking if an Installation resource exists
func isECInstalled(ctx context.Context) (bool, error) {
	rc := runtimeconfig.New(nil)
	kubeconfigPath := rc.PathToKubeConfig()

	// Try to create a client using EC kubeconfig
	kcli, err := createKubeClientFromPath(kubeconfigPath)
	if err != nil {
		// EC kubeconfig doesn't exist or can't connect
		return false, nil
	}

	// Check if Installation CRD exists by trying to list installations
	installationList := &ecv1beta1.InstallationList{}
	if err := kcli.List(ctx, installationList); err != nil {
		// CRD doesn't exist or error listing
		return false, nil
	}

	// Return true if we found at least one installation
	return len(installationList.Items) > 0, nil
}

// createKubeClientFromPath creates a kubernetes client from a specific kubeconfig path.
func createKubeClientFromPath(kubeconfigPath string) (client.Client, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build config from kubeconfig %s: %w", kubeconfigPath, err)
	}

	return client.New(cfg, client.Options{Scheme: kubeutils.Scheme})
}

// getKurlInstallDirectory reads the kURL ConfigMap in kube-system namespace
// to determine the actual kURL installer directory (which may be customized).
// Returns the configured directory or the default if not specified.
func getKurlInstallDirectory(ctx context.Context, kcli client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: kubeSystemNamespace,
		Name:      kurlConfigMapName,
	}, cm)
	if err != nil {
		return "", fmt.Errorf("get kurl configmap from %s namespace: %w", kubeSystemNamespace, err)
	}

	installDir, exists := cm.Data[kurlConfigMapKey]
	if !exists || installDir == "" {
		// Return default if not customized
		return kurlDefaultInstallDir, nil
	}

	return installDir, nil
}

// exportKurlPasswordHash reads the kotsadm-password secret from the kotsadm namespace
// and returns the bcrypt password hash. This is used during migration to preserve the
// existing admin console password.
func exportKurlPasswordHash(ctx context.Context, kcli client.Client, namespace string) (string, error) {
	if namespace == "" {
		namespace = kotsadmNamespace
	}

	secret := &corev1.Secret{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      kotsadmPasswordSecret,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("read kotsadm-password secret from cluster: %w", err)
	}

	passwordHash, exists := secret.Data[kotsadmPasswordSecretKey]
	if !exists || len(passwordHash) == 0 {
		return "", fmt.Errorf("kotsadm-password secret is missing required passwordBcrypt data")
	}

	return string(passwordHash), nil
}

// detectKurlMigration checks if this is a kURL cluster that needs migration to EC.
// Returns:
//   - (true, nil): Migration is needed (kURL cluster without EC installed)
//   - (false, nil): Not a migration scenario, caller should continue with normal upgrade
//   - (false, error): Detection failed
func detectKurlMigration(ctx context.Context) (bool, error) {
	// Check if this is a kURL cluster
	kurlDetected, kurlInstallDir, _, err := isKurlCluster(ctx)
	if err != nil {
		logrus.Debugf("Failed to detect kURL cluster: %v", err)
		return false, nil // Not fatal, continue with normal flow
	}
	if !kurlDetected {
		return false, nil // Not kURL, continue normally
	}

	logrus.Debugf("Detected kURL cluster with install directory: %s", kurlInstallDir)

	// Check if EC is already installed
	ecInstalled, err := isECInstalled(ctx)
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
