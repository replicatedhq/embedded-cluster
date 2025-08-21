package install

import (
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
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided installable Helm charts and config values
func (m *appInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart, kotsConfigValues kotsv1beta1.ConfigValues) (finalErr error) {
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

	if err := m.install(ctx, installableCharts, kotsConfigValues); err != nil {
		return err
	}

	return nil
}

func (m *appInstallManager) install(ctx context.Context, installableCharts []types.InstallableHelmChart, kotsConfigValues kotsv1beta1.ConfigValues) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	// Setup Helm client
	if err := m.setupHelmClient(); err != nil {
		return fmt.Errorf("setup helm client: %w", err)
	}

	// Install Helm charts
	if err := m.installHelmCharts(ctx, installableCharts); err != nil {
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

	configValuesFile, err := m.createConfigValuesFile(kotsConfigValues)
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
func (m *appInstallManager) createConfigValuesFile(kotsConfigValues kotsv1beta1.ConfigValues) (string, error) {
	// Use Kubernetes-specific YAML serialization to properly handle TypeMeta and ObjectMeta
	data, err := kyaml.Marshal(kotsConfigValues)
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

func (m *appInstallManager) installHelmCharts(ctx context.Context, installableCharts []types.InstallableHelmChart) error {
	logFn := m.logFn("app-helm")

	if len(installableCharts) == 0 {
		return fmt.Errorf("no helm charts found")
	}

	logFn("installing %d helm charts", len(installableCharts))

	for _, installableChart := range installableCharts {
		logFn("installing %s helm chart", installableChart.CR.GetChartName())

		if err := m.installHelmChart(ctx, installableChart); err != nil {
			return fmt.Errorf("install %s helm chart: %w", installableChart.CR.GetChartName(), err)
		}

		logFn("successfully installed %s helm chart", installableChart.CR.GetChartName())
	}

	return nil
}

func (m *appInstallManager) installHelmChart(ctx context.Context, installableChart types.InstallableHelmChart) error {
	// Write chart archive to temp file
	chartPath, err := m.writeChartArchiveToTemp(installableChart.Archive)
	if err != nil {
		return fmt.Errorf("write chart archive to temp: %w", err)
	}
	defer os.Remove(chartPath)

	// Fallback to admin console namespace if namespace is not set
	namespace := installableChart.CR.GetNamespace()
	if namespace == "" {
		namespace = constants.KotsadmNamespace
	}

	// Install chart using Helm client with pre-processed values
	_, err = m.hcli.Install(ctx, helm.InstallOptions{
		ChartPath:   chartPath,
		Namespace:   namespace,
		ReleaseName: installableChart.CR.GetReleaseName(),
		Values:      installableChart.Values,
	})
	if err != nil {
		return fmt.Errorf("helm install: %w", err)
	}

	return nil
}
