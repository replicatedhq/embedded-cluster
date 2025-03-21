package cmx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
)

// createNodeOpts represents the configuration options for creating a VM node
type createNodeOpts struct {
	// Distribution is the Linux distribution of the VM to provision
	Distribution string

	// Version is the version to provision (format depends on distribution)
	Version string

	// Count is the number of matching VMs to create (default 1)
	Count int

	// InstanceType is the type of instance to use (e.g. r1.medium)
	InstanceType string

	// DiskSize is the disk size in GiB to request per node (default 50)
	DiskSize int
}

// node represents a VM node instance
type node struct {
	// ID is the unique identifier for the VM
	ID string `json:"id"`

	// Status is the status of the VM
	Status string `json:"status"`

	// DirectSSHEndpoint is the endpoint to connect to the VM via SSH
	DirectSSHEndpoint string `json:"direct_ssh_endpoint"`

	// DirectSSHPort is the port to connect to the VM via SSH
	DirectSSHPort int `json:"direct_ssh_port"`
}

// createNodes creates nodes in the network with the given group ID and configuration
func createNodes(ctx context.Context, gid string, networkID string, ttl string, opts createNodeOpts) ([]*node, error) {
	// Build the command with required flags
	args := []string{
		"vm", "create", "-o", "json",
		"--network", networkID,
		"--tag", fmt.Sprintf("ec.e2e.group-id=%s", gid),
		"--ttl", ttl,
		"--wait", "5m",
	}

	if opts.Distribution != "" {
		args = append(args, "--distribution", opts.Distribution)
	}
	if opts.Version != "" {
		args = append(args, "--version", opts.Version)
	}
	if opts.Count > 0 {
		args = append(args, "--count", strconv.Itoa(opts.Count))
	}
	if opts.InstanceType != "" {
		args = append(args, "--instance-type", opts.InstanceType)
	}
	if opts.DiskSize > 0 {
		args = append(args, "--disk", strconv.Itoa(opts.DiskSize))
	}

	// Execute replicated CLI command
	cmd := exec.CommandContext(ctx, "replicated", args...)

	apiTokenEnv, err := replicatedApiTokenEnv()
	if err != nil {
		return nil, err
	}
	cmd.Env = append(cmd.Environ(), apiTokenEnv...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("err: %v, stderr: %s", err, errBuf.String())
	}

	// Parse the JSON output
	var nodes []*node
	if err := json.Unmarshal(outBuf.Bytes(), &nodes); err != nil {
		return nil, fmt.Errorf("parse replicated vm create output: %v", err)
	}

	for _, node := range nodes {
		if node.Status != "running" {
			return nil, fmt.Errorf("VM %s is not running", node.ID)
		}
	}

	return nodes, nil
}

func deleteNodesByGroupID(ctx context.Context, gid string) error {
	cmd := exec.CommandContext(ctx, "replicated", "vm", "delete", "--tag", fmt.Sprintf("ec.e2e.group-id=%s", gid))

	apiTokenEnv, err := replicatedApiTokenEnv()
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Environ(), apiTokenEnv...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("err: %v, output: %s", err, string(output))
	}
	return nil
}
