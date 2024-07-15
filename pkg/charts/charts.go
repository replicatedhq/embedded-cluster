package charts

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/k0sproject/dig"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/ohler55/ojg/jp"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/registry"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/util"
)

const (
	DefaultVendorChartOrder = 10
)

// K0sHelmExtensionsFromInstallation returns the HelmExtensions object for the given installation,
// merging in the default charts and repositories from the release metadata with the user-provided
// charts and repositories from the installation spec.
func K0sHelmExtensionsFromInstallation(
	ctx context.Context, in *clusterv1beta1.Installation,
	metadata *ectypes.ReleaseMetadata, clusterConfig *k0sv1beta1.ClusterConfig,
) (*v1beta1.Helm, error) {
	combinedConfigs, err := mergeHelmConfigs(ctx, metadata, in, clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("merge helm configs: %w", err)
	}

	if in.Spec.AirGap {
		// if in airgap mode then all charts are already on the node's disk. we just need to
		// make sure that the helm charts are pointing to the right location on disk and that
		// we do not have any kind of helm repository configuration.
		combinedConfigs = patchExtensionsForAirGap(combinedConfigs)
	}

	combinedConfigs, err = applyUserProvidedAddonOverrides(in, combinedConfigs)
	if err != nil {
		return nil, fmt.Errorf("apply user provided overrides: %w", err)
	}

	return combinedConfigs, nil
}

// merge the default helm charts and repositories (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
func mergeHelmConfigs(ctx context.Context, meta *ectypes.ReleaseMetadata, in *clusterv1beta1.Installation, clusterConfig *k0sv1beta1.ClusterConfig) (*v1beta1.Helm, error) {
	// merge default helm charts (from meta.Configs) with vendor helm charts (from in.Spec.Config.Extensions.Helm)
	combinedConfigs := &v1beta1.Helm{ConcurrencyLevel: 1}
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
				combinedConfigs.Charts[k].Order = DefaultVendorChartOrder
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
	var err error
	combinedConfigs.Charts, err = updateInfraChartsFromInstall(in, clusterConfig, combinedConfigs.Charts)
	if err != nil {
		return nil, fmt.Errorf("update infrastructure charts from install: %w", err)
	}

	// k0s sorts order numbers alphabetically because they're used in file names,
	// which means double digits can be sorted before single digits (e.g. "10" comes before "5").
	// We add 100 to the order of each chart to work around this.
	for k := range combinedConfigs.Charts {
		combinedConfigs.Charts[k].Order += 100
	}
	return combinedConfigs, nil
}

