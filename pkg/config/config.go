// Package config handles the cluster configuration file generation.
package config

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/k0sproject/dig"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/controllers"
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
func RenderK0sConfig(ctx context.Context) (*k0sconfig.ClusterConfig, error) {
	bin := defaults.PathToEmbeddedClusterBinary("k0s")
	cmd := exec.Command(bin, "config", "create")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("unable to generate default config: %w", err)
	}
	var cfg k0sconfig.ClusterConfig
	if err := k8syaml.Unmarshal(output, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal default config: %w", err)
	}
	// Customize the default k0s configuration to our taste.
	cfg.Name = defaults.BinaryName()
	cfg.Spec.Images = nil
	cfg.Spec.Konnectivity = nil
	cfg.Spec.Network.KubeRouter = nil
	cfg.Spec.Network.Provider = "calico"
	cfg.Spec.Telemetry.Enabled = false
	return &cfg, nil
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
	cfg.Spec.Extensions = &k0sconfig.ClusterExtensions{
		Helm: &k0sconfig.HelmExtensions{
			Charts:       chtconfig,
			Repositories: repconfig,
		},
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

// nodeLabels return a slice of string with labels (key=value format) for the node where we
// are installing the k0s.
func nodeLabels() []string {
	lmap := additionalControllerLabels()
	lmap["kots.io/embedded-cluster-role-0"] = getControllerRoleName()
	lmap["kots.io/embedded-cluster-role"] = "total-1"
	labels := []string{}
	for k, v := range lmap {
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

func AdditionalCharts() []k0sconfig.Chart {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Extensions.Helm != nil {
				for k := range clusterConfig.Spec.Extensions.Helm.Charts {
					if clusterConfig.Spec.Extensions.Helm.Charts[k].Order == 0 {
						clusterConfig.Spec.Extensions.Helm.Charts[k].Order = controllers.DEFAULT_VENDOR_CHART_ORDER
					}
				}

				return clusterConfig.Spec.Extensions.Helm.Charts
			}
		}
	}
	return []k0sconfig.Chart{}
}

func AdditionalRepositories() []k0sconfig.Repository {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Extensions.Helm != nil {
				return clusterConfig.Spec.Extensions.Helm.Repositories
			}
		}
	}
	return []k0sconfig.Repository{}
}
