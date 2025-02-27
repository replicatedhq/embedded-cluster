// Package config handles the cluster configuration file generation.
package config

import (
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"gopkg.in/yaml.v2"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

const (
	DefaultServiceNodePortRange = "80-32767"
	DefaultVendorChartOrder     = 10
)

// RenderK0sConfig renders a k0s cluster configuration.
func RenderK0sConfig() *k0sconfig.ClusterConfig {
	cfg := k0sconfig.DefaultClusterConfig()
	// Customize the default k0s configuration to our taste.
	cfg.Name = runtimeconfig.BinaryName()
	cfg.Spec.Konnectivity = nil
	cfg.Spec.Network.KubeRouter = nil
	cfg.Spec.Network.Provider = "calico"
	cfg.Spec.Telemetry.Enabled = false
	if cfg.Spec.API.ExtraArgs == nil {
		cfg.Spec.API.ExtraArgs = map[string]string{}
	}
	cfg.Spec.API.ExtraArgs["service-node-port-range"] = DefaultServiceNodePortRange
	cfg.Spec.API.SANs = append(cfg.Spec.API.SANs, "kubernetes.default.svc.cluster.local")
	overrideK0sImages(cfg)
	return cfg
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
func InstallFlags(nodeIP string, cfg *k0sconfig.ClusterConfig) ([]string, error) {
	flags := []string{
		"install",
		"controller",
		"--labels", strings.Join(nodeLabels(), ","),
		"--enable-worker",
		"--no-taints",
		"-c", runtimeconfig.PathToK0sConfig(),
	}
	profile, err := ProfileInstallFlag(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to get profile install flag: %w", err)
	}
	flags = append(flags, profile)
	flags = append(flags, AdditionalInstallFlags(nodeIP)...)
	flags = append(flags, AdditionalInstallFlagsController()...)
	return flags, nil
}

func AdditionalInstallFlags(nodeIP string) []string {
	return []string{
		// NOTE: quotes are not supported in older systemd
		// kardianos/service will escape spaces with "\x20"
		"--kubelet-extra-args", fmt.Sprintf("--node-ip=%s", nodeIP),
		"--data-dir", runtimeconfig.EmbeddedClusterK0sSubDir(),
	}
}

func AdditionalInstallFlagsController() []string {
	return []string{
		"--disable-components", "konnectivity-server",
		"--enable-dynamic-config",
	}
}

func ProfileInstallFlag(cfg *k0sconfig.ClusterConfig) (string, error) {
	controllerProfile := controllerWorkerProfile()
	if controllerProfile == "" {
		return "", nil
	}

	// make sure that the controller profile role name exists in the worker profiles
	for _, profile := range cfg.Spec.WorkerProfiles {
		if profile.Name == controllerProfile {
			return "--profile=" + controllerProfile, nil
		}
	}

	return "", fmt.Errorf("controller profile %q not found in k0s worker profiles", controllerProfile)
}

// nodeLabels return a slice of string with labels (key=value format) for the node where we
// are installing the k0s.
func nodeLabels() []string {
	labels := []string{}
	for k, v := range controllerLabels() {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	return labels
}

func controllerLabels() map[string]string {
	lmap := additionalControllerLabels()
	lmap["kots.io/embedded-cluster-role-0"] = getControllerRoleName()
	lmap["kots.io/embedded-cluster-role"] = "total-1"
	return lmap
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

func controllerWorkerProfile() string {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Roles.Controller.WorkerProfile != "" {
				return clusterConfig.Spec.Roles.Controller.WorkerProfile
			}
		}
	}
	return ""
}

func AdditionalCharts() []embeddedclusterv1beta1.Chart {
	clusterConfig, err := release.GetEmbeddedClusterConfig()
	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Extensions.Helm != nil {
				for k := range clusterConfig.Spec.Extensions.Helm.Charts {
					if clusterConfig.Spec.Extensions.Helm.Charts[k].Order == 0 {
						clusterConfig.Spec.Extensions.Helm.Charts[k].Order = DefaultVendorChartOrder
					}
				}

				return clusterConfig.Spec.Extensions.Helm.Charts
			}
		}
	}
	return []embeddedclusterv1beta1.Chart{}
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
