// Package config handles the cluster configuration file generation. It implements
// an interactive configuration generation as well as provides default configuration
// for single node deployments.
package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	jsonpatch "github.com/evanphx/json-patch"
	fmtconvert "github.com/ghodss/yaml"
	"github.com/k0sproject/dig"
	helmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	k0sversion "github.com/k0sproject/version"
	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster-operator/api/v1beta1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/customization"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/ssh"
)

// roles holds a list of valid roles.
var roles = []string{"controller+worker", "worker"}

// quiz prompts for the cluster configuration interactively.
var quiz = prompts.New()

// ReadConfigFile reads the cluster configuration from the provided file.
func ReadConfigFile(cfgPath string) (*v1beta1.Cluster, error) {
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read current config: %w", err)
	}
	var cfg v1beta1.Cluster
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unable to unmarshal current config: %w", err)
	}
	return &cfg, nil
}

// RenderClusterConfig renders a cluster configuration interactively.
func RenderClusterConfig(ctx context.Context, multi bool) (*v1beta1.Cluster, error) {
	clusterConfig, err := customization.AdminConsole{}.EmbeddedClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to get embedded cluster config: %w", err)
	} else if clusterConfig == nil {
		clusterConfig = &embeddedclusterv1beta1.Config{}
	}
	if multi {
		cfg, err := renderMultiNodeConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to render multi-node config: %w", err)
		}
		ApplyEmbeddedUnsupportedOverrides(cfg, clusterConfig.Spec.UnsupportedOverrides.K0s)
		return cfg, nil
	}
	cfg, err := renderSingleNodeConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to render single-node config: %w", err)
	}
	ApplyEmbeddedUnsupportedOverrides(cfg, clusterConfig.Spec.UnsupportedOverrides.K0s)
	return renderSingleNodeConfig(ctx)
}

// listUserSSHKeys returns a list of private SSH keys in the user's ~/.ssh directory.
func listUserSSHKeys() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to get user home dir: %w", err)
	}
	keysdir := fmt.Sprintf("%s/.ssh", home)
	keys := []string{}
	if err := filepath.Walk(keysdir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if content, err := os.ReadFile(path); err != nil {
			return fmt.Errorf("unable to ssh config %s: %w", path, err)
		} else if bytes.Contains(content, []byte("PRIVATE KEY")) {
			keys = append(keys, path)
		}
		return nil
	}); err != nil {
		if os.IsNotExist(err) {
			return keys, nil
		}
		return nil, fmt.Errorf("unable to read ssh keys dir: %w", err)
	}
	return keys, nil
}

// askUserForHostSSHKey asks the user for the SSH key path.
func askUserForHostSSHKey(keys []string, host *hostcfg) error {
	if len(keys) == 0 {
		host.KeyPath = quiz.Input("SSH key path:", "", true)
		return nil
	}
	keys = append(keys, "use existing agent", "other")
	if host.KeyPath == "" {
		host.KeyPath = keys[0]
	}
	host.KeyPath = quiz.Select("SSH key path:", keys, host.KeyPath)
	if host.KeyPath == "other" {
		host.KeyPath = quiz.Input("SSH key path:", "", true)
	} else if host.KeyPath == "use existing agent" {
		host.KeyPath = ""
	}
	return nil
}

// askUserForHostConfig collects a host SSH configuration interactively.
func askUserForHostConfig(keys []string, host *hostcfg) error {
	fmt.Println("Please provide SSH configuration for the host.")
	host.Address = quiz.Input("Node address:", host.Address, true)
	host.User = quiz.Input("SSH user:", host.User, true)
	var err error
	var port int
	for port == 0 {
		str := quiz.Input("SSH port:", strconv.Itoa(host.Port), true)
		if port, err = strconv.Atoi(str); err != nil {
			fmt.Println("Invalid port number")
		}
	}
	host.Port = port
	host.Role = quiz.Select("Node role:", roles, host.Role)
	if err := askUserForHostSSHKey(keys, host); err != nil {
		return fmt.Errorf("unable to ask for host ssh key: %w", err)
	}
	return nil
}

