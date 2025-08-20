package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"helm.sh/helm/v3/pkg/chart/loader"
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided config values
func (m *appInstallManager) Install(ctx context.Context, configValues kotsv1beta1.ConfigValues) (finalErr error) {
	if err := m.setStatus(types.StateRunning, "Installing application"); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setStatus(types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setStatus(types.StateSucceeded, "Installation complete"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	if err := m.install(ctx, configValues); err != nil {
		return err
	}

	return nil
}

func (m *appInstallManager) install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	// Setup Helm client
	if err := m.setupHelmClient(); err != nil {
		return fmt.Errorf("setup helm client: %w", err)
	}

	// Install Helm charts
	if err := m.installHelmCharts(ctx); err != nil {
		return fmt.Errorf("install helm charts: %w", err)
	}
	ecDomains := utils.GetDomains(m.releaseData)

	installOpts := kotscli.InstallOptions{
		AppSlug:               license.Spec.AppSlug,
		License:               m.license,
		Namespace:             constants.KotsadmNamespace,
		ClusterID:             m.clusterID,
		AirgapBundle:          m.airgapBundle,
		SkipPreflights:        true,
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
		return "", fmt.Errorf("marshaling config values: %w", err)
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

func (m *appInstallManager) installHelmCharts(ctx context.Context) error {
	logFn := m.logFn("app-helm")

	if m.releaseData == nil || len(m.releaseData.HelmChartArchives) == 0 {
		logFn("no helm charts found in release data")
		return nil
	}

	logFn("installing %d helm charts from release data", len(m.releaseData.HelmChartArchives))

	for i, chartArchive := range m.releaseData.HelmChartArchives {
		logFn("installing chart %d/%d", i+1, len(m.releaseData.HelmChartArchives))

		// Write chart archive to temp file
		chartPath, err := m.writeChartArchiveToTemp(chartArchive)
		if err != nil {
			return fmt.Errorf("write chart archive %d to temp: %w", i, err)
		}
		defer os.Remove(chartPath)

		// Get chart name to use as release name
		ch, err := loader.LoadArchive(bytes.NewReader(chartArchive))
		if err != nil {
			return fmt.Errorf("load archive: %w", err)
		}

		// Install chart using Helm client
		_, err = m.hcli.Install(ctx, helm.InstallOptions{
			ChartPath: chartPath,
			// TODO: namespace should come from HelmChart custom resource instead of constants.KotsadmNamespace
			Namespace: constants.KotsadmNamespace,
			// TODO: release name should come from HelmChart custom resource instead of chart name
			ReleaseName: ch.Metadata.Name,
		})
		if err != nil {
			return fmt.Errorf("install chart %d: %w", i, err)
		}

		logFn("successfully installed chart %d/%d", i+1, len(m.releaseData.HelmChartArchives))
	}

	return nil
}
