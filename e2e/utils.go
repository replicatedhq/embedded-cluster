package e2e

import (
	"bytes"
	"context"
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
		if err != nil {
			t.Logf("stdout:\n%s", stdout)
			t.Logf("stderr:\n%s", stderr)
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
