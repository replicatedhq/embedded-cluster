package install

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided config values
func (m *appInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart, configValues kotsv1beta1.ConfigValues) error {
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

	// Initialize components for tracking
	if err := m.initializeComponents(installableCharts); err != nil {
		return fmt.Errorf("initialize components: %w", err)
	}

	// Install Helm charts first
	if err := m.installHelmCharts(ctx, installableCharts, kotsadmNamespace); err != nil {
		return fmt.Errorf("install helm charts: %w", err)
	}

	// Then install the app using KOTS CLI
	if err := m.installWithKotsCLI(license, kotsadmNamespace, configValues); err != nil {
		return fmt.Errorf("install with kots cli: %w", err)
	}

	return nil
}

func (m *appInstallManager) installWithKotsCLI(license *kotsv1beta1.License, kotsadmNamespace string, configValues kotsv1beta1.ConfigValues) error {
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

func (m *appInstallManager) installHelmCharts(ctx context.Context, installableCharts []types.InstallableHelmChart, kotsadmNamespace string) error {
	logFn := m.logFn("app")

	if len(installableCharts) == 0 {
		return fmt.Errorf("no helm charts found")
	}

	logFn("installing %d helm charts", len(installableCharts))

	for _, installableChart := range installableCharts {
		chartName := getChartDisplayName(installableChart)
		logFn("installing %s chart", chartName)

		if err := m.installHelmChart(ctx, installableChart, kotsadmNamespace); err != nil {
			return fmt.Errorf("install %s helm chart: %w", chartName, err)
		}

		logFn("successfully installed %s chart", chartName)
	}

	logFn("successfully installed all %d helm charts", len(installableCharts))

	return nil
}

func (m *appInstallManager) installHelmChart(ctx context.Context, installableChart types.InstallableHelmChart, kotsadmNamespace string) (finalErr error) {
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
		namespace = kotsadmNamespace
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
