// Package config handles the cluster configuration file generation.
package config

import (
	"fmt"
	"os"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/k0sproject/dig"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/charts"
	"gopkg.in/yaml.v2"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

// ReadConfigFile reads the cluster configuration from the provided file.
func ReadConfigFile(cfgPath string) (dig.Mapping, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read current config: %w", err)
	}
	cfg := dig.Mapping{}
	if err := k8syaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal current config: %w", err)
	}
	return cfg, nil
}

// RenderK0sConfig renders a k0s cluster configuration.
func RenderK0sConfig() *k0sconfig.ClusterConfig {
	cfg := k0sconfig.DefaultClusterConfig()
	// Customize the default k0s configuration to our taste.
	cfg.Name = defaults.BinaryName()
	cfg.Spec.Konnectivity = nil
	cfg.Spec.Network.KubeRouter = nil
	cfg.Spec.Network.Provider = "calico"
	cfg.Spec.Telemetry.Enabled = false
	return cfg
}

// UpdateHelmConfigs updates the helm config in the provided cluster configuration.
func UpdateHelmConfigs(cfg *k0sconfig.ClusterConfig, opts ...addons.Option) error {
	applier := addons.NewApplier(opts...)
	chtconfig, repconfig, err := applier.GenerateHelmConfigs(
		AdditionalCharts(), AdditionalRepositories(),
	)
	if err != nil {
		return fmt.Errorf("unable to apply addons: %w", err)
	}
	return updateHelmConfigs(cfg, chtconfig, repconfig)
}

// UpdateHelmConfigsForRestore updates the helm config in the provided cluster configuration for a restore operation.
func UpdateHelmConfigsForRestore(cfg *k0sconfig.ClusterConfig, opts ...addons.Option) error {
	applier := addons.NewApplier(opts...)
	chtconfig, repconfig, err := applier.GenerateHelmConfigsForRestore()
	if err != nil {
		return fmt.Errorf("unable to apply addons: %w", err)
	}
	return updateHelmConfigs(cfg, chtconfig, repconfig)
}

func updateHelmConfigs(cfg *k0sconfig.ClusterConfig, chtconfig []embeddedclusterv1beta1.Chart, repconfig []embeddedclusterv1beta1.Repository) error {
	// k0s sorts order numbers alphabetically because they're used in file names,
	// which means double digits can be sorted before single digits (e.g. "10" comes before "5").
	// We add 100 to the order of each chart to work around this.
	for k := range chtconfig {
		chtconfig[k].Order += 100
	}

	helm := embeddedclusterv1beta1.Helm{
		Charts:       chtconfig,
		Repositories: repconfig,
	}

	convertedHelm := &k0sconfig.HelmExtensions{}
	var err error
	convertedHelm, err = embeddedclusterv1beta1.ConvertTo(helm, convertedHelm)
	if err != nil {
		return err
	}

	cfg.Spec.Extensions = &k0sconfig.ClusterExtensions{
		Helm: convertedHelm,
	}
	return nil
}

// extractK0sConfigPatch extracts the k0s config portion of the provided patch.
func extractK0sConfigPatch(raw string) (string, error) {
	type PatchBody struct {
		Config map[string]interface{} `yaml:"config"`
	}
	var body PatchBody
	if err := yaml.Unmarshal([]byte(raw), &body); err != nil {
		return "", fmt.Errorf("unable to unmarshal patch body: %w", err)
	}
	data, err := yaml.Marshal(body.Config)
	if err != nil {
		return "", fmt.Errorf("unable to marshal patch body: %w", err)
	}
	return string(data), nil
}

// ApplyBuiltIndExtensionsOverrides applies the cluster config built in extensions overrides on top
// of the provided cluster configuration. Returns the changed configuration.
func ApplyBuiltInExtensionsOverrides(cfg *k0sconfig.ClusterConfig, releaseConfig *embeddedclusterv1beta1.Config) (*k0sconfig.ClusterConfig, error) {
	if cfg.Spec == nil || cfg.Spec.Extensions == nil || cfg.Spec.Extensions.Helm == nil {
		return cfg, nil
	}
	for i, chart := range cfg.Spec.Extensions.Helm.Charts {
		values, err := releaseConfig.Spec.ApplyEndUserAddOnOverrides(chart.Name, chart.Values)
		if err != nil {
			return nil, fmt.Errorf("unable to apply end user overrides for %s: %w", chart.Name, err)
		}
		cfg.Spec.Extensions.Helm.Charts[i].Values = values
	}
	return cfg, nil
}

