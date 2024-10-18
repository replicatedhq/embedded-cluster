package util

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type KindClusterOptions struct {
	NumControlPlaneNodes int
	NumWorkerNodes       int
	ExposedPorts         []int32
}

type KindExposedPorts map[string]string

func CreateKindCluster(t *testing.T, name string, opts *KindClusterOptions) string {
	config := createKindClusterConfig(t, name, opts)
	t.Logf("creating kind cluster %s", name)
	out, err := exec.Command("kind", "create", "cluster", "--config", config).CombinedOutput()
	if err != nil {
		t.Logf("stdout: %s", out)
		t.Fatalf("failed to create kind cluster: %s", err)
	}
	t.Logf("created kind cluster %s", name)
	return kindGetKubeconfig(t, name)
}

func DeleteKindCluster(t *testing.T, name string) {
	t.Logf("deleting kind cluster %s", name)
	out, err := exec.Command("kind", "delete", "cluster", "--name", name).CombinedOutput()
	if err != nil {
		t.Logf("stdout: %s", out)
		t.Fatalf("failed to delete kind cluster: %s", err)
	}
	t.Logf("deleted kind cluster %s", name)
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

type kindCluster struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Name       string         `yaml:"name" json:"name"`
	Nodes      []kindNode     `yaml:"nodes,omitempty" json:"nodes,omitempty"`
	Networking kindNetworking `yaml:"networking,omitempty" json:"networking,omitempty"`
}

type kindNode struct {
	Role              string            `yaml:"role,omitempty" json:"role,omitempty"`
	ExtraPortMappings []kindPortMapping `yaml:"extraPortMappings,omitempty" json:"extraPortMappings,omitempty"`
}

type kindPortMapping struct {
	ContainerPort int32  `yaml:"containerPort,omitempty" json:"containerPort,omitempty"`
	HostPort      int32  `yaml:"hostPort,omitempty" json:"hostPort,omitempty"`
	ListenAddress string `yaml:"listenAddress,omitempty" json:"listenAddress,omitempty"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

type kindNetworking struct {
	PodSubnet     string `yaml:"podSubnet,omitempty" json:"podSubnet,omitempty"`
	ServiceSubnet string `yaml:"serviceSubnet,omitempty" json:"serviceSubnet,omitempty"`
}

func createKindClusterConfig(t *testing.T, name string, opts *KindClusterOptions) string {
	f, err := os.CreateTemp("", "kind-config-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	f.Close()
	config := kindCluster{
		APIVersion: "kind.x-k8s.io/v1alpha4",
		Kind:       "Cluster",
		Name:       name,
		Networking: kindNetworking{
			// same as k0s
			PodSubnet:     "10.244.0.0/16",
			ServiceSubnet: "10.96.0.0/12",
		},
	}
	numControllerNodes := 1
	numWorkerNodes := 0
	portMappings := []kindPortMapping{
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
			portMappings = append(portMappings, kindPortMapping{
				ContainerPort: port,
			})
		}
	}
	for i := 0; i < numControllerNodes; i++ {
		node := kindNode{
			Role: "control-plane",
		}
		if numWorkerNodes == 0 && i == 0 {
			node.ExtraPortMappings = portMappings
		}
		config.Nodes = append(config.Nodes, node)
	}
	for i := 0; i < numWorkerNodes; i++ {
		node := kindNode{
			Role: "worker",
		}
		if i == 0 {
			node.ExtraPortMappings = portMappings
		}
		config.Nodes = append(config.Nodes, node)
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal kind cluster config: %s", err)
	}
	if err := os.WriteFile(f.Name(), data, 0644); err != nil {
		t.Fatalf("failed to write kind cluster config: %s", err)
	}
	return f.Name()
}

func kindGetKubeconfig(t *testing.T, name string) string {
	out, err := exec.Command("kind", "get", "kubeconfig", "--name", name).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get kind kubeconfig: %s", err)
	}
	f, err := os.CreateTemp("", "kind-kubeconfig-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	f.Close()
	os.WriteFile(f.Name(), out, 0644)
	return f.Name()
}

func kindListNodes(t *testing.T, name string) []string {
	out, err := exec.Command("kind", "get", "nodes", "--name", name).CombinedOutput()
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
	out, err := exec.Command("docker", "container", "inspect", name).CombinedOutput()
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
