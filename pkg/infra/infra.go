// Package infra manages infrastructure creation for helmvm. This mostly works
// with terraform to apply the infrastructure defined in a terraform directory.
package infra

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/jedib0t/go-pretty/table"

	"github.com/replicatedhq/helmvm/pkg/defaults"
	pb "github.com/replicatedhq/helmvm/pkg/progressbar"
	"github.com/replicatedhq/helmvm/pkg/prompts"
)

// Node is a node as defined in the terraform output. This is used to parse the
// terraform output directly into a cluster node configuration that can be used
// to create a cluster.
type Node struct {
	Address string `json:"address"`
	Role    string `json:"role"`
	Port    int    `json:"port"`
	User    string `json:"user"`
	KeyPath string `json:"keyPath"`
}

// ApplyFn is the function that actually applies the infrastructure using terraform
// library. This exists so we can make tests that don't actually run terraform.
type ApplyFn func(context.Context, string, *pb.MessageWriter) (map[string]tfexec.OutputMeta, error)

// Infra is a struct that holds functions to apply and create infrastructure.
type Infra struct {
	apply  ApplyFn
	printf func(string, ...any) (int, error)
}

// New creates a new Infra struct.
func New() *Infra {
	return &Infra{printf: fmt.Printf}
}

// Apply uses "terraform apply" to apply the infrastructe defined in the
// directory passed as argument.
func (inf *Infra) Apply(ctx context.Context, dir string, useprompt bool) ([]Node, error) {
	loading := pb.Start()
	loading.Infof("Applying infrastructure from %s", dir)
	applyfn := inf.runApply
	if inf.apply != nil {
		applyfn = inf.apply
	}
	outputs, err := applyfn(ctx, dir, loading)
	if err != nil {
		loading.Close()
		return nil, fmt.Errorf("unable to apply infrastructure: %w", err)
	}
	loading.Close()
	nodes, err := inf.ReadNodes(outputs)
	if err != nil {
		return nil, fmt.Errorf("unable to process terraform output: %w", err)
	}
	inf.PrintNodes(nodes)
	if !useprompt {
		return nodes, nil
	}
	fmt.Println("You may want to take note of the output for later use")
	prompts.New().PressEnter("Press enter to continue")
	return nodes, nil
}

// PrintNodes prints the nodes to stdout in a table.
func (inf *Infra) PrintNodes(nodes []Node) {
	fmt.Println("These are the nodes configuration applied by your configuration:")
	writer := table.NewWriter()
	writer.AppendHeader(table.Row{"Address", "Role", "SSH Port", "SSH User", "SSH Key Path"})
	for _, node := range nodes {
		writer.AppendRow(table.Row{node.Address, node.Role, node.Port, node.User, node.KeyPath})
	}
	inf.printf("%s\n", writer.Render())
	fmt.Println("These are going to be used as your cluster configuration")
}

// readIPAddresses reads the nodes from the instance_ips terraform output.
func (inf *Infra) ReadNodes(outputs map[string]tfexec.OutputMeta) ([]Node, error) {
	var nodes []Node
	for key, output := range outputs {
		if key != "nodes" {
			continue
		}
		if err := json.Unmarshal(output.Value, &nodes); err != nil {
			return nil, fmt.Errorf("unable to unmarshal terraform output: %w", err)
		}
		break
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes found in terraform output")
	}
	return nodes, nil
}

// runApply actually runs the terraform apply. This expects the terraform output to contain a
// property 'instance_ips' which is a list of the created or updated node ip addresses.
func (inf *Infra) runApply(ctx context.Context, dir string, log *pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
	log.Infof("Reading terraform infrastructure from %s", dir)
	exe := defaults.PathToHelmVMBinary("terraform")
	tf, err := tfexec.NewTerraform(dir, exe)
	if err != nil {
		return nil, fmt.Errorf("unable to create terraform: %w", err)
	}
	log.Infof("Using terraform binary stored at %s", exe)
	err = tf.Init(ctx, tfexec.Upgrade(true))
	if err != nil {
		return nil, fmt.Errorf("unable to init terraform: %w", err)
	}
	log.Infof("Applying terraform infrastructure from %s", dir)
	if err := tf.Apply(ctx); err != nil {
		return nil, fmt.Errorf("unable to apply terraform: %w", err)
	}
	outputs, err := tf.Output(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get terraform output: %w", err)
	}
	return outputs, nil
}

// Apply is a helper function that creates an Infra instance and calls Apply on it.
func Apply(ctx context.Context, dir string, useprompt bool) ([]Node, error) {
	return New().Apply(ctx, dir, useprompt)
}
