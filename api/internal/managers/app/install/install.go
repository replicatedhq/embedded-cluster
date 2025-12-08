package install

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// Install installs the app with the provided Helm charts
func (m *appInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart, registrySettings *types.RegistrySettings, hostCABundlePath string) error {
	if err := m.setupClients(); err != nil {
		return fmt.Errorf("setup clients: %w", err)
	}

	// Start the namespace reconciler to ensure image pull secrets and other required resources in app namespaces
	nsReconciler, err := runNamespaceReconciler(ctx, m.kcli, m.mcli, registrySettings, hostCABundlePath, m.logger)
	if err != nil {
		return fmt.Errorf("start namespace reconciler: %w", err)
	}
	defer nsReconciler.Stop()

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	// Initialize components for tracking
	if err := m.initializeComponents(installableCharts); err != nil {
		return fmt.Errorf("initialize components: %w", err)
	}

	// Install Helm charts
	if err := m.installHelmCharts(ctx, installableCharts, kotsadmNamespace); err != nil {
		return fmt.Errorf("install helm charts: %w", err)
	}

	return nil
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
