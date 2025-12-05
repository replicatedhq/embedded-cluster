package install

import (
	"context"
	"fmt"
	"os"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided config values
func (m *appInstallManager) Install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initKubeClient(); err != nil {
		return fmt.Errorf("init kube client: %w", err)
	}

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Create or update secret with config values before installing
	if err := m.createConfigValuesSecret(ctx, configValues, kotsadmNamespace); err != nil {
		return fmt.Errorf("creating config values secret: %w", err)
	}

	ecDomains := utils.GetDomains(m.releaseData)

	installOpts := kotscli.InstallOptions{
		AppSlug:      license.Spec.AppSlug,
		License:      m.license,
		Namespace:    kotsadmNamespace,
		ClusterID:    m.clusterID,
		AirgapBundle: m.airgapBundle,
		// Skip running the KOTS app preflights in the Admin Console; they run in the manager experience installer when ENABLE_V3 is enabled
		SkipPreflights: true,
		// Skip pushing images to the registry since we do it separately earlier in the install process
		DisableImagePush:      true,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		Stdout:                m.newLogWriter(),
	}

	configValuesFile, err := m.createConfigValuesFile(configValues)
	if err != nil {
		return fmt.Errorf("creating config values file: %w", err)
	}
	installOpts.ConfigValuesFile = configValuesFile

	if m.kotsCLI != nil {
		return m.kotsCLI.Install(installOpts)
	}

	return kotscli.Install(installOpts)
}

// createConfigValuesFile creates a temporary file with the config values
func (m *appInstallManager) createConfigValuesFile(configValues kotsv1beta1.ConfigValues) (string, error) {
	// Use Kubernetes-specific YAML serialization to properly handle TypeMeta and ObjectMeta
	data, err := kyaml.Marshal(configValues)
	if err != nil {
		return "", fmt.Errorf("marshal config values: %w", err)
	}

	configValuesFile, err := os.CreateTemp("", "config-values*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer configValuesFile.Close()

	if _, err := configValuesFile.Write(data); err != nil {
		_ = os.Remove(configValuesFile.Name())
		return "", fmt.Errorf("write config values to temp file: %w", err)
	}

	return configValuesFile.Name(), nil
}

// createConfigValuesSecret creates or updates a Kubernetes secret with the config values.
// TODO: Handle 1MB size limitation by storing large file data fields as pointers to other secrets
// TODO: Consider maintaining history of config values for potential rollbacks
func (m *appInstallManager) createConfigValuesSecret(ctx context.Context, configValues kotsv1beta1.ConfigValues, namespace string) error {
	// Get app slug and version from release data
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if m.releaseData == nil || m.releaseData.ChannelRelease == nil {
		return fmt.Errorf("release data is required for secret creation")
	}

	// Marshal config values to YAML
	data, err := kyaml.Marshal(configValues)
	if err != nil {
		return fmt.Errorf("marshal config values: %w", err)
	}

	secretName := fmt.Sprintf("%s-config-values", license.Spec.AppSlug)

	// Create secret object
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       license.Spec.AppSlug,
				"app.kubernetes.io/version":    m.releaseData.ChannelRelease.VersionLabel,
				"app.kubernetes.io/component":  "config",
				"app.kubernetes.io/part-of":    "embedded-cluster",
				"app.kubernetes.io/managed-by": "embedded-cluster-installer",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"config-values.yaml": data,
		},
	}

	// Try to create the secret
	if err := m.kcli.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create config values secret: %w", err)
		}

		// Secret exists, get and update it
		existingSecret := &corev1.Secret{}
		if err := m.kcli.Get(ctx, client.ObjectKey{
			Name:      secretName,
			Namespace: namespace,
		}, existingSecret); err != nil {
			return fmt.Errorf("get existing config values secret: %w", err)
		}

		// Update the existing secret's data and labels
		existingSecret.Data = secret.Data
		existingSecret.Labels = secret.Labels

		if err := m.kcli.Update(ctx, existingSecret); err != nil {
			return fmt.Errorf("update config values secret: %w", err)
		}
	}

	return nil
}
