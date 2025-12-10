package appupgrademanager

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

// Upgrade upgrades the app with the provided config values
func (m *appUpgradeManager) Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	ecDomains := utils.GetDomains(m.releaseData)

	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return fmt.Errorf("release data with app slug is required for upgrade")
	}

	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	configValuesFile, err := m.createConfigValuesFile(configValues)
	if err != nil {
		return fmt.Errorf("creating config values file: %w", err)
	}
	defer os.Remove(configValuesFile)

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Create or update secret with config values before upgrading
	if err := m.createConfigValuesSecret(ctx, license.Spec.AppSlug, kotsadmNamespace, configValues); err != nil {
		return fmt.Errorf("creating config values secret: %w", err)
	}

	deployOpts := kotscli.DeployOptions{
		AppSlug:               m.releaseData.ChannelRelease.AppSlug,
		License:               m.license,
		Namespace:             kotsadmNamespace,
		ClusterID:             m.clusterID,
		AirgapBundle:          m.airgapBundle,
		ConfigValuesFile:      configValuesFile,
		ChannelID:             m.releaseData.ChannelRelease.ChannelID,
		ChannelSequence:       m.releaseData.ChannelRelease.ChannelSequence,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		// Skip running the KOTS app preflights in the Admin Console; they run in the manager experience installer when ENABLE_V3 is enabled
		SkipPreflights: os.Getenv("ENABLE_V3") == "1",
		Stdout:         m.newLogWriter(),
	}

	if m.airgapBundle != "" {
		m.log(deployOpts, "Deploying airgap bundle with license sync, configuration, and deployment")
	} else {
		m.log(deployOpts, "Deploying online update with license sync, configuration, and deployment")
	}

	// Deploy with KOTS
	if m.kotsCLI != nil {
		if err := m.kotsCLI.Deploy(deployOpts); err != nil {
			return fmt.Errorf("kots cli deploy: %w", err)
		}
	} else {
		if err := kotscli.Deploy(deployOpts); err != nil {
			return fmt.Errorf("kots cli deploy: %w", err)
		}
	}

	return nil
}

// createConfigValuesFile creates a temporary file with the config values
func (m *appUpgradeManager) createConfigValuesFile(configValues kotsv1beta1.ConfigValues) (string, error) {
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
// TODO: Consolidate with similar function in app install manager
// TODO: Handle 1MB size limitation by storing large file data fields as pointers to other secrets
// TODO: Consider maintaining history of config values for potential rollbacks
func (m *appUpgradeManager) createConfigValuesSecret(ctx context.Context, appSlug string, namespace string, configValues kotsv1beta1.ConfigValues) error {
	if m.releaseData == nil || m.releaseData.ChannelRelease == nil {
		return fmt.Errorf("release data is required for secret creation")
	}

	// Marshal config values to YAML
	data, err := kyaml.Marshal(configValues)
	if err != nil {
		return fmt.Errorf("marshal config values: %w", err)
	}

	// Create secret object
	secret := utils.GenerateConfigValueSecret(data, appSlug, namespace, m.releaseData.ChannelRelease.VersionLabel)

	// Try to create the secret
	if err := m.kcli.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("create config values secret: %w", err)
		}

		// Secret exists, get and update it
		existingSecret := &corev1.Secret{}
		if err := m.kcli.Get(ctx, client.ObjectKey{
			Name:      secret.Name,
			Namespace: secret.Namespace,
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
