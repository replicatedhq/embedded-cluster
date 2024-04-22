package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	return nil
}

// RunCommandsOnNode runs a series of commands on a node.
func RunCommandsOnNode(t *testing.T, cl *cluster.Output, node int, cmds [][]string) error {
	for _, cmd := range cmds {
		cmdstr := strings.Join(cmd, " ")
		t.Logf("running `%s` node %d", cmdstr, node)
		stdout, stderr, err := RunCommandOnNode(t, cl, node, cmd)
		if len(stdout) > 0 || len(stderr) > 0 {
			t.Logf("-----")
			t.Logf("stdout:\n%s", stdout)
			t.Logf("stderr:\n%s", stderr)
			t.Logf("-----")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// RunRegularUserCommandOnNode runs a command on a node as a regular user (not root) with a timeout.
func RunRegularUserCommandOnNode(t *testing.T, cl *cluster.Output, node int, line []string) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:        cl.Nodes[node],
		Line:        line,
		Stdout:      stdout,
		Stderr:      stderr,
		RegularUser: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s\nstderr:%s\n", stdout.String(), stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnNode runs a command on a node with a timeout.
func RunCommandOnNode(t *testing.T, cl *cluster.Output, node int, line []string) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   cl.Nodes[node],
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnProxyNode runs a command on the proxy node with a timeout.
func RunCommandOnProxyNode(t *testing.T, cl *cluster.Output, line []string) (string, string, error) {
	if cl.Proxy == "" {
		return "", "", fmt.Errorf("no proxy node found")
	}

	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := cluster.Command{
		Node:   cl.Proxy,
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

var commandOutputRegex = regexp.MustCompile(`{"command":"[^"]*"}`)

type nodeJoinResponse struct {
	Command string `json:"command"`
}

// findJoinCommandInOutput parses the output of the testim.sh script and returns the join command.
func findJoinCommandInOutput(stdout string) (string, error) {
	output := commandOutputRegex.FindString(stdout)
	if output == "" {
		return "", fmt.Errorf("failed to find the join command in the output: %s", stdout)
	}
	var r nodeJoinResponse
	if err := json.Unmarshal([]byte(output), &r); err != nil {
		return "", fmt.Errorf("failed to parse node join response: %v", err)
	}
	// trim down the "./" and the "sudo" command as those are not needed. we run as
	// root and the embedded-cluster binary is on the PATH.
	command := strings.TrimPrefix(r.Command, "sudo ./")
	// replace the airgap bundle path (if any) with the local path.
	command = strings.ReplaceAll(command, "embedded-cluster.airgap", "/tmp/release.airgap")
	return command, nil
}
