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
	"strings"

	"github.com/k0sproject/dig"
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/sirupsen/logrus"
	yamlv2 "gopkg.in/yaml.v2"
	"sigs.k8s.io/yaml"

	"github.com/replicatedhq/helmvm/pkg/addons"
	"github.com/replicatedhq/helmvm/pkg/defaults"
	"github.com/replicatedhq/helmvm/pkg/prompts"
	"github.com/replicatedhq/helmvm/pkg/ssh"
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

// SetUploadBinary sets the upload binary config to true in all hosts in the provided configuration.
func SetUploadBinary(config *v1beta1.Cluster) {
	for i := range config.Spec.Hosts {
		config.Spec.Hosts[i].UploadBinary = true
	}
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
	keys = append(keys, "other")
	if host.KeyPath == "" {
		host.KeyPath = keys[0]
	}
	host.KeyPath = quiz.Select("SSH key path:", keys, host.KeyPath)
	if host.KeyPath == "other" {
		host.KeyPath = quiz.Input("SSH key path:", "", true)
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

// validateHostConfig validates if the list of hosts represents a valid cluster.
// returns the number of controllers and workers in the configuration.
func validateHosts(hosts []*cluster.Host) (int, int, error) {
	if len(hosts) == 0 {
		return 0, 0, fmt.Errorf("no hosts configured")
	}
	var workers int
	var controllers int
	for _, host := range hosts {
		if strings.Contains(host.Role, "worker") {
			workers++
		}
		if strings.Contains(host.Role, "controller") {
			controllers++
		}
	}
	if controllers == 0 {
		return 0, 0, fmt.Errorf("at least one controller is required")
	}
	if workers == 0 {
		return 0, 0, fmt.Errorf("at least one worker is required")
	}
	return controllers, workers, nil
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

func askForLoadBalancer() (string, error) {
	fmt.Println("You have configured more than one controller. To ensure a highly available")
	fmt.Println("cluster with multiple controllers, configure a load balancer address that")
	fmt.Println("forwards traffic to the controllers on TCP ports 6443, 8132, and 9443.")
	fmt.Println("Optionally, you can press enter to skip load balancer configuration.")
	return quiz.Input("Load balancer address:", "", false), nil
}

// generateConfigForHosts generates a v1beta1.Cluster object for a given list of hosts.
func generateConfigForHosts(ctx context.Context, lb string, hosts ...*cluster.Host) (*v1beta1.Cluster, error) {

	var configSpec = dig.Mapping{
		"network": dig.Mapping{
			"provider": "calico",
		},
		"telemetry": dig.Mapping{
			"enabled": false,
		},
	}

	if lb != "" {
		configSpec["api"] = dig.Mapping{"externalAddress": lb, "sans": []string{lb}}
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
				Version: defaults.K0sVersion,
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
	host := &hostcfg{
		Address: ipaddr,
		Role:    "controller+worker",
		Port:    22,
		User:    usr.Username,
		KeyPath: defaults.SSHKeyPath(),
	}
	if err := host.testConnection(); err != nil {
		return nil, fmt.Errorf("unable to connect to %s: %w", ipaddr, err)
	}
	rhost := host.render()
	return generateConfigForHosts(ctx, "", rhost)
}

// UpdateHelmConfigs updates the helm config in the provided cluster configuration.
func UpdateHelmConfigs(cfg *v1beta1.Cluster, opts ...addons.Option) error {

	if cfg.Spec == nil || cfg.Spec.K0s == nil || cfg.Spec.K0s.Config == nil {
		return fmt.Errorf("invalid cluster configuration")
	}

	currentSpec := cfg.Spec.K0s.Config
	configString, err := yamlv2.Marshal(currentSpec)
	if err != nil {
		return fmt.Errorf("failed to marshal helm config: %w", err)
	}

	k0s := k0sconfig.ClusterConfig{}

	if err := yamlv2.Unmarshal(configString, &k0s); err != nil {
		return fmt.Errorf("unable to unmarshal k0s config: %w", err)
	}

	opts = append(opts, addons.WithConfig(k0s))

	newHelmConfig, err := addons.NewApplier(opts...).GenerateHelmConfigs()
	if err != nil {
		return fmt.Errorf("unable to apply addons: %w", err)
	}

	newHelmExtension := &k0sconfig.HelmExtensions{
		Charts: newHelmConfig,
	}

	newClusterExtensions := &k0sconfig.ClusterExtensions{
		Helm: newHelmExtension,
	}

	if spec, ok := cfg.Spec.K0s.Config["spec"].(map[string]interface{}); ok {
		spec["extensions"] = newClusterExtensions
		cfg.Spec.K0s.Config["spec"] = spec
	} else {

		if spec, ok := cfg.Spec.K0s.Config["spec"].(dig.Mapping); ok {
			spec["extensions"] = newClusterExtensions
			cfg.Spec.K0s.Config["spec"] = spec
		} else {
			return fmt.Errorf("unable to update cluster config")
		}
	}

	return nil
}

// UpdateHostsFiles passes through all hosts in the provided configuration and makes sure
// they all have their "Files" property properly set. The Files property is a list of files
// that need to be uploaded to the remote host.
func UpdateHostsFiles(cfg *v1beta1.Cluster, bundleDir string) error {
	for _, host := range cfg.Spec.Hosts {
		if err := updateHostFiles(host, bundleDir); err != nil {
			return fmt.Errorf("unable to update host file: %w", err)
		}
	}
	return nil
}

// updateHostFiles reads the provided bundle dir and sets up the host "Files" property.
func updateHostFiles(host *cluster.Host, bundleDir string) error {
	if len(bundleDir) == 0 || !strings.Contains(host.Role, "worker") {
		host.Files = nil
		return nil
	}
	entries, err := os.ReadDir(bundleDir)
	if err != nil {
		return fmt.Errorf("unable to read bundle directory: %w", err)
	}
	for _, entry := range entries {
		if !strings.Contains(entry.Name(), ".tar") {
			continue
		}
		fpath := fmt.Sprintf("%s/%s", bundleDir, entry.Name())
		file := &cluster.UploadFile{
			Source:         fpath,
			DestinationDir: "/var/lib/k0s/images",
			PermMode:       "0755",
		}
		host.Files = append(host.Files, file)
	}
	return nil
}
