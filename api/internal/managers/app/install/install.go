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
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided config values
func (m *appInstallManager) install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	licenseWrapper, err := licensewrapper.LoadLicenseFromBytes(m.license)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initKubeClient(); err != nil {
		return fmt.Errorf("init kube client: %w", err)
	}

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	ecDomains := utils.GetDomains(m.releaseData)

	installOpts := kotscli.InstallOptions{
		AppSlug:      licenseWrapper.GetAppSlug(),
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
