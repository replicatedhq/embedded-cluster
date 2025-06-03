// Package config handles the cluster configuration file generation.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/paths"
	"gopkg.in/yaml.v2"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

const (
	DefaultServiceNodePortRange = "80-32767"
	DefaultVendorChartOrder     = 10
)

// k0sConfigPathOverride is used during tests to override the path to the k0s config file.
var k0sConfigPathOverride string

// RenderK0sConfig renders a k0s cluster configuration.
func RenderK0sConfig(proxyRegistryDomain string) *k0sconfig.ClusterConfig {
	cfg := k0sconfig.DefaultClusterConfig()
	// Customize the default k0s configuration to our taste.
	cfg.Name = runtimeconfig.BinaryName()
	cfg.Spec.Konnectivity = nil
	cfg.Spec.Network.KubeRouter = nil
	cfg.Spec.Network.Provider = "calico"
	// We need to disable telemetry in a backwards compatible way with k0s v1.30 and v1.29
	// See - https://github.com/k0sproject/k0s/pull/4674/files#diff-eea4a0c68e41d694c3fd23b4865a7b28bcbba61dc9c642e33c2e2f5f7f9ee05d
	// We can drop the json.Unmarshal once we drop support for 1.30
	err := json.Unmarshal([]byte("false"), &cfg.Spec.Telemetry.Enabled)
	if err != nil {
		panic(fmt.Sprintf("unable to unmarshal telemetry enabled: %v", err))
	}
	if cfg.Spec.API.ExtraArgs == nil {
		cfg.Spec.API.ExtraArgs = map[string]string{}
	}
	cfg.Spec.API.ExtraArgs["service-node-port-range"] = DefaultServiceNodePortRange
	cfg.Spec.API.SANs = append(cfg.Spec.API.SANs, "kubernetes.default.svc.cluster.local")
	cfg.Spec.Network.NodeLocalLoadBalancing.Enabled = true
	cfg.Spec.Network.NodeLocalLoadBalancing.Type = k0sconfig.NllbTypeEnvoyProxy
	overrideK0sImages(cfg, proxyRegistryDomain)
	return cfg
}

// extractK0sConfigPatch extracts the k0s config portion of the provided patch.
func extractK0sConfigPatch(raw string, respectImmutableFields bool) (string, error) {
	type PatchBody struct {
		Config map[string]interface{} `yaml:"config"`
	}
	var body PatchBody
	if err := yaml.Unmarshal([]byte(raw), &body); err != nil {
		return "", fmt.Errorf("unable to unmarshal patch body: %w", err)
	}

	if respectImmutableFields {
		body.Config = removeImmutableFields(body.Config)
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
func PatchK0sConfig(config *k0sconfig.ClusterConfig, patch string, respectImmutableFields bool) (*k0sconfig.ClusterConfig, error) {
	if patch == "" {
		return config, nil
	}
	patch, err := extractK0sConfigPatch(patch, respectImmutableFields)
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
	// Fix for - https://github.com/k0sproject/k0s/pull/5834 - currently the process of unmarshaling a config with a
	// calico config will also set a default kube-router config. We remove it here.
	if patched.Spec.Network.Provider == "calico" {
		patched.Spec.Network.KubeRouter = nil
	}
	return &patched, nil
}

// InstallFlags returns a list of default flags to be used when bootstrapping a k0s cluster.
func InstallFlags(nodeIP string) ([]string, error) {
	flags := []string{
		"install",
		"controller",
		"--labels", strings.Join(nodeLabels(), ","),
		"--enable-worker",
		"--no-taints",
		"-c", paths.PathToK0sConfig(),
	}
	profile, err := ProfileInstallFlag()
	if err != nil {
		return nil, fmt.Errorf("unable to get profile install flag: %w", err)
	}
	if profile != "" {
		flags = append(flags, profile)
	}
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

func ProfileInstallFlag() (string, error) {
	controllerProfile, err := controllerWorkerProfile()
	if err != nil {
		return "", fmt.Errorf("unable to get controller worker profile: %w", err)
	}
	if controllerProfile == "" {
		return "", nil
	}
	return "--profile=" + controllerProfile, nil
}

func GetControllerRoleName() string {
	clusterConfig := release.GetEmbeddedClusterConfig()
	controllerRoleName := "controller"
	if clusterConfig != nil {
		if clusterConfig.Spec.Roles.Controller.Name != "" {
			controllerRoleName = clusterConfig.Spec.Roles.Controller.Name
		}
	}
	return controllerRoleName
}

func HasCustomRoles() bool {
	clusterConfig := release.GetEmbeddedClusterConfig()
	return clusterConfig != nil && clusterConfig.Spec.Roles.Custom != nil && len(clusterConfig.Spec.Roles.Custom) > 0
}

// nodeLabels return a slice of string with labels (key=value format) for the node where we
// are installing the k0s.
func nodeLabels() []string {
	labels := []string{}
	for k, v := range controllerLabels() {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(labels)
	return labels
}

func controllerLabels() map[string]string {
	lmap := additionalControllerLabels()
	lmap["kots.io/embedded-cluster-role-0"] = GetControllerRoleName()
	lmap["kots.io/embedded-cluster-role"] = "total-1"
	return lmap
}

func additionalControllerLabels() map[string]string {
	clusterConfig := release.GetEmbeddedClusterConfig()
	if clusterConfig != nil {
		if clusterConfig.Spec.Roles.Controller.Labels != nil {
			return clusterConfig.Spec.Roles.Controller.Labels
		}
	}
	return map[string]string{}
}

func controllerWorkerProfile() (string, error) {
	// Read the k0s config file
	k0sPath := paths.PathToK0sConfig()
	if k0sConfigPathOverride != "" {
		k0sPath = k0sConfigPathOverride
	}

	data, err := os.ReadFile(k0sPath)
	if err != nil {
		return "", fmt.Errorf("unable to read k0s config: %w", err)
	}

	var cfg k0sconfig.ClusterConfig
	if err := k8syaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("unable to unmarshal k0s config: %w", err)
	}

	// Return the first worker profile name if any exist
	if len(cfg.Spec.WorkerProfiles) > 0 {
		return cfg.Spec.WorkerProfiles[0].Name, nil
	}
	return "", nil
}

func AdditionalCharts() []embeddedclusterv1beta1.Chart {
	clusterConfig := release.GetEmbeddedClusterConfig()
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
	return []embeddedclusterv1beta1.Chart{}
}

func AdditionalRepositories() []k0sconfig.Repository {
	clusterConfig := release.GetEmbeddedClusterConfig()
	if clusterConfig != nil {
		if clusterConfig.Spec.Extensions.Helm != nil {
			return clusterConfig.Spec.Extensions.Helm.Repositories
		}
	}
	return []k0sconfig.Repository{}
}

// removeImmutableFields removes the immutable fields from the patch.
// 'Immutable fields' are things that should not be changed in the k0s cluster config after installation,
// such as the cluster name, the spec.api object, and the spec.storage object.
func removeImmutableFields(patch map[string]interface{}) map[string]interface{} {
	delete(patch, "metadata")

	// handle "spec" subkeys
	spec, ok := patch["spec"].(map[interface{}]interface{})
	if !ok {
		return patch
	}

	delete(spec, "api")
	delete(spec, "storage")
	patch["spec"] = spec

	return patch
}
