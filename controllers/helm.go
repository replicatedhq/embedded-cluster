package controllers

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/k0sproject/dig"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/ohler55/ojg/jp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/registry"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
)

const DEFAULT_VENDOR_CHART_ORDER = 10

func setHelmValue(valuesYaml string, path string, newValue interface{}) (string, error) {
	newValuesMap := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(valuesYaml), &newValuesMap); err != nil {
		return "", fmt.Errorf("failed to unmarshal initial values: %w", err)
	}

	x, err := jp.ParseString(path)
	if err != nil {
		return "", fmt.Errorf("failed to parse json path %q: %w", path, err)
	}

	err = x.Set(newValuesMap, newValue)
	if err != nil {
		return "", fmt.Errorf("failed to set json path %q to %q: %w", path, newValue, err)
	}

	newValuesYaml, err := yaml.Marshal(newValuesMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal updated values: %w", err)
	}
	return string(newValuesYaml), nil
}

// merge the default helm charts and repositories (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
func mergeHelmConfigs(ctx context.Context, meta *ectypes.ReleaseMetadata, in *v1beta1.Installation, clusterConfig k0sv1beta1.ClusterConfig) *k0sv1beta1.HelmExtensions {
	// merge default helm charts (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
	combinedConfigs := &k0sv1beta1.HelmExtensions{ConcurrencyLevel: 1}
	if meta != nil {
		combinedConfigs.Charts = meta.Configs.Charts
		combinedConfigs.Repositories = meta.Configs.Repositories
	}
	if in != nil && in.Spec.Config != nil && in.Spec.Config.Extensions.Helm != nil {
		// set the concurrency level to the minimum of our default and the user provided value
		if in.Spec.Config.Extensions.Helm.ConcurrencyLevel > 0 {
			combinedConfigs.ConcurrencyLevel = min(in.Spec.Config.Extensions.Helm.ConcurrencyLevel, combinedConfigs.ConcurrencyLevel)
		}

		// append the user provided charts to the default charts
		combinedConfigs.Charts = append(combinedConfigs.Charts, in.Spec.Config.Extensions.Helm.Charts...)
		for k := range combinedConfigs.Charts {
			if combinedConfigs.Charts[k].Order == 0 {
				combinedConfigs.Charts[k].Order = DEFAULT_VENDOR_CHART_ORDER
			}
		}

		// append the user provided repositories to the default repositories
		combinedConfigs.Repositories = append(combinedConfigs.Repositories, in.Spec.Config.Extensions.Helm.Repositories...)
	}

	if in != nil && in.Spec.AirGap {
		if in.Spec.HighAvailability {
			seaweedfsConfig, ok := meta.BuiltinConfigs["seaweedfs"]
			if ok {
				combinedConfigs.Charts = append(combinedConfigs.Charts, seaweedfsConfig.Charts...)
				combinedConfigs.Repositories = append(combinedConfigs.Repositories, seaweedfsConfig.Repositories...)
			}

			migrationStatus := k8sutil.CheckConditionStatus(in.Status, registry.RegistryMigrationStatusConditionType)
			if migrationStatus == metav1.ConditionTrue {
				registryConfig, ok := meta.BuiltinConfigs["registry-ha"]
				if ok {
					combinedConfigs.Charts = append(combinedConfigs.Charts, registryConfig.Charts...)
					combinedConfigs.Repositories = append(combinedConfigs.Repositories, registryConfig.Repositories...)
				}
			}
		} else {
			registryConfig, ok := meta.BuiltinConfigs["registry"]
			if ok {
				combinedConfigs.Charts = append(combinedConfigs.Charts, registryConfig.Charts...)
				combinedConfigs.Repositories = append(combinedConfigs.Repositories, registryConfig.Repositories...)
			}
		}
	}

	if in != nil && in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsDisasterRecoverySupported {
		config, ok := meta.BuiltinConfigs["velero"]
		if ok {
			combinedConfigs.Charts = append(combinedConfigs.Charts, config.Charts...)
			combinedConfigs.Repositories = append(combinedConfigs.Repositories, config.Repositories...)
		}
	}

	// update the infrastructure charts from the install spec
	combinedConfigs.Charts = updateInfraChartsFromInstall(ctx, in, clusterConfig, combinedConfigs.Charts)

	// k0s sorts order numbers alphabetically because they're used in file names,
	// which means double digits can be sorted before single digits (e.g. "10" comes before "5").
	// We add 100 to the order of each chart to work around this.
	for k := range combinedConfigs.Charts {
		combinedConfigs.Charts[k].Order += 100
	}
	return combinedConfigs
}