// updateInfraChartsFromInstall updates the infrastructure charts with dynamic values from the installation spec
func updateInfraChartsFromInstall(in *v1beta1.Installation, clusterConfig *k0sv1beta1.ClusterConfig, charts []v1beta1.Chart) ([]v1beta1.Chart, error) {
	for i, chart := range charts {
		if chart.Name == "admin-console" {
			// admin-console has "embeddedClusterID" and "isAirgap" as dynamic values
			newVals, err := setHelmValue(chart.Values, "embeddedClusterID", in.Spec.ClusterID)
			if err != nil {
				return nil, fmt.Errorf("set helm values admin-console.embeddedClusterID: %w", err)
			}

			newVals, err = setHelmValue(newVals, "isAirgap", fmt.Sprintf("%t", in.Spec.AirGap))
			if err != nil {
				return nil, fmt.Errorf("set helm values admin-console.isAirgap: %w", err)
			}

			newVals, err = setHelmValue(newVals, "isHA", in.Spec.HighAvailability)
			if err != nil {
				return nil, fmt.Errorf("set helm values admin-console.isHA: %w", err)
			}

			if in.Spec.Proxy != nil {
				extraEnv := getExtraEnvFromProxy(in.Spec.Proxy.HTTPProxy, in.Spec.Proxy.HTTPSProxy, in.Spec.Proxy.NoProxy)
				newVals, err = setHelmValue(newVals, "extraEnv", extraEnv)
				if err != nil {
					return nil, fmt.Errorf("set helm values admin-console.extraEnv: %w", err)
				}
			}

			charts[i].Values = newVals
		}
		if chart.Name == "embedded-cluster-operator" {
			// embedded-cluster-operator has "embeddedBinaryName" and "embeddedClusterID" as dynamic values
			newVals, err := setHelmValue(chart.Values, "embeddedBinaryName", in.Spec.BinaryName)
			if err != nil {
				return nil, fmt.Errorf("set helm values embedded-cluster-operator.embeddedBinaryName: %w", err)
			}

			newVals, err = setHelmValue(newVals, "embeddedClusterID", in.Spec.ClusterID)
			if err != nil {
				return nil, fmt.Errorf("set helm values embedded-cluster-operator.embeddedClusterID: %w", err)
			}

			if in.Spec.Proxy != nil {
				extraEnv := getExtraEnvFromProxy(in.Spec.Proxy.HTTPProxy, in.Spec.Proxy.HTTPSProxy, in.Spec.Proxy.NoProxy)
				newVals, err = setHelmValue(newVals, "extraEnv", extraEnv)
				if err != nil {
					return nil, fmt.Errorf("set helm values embedded-cluster-operator.extraEnv: %w", err)
				}
			}

			charts[i].Values = newVals
		}
		if chart.Name == "docker-registry" {
			if !in.Spec.AirGap {
				continue
			}

			// handle the registry IP, which will always be present in airgap
			serviceCIDR := util.ClusterServiceCIDR(*clusterConfig, in)
			registryEndpoint, err := registry.GetRegistryServiceIP(serviceCIDR)
			if err != nil {
				return nil, fmt.Errorf("get registry service IP: %w", err)
			}

			newVals, err := setHelmValue(chart.Values, "service.clusterIP", registryEndpoint)
			if err != nil {
				return nil, fmt.Errorf("set helm values docker-registry.service.clusterIP: %w", err)
			}
			charts[i].Values = newVals

			if !in.Spec.HighAvailability {
				continue
			}

			// handle the seaweedFS endpoint, which will only be present in HA airgap
			seaweedfsS3Endpoint, err := registry.GetSeaweedfsS3Endpoint(serviceCIDR)
			if err != nil {
				return nil, fmt.Errorf("get seaweedfs s3 endpoint: %w", err)
			}

			newVals, err = setHelmValue(newVals, "s3.regionEndpoint", seaweedfsS3Endpoint)
			if err != nil {
				return nil, fmt.Errorf("set helm values docker-registry.s3.regionEndpoint: %w", err)
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
					return nil, fmt.Errorf("set helm values velero.configuration: %w", err)
				}
				charts[i].Values = newVals
			}
		}
	}
	return charts, nil
}

// applyUserProvidedAddonOverrides applies user-provided overrides to the HelmExtensions spec.
func applyUserProvidedAddonOverrides(in *clusterv1beta1.Installation, combinedConfigs *v1beta1.Helm) (*v1beta1.Helm, error) {
	if in == nil || in.Spec.Config == nil {
		return combinedConfigs, nil
	}
	patchedConfigs := combinedConfigs.DeepCopy()
	patchedConfigs.Charts = []v1beta1.Chart{}
	for _, chart := range combinedConfigs.Charts {
		newValues, err := in.Spec.Config.ApplyEndUserAddOnOverrides(chart.Name, chart.Values)
		if err != nil {
			return nil, fmt.Errorf("apply end user overrides for chart %s: %w", chart.Name, err)
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
func patchExtensionsForAirGap(config *v1beta1.Helm) *v1beta1.Helm {
	config.Repositories = nil
	for idx, chart := range config.Charts {
		chartName := fmt.Sprintf("%s-%s.tgz", chart.Name, chart.Version)
		chartPath := filepath.Join("var", "lib", "embedded-cluster", "charts", chartName)
		config.Charts[idx].ChartName = chartPath
	}
	return config
}

func setHelmValue(valuesYaml string, path string, newValue interface{}) (string, error) {
	newValuesMap := dig.Mapping{}
	if err := yaml.Unmarshal([]byte(valuesYaml), &newValuesMap); err != nil {
		return "", fmt.Errorf("unmarshal initial values: %w", err)
	}

	x, err := jp.ParseString(path)
	if err != nil {
		return "", fmt.Errorf("parse json path %q: %w", path, err)
	}

	err = x.Set(newValuesMap, newValue)
	if err != nil {
		return "", fmt.Errorf("set json path %q to %q: %w", path, newValue, err)
	}

	newValuesYaml, err := yaml.Marshal(newValuesMap)
	if err != nil {
		return "", fmt.Errorf("marshal updated values: %w", err)
	}
	return string(newValuesYaml), nil
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
