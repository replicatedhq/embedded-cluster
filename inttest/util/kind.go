package util

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/inttest/util/kind"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/util/wait"
)

type KindClusterOptions struct {
	NumControlPlaneNodes int
	NumWorkerNodes       int
	ExposedPorts         []int32
}

type KindExposedPorts map[string]string

func SetupKindCluster(t *testing.T, name string, opts *KindClusterOptions) string {
	config := NewKindClusterConfig(t, name, opts)
	return SetupKindClusterFromConfig(t, config)
}

func SetupKindClusterFromConfig(t *testing.T, config kind.Cluster) string {
	// cleanup previous test runs
	DeleteKindCluster(t, config.Name)

	kubeconfig := CreateKindClusterFromConfig(t, config)
	DeferCleanupKindCluster(t, config.Name)
	return kubeconfig
}

func CreateKindCluster(t *testing.T, name string, opts *KindClusterOptions) string {
	config := NewKindClusterConfig(t, name, opts)
	return CreateKindClusterFromConfig(t, config)
}

func CreateKindClusterFromConfig(t *testing.T, config kind.Cluster) string {
	kubeconfig := kindGetKubeconfigPath(t)
	configPath := writeKindClusterConfig(t, config)
	t.Logf("creating kind cluster %s", config.Name)
	out, err := exec.Command(
		"kind", "create", "cluster",
		"--config", configPath,
		"--kubeconfig", kubeconfig,
	).CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Fatalf("failed to create kind cluster: %s", err)
	}
	t.Logf("created kind cluster %s", config.Name)
	waitForDefaultServiceAccount(t, kubeconfig, 30*time.Second)
	return kubeconfig
}

func DeleteKindCluster(t *testing.T, name string) {
	t.Logf("deleting kind cluster %s", name)
	kubeconfig := kindGetKubeconfigPath(t)
	out, err := exec.Command("kind", "delete", "cluster", "--name", name, "--kubeconfig", kubeconfig).CombinedOutput()
	if err != nil {
		t.Logf("output: %s", out)
		t.Logf("WARN: failed to delete kind cluster: %s", err)
	}
	t.Logf("deleted kind cluster %s", name)
}

func DeferCleanupKindCluster(t *testing.T, name string) {
	if os.Getenv("DEBUG") != "" {
		return
	}
	t.Cleanup(func() { DeleteKindCluster(t, name) })
}

func KindGetExposedPort(t *testing.T, name string, containerPort string) string {
	nodes := kindListNodes(t, name)
	for _, node := range nodes {
		p := kindNodeGetExposedPorts(t, node)
		if v := p[containerPort]; v != "" {
			return v
		}
	}
	t.Fatalf("failed to find exposed port for container port %s", containerPort)
	return ""
}

func NewKindClusterConfig(t *testing.T, name string, opts *KindClusterOptions) kind.Cluster {
	config := kind.Cluster{
		APIVersion: "kind.x-k8s.io/v1alpha4",
		Kind:       "Cluster",
		Name:       name,
		Networking: kind.Networking{
			// same as k0s
			PodSubnet:     "10.244.0.0/16",
			ServiceSubnet: "10.96.0.0/12",
		},
	}
	numControllerNodes := 1
	numWorkerNodes := 0
	portMappings := []kind.PortMapping{
		{
			ContainerPort: 30000,
		},
	}
	if opts != nil {
		if opts.NumControlPlaneNodes > 0 {
			numControllerNodes = opts.NumControlPlaneNodes
		}
		numWorkerNodes = opts.NumWorkerNodes
		for _, port := range opts.ExposedPorts {
			portMappings = append(portMappings, kind.PortMapping{
				ContainerPort: port,
			})
		}
	}
	for i := 0; i < numControllerNodes; i++ {
		node := kind.Node{
			Role: "control-plane",
		}
		if numWorkerNodes == 0 && i == 0 {
			node.ExtraPortMappings = portMappings
		}
		config.Nodes = append(config.Nodes, node)
	}
	for i := 0; i < numWorkerNodes; i++ {
		node := kind.Node{
			Role: "worker",
		}
		if i == 0 {
			node.ExtraPortMappings = portMappings
		}
		config.Nodes = append(config.Nodes, node)
	}
	return config
}

func kindGetKubeconfigPath(t *testing.T) string {
	return filepath.Join("/tmp", t.Name(), "kubeconfig")
}

func writeKindClusterConfig(t *testing.T, config kind.Cluster) string {
	f, err := os.CreateTemp("", "kind-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	f.Close()
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal kind cluster config: %s", err)
	}
	if err := os.WriteFile(f.Name(), data, 0644); err != nil {
		t.Fatalf("failed to write kind cluster config: %s", err)
	}
	return f.Name()
}

func kindListNodes(t *testing.T, name string) []string {
	out, err := exec.Command("kind", "get", "nodes", "--name", name).Output()
	if err != nil {
		t.Fatalf("failed to get kind nodes: %s", err)
	}
	var nodes []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		nodes = append(nodes, strings.TrimSpace(line))
	}
	return nodes
}

func kindNodeGetExposedPorts(t *testing.T, name string) KindExposedPorts {
	out, err := exec.Command("docker", "container", "inspect", name).Output()
	if err != nil {
		t.Fatalf("failed to get docker container inspect: %s", err)
	}
	var inspect struct {
		NetworkSettings struct {
			Ports map[string][]struct {
				HostPort string `json:"HostPort"`
			}
		}
	}
	err = json.Unmarshal(out, &inspect)
	if err != nil {
		t.Fatalf("failed to unmarshal docker container inspect: %s", err)
	}
	ports := KindExposedPorts{}
	for port, bindings := range inspect.NetworkSettings.Ports {
		containerPort := strings.Split(port, "/")[0]
		for _, p := range bindings {
			ports[containerPort] = p.HostPort
		}
	}
	return ports
}

func waitForDefaultServiceAccount(t *testing.T, kubeconfig string, timeout time.Duration) {
	t.Logf("waiting for default service account to be ready")
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		cmd := exec.Command("kubectl", "--kubeconfig", kubeconfig, "get", "serviceaccount", "default", "-n", "default")
		err := cmd.Run()
		return err == nil, nil
	})
	if err != nil {
		t.Fatalf("failed to wait for default service account: %s", err)
	}
	t.Logf("default service account is ready")
}