// update the 'admin-console' and 'embedded-cluster-operator' charts to add cluster ID, binary name, airgap status, and HA status
func updateInfraChartsFromInstall(ctx context.Context, in *v1beta1.Installation, clusterConfig k0sv1beta1.ClusterConfig, charts k0sv1beta1.ChartsSettings) k0sv1beta1.ChartsSettings {
	log := ctrl.LoggerFrom(ctx)

	if in == nil {
		return charts
	}
	serviceCIDR := util.ClusterServiceCIDR(clusterConfig, in)

	for i, chart := range charts {
		if chart.Name == "admin-console" {
			// admin-console has "embeddedClusterID" and "isAirgap" as dynamic values
			newVals, err := setHelmValue(chart.Values, "embeddedClusterID", in.Spec.ClusterID)
			if err != nil {
				log.Error(err, "failed to set helm values embeddedClusterID", "chart", chart.Name)
				continue
			}

			newVals, err = setHelmValue(newVals, "isAirgap", fmt.Sprintf("%t", in.Spec.AirGap))
			if err != nil {
				log.Error(err, "failed to set helm values isAirgap", "chart", chart.Name)
				continue
			}

			newVals, err = setHelmValue(newVals, "isHA", in.Spec.HighAvailability)
			if err != nil {
				log.Error(err, "failed to set helm values isHA", "chart", chart.Name)
				continue
			}

			if in.Spec.Proxy != nil {
				extraEnv := getExtraEnvFromProxy(in.Spec.Proxy.HTTPProxy, in.Spec.Proxy.HTTPSProxy, in.Spec.Proxy.NoProxy)
				newVals, err = setHelmValue(newVals, "extraEnv", extraEnv)
				if err != nil {
					log.Error(err, "failed to set helm values extraEnv", "chart", chart.Name)
					continue
				}
			}

			charts[i].Values = newVals
		}
		if chart.Name == "embedded-cluster-operator" {
			// embedded-cluster-operator has "embeddedBinaryName" and "embeddedClusterID" as dynamic values
			newVals, err := setHelmValue(chart.Values, "embeddedBinaryName", in.Spec.BinaryName)
			if err != nil {
				log.Error(err, "failed to set helm values embeddedBinaryName", "chart", chart.Name)
				continue
			}

			newVals, err = setHelmValue(newVals, "embeddedClusterID", in.Spec.ClusterID)
			if err != nil {
				log.Error(err, "failed to set helm values embeddedClusterID", "chart", chart.Name)
				continue
			}

			if in.Spec.Proxy != nil {
				extraEnv := getExtraEnvFromProxy(in.Spec.Proxy.HTTPProxy, in.Spec.Proxy.HTTPSProxy, in.Spec.Proxy.NoProxy)
				newVals, err = setHelmValue(newVals, "extraEnv", extraEnv)
				if err != nil {
					log.Error(err, "failed to set helm values extraEnv", "chart", chart.Name)
					continue
				}
			}

			charts[i].Values = newVals
		}
		if chart.Name == "docker-registry" {
			if !in.Spec.AirGap {
				continue
			}

			// handle the registry IP, which will always be present in airgap
			registryEndpoint, err := registry.GetRegistryServiceIP(serviceCIDR)
			if err != nil {
				log.Error(err, "failed to get registry endpoint", "chart", chart.Name)
				continue
			}

			newVals, err := setHelmValue(chart.Values, "service.clusterIP", registryEndpoint)
			if err != nil {
				log.Error(err, "failed to set helm values service.clusterIP", "chart", chart.Name)
			}
			charts[i].Values = newVals

			if !in.Spec.HighAvailability {
				continue
			}

			// handle the seaweedFS endpoint, which will only be present in HA airgap
			seaweedfsS3Endpoint, err := registry.GetSeaweedfsS3Endpoint(serviceCIDR)
			if err != nil {
				log.Error(err, "failed to get seaweedfs s3 endpoint", "chart", chart.Name)
				continue
			}

			newVals, err = setHelmValue(newVals, "s3.regionEndpoint", seaweedfsS3Endpoint)
			if err != nil {
				log.Error(err, "failed to set helm values s3.regionEndpoint", "chart", chart.Name)
				continue
			}

			charts[i].Values = newVals
		}
		if chart.Name == "velero" {
			if in.Spec.Proxy != nil {
				extraEnvVars := map[string]interface{}{
					"extraEnvVars": map[string]string{
						"HTTP_PROXY":  in.Spec.Proxy.HTTPProxy,
						"HTTPS_PROXY": in.Spec.Proxy.HTTPSProxy,
						"NO_PROXY":    in.Spec.Proxy.NoProxy,
					},
				}

				newVals, err := setHelmValue(chart.Values, "configuration", extraEnvVars)
				if err != nil {
					log.Error(err, "failed to set helm values extraEnvVars", "chart", chart.Name)
					continue
				}
				charts[i].Values = newVals
			}
		}
	}
	return charts
}

