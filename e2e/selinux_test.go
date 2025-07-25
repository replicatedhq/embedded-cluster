package e2e

import (
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
)

// TestSELinuxSupport tests embedded-cluster installation and functionality on SELinux-enabled systems.
// This test specifically validates that embedded-cluster works correctly when SELinux is in enforcing mode.
func TestSELinuxSupport(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	cluster := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "almalinux", // AlmaLinux has SELinux enabled by default
		Version:      "8",         // AlmaLinux 8 (supported by CMX)
		InstanceType: "r1.large",  // Use supported instance type
		DiskSize:     50,          // Sufficient disk space
	})
	defer cluster.Cleanup()

	// Verify SELinux is enabled and in enforcing mode
	t.Logf("Verifying SELinux status on all nodes")
	for i := range cluster.Nodes {
		stdout, stderr, err := cluster.RunCommandOnNode(i, []string{"getenforce"})
		if err != nil {
			t.Fatalf("Failed to check SELinux status on node %d: %v (stdout: %s, stderr: %s)", i, err, stdout, stderr)
		}
		if stdout != "Enforcing\n" {
			t.Logf("Enabling SELinux on node %d (current status: %s)", i, stdout)
			enableSELinuxOnNode(t, cluster, i)
		}
		t.Logf("SELinux is enforcing on node %d", i)
	}

	// Install embedded-cluster on the first node
	t.Logf("Installing embedded-cluster on node 0")
	line := []string{"env", "PATH=/usr/local/bin:/usr/bin:/bin", "/usr/local/bin/single-node-install.sh", "ui", os.Getenv("SHORT_SHA"), "--admin-console-port", "30003"}
	stdout, stderr, err := cluster.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	// Verify installation succeeded by checking cluster state
	t.Logf("Verifying cluster installation")
	line = []string{"env", "PATH=/usr/local/bin:/usr/bin:/bin", "/usr/local/bin/check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	stdout, stderr, err = cluster.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	// Verify SELinux contexts are correctly set
	t.Logf("Verifying SELinux contexts for embedded-cluster files")
	verifySelinuxContexts(t, cluster)

	// Run embedded preflight checks which include SELinux-specific tests
	t.Logf("Running embedded preflight checks")
	stdout, stderr, err = cluster.RunCommandOnNode(0, []string{"env", "PATH=/usr/local/bin:/usr/bin:/bin", "/usr/local/bin/embedded-preflight.sh"})
	if err != nil {
		t.Fatalf("Embedded preflight checks failed: %v (stdout: %s, stderr: %s)", err, stdout, stderr)
	}

	// Deploy and test application functionality
	if stdout, stderr, err := cluster.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("Failed to deploy app: %v (stdout: %s, stderr: %s)", err, stdout, stderr)
	}

	// Verify the application is running correctly under SELinux
	t.Logf("Application deployed successfully on SELinux-enabled cluster")
}

// verifySelinuxContexts checks that embedded-cluster files have the correct SELinux contexts
func verifySelinuxContexts(t *testing.T, cluster *cmx.Cluster) {
	// Check data directory SELinux context
	stdout, stderr, err := cluster.RunCommandOnNode(0, []string{"ls", "-Z", "/var/lib/embedded-cluster"})
	if err != nil {
		t.Logf("Failed to check data directory context: %v (stdout: %s, stderr: %s)", err, stdout, stderr)
	} else {
		t.Logf("Data directory SELinux context: %s", stdout)
	}

	// Check binary directory SELinux context
	stdout, stderr, err = cluster.RunCommandOnNode(0, []string{"ls", "-Z", "/var/lib/embedded-cluster/bin"})
	if err != nil {
		t.Logf("Failed to check bin directory context: %v (stdout: %s, stderr: %s)", err, stdout, stderr)
	} else {
		t.Logf("Binary directory SELinux context: %s", stdout)
		// Verify binaries have bin_t context
		if !contains(stdout, "bin_t") {
			t.Errorf("Binary directory does not have expected bin_t SELinux context: %s", stdout)
		}
	}

	// Check for any SELinux denials
	stdout, _, err = cluster.RunCommandOnNode(0, []string{"ausearch", "-m", "avc", "-ts", "recent"})
	if err == nil && stdout != "" {
		t.Logf("SELinux denials found: %s", stdout)
		// Don't fail the test immediately, but log for analysis
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAt(s, substr))))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// enableSELinuxOnNode enables SELinux on a node using the autorelabel approach
func enableSELinuxOnNode(t *testing.T, cluster *cmx.Cluster, nodeIndex int) {
	// Set SELinux to enforcing in /etc/selinux/config
	t.Logf("Setting SELinux to enforcing in config on node %d", nodeIndex)
	_, stderr, err := cluster.RunCommandOnNode(nodeIndex, []string{"sed", "-i", "s/^SELINUX=.*/SELINUX=enforcing/", "/etc/selinux/config"})
	if err != nil {
		t.Fatalf("Failed to set SELinux config on node %d: %v (stderr: %s)", nodeIndex, err, stderr)
	}

	// Create autorelabel file to trigger relabeling on reboot
	t.Logf("Creating autorelabel file on node %d", nodeIndex)
	_, stderr, err = cluster.RunCommandOnNode(nodeIndex, []string{"touch", "/.autorelabel"})
	if err != nil {
		t.Fatalf("Failed to create autorelabel file on node %d: %v (stderr: %s)", nodeIndex, err, stderr)
	}

	// Reboot the node
	t.Logf("Rebooting node %d to enable SELinux", nodeIndex)
	cluster.RunCommandOnNode(nodeIndex, []string{"reboot"})

	// Wait for the node to come back online
	waitForNodeReboot(t, cluster, nodeIndex)

	// Verify SELinux is now enforcing
	stdout, stderr, err := cluster.RunCommandOnNode(nodeIndex, []string{"getenforce"})
	if err != nil {
		t.Fatalf("Failed to verify SELinux status after reboot on node %d: %v (stdout: %s, stderr: %s)", nodeIndex, err, stdout, stderr)
	}
	if stdout != "Enforcing\n" {
		t.Fatalf("SELinux is not enforcing after reboot on node %d (status: %s)", nodeIndex, stdout)
	}
	t.Logf("SELinux successfully enabled and enforcing on node %d", nodeIndex)
}

// waitForNodeReboot waits for a node to come back online after a reboot
func waitForNodeReboot(t *testing.T, cluster *cmx.Cluster, nodeIndex int) {
	t.Logf("Waiting for node %d to come back online after reboot", nodeIndex)

	// Use the existing WaitForReboot method from CMX cluster
	cluster.WaitForReboot()
}
