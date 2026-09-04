package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/ec2"
)

// TestSingleNodeSelinuxRHEL verifies that embedded cluster installs and runs
// confined on a selinux-enforcing Red Hat Enterprise Linux host.
//
// RHEL rather than ubuntu because workload confinement comes from
// container-selinux, which only ships on RHEL-family distros. On ubuntu there
// is no container_t for workloads to run in, so the most such a run can show
// is that nothing crashed.
//
// Online rather than airgap, and no playwright: every bundled addon installs
// either way, and those are what we check confinement on. Airgap emulation is
// unrelated to selinux and is what makes a cluster backend expensive.
//
// Covers a fresh install and an upgrade. Upgrades do not apply selinux policy
// -- the policy store is not reachable from the pod that runs the upgrade's
// host-side steps -- so an upgraded cluster keeps whatever the installing
// release applied. What the upgrade half checks is that confinement does not
// break the upgrade itself.
func TestSingleNodeSelinuxRHEL(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"})

	tc := ec2.NewCluster(&ec2.ClusterInput{T: t})
	defer tc.Cleanup()

	// semanage lives in policycoreutils-python-utils, which the RHEL cloud
	// image does not carry. container-selinux should already be present, but
	// name it explicitly since everything below depends on it.
	t.Logf("%s: installing selinux tooling", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"dnf install -y policycoreutils-python-utils container-selinux",
	}); err != nil {
		t.Fatalf("fail to install selinux tooling: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: setting selinux to Enforcing mode", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"setenforce 1 && getenforce"}); err != nil || !strings.Contains(stdout, "Enforcing") {
		t.Fatalf("fail to set selinux to Enforcing mode: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: copying binary and license to node", time.Now().Format(time.RFC3339))
	if err := tc.CopyFileToNode(0, "../output/bin/embedded-cluster", "/usr/local/bin/embedded-cluster"); err != nil {
		t.Fatalf("fail to copy embedded-cluster binary: %v", err)
	}
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"mkdir", "-p", "/assets"}); err != nil {
		t.Fatalf("fail to create /assets: %v: %s: %s", err, stdout, stderr)
	}
	if err := tc.CopyFileToNode(0, "licenses/license.yaml", "/assets/license.yaml"); err != nil {
		t.Fatalf("fail to copy license: %v", err)
	}

	// deliberately not labeling anything first: labeling the host is what we
	// are testing. Doing it here previously masked the product's own call.
	t.Logf("%s: installing embedded-cluster", time.Now().Format(time.RFC3339))
	line := []string{
		"embedded-cluster", "install", "--yes",
		"--license", "/assets/license.yaml",
		"--admin-console-password", "password",
	}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster: %v: %s: %s", err, stdout, stderr)
	}

	checkSELinuxLabels(t, tc, 0)
	checkSELinuxConfinement(t, tc, 0)

	// The denial sweep only means something once the workloads have exercised
	// their host mounts, which is where velero's node-agent failed.
	t.Logf("%s: waiting for workloads to settle", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"kubectl wait --for=condition=Ready pods --all -A --timeout=10m",
	}, map[string]string{"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf"}); err != nil {
		t.Fatalf("pods did not become ready under selinux enforcing: %v: %s: %s", err, stdout, stderr)
	}

	// The upgrade is where confinement is hardest: the artifacts job is an
	// ordinary pod that mounts the data directory and rewrites the binaries in
	// it, which container_t is denied. It runs as spc_t for that reason, and if
	// that stopped working the operator would wait on a job that never
	// succeeds, so reaching the end of the upgrade is the assertion.
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	t.Logf("%s: upgrading to %s", time.Now().Format(time.RFC3339), appUpgradeVersion)
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", appUpgradeVersion); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	// Upgrades do not re-run ConfigureHost, so this checks that the labels and
	// the drop-in survive an upgrade, not that the upgrade applied them.
	checkSELinuxLabels(t, tc, 0)
	checkSELinuxConfinement(t, tc, 0)

	t.Logf("%s: waiting for workloads to settle after upgrade", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{
		"kubectl wait --for=condition=Ready pods --all -A --timeout=10m",
	}, map[string]string{"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf"}); err != nil {
		t.Fatalf("pods did not become ready after upgrade: %v: %s: %s", err, stdout, stderr)
	}

	// Covers the whole run, install and upgrade both.
	checkNoSELinuxDenials(t, tc, 0)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