// PatchK0sConfig patches a K0s config with the provided patch. Returns the patched config,
// patch is expected to be a YAML encoded k0s configuration. We marshal the original config
// and the patch into JSON and apply the latter as a merge patch to the former.
func PatchK0sConfig(config *k0sconfig.ClusterConfig, patch string) (*k0sconfig.ClusterConfig, error) {
	if patch == "" {
		return config, nil
	}
	patch, err := extractK0sConfigPatch(patch)
	if err != nil {
		return nil, fmt.Errorf("unable to extract k0s config patch: %w", err)
	}
	originalYAML, err := k8syaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal original config: %w", err)
	}
	originalJSON, err := k8syaml.YAMLToJSON(originalYAML)
	if err != nil {
		return nil, fmt.Errorf("unable to convert original config to json: %w", err)
	}
	patchJSON, err := k8syaml.YAMLToJSON([]byte(patch))
	if err != nil {
		return nil, fmt.Errorf("unable to convert patch to json: %w", err)
	}
	result, err := jsonpatch.MergePatch(originalJSON, patchJSON)
	if err != nil {
		return nil, fmt.Errorf("unable to patch configuration: %w", err)
	}
	resultYAML, err := k8syaml.JSONToYAML(result)
	if err != nil {
		return nil, fmt.Errorf("unable to convert patched config to json: %w", err)
	}
	var patched k0sconfig.ClusterConfig
	if err := k8syaml.Unmarshal(resultYAML, &patched); err != nil {
		return nil, fmt.Errorf("unable to unmarshal patched config: %w", err)
	}
	return &patched, nil
}

// InstallFlags returns a list of default flags to be used when bootstrapping a k0s cluster.
func InstallFlags() []string {
	return []string{
		"install",
		"controller",
		"--disable-components", "konnectivity-server",
		"--labels", strings.Join(nodeLabels(), ","),
		"--enable-worker",
		"--no-taints",
		"--enable-dynamic-config",
		"-c", defaults.PathToK0sConfig(),
	}
}

func ControllerLabels() map[string]string {
	lmap := additionalControllerLabels()
	lmap["kots.io/embedded-cluster-role-0"] = getControllerRoleName()
	lmap["kots.io/embedded-cluster-role"] = "total-1"
	return lmap
}

// nodeLabels return a slice of string with labels (key=value format) for the node where we
// are installing the k0s.
func nodeLabels() []string {
	labels := []string{}
	for k, v := range ControllerLabels() {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return labels
}

func getControllerRoleName() string {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	controllerRoleName := "controller"
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Roles.Controller.Name != "" {
				controllerRoleName = clusterConfig.Spec.Roles.Controller.Name
			}
		}
	}
	return controllerRoleName
}

func additionalControllerLabels() map[string]string {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Roles.Controller.Labels != nil {
				return clusterConfig.Spec.Roles.Controller.Labels
			}
		}
	}
	return map[string]string{}
}

func AdditionalCharts() []embeddedclusterv1beta1.Chart {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Extensions.Helm != nil {
				for k := range clusterConfig.Spec.Extensions.Helm.Charts {
					if clusterConfig.Spec.Extensions.Helm.Charts[k].Order == 0 {
						clusterConfig.Spec.Extensions.Helm.Charts[k].Order = charts.DefaultVendorChartOrder
					}
				}

				return clusterConfig.Spec.Extensions.Helm.Charts
			}
		}
	}
	return []embeddedclusterv1beta1.Chart{}
}

func AdditionalRepositories() []embeddedclusterv1beta1.Repository {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Extensions.Helm != nil {
				return clusterConfig.Spec.Extensions.Helm.Repositories
			}
		}
	}
	return []embeddedclusterv1beta1.Repository{}
}