func getExtraEnvFromProxy(httpProxy string, httpsProxy string, noProxy string) []map[string]interface{} {
	extraEnv := []map[string]interface{}{}
	extraEnv = append(extraEnv, map[string]interface{}{
		"name":  "HTTP_PROXY",
		"value": httpProxy,
	})
	extraEnv = append(extraEnv, map[string]interface{}{
		"name":  "HTTPS_PROXY",
		"value": httpsProxy,
	})
	extraEnv = append(extraEnv, map[string]interface{}{
		"name":  "NO_PROXY",
		"value": noProxy,
	})
	return extraEnv
}

// detect if the charts currently installed in the cluster (currentConfigs) match the desired charts (combinedConfigs)
func detectChartDrift(combinedConfigs, currentConfigs *k0sv1beta1.HelmExtensions) (bool, []string, error) {
	chartDrift := false
	driftMap := map[string]struct{}{}
	if len(currentConfigs.Repositories) != len(combinedConfigs.Repositories) {
		chartDrift = true
		driftMap["repositories"] = struct{}{}
	}

	targetCharts := combinedConfigs.Charts
	// grab the desired charts
	for _, targetChart := range targetCharts {
		// check for version and values drift between installed charts and desired charts
		chartSeen := false
		for _, chart := range currentConfigs.Charts {
			if targetChart.Name != chart.Name {
				continue
			}
			chartSeen = true

			if targetChart.Version != chart.Version {
				chartDrift = true
				driftMap[chart.Name] = struct{}{}
			}

			valuesDiff, err := yamlDiff(targetChart.Values, chart.Values)
			if err != nil {
				return false, nil, fmt.Errorf("failed to compare values of chart %s: %w", chart.Name, err)
			}
			if valuesDiff {
				chartDrift = true
				driftMap[chart.Name] = struct{}{}
			}
		}
		if !chartSeen { // if this chart in the spec is not in the cluster, there is drift
			chartDrift = true
			driftMap[targetChart.Name] = struct{}{}
		}
	}

	// flatten map to []string
	driftSlice := []string{}
	for k := range driftMap {
		driftSlice = append(driftSlice, k)
	}

	return chartDrift, driftSlice, nil
}

