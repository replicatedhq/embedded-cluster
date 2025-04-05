package cmx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type network struct {
	// ID is the unique identifier for the Network
	ID string `json:"id"`

	// Status is the status of the Network
	Status string `json:"status"`
}

func createNetwork(ctx context.Context, gid string, ttl string) (*network, error) {
	cmd := exec.CommandContext(ctx, "replicated", "network", "create", "--name", gid, "--ttl", ttl, "--wait", "2m", "--output", "json")

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

	var networks []*network
	if err := json.Unmarshal(outBuf.Bytes(), &networks); err != nil {
		return nil, fmt.Errorf("parse replicated network create output: %v", err)
	} else if len(networks) == 0 {
		return nil, fmt.Errorf("no network created")
	} else if networks[0].Status != "running" {
		return nil, fmt.Errorf("network %s is not running", networks[0].ID)
	}

	return networks[0], nil
}

func deleteNetwork(ctx context.Context, networkID string) error {
	cmd := exec.CommandContext(ctx, "replicated", "network", "rm", "--name", networkID)

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
