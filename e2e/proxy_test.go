package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/require"
)

// SkipProxyTest returns true if the k0s version in use does not support
// proxied environments.
func SkipProxyTest() bool {
	supportedVersion := semver.MustParse("1.29.0")
	currentVersion := semver.MustParse(versions.K0sVersion)
	return currentVersion.LessThan(supportedVersion)
}

// TestProxiedEnvironment tests the installation behind a proxy server with custom pod and service CIDRs
func TestProxiedEnvironment(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	requiredEnvVars := []string{
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               3,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "licenses/snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("Proxied infrastructure created")

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		failOnProxyTCPDenied(t, tc)
	})

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	installSingleNodeWithOptions(t, tc, installOptions{
		httpProxy:   lxd.HTTPProxy,
		httpsProxy:  lxd.HTTPProxy,
		noProxy:     strings.Join(tc.IPs, ","),
		podCidr:     "10.128.0.0/20",
		serviceCidr: "10.129.0.0/20",
		withEnv:     lxd.WithProxyEnv(tc.IPs),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join a worker node
	joinWorkerNode(t, tc, 2)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, nil)

	// check the installation state
	checkInstallationState(t, tc)

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", appUpgradeVersion); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 2, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 1, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 0, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 0: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		},
	)

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	line = append(line, "--http-proxy", lxd.HTTPProxy)
	line = append(line, "--https-proxy", lxd.HTTPProxy)
	line = append(line, "--no-proxy", strings.Join(tc.IPs, ","))
	line = append(line, "--pod-cidr", "10.128.0.0/20")
	line = append(line, "--service-cidr", "10.129.0.0/20")
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithMITMProxy(t *testing.T) {
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	requiredEnvVars := []string{
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               3,
		WithProxy:           true,
		Image:               "debian/12",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
		LicensePath:         "licenses/snapshot-license.yaml",
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithMITMProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		failOnProxyTCPDenied(t, tc)
	})

	// TODO: our preflight checks do not yet fail when run with a MITM proxy, the MITM CA cert on the host, but without the CA cert passed as a CLI arg
	//// test to ensure that preflight checks fail without the CA cert
	//t.Logf("%s: checking preflight checks with MITM proxy", time.Now().Format(time.RFC3339))
	//line = []string{"check-preflights-fail.sh", "--http-proxy", lxd.HTTPMITMProxy, "--https-proxy", lxd.HTTPMITMProxy}
	//if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithMITMProxyEnv(tc.IPs)); err != nil {
	//	t.Fatalf("fail to check preflight checks: %v: %s: %s", err, stdout, stderr)
	//} else {
	//	t.Logf("Preflight checks failed as expected:\n%s\n%s", stdout, stderr)
	//}

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	installSingleNodeWithOptions(t, tc, installOptions{
		httpProxy:  lxd.HTTPMITMProxy,
		httpsProxy: lxd.HTTPMITMProxy,
		withEnv:    lxd.WithMITMProxyEnv(tc.IPs),
	})

	_, _, err := tc.SetupPlaywrightAndRunTest("deploy-app")
	require.NoError(t, err, "failed to deploy app")

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join a worker node
	joinWorkerNode(t, tc, 2)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, nil)

	// check the installation state
	checkInstallationState(t, tc)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", appUpgradeVersion); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 2, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 1, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 0, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 0: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		},
	)

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	line = append(line, "--http-proxy", lxd.HTTPMITMProxy)
	line = append(line, "--https-proxy", lxd.HTTPMITMProxy)
	line = append(line, "--no-proxy", strings.Join(tc.IPs, ","))
	line = append(line, "--private-ca", "/usr/local/share/ca-certificates/proxy/ca.crt")
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithMITMProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func failOnProxyTCPDenied(t *testing.T, tc *lxd.Cluster) {
	line := []string{"sh", "-c", `grep -A1 TCP_DENIED /var/log/squid/access.log | grep -v speedtest\.net`}
	stdout, _, err := tc.RunCommandOnProxyNode(t, line)
	if err != nil {
		t.Fatalf("fail to check squid access log: %v", err)
	}
	t.Logf("TCP_DENIED logs:")
	t.Log(stdout)
	if strings.Contains(stdout, "TCP_DENIED") {
		t.Fatalf("TCP_DENIED logs found")
	}
}
