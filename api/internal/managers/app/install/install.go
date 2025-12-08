package install

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

// Install installs the app with the provided Helm charts
func (m *appInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart, configValues types.AppConfigValues, registrySettings *types.RegistrySettings, hostCABundlePath string) error {
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

	// Create or update secret with config values before installing
	if err := m.createConfigValuesSecret(ctx, configValues, kotsadmNamespace); err != nil {
		return fmt.Errorf("creating config values secret: %w", err)
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

// createConfigValuesSecret creates or updates a Kubernetes secret with the config values.
// TODO: Handle 1MB size limitation by storing large file data fields as pointers to other secrets
// TODO: Consider maintaining history of config values for potential rollbacks
func (m *appInstallManager) createConfigValuesSecret(ctx context.Context, configValues types.AppConfigValues, namespace string) error {
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

func (m *appInstallManager) installHelmCharts(ctx context.Context, installableCharts []types.InstallableHelmChart, kotsadmNamespace string) error {
	logFn := m.logFn("app")

	if len(installableCharts) == 0 {
		logFn("no helm charts to install")
		return nil
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
