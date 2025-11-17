package kurl

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

const (
	ConfigMapName            = "kurl-config"
	ConfigMapKey             = "kurl_install_directory"
	DefaultInstallDir        = "/var/lib/kurl"
	KotsadmNamespace         = "kotsadm"
	KubeSystemNamespace      = "kube-system"
	KotsadmPasswordSecret    = "kotsadm-password"
	KotsadmPasswordSecretKey = "passwordBcrypt"
	KubeconfigPath           = "/etc/kubernetes/admin.conf"
)

// Config holds configuration information about a kURL cluster.
type Config struct {
	// Client is a kubernetes client authenticated to the kURL cluster.
	Client client.Client
	// InstallDir is the directory where kURL installed its assets.
	InstallDir string
}

// GetConfig attempts to detect and return configuration for a kURL cluster.
// Returns nil if no kURL cluster is detected, or an error if detection fails.
// The KURL_KUBECONFIG_PATH environment variable can override the default path for testing.
func GetConfig(ctx context.Context) (*Config, error) {
	// Use environment variable override for tests, otherwise use default
	kubeconfigPath := os.Getenv("KURL_KUBECONFIG_PATH")
	if kubeconfigPath == "" {
		kubeconfigPath = KubeconfigPath
	}

	// Check if kURL's kubeconfig file exists
	if _, err := os.Stat(kubeconfigPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - not a kURL cluster
			return nil, nil
		}
		// File exists but can't stat it - that's an error
		return nil, fmt.Errorf("failed to check kurl kubeconfig at %s: %w", kubeconfigPath, err)
	}

	// Create kURL client
	kcli, err := kubeutils.KURLKubeClient(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create kurl client: %w", err)
	}

	// Check for kURL ConfigMap and get install directory
	installDir, err := getInstallDirectory(ctx, kcli)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check for kurl configmap: %w", err)
	}

	return &Config{
		Client:     kcli,
		InstallDir: installDir,
	}, nil
}

// getInstallDirectory reads the kURL ConfigMap in kube-system namespace
// to determine the actual kURL installer directory (which may be customized).
// Returns the configured directory or the default if not specified.
func getInstallDirectory(ctx context.Context, kcli client.Client) (string, error) {
	cm := &corev1.ConfigMap{}
	err := kcli.Get(ctx, client.ObjectKey{
		Namespace: KubeSystemNamespace,
		Name:      ConfigMapName,
	}, cm)
	if err != nil {
		return "", fmt.Errorf("get kurl configmap from %s namespace: %w", KubeSystemNamespace, err)
	}

	installDir, exists := cm.Data[ConfigMapKey]
	if !exists || installDir == "" {
		// Return default if not customized
		return DefaultInstallDir, nil
	}

	return installDir, nil
}

// GetPasswordHash reads the kotsadm-password secret from the kotsadm namespace
// and returns the bcrypt password hash. This is used during migration to preserve the
// existing admin console password.
func GetPasswordHash(ctx context.Context, cfg *Config, namespace string) (string, error) {
	if namespace == "" {
		namespace = KotsadmNamespace
	}

	secret := &corev1.Secret{}
	err := cfg.Client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      KotsadmPasswordSecret,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("read kotsadm-password secret from cluster: %w", err)
	}

	passwordHash, exists := secret.Data[KotsadmPasswordSecretKey]
	if !exists || len(passwordHash) == 0 {
		return "", fmt.Errorf("kotsadm-password secret is missing required passwordBcrypt data")
	}

	return string(passwordHash), nil
}
