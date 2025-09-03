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
	if err := m.initializeComponents(installableCharts); err != nil {
		return fmt.Errorf("initialize components: %w", err)
	}

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
	if err := m.installKots(kotsConfigValues); err != nil {
		return fmt.Errorf("install kots: %w", err)
	}

	// Install Helm charts
	if err := m.installHelmCharts(ctx, installableCharts); err != nil {
		return fmt.Errorf("install helm charts: %w", err)
	}

	return nil
}

func (m *appInstallManager) installKots(kotsConfigValues kotsv1beta1.ConfigValues) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
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

	logFn := m.logFn("app")

	logFn("preparing the app for installation")

	if m.kotsCLI != nil {
		err := m.kotsCLI.Install(installOpts)
		if err != nil {
			return fmt.Errorf("install kots: %w", err)
		}
	} else {
		err := kotscli.Install(installOpts)
		if err != nil {
			return fmt.Errorf("install kots: %w", err)
		}
	}

	logFn("successfully prepared the app for installation")

	return nil
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
	logFn := m.logFn("app")

	if len(installableCharts) == 0 {
		return fmt.Errorf("no helm charts found")
	}

	logFn("installing %d helm charts", len(installableCharts))

	for _, installableChart := range installableCharts {
		chartName := getChartDisplayName(installableChart)
		logFn("installing %s chart", chartName)

		if err := m.installHelmChart(ctx, installableChart); err != nil {
			return fmt.Errorf("install %s helm chart: %w", chartName, err)
		}

		logFn("successfully installed %s chart", chartName)
	}

	logFn("successfully installed all %d helm charts", len(installableCharts))

	return nil
}

func (m *appInstallManager) installHelmChart(ctx context.Context, installableChart types.InstallableHelmChart) (finalErr error) {
	chartName := getChartDisplayName(installableChart)

	if err := m.setComponentStatus(chartName, types.StateRunning, "Installing"); err != nil {
		return fmt.Errorf("set component status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("recovered from panic: %v: %s", r, string(debug.Stack()))
		}

		if finalErr != nil {
			if err := m.setComponentStatus(chartName, types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Errorf("failed to set %s chart failed status", chartName)
			}
		} else {
			if err := m.setComponentStatus(chartName, types.StateSucceeded, ""); err != nil {
				m.logger.WithError(err).Errorf("failed to set %s chart succeeded status", chartName)
			}
		}
	}()

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
		LogFn:       m.logFn("helm"),
	})
	if err != nil {
		return err // do not wrap as wrapping is repetitive, e.g. "helm install: helm install: context deadline exceeded"
	}

	return nil
}

// initializeComponents initializes the component tracking with chart names
func (m *appInstallManager) initializeComponents(charts []types.InstallableHelmChart) error {
	chartNames := make([]string, 0, len(charts))
	for _, chart := range charts {
		chartNames = append(chartNames, getChartDisplayName(chart))
	}

	return m.appInstallStore.RegisterComponents(chartNames)
}

// getChartDisplayName returns the name of the chart for display purposes. It prefers the
// metadata.name field if available and falls back to the chart name.
func getChartDisplayName(chart types.InstallableHelmChart) string {
	chartName := chart.CR.GetName()
	if chartName == "" {
		chartName = chart.CR.GetChartName()
	}
	return chartName
}
