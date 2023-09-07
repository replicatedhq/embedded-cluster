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

// Apply uses "terraform apply" to apply the infrastructe defined in the
// directory passed as argument.
func Apply(ctx context.Context, dir string, useprompt bool) ([]Node, error) {
	log, end := pb.Start()
	outputs, err := runApply(ctx, dir, log)
	if err != nil {
		log.Close()
		<-end
		return nil, fmt.Errorf("unable to apply infrastructure: %w", err)
	}
	log.Close()
	<-end
	fmt.Println("Infrastructure applied successfully")
	nodes, err := readNodes(outputs)
	if err != nil {
		return nil, fmt.Errorf("unable to process terraform output: %w", err)
	}
	printNodes(nodes)
	if !useprompt {
		return nodes, nil
	}
	fmt.Println("You may want to take note of the output for later use")
	prompts.New().PressEnter("Press enter to continue")
	return nodes, nil
}

// printNodes prints the nodes to stdout in a table.
func printNodes(nodes []Node) {
	if len(nodes) == 0 {
		fmt.Println("No node found in terraform output")
		return
	}
	fmt.Println("These are the nodes configuration applied by your configuration:")
	writer := table.NewWriter()
	writer.AppendHeader(table.Row{"Address", "Role", "SSH Port", "SSH User", "SSH Key Path"})
	for _, node := range nodes {
		writer.AppendRow(table.Row{node.Address, node.Role, node.Port, node.User, node.KeyPath})
	}
	fmt.Printf("%s\n", writer.Render())
	fmt.Println("These are going to be used as your cluster configuration")
}

// readIPAddresses reads the nodes from the instance_ips terraform output.
func readNodes(outputs map[string]tfexec.OutputMeta) ([]Node, error) {
	for key, output := range outputs {
		if key != "nodes" {
			continue
		}
		var nodes []Node
		if err := json.Unmarshal(output.Value, &nodes); err != nil {
			return nil, fmt.Errorf("unable to unmarshal terraform output: %w", err)
		}
		return nodes, nil
	}
	return nil, nil
}

// runApply actually runs the terraform apply. This expects the terraform output
// to contain a property 'instance_ips' which is a list of the created or updated
// node ip addresses.
func runApply(ctx context.Context, dir string, log pb.MessageWriter) (map[string]tfexec.OutputMeta, error) {
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