// yamlDiff compares two yaml strings and returns true if they are different
func yamlDiff(a, b string) (bool, error) {
	aMap := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(a), &aMap)
	if err != nil {
		return false, fmt.Errorf("yaml A values error: %w", err)
	}

	bMap := map[string]interface{}{}
	err = yaml.Unmarshal([]byte(b), &bMap)
	if err != nil {
		return false, fmt.Errorf("yaml B values error: %w", err)
	}

	aYaml, err := yaml.Marshal(aMap)
	if err != nil {
		return false, fmt.Errorf("yaml A marshal error: %w", err)
	}

	bYaml, err := yaml.Marshal(bMap)
	if err != nil {
		return false, fmt.Errorf("yaml B marshal error: %w", err)
	}

	return string(aYaml) != string(bYaml), nil
}

// check if all charts in the combinedConfigs are installed successfully with the desired version and values
func detectChartCompletion(combinedConfigs *k0sv1beta1.HelmExtensions, installedCharts k0shelm.ChartList) ([]string, []string, error) {
	incompleteCharts := []string{}
	chartErrors := []string{}
	if combinedConfigs == nil {
		return incompleteCharts, chartErrors, nil
	}
	for _, chart := range combinedConfigs.Charts {
		diffDetected := false
		chartSeen := false
		for _, installedChart := range installedCharts.Items {
			if chart.Name == installedChart.Spec.ReleaseName {
				chartSeen = true

				valuesDiff, err := yamlDiff(chart.Values, installedChart.Spec.Values)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to compare values of chart %s: %w", chart.Name, err)
				}
				if valuesDiff {
					diffDetected = true
				}

				// if the spec's HashValues does not match the status's ValuesHash, the chart is currently being applied with the new values
				if installedChart.Spec.HashValues() != installedChart.Status.ValuesHash {
					diffDetected = true
				}

				if installedChart.Status.Version != chart.Version {
					diffDetected = true
				}

				if installedChart.Status.Error != "" {
					chartErrors = append(chartErrors, installedChart.Status.Error)
					diffDetected = false
				}

				break
			}
		}
		if !chartSeen || diffDetected {
			incompleteCharts = append(incompleteCharts, chart.Name)
		}
	}

	return incompleteCharts, chartErrors, nil
}

// applyUserProvidedAddonOverrides applies user-provided overrides to the HelmExtensions spec.
func applyUserProvidedAddonOverrides(in *v1beta1.Installation, combinedConfigs *k0sv1beta1.HelmExtensions) (*k0sv1beta1.HelmExtensions, error) {
	if in == nil || in.Spec.Config == nil {
		return combinedConfigs, nil
	}
	patchedConfigs := combinedConfigs.DeepCopy()
	patchedConfigs.Charts = k0sv1beta1.ChartsSettings{}
	for _, chart := range combinedConfigs.Charts {
		newValues, err := in.Spec.Config.ApplyEndUserAddOnOverrides(chart.Name, chart.Values)
		if err != nil {
			return nil, fmt.Errorf("failed to apply end user overrides for chart %s: %w", chart.Name, err)
		}
		chart.Values = newValues
		patchedConfigs.Charts = append(patchedConfigs.Charts, chart)
	}
	return patchedConfigs, nil
}

// patchExtensionsForAirGap makes sure we do not have any external repository reference and also makes
// sure that all helm charts point to a chart stored on disk as a tgz file. These files are already
// expected to be present on the disk and, during an upgrade, are laid down on disk by the artifact
// copy job.
func patchExtensionsForAirGap(config *k0sv1beta1.HelmExtensions) *k0sv1beta1.HelmExtensions {
	config.Repositories = nil
	for idx, chart := range config.Charts {
		chartName := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
		chartPath := filepath.Join("var", "lib", "embedded-cluster", "charts", chartName)
		config.Charts[idx].ChartName = chartPath
	}
	return config
}