// collectHostConfig collects a host SSH configuration interactively and verifies we can
// connect using the provided information.
func collectHostConfig(host hostcfg) (*cluster.Host, error) {
	keys, err := listUserSSHKeys()
	if err != nil {
		return nil, fmt.Errorf("unable to list user ssh keys: %w", err)
	}
	for {
		if err := askUserForHostConfig(keys, &host); err != nil {
			return nil, fmt.Errorf("unable to ask user for host config: %w", err)
		}
		logrus.Infof("Testing SSH connection to %s", host.Address)
		if err := host.testConnection(); err != nil {
			fmt.Printf("Unable to connect to %s\n", host.Address)
			fmt.Println("Please check the provided SSH configuration.")
			continue
		}
		logrus.Infof("SSH connection to %s successful", host.Address)
		break
	}
	return host.render(), nil
}

// interactiveHosts asks the user for host configuration interactively.
func interactiveHosts(ctx context.Context) ([]*cluster.Host, error) {
	hosts := []*cluster.Host{}
	user, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("unable to get current user: %w", err)
	}
	defhost := hostcfg{
		Role:    "controller+worker",
		Port:    22,
		User:    user.Username,
		KeyPath: "",
	}
	for {
		host, err := collectHostConfig(defhost)
		if err != nil {
			return nil, fmt.Errorf("unable to assemble node config: %w", err)
		}
		hosts = append(hosts, host)
		if !quiz.Confirm("Add another node?", true) {
			break
		}
		defhost.Port = host.SSH.Port
		defhost.User = host.SSH.User
		if host.SSH.KeyPath != nil {
			defhost.KeyPath = *host.SSH.KeyPath
		}
	}
	return hosts, nil
}

// renderMultiNodeConfig renders a configuration to allow k0sctl to install in a multi-node
// configuration.
func renderMultiNodeConfig(ctx context.Context) (*v1beta1.Cluster, error) {
	var err error
	var hosts []*cluster.Host
	fmt.Println("You are about to configure a new cluster.")
	if hosts, err = interactiveHosts(ctx); err != nil {
		return nil, fmt.Errorf("unable to collect hosts: %w", err)
	}
	return generateConfigForHosts(ctx, hosts...)
}

// generateConfigForHosts generates a v1beta1.Cluster object for a given list of hosts.
func generateConfigForHosts(ctx context.Context, hosts ...*cluster.Host) (*v1beta1.Cluster, error) {
	var configSpec = dig.Mapping{
		"network": dig.Mapping{
			"provider": "calico",
		},
		"telemetry": dig.Mapping{
			"enabled": false,
		},
	}
	var k0sconfig = dig.Mapping{
		"apiVersion": "k0s.k0sproject.io/v1beta1",
		"kind":       "ClusterConfig",
		"metadata":   dig.Mapping{"name": defaults.BinaryName()},
		"spec":       configSpec,
	}
	return &v1beta1.Cluster{
		APIVersion: "k0sctl.k0sproject.io/v1beta1",
		Kind:       "Cluster",
		Metadata:   &v1beta1.ClusterMetadata{Name: defaults.BinaryName()},
		Spec: &cluster.Spec{
			Hosts: hosts,
			K0s: &cluster.K0s{
				Version: k0sversion.MustParse(defaults.K0sVersion),
				Config:  k0sconfig,
			},
		},
	}, nil
}

// renderSingleNodeConfig renders a configuration to allow k0sctl to install in the localhost
// in a single-node configuration.
func renderSingleNodeConfig(ctx context.Context) (*v1beta1.Cluster, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("unable to get current user: %w", err)
	}
	if err := ssh.AllowLocalSSH(); err != nil {
		return nil, fmt.Errorf("unable to allow localhost SSH: %w", err)
	}
	ipaddr, err := defaults.PreferredNodeIPAddress()
	if err != nil {
		return nil, fmt.Errorf("unable to get preferred node IP address: %w", err)
	}

	labels := additionalControllerLabels()
	labels["kots.io/embedded-cluster-role-0"] = getControllerRoleName()
	labels["kots.io/embedded-cluster-role"] = "total-1"

	host := &hostcfg{
		Address: ipaddr,
		Role:    "controller+worker",
		Port:    22,
		User:    usr.Username,
		KeyPath: defaults.SSHKeyPath(),
		Labels:  labels,
	}
	if err := host.testConnection(); err != nil {
		return nil, fmt.Errorf("unable to connect to %s: %w", ipaddr, err)
	}
	rhost := host.render()
	return generateConfigForHosts(ctx, rhost)
}

