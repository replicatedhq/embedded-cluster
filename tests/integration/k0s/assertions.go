package k0s

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

func assertK0sVersion(t *testing.T, node *K0sNode, expectedVersion string) {
	var stdout bytes.Buffer
	code, err := node.container.Exec(
		[]string{"k0s", "version"},
		dockertest.ExecOptions{StdOut: &stdout},
	)
	require.NoError(t, err)
	require.Equal(t, 0, code, "k0s version command failed")

	output := stdout.String()
	require.Contains(t, output, expectedVersion, "k0s version mismatch")
	t.Logf("Node %s is running k0s %s", node.name, expectedVersion)
}

func assertAllNodesReady(t *testing.T, cluster *K0sCluster, expectedNodes int) {
	var stdout bytes.Buffer
	code, err := cluster.nodes[0].container.Exec(
		[]string{"k0s", "kubectl", "get", "nodes", "--no-headers"},
		dockertest.ExecOptions{StdOut: &stdout},
	)
	require.NoError(t, err)
	require.Equal(t, 0, code, "kubectl get nodes failed")

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Equal(t, expectedNodes, len(lines), "Expected %d nodes, got %d", expectedNodes, len(lines))

	// Verify all nodes are Ready
	for _, line := range lines {
		require.Contains(t, line, "Ready", "Node is not ready: %s", line)
	}

	t.Logf("All %d nodes are Ready", expectedNodes)
}

func assertNoExternalNetworkCalls(t *testing.T, node *K0sNode) {
	// Try to reach github.com (should fail in airgap)
	code, _ := node.container.Exec(
		[]string{"sh", "-c", "curl -m 5 https://github.com"},
		dockertest.ExecOptions{},
	)
	require.NotEqual(t, 0, code, "External network call should fail in airgap mode")
	t.Logf("Verified node %s cannot reach external network", node.name)
}

func assertAutopilotPlanExists(t *testing.T, node *K0sNode) {
	var stdout bytes.Buffer
	code, err := node.container.Exec(
		[]string{"k0s", "kubectl", "get", "plan", "-n", "kube-system", "-o", "name"},
		dockertest.ExecOptions{StdOut: &stdout},
	)
	require.NoError(t, err)
	require.Equal(t, 0, code, "kubectl get plan failed")
	require.Contains(t, stdout.String(), "plan/autopilot", "Autopilot plan not found")
	t.Log("Autopilot plan exists")
}

func assertAutopilotPlanComplete(t *testing.T, node *K0sNode, timeoutSeconds int) {
	t.Logf("Waiting up to %d seconds for autopilot plan to complete", timeoutSeconds)

	for i := 0; i < timeoutSeconds; i++ {
		var stdout bytes.Buffer
		code, err := node.container.Exec(
			[]string{"k0s", "kubectl", "get", "plan", "autopilot", "-n", "kube-system", "-o", "jsonpath={.status.state}"},
			dockertest.ExecOptions{StdOut: &stdout},
		)

		if err == nil && code == 0 {
			state := strings.TrimSpace(stdout.String())
			if state == "Completed" {
				t.Log("Autopilot plan completed successfully")
				return
			}
			if i%10 == 0 {
				t.Logf("Autopilot plan state: %s (waiting...)", state)
			}
		}

		// Check every 2 seconds
		if i < timeoutSeconds-1 {
			// Sleep inline - we're checking every iteration
			code, _ := node.container.Exec(
				[]string{"sleep", "2"},
				dockertest.ExecOptions{},
			)
			if code != 0 {
				break
			}
		}
	}

	// If we get here, timeout was reached
	var stdout bytes.Buffer
	node.container.Exec(
		[]string{"k0s", "kubectl", "get", "plan", "autopilot", "-n", "kube-system", "-o", "yaml"},
		dockertest.ExecOptions{StdOut: &stdout},
	)
	t.Logf("Autopilot plan status:\n%s", stdout.String())
	t.Fatalf("Autopilot plan did not complete within %d seconds", timeoutSeconds)
}
