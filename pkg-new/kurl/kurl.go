package kurl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
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
func GetConfig(ctx context.Context) (*Config, error) {
	// Try to create a client using kURL's kubeconfig at /etc/kubernetes/admin.conf
	// In production, this connects to the kURL cluster.
	// In dryrun tests where this file doesn't exist, we fall back to kubeutils.KubeClient()
	kcli, err := createKubeClientFromPath(KubeconfigPath)
	if err != nil {
		// kURL kubeconfig doesn't exist or can't connect
		// Fall back to standard client (enables dryrun testing)
		kcli, err = kubeutils.KubeClient()
		if err != nil {
			// No client available
			return nil, nil
		}
	}

	// Check for kURL ConfigMap and get install directory
	installDir, err := getInstallDirectory(ctx, kcli)
	if err != nil {
		// ConfigMap doesn't exist, not a kURL cluster
		return nil, nil
	}

	return &Config{
		Client:     kcli,
		InstallDir: installDir,
	}, nil
}

// createKubeClientFromPath creates a kubernetes client from a specific kubeconfig path.
func createKubeClientFromPath(kubeconfigPath string) (client.Client, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("build config from kubeconfig %s: %w", kubeconfigPath, err)
	}

	return client.New(cfg, client.Options{Scheme: kubeutils.Scheme})
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