// UpdateHelmConfigs updates the helm config in the provided cluster configuration.
func UpdateHelmConfigs(cfg *v1beta1.Cluster, opts ...addons.Option) ([]helmv1beta1.Chart, error) {
	if cfg.Spec == nil || cfg.Spec.K0s == nil || cfg.Spec.K0s.Config == nil {
		return nil, fmt.Errorf("invalid cluster configuration")
	}
	currentSpec := cfg.Spec.K0s.Config
	configString, err := yaml.Marshal(currentSpec)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal helm config: %w", err)
	}
	k0s := k0sconfig.ClusterConfig{}
	if err := yaml.Unmarshal(configString, &k0s); err != nil {
		return nil, fmt.Errorf("unable to unmarshal k0s config: %w", err)
	}
	opts = append(opts, addons.WithConfig(k0s))
	chtconfig, repconfig, err := addons.NewApplier(opts...).GenerateHelmConfigs()
	if err != nil {
		return nil, fmt.Errorf("unable to apply addons: %w", err)
	}

	newHelmExtension := &k0sconfig.HelmExtensions{
		Repositories: repconfig,
	}
	newClusterExtensions := &k0sconfig.ClusterExtensions{
		Helm: newHelmExtension,
	}

	// marshal and unmarshal to convert key names from struct tags
	newClusterExtensionsBytes, err := yaml.Marshal(newClusterExtensions)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal new cluster extensions: %w", err)
	}
	newClusterExtensionsMap := dig.Mapping{}
	if err := yaml.Unmarshal(newClusterExtensionsBytes, &newClusterExtensionsMap); err != nil {
		return nil, fmt.Errorf("unable to unmarshal new cluster extensions: %w", err)
	}

	// workaround for no extensions.storage safety in k0s
	delete(newClusterExtensionsMap, "storage")

	if spec, ok := cfg.Spec.K0s.Config["spec"].(map[string]interface{}); ok {
		spec["extensions"] = newClusterExtensionsMap
		cfg.Spec.K0s.Config["spec"] = spec
	} else {
		if spec, ok := cfg.Spec.K0s.Config["spec"].(dig.Mapping); ok {
			spec["extensions"] = newClusterExtensionsMap
			cfg.Spec.K0s.Config["spec"] = spec
		} else {
			return nil, fmt.Errorf("unable to update cluster config")
		}
	}
	return chtconfig, nil
}

// ApplyEmbeddedUnsupportedOverrides applies the custom configuration to the cluster config.
func ApplyEmbeddedUnsupportedOverrides(config *v1beta1.Cluster, embconfig string) error {
	if embconfig == "" {
		return nil
	}
	newConfigBytes, err := yaml.Marshal(config.Spec.K0s.Config)
	if err != nil {
		return fmt.Errorf("unable to marshal original cluster config: %w", err)
	}
	original, err := fmtconvert.YAMLToJSON(newConfigBytes)
	if err != nil {
		return fmt.Errorf("unable to convert cluster config overrides to json: %w", err)
	}
	target, err := fmtconvert.YAMLToJSON([]byte(embconfig))
	if err != nil {
		return fmt.Errorf("unable to convert original cluster config to json: %w", err)
	}
	result, err := jsonpatch.MergePatch(original, target)
	if err != nil {
		return fmt.Errorf("unable to create patch configuration: %w", err)
	}
	newConfigBytes, err = fmtconvert.JSONToYAML(result)
	if err != nil {
		return fmt.Errorf("unable to convert patched configuration to json: %w", err)
	}
	var newK0sConfig dig.Mapping
	if err := yaml.Unmarshal(newConfigBytes, &newK0sConfig); err != nil {
		return fmt.Errorf("unable to unmarshal patched cluster config: %w", err)
	}
	config.Spec.K0s.Config = newK0sConfig
	return nil
}

func getControllerRoleName() string {
	clusterConfig, err := customization.AdminConsole{}.EmbeddedClusterConfig()

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
	clusterConfig, err := customization.AdminConsole{}.EmbeddedClusterConfig()

	if err == nil {
		if clusterConfig != nil {
			if clusterConfig.Spec.Roles.Controller.Labels != nil {
				return clusterConfig.Spec.Roles.Controller.Labels
			}
		}
	}
	return map[string]string{}
}
