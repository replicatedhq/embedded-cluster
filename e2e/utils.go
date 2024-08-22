package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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

func RequireEnvVars(t *testing.T, envVars []string) {
	for _, envVar := range envVars {
		if os.Getenv(envVar) == "" {
			t.Fatalf("missing required environment variable: %s", envVar)
		}
	}
}

type RunCommandOption func(cmd *cluster.Command)

func WithEnv(env map[string]string) RunCommandOption {
	return func(cmd *cluster.Command) {
		cmd.Env = env
	}
}

// RunCommandsOnNode runs a series of commands on a node.
func RunCommandsOnNode(t *testing.T, cl *cluster.Output, node int, cmds [][]string, opts ...RunCommandOption) error {
	for _, cmd := range cmds {
		cmdstr := strings.Join(cmd, " ")
		t.Logf("running `%s` node %d", cmdstr, node)
		_, _, err := RunCommandOnNode(t, cl, node, cmd, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunRegularUserCommandOnNode runs a command on a node as a regular user (not root) with a timeout.
func RunRegularUserCommandOnNode(t *testing.T, cl *cluster.Output, node int, line []string, opts ...RunCommandOption) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &cluster.Command{
		Node:        cl.Nodes[node],
		Line:        line,
		Stdout:      stdout,
		Stderr:      stderr,
		RegularUser: true,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, *cmd); err != nil {
		t.Logf("stdout:\n%s\nstderr:%s\n", stdout.String(), stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnNode runs a command on a node with a timeout.
func RunCommandOnNode(t *testing.T, cl *cluster.Output, node int, line []string, opts ...RunCommandOption) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &cluster.Command{
		Node:   cl.Nodes[node],
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, *cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnProxyNode runs a command on the proxy node with a timeout.
func RunCommandOnProxyNode(t *testing.T, cl *cluster.Output, line []string, opts ...RunCommandOption) (string, string, error) {
	if cl.Proxy == "" {
		return "", "", fmt.Errorf("no proxy node found")
	}

	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &cluster.Command{
		Node:   cl.Proxy,
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := cluster.Run(ctx, t, *cmd); err != nil {
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

// findJoinCommandInOutput parses the output of the playwright.sh script and returns the join command.
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
	command = strings.ReplaceAll(command, "embedded-cluster.airgap", "/assets/release.airgap")
	return command, nil
}

func injectString(original, injection, after string) string {
	// Split the string into parts using the 'after' substring
	parts := strings.SplitN(original, after, 2)
	if len(parts) < 2 {
		// If 'after' substring is not found, return the original string
		return original
	}
	// Construct the new string by adding the injection between the parts
	return parts[0] + after + " " + injection + parts[1]
}

func k8sVersion() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse k8s version %q", os.Getenv("EXPECT_K0S_VERSION")))
	}
	return verParts[0]
}

func k8sVersionPrevious() string {
	// split the version string (like 'v1.29.6+k0s.0') into the k8s version and the k0s revision
	verParts := strings.Split(os.Getenv("EXPECT_K0S_VERSION_PREVIOUS"), "+")
	if len(verParts) < 2 {
		panic(fmt.Sprintf("failed to parse previous k8s version %q", os.Getenv("EXPECT_K0S_VERSION_PREVIOUS")))
	}
	return verParts[0]
}

func runInParallel(t *testing.T, fns ...func(t *testing.T) error) {
	t.Helper()
	errCh := make(chan error, len(fns))
	for _, fn := range fns {
		go func(fn func(t *testing.T) error) {
			errCh <- fn(t)
		}(fn)
	}
	for range fns {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}
